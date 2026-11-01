package mcpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	internalAuth "github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/cards"
	"github.com/ReanSn0w/justkb/internal/tokens"
	mcpAuth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type services struct {
	board  *board.Service
	cards  *cards.Service
	tokens *tokens.Service
}

func NewHandler(db *sql.DB) http.Handler {
	services := &services{board: board.NewService(db), cards: cards.NewService(db), tokens: tokens.NewService(db)}
	server := mcp.NewServer(&mcp.Implementation{Name: "jkb", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions:              "Manage the shared JKB kanban board. Read a card before mutating it and pass its current version as expected_version.",
		SupportedProtocolVersions: []string{"2026-07-28", "2025-11-25"},
	})
	services.addTools(server)
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		Stateless: true,
	})
	verifier := func(ctx context.Context, secret string, _ *http.Request) (*mcpAuth.TokenInfo, error) {
		principal, err := services.tokens.Authenticate(ctx, secret)
		if err != nil {
			return nil, fmt.Errorf("%w: personal access token rejected", mcpAuth.ErrInvalidToken)
		}
		scopes := make([]string, 0, len(principal.Scopes))
		for scope := range principal.Scopes {
			scopes = append(scopes, scope)
		}
		return &mcpAuth.TokenInfo{UserID: principal.User.ID, Scopes: scopes, Extra: map[string]any{"principal": principal}}, nil
	}
	return mcpAuth.RequireBearerToken(verifier, &mcpAuth.RequireBearerTokenOptions{AllowMissingExpiration: true})(streamable)
}

