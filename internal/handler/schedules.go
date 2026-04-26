package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type SchedulesHandler struct {
	repo *repository.SchedulesRepo
}

func NewSchedulesHandler(repo *repository.SchedulesRepo) *SchedulesHandler {
	return &SchedulesHandler{repo: repo}
}

func (h *SchedulesHandler) List(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: schedules})
}

func (h *SchedulesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		CronExpression string `json:"cron_expression"`
		Action         string `json:"action"`
		Enabled        bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	if err := h.repo.Update(r.Context(), id, req.CronExpression, req.Action, req.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}
