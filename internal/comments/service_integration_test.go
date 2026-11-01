package comments

import (
	"context"
	"testing"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/cards"
	"github.com/ReanSn0w/justkb/internal/testdb"
)

func TestCommentLifecycleIntegration(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	user, err := auth.NewService(db).CreateAdmin(ctx, "commenter", "Commenter", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	var columnID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position LIMIT 1`).Scan(&columnID); err != nil {
		t.Fatal(err)
	}
	card, err := cards.NewService(db).Create(ctx, cards.Actor{UserID: user.ID}, cards.CreateInput{Title: "Discuss", ColumnID: columnID})
	if err != nil {
		t.Fatal(err)
	}
	epic, err := board.NewService(db).SaveEpic(ctx, "", "Discuss project", "#00d1b2", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	cardID, epicID := card.ID, epic.ID
	service := NewService(db)
	cardComment, err := service.CreateCard(ctx, cardID, Actor{UserID: user.ID}, "**card** comment")
	if err != nil || cardComment.Author.DisplayName != "Commenter" {
		t.Fatalf("card comment=%#v err=%v", cardComment, err)
	}
	projectComment, err := service.CreateEpic(ctx, epicID, Actor{UserID: user.ID}, "project comment")
	if err != nil {
		t.Fatal(err)
	}
	cards, err := service.ListCard(ctx, cardID)
	if err != nil || len(cards) != 1 || cards[0].ID != cardComment.ID {
		t.Fatalf("card comments=%#v err=%v", cards, err)
	}
	projects, err := service.ListEpic(ctx, epicID)
	if err != nil || len(projects) != 1 || projects[0].ID != projectComment.ID {
		t.Fatalf("project comments=%#v err=%v", projects, err)
	}
}
