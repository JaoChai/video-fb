package orchestrator

import (
	"math"
	"testing"
)

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

// Each sample must fall strictly inside its own scene's [start,end) window even
// when scene durations are wildly unequal — this is what stops samples landing on
// scene transitions (blank crossfade frames).
func TestSceneAwareTimestamps_LandsInsideEachScene(t *testing.T) {
	durs := []float64{3.92, 10.68, 9.16, 9.72, 8.32, 12.56, 12, 12.88, 10.68, 16.8}
	var total float64
	for _, d := range durs {
		total += d
	}
	ts := sceneAwareTimestamps(durs, total, qaSceneFrac) // probed == sum → scale 1
	if len(ts) != len(durs) {
		t.Fatalf("want %d timestamps, got %d", len(durs), len(ts))
	}
	var start float64
	for i, d := range durs {
		end := start + d
		if ts[i] <= start || ts[i] >= end {
			t.Errorf("scene %d: ts %.3f not inside (%.3f, %.3f)", i, ts[i], start, end)
		}
		start = end
	}
}

// When the probed duration differs from the sum of estimates, every sample must
// scale by probed/sum so it still lands in the right place on the real encode.
func TestSceneAwareTimestamps_RescalesToProbed(t *testing.T) {
	durs := []float64{10, 30, 10} // sum 50
	ts := sceneAwareTimestamps(durs, 100, 0.6) // scale 2.0
	want := []float64{
		(0 + 10*0.6) * 2,  // 12
		(10 + 30*0.6) * 2, // 56
		(40 + 10*0.6) * 2, // 92
	}
	if len(ts) != len(want) {
		t.Fatalf("want %d, got %d", len(want), len(ts))
	}
	for i := range want {
		if math.Abs(ts[i]-want[i]) > 1e-6 {
			t.Errorf("i%d: want %.3f got %.3f", i, want[i], ts[i])
		}
	}
}

// No usable durations (all zero) → nil, so the caller can fall back.
func TestSceneAwareTimestamps_ZeroDurationsNil(t *testing.T) {
	if ts := sceneAwareTimestamps([]float64{0, 0, 0}, 30, 0.6); ts != nil {
		t.Errorf("want nil for zero durations, got %v", ts)
	}
}
