package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type TutorialFeaturesHandler struct {
	repo *repository.TutorialFeaturesRepo
}

func NewTutorialFeaturesHandler(repo *repository.TutorialFeaturesRepo) *TutorialFeaturesHandler {
	return &TutorialFeaturesHandler{repo: repo}
}

// Unpark returns one feature to the pool right away, for when a human has looked
// at the real menu and knows the research verdict was wrong. Without it the only
// way back in is waiting out the park window.
func (h *TutorialFeaturesHandler) Unpark(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Unpark(r.Context(), chi.URLParam(r, "featureKey")); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]string{"status": "unparked"}})
}
