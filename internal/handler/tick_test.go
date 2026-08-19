package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/scheduler"
)

func TestTickHandler(t *testing.T) {
	cases := []struct {
		name       string
		dispatch   func(ctx context.Context, action string) error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "success",
			dispatch:   func(ctx context.Context, action string) error { return nil },
			wantStatus: http.StatusOK,
			wantBody:   "ok",
		},
		{
			name: "unknown action",
			dispatch: func(ctx context.Context, action string) error {
				return fmt.Errorf("%w: %q", scheduler.ErrUnknownAction, action)
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "error",
		},
		{
			name: "production already running",
			dispatch: func(ctx context.Context, action string) error {
				return orchestrator.ErrProductionRunning
			},
			wantStatus: http.StatusConflict,
			wantBody:   "error",
		},
		{
			name: "other failure",
			dispatch: func(ctx context.Context, action string) error {
				return errors.New("boom")
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "error",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.Handle("POST /internal/tick/{action}", NewTickHandler(c.dispatch))

			req := httptest.NewRequest(http.MethodPost, "/internal/tick/produce_and_publish", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != c.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", w.Code, c.wantStatus, w.Body.String())
			}
			var got map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("response not valid JSON: %v (%s)", err, w.Body.String())
			}
			if got["status"] != c.wantBody {
				t.Errorf("status field = %q, want %q", got["status"], c.wantBody)
			}
			if got["action"] != "produce_and_publish" {
				t.Errorf("action field = %q, want %q", got["action"], "produce_and_publish")
			}
		})
	}
}
