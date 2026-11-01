package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ReanSn0w/justkb/internal/attachments"
	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/cards"
	"github.com/ReanSn0w/justkb/internal/ratelimit"
	"github.com/ReanSn0w/justkb/internal/tokens"
)

const jsonBodyLimit = 1 << 20

type api struct {
	auth          *auth.Service
	board         *board.Service
	cards         *cards.Service
	attachments   *attachments.Service
	attachmentMax int64
	cookieSecure  bool
	loginLimiter  *ratelimit.Limiter
	tokens        *tokens.Service
}

func registerAPI(mux *http.ServeMux, deps Dependencies) {
	attachmentService, err := attachments.NewService(deps.DB, deps.StoragePath, deps.AttachmentMaxSize)
	if err != nil {
		panic(err)
	}
	server := &api{
		auth: auth.NewService(deps.DB), board: board.NewService(deps.DB), cards: cards.NewService(deps.DB),
		attachments: attachmentService, attachmentMax: deps.AttachmentMaxSize, cookieSecure: deps.CookieSecure,
		loginLimiter: ratelimit.New(10, 15*time.Minute),
		tokens:       tokens.NewService(deps.DB),
	}
	mux.HandleFunc("POST /api/v1/auth/login", server.login)
	mux.Handle("POST /api/v1/auth/logout", server.member(http.HandlerFunc(server.logout)))
	mux.Handle("GET /api/v1/auth/me", server.member(http.HandlerFunc(server.me)))
	mux.Handle("PUT /api/v1/auth/password", server.member(http.HandlerFunc(server.changePassword)))

	mux.Handle("GET /api/v1/users", server.member(http.HandlerFunc(server.listUsers)))
	mux.Handle("POST /api/v1/users", server.admin(http.HandlerFunc(server.createUser)))
	mux.Handle("GET /api/v1/users/{id}", server.member(http.HandlerFunc(server.getUser)))
	mux.Handle("PATCH /api/v1/users/{id}", server.admin(http.HandlerFunc(server.updateUser)))
	mux.Handle("PUT /api/v1/users/{id}/groups", server.admin(http.HandlerFunc(server.setUserGroups)))
	mux.Handle("PUT /api/v1/users/{id}/password", server.admin(http.HandlerFunc(server.resetUserPassword)))

	mux.Handle("GET /api/v1/groups", server.member(http.HandlerFunc(server.listGroups)))
	mux.Handle("POST /api/v1/groups", server.admin(http.HandlerFunc(server.createGroup)))
	mux.Handle("GET /api/v1/groups/{id}", server.member(http.HandlerFunc(server.getGroup)))
	mux.Handle("PATCH /api/v1/groups/{id}", server.admin(http.HandlerFunc(server.updateGroup)))

	mux.Handle("GET /api/v1/columns", server.member(http.HandlerFunc(server.listColumns)))
	mux.Handle("POST /api/v1/columns", server.admin(http.HandlerFunc(server.createColumn)))
	mux.Handle("PATCH /api/v1/columns/{id}", server.admin(http.HandlerFunc(server.updateColumn)))
	mux.Handle("POST /api/v1/columns/reorder", server.admin(http.HandlerFunc(server.reorderColumns)))
	mux.Handle("DELETE /api/v1/columns/{id}", server.admin(http.HandlerFunc(server.deleteColumn)))

	mux.Handle("GET /api/v1/epics", server.member(http.HandlerFunc(server.listEpics)))
	mux.Handle("POST /api/v1/epics", server.admin(http.HandlerFunc(server.createEpic)))
	mux.Handle("GET /api/v1/epics/{id}", server.member(http.HandlerFunc(server.getEpic)))
	mux.Handle("PATCH /api/v1/epics/{id}", server.admin(http.HandlerFunc(server.updateEpic)))
	mux.Handle("POST /api/v1/epics/{id}/attachments", server.member(http.HandlerFunc(server.uploadEpicAttachment)))

	mux.Handle("GET /api/v1/tags", server.member(http.HandlerFunc(server.listTags)))
	mux.Handle("POST /api/v1/tags", server.admin(http.HandlerFunc(server.createTag)))
	mux.Handle("GET /api/v1/tags/{id}", server.member(http.HandlerFunc(server.getTag)))
	mux.Handle("PATCH /api/v1/tags/{id}", server.admin(http.HandlerFunc(server.updateTag)))

	mux.Handle("GET /api/v1/cards", server.member(http.HandlerFunc(server.listCards)))
	mux.Handle("POST /api/v1/cards", server.member(http.HandlerFunc(server.createCard)))
	mux.Handle("GET /api/v1/cards/{id}", server.member(http.HandlerFunc(server.getCard)))
	mux.Handle("PATCH /api/v1/cards/{id}", server.member(http.HandlerFunc(server.updateCard)))
	mux.Handle("POST /api/v1/cards/{id}/move", server.member(http.HandlerFunc(server.moveCard)))
	mux.Handle("POST /api/v1/cards/{id}/archive", server.member(http.HandlerFunc(server.archiveCard)))
	mux.Handle("POST /api/v1/cards/{id}/restore", server.member(http.HandlerFunc(server.restoreCard)))
	mux.Handle("GET /api/v1/cards/{id}/activity", server.member(http.HandlerFunc(server.cardActivity)))
	mux.Handle("POST /api/v1/cards/{id}/attachments", server.member(http.HandlerFunc(server.uploadAttachment)))
	mux.Handle("GET /api/v1/attachments/{id}", server.member(http.HandlerFunc(server.downloadAttachment)))
	mux.Handle("DELETE /api/v1/attachments/{id}", server.member(http.HandlerFunc(server.deleteAttachment)))
	mux.Handle("GET /api/v1/tokens", server.member(http.HandlerFunc(server.listTokens)))
	mux.Handle("POST /api/v1/tokens", server.member(http.HandlerFunc(server.createToken)))
	mux.Handle("DELETE /api/v1/tokens/{id}", server.member(http.HandlerFunc(server.revokeToken)))
}

