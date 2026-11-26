package tokens

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/cards"
	"github.com/ReanSn0w/justkb/internal/comments"
	"github.com/ReanSn0w/justkb/internal/testdb"
)

func TestBotLifecycleAndIsolation(t *testing.T) {
	ctx := context.Background()
	db := testdb.Open(t).DB
	a := auth.NewService(db)
	b := board.NewService(db)
	s := NewService(db)
	admin, err := a.CreateAdmin(ctx, "admin", "Admin", "test-password-long")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.CreateBot(ctx, admin.ID, "Agent One")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateBot(ctx, admin.ID, "Agent Two")
	if err != nil {
		t.Fatal(err)
	}
	user, err := a.UserByID(ctx, first.UserID)
	if err != nil || user.AccountType != "bot" || user.Role != "member" {
		t.Fatal("invalid bot identity")
	}
	if _, err = a.Login(ctx, user.Username, "", time.Hour); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatal("bot login accepted")
	}
	if err = b.ResetUserPasswordByUsername(ctx, user.Username, "new-test-password"); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("password reset: %v", err)
	}
	if err = a.ChangePassword(ctx, user.ID, "", "new-test-password"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatal("password change accepted")
	}
	if _, err = b.UpdateUser(ctx, user.ID, "Elevated", "", "admin", true); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("bot promotion accepted")
	}
	if _, err = s.Create(ctx, user.ID, "extra", []string{"cards:read"}, nil); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("extra PAT accepted")
	}
	if err = s.Revoke(ctx, user.ID, first.Token.ID); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("ordinary revoke accepted")
	}
	if _, err = s.CreateBot(ctx, second.UserID, "Nested"); !errors.Is(err, auth.ErrForbidden) {
		t.Fatal("bot created another bot")
	}
	for _, bot := range []CreatedBot{first, second} {
		p, e := s.Authenticate(ctx, bot.Token.Secret)
		if e != nil || p.User.ID != bot.UserID || !p.Scopes["cards:read"] || !p.Scopes["cards:write"] || bot.Token.ExpiresAt != nil {
			t.Fatal("invalid bot token principal")
		}
	}
	cols, err := b.ListColumns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c := cards.NewService(db)
	actor := cards.Actor{UserID: first.UserID, TokenID: first.Token.ID}
	card, err := c.Create(ctx, actor, cards.CreateInput{Title: "Bot task", ColumnID: cols[0].ID, AssigneeIDs: []string{first.UserID, second.UserID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = comments.NewService(db).CreateCard(ctx, card.ID, comments.Actor{UserID: second.UserID, TokenID: second.Token.ID}, "Agent discussion"); err != nil {
		t.Fatal(err)
	}
	disabled := false
	if _, err = b.SaveColumn(ctx, cols[1].ID, cols[1].Name, nil, &disabled); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Create(ctx, actor, cards.CreateInput{Title: "Forbidden", ColumnID: cols[1].ID}); err == nil {
		t.Fatal("bot bypassed column policy")
	}
	if _, err = c.Move(ctx, actor, card.ID, cards.MoveInput{ExpectedVersion: card.Version, TargetColumnID: cols[1].ID}); err != nil {
		t.Fatal(err)
	}
	rotated, err := s.RotateBot(ctx, admin.ID, user.ID, first.Token.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(ctx, first.Token.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("old token still valid")
	}
	if _, err = s.Authenticate(ctx, rotated.Secret); err != nil {
		t.Fatal(err)
	}
	if _, err = b.UpdateUser(ctx, user.ID, "Renamed", "", "member", false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(ctx, rotated.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("inactive bot authenticated")
	}
	if _, err = b.UpdateUser(ctx, user.ID, "Renamed", "", "member", true); err != nil {
		t.Fatal(err)
	}
	p, err := s.Authenticate(ctx, rotated.Secret)
	if err != nil || p.User.DisplayName != "Renamed" {
		t.Fatal("reactivation failed")
	}
	if _, err = s.Authenticate(ctx, first.Token.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("revoked token revived")
	}
	var n int
	if err = db.QueryRow(`SELECT count(*) FROM card_activity WHERE actor_user_id=? AND actor_token_id=?`, user.ID, first.Token.ID).Scan(&n); err != nil || n != 2 {
		t.Fatal("history lost")
	}
	// Humans retain multiple independent PATs.
	for i := 0; i < 2; i++ {
		if _, err = s.Create(ctx, admin.ID, "Human token", []string{"cards:read"}, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBotRotationConcurrencyAndRollback(t *testing.T) {
	ctx := context.Background()
	db := testdb.Open(t).DB
	s := NewService(db)
	admin, err := auth.NewService(db).CreateAdmin(ctx, "admin", "Admin", "test-password-long")
	if err != nil {
		t.Fatal(err)
	}
	bot, err := s.CreateBot(ctx, admin.ID, "Agent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_new_token BEFORE INSERT ON api_tokens BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RotateBot(ctx, admin.ID, bot.UserID, bot.Token.ID); err == nil {
		t.Fatal("expected rotation failure")
	}
	if _, err = s.Authenticate(ctx, bot.Token.Secret); err != nil {
		t.Fatal("failed rotation revoked old token")
	}
	if _, err = s.CreateBot(ctx, admin.ID, "Failed agent"); err == nil {
		t.Fatal("expected creation failure")
	}
	var n int
	if err = db.QueryRow(`SELECT count(*) FROM users WHERE display_name='Failed agent'`).Scan(&n); err != nil || n != 0 {
		t.Fatal("orphan bot survived")
	}
	if _, err = db.Exec(`DROP TRIGGER fail_new_token`); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, e := s.RotateBot(ctx, admin.ID, bot.UserID, bot.Token.ID)
			results <- e
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, ErrBotTokenConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	if err = db.QueryRow(`SELECT count(*) FROM api_tokens WHERE user_id=? AND revoked_at IS NULL`, bot.UserID).Scan(&n); err != nil || n != 1 {
		t.Fatal("not exactly one live token")
	}
}
