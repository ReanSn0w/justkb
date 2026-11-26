package cards

import (
	"context"
	"errors"
	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/testdb"
	"testing"
)

func TestCreationPolicyDoesNotRestrictExistingCards(t *testing.T) {
	ctx := context.Background()
	db := testdb.Open(t).DB
	user, err := auth.NewService(db).CreateAdmin(ctx, "admin", "Admin", "test-password-long")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	directory := board.NewService(db)
	cols, err := directory.ListColumns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{UserID: user.ID}
	card, err := service.Create(ctx, actor, CreateInput{Title: "Existing", ColumnID: cols[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	denied := false
	if _, err = directory.SaveColumn(ctx, cols[1].ID, cols[1].Name, nil, &denied); err != nil {
		t.Fatal(err)
	}
	var before int64
	db.QueryRow(`SELECT next_number FROM card_number_sequence`).Scan(&before)
	_, err = service.Create(ctx, actor, CreateInput{Title: "Rejected", ColumnID: cols[1].ID})
	var policy *CreationNotAllowedError
	if !errors.As(err, &policy) || policy.ColumnID != cols[1].ID {
		t.Fatalf("expected policy error, got %v", err)
	}
	var after, count int64
	db.QueryRow(`SELECT next_number FROM card_number_sequence`).Scan(&after)
	db.QueryRow(`SELECT count(*) FROM card_activity`).Scan(&count)
	if before != after || count != 1 {
		t.Fatalf("failed create changed number/activity: %d %d %d", before, after, count)
	}
	card, err = service.Move(ctx, actor, card.ID, MoveInput{ExpectedVersion: card.Version, TargetColumnID: cols[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	title := "Edited"
	card, err = service.Update(ctx, actor, card.ID, UpdateInput{ExpectedVersion: card.Version, Title: &title})
	if err != nil {
		t.Fatal(err)
	}
	card, err = service.SetArchived(ctx, actor, card.ID, card.Version, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetArchived(ctx, actor, card.ID, card.Version, false); err != nil {
		t.Fatal(err)
	}
	col, err := directory.SaveColumn(ctx, cols[1].ID, "Renamed", nil)
	if err != nil || col.AllowCardCreation {
		t.Fatalf("rename reset policy: %v %v", col, err)
	}
	allowed := true
	if _, err = directory.SaveColumn(ctx, col.ID, col.Name, nil, &allowed); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Create(ctx, actor, CreateInput{Title: "Allowed", ColumnID: col.ID}); err != nil {
		t.Fatal(err)
	}
}
