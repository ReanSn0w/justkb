package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/cards"
	"github.com/ReanSn0w/justkb/internal/testdb"
	"github.com/ReanSn0w/justkb/internal/tokens"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAdvertisedContractAndReportPagination(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	user, err := auth.NewService(db).CreateAdmin(ctx, "reporter", "Reporter", "test-password-long")
	if err != nil {
		t.Fatal(err)
	}
	token, err := tokens.NewService(db).Create(ctx, user.ID, "test", []string{"cards:read", "cards:write"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewHandler(db))
	defer srv.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "contract-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: bearerTransport{secret: token.Secret, base: http.DefaultTransport}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range listed.Tools {
		guidance := map[string][]string{
			"list_columns": {"allow_card_creation", "does not restrict moving or restoring", "If no column allows creation", "do not attempt a workaround"},
			"create_card":  {"allow_card_creation=true", "card_creation_not_allowed", "refresh list_columns", "If no column allows creation", "Do not create elsewhere and immediately move"},
			"move_card":    {"allow_card_creation=false does not restrict moving", "Do not use create-then-move"},
		}
		for _, phrase := range guidance[tool.Name] {
			if !strings.Contains(tool.Description, phrase) {
				t.Errorf("%s missing creation policy guidance %q", tool.Name, phrase)
			}
		}
		raw, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]struct {
				Description string
				Maximum     *float64
			}
			Required []string
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		for name, p := range schema.Properties {
			if p.Description == "" {
				t.Errorf("%s.%s has no description", tool.Name, name)
			}
			if name == "limit" && p.Maximum == nil {
				t.Errorf("%s has no maximum", tool.Name)
			}
		}
		if tool.Name == "create_epic" && strings.Join(schema.Required, ",") != "name" {
			t.Errorf("create required=%v", schema.Required)
		}
		if tool.Name == "get_card_activity" && schema.Properties["cursor"].Description != "" {
			t.Fatal("activity advertises unsupported cursor")
		}
	}
	call := func(name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	created := call("create_epic", map[string]any{"name": "Minimal project"})
	if created.IsError {
		t.Fatal("minimal project creation rejected")
	}
	var column string
	if err := db.QueryRow(`SELECT id FROM board_columns ORDER BY position LIMIT 1`).Scan(&column); err != nil {
		t.Fatal(err)
	}
	card, err := cards.NewService(db).Create(ctx, cards.Actor{UserID: user.ID}, cards.CreateInput{Title: "Report", ColumnID: column})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 507; i++ {
		_, err = tx.Exec(`INSERT INTO card_activity(id,card_id,actor_user_id,action,changes,created_at) VALUES(?,?,?,'moved',?,'2026-11-25 12:00:00')`, fmt.Sprintf("movement-%04d", i), card.ID, user.ID, fmt.Sprintf(`{"from_column_id":%q,"to_column_id":%q}`, column, column))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		max   int
		extra map[string]any
	}{
		{"list_cards", 100, map[string]any{}}, {"get_card_activity", 100, map[string]any{"card_id": card.ID}},
		{"list_card_movements", 500, map[string]any{"from": "2026-11-25T00:00:00Z", "to": "2026-11-26T00:00:00Z"}}, {"list_archive", 100, map[string]any{}},
	} {
		for _, limit := range []int{0, -1, 1, tc.max, tc.max + 1} {
			tc.extra["limit"] = limit
			result := call(tc.name, tc.extra)
			wantError := limit > tc.max || (tc.name == "list_archive" && limit < 0)
			if result.IsError != wantError {
				t.Errorf("%s limit=%d error=%v want=%v", tc.name, limit, result.IsError, wantError)
			}
		}
		delete(tc.extra, "limit")
		if call(tc.name, tc.extra).IsError {
			t.Errorf("%s missing limit", tc.name)
		}
	}
	if _, err := db.Exec(`UPDATE board_columns SET allow_card_creation=0 WHERE id=?`, column); err != nil {
		t.Fatal(err)
	}
	denied := call("create_card", map[string]any{"title": "Forbidden", "column_id": column})
	if !denied.IsError {
		t.Fatal("MCP admin bypassed creation policy")
	}
	if text, ok := denied.Content[0].(*mcp.TextContent); !ok || !strings.Contains(text.Text, "card_creation_not_allowed") {
		t.Fatal("missing policy error")
	}
	seen := map[string]bool{}
	cursor := ""
	for {
		args := map[string]any{"from": "2026-11-25T00:00:00Z", "to": "2026-11-26T00:00:00Z", "limit": 500}
		if cursor != "" {
			args["cursor"] = cursor
		}
		result := call("list_card_movements", args)
		if result.IsError {
			t.Fatal("report failed after invalid limit")
		}
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var page cards.MovementPage
		if err = json.Unmarshal(raw, &page); err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatalf("duplicate %s", item.ID)
			}
			seen[item.ID] = true
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
		if len(seen) > 507 {
			t.Fatal("pagination loop")
		}
	}
	if len(seen) != 507 {
		t.Fatalf("report collected %d/507", len(seen))
	}
	if !call("list_comments", map[string]any{"card_id": card.ID, "project_id": "other"}).IsError {
		t.Fatal("ambiguous target accepted")
	}
	if !call("assign_card", map[string]any{"card_id": card.ID, "expected_version": card.Version, "assignee_id": user.ID, "assignee_ids": []string{user.ID}}).IsError {
		t.Fatal("ambiguous assignees accepted")
	}
	// A real cursor typo must not be interpreted as the first page.
	if !call("list_card_movements", map[string]any{"from": time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), "to": "2026-11-26T00:00:00Z", "cursor": "bad"}).IsError {
		t.Fatal("invalid cursor accepted")
	}
}
