package mcpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/ReanSn0w/justkb/internal/archive"
	internalAuth "github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/cards"
	"github.com/ReanSn0w/justkb/internal/comments"
	appdb "github.com/ReanSn0w/justkb/internal/database"
	"github.com/ReanSn0w/justkb/internal/tokens"
	mcpAuth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type services struct {
	auth     *internalAuth.Service
	board    *board.Service
	cards    *cards.Service
	comments *comments.Service
	tokens   *tokens.Service
}

func NewHandler(db *sql.DB) http.Handler {
	services := &services{auth: internalAuth.NewService(db), board: board.NewService(db), cards: cards.NewService(db), comments: comments.NewService(db), tokens: tokens.NewService(db)}
	server := mcp.NewServer(&mcp.Implementation{Name: "jkb", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions:              "Manage the shared JKB kanban board. Call get_current_user before choosing assigned work. Read a card before mutating it and pass its current version as expected_version. Before starting work on a card or project, read its comments with list_comments because discussion may contain requirements not present in the description.",
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

	type currentUserInfo struct {
		User        internalAuth.User `json:"user"`
		Groups      []board.Group     `json:"groups"`
		TokenID     string            `json:"token_id"`
		TokenScopes []string          `json:"token_scopes"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "get_current_user", Description: "Get the owner of the current API token, their groups, role, and token scopes. Use the user ID with assignee filters; list_cards also accepts assignee_id='me'.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		principal, result := principalFromContext(ctx)
		if result != nil {
			return result, nil, nil
		}
		user, err := s.auth.UserByID(ctx, principal.User.ID)
		if err != nil {
			return domainResult(nil, err)
		}
		groupIDs := make(map[string]bool, len(user.GroupIDs))
		for _, id := range user.GroupIDs {
			groupIDs[id] = true
		}
		allGroups, err := s.board.ListGroups(ctx)
		if err != nil {
			return domainResult(nil, err)
		}
		groups := make([]board.Group, 0, len(user.GroupIDs))
		for _, group := range allGroups {
			if groupIDs[group.ID] {
				groups = append(groups, group)
			}
		}
		scopes := make([]string, 0, len(principal.Scopes))
		for scope := range principal.Scopes {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)
		return domainResult(currentUserInfo{User: user, Groups: groups, TokenID: principal.TokenID, TokenScopes: scopes}, nil)
	})

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
		RestoreMode     string  `json:"restore_mode"`
		ExpectedVersion int64   `json:"expected_version"`
		EpicID          string  `json:"epic_id,omitempty"`
		Name            string  `json:"name"`
		Description     *string `json:"description,omitempty"`
		Color           string  `json:"color,omitempty"`
		IsArchived      bool    `json:"is_archived,omitempty"`
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
		item, err := s.board.SaveEpic(ctx, input.EpicID, input.Name, input.Color, input.Description, input.IsArchived, board.EpicSaveOptions{ExpectedVersion: input.ExpectedVersion, Actor: archive.Actor{UserID: principal.User.ID, TokenID: principal.TokenID}, RestoreMode: input.RestoreMode})
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
	type listMovementsInput struct {
		From        string `json:"from,omitempty"`
		To          string `json:"to,omitempty"`
		ActorUserID string `json:"actor_user_id,omitempty"`
		AssigneeID  string `json:"assignee_id,omitempty"`
		ProjectID   string `json:"project_id,omitempty"`
		GroupID     string `json:"group_id,omitempty"`
		CardID      string `json:"card_id,omitempty"`
		Limit       int    `json:"limit,omitempty"`
		Cursor      string `json:"cursor,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "list_card_movements", Description: "List card movements across the whole board for a time period. from/to use RFC3339; when omitted the period is the previous 24 hours. Omit user filters for an all-user report. actor_user_id filters who performed the move; assignee_id filters the card's current assignee. Both accept 'me'. Results include card titles, column names, actor, current assignee, project, and group.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, input listMovementsInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:read")
		if result != nil {
			return result, nil, nil
		}
		from, to, err := movementPeriod(input.From, input.To, time.Now().UTC())
		if err != nil {
			return errorResult("validation_failed", map[string]any{"fields": []string{"from", "to"}, "format": time.RFC3339}), nil, nil
		}
		if input.ActorUserID == "me" {
			input.ActorUserID = principal.User.ID
		}
		if input.AssigneeID == "me" {
			input.AssigneeID = principal.User.ID
		}
		page, err := s.cards.ListMovements(ctx, cards.MovementFilters{
			From: from, To: to, ActorUserID: input.ActorUserID, AssigneeID: input.AssigneeID,
			EpicID: input.ProjectID, GroupID: input.GroupID, CardID: input.CardID,
			Limit: input.Limit, Cursor: input.Cursor,
		})
		return domainResult(page, err)
	})
	type commentTarget struct {
		CardID    string `json:"card_id,omitempty"`
		ProjectID string `json:"project_id,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "list_comments", Description: "List the discussion for one card or project. Read it before starting work because comments may contain current requirements and decisions.", Annotations: readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, input commentTarget) (*mcp.CallToolResult, any, error) {
		if result := requireScope(ctx, "cards:read"); result != nil {
			return result, nil, nil
		}
		if (input.CardID == "") == (input.ProjectID == "") {
			return errorResult("validation_failed", map[string]any{"fields": []string{"card_id", "project_id"}}), nil, nil
		}
		if input.ProjectID != "" {
			items, err := s.comments.ListEpic(ctx, input.ProjectID)
			return domainResult(items, err)
		}
		items, err := s.comments.ListCard(ctx, input.CardID)
		return domainResult(items, err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "add_comment", Description: "Add a Markdown comment to one card or project.", Annotations: closedWorld}, func(ctx context.Context, _ *mcp.CallToolRequest, input struct {
		CardID    string `json:"card_id,omitempty"`
		ProjectID string `json:"project_id,omitempty"`
		Body      string `json:"body"`
	}) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		if (input.CardID == "") == (input.ProjectID == "") {
			return errorResult("validation_failed", map[string]any{"fields": []string{"card_id", "project_id"}}), nil, nil
		}
		actor := comments.Actor{UserID: principal.User.ID, TokenID: principal.TokenID}
		if input.ProjectID != "" {
			item, err := s.comments.CreateEpic(ctx, input.ProjectID, actor, input.Body)
			return domainResult(item, err)
		}
		item, err := s.comments.CreateCard(ctx, input.CardID, actor, input.Body)
		return domainResult(item, err)
	})

	type createInput struct {
		Title       string     `json:"title"`
		Description string     `json:"description,omitempty"`
		ColumnID    string     `json:"column_id"`
		EpicID      string     `json:"epic_id,omitempty"`
		GroupID     string     `json:"group_id,omitempty"`
		AssigneeID  string     `json:"assignee_id,omitempty"`
		AssigneeIDs []string   `json:"assignee_ids,omitempty"`
		TagIDs      []string   `json:"tag_ids,omitempty"`
		DueAt       *time.Time `json:"due_at,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "create_card", Description: "Create a card.", Annotations: closedWorld}, func(ctx context.Context, _ *mcp.CallToolRequest, input createInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		item, err := s.cards.Create(ctx, cards.Actor{UserID: principal.User.ID, TokenID: principal.TokenID}, cards.CreateInput{Title: input.Title, Description: input.Description, ColumnID: input.ColumnID, EpicID: input.EpicID, GroupID: input.GroupID, AssigneeID: input.AssigneeID, AssigneeIDs: input.AssigneeIDs, TagIDs: input.TagIDs, DueAt: input.DueAt})
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
		AssigneeIDs     []string  `json:"assignee_ids,omitempty"`
		TagIDs          *[]string `json:"tag_ids,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "update_card", Description: "Update card fields using optimistic locking.", Annotations: closedWorld}, func(ctx context.Context, req *mcp.CallToolRequest, input updateInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		item, err := s.cards.Update(ctx, cards.Actor{UserID: principal.User.ID, TokenID: principal.TokenID}, input.CardID, cards.UpdateInput{ExpectedVersion: input.ExpectedVersion, Title: input.Title, Description: input.Description, EpicID: input.EpicID, GroupID: input.GroupID, AssigneeID: legacyAssigneeID(req, input.AssigneeID), AssigneeIDs: input.AssigneeIDs, TagIDs: input.TagIDs})
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
		CardID          string   `json:"card_id"`
		ExpectedVersion int64    `json:"expected_version"`
		GroupID         *string  `json:"group_id,omitempty"`
		AssigneeID      *string  `json:"assignee_id,omitempty"`
		AssigneeIDs     []string `json:"assignee_ids,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "assign_card", Description: "Assign a group or replace the assignees of a card. An empty assignee_ids array clears the list.", Annotations: closedWorld}, func(ctx context.Context, req *mcp.CallToolRequest, input assignInput) (*mcp.CallToolResult, any, error) {
		principal, result := principalWithScope(ctx, "cards:write")
		if result != nil {
			return result, nil, nil
		}
		item, err := s.cards.Update(ctx, cards.Actor{UserID: principal.User.ID, TokenID: principal.TokenID}, input.CardID, cards.UpdateInput{ExpectedVersion: input.ExpectedVersion, GroupID: input.GroupID, AssigneeID: legacyAssigneeID(req, input.AssigneeID), AssigneeIDs: input.AssigneeIDs})
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
	principal, result := principalFromContext(ctx)
	if result != nil {
		return internalAuth.Principal{}, result
	}
	if !principal.Scopes[scope] {
		return internalAuth.Principal{}, errorResult("insufficient_scope", map[string]any{"required_scope": scope})
	}
	return principal, nil
}

func principalFromContext(ctx context.Context) (internalAuth.Principal, *mcp.CallToolResult) {
	info := mcpAuth.TokenInfoFromContext(ctx)
	if info == nil {
		return internalAuth.Principal{}, errorResult("unauthenticated", nil)
	}
	principal, ok := info.Extra["principal"].(internalAuth.Principal)
	if !ok {
		return internalAuth.Principal{}, errorResult("unauthenticated", nil)
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
	case errors.Is(err, archive.ErrNameConflict):
		code = "project_name_conflict"
	case errors.Is(err, archive.ErrArchivedProject):
		code = "project_archived"
	case errors.Is(err, archive.ErrReadOnly):
		code = "object_archived"
	case errors.Is(err, archive.ErrValidation):
		code = "validation_failed"
	case appdb.IsBusy(err):
		code = "database_busy"
	case errors.Is(err, cards.ErrNotFound), errors.Is(err, board.ErrNotFound), errors.Is(err, comments.ErrNotFound):
		code = "not_found"
	case errors.Is(err, cards.ErrValidation), errors.Is(err, board.ErrValidation), errors.Is(err, comments.ErrValidation):
		code = "validation_failed"
	case errors.Is(err, board.ErrVersionConflict):
		code = "project_version_conflict"
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

func movementPeriod(fromValue, toValue string, now time.Time) (time.Time, time.Time, error) {
	to := now
	var err error
	if toValue != "" {
		to, err = time.Parse(time.RFC3339, toValue)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	from := to.Add(-24 * time.Hour)
	if fromValue != "" {
		from, err = time.Parse(time.RFC3339, fromValue)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if !from.Before(to) {
		return time.Time{}, time.Time{}, cards.ErrValidation
	}
	return from, to, nil
}

// The typed MCP input uses nil for both an omitted field and explicit JSON null.
func legacyAssigneeID(req *mcp.CallToolRequest, id *string) *string {
	if id != nil {
		return id
	}
	var arguments map[string]json.RawMessage
	if json.Unmarshal(req.Params.Arguments, &arguments) == nil && string(arguments["assignee_id"]) == "null" {
		empty := ""
		return &empty
	}
	return nil
}
