package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/orchestrator"
)

type OrchestratorHandler struct {
	orch *orchestrator.Orchestrator
}

func NewOrchestratorHandler(orch *orchestrator.Orchestrator) *OrchestratorHandler {
	return &OrchestratorHandler{orch: orch}
}

func (h *OrchestratorHandler) TriggerWeekly(w http.ResponseWriter, r *http.Request) {
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
		ctx := context.Background()
		if err := h.orch.ProduceWeekly(ctx, count); err != nil {
			log.Printf("Weekly production failed: %v", err)
		}
	}()
}
