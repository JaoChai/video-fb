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

type presetInfo struct {
	Key          string `json:"key"`
	DisplayName  string `json:"display_name"`
	PrimaryColor string `json:"primary_color"`
	AccentColor  string `json:"accent_color"`
}

func (h *PresetsHandler) List(w http.ResponseWriter, r *http.Request) {
	infos := make([]presetInfo, 0, len(producer.Presets))
	for _, p := range producer.Presets {
		infos = append(infos, presetInfo{
			Key: p.Key, DisplayName: p.DisplayName,
			PrimaryColor: p.Palette.Navy, AccentColor: p.Palette.Orange,
		})
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{
		"presets":               infos,
		"style_presets_enabled": producer.StylePresetsEnabled(),
		"performance_enabled":   producer.StylePresetsPerformanceEnabled(),
	}})
}

func (h *PresetsHandler) Performance(w http.ResponseWriter, r *http.Request) {
	scores, err := h.analytics.PresetRetention(r.Context(), producer.DefaultWindowDays)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: scores})
}
