package handler

import (
	"net/http"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
	"github.com/jaochai/video-fb/internal/repository"
)

type PresetsHandler struct {
	analytics *repository.AnalyticsRepo
}

func NewPresetsHandler(analytics *repository.AnalyticsRepo) *PresetsHandler {
	return &PresetsHandler{analytics: analytics}
}

// Performance reports mean retention per preset. With one preset per format it
// answers "does the case file or the manual hold viewers longer".
func (h *PresetsHandler) Performance(w http.ResponseWriter, r *http.Request) {
	scores, err := h.analytics.PresetRetention(r.Context(), producer.DefaultWindowDays)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: scores})
}
