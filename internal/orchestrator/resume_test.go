package orchestrator

import (
	"encoding/json"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func TestResumeAtRender(t *testing.T) {
	cases := map[string]bool{
		"":              false,
		"content_ready": true,
		"rendered":      true,
		"something":     false,
	}
	for stage, want := range cases {
		if got := resumeAtRender(stage); got != want {
			t.Errorf("resumeAtRender(%q) = %v, want %v", stage, got, want)
		}
	}
}

func TestScenesToGeneratedPreservesRenderFields(t *testing.T) {
	in := []models.Scene{{
		SceneNumber: 1, SceneType: "hero", TextContent: "tc", VoiceText: "vt",
		DurationSeconds: 5, LayoutVariant: "hero_big", OnScreenText: "HELLO",
		EmphasisWords: json.RawMessage(`["HELLO"]`), Beat: "b", CaptionStyle: "word_pop",
		ImagePrompt: "a cat", Layout: "hero", Content: json.RawMessage(`{"title":"Hi"}`),
	}}
	got := scenesToGenerated(in)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	g := got[0]
	if g.Layout != "hero" || string(g.Content) != `{"title":"Hi"}` {
		t.Errorf("Layout/Content dropped: %q / %s", g.Layout, g.Content)
	}
	if g.OnScreenText != "HELLO" || g.ImagePrompt != "a cat" || g.LayoutVariant != "hero_big" || g.CaptionStyle != "word_pop" {
		t.Errorf("render fields dropped: %+v", g)
	}
	if len(g.EmphasisWords) != 1 || g.EmphasisWords[0] != "HELLO" {
		t.Errorf("EmphasisWords not unmarshaled: %v", g.EmphasisWords)
	}
}
