package handler

import (
	"net/http"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type PromptHistoryHandler struct {
	repo *repository.AgentsRepo
}

func NewPromptHistoryHandler(repo *repository.AgentsRepo) *PromptHistoryHandler {
	return &PromptHistoryHandler{repo: repo}
}

func (h *PromptHistoryHandler) List(w http.ResponseWriter, r *http.Request) {
	entries, err := h.repo.ListPromptHistory(r.Context(), 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	if entries == nil {
		entries = []repository.PromptHistoryEntry{}
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: entries})
}
