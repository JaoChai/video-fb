package producer

import (
	"testing"
)

func TestComputeBounds(t *testing.T) {
	t.Run("three scenes", func(t *testing.T) {
		got := computeBounds([]float64{8, 11, 5})
		want := []sceneBound{{0, 8}, {8, 19}, {19, 24}}
		if len(got) != len(want) {
			t.Fatalf("len=%d want %d", len(got), len(want))
		}
		for i, w := range want {
			if got[i] != w {
				t.Errorf("bounds[%d] = {%v,%v}, want {%v,%v}", i, got[i].Start, got[i].End, w.Start, w.End)
			}
		}
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		got := computeBounds(nil)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
		got = computeBounds([]float64{})
		if got != nil {
			t.Errorf("expected nil for empty slice, got %v", got)
		}
	})

	t.Run("zero-duration scene", func(t *testing.T) {
		// A scene with 0s duration: its window is a zero-width point, but
		// subsequent scenes still start at the right offset.
		got := computeBounds([]float64{5, 0, 3})
		want := []sceneBound{{0, 5}, {5, 5}, {5, 8}}
		if len(got) != len(want) {
			t.Fatalf("len=%d want %d", len(got), len(want))
		}
		for i, w := range want {
			if got[i] != w {
				t.Errorf("bounds[%d] = {%v,%v}, want {%v,%v}", i, got[i].Start, got[i].End, w.Start, w.End)
			}
		}
	})

	t.Run("single scene starts at zero", func(t *testing.T) {
		got := computeBounds([]float64{12.5})
		want := []sceneBound{{0, 12.5}}
		if len(got) != len(want) {
			t.Fatalf("len=%d want %d", len(got), len(want))
		}
		if got[0] != want[0] {
			t.Errorf("bounds[0] = {%v,%v}, want {%v,%v}", got[0].Start, got[0].End, want[0].Start, want[0].End)
		}
	})
}
