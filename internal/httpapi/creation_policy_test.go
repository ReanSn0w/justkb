package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/testdb"
)

func TestRESTCreationPolicy(t *testing.T) {
	fixture := testdb.Open(t)
	ctx := context.Background()
	a := auth.NewService(fixture.DB)
	_, err := a.CreateAdmin(ctx, "admin", "Admin", "test-password-long")
	if err != nil {
		t.Fatal(err)
	}
	_, err = board.NewService(fixture.DB).CreateUser(ctx, "member", "Member", "", "member", "test-password-long")
	if err != nil {
		t.Fatal(err)
	}
	session, err := a.Login(ctx, "admin", "test-password-long", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), StoragePath: t.TempDir(), Database: fixture.DB, DB: fixture.DB, AttachmentMaxSize: 1024, DatabasePath: fixture.Path})
	cookie := &http.Cookie{Name: sessionCookieName, Value: session.Token}
	created := request(h, "POST", "/api/v1/columns", `{"name":"Restricted"}`, cookie, session.CSRF)
	var col board.Column
	if created.Code != 201 || json.Unmarshal(created.Body.Bytes(), &col) != nil || col.AllowCardCreation {
		t.Fatalf("create: %s", created.Body.String())
	}
	for _, name := range []string{"admin", "member"} {
		login, err := a.Login(ctx, name, "test-password-long", time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		response := request(h, "POST", "/api/v1/cards", `{"title":"Rejected","column_id":"`+col.ID+`"}`, &http.Cookie{Name: sessionCookieName, Value: login.Token}, login.CSRF)
		if response.Code != 409 || !strings.Contains(response.Body.String(), "card_creation_not_allowed") || !strings.Contains(response.Body.String(), col.ID) {
			t.Fatalf("%s: %s", name, response.Body.String())
		}
	}
	for _, body := range []string{`{"name":"Restricted","allow_card_creation":null}`, `{"name":"Restricted","allow_card_creation":"false"}`} {
		if r := request(h, "PATCH", "/api/v1/columns/"+col.ID, body, cookie, session.CSRF); r.Code != 400 {
			t.Fatalf("bad boolean status=%d", r.Code)
		}
	}
	for _, body := range []string{`{"name":"Restricted","allow_card_creation":true}`, `{"name":"Renamed"}`} {
		r := request(h, "PATCH", "/api/v1/columns/"+col.ID, body, cookie, session.CSRF)
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &col) != nil || !col.AllowCardCreation {
			t.Fatalf("patch: %s", r.Body.String())
		}
	}
	if r := request(h, "POST", "/api/v1/cards", `{"title":"Accepted","column_id":"`+col.ID+`"}`, cookie, session.CSRF); r.Code != 201 {
		t.Fatalf("allowed: %s", r.Body.String())
	}
}