func (s *services) addTools(server *mcp.Server) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPointer(false)}
	closedWorld := &mcp.ToolAnnotations{OpenWorldHint: boolPointer(false)}

	mcp.AddTool(server, &mcp.Tool{Name: "list_columns", Description: "List kanban columns in board order.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		if result := requireScope(ctx, "cards:read"); result != nil {
			return result, nil, nil
		}
		items, err := s.board.ListColumns(ctx)
		return domainResult(items, err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "list_epics", Description: "List projects.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, input struct {
		IncludeArchived bool `json:"include_archived,omitempty"`
	}) (*mcp.CallToolResult, any, error) {
		if result := requireScope(ctx, "cards:read"); result != nil {
			return result, nil, nil
		}
		items, err := s.board.ListEpics(ctx, input.IncludeArchived)
		return domainResult(items, err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_epic", Description: "Get a project with its description and task count.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, input struct {
		EpicID string `json:"epic_id"`
	}) (*mcp.CallToolResult, any, error) {
		if result := requireScope(ctx, "cards:read"); result != nil {
			return result, nil, nil
		}
		item, err := s.board.GetEpic(ctx, input.EpicID)
		return domainResult(item, err)
	})
	type epicInput struct {
		EpicID      string  `json:"epic_id,omitempty"`
		Name        string  `json:"name"`
		Description *string `json:"description,omitempty"`
		Color       string  `json:"color,omitempty"`
		IsArchived  bool    `json:"is_archived,omitempty"`
	}
	saveEpic := func(ctx context.Context, input epicInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		if principal.User.Role != "admin" {
			return errorResult("forbidden", nil), nil, nil
		}
		if input.Color == "" {
			input.Color = "#00d1b2"
		}
		item, err := s.board.SaveEpic(ctx, input.EpicID, input.Name, input.Color, input.Description, input.IsArchived)
		return domainResult(item, err)
	}
	mcp.AddTool(server, &mcp.Tool{Name: "create_epic", Description: "Create a project. Admin only.", Annotations: closedWorld}, func(ctx context.Context, _ *mcp.CallToolRequest, input epicInput) (*mcp.CallToolResult, any, error) {
		input.EpicID = ""
		return saveEpic(ctx, input)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "update_epic", Description: "Update a project. Read it first and provide the complete state. Admin only.", Annotations: closedWorld}, func(ctx context.Context, _ *mcp.CallToolRequest, input epicInput) (*mcp.CallToolResult, any, error) {
		return saveEpic(ctx, input)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "list_groups", Description: "List task groups.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		if result := requireScope(ctx, "cards:read"); result != nil {
			return result, nil, nil
		}
		items, err := s.board.ListGroups(ctx)
		return domainResult(items, err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "list_users", Description: "List assignable users without sensitive fields.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, input struct {
		GroupID         string `json:"group_id,omitempty"`
		IncludeInactive bool   `json:"include_inactive,omitempty"`
	}) (*mcp.CallToolResult, any, error) {
		if result := requireScope(ctx, "cards:read"); result != nil {
			return result, nil, nil
		}
		items, err := s.board.ListUsers(ctx, input.IncludeInactive, input.GroupID)
		return domainResult(items, err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "list_tags", Description: "List card tags.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		if result := requireScope(ctx, "cards:read"); result != nil {
			return result, nil, nil
		}
		items, err := s.board.ListTags(ctx)
		return domainResult(items, err)
	})
	type listCardsInput struct {
		EpicID     string   `json:"epic_id,omitempty"`
		GroupID    string   `json:"group_id,omitempty"`
		AssigneeID string   `json:"assignee_id,omitempty"`
		TagIDs     []string `json:"tag_ids,omitempty"`
		ColumnID   string   `json:"column_id,omitempty"`
		Query      string   `json:"query,omitempty"`
		Archived   bool     `json:"archived,omitempty"`
		Limit      int      `json:"limit,omitempty"`
		Cursor     string   `json:"cursor,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "list_cards", Description: "List cards using combinable board filters.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, input listCardsInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:read")
		if result != nil {
			return result, nil, nil
		}
		page, err := s.cards.List(ctx, cards.Filters{EpicID: input.EpicID, GroupID: input.GroupID, AssigneeID: input.AssigneeID, CurrentUserID: principal.User.ID, TagIDs: input.TagIDs, ColumnID: input.ColumnID, Query: input.Query, Archived: input.Archived, Limit: input.Limit, Cursor: input.Cursor})
		return domainResult(page, err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_card", Description: "Get a card by UUID or JKB number.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, input struct {
		CardID     string `json:"card_id,omitempty"`
		CardNumber string `json:"card_number,omitempty"`
	}) (*mcp.CallToolResult, any, error) {
		if result := requireScope(ctx, "cards:read"); result != nil {
			return result, nil, nil
		}
		identifier := input.CardID
		if identifier == "" {
			identifier = input.CardNumber
		}
		if identifier == "" {
			return errorResult("validation_failed", map[string]any{"field": "card_id"}), nil, nil
		}
		item, err := s.cards.Get(ctx, identifier)
		return domainResult(item, err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_card_activity", Description: "Get recent card activity.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, input struct {
		CardID string `json:"card_id"`
		Limit  int    `json:"limit,omitempty"`
	}) (*mcp.CallToolResult, any, error) {
		if result := requireScope(ctx, "cards:read"); result != nil {
			return result, nil, nil
		}
		items, err := s.cards.Activity(ctx, input.CardID, input.Limit)
		return domainResult(items, err)
	})

	type createInput struct {
		Title       string     `json:"title"`
		Description string     `json:"description,omitempty"`
		ColumnID    string     `json:"column_id"`
		EpicID      string     `json:"epic_id,omitempty"`
		GroupID     string     `json:"group_id,omitempty"`
		AssigneeID  string     `json:"assignee_id,omitempty"`
		TagIDs      []string   `json:"tag_ids,omitempty"`
		DueAt       *time.Time `json:"due_at,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "create_card", Description: "Create a card.", Annotations: closedWorld}, func(ctx context.Context, _ *mcp.CallToolRequest, input createInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		item, err := s.cards.Create(ctx, cards.Actor{UserID: principal.User.ID, TokenID: principal.TokenID}, cards.CreateInput{Title: input.Title, Description: input.Description, ColumnID: input.ColumnID, EpicID: input.EpicID, GroupID: input.GroupID, AssigneeID: input.AssigneeID, TagIDs: input.TagIDs, DueAt: input.DueAt})
		return domainResult(item, err)
	})
	type updateInput struct {
		CardID          string    `json:"card_id"`
		ExpectedVersion int64     `json:"expected_version"`
		Title           *string   `json:"title,omitempty"`
		Description     *string   `json:"description,omitempty"`
		EpicID          *string   `json:"epic_id,omitempty"`
		GroupID         *string   `json:"group_id,omitempty"`
		AssigneeID      *string   `json:"assignee_id,omitempty"`
		TagIDs          *[]string `json:"tag_ids,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "update_card", Description: "Update card fields using optimistic locking.", Annotations: closedWorld}, func(ctx context.Context, _ *mcp.CallToolRequest, input updateInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		item, err := s.cards.Update(ctx, cards.Actor{UserID: principal.User.ID, TokenID: principal.TokenID}, input.CardID, cards.UpdateInput{ExpectedVersion: input.ExpectedVersion, Title: input.Title, Description: input.Description, EpicID: input.EpicID, GroupID: input.GroupID, AssigneeID: input.AssigneeID, TagIDs: input.TagIDs})
		return domainResult(item, err)
	})
	type moveInput struct {
		CardID          string `json:"card_id"`
		ExpectedVersion int64  `json:"expected_version"`
		TargetColumnID  string `json:"target_column_id"`
		BeforeCardID    string `json:"before_card_id,omitempty"`
		AfterCardID     string `json:"after_card_id,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "move_card", Description: "Move a card using optimistic locking.", Annotations: closedWorld}, func(ctx context.Context, _ *mcp.CallToolRequest, input moveInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		item, err := s.cards.Move(ctx, cards.Actor{UserID: principal.User.ID, TokenID: principal.TokenID}, input.CardID, cards.MoveInput{ExpectedVersion: input.ExpectedVersion, TargetColumnID: input.TargetColumnID, BeforeCardID: input.BeforeCardID, AfterCardID: input.AfterCardID})
		return domainResult(item, err)
	})
	type assignInput struct {
		CardID          string  `json:"card_id"`
		ExpectedVersion int64   `json:"expected_version"`
		GroupID         *string `json:"group_id,omitempty"`
		AssigneeID      *string `json:"assignee_id,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "assign_card", Description: "Assign a group or user to a card.", Annotations: closedWorld}, func(ctx context.Context, _ *mcp.CallToolRequest, input assignInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		item, err := s.cards.Update(ctx, cards.Actor{UserID: principal.User.ID, TokenID: principal.TokenID}, input.CardID, cards.UpdateInput{ExpectedVersion: input.ExpectedVersion, GroupID: input.GroupID, AssigneeID: input.AssigneeID})
		return domainResult(item, err)
	})
	type archiveInput struct {
		CardID          string `json:"card_id"`
		ExpectedVersion int64  `json:"expected_version"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "archive_card", Description: "Archive a card.", Annotations: closedWorld}, func(ctx context.Context, _ *mcp.CallToolRequest, input archiveInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		item, err := s.cards.SetArchived(ctx, cards.Actor{UserID: principal.User.ID, TokenID: principal.TokenID}, input.CardID, input.ExpectedVersion, true)
		return domainResult(item, err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "restore_card", Description: "Restore an archived card.", Annotations: closedWorld}, func(ctx context.Context, _ *mcp.CallToolRequest, input archiveInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		item, err := s.cards.SetArchived(ctx, cards.Actor{UserID: principal.User.ID, TokenID: principal.TokenID}, input.CardID, input.ExpectedVersion, false)
		return domainResult(item, err)
	})
}

