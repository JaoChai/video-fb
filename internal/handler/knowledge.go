package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type KnowledgeHandler struct {
	repo *repository.KnowledgeRepo
}

func NewKnowledgeHandler(repo *repository.KnowledgeRepo) *KnowledgeHandler {
	return &KnowledgeHandler{repo: repo}
}

func (h *KnowledgeHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.repo.ListSources(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: sources})
}

func (h *KnowledgeHandler) CreateSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		URL            string `json:"url"`
		SourceType     string `json:"source_type"`
		CrawlFrequency string `json:"crawl_frequency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	source, err := h.repo.CreateSource(r.Context(), req.Name, req.URL, req.SourceType, req.CrawlFrequency)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, models.APIResponse{Data: source})
}

func (h *KnowledgeHandler) ToggleSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct{ Enabled bool `json:"enabled"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	if err := h.repo.ToggleSource(r.Context(), id, req.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}

func (h *KnowledgeHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteSource(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "deleted"})
}
