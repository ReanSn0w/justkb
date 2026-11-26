package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/testdb"
	"github.com/ReanSn0w/justkb/internal/tokens"
	"github.com/ReanSn0w/justkb/internal/updates"
)

func TestProjectPollingAsMember(t *testing.T) {
	database := testdb.Open(t)
	db := database.DB
	ctx := context.Background()
	user, err := auth.NewService(db).CreateAdmin(ctx, "reader", "Reader", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE users SET role='member' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	h := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), DB: db, Database: db, DatabasePath: database.Path, StoragePath: t.TempDir(), AttachmentMaxSize: 1024})
	ts := tokens.NewService(db)
	read, err := ts.Create(ctx, user.ID, "reader", []string{"cards:read"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	write, err := ts.Create(ctx, user.ID, "writer", []string{"cards:write"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := ts.Create(ctx, user.ID, "revoked", []string{"cards:read"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = ts.Revoke(ctx, user.ID, revoked.ID); err != nil {
		t.Fatal(err)
	}
	expired, err := ts.Create(ctx, user.ID, "expired", []string{"cards:read"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE api_tokens SET expires_at=? WHERE id=?`, time.Now().Add(-time.Hour), expired.ID); err != nil {
		t.Fatal(err)
	}
	login := request(h, "POST", "/api/v1/auth/login", `{"username":"reader","password":"correct horse battery staple"}`, nil, "")
	if login.Code != 200 {
		t.Fatalf("login: %d", login.Code)
	}
	cookie := login.Result().Cookies()[0]
	var payload struct {
		CSRF string `json:"csrf_token"`
	}
	if err = json.Unmarshal(login.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	call := func(path, token string, withCookie bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", path, nil)
		if token != "" {
			r.Header.Set("Authorization", token)
		}
		if withCookie {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/api/v1/epics", "/api/v1/epics/missing/updates"} {
		for _, tc := range []struct {
			token  string
			cookie bool
			code   int
		}{{"", false, 401}, {"Bearer invalid", true, 401}, {"Basic abc", true, 401}, {"Bearer " + write.Secret, false, 403}, {"Bearer " + revoked.Secret, false, 401}, {"Bearer " + expired.Secret, false, 401}} {
			w := call(path, tc.token, tc.cookie)
			if w.Code != tc.code {
				t.Fatalf("auth %s: %d expected %d", path, w.Code, tc.code)
			}
		}
	}
	// A real session creates a project and a comment. The read-only token sees both.
	created := request(h, "POST", "/api/v1/epics", `{"name":"Watch","color":"#112233"}`, cookie, payload.CSRF)
	if created.Code != 201 {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var project board.Epic
	if err = json.Unmarshal(created.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	comment := request(h, "POST", "/api/v1/epics/"+project.ID+"/comments", `{"body":"Ready to start"}`, cookie, payload.CSRF)
	if comment.Code != 201 {
		t.Fatalf("comment: %d %s", comment.Code, comment.Body.String())
	}
	path := "/api/v1/epics/" + project.ID + "/updates"
	bearer := "Bearer " + read.Secret
	first := call(path+"?limit=1", bearer, false)
	var p updates.Page
	if first.Code != 200 || json.Unmarshal(first.Body.Bytes(), &p) != nil || len(p.Items) != 1 || !p.HasMore || p.Items[0].Type != "project.created" {
		t.Fatalf("first: %d %s", first.Code, first.Body.String())
	}
	second := call(path+"?limit=1&cursor="+url.QueryEscape(p.NextCursor), bearer, false)
	if second.Code != 200 || json.Unmarshal(second.Body.Bytes(), &p) != nil || len(p.Items) != 1 || p.HasMore || p.Items[0].Type != "comment.created" || p.Items[0].ActorUserID == nil || *p.Items[0].ActorUserID != user.ID {
		t.Fatalf("second: %s", second.Body.String())
	}
	saved := p.NextCursor
	for range 2 {
		w := call(path+"?cursor="+url.QueryEscape(saved), bearer, false)
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &p) != nil || len(p.Items) != 0 || p.NextCursor == "" {
			t.Fatalf("empty/restart: %s", w.Body.String())
		}
	}
	for _, query := range []string{"?cursor=bad", "?limit=0", "?limit=-1", "?limit=501", "?limit=x", "?limit=", "?limit=1&limit=2", "?cursor=now&cursor=bad"} {
		if w := call(path+query, bearer, false); w.Code != 400 {
			t.Fatalf("validation %s: %d", query, w.Code)
		}
	}
	if w := call(path+"?cursor=now", bearer, false); w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("now: %s", w.Body.String())
	}
	if w := call(path, "", true); w.Code != 200 {
		t.Fatalf("session: %d", w.Code)
	}
	if w := call("/api/v1/epics/missing/updates", bearer, false); w.Code != 404 {
		t.Fatalf("missing: %d", w.Code)
	}
	// The token must not gain access to unrelated REST endpoints.
	if w := call("/api/v1/cards", bearer, false); w.Code != 401 {
		t.Fatalf("expanded access: %d", w.Code)
	}
	if _, err = db.Exec(`UPDATE epics SET is_archived=1 WHERE id=?`, project.ID); err != nil {
		t.Fatal(err)
	}
	if w := call("/api/v1/epics", bearer, false); w.Code != 200 || strings.Contains(w.Body.String(), project.ID) {
		t.Fatalf("active: %s", w.Body.String())
	}
	if w := call("/api/v1/epics?include_archived=true", bearer, false); w.Code != 200 || !strings.Contains(w.Body.String(), project.ID) {
		t.Fatalf("all: %s", w.Body.String())
	}
	if w := call(path, bearer, false); w.Code != 200 {
		t.Fatalf("archived feed: %d", w.Code)
	}
	if _, err = db.Exec(`UPDATE users SET is_active=0 WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	if w := call(path, bearer, false); w.Code != 401 {
		t.Fatalf("disabled: %d", w.Code)
	}
}
