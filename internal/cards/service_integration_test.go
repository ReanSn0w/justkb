package cards

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/ReanSn0w/justkb/internal/auth"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCardLifecycleIntegration(t *testing.T) {
	url := os.Getenv("JKB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("JKB_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `TRUNCATE card_activity,api_tokens,web_sessions,attachments,card_tags,cards,group_members,users,user_groups,epics,tags RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	user, err := auth.NewService(db).CreateAdmin(ctx, "admin", "Admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	var firstColumn, secondColumn string
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position LIMIT 1`).Scan(&firstColumn); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position OFFSET 1 LIMIT 1`).Scan(&secondColumn); err != nil {
		t.Fatal(err)
	}
	var tagID string
	if err := db.QueryRowContext(ctx, `INSERT INTO tags(name,color) VALUES('bug','#CC0000') RETURNING id`).Scan(&tagID); err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	actor := Actor{UserID: user.ID}
	card, err := service.Create(ctx, actor, CreateInput{Title: "Fix login", ColumnID: firstColumn, TagIDs: []string{tagID}})
	if err != nil {
		t.Fatal(err)
	}
	if card.Number != 1 || len(card.TagIDs) != 1 || card.Version != 1 {
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
	archived, err := service.SetArchived(ctx, actor, card.ID, moved.Version, true)
	if err != nil || !archived.IsArchived {
		t.Fatalf("archived=%#v err=%v", archived, err)
	}
	activity, err := service.Activity(ctx, card.ID, 20)
	if err != nil || len(activity) != 4 {
		t.Fatalf("activity=%#v err=%v", activity, err)
	}
}
