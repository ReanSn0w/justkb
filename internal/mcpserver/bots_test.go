package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/testdb"
	"github.com/ReanSn0w/justkb/internal/tokens"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBotMCPIdentityAndRevocationOnExistingConnection(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	ts := tokens.NewService(db)
	admin, err := auth.NewService(db).CreateAdmin(ctx, "admin", "Admin", "test-password-long")
	if err != nil {
		t.Fatal(err)
	}
	first, err := ts.CreateBot(ctx, admin.ID, "First agent")
	if err != nil {
		t.Fatal(err)
	}
	second, err := ts.CreateBot(ctx, admin.ID, "Second agent")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewHandler(db))
	defer srv.Close()
	connect := func(secret string) *mcp.ClientSession {
		t.Helper()
		client := mcp.NewClient(&mcp.Implementation{Name: "bot-test", Version: "1"}, nil)
		session, e := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: bearerTransport{secret: secret, base: http.DefaultTransport}}, DisableStandaloneSSE: true}, nil)
		if e != nil {
			t.Fatal("MCP connection failed")
		}
		t.Cleanup(func() { session.Close() })
		return session
	}
	one, two := connect(first.Token.Secret), connect(second.Token.Secret)
	for _, item := range []struct {
		session *mcp.ClientSession
		id      string
	}{{one, first.UserID}, {two, second.UserID}} {
		result, e := item.session.CallTool(ctx, &mcp.CallToolParams{Name: "get_current_user", Arguments: map[string]any{}})
		if e != nil || result.IsError {
			t.Fatal("identity failed")
		}
		raw, _ := json.Marshal(result.StructuredContent)
		var payload struct{ User auth.User }
		if json.Unmarshal(raw, &payload) != nil || payload.User.ID != item.id || payload.User.AccountType != "bot" {
			t.Fatal("wrong bot identity")
		}
	}
	if _, err = ts.RotateBot(ctx, admin.ID, first.UserID, first.Token.ID); err != nil {
		t.Fatal(err)
	}
	result, err := one.CallTool(ctx, &mcp.CallToolParams{Name: "list_columns", Arguments: map[string]any{}})
	if err == nil && result != nil && !result.IsError {
		t.Fatal("existing connection bypassed token revocation")
	}
	result, err = two.CallTool(ctx, &mcp.CallToolParams{Name: "list_users", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatal("other bot affected by rotation")
	}
	b := board.NewService(db)
	if _, err = b.UpdateUser(ctx, second.UserID, "Second agent", "", "member", false); err != nil {
		t.Fatal(err)
	}
	result, err = two.CallTool(ctx, &mcp.CallToolParams{Name: "get_current_user", Arguments: map[string]any{}})
	if err == nil && result != nil && !result.IsError {
		t.Fatal("existing connection bypassed deactivation")
	}
}
