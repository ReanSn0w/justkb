package board

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/testdb"
)

func TestReferenceDataIntegration(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	service := NewService(db)
	group, err := service.SaveGroup(ctx, "", "Backend", "#3366FF", nil)
	if err != nil {
		t.Fatal(err)
	}
	user, err := service.CreateUser(ctx, "developer", "Developer", "", "member", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ResetUserPasswordByUsername(ctx, "DEVELOPER", "another correct password"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.NewService(db).Login(ctx, "developer", "another correct password", time.Hour); err != nil {
		t.Fatalf("login after password reset: %v", err)
	}
	admin, err := service.CreateUser(ctx, "admin", "Administrator", "", "admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetUserActiveByUsername(ctx, admin.Username, false); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable last admin error=%v", err)
	}
	if _, err := service.CreateUser(ctx, "backup-admin", "Backup Administrator", "", "admin", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	disabled, err := service.SetUserActiveByUsername(ctx, admin.Username, false)
	if err != nil || disabled.IsActive {
		t.Fatalf("disabled admin=%#v err=%v", disabled, err)
	}
	loaded, err := service.UserByUsername(ctx, "ADMIN")
	if err != nil || loaded.ID != admin.ID || loaded.IsActive {
		t.Fatalf("loaded admin=%#v err=%v", loaded, err)
	}
	if err := service.SetUserGroups(ctx, user.ID, []string{group.ID}); err != nil {
		t.Fatal(err)
	}
	users, err := service.ListUsers(ctx, false, group.ID)
	if err != nil || len(users) != 1 || len(users[0].GroupIDs) != 1 {
		t.Fatalf("users = %#v, err = %v", users, err)
	}
	epic, err := service.SaveEpic(ctx, "", "Platform", "#AA5500", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetEpic(ctx, epic.ID)
	if err != nil || detail.Name != "Platform" || detail.CardCount != 0 {
		t.Fatalf("epic detail = %#v, err = %v", detail, err)
	}
	if _, err := service.SaveEpic(ctx, epic.ID, epic.Name, epic.Color, nil, true, epic.Version); err != nil {
		t.Fatal(err)
	}
	epics, err := service.ListEpics(ctx, false)
	if err != nil || len(epics) != 0 {
		t.Fatalf("active epics = %#v, err = %v", epics, err)
	}
	if _, err := service.SaveTag(ctx, "", "bug", "bad-color"); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid color error = %v", err)
	}
	columns, err := service.ListColumns(ctx)
	if err != nil || len(columns) < 2 {
		t.Fatalf("columns = %#v, err = %v", columns, err)
	}
	ids := make([]string, len(columns))
	for index := range columns {
		ids[len(columns)-1-index] = columns[index].ID
	}
	if err := service.ReorderColumns(ctx, ids); err != nil {
		t.Fatal(err)
	}
	reordered, _ := service.ListColumns(ctx)
	if reordered[0].ID != ids[0] {
		t.Fatalf("first column = %s, want %s", reordered[0].ID, ids[0])
	}
}

func TestEpicProgressCountsOnlyActiveCardsInLastColumn(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	service := NewService(db)
	user, err := service.CreateUser(ctx, "reporter", "Reporter", "", "member", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	epic, err := service.SaveEpic(ctx, "", "Delivery", "#00D1B2", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	columns, err := service.ListColumns(ctx)
	if err != nil || len(columns) < 2 {
		t.Fatalf("columns=%#v err=%v", columns, err)
	}
	firstColumn := columns[0].ID
	lastColumn := columns[len(columns)-1].ID
	for _, card := range []struct {
		id       string
		number   int
		columnID string
		archived bool
	}{
		{id: "card-backlog", number: 1, columnID: firstColumn},
		{id: "card-done", number: 2, columnID: lastColumn},
		{id: "card-archived", number: 3, columnID: lastColumn, archived: true},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO cards (id,number,title,column_id,epic_id,position,is_archived,created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			card.id, card.number, card.id, card.columnID, epic.ID, card.number*1000, card.archived, user.ID,
		); err != nil {
			t.Fatal(err)
		}
	}

	items, err := service.ListEpics(ctx, false)
	if err != nil || len(items) != 1 {
		t.Fatalf("epics=%#v err=%v", items, err)
	}
	if items[0].CardCount != 2 || items[0].CompletedCardCount != 1 {
		t.Fatalf("epic progress=%#v", items[0])
	}
	detail, err := service.GetEpic(ctx, epic.ID)
	if err != nil || detail.CardCount != 2 || detail.CompletedCardCount != 1 {
		t.Fatalf("epic detail=%#v err=%v", detail, err)
	}
}

func TestEpicRejectsStaleUpdate(t *testing.T) {
	service := NewService(testdb.Open(t).DB)
	ctx := context.Background()
	epic, err := service.SaveEpic(ctx, "", "Versioned", "#112233", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SaveEpic(ctx, epic.ID, "Updated", epic.Color, nil, false, epic.Version); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SaveEpic(ctx, epic.ID, "Lost", epic.Color, nil, false, epic.Version); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale update: %v", err)
	}
	current, err := service.GetEpic(ctx, epic.ID)
	if err != nil || current.Name != "Updated" || current.Version != 2 {
		t.Fatalf("current=%+v err=%v", current, err)
	}
}
