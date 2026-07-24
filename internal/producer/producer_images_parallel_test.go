package producer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// Locks the parallel image-gen contract for the paths that must never hit kie:
// scenes with an empty ImagePrompt are skipped, and scenes whose bg file
// already exists (resume) are collected as-is. p.kie is nil, so any accidental
// GenerateImage call panics the test.
func TestGenerateSceneImagesParallel_SkipsAndReuses(t *testing.T) {
	clipDir := t.TempDir()

	// Scene 2's background already exists on disk (e.g. a resumed clip).
	existing := filepath.Join(clipDir, "bg-scene2.png")
	if err := os.WriteFile(existing, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, ImagePrompt: ""},       // no prompt → skipped entirely
		{SceneNumber: 2, ImagePrompt: "a city"}, // file exists → reused, no kie call
		{SceneNumber: 3},                        // no prompt → skipped
	}

	p := &Producer{} // kie == nil: a GenerateImage call would panic
	bgPaths := map[int]string{}
	p.generateSceneImagesParallel(context.Background(), scenes, StylePreset{}, "clip", clipDir, bgPaths, CaseInfo{})

	if len(bgPaths) != 1 {
		t.Fatalf("want only scene 2 collected, got %v", bgPaths)
	}
	if got := bgPaths[2]; got != existing {
		t.Fatalf("scene 2: want %s, got %s", existing, got)
	}
}
