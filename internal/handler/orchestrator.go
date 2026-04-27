package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/progress"
	"github.com/jaochai/video-fb/internal/publisher"
)

type OrchestratorHandler struct {
	orch    *orchestrator.Orchestrator
	tracker *progress.Tracker
	pub     *publisher.Publisher
}

func NewOrchestratorHandler(orch *orchestrator.Orchestrator, tracker *progress.Tracker, pub *publisher.Publisher) *OrchestratorHandler {
	return &OrchestratorHandler{orch: orch, tracker: tracker, pub: pub}
}

func (h *OrchestratorHandler) TriggerWeekly(w http.ResponseWriter, r *http.Request) {
	if s := h.tracker.GetStatus(); s.Active {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: "Production already in progress"})
		return
	}

	count := 7
	if countStr := r.URL.Query().Get("count"); countStr != "" {
		if n, err := strconv.Atoi(countStr); err == nil && n > 0 {
			count = n
		}
	} else if r.Body != nil {
		var body struct {
			Count int `json:"count"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.Count > 0 {
			count = body.Count
		}
	}

	writeJSON(w, http.StatusAccepted, models.APIResponse{
		Message: "Weekly production started in background",
	})

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		h.tracker.SetCancelFunc(cancel)
		defer cancel()

		if err := h.orch.ProduceWeekly(ctx, count); err != nil {
			log.Printf("Weekly production failed: %v", err)
			h.tracker.AddErrorLog(err.Error())
		}
	}()
}

func (h *OrchestratorHandler) StopProduction(w http.ResponseWriter, r *http.Request) {
	h.tracker.Cancel()
	h.tracker.AddErrorLog("Production stopped by user")
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "Production stop requested"})
}

func (h *OrchestratorHandler) TriggerPublish(w http.ResponseWriter, r *http.Request) {
	go func() {
		if err := h.pub.PublishReady(context.Background()); err != nil {
			log.Printf("Manual publish failed: %v", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, models.APIResponse{Message: "Publishing ready clips"})
}
