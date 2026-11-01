package cards

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/testdb"
)

func TestCardLifecycleIntegration(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	user, err := auth.NewService(db).CreateAdmin(ctx, "admin", "Admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	var firstColumn, secondColumn string
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position LIMIT 1`).Scan(&firstColumn); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position LIMIT 1 OFFSET 1`).Scan(&secondColumn); err != nil {
		t.Fatal(err)
	}
	tag, err := board.NewService(db).SaveTag(ctx, "", "bug", "#CC0000")
	if err != nil {
		t.Fatal(err)
	}
	tagID := tag.ID
	service := NewService(db)
	actor := Actor{UserID: user.ID}
	card, err := service.Create(ctx, actor, CreateInput{Title: "Fix login", ColumnID: firstColumn, TagIDs: []string{tagID}})
	if err != nil {
		t.Fatal(err)
	}
	if card.Number != 1 || len(card.TagIDs) != 1 || len(card.Tags) != 1 || card.Tags[0].Name != "bug" || card.Version != 1 {
		t.Fatalf("card = %#v", card)
	}
	if _, err := service.Create(ctx, actor, CreateInput{Title: "Second tagged card", ColumnID: firstColumn, TagIDs: []string{tagID}}); err != nil {
		t.Fatal(err)
	}
	page, err := service.List(ctx, Filters{TagIDs: []string{tagID}, Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	nextPage, err := service.List(ctx, Filters{TagIDs: []string{tagID}, Limit: 1, Cursor: page.NextCursor})
	if err != nil || len(nextPage.Items) != 1 || nextPage.Items[0].ID == page.Items[0].ID {
		t.Fatalf("next page=%#v err=%v", nextPage, err)
	}
	title := "Fix secure login"
	updated, err := service.Update(ctx, actor, card.ID, UpdateInput{ExpectedVersion: card.Version, Title: &title})
	if err != nil || updated.Version != 2 {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	if _, err := service.Update(ctx, actor, card.ID, UpdateInput{ExpectedVersion: 1, Title: &title}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	moved, err := service.Move(ctx, actor, card.ID, MoveInput{ExpectedVersion: updated.Version, TargetColumnID: secondColumn})
	if err != nil || moved.ColumnID != secondColumn || moved.Version != 3 {
		t.Fatalf("moved=%#v err=%v", moved, err)
	}
	movedAgain, err := service.Move(ctx, actor, card.ID, MoveInput{ExpectedVersion: moved.Version, TargetColumnID: firstColumn})
	if err != nil || movedAgain.ColumnID != firstColumn || movedAgain.Version != 4 {
		t.Fatalf("moved again=%#v err=%v", movedAgain, err)
	}
	movements, err := service.ListMovements(ctx, MovementFilters{From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour), ActorUserID: user.ID, Limit: 1})
	if err != nil || len(movements.Items) != 1 || movements.NextCursor == "" || movements.Items[0].CardTitle != "Fix secure login" || movements.Items[0].Actor.DisplayName != "Admin" {
		t.Fatalf("movements=%#v err=%v", movements, err)
	}
	nextMovements, err := service.ListMovements(ctx, MovementFilters{From: movements.PeriodFrom, To: movements.PeriodTo, ActorUserID: user.ID, Limit: 1, Cursor: movements.NextCursor})
	if err != nil || len(nextMovements.Items) != 1 || nextMovements.Items[0].ID == movements.Items[0].ID {
		t.Fatalf("next movements=%#v err=%v", nextMovements, err)
	}
	archived, err := service.SetArchived(ctx, actor, card.ID, movedAgain.Version, true)
	if err != nil || !archived.IsArchived {
		t.Fatalf("archived=%#v err=%v", archived, err)
	}
	activity, err := service.Activity(ctx, card.ID, 20)
	if err != nil || len(activity) != 5 {
		t.Fatalf("activity=%#v err=%v", activity, err)
	}
}
