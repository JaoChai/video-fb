package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type AnalyticsHandler struct {
	repo *repository.AnalyticsRepo
}

func NewAnalyticsHandler(repo *repository.AnalyticsRepo) *AnalyticsHandler {
	return &AnalyticsHandler{repo: repo}
}

func (h *AnalyticsHandler) ListByClip(w http.ResponseWriter, r *http.Request) {
	clipID := chi.URLParam(r, "clipId")
	analytics, err := h.repo.ListByClip(r.Context(), clipID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: analytics})
}

func (h *AnalyticsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.repo.Summary(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	topClips, err := h.repo.TopClips(r.Context(), 10)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{
		"summary":   summary,
		"top_clips": topClips,
	}})
}
