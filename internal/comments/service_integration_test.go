package comments

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/ReanSn0w/justkb/internal/auth"
)

func TestCommentLifecycleIntegration(t *testing.T) {
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
	user, err := auth.NewService(db).CreateAdmin(ctx, "commenter", "Commenter", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	var columnID, cardID, epicID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position LIMIT 1`).Scan(&columnID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO cards(title,column_id,position,created_by) VALUES('Discuss',$1,1000,$2) RETURNING id`, columnID, user.ID).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO epics(name,color) VALUES('Discuss project','#00d1b2') RETURNING id`).Scan(&epicID); err != nil {
		t.Fatal(err)
	}
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
