package httpapi

import (
	"net/http"
	"strconv"

	"github.com/ReanSn0w/justkb/internal/archive"
)

func (a *api) archivePolicy(w http.ResponseWriter, r *http.Request) {
	p, err := a.archive.Policy(r.Context())
	writeItem(w, r, p, err)
}

type policyInput struct {
	archive.Policy
	ExpectedVersion int64 `json:"expected_version"`
}

func (a *api) saveArchivePolicy(w http.ResponseWriter, r *http.Request) {
	var input policyInput
	if !decodeJSON(w, r, &input) {
		return
	}
	p, err := a.archive.SavePolicy(r.Context(), input.Policy, input.ExpectedVersion)
	writeItem(w, r, p, err)
}
func (a *api) previewArchivePolicy(w http.ResponseWriter, r *http.Request) {
	var input archive.Policy
	if !decodeJSON(w, r, &input) {
		return
	}
	p, err := a.archive.Preview(r.Context(), input)
	writeItem(w, r, p, err)
}
func (a *api) listArchive(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 30
	var err error
	if q.Get("limit") != "" {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil {
			handleError(w, r, archive.ErrValidation)
			return
		}
	}
	history := q.Get("history") == "true"
	if h := q.Get("history"); h != "" && h != "true" && h != "false" {
		handleError(w, r, archive.ErrValidation)
		return
	}
	page, err := a.archive.List(r.Context(), archive.Filters{Type: q.Get("type"), ProjectID: q.Get("project_id"), Query: q.Get("q"), From: q.Get("from"), To: q.Get("to"), State: q.Get("state"), History: history, Cursor: q.Get("cursor"), Limit: limit, BatchID: q.Get("batch_id")})
	writeItem(w, r, page, err)
}
func (a *api) restoreProject(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ExpectedVersion int64  `json:"expected_version"`
		Mode            string `json:"mode"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	p, _ := PrincipalFromContext(r.Context())
	if err := a.archive.RestoreProject(r.Context(), archive.Actor{UserID: p.User.ID}, r.PathValue("id"), input.ExpectedVersion, input.Mode); err != nil {
		handleError(w, r, err)
		return
	}
	item, err := a.board.GetEpic(r.Context(), r.PathValue("id"))
	writeItem(w, r, item, err)
}
