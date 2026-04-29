package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type AgentsHandler struct {
	repo *repository.AgentsRepo
}

func NewAgentsHandler(repo *repository.AgentsRepo) *AgentsHandler {
	return &AgentsHandler{repo: repo}
}

func (h *AgentsHandler) List(w http.ResponseWriter, r *http.Request) {
	agents, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: agents})
}

func (h *AgentsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		SystemPrompt   string  `json:"system_prompt"`
		PromptTemplate string  `json:"prompt_template"`
		Model          string  `json:"model"`
		Temperature    float64 `json:"temperature"`
		Enabled        bool    `json:"enabled"`
		Skills         string  `json:"skills"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	if err := h.repo.Update(r.Context(), id, req.SystemPrompt, req.PromptTemplate, req.Model, req.Temperature, req.Enabled, req.Skills); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}
