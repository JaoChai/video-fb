package orchestrator

import "testing"

// downgradeIfReady centralizes every render gate's "first gate to fire wins"
// invariant: it only ever moves ready → needs_review and never clobbers any
// other status an earlier gate (or failure path) already set.
func TestDowngradeIfReady(t *testing.T) {
	status := "ready"
	downgradeIfReady(&status, false, "no-op %s", "x")
	if status != "ready" {
		t.Fatalf("cond=false must not downgrade, got %q", status)
	}

	downgradeIfReady(&status, true, "flagged %s", "x")
	if status != "needs_review" {
		t.Fatalf("cond=true from ready must downgrade, got %q", status)
	}

	// Already downgraded (or failed) — a later gate must be a no-op.
	for _, s := range []string{"needs_review", "failed", "published"} {
		status = s
		downgradeIfReady(&status, true, "flagged %s", "x")
		if status != s {
			t.Fatalf("status %q must never be clobbered, got %q", s, status)
		}
	}
}
