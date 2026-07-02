package orchestrator

import "testing"

// The QA/auto-review frame sampler must NOT collapse to t=0 when per-scene
// duration estimates are missing (persisted duration_seconds is 0). It spreads
// n frames over the real, probed video duration so each lands on content.
func TestEvenFrameTimestamps_SpreadsOverRealDuration(t *testing.T) {
	dur := 89.45
	n := 10
	ts := evenFrameTimestamps(dur, n)
	if len(ts) != n {
		t.Fatalf("len = %d, want %d", len(ts), n)
	}
	// The bug being fixed: all timestamps were 0 → frames grabbed at the blank
	// opening frame. Every timestamp must now be strictly inside (0, dur) and
	// strictly increasing.
	var prev float64 = -1
	for i, v := range ts {
		if v <= 0 || v >= dur {
			t.Errorf("ts[%d] = %v, want in (0, %v)", i, v, dur)
		}
		if v <= prev {
			t.Errorf("ts not strictly increasing at %d: %v <= %v", i, v, prev)
		}
		prev = v
	}
	// First and last should sit near the first/last scene regions, not at the edges.
	if ts[0] >= dur/float64(n) {
		t.Errorf("first ts %v should be within the first slice (< %v)", ts[0], dur/float64(n))
	}
}

func TestEvenFrameTimestamps_Degenerate(t *testing.T) {
	if ts := evenFrameTimestamps(0, 5); ts != nil {
		t.Errorf("duration 0 should yield nil, got %v", ts)
	}
	if ts := evenFrameTimestamps(10, 0); ts != nil {
		t.Errorf("n 0 should yield nil, got %v", ts)
	}
	if ts := evenFrameTimestamps(-3, 4); ts != nil {
		t.Errorf("negative duration should yield nil, got %v", ts)
	}
}
