package producer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// realClipScenes loads the EXACT scene rows a production clip was rendered from
// (dumped out of prod Postgres) so a layout issue can be reproduced locally without
// spending an LLM/image credit re-producing the clip.
//
// testdata/clip_61a5f36c_scenes.json = clip 61a5f36c-a5cb-4cd5-98e3-059315bb270b
// (tutorial format, feature automated_rules_cpm_guard, 4 steps) — the clip that
// landed in needs_review with the layout inspector flagged.
func realClipScenes(t *testing.T, name string) []agent.GeneratedScene {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var scenes []agent.GeneratedScene
	if err := json.Unmarshal(raw, &scenes); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(scenes) == 0 {
		t.Fatal("fixture has no scenes")
	}
	return scenes
}

// realClipBounds rebuilds the per-scene timeline from the persisted durations.
// Production measures these from the synthesized voice; the persisted
// scenes.duration_seconds IS that measurement, so replaying it cumulatively
// reproduces the same scene windows the render used.
func realClipBounds(scenes []agent.GeneratedScene) []sceneBound {
	bounds := make([]sceneBound, len(scenes))
	at := 0.0
	for i, s := range scenes {
		bounds[i] = sceneBound{Start: at, End: at + s.DurationSeconds}
		at = bounds[i].End
	}
	return bounds
}

// realTutorialParams assembles ScenesParams the same way AssembleHyperframes916
// does for a tutorial-mode clip: TutorialPreset, Format "tutorial", the same
// spec/caption builders, and the cover/motion flags read from the environment.
func realTutorialParams(t *testing.T) ScenesParams {
	t.Helper()
	scenes := realClipScenes(t, "clip_61a5f36c_scenes.json")
	bounds := realClipBounds(scenes)
	specs := buildSceneSpecs(scenes, bounds)
	if len(specs) == 0 {
		t.Fatal("buildSceneSpecs returned empty")
	}
	preset := TutorialPreset
	return ScenesParams{
		AspectRatio:     "9:16",
		BrandName:       BrandName,
		CTAText:         BrandCTA,
		VoiceSrc:        "assets/voice.wav",
		DurationSeconds: bounds[len(bounds)-1].End,
		Scenes:          specs,
		Segments:        captionSegmentsFromScenes(scenes, bounds),
		Palette:         preset.Palette,
		BrandCSS:        preset.BrandCSS(),
		ThemeKey:        preset.Key,
		Motion:          preset.Motion,
		Format:          "tutorial",
		MotionV2:        SceneMotionV2Enabled(),
		Cover:           CoverSceneEnabled(),
	}
}

// TestInspectRealTutorialClip rebuilds the real project directory for the
// flagged clip and runs the same `hyperframes inspect` gate production runs, so
// the inspector names the element it objected to. Guarded by HF_RENDER=1 (needs
// Node + Chromium); set HF_KEEP_DIR to keep the project for eyeballing.
func TestInspectRealTutorialClip(t *testing.T) {
	if os.Getenv("HF_RENDER") != "1" {
		t.Skip("set HF_RENDER=1 to run the Hyperframes inspect harness")
	}

	params := realTutorialParams(t)

	dir := t.TempDir()
	if keep := os.Getenv("HF_KEEP_DIR"); keep != "" {
		dir = keep
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	voiceSrc := os.Getenv("HF_VOICE_SRC")
	if voiceSrc == "" {
		voiceSrc = "../../hyperframes-poc/poc-video/assets/voice.wav"
	}

	// Scene 1 carries an image_prompt, so production rendered it with a real
	// cover photo. Supply a stand-in of the same aspect (HF_BG_SRC to override);
	// without one BuildScenes downgrades the scene to a css background, which
	// changes the layout the inspector measures.
	bgPaths := map[int]string{}
	bgSrc := os.Getenv("HF_BG_SRC")
	if bgSrc == "" {
		bgSrc = "../../hyperframes-poc/poc-video/assets/background-9x16.png"
	}
	if fileExists(bgSrc) {
		for _, s := range params.Scenes {
			if s.BackgroundMode == "image" {
				bgPaths[s.SceneNumber] = bgSrc
			}
		}
	} else {
		t.Logf("no background stand-in at %s — image scenes fall back to css", bgSrc)
	}

	builder := NewCompositionBuilder("assets/fonts")
	if _, err := builder.BuildScenes(params, "61a5f36c-a5cb-4cd5-98e3-059315bb270b", dir, voiceSrc, bgPaths); err != nil {
		t.Fatalf("build scenes: %v", err)
	}
	t.Logf("project dir: %s", dir)

	if _, err := NewHyperframesRenderer().Inspect(context.Background(), dir); err != nil {
		t.Errorf("inspect flagged the layout (this is the reproduction):\n%v", err)
	}
}
