package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type CritiquesHandler struct{ repo *repository.CritiquesRepo }

func NewCritiquesHandler(repo *repository.CritiquesRepo) *CritiquesHandler {
	return &CritiquesHandler{repo: repo}
}

func (h *CritiquesHandler) GetByClip(w http.ResponseWriter, r *http.Request) {
	clipID := chi.URLParam(r, "clipId")
	c, err := h.repo.GetByClip(r.Context(), clipID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: c}) // c==nil → data:null
}