func (a *api) member(next http.Handler) http.Handler { return RequireSession(a.auth, next) }
func (a *api) admin(next http.Handler) http.Handler {
	return RequireSession(a.auth, RequireRole("admin", next))
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	key, _, _ := net.SplitHostPort(r.RemoteAddr)
	if key == "" {
		key = r.RemoteAddr
	}
	if !a.loginLimiter.Allow(key) {
		writeAPIError(w, r, http.StatusTooManyRequests, "rate_limited", "Too many login attempts", nil)
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	session, err := a.auth.Login(r.Context(), input.Username, input.Password, 7*24*time.Hour)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeAPIError(w, r, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials", nil)
		return
	}
	if err != nil {
		handleError(w, r, err)
		return
	}
	a.loginLimiter.Reset(key)
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: session.Token, Path: "/", HttpOnly: true, Secure: a.cookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 60 * 60})
	writeJSON(w, http.StatusOK, map[string]any{"user": session.User, "csrf_token": session.CSRF})
}

func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_ = a.auth.Logout(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Path: "/", HttpOnly: true, Secure: a.cookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) me(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	allGroups, err := a.board.ListGroups(r.Context())
	if err != nil {
		handleError(w, r, err)
		return
	}
	wanted := make(map[string]bool, len(principal.User.GroupIDs))
	for _, id := range principal.User.GroupIDs {
		wanted[id] = true
	}
	groups := make([]board.Group, 0, len(wanted))
	for _, group := range allGroups {
		if wanted[group.ID] {
			groups = append(groups, group)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": principal.User, "groups": groups, "csrf_token": principal.CSRFToken})
}

func (a *api) changePassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Current string `json:"current_password"`
		Next    string `json:"new_password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	if err := a.auth.ChangePassword(r.Context(), principal.User.ID, input.Current, input.Next); err != nil {
		handleError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cookieSecure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) listUsers(w http.ResponseWriter, r *http.Request) {
	items, err := a.board.ListUsers(r.Context(), r.URL.Query().Get("include_inactive") == "true", r.URL.Query().Get("group_id"))
	writeList(w, r, items, err)
}
func (a *api) getUser(w http.ResponseWriter, r *http.Request) {
	items, err := a.board.ListUsers(r.Context(), true, "")
	if err != nil {
		handleError(w, r, err)
		return
	}
	for _, item := range items {
		if item.ID == r.PathValue("id") {
			writeJSON(w, http.StatusOK, item)
			return
		}
	}
	handleError(w, r, board.ErrNotFound)
}
func (a *api) createUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		Password    string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := a.board.CreateUser(r.Context(), input.Username, input.DisplayName, input.Email, input.Role, input.Password)
	writeCreated(w, r, item, err)
}
func (a *api) updateUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		IsActive    bool   `json:"is_active"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := a.board.UpdateUser(r.Context(), r.PathValue("id"), input.DisplayName, input.Email, input.Role, input.IsActive)
	writeItem(w, r, item, err)
}
func (a *api) setUserGroups(w http.ResponseWriter, r *http.Request) {
	var input struct {
		GroupIDs []string `json:"group_ids"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := a.board.SetUserGroups(r.Context(), r.PathValue("id"), input.GroupIDs); err != nil {
		handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *api) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := a.board.ResetUserPassword(r.Context(), r.PathValue("id"), input.Password); err != nil {
		handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) listGroups(w http.ResponseWriter, r *http.Request) {
	items, err := a.board.ListGroups(r.Context())
	writeList(w, r, items, err)
}
func (a *api) getGroup(w http.ResponseWriter, r *http.Request) {
	items, err := a.board.ListGroups(r.Context())
	if err != nil {
		handleError(w, r, err)
		return
	}
	for _, item := range items {
		if item.ID == r.PathValue("id") {
			writeJSON(w, http.StatusOK, item)
			return
		}
	}
	handleError(w, r, board.ErrNotFound)
}

type groupInput struct {
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	Description *string `json:"description"`
}

func (a *api) createGroup(w http.ResponseWriter, r *http.Request) {
	var input groupInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := a.board.SaveGroup(r.Context(), "", input.Name, input.Color, input.Description)
	writeCreated(w, r, item, err)
}
func (a *api) updateGroup(w http.ResponseWriter, r *http.Request) {
	var input groupInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := a.board.SaveGroup(r.Context(), r.PathValue("id"), input.Name, input.Color, input.Description)
	writeItem(w, r, item, err)
}

func (a *api) listColumns(w http.ResponseWriter, r *http.Request) {
	items, err := a.board.ListColumns(r.Context())
	writeList(w, r, items, err)
}

type columnInput struct {
	Name     string `json:"name"`
	WIPLimit *int   `json:"wip_limit"`
}

func (a *api) createColumn(w http.ResponseWriter, r *http.Request) {
	var input columnInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := a.board.SaveColumn(r.Context(), "", input.Name, input.WIPLimit)
	writeCreated(w, r, item, err)
}
func (a *api) updateColumn(w http.ResponseWriter, r *http.Request) {
	var input columnInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := a.board.SaveColumn(r.Context(), r.PathValue("id"), input.Name, input.WIPLimit)
	writeItem(w, r, item, err)
}
func (a *api) reorderColumns(w http.ResponseWriter, r *http.Request) {
	var input struct {
		IDs []string `json:"column_ids"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := a.board.ReorderColumns(r.Context(), input.IDs); err != nil {
		handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *api) deleteColumn(w http.ResponseWriter, r *http.Request) {
	if err := a.board.DeleteColumn(r.Context(), r.PathValue("id")); err != nil {
		handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) listEpics(w http.ResponseWriter, r *http.Request) {
	items, err := a.board.ListEpics(r.Context(), r.URL.Query().Get("include_archived") == "true")
	writeList(w, r, items, err)
}
func (a *api) getEpic(w http.ResponseWriter, r *http.Request) {
	item, err := a.board.GetEpic(r.Context(), r.PathValue("id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	files, err := a.attachments.ListEpic(r.Context(), item.ID)
	if err != nil {
		handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		board.Epic
		Attachments []attachments.Metadata `json:"attachments"`
	}{Epic: item, Attachments: files})
}

type epicInput struct {
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	Description *string `json:"description"`
	IsArchived  bool    `json:"is_archived"`
}

func (a *api) createEpic(w http.ResponseWriter, r *http.Request) {
	var input epicInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := a.board.SaveEpic(r.Context(), "", input.Name, input.Color, input.Description, input.IsArchived)
	writeCreated(w, r, item, err)
}
func (a *api) updateEpic(w http.ResponseWriter, r *http.Request) {
	var input epicInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := a.board.SaveEpic(r.Context(), r.PathValue("id"), input.Name, input.Color, input.Description, input.IsArchived)
	writeItem(w, r, item, err)
}

func (a *api) listTags(w http.ResponseWriter, r *http.Request) {
	items, err := a.board.ListTags(r.Context())
	writeList(w, r, items, err)
}
func (a *api) getTag(w http.ResponseWriter, r *http.Request) {
	items, err := a.board.ListTags(r.Context())
	if err != nil {
		handleError(w, r, err)
		return
	}
	for _, item := range items {
		if item.ID == r.PathValue("id") {
			writeJSON(w, http.StatusOK, item)
			return
		}
	}
	handleError(w, r, board.ErrNotFound)
}

type tagInput struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (a *api) createTag(w http.ResponseWriter, r *http.Request) {
	var input tagInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := a.board.SaveTag(r.Context(), "", input.Name, input.Color)
	writeCreated(w, r, item, err)
}
func (a *api) updateTag(w http.ResponseWriter, r *http.Request) {
	var input tagInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := a.board.SaveTag(r.Context(), r.PathValue("id"), input.Name, input.Color)
	writeItem(w, r, item, err)
}

type createCardInput struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	ColumnID    string     `json:"column_id"`
	EpicID      string     `json:"epic_id"`
	GroupID     string     `json:"group_id"`
	AssigneeID  string     `json:"assignee_id"`
	TagIDs      []string   `json:"tag_ids"`
	DueAt       *time.Time `json:"due_at"`
}

func (a *api) createCard(w http.ResponseWriter, r *http.Request) {
	var input createCardInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p, _ := PrincipalFromContext(r.Context())
	item, err := a.cards.Create(r.Context(), cards.Actor{UserID: p.User.ID}, cards.CreateInput{Title: input.Title, Description: input.Description, ColumnID: input.ColumnID, EpicID: input.EpicID, GroupID: input.GroupID, AssigneeID: input.AssigneeID, TagIDs: input.TagIDs, DueAt: input.DueAt})
	writeCreated(w, r, item, err)
}
func (a *api) getCard(w http.ResponseWriter, r *http.Request) {
	item, err := a.cards.Get(r.Context(), r.PathValue("id"))
	writeItem(w, r, item, err)
}
func (a *api) listCards(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	archived, _ := strconv.ParseBool(r.URL.Query().Get("archived"))
	page, err := a.cards.List(r.Context(), cards.Filters{EpicID: r.URL.Query().Get("epic_id"), GroupID: r.URL.Query().Get("group_id"), AssigneeID: r.URL.Query().Get("assignee_id"), CurrentUserID: p.User.ID, TagIDs: r.URL.Query()["tag_id"], ColumnID: r.URL.Query().Get("column_id"), Query: r.URL.Query().Get("q"), Archived: archived, Limit: limit, Cursor: r.URL.Query().Get("cursor")})
	if err != nil {
		handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "next_cursor": nullableString(page.NextCursor)})
}
func (a *api) updateCard(w http.ResponseWriter, r *http.Request) {
	var raw map[string]json.RawMessage
	if !decodeJSON(w, r, &raw) {
		return
	}
	expected, err := requiredInt64(raw, "expected_version")
	if err != nil {
		handleError(w, r, cards.ErrValidation)
		return
	}
	input := cards.UpdateInput{ExpectedVersion: expected}
	decodeOptionalString(raw, "title", &input.Title)
	decodeOptionalString(raw, "description", &input.Description)
	decodeNullableString(raw, "epic_id", &input.EpicID)
	decodeNullableString(raw, "group_id", &input.GroupID)
	decodeNullableString(raw, "assignee_id", &input.AssigneeID)
	if value, ok := raw["tag_ids"]; ok {
		var ids []string
		if json.Unmarshal(value, &ids) != nil {
			handleError(w, r, cards.ErrValidation)
			return
		}
		input.TagIDs = &ids
	}
	if value, ok := raw["due_at"]; ok {
		var dueAt *time.Time
		if string(value) != "null" {
			var rawTime string
			if json.Unmarshal(value, &rawTime) != nil {
				handleError(w, r, cards.ErrValidation)
				return
			}
			parsed, parseErr := time.Parse(time.RFC3339, rawTime)
			if parseErr != nil {
				parsed, parseErr = time.Parse("2006-01-02T15:04", rawTime)
			}
			if parseErr != nil {
				handleError(w, r, cards.ErrValidation)
				return
			}
			dueAt = &parsed
		}
		input.DueAt = &dueAt
	}
	p, _ := PrincipalFromContext(r.Context())
	item, err := a.cards.Update(r.Context(), cards.Actor{UserID: p.User.ID}, r.PathValue("id"), input)
	writeItem(w, r, item, err)
}
func (a *api) moveCard(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ExpectedVersion int64  `json:"expected_version"`
		TargetColumnID  string `json:"target_column_id"`
		BeforeCardID    string `json:"before_card_id"`
		AfterCardID     string `json:"after_card_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	p, _ := PrincipalFromContext(r.Context())
	item, err := a.cards.Move(r.Context(), cards.Actor{UserID: p.User.ID}, r.PathValue("id"), cards.MoveInput{ExpectedVersion: input.ExpectedVersion, TargetColumnID: input.TargetColumnID, BeforeCardID: input.BeforeCardID, AfterCardID: input.AfterCardID})
	writeItem(w, r, item, err)
}
func (a *api) archiveCard(w http.ResponseWriter, r *http.Request) { a.setCardArchive(w, r, true) }
func (a *api) restoreCard(w http.ResponseWriter, r *http.Request) { a.setCardArchive(w, r, false) }
func (a *api) setCardArchive(w http.ResponseWriter, r *http.Request, archived bool) {
	var input struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	p, _ := PrincipalFromContext(r.Context())
	item, err := a.cards.SetArchived(r.Context(), cards.Actor{UserID: p.User.ID}, r.PathValue("id"), input.ExpectedVersion, archived)
	writeItem(w, r, item, err)
}
func (a *api) cardActivity(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := a.cards.Activity(r.Context(), r.PathValue("id"), limit)
	writeList(w, r, items, err)
}

func (a *api) uploadAttachment(w http.ResponseWriter, r *http.Request) {
	a.uploadFor(w, r, r.PathValue("id"), "")
}
func (a *api) uploadEpicAttachment(w http.ResponseWriter, r *http.Request) {
	a.uploadFor(w, r, "", r.PathValue("id"))
}
func (a *api) uploadFor(w http.ResponseWriter, r *http.Request, cardID, epicID string) {
	r.Body = http.MaxBytesReader(w, r.Body, a.attachmentMax+1<<20)
	if err := r.ParseMultipartForm(a.attachmentMax); err != nil {
		handleError(w, r, attachments.ErrTooLarge)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		handleError(w, r, err)
		return
	}
	defer file.Close()
	p, _ := PrincipalFromContext(r.Context())
	var item attachments.Metadata
	if epicID != "" {
		item, err = a.attachments.UploadEpic(r.Context(), epicID, p.User.ID, header.Filename, file)
	} else {
		item, err = a.attachments.Upload(r.Context(), cardID, p.User.ID, header.Filename, file)
	}
	writeCreated(w, r, item, err)
}
func (a *api) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	download, err := a.attachments.Open(r.Context(), r.PathValue("id"))
	if err != nil {
		handleError(w, r, err)
		return
	}
	defer download.File.Close()
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": download.Metadata.OriginalName}))
	w.Header().Set("Content-Type", download.Metadata.MediaType)
	w.Header().Set("Content-Length", strconv.FormatInt(download.Metadata.Size, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, download.File)
}
func (a *api) deleteAttachment(w http.ResponseWriter, r *http.Request) {
	if err := a.attachments.Delete(r.Context(), r.PathValue("id")); err != nil {
		handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) listTokens(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	items, err := a.tokens.List(r.Context(), principal.User.ID)
	writeList(w, r, items, err)
}

func (a *api) createToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name      string     `json:"name"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	item, err := a.tokens.Create(r.Context(), principal.User.ID, input.Name, input.Scopes, input.ExpiresAt)
	writeCreated(w, r, item, err)
}

func (a *api) revokeToken(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	if err := a.tokens.Revoke(r.Context(), principal.User.ID, r.PathValue("id")); err != nil {
		handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, jsonBodyLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_json", "Request body is invalid", nil)
		return false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_json", "Request body must contain one JSON value", nil)
		return false
	}
	return true
}
func handleError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "Internal server error"
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		status, code, message = http.StatusUnauthorized, "invalid_credentials", "Invalid credentials"
	case errors.Is(err, board.ErrNotFound), errors.Is(err, cards.ErrNotFound), errors.Is(err, attachments.ErrNotFound), errors.Is(err, tokens.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "Resource not found"
	case errors.Is(err, board.ErrValidation), errors.Is(err, cards.ErrValidation), errors.Is(err, attachments.ErrTypeForbidden), errors.Is(err, tokens.ErrInvalidScope), errors.Is(err, tokens.ErrInvalidToken):
		status, code, message = http.StatusUnprocessableEntity, "validation_failed", "Validation failed"
	case errors.Is(err, cards.ErrVersionConflict):
		status, code, message = http.StatusConflict, "card_version_conflict", "Card was changed by another actor"
	case errors.Is(err, board.ErrColumnNotEmpty):
		status, code, message = http.StatusConflict, "column_not_empty", "Column contains cards"
	case errors.Is(err, attachments.ErrTooLarge):
		status, code, message = http.StatusRequestEntityTooLarge, "attachment_too_large", "Attachment is too large"
	}
	writeAPIError(w, r, status, code, message, nil)
}
func writeList(w http.ResponseWriter, r *http.Request, items any, err error) {
	if err != nil {
		handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
}
func writeItem(w http.ResponseWriter, r *http.Request, item any, err error) {
	if err != nil {
		handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func writeCreated(w http.ResponseWriter, r *http.Request, item any, err error) {
	if err != nil {
		handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func requiredInt64(raw map[string]json.RawMessage, key string) (int64, error) {
	value, ok := raw[key]
	if !ok {
		return 0, fmt.Errorf("missing")
	}
	var result int64
	return result, json.Unmarshal(value, &result)
}
func decodeOptionalString(raw map[string]json.RawMessage, key string, target **string) {
	value, ok := raw[key]
	if !ok {
		return
	}
	var parsed string
	if json.Unmarshal(value, &parsed) == nil {
		*target = &parsed
	}
}
func decodeNullableString(raw map[string]json.RawMessage, key string, target **string) {
	value, ok := raw[key]
	if !ok {
		return
	}
	if string(value) == "null" {
		empty := ""
		*target = &empty
		return
	}
	decodeOptionalString(raw, key, target)
}
