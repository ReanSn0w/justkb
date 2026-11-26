package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/testdb"
	"github.com/ReanSn0w/justkb/internal/tokens"
)

func TestBotAdminAPI(t *testing.T) {
	f := testdb.Open(t)
	ctx := context.Background()
	a := auth.NewService(f.DB)
	admin, err := a.CreateAdmin(ctx, "admin", "Admin", "test-password-long")
	if err != nil {
		t.Fatal(err)
	}
	_, err = board.NewService(f.DB).CreateUser(ctx, "member", "Member", "", "member", "test-password-long")
	if err != nil {
		t.Fatal(err)
	}
	session, err := a.Login(ctx, "admin", "test-password-long", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	member, err := a.Login(ctx, "member", "test-password-long", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), StoragePath: t.TempDir(), Database: f.DB, DB: f.DB, DatabasePath: f.Path, AttachmentMaxSize: 1024})
	cookie := &http.Cookie{Name: sessionCookieName, Value: session.Token}
	mc := &http.Cookie{Name: sessionCookieName, Value: member.Token}
	if r := request(h, "POST", "/api/v1/bots", `{"display_name":"No"}`, mc, member.CSRF); r.Code != 403 {
		t.Fatal("member created bot")
	}
	if r := request(h, "POST", "/api/v1/bots", `{"display_name":"No"}`, cookie, ""); r.Code != 403 {
		t.Fatal("missing CSRF accepted")
	}
	r := request(h, "POST", "/api/v1/bots", `{"display_name":"Agent"}`, cookie, session.CSRF)
	var created tokens.CreatedBot
	if r.Code != 201 || json.Unmarshal(r.Body.Bytes(), &created) != nil || created.UserID == "" || created.Token.Secret == "" {
		t.Fatal("bot creation failed")
	}
	if r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("secret response cacheable")
	}
	for _, path := range []string{"/api/v1/users?include_inactive=true", "/api/v1/bots/" + created.UserID + "/token"} {
		r = request(h, "GET", path, "", cookie, "")
		if r.Code != 200 || strings.Contains(r.Body.String(), created.Token.Secret) {
			t.Fatal("metadata failed or leaked secret")
		}
	}
	for _, methodPath := range [][2]string{{"GET", "/token"}, {"POST", "/token/rotate"}, {"POST", "/avatar"}, {"DELETE", "/avatar"}} {
		r = request(h, methodPath[0], "/api/v1/bots/"+created.UserID+methodPath[1], `{}`, mc, member.CSRF)
		if r.Code != 403 {
			t.Fatalf("member access %v: %d", methodPath, r.Code)
		}
	}
	if r = request(h, "PUT", "/api/v1/users/"+created.UserID+"/password", `{"password":"new-test-password"}`, cookie, session.CSRF); r.Code != 403 {
		t.Fatal("bot password reset allowed")
	}
	if r = request(h, "PATCH", "/api/v1/users/"+created.UserID, `{"display_name":"Agent","role":"admin","is_active":true}`, cookie, session.CSRF); r.Code != 403 {
		t.Fatal("bot promotion allowed")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "test.png")
	part.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	writer.Close()
	req := httptest.NewRequest("POST", "/api/v1/bots/"+created.UserID+"/avatar", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", session.CSRF)
	req.AddCookie(cookie)
	uploaded := httptest.NewRecorder()
	h.ServeHTTP(uploaded, req)
	if uploaded.Code != 204 {
		t.Fatalf("avatar status=%d", uploaded.Code)
	}
	if r = request(h, "GET", "/api/v1/users/"+created.UserID+"/avatar", "", cookie, ""); r.Code != 200 {
		t.Fatal("avatar missing")
	}
	if r = request(h, "DELETE", "/api/v1/bots/"+created.UserID+"/avatar", "", cookie, session.CSRF); r.Code != 204 {
		t.Fatal("delete avatar failed")
	}
	if r = request(h, "DELETE", "/api/v1/bots/"+admin.ID+"/avatar", "", cookie, session.CSRF); r.Code != 404 {
		t.Fatal("bot endpoint modified human")
	}
	payload := `{"expected_token_id":"` + created.Token.ID + `"}`
	r = request(h, "POST", "/api/v1/bots/"+created.UserID+"/token/rotate", payload, cookie, session.CSRF)
	var rotated tokens.Created
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &rotated) != nil || rotated.Secret == "" {
		t.Fatal("rotation failed")
	}
	if _, err = tokens.NewService(f.DB).Authenticate(ctx, created.Token.Secret); err == nil {
		t.Fatal("old token accepted")
	}
	if r = request(h, "POST", "/api/v1/bots/"+created.UserID+"/token/rotate", payload, cookie, session.CSRF); r.Code != 409 || !strings.Contains(r.Body.String(), "bot_token_conflict") {
		t.Fatal("rotation conflict missing")
	}
}
