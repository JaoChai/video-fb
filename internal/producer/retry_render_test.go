package producer

import (
	"context"
	"errors"
	"testing"
)

func TestRetryRenderRetriesThenSucceeds(t *testing.T) {
	calls := 0
	err := retryRender(context.Background(), 3, func() error {
		calls++
		if calls < 2 {
			return errors.New("boom")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestRetryRenderExhausts(t *testing.T) {
	calls := 0
	err := retryRender(context.Background(), 2, func() error { calls++; return errors.New("nope") })
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}
