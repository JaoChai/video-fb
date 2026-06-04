package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestBuildSceneSpecs(t *testing.T) {
	designs := []agent.SceneDesign{
		{
			SceneNumber:    1,
			LayoutVariant:  "hook_big",
			AccentColor:    "#ff6b2b",
			AnimationSpeed: "fast",
			Slots: []agent.Slot{
				{Role: "headline", Text: "ทำไม Pixel ถึง สำคัญ", Emphasis: []string{"Pixel"}},
				{Role: "step", Text: "ขั้นตอนแรก"},
				{Role: "step", Text: "ขั้นตอนสอง"},
			},
		},
		{
			SceneNumber:    2,
			LayoutVariant:  "list_steps",
			AccentColor:    "#2fd17a",
			AnimationSpeed: "normal",
			Slots: []agent.Slot{
				{Role: "body", Text: "รายละเอียด", Emphasis: []string{}},
			},
		},
	}
	bounds := []sceneBound{
		{Start: 0, End: 8.5},
		{Start: 8.5, End: 19.0},
	}
	bgMode := func(sceneNumber int) string {
		if sceneNumber == 1 {
			return "image"
		}
		return "css"
	}

	specs := buildSceneSpecs(designs, bounds, bgMode)

	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}

	// Scene 1: timing from bounds
	if specs[0].StartSec != 0 || specs[0].EndSec != 8.5 {
		t.Errorf("scene 1 timing: got [%.1f, %.1f], want [0, 8.5]", specs[0].StartSec, specs[0].EndSec)
	}
	if specs[0].LayoutVariant != "hook_big" {
		t.Errorf("scene 1 layout: got %q, want 'hook_big'", specs[0].LayoutVariant)
	}
	if specs[0].BackgroundMode != "image" {
		t.Errorf("scene 1 bgMode: got %q, want 'image'", specs[0].BackgroundMode)
	}

	// Scene 1: headline slot gets emphasis span
	if len(specs[0].Slots) != 3 {
		t.Fatalf("scene 1: expected 3 slots, got %d", len(specs[0].Slots))
	}
	headlineHTML := string(specs[0].Slots[0].HTML)
	if !strings.Contains(headlineHTML, `<span class="hl">`) {
		t.Errorf("headline HTML missing emphasis span: %q", headlineHTML)
	}
	if !strings.Contains(headlineHTML, "Pixel") {
		t.Errorf("headline HTML missing 'Pixel': %q", headlineHTML)
	}

	// Scene 1: step slots get sequential StepNum
	if specs[0].Slots[1].StepNum != 1 {
		t.Errorf("step 1 StepNum: got %d, want 1", specs[0].Slots[1].StepNum)
	}
	if specs[0].Slots[2].StepNum != 2 {
		t.Errorf("step 2 StepNum: got %d, want 2", specs[0].Slots[2].StepNum)
	}

	// Scene 2: timing wired from bounds
	if specs[1].StartSec != 8.5 || specs[1].EndSec != 19.0 {
		t.Errorf("scene 2 timing: got [%.1f, %.1f], want [8.5, 19.0]", specs[1].StartSec, specs[1].EndSec)
	}
	if specs[1].BackgroundMode != "css" {
		t.Errorf("scene 2 bgMode: got %q, want 'css'", specs[1].BackgroundMode)
	}

	// Mismatch guard: fewer bounds than designs → truncate to min
	shortBounds := []sceneBound{{Start: 0, End: 5}}
	truncated := buildSceneSpecs(designs, shortBounds, bgMode)
	if len(truncated) != 1 {
		t.Errorf("truncated: expected 1 spec, got %d", len(truncated))
	}
}

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
