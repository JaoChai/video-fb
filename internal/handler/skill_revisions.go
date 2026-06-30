package handler

import (
	"net/http"
	"strconv"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type SkillRevisionsHandler struct{ repo *repository.SkillRevisionsRepo }

func NewSkillRevisionsHandler(repo *repository.SkillRevisionsRepo) *SkillRevisionsHandler {
	return &SkillRevisionsHandler{repo: repo}
}

func (h *SkillRevisionsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}
	revs, err := h.repo.List(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: revs})
}
