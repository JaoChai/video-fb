package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// 'published' must only ever be written by the publisher (it's what marks a
// real Zernio upload) and 'producing' only by the orchestrator. A raw PATCH
// setting either would bypass the publish gate entirely.
func TestUpdateRejectsPipelineOnlyStatus(t *testing.T) {
	h := NewClipsHandler(nil) // guard rejects before the repo is touched
	r := chi.NewRouter()
	r.Patch("/clips/{id}", h.Update)

	for _, s := range []string{"published", "producing"} {
		req := httptest.NewRequest(http.MethodPatch, "/clips/abc",
			strings.NewReader(`{"status":"`+s+`"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status %q: got %d, want 400", s, w.Code)
		}
	}
}
