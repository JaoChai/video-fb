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

// fail_reason ของคลิป needs_review คือคำอธิบายเดียวที่เหลือว่าทำไมมันถูกกัก
// (log หายภายในไม่กี่ชั่วโมง). ก่อนหน้านี้ ClearFailReason ถูกเรียกแบบไม่มี
// เงื่อนไขทุกครั้ง จึงล้างเหตุผลที่เพิ่งเขียนไป 20 บรรทัดก่อนหน้าเสมอ
func TestShouldClearFailReason(t *testing.T) {
	if !shouldClearFailReason("ready") {
		t.Error(`shouldClearFailReason("ready") = false, want true — คลิปที่กลับมาดีต้องล้างเหตุผลเก่า`)
	}
	for _, s := range []string{"needs_review", "failed"} {
		if shouldClearFailReason(s) {
			t.Errorf("shouldClearFailReason(%q) = true, want false — เหตุผลต้องอยู่ให้คนอ่าน", s)
		}
	}
}
