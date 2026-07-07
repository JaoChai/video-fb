package producer

import (
	"errors"
	"testing"
)

func TestUploadWithFallbackUsesPrimaryOnSuccess(t *testing.T) {
	fallbackCalled := false
	url, err := uploadWithFallback(
		func() (string, error) { return "primary-url", nil },
		func() (string, error) { fallbackCalled = true; return "fallback-url", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "primary-url" {
		t.Errorf("url = %q, want primary-url", url)
	}
	if fallbackCalled {
		t.Error("fallback should not run when primary succeeds")
	}
}

func TestUploadWithFallbackFallsBackOnPrimaryError(t *testing.T) {
	url, err := uploadWithFallback(
		func() (string, error) { return "", errors.New("r2 down") },
		func() (string, error) { return "fallback-url", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "fallback-url" {
		t.Errorf("url = %q, want fallback-url", url)
	}
}

func TestUploadWithFallbackReturnsFallbackError(t *testing.T) {
	_, err := uploadWithFallback(
		func() (string, error) { return "", errors.New("r2 down") },
		func() (string, error) { return "", errors.New("kie down") },
	)
	if err == nil {
		t.Fatal("expected error when both uploads fail")
	}
}
