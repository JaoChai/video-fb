package config

import "testing"

func TestLoadWorkerModeDefaultsFalse(t *testing.T) {
	t.Setenv("WORKER_MODE", "")
	cfg := Load()
	if cfg.WorkerMode {
		t.Error("WorkerMode should default to false when WORKER_MODE is unset")
	}
}

func TestLoadWorkerModeTrue(t *testing.T) {
	t.Setenv("WORKER_MODE", "true")
	cfg := Load()
	if !cfg.WorkerMode {
		t.Error("WorkerMode should be true when WORKER_MODE=true")
	}
}
