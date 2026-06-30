package scheduler

import "testing"

func TestHandlerForRetryFailed(t *testing.T) {
	s := &Scheduler{}
	if s.handlerFor("retry_failed") == nil {
		t.Error(`handlerFor("retry_failed") returned nil; expected the retryFailed handler`)
	}
	if s.handlerFor("does_not_exist") != nil {
		t.Error("handlerFor(unknown) should be nil")
	}
}
