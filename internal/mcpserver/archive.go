package mcpserver

import (
	"context"
	"github.com/ReanSn0w/justkb/internal/archive"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *services) addArchiveTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "list_archive", Description: "Read the shared archive timeline, including project batches and past transitions.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, input archive.Filters) (*mcp.CallToolResult, any, error) {
		if result := requireScope(ctx, "cards:read"); result != nil {
			return result, nil, nil
		}
		page, err := s.archive.List(ctx, input)
		return domainResult(page, err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "restore_epic", Description: "Restore a project. Admin only. Read its version and restorable_cards first; choose project_only or with_cards."}, func(ctx context.Context, _ *mcp.CallToolRequest, input struct {
		EpicID          string `json:"epic_id"`
		ExpectedVersion int64  `json:"expected_version"`
		Mode            string `json:"mode"`
	}) (*mcp.CallToolResult, any, error) {
		p, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		if p.User.Role != "admin" {
			return errorResult("forbidden", nil), nil, nil
		}
		if err := s.archive.RestoreProject(ctx, archive.Actor{UserID: p.User.ID, TokenID: p.TokenID}, input.EpicID, input.ExpectedVersion, input.Mode); err != nil {
			return domainResult(nil, err)
		}
		item, err := s.board.GetEpic(ctx, input.EpicID)
		return domainResult(item, err)
	})
}
