package producer

import "testing"

// boundsToDurations must recover each scene's real rendered length from the
// measured voice bounds, so the caller can persist accurate durations instead
// of the scene agent's always-zero estimate.
func TestBoundsToDurations(t *testing.T) {
	bounds := []sceneBound{{Start: 0, End: 8}, {Start: 8, End: 19}, {Start: 19, End: 24}}
	got := boundsToDurations(bounds)
	want := []float64{8, 11, 5}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("durations[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestBoundsToDurations_Empty(t *testing.T) {
	if got := boundsToDurations(nil); len(got) != 0 {
		t.Errorf("nil bounds should yield empty, got %v", got)
	}
}
