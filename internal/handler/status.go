package handler

import (
	"net/http"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
)

type StatusHandler struct{ prod *producer.Producer }

func NewStatusHandler(p *producer.Producer) *StatusHandler { return &StatusHandler{prod: p} }

func (h *StatusHandler) KieCredits(w http.ResponseWriter, r *http.Request) {
	credits, err := h.prod.KieCredits(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{"credits": -1, "error": err.Error()}})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{"credits": credits}})
}
