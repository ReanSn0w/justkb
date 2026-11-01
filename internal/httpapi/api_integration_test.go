package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ReanSn0w/justkb/internal/auth"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRESTContractIntegration(t *testing.T) {
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
	if _, err := auth.NewService(db).CreateAdmin(ctx, "admin", "Admin", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	var columnID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position LIMIT 1`).Scan(&columnID); err != nil {
		t.Fatal(err)
	}
	handler := New(Dependencies{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), StoragePath: t.TempDir(), Database: db, DB: db,
		AttachmentMaxSize: 1024, CookieSecure: false,
	})

	unauthorized := request(handler, http.MethodGet, "/api/v1/cards", "", nil, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}
	login := request(handler, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"correct horse battery staple"}`, nil, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", login.Code, login.Body.String())
	}
	var loginPayload struct {
		CSRF string `json:"csrf_token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginPayload); err != nil {
		t.Fatal(err)
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || loginPayload.CSRF == "" {
		t.Fatal("login did not return session and CSRF")
	}

	unknown := request(handler, http.MethodPost, "/api/v1/cards", `{"title":"Bad","column_id":"`+columnID+`","unknown":true}`, cookies[0], loginPayload.CSRF)
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d", unknown.Code)
	}
	created := request(handler, http.MethodPost, "/api/v1/cards", `{"title":"REST card","column_id":"`+columnID+`"}`, cookies[0], loginPayload.CSRF)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var card struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &card); err != nil {
		t.Fatal(err)
	}
	conflict := request(handler, http.MethodPatch, "/api/v1/cards/"+card.ID, `{"expected_version":99,"title":"Conflict"}`, cookies[0], loginPayload.CSRF)
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "card_version_conflict") {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, "hello attachment")
	_ = writer.Close()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/v1/cards/"+card.ID+"/attachments", &body)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRequest.Header.Set("X-CSRF-Token", loginPayload.CSRF)
	uploadRequest.AddCookie(cookies[0])
	upload := httptest.NewRecorder()
	handler.ServeHTTP(upload, uploadRequest)
	if upload.Code != http.StatusCreated {
		t.Fatalf("upload status=%d body=%s", upload.Code, upload.Body.String())
	}

	createdEpic := request(handler, http.MethodPost, "/api/v1/epics", `{"name":"REST EPIC","description":"Project brief","color":"#00d1b2"}`, cookies[0], loginPayload.CSRF)
	if createdEpic.Code != http.StatusCreated {
		t.Fatalf("create epic status=%d body=%s", createdEpic.Code, createdEpic.Body.String())
	}
	var epic struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createdEpic.Body.Bytes(), &epic); err != nil {
		t.Fatal(err)
	}
	body.Reset()
	writer = multipart.NewWriter(&body)
	part, err = writer.CreateFormFile("file", "brief.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, "epic attachment")
	_ = writer.Close()
	epicUploadRequest := httptest.NewRequest(http.MethodPost, "/api/v1/epics/"+epic.ID+"/attachments", &body)
	epicUploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	epicUploadRequest.Header.Set("X-CSRF-Token", loginPayload.CSRF)
	epicUploadRequest.AddCookie(cookies[0])
	epicUpload := httptest.NewRecorder()
	handler.ServeHTTP(epicUpload, epicUploadRequest)
	if epicUpload.Code != http.StatusCreated {
		t.Fatalf("epic upload status=%d body=%s", epicUpload.Code, epicUpload.Body.String())
	}
	detail := request(handler, http.MethodGet, "/api/v1/epics/"+epic.ID, "", cookies[0], "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "brief.txt") {
		t.Fatalf("epic detail status=%d body=%s", detail.Code, detail.Body.String())
	}
}

func request(handler http.Handler, method, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}
