package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/ReanSn0w/justkb/internal/archive"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/testdb"
)

func TestArchivePolicyPermissionsAndConflicts(t *testing.T) {
	database := testdb.Open(t)
	ctx := context.Background()
	b := board.NewService(database.DB)
	for _, role := range []string{"admin", "member"} {
		if _, err := b.CreateUser(ctx, role, role, "", role, "correct horse battery staple"); err != nil {
			t.Fatal(err)
		}
	}
	handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), DB: database.DB, Database: database.DB, DatabasePath: database.Path, StoragePath: t.TempDir(), AttachmentMaxSize: 1024})
	for _, role := range []string{"admin", "member"} {
		t.Run(role, func(t *testing.T) {
			login := request(handler, "POST", "/api/v1/auth/login", `{"username":"`+role+`","password":"correct horse battery staple"}`, nil, "")
			var session struct {
				CSRF string `json:"csrf_token"`
			}
			if err := json.Unmarshal(login.Body.Bytes(), &session); err != nil {
				t.Fatal(err)
			}
			cookie := login.Result().Cookies()[0]
			read := request(handler, "GET", "/api/v1/archive/policy", "", cookie, "")
			if read.Code != 200 {
				t.Fatal(read.Body.String())
			}
			var p archive.Policy
			if err := json.Unmarshal(read.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(policyInput{Policy: p, ExpectedVersion: p.Version})
			response := request(handler, "PUT", "/api/v1/archive/policy", string(body), cookie, session.CSRF)
			want := http.StatusOK
			if role == "member" {
				want = http.StatusForbidden
			}
			if response.Code != want {
				t.Fatalf("save=%d %s", response.Code, response.Body.String())
			}
			if role == "admin" {
				response = request(handler, "PUT", "/api/v1/archive/policy", string(body), cookie, session.CSRF)
				if response.Code != 409 {
					t.Fatalf("stale save=%d", response.Code)
				}
			}
			response = request(handler, "GET", "/api/v1/archive?cursor=not-a-cursor", "", cookie, "")
			if response.Code != 422 {
				t.Fatalf("invalid cursor=%d %s", response.Code, response.Body.String())
			}
			response = request(handler, "GET", "/api/v1/archive", "", cookie, "")
			if response.Code != 200 {
				t.Fatalf("archive read=%d", response.Code)
			}
		})
	}
}
