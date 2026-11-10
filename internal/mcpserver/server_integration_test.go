package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/cards"
	"github.com/ReanSn0w/justkb/internal/testdb"
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
	db := testdb.Open(t).DB
	ctx := context.Background()
	user, err := auth.NewService(db).CreateAdmin(ctx, "admin", "Admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	boardService := board.NewService(db)
	group, err := boardService.SaveGroup(ctx, "", "Backend", "#00d1b2", nil)
	if err != nil {
		t.Fatal(err)
	}
	groupID := group.ID
	if err := boardService.SetUserGroups(ctx, user.ID, []string{groupID}); err != nil {
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
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position LIMIT 1 OFFSET 1`).Scan(&secondColumnID); err != nil {
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
	var version int64
	if err := db.QueryRowContext(ctx, `SELECT version FROM cards WHERE id=$1`, cardID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(id,username,display_name,password_hash) VALUES ('extra','extra','Extra','unused')`); err != nil {
		t.Fatal(err)
	}
	assigned, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "assign_card", Arguments: map[string]any{
		"card_id": cardID, "expected_version": version, "assignee_ids": []string{user.ID, "extra"},
	}})
	if err != nil || assigned.IsError {
		t.Fatalf("assign_card result=%#v err=%v", assigned, err)
	}
	reported, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_card_movements", Arguments: map[string]any{
		"assignee_id": "extra", "card_id": cardID,
		"from": time.Now().Add(-time.Hour).Format(time.RFC3339),
		"to":   time.Now().Add(time.Hour).Format(time.RFC3339),
	}})
	if err != nil || reported.IsError {
		t.Fatalf("additional assignee report=%#v err=%v", reported, err)
	}
	reportJSON, err := json.Marshal(reported.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var report cards.MovementPage
	if err := json.Unmarshal(reportJSON, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Items) != 1 || len(report.Items[0].CurrentAssignees) != 2 || report.Items[0].CurrentAssignees[1].ID != "extra" {
		t.Fatalf("additional assignee report=%s", reportJSON)
	}
	cleared, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "assign_card", Arguments: map[string]any{
		"card_id": cardID, "expected_version": version + 1, "assignee_id": nil,
	}})
	if err != nil || cleared.IsError {
		t.Fatalf("legacy clear result=%#v err=%v", cleared, err)
	}
	var primary string
	if err := db.QueryRowContext(ctx, `SELECT assignee_id FROM cards WHERE id=$1`, cardID).Scan(&primary); err != nil || primary != "extra" {
		t.Fatalf("legacy clear primary=%s err=%v", primary, err)
	}
	var actorTokenID string
	if err := db.QueryRowContext(ctx, `SELECT actor_token_id FROM card_activity WHERE action='created' ORDER BY created_at DESC LIMIT 1`).Scan(&actorTokenID); err != nil {
		t.Fatal(err)
	}
	if actorTokenID != created.ID {
		t.Fatalf("actor token=%s want=%s", actorTokenID, created.ID)
	}
}
