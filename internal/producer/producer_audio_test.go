package producer

import "testing"

func TestTransitionCuesAtBoundaries(t *testing.T) {
	specs := []SceneSpec{
		{SceneNumber: 1, StartSec: 0, EndSec: 3},
		{SceneNumber: 2, StartSec: 3, EndSec: 6},
		{SceneNumber: 3, StartSec: 6, EndSec: 9},
	}
	names := []string{"a.mp3", "b.mp3"} // one per boundary (scenes-1 = 2)
	cues := buildTransitionCues(specs, names)
	if len(cues) != 2 {
		t.Fatalf("want 2 cues (boundaries), got %d", len(cues))
	}
	// Cue fires slightly before the incoming scene start.
	if cues[0].AtSec < 2.5 || cues[0].AtSec > 3.0 {
		t.Errorf("cue0 AtSec=%v, want just before 3.0", cues[0].AtSec)
	}
	if cues[0].Name != "a.mp3" || cues[1].Name != "b.mp3" {
		t.Errorf("cue names mismatch: %+v", cues)
	}
}

func TestTransitionCuesSingleScene(t *testing.T) {
	if cues := buildTransitionCues([]SceneSpec{{SceneNumber: 1, StartSec: 0, EndSec: 5}}, nil); len(cues) != 0 {
		t.Errorf("single scene → 0 boundaries, got %d", len(cues))
	}
}

func TestTransitionCuesEdges(t *testing.T) {
	// 1. AtSec clamping: StartSec=0.1 minus 0.2 lead = -0.1, must clamp to 0.
	specs2 := []SceneSpec{
		{SceneNumber: 1, StartSec: 0, EndSec: 3},
		{SceneNumber: 2, StartSec: 0.1, EndSec: 6},
	}
	cues := buildTransitionCues(specs2, []string{"a.mp3"})
	if len(cues) != 1 {
		t.Fatalf("clamp test: want 1 cue, got %d", len(cues))
	}
	if cues[0].AtSec != 0 {
		t.Errorf("clamp test: AtSec=%v, want 0 (clamped from -0.1)", cues[0].AtSec)
	}

	// 2. Early-break on short names: 3 scenes (2 boundaries) but only 1 name → 1 cue.
	specs3 := []SceneSpec{
		{SceneNumber: 1, StartSec: 0, EndSec: 3},
		{SceneNumber: 2, StartSec: 3, EndSec: 6},
		{SceneNumber: 3, StartSec: 6, EndSec: 9},
	}
	cues2 := buildTransitionCues(specs3, []string{"b.mp3"})
	if len(cues2) != 1 {
		t.Errorf("short-names test: want 1 cue, got %d", len(cues2))
	}
}
