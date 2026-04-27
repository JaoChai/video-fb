package handler

import (
	"context"
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
	countStr := r.URL.Query().Get("count")
	count := 7
	if countStr != "" {
		if n, err := strconv.Atoi(countStr); err == nil && n > 0 {
			count = n
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
