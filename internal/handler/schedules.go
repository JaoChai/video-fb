package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type SchedulesHandler struct {
	repo   *repository.SchedulesRepo
	reload func()
}

func NewSchedulesHandler(repo *repository.SchedulesRepo, reload func()) *SchedulesHandler {
	return &SchedulesHandler{repo: repo, reload: reload}
}

func (h *SchedulesHandler) List(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: schedules})
}

// schedulePatch uses pointers so a field the caller omits stays nil and is
// left untouched by the repo. A value type here would decode a missing field
// to its zero value (e.g. enabled=false) and silently clobber the stored row.
type schedulePatch struct {
	CronExpression *string `json:"cron_expression"`
	Action         *string `json:"action"`
	Enabled        *bool   `json:"enabled"`
}

func decodeSchedulePatch(r io.Reader) (schedulePatch, error) {
	var p schedulePatch
	err := json.NewDecoder(r).Decode(&p)
	return p, err
}

func (h *SchedulesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	patch, err := decodeSchedulePatch(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	if patch.CronExpression == nil && patch.Action == nil && patch.Enabled == nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "no fields to update"})
		return
	}
	if err := h.repo.Update(r.Context(), id, patch.CronExpression, patch.Action, patch.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	// Apply changes to the running cron without requiring a server restart.
	if h.reload != nil {
		go h.reload()
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}
