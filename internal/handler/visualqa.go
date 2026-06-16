package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type VisualQAHandler struct {
	repo *repository.VisualQARepo
}

func NewVisualQAHandler(repo *repository.VisualQARepo) *VisualQAHandler {
	return &VisualQAHandler{repo: repo}
}

// GetByClip returns the latest Visual QA run for a clip so the review UI can
// explain why it was flagged. Returns null data (200) when the clip has no QA
// row — the absence is a valid state, not an error.
func (h *VisualQAHandler) GetByClip(w http.ResponseWriter, r *http.Request) {
	clipID := chi.URLParam(r, "clipId")
	qa, err := h.repo.GetLatestByClipID(r.Context(), clipID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: qa})
}
