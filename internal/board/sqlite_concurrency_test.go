package board

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	appdb "github.com/ReanSn0w/justkb/internal/database"
	"github.com/pressly/goose/v3"
)

func TestConcurrentAdminDemotionKeepsOneActiveAdmin(t *testing.T) {
	db, err := appdb.Open(context.Background(), filepath.Join(t.TempDir(), "jkb.sqlite"))
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
	service := NewService(db)
	first, err := service.CreateUser(context.Background(), "first-admin", "First", "", "admin", "test-password-long-enough")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateUser(context.Background(), "second-admin", "Second", "", "admin", "test-password-long-enough")
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errorsByUser := make(chan error, 2)
	var wait sync.WaitGroup
	for _, user := range []User{first, second} {
		wait.Add(1)
		go func(user User) {
			defer wait.Done()
			<-start
			_, err := service.UpdateUser(context.Background(), user.ID, user.DisplayName, "", "member", true)
			errorsByUser <- err
		}(user)
	}
	close(start)
	wait.Wait()
	close(errorsByUser)
	succeeded, protected := 0, 0
	for err := range errorsByUser {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrLastAdmin):
			protected++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || protected != 1 {
		t.Fatalf("succeeded=%d protected=%d", succeeded, protected)
	}
}
