package producer

import (
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testFontsDir returns a temp dir with a dummy .ttf so copyDir succeeds without
// depending on the on-disk PoC directory (which may not exist in CI).
func testFontsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dummy.ttf"), []byte("dummy font"), 0o644); err != nil {
		t.Fatalf("create dummy font: %v", err)
	}
	return dir
}

func TestBuildScenes(t *testing.T) {
	fontsDir := testFontsDir(t)

	// Create a temp voice file.
	voiceDir := t.TempDir()
	voicePath := filepath.Join(voiceDir, "voice.wav")
	if err := os.WriteFile(voicePath, []byte("RIFF----WAVEfmt "), 0o644); err != nil {
		t.Fatalf("create voice: %v", err)
	}

	projectDir := t.TempDir()

	params := ScenesParams{
		AspectRatio:     "9:16",
		BrandName:       "ADS VANCE",
		CategoryLabel:   "PIXEL",
		QuestionerName:  "คุณป๊อบ",
		Kicker:          "CAPI & PIXEL",
		VoiceSrc:        "will-be-overwritten",
		DurationSeconds: 30,
		Scenes: []SceneSpec{
			{
				SceneNumber:    1,
				LayoutVariant:  "hook_big",
				AccentColor:    "#ff6b2b",
				AnimationSpeed: "normal",
				StartSec:       0,
				EndSec:         15,
				BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "headline", HTML: template.HTML("บัญชีโดนแบน")},
				},
				Content: SceneContent{SceneNumber: 1, Start: 0, End: 15, Layout: "hero", CaptionStyle: "phrase_block", Title: "บัญชีโดนแบน"},
			},
			{
				SceneNumber:    2,
				LayoutVariant:  "quote_cta",
				AccentColor:    "#2fd17a",
				AnimationSpeed: "normal",
				StartSec:       15,
				EndSec:         30,
				BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "headline", HTML: template.HTML("ทักแอดส์แวนซ์")},
				},
				Content: SceneContent{SceneNumber: 2, Start: 15, End: 30, Layout: "cta", CaptionStyle: "phrase_block", Title: "ทักแอดส์แวนซ์", CTA: "ทักเลย", Brand: "ADS VANCE"},
			},
		},
		Segments: []TranscriptSegment{{Text: "ทดสอบ", Start: 0, End: 2}},
	}

	b := NewCompositionBuilder(fontsDir)
	got, err := b.BuildScenes(params, "clip-abc-123", projectDir, voicePath, map[int]string{})
	if err != nil {
		t.Fatalf("BuildScenes: %v", err)
	}

	if got != projectDir {
		t.Errorf("returned dir = %q, want %q", got, projectDir)
	}

	// All expected files must exist.
	for _, rel := range []string{
		"index.html",
		"package.json",
		"hyperframes.json",
		"meta.json",
		filepath.Join("assets", "voice.wav"),
	} {
		path := filepath.Join(projectDir, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file missing: %s (%v)", rel, err)
		}
	}

	// index.html must be non-empty and contain the headline text from scene 1.
	htmlBytes, err := os.ReadFile(filepath.Join(projectDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if len(htmlBytes) == 0 {
		t.Error("index.html is empty")
	}
	if !strings.Contains(string(htmlBytes), "บัญชีโดนแบน") {
		t.Error("index.html missing scene 1 headline text")
	}

	// meta.json must contain the clipID.
	metaBytes, err := os.ReadFile(filepath.Join(projectDir, "meta.json"))
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}
	var meta map[string]string
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("unmarshal meta.json: %v", err)
	}
	if meta["id"] != "clip-abc-123" {
		t.Errorf("meta.json id = %q, want %q", meta["id"], "clip-abc-123")
	}

	// Caller's params.Scenes must not be mutated.
	if params.VoiceSrc != "will-be-overwritten" {
		t.Error("BuildScenes mutated caller's params.VoiceSrc")
	}
	if params.Scenes[0].BackgroundMode != "css" {
		t.Error("BuildScenes mutated caller's params.Scenes")
	}
}

func TestBuildScenesCopiesAudio(t *testing.T) {
	projectDir := t.TempDir()
	// A throwaway voice + ambient source file.
	clipDir := t.TempDir()
	voice := filepath.Join(clipDir, "voice.wav")
	if err := os.WriteFile(voice, []byte("RIFFfake"), 0o644); err != nil {
		t.Fatal(err)
	}
	amb := filepath.Join(clipDir, "ambient.mp3")
	if err := os.WriteFile(amb, []byte("ID3fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	sfx := SfxTransitionFiles()
	if len(sfx) == 0 {
		t.Skip("no sfx embedded")
	}
	params := ScenesParams{
		AspectRatio:      "9:16",
		BrandName:        "Ads Vance",
		DurationSeconds:  10,
		VoiceSrc:         "assets/voice.wav",
		AmbientLocalPath: amb,
		TransitionCues:   []TransitionCue{{Name: sfx[0], AtSec: 3.2}},
		AudioMotion:      true,
		Scenes: []SceneSpec{{
			SceneNumber: 1, StartSec: 0, EndSec: 10, BackgroundMode: "css",
			Content: SceneContent{SceneNumber: 1, Start: 0, End: 10, Layout: "hero", Title: "Hi"},
		}},
	}
	b := NewCompositionBuilder(testFontsDir(t))
	if _, err := b.BuildScenes(params, "clipX", projectDir, voice, nil); err != nil {
		t.Fatalf("BuildScenes: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "assets", "ambient.mp3")); err != nil {
		t.Errorf("ambient.mp3 not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "assets", "sfx", sfx[0])); err != nil {
		t.Errorf("sfx %s not copied: %v", sfx[0], err)
	}
}
