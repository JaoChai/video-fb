package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type AutoReviewsHandler struct{ repo *repository.AutoReviewsRepo }

func NewAutoReviewsHandler(repo *repository.AutoReviewsRepo) *AutoReviewsHandler {
	return &AutoReviewsHandler{repo: repo}
}

func (h *AutoReviewsHandler) GetByClip(w http.ResponseWriter, r *http.Request) {
	clipID := chi.URLParam(r, "clipId")
	a, err := h.repo.GetByClip(r.Context(), clipID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: a}) // a==nil → data:null
}
