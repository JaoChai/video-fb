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
		// ProduceWeekly owns the production gate AND its cancellation registration,
		// so the handler just kicks it off with a base context.
		if err := h.orch.ProduceWeekly(context.Background(), count, nil); err != nil {
			log.Printf("Weekly production failed: %v", err)
			h.tracker.AddErrorLog(err.Error())
		}
	}()
}

// TriggerTutorial produces ONE tutorial clip from the catalog on demand. Exists
// so a tutorial can be produced and eyeballed before the 21:00 schedule is turned
// on — without it the only way to see a tutorial clip is to wait for the cron.
// Takes no count: the topic is a catalog row, and one clip per run is the shape
// the whole tutorial path is built around.
func (h *OrchestratorHandler) TriggerTutorial(w http.ResponseWriter, r *http.Request) {
	if s := h.tracker.GetStatus(); s.Active {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: "Production already in progress"})
		return
	}

	writeJSON(w, http.StatusAccepted, models.APIResponse{
		Message: "Tutorial production started in background",
	})

	go func() {
		// ProduceTutorial owns the production gate AND its cancellation
		// registration, so the handler just kicks it off with a base context.
		if err := h.orch.ProduceTutorial(context.Background()); err != nil {
			log.Printf("Tutorial production failed: %v", err)
			h.tracker.AddErrorLog(err.Error())
		}
	}()
}

// TriggerBasic produces ONE basic (beginner) clip from the catalog on demand.
// Mirrors TriggerTutorial: the topic is a catalog row, so one clip per run is the
// shape the basic path is built around — no count.
func (h *OrchestratorHandler) TriggerBasic(w http.ResponseWriter, r *http.Request) {
	if s := h.tracker.GetStatus(); s.Active {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: "Production already in progress"})
		return
	}

	writeJSON(w, http.StatusAccepted, models.APIResponse{
		Message: "Basic production started in background",
	})

	go func() {
		// ProduceBasic owns the production gate AND its cancellation
		// registration, so the handler just kicks it off with a base context.
		if err := h.orch.ProduceBasic(context.Background()); err != nil {
			log.Printf("Basic production failed: %v", err)
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

func (h *OrchestratorHandler) TriggerPublishTikTok(w http.ResponseWriter, r *http.Request) {
	go func() {
		if err := h.pub.PublishTikTok(context.Background()); err != nil {
			log.Printf("Manual TikTok publish failed: %v", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, models.APIResponse{Message: "Publishing latest clip to TikTok"})
}

func (h *OrchestratorHandler) RetryFailed(w http.ResponseWriter, r *http.Request) {
	if s := h.tracker.GetStatus(); s.Active {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: "Production already in progress"})
		return
	}

	writeJSON(w, http.StatusAccepted, models.APIResponse{
		Message: "Retrying failed clips in background",
	})

	go func() {
		if err := h.orch.RetryAllFailed(context.Background(), 2, 0); err != nil {
			log.Printf("Retry all failed: %v", err)
			h.tracker.AddErrorLog(err.Error())
		}
	}()
}
