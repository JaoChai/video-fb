package handler

import (
	"encoding/json"
	"net/http"

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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []repository.PromptHistoryEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": entries})
}
