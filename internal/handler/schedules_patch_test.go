package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// A PATCH must update only the fields the caller actually sent. Before the
// fix, every field was a value type, so an unset "enabled" decoded to false
// and clobbered the stored value — indistinguishable from an explicit
// "disable". These tests pin the pointer-based decode + the empty-body guard.

func TestDecodeSchedulePatch_UnsentFieldsAreNil(t *testing.T) {
	p, err := decodeSchedulePatch(strings.NewReader(`{"cron_expression":"0 9 * * 1,4"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Enabled != nil {
		t.Errorf("Enabled: want nil (not sent), got %v", *p.Enabled)
	}
	if p.Action != nil {
		t.Errorf("Action: want nil (not sent), got %q", *p.Action)
	}
	if p.CronExpression == nil {
		t.Fatal("CronExpression: want non-nil, got nil")
	}
	if got := *p.CronExpression; got != "0 9 * * 1,4" {
		t.Errorf("CronExpression: want %q, got %q", "0 9 * * 1,4", got)
	}
}

func TestDecodeSchedulePatch_ExplicitFalseSurvives(t *testing.T) {
	p, err := decodeSchedulePatch(strings.NewReader(`{"enabled":false}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Enabled == nil {
		t.Fatal("Enabled: want non-nil, got nil (explicit false must survive)")
	}
	if *p.Enabled != false {
		t.Errorf("Enabled: want false, got true")
	}
}

func TestScheduleUpdate_EmptyBodyDoesNotTouchRepo(t *testing.T) {
	h := NewSchedulesHandler(nil, nil) // nil repo: any touch panics
	r := chi.NewRouter()
	r.Patch("/schedules/{id}", h.Update)

	req := httptest.NewRequest(http.MethodPatch, "/schedules/abc", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("empty body: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestScheduleUpdate_BrokenJSONDoesNotTouchRepo(t *testing.T) {
	h := NewSchedulesHandler(nil, nil) // nil repo: any touch panics
	r := chi.NewRouter()
	r.Patch("/schedules/{id}", h.Update)

	req := httptest.NewRequest(http.MethodPatch, "/schedules/abc", strings.NewReader(`{`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("broken JSON: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}
