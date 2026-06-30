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
