package mcpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/tokens"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerTransport struct {
	secret string
	base   http.RoundTripper
}

func (t bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+t.secret)
	return t.base.RoundTrip(clone)
}

func TestMCPIntegration(t *testing.T) {
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
	user, err := auth.NewService(db).CreateAdmin(ctx, "admin", "Admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	var groupID string
	if err := db.QueryRowContext(ctx, `INSERT INTO user_groups(name,color) VALUES('Backend','#00d1b2') RETURNING id`).Scan(&groupID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO group_members(group_id,user_id) VALUES($1,$2)`, groupID, user.ID); err != nil {
		t.Fatal(err)
	}
	created, err := tokens.NewService(db).Create(ctx, user.ID, "mcp test", []string{"cards:read", "cards:write"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var columnID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position LIMIT 1`).Scan(&columnID); err != nil {
		t.Fatal(err)
	}

	httpServer := httptest.NewServer(NewHandler(db))
	defer httpServer.Close()
	response, err := http.Post(httpServer.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing bearer status=%d", response.StatusCode)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "jkb-test", Version: "1.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             httpServer.URL,
		HTTPClient:           &http.Client{Transport: bearerTransport{secret: created.Secret, base: http.DefaultTransport}},
		DisableStandaloneSSE: true,
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 21 {
		t.Fatalf("tool count=%d", len(listed.Tools))
	}
	currentUser, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_current_user", Arguments: map[string]any{}})
	if err != nil || currentUser.IsError {
		t.Fatalf("get_current_user result=%#v err=%v", currentUser, err)
	}
	currentUserJSON, err := json.Marshal(currentUser.StructuredContent)
	if err != nil || !strings.Contains(string(currentUserJSON), user.ID) || !strings.Contains(string(currentUserJSON), groupID) || !strings.Contains(string(currentUserJSON), created.ID) {
		t.Fatalf("get_current_user content=%s err=%v", currentUserJSON, err)
	}
	columns, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_columns", Arguments: map[string]any{}})
	if err != nil || columns.IsError {
		t.Fatalf("list_columns result=%#v err=%v", columns, err)
	}
	createdCard, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "create_card", Arguments: map[string]any{"title": "MCP card", "column_id": columnID}})
	if err != nil || createdCard.IsError {
		t.Fatalf("create_card result=%#v err=%v", createdCard, err)
	}
	var cardID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM cards WHERE title='MCP card'`).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	var secondColumnID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position OFFSET 1 LIMIT 1`).Scan(&secondColumnID); err != nil {
		t.Fatal(err)
	}
	movedCard, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "move_card", Arguments: map[string]any{"card_id": cardID, "expected_version": 1, "target_column_id": secondColumnID}})
	if err != nil || movedCard.IsError {
		t.Fatalf("move_card result=%#v err=%v", movedCard, err)
	}
	listedMovements, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_card_movements", Arguments: map[string]any{
		"from": time.Now().Add(-time.Hour).Format(time.RFC3339),
		"to":   time.Now().Add(time.Hour).Format(time.RFC3339),
	}})
	if err != nil || listedMovements.IsError {
		t.Fatalf("list_card_movements result=%#v err=%v", listedMovements, err)
	}
	movementsJSON, err := json.Marshal(listedMovements.StructuredContent)
	if err != nil || !strings.Contains(string(movementsJSON), "MCP card") || !strings.Contains(string(movementsJSON), "Admin") || !strings.Contains(string(movementsJSON), secondColumnID) {
		t.Fatalf("list_card_movements content=%s err=%v", movementsJSON, err)
	}
	addedComment, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "add_comment", Arguments: map[string]any{"card_id": cardID, "body": "Use the discussion requirement"}})
	if err != nil || addedComment.IsError {
		t.Fatalf("add_comment result=%#v err=%v", addedComment, err)
	}
	listedComments, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_comments", Arguments: map[string]any{"card_id": cardID}})
	if err != nil || listedComments.IsError {
		t.Fatalf("list_comments result=%#v err=%v", listedComments, err)
	}
	commentsJSON, err := json.Marshal(listedComments.StructuredContent)
	if err != nil || !strings.Contains(string(commentsJSON), "Use the discussion requirement") {
		t.Fatalf("list_comments content=%s err=%v", commentsJSON, err)
	}
	conflict, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "update_card", Arguments: map[string]any{"card_id": cardID, "expected_version": 99, "title": "Conflict"}})
	if err != nil || !conflict.IsError {
		t.Fatalf("conflict result=%#v err=%v", conflict, err)
	}
	conflictText, ok := conflict.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(conflictText.Text, "card_version_conflict") {
		t.Fatalf("conflict content=%#v", conflict.Content)
	}
	var actorTokenID string
	if err := db.QueryRowContext(ctx, `SELECT actor_token_id FROM card_activity WHERE action='created' ORDER BY created_at DESC LIMIT 1`).Scan(&actorTokenID); err != nil {
		t.Fatal(err)
	}
	if actorTokenID != created.ID {
		t.Fatalf("actor token=%s want=%s", actorTokenID, created.ID)
	}
}
