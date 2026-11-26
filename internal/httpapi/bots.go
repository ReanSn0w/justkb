package httpapi

import (
	"net/http"

	"github.com/ReanSn0w/justkb/internal/avatars"
)

func (a *api) createBot(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DisplayName string `json:"display_name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	p, _ := PrincipalFromContext(r.Context())
	item, err := a.tokens.CreateBot(r.Context(), p.User.ID, input.DisplayName)
	w.Header().Set("Cache-Control", "no-store")
	writeCreated(w, r, item, err)
}
func (a *api) botToken(w http.ResponseWriter, r *http.Request) {
	item, err := a.tokens.BotToken(r.Context(), r.PathValue("id"))
	w.Header().Set("Cache-Control", "no-store")
	writeItem(w, r, item, err)
}
func (a *api) rotateBotToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ExpectedTokenID string `json:"expected_token_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	p, _ := PrincipalFromContext(r.Context())
	item, err := a.tokens.RotateBot(r.Context(), p.User.ID, r.PathValue("id"), input.ExpectedTokenID)
	w.Header().Set("Cache-Control", "no-store")
	writeItem(w, r, item, err)
}
func (a *api) uploadBotAvatar(w http.ResponseWriter, r *http.Request) {
	if _, err := a.tokens.BotToken(r.Context(), r.PathValue("id")); err != nil {
		handleError(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, avatars.MaxSize+(1<<20))
	if err := r.ParseMultipartForm(avatars.MaxSize); err != nil {
		handleError(w, r, avatars.ErrTooLarge)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "validation_failed", "Avatar file is required", nil)
		return
	}
	defer file.Close()
	if _, err = a.avatars.Upload(r.Context(), r.PathValue("id"), file); err != nil {
		handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *api) deleteBotAvatar(w http.ResponseWriter, r *http.Request) {
	if _, err := a.tokens.BotToken(r.Context(), r.PathValue("id")); err != nil {
		handleError(w, r, err)
		return
	}
	if err := a.avatars.Delete(r.Context(), r.PathValue("id")); err != nil {
		handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