func principalWithScope(ctx context.Context, scope string) (internalAuth.Principal, *mcp.CallToolResult) {
	info := mcpAuth.TokenInfoFromContext(ctx)
	if info == nil {
		return internalAuth.Principal{}, errorResult("unauthenticated", nil)
	}
	principal, ok := info.Extra["principal"].(internalAuth.Principal)
	if !ok {
		return internalAuth.Principal{}, errorResult("unauthenticated", nil)
	}
	if !principal.Scopes[scope] {
		return internalAuth.Principal{}, errorResult("insufficient_scope", map[string]any{"required_scope": scope})
	}
	return principal, nil
}

func requireScope(ctx context.Context, scope string) *mcp.CallToolResult {
	_, result := principalWithScope(ctx, scope)
	return result
}

func domainResult(value any, err error) (*mcp.CallToolResult, any, error) {
	if err == nil {
		return nil, value, nil
	}
	code := "internal_error"
	details := map[string]any{}
	switch {
	case errors.Is(err, cards.ErrNotFound), errors.Is(err, board.ErrNotFound):
		code = "not_found"
	case errors.Is(err, cards.ErrValidation), errors.Is(err, board.ErrValidation):
		code = "validation_failed"
	case errors.Is(err, cards.ErrVersionConflict):
		code = "card_version_conflict"
	}
	return errorResult(code, details), nil, nil
}

func errorResult(code string, details map[string]any) *mcp.CallToolResult {
	if details == nil {
		details = map[string]any{}
	}
	payload, _ := json.Marshal(map[string]any{"code": code, "details": details})
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}}}
}

func boolPointer(value bool) *bool { return &value }
