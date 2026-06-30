package producer

import "testing"

func TestAmbientAndSfxFilesNonEmpty(t *testing.T) {
	if len(AmbientFiles()) == 0 {
		t.Error("AmbientFiles empty")
	}
	if len(SfxTransitionFiles()) == 0 {
		t.Error("SfxTransitionFiles empty")
	}
}

func TestPickAmbientAvoidsLast(t *testing.T) {
	files := AmbientFiles()
	if len(files) < 2 {
		t.Skip("need >=2 ambient beds to test avoid-last")
	}
	last := files[0]
	// rng always returns 0 → would pick candidates[0]; candidates exclude `last`.
	got := PickAmbient(last, func(int) int { return 0 })
	if got == last {
		t.Errorf("PickAmbient returned the avoided last %q", got)
	}
}

func TestPickAmbientSingleOrEmpty(t *testing.T) {
	// With an unknown last and a forced index, it still returns a real file.
	if got := PickAmbient("", func(int) int { return 0 }); got == "" {
		t.Error("PickAmbient returned empty with assets present")
	}
}

func TestPickTransitionSFXCount(t *testing.T) {
	if got := PickTransitionSFX(0, func(int) int { return 0 }); len(got) != 0 {
		t.Errorf("n=0 want 0 cues, got %d", len(got))
	}
	got := PickTransitionSFX(5, func(int) int { return 0 })
	if len(got) != 5 {
		t.Errorf("n=5 want 5 cues, got %d", len(got))
	}
	for _, name := range got {
		if name == "" {
			t.Error("empty sfx name in result")
		}
	}
}
