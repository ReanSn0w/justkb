package cards

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ReanSn0w/justkb/internal/auth"
	appdb "github.com/ReanSn0w/justkb/internal/database"
	"github.com/pressly/goose/v3"
)

func TestConcurrentMovesPreserveOptimisticLock(t *testing.T) {
	ctx := context.Background()
	db, err := appdb.Open(ctx, filepath.Join(t.TempDir(), "jkb.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	user, err := auth.NewService(db).CreateAdmin(ctx, "admin", "Administrator", "test-password-long-enough")
	if err != nil {
		t.Fatal(err)
	}
	var sourceColumn, firstTarget, secondTarget string
	rows, err := db.Query(`SELECT id FROM board_columns ORDER BY position LIMIT 3`)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []*string{&sourceColumn, &firstTarget, &secondTarget} {
		if !rows.Next() || rows.Scan(target) != nil {
			t.Fatal("initial columns are missing")
		}
	}
	rows.Close()
	service := NewService(db)
	card, err := service.Create(ctx, Actor{UserID: user.ID}, CreateInput{Title: "Concurrent move", ColumnID: sourceColumn})
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, columnID := range []string{firstTarget, secondTarget} {
		wait.Add(1)
		go func(columnID string) {
			defer wait.Done()
			<-start
			_, err := service.Move(ctx, Actor{UserID: user.ID}, card.ID, MoveInput{ExpectedVersion: card.Version, TargetColumnID: columnID})
			results <- err
		}(columnID)
	}
	close(start)
	wait.Wait()
	close(results)
	succeeded, conflicted := 0, 0
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrVersionConflict):
			conflicted++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("succeeded=%d conflicted=%d", succeeded, conflicted)
	}
}
