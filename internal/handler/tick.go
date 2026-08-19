package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/scheduler"
)

// NewTickHandler wraps a Dispatch func (scheduler.Scheduler.Dispatch in
// production) as an HTTP handler for POST /internal/tick/{action}. This is
// the endpoint a Cloudflare Workflow step calls instead of the Go binary's
// own internal cron — see docs/superpowers/specs/2026-08-19-cloudflare-migration-design.md.
//
// This endpoint has no authentication because it is only reachable through a
// Cloudflare Container binding (not exposed to the public internet). That trust
// boundary assumption must hold for as long as this endpoint lacks auth.
func NewTickHandler(dispatch func(ctx context.Context, action string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		action := r.PathValue("action")
		err := dispatch(r.Context(), action)

		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, scheduler.ErrUnknownAction):
				status = http.StatusNotFound
			case errors.Is(err, orchestrator.ErrProductionRunning):
				// Defensive: none of the 12 scheduler actions currently let ErrProductionRunning
				// escape to the caller (they catch it and return nil), so this 409 branch is
				// unreachable today but may be triggered if a future action changes that behavior.
				status = http.StatusConflict
			}
			log.Printf("tick %q failed (%d): %v", action, status, err)
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "action": action, "error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "action": action})
	}
}
