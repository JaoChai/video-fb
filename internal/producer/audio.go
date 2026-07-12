package producer

import (
	"io/fs"
	"os"
	"sort"
	"strings"
)

const (
	ambientDir = "assets/audio/ambient"
	sfxDir     = "assets/audio/sfx/transition"
)

// AudioMotionEnabled reports whether the audio + motion upgrade is on. Off ⇒ the
// hyperframes path renders voice-only with today's motion (see presets.go pattern).
func AudioMotionEnabled() bool { return os.Getenv("AUDIO_MOTION_ENABLED") == "true" }

// SceneMotionV2Enabled turns on the v2 scene motion (mid-scene parallax drift,
// entrance variety, stat count-up). Off → current motion behavior.
func SceneMotionV2Enabled() bool { return os.Getenv("SCENE_MOTION_V2_ENABLED") == "true" }

// CoverSceneEnabled turns on the frame-0 cover: scene index 0 renders fully at
// opacity:1 from frame 0 (the poster the platform grabs) instead of fading in
// from a blank navy frame. Off ⇒ today's behavior.
func CoverSceneEnabled() bool { return os.Getenv("COVER_SCENE_ENABLED") == "true" }

// PipelineFastEnabled turns on the fast pipeline: parallel per-scene image gen,
// fail-fast image timeouts (75s, no retry → css fallback), and parallel visual
// QA. Off → current sequential behavior.
func PipelineFastEnabled() bool { return os.Getenv("PIPELINE_FAST_ENABLED") == "true" }

func listAudio(dir string) []string {
	entries, err := fs.ReadDir(audioAssetsFS, dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".mp3") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// AmbientFiles returns the embedded ambient bed base names, sorted.
func AmbientFiles() []string { return listAudio(ambientDir) }

// SfxTransitionFiles returns the embedded transition SFX base names, sorted.
func SfxTransitionFiles() []string { return listAudio(sfxDir) }

// PickAmbient chooses an ambient bed at random, excluding lastName when more than
// one exists so two clips in a row never share a bed. rng(n) returns an int in
// [0,n) — pass rand.Intn in production. Returns "" when no beds are embedded.
func PickAmbient(lastName string, rng func(int) int) string {
	files := AmbientFiles()
	if len(files) == 0 {
		return ""
	}
	if len(files) == 1 {
		return files[0]
	}
	candidates := make([]string, 0, len(files))
	for _, f := range files {
		if f != lastName {
			candidates = append(candidates, f)
		}
	}
	if len(candidates) == 0 {
		return files[0]
	}
	return candidates[rng(len(candidates))]
}

// PickTransitionSFX returns n SFX base names (one per scene boundary). When n
// exceeds the pool size, names repeat. Returns an empty slice for n<=0 or when no
// SFX are embedded.
func PickTransitionSFX(n int, rng func(int) int) []string {
	if n <= 0 {
		return nil
	}
	files := SfxTransitionFiles()
	if len(files) == 0 {
		return nil
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, files[rng(len(files))])
	}
	return out
}
