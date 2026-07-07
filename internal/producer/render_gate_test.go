package producer

import "testing"

func TestRenderGateDisabled(t *testing.T) {
	if RenderGateDecision(true, false, 0) != RenderGateNone {
		t.Error("gate off must be RenderGateNone even when flagged")
	}
}

func TestRenderGateNotFlagged(t *testing.T) {
	if RenderGateDecision(false, true, 5) != RenderGateNone {
		t.Error("not flagged must be RenderGateNone")
	}
}

func TestRenderGateFirstOffenseRetries(t *testing.T) {
	if RenderGateDecision(true, true, 0) != RenderGateRetry {
		t.Error("first offense (retryCount 0) must retry")
	}
}

func TestRenderGatePersistentGoesToReview(t *testing.T) {
	if RenderGateDecision(true, true, 1) != RenderGateReview {
		t.Error("still broken after a retry must go to review")
	}
	if RenderGateDecision(true, true, 3) != RenderGateReview {
		t.Error("retryCount>=1 must go to review")
	}
}
