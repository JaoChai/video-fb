package router

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/handler"
)

// TestOrchestratorRoutesRegistered กัน route ที่พิมพ์ผิดแล้ว 404 เงียบๆ — ถ้า
// produce-tutorial ไม่ถูก register จะไม่มีทางผลิตคลิป tutorial ด้วยมือเลย
// (ต้องรอ cron 21:00) ซึ่งทำให้ทดสอบก่อนเปิดใช้จริงไม่ได้
func TestOrchestratorRoutesRegistered(t *testing.T) {
	r := chi.NewRouter()
	SetOrchestrator(r, &handler.OrchestratorHandler{})

	got := map[string]bool{}
	if err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method == http.MethodPost {
			got[route] = true
		}
		return nil
	}); err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	for _, want := range []string{
		"/api/v1/orchestrator/produce",
		"/api/v1/orchestrator/produce-tutorial",
		"/api/v1/orchestrator/produce-basic",
		"/api/v1/orchestrator/stop",
		"/api/v1/orchestrator/publish",
		"/api/v1/orchestrator/publish-tiktok",
		"/api/v1/orchestrator/retry",
	} {
		if !got[want] {
			t.Errorf("POST %s not registered", want)
		}
	}
}
