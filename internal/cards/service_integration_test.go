package cards

import (
	"context"
	"errors"
	"slices"
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
	group, err := board.NewService(db).SaveGroup(ctx, "", "Backend", "#3366FF", nil)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	actor := Actor{UserID: user.ID}
	card, err := service.Create(ctx, actor, CreateInput{Title: "Fix login", ColumnID: firstColumn, GroupID: group.ID, TagIDs: []string{tagID}})
	if err != nil {
		t.Fatal(err)
	}
	if card.Number != 1 || len(card.TagIDs) != 1 || len(card.Tags) != 1 || card.Tags[0].Name != "bug" || card.Group == nil || card.Group.Color != "#3366FF" || card.Version != 1 {
		t.Fatalf("card = %#v", card)
	}
	second, err := service.Create(ctx, actor, CreateInput{Title: "Second tagged card", ColumnID: firstColumn, TagIDs: []string{tagID}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.List(ctx, Filters{TagIDs: []string{tagID}, Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != second.ID || page.NextCursor == "" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	nextPage, err := service.List(ctx, Filters{TagIDs: []string{tagID}, Limit: 1, Cursor: page.NextCursor})
	if err != nil || len(nextPage.Items) != 1 || nextPage.Items[0].ID != card.ID {
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

func TestCardOrderingIntegration(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	user, err := auth.NewService(db).CreateAdmin(ctx, "order-admin", "Order Admin", "correct horse battery staple")
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
	service := NewService(db)
	actor := Actor{UserID: user.ID}
	first, err := service.Create(ctx, actor, CreateInput{Title: "First", ColumnID: firstColumn})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, actor, CreateInput{Title: "Second", ColumnID: firstColumn})
	if err != nil {
		t.Fatal(err)
	}
	third, err := service.Create(ctx, actor, CreateInput{Title: "Third", ColumnID: firstColumn})
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.List(ctx, Filters{ColumnID: firstColumn, Limit: 2})
	if err != nil || len(page.Items) != 2 || page.Items[0].ID != third.ID || page.Items[1].ID != second.ID || page.NextCursor == "" {
		t.Fatalf("first page=%#v err=%v", page, err)
	}
	nextPage, err := service.List(ctx, Filters{ColumnID: firstColumn, Limit: 2, Cursor: page.NextCursor})
	if err != nil || len(nextPage.Items) != 1 || nextPage.Items[0].ID != first.ID {
		t.Fatalf("next page=%#v err=%v", nextPage, err)
	}

	moved, err := service.Move(ctx, actor, first.ID, MoveInput{
		ExpectedVersion: first.Version,
		TargetColumnID:  secondColumn,
	})
	if err != nil || moved.ColumnID != secondColumn {
		t.Fatalf("move between columns=%#v err=%v", moved, err)
	}
	moved, err = service.Move(ctx, actor, first.ID, MoveInput{
		ExpectedVersion: moved.Version,
		TargetColumnID:  firstColumn,
		BeforeCardID:    third.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	moved, err = service.Move(ctx, actor, first.ID, MoveInput{
		ExpectedVersion: moved.Version,
		TargetColumnID:  firstColumn,
		AfterCardID:     second.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	ordered, err := service.List(ctx, Filters{ColumnID: firstColumn, Limit: 10})
	if err != nil || len(ordered.Items) != 3 || ordered.Items[0].ID != third.ID || ordered.Items[1].ID != second.ID || ordered.Items[2].ID != first.ID {
		t.Fatalf("ordered=%#v err=%v", ordered, err)
	}
}

func TestMultipleAssigneesKeepLegacyWritesAndFiltersCompatible(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO users(id,username,display_name,password_hash,is_active)
		VALUES ('first','first','First','unused',0), ('second','second','Second','unused',1),
		       ('third','third','Third','unused',1)`); err != nil {
		t.Fatal(err)
	}
	var columnID string
	if err := db.QueryRow(`SELECT id FROM board_columns ORDER BY position LIMIT 1`).Scan(&columnID); err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	actor := Actor{UserID: "second"}
	card, err := service.Create(ctx, actor, CreateInput{Title: "Existing", ColumnID: columnID, AssigneeID: "first"})
	if err != nil || !slices.Equal(card.AssigneeIDs, []string{"first"}) {
		t.Fatalf("legacy create: %#v, %v", card, err)
	}
	card, err = service.Update(ctx, actor, card.ID, UpdateInput{ExpectedVersion: card.Version, AssigneeIDs: []string{"first", "second", "third"}})
	if err != nil || !slices.Equal(card.AssigneeIDs, []string{"first", "second", "third"}) || card.Assignee == nil || card.Assignee.ID != "first" {
		t.Fatalf("set assignees: %#v, %v", card, err)
	}
	for _, filter := range []string{"second", "me"} {
		page, err := service.List(ctx, Filters{AssigneeID: filter, CurrentUserID: "second"})
		if err != nil || len(page.Items) != 1 || page.Items[0].ID != card.ID {
			t.Fatalf("filter %s: %#v, %v", filter, page, err)
		}
	}
	title := "Preserve all assignees"
	card, err = service.Update(ctx, actor, card.ID, UpdateInput{ExpectedVersion: card.Version, Title: &title})
	if err != nil || !slices.Equal(card.AssigneeIDs, []string{"first", "second", "third"}) {
		t.Fatalf("unrelated update: %#v, %v", card, err)
	}
	for _, ids := range [][]string{{"missing"}, {"second", "second"}} {
		if _, err := service.Update(ctx, actor, card.ID, UpdateInput{ExpectedVersion: card.Version, AssigneeIDs: ids}); !errors.Is(err, ErrValidation) {
			t.Fatalf("invalid assignees: %v", err)
		}
	}
	clearPrimary := ""
	card, err = service.Update(ctx, actor, card.ID, UpdateInput{ExpectedVersion: card.Version, AssigneeID: &clearPrimary})
	if err != nil || !slices.Equal(card.AssigneeIDs, []string{"second", "third"}) || card.Assignee == nil || card.Assignee.ID != "second" {
		t.Fatalf("legacy clear: %#v, %v", card, err)
	}
	if _, err := service.Update(ctx, actor, card.ID, UpdateInput{ExpectedVersion: card.Version - 1, AssigneeIDs: []string{}}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale clear: %v", err)
	}
	card, err = service.Update(ctx, actor, card.ID, UpdateInput{ExpectedVersion: card.Version, AssigneeIDs: []string{}})
	if err != nil || len(card.AssigneeIDs) != 0 || card.Assignee != nil {
		t.Fatalf("clear all: %#v, %v", card, err)
	}
	page, err := service.List(ctx, Filters{AssigneeID: "none"})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != card.ID {
		t.Fatalf("filter none: %#v, %v", page, err)
	}
}
