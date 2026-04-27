package handler

import (
	"net/http"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/progress"
)

type ProductionHandler struct {
	tracker *progress.Tracker
}

func NewProductionHandler(tracker *progress.Tracker) *ProductionHandler {
	return &ProductionHandler{tracker: tracker}
}

func (h *ProductionHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, models.APIResponse{Data: h.tracker.GetStatus()})
}
