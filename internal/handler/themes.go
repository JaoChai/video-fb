package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type ThemesHandler struct {
	repo *repository.ThemesRepo
}

func NewThemesHandler(repo *repository.ThemesRepo) *ThemesHandler {
	return &ThemesHandler{repo: repo}
}

func (h *ThemesHandler) List(w http.ResponseWriter, r *http.Request) {
	themes, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: themes})
}

func (h *ThemesHandler) GetActive(w http.ResponseWriter, r *http.Request) {
	theme, err := h.repo.GetActive(r.Context())
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "no active theme"})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: theme})
}

func (h *ThemesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var t models.BrandTheme
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	if err := h.repo.Update(r.Context(), id, t); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}
