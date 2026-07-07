package producer

import "testing"

// Fail-closed must be OFF by default — flipping publish policy silently on
// deploy is exactly the class of surprise this repo's flags exist to prevent.
func TestQAFailClosedEnabledDefaultOff(t *testing.T) {
	t.Setenv("QA_FAIL_CLOSED_ENABLED", "")
	if QAFailClosedEnabled() {
		t.Error("must default to off")
	}
	t.Setenv("QA_FAIL_CLOSED_ENABLED", "true")
	if !QAFailClosedEnabled() {
		t.Error("'true' must enable")
	}
	t.Setenv("QA_FAIL_CLOSED_ENABLED", "1")
	if QAFailClosedEnabled() {
		t.Error("only the literal 'true' enables (repo flag convention)")
	}
}
