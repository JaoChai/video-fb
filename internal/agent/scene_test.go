package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

// An empty scene list from the Director LLM must be rejected rather than passed
// downstream to render a blank video. Defense-in-depth alongside the script-side
// guard.
func TestValidateGeneratedScenes_Empty(t *testing.T) {
	if err := validateGeneratedScenes(nil); err == nil {
		t.Fatal("expected error for empty scenes, got nil")
	}
	if err := validateGeneratedScenes([]GeneratedScene{}); err == nil {
		t.Fatal("expected error for empty scene slice, got nil")
	}
}

func TestValidateGeneratedScenes_Valid(t *testing.T) {
	if err := validateGeneratedScenes([]GeneratedScene{{SceneNumber: 1}}); err != nil {
		t.Fatalf("expected nil for non-empty scenes, got %v", err)
	}
}

// The seeded `scene` prompt (migration 030) asks for a JSON array of objects with
// these exact fields. This test locks the prompt↔struct contract: if the JSON the
// LLM is told to emit ever stops unmarshalling into GeneratedScene, the field tags
// drifted and the pipeline would silently lose scene data.
func TestSceneOutputParsesSeededSchema(t *testing.T) {
	raw := `[
	  {
	    "scene_number": 1,
	    "beat": "hook",
	    "voice_text": "คุณรู้ไหมว่าบัญชีโฆษณาโดนแบนได้ใน 3 วินาที",
	    "on_screen_text": "โดนแบนใน 3 วิ?",
	    "emphasis_words": ["แบน", "3 วินาที"],
	    "layout_variant": "hook_big",
	    "caption_style": "word_pop",
	    "duration_seconds": 4.5,
	    "image_prompt": "dark navy gradient, orange accent"
	  },
	  {
	    "scene_number": 2,
	    "beat": "payoff",
	    "voice_text": "วิธีกันไว้ก่อนคือแยกบัญชีสำรอง",
	    "on_screen_text": "แยกบัญชีสำรอง",
	    "emphasis_words": ["สำรอง"],
	    "layout_variant": "phrase_block",
	    "caption_style": "phrase_block",
	    "duration_seconds": 6,
	    "image_prompt": ""
	  }
	]`

	var scenes []GeneratedScene
	if err := json.Unmarshal([]byte(raw), &scenes); err != nil {
		t.Fatalf("seeded scene JSON did not unmarshal into []GeneratedScene: %v", err)
	}
	if len(scenes) != 2 {
		t.Fatalf("len(scenes) = %d, want 2", len(scenes))
	}
	s0 := scenes[0]
	if s0.SceneNumber != 1 {
		t.Errorf("SceneNumber = %d, want 1", s0.SceneNumber)
	}
	if s0.Beat != "hook" {
		t.Errorf("Beat = %q, want hook", s0.Beat)
	}
	if s0.LayoutVariant != "hook_big" {
		t.Errorf("LayoutVariant = %q, want hook_big", s0.LayoutVariant)
	}
	if s0.CaptionStyle != "word_pop" {
		t.Errorf("CaptionStyle = %q, want word_pop", s0.CaptionStyle)
	}
	if s0.OnScreenText != "โดนแบนใน 3 วิ?" {
		t.Errorf("OnScreenText = %q", s0.OnScreenText)
	}
	if len(s0.EmphasisWords) != 2 || s0.EmphasisWords[0] != "แบน" {
		t.Errorf("EmphasisWords = %v, want [แบน 3 วินาที]", s0.EmphasisWords)
	}
	if s0.DurationSeconds != 4.5 {
		t.Errorf("DurationSeconds = %v, want 4.5", s0.DurationSeconds)
	}
	if s0.VoiceText == "" {
		t.Errorf("VoiceText is empty")
	}
}

// Locks the prompt↔struct contract for the themed scene prompt (migration 046):
// on_screen_text + emphasis_words must still unmarshal into GeneratedScene.
func TestSceneOutput_ThemedSchemaHasHookAndEmphasis(t *testing.T) {
	raw := `[{"scene_number":1,"layout":"hook","voice_text":"บัญชีโฆษณาโดนแบนถาวรเพราะอะไร",
	  "on_screen_text":"โดนแบนถาวรใน 3 วิ","emphasis_words":["โดนแบน"],
	  "caption_style":"word_pop","image_prompt":"a locked facebook ads dashboard",
	  "content":{"kicker":"ระวัง","rows":[{"t":"ยิงผิดกฎ","bad":true}]}}]`
	var scenes []GeneratedScene
	if err := json.Unmarshal([]byte(raw), &scenes); err != nil {
		t.Fatalf("themed scene JSON did not unmarshal: %v", err)
	}
	s := scenes[0]
	if s.Layout != "hook" || s.OnScreenText != "โดนแบนถาวรใน 3 วิ" {
		t.Errorf("hook fields drifted: layout=%q ost=%q", s.Layout, s.OnScreenText)
	}
	if len(s.EmphasisWords) == 0 {
		t.Errorf("emphasis_words must be present")
	}
	// on_screen_text ของ hook ต้อง <=7 คำ (นับ token คั่นด้วยช่องว่าง)
	if n := len(strings.Fields(s.OnScreenText)); n > 7 {
		t.Errorf("hook on_screen_text has %d words, want <=7", n)
	}
}

func TestBuildSceneThemeDescription(t *testing.T) {
	style := "flat illustration"
	theme := &models.BrandTheme{
		PrimaryColor: "#0f1d35",
		AccentColor:  "#ff6b2b",
		ImageStyle:   &style,
	}
	got := buildSceneThemeDescription(theme)
	if !strings.Contains(got, "#0f1d35") || !strings.Contains(got, "#ff6b2b") {
		t.Errorf("description missing brand colors: %q", got)
	}
	if !strings.Contains(got, "flat illustration") {
		t.Errorf("description missing image style: %q", got)
	}
}

// Renders a stand-in template with the four registry vars and confirms each is
// substituted — guards SceneTemplateData field names against the §4.6 registry.
func TestSceneTemplateRendersRegistryVars(t *testing.T) {
	tmpl := "dur={{.TargetDurationSec}} count={{.TargetSceneCount}} script={{.Script}} theme={{.ThemeDescription}}"
	out, err := renderTemplate(tmpl, SceneTemplateData{
		Script:            "SCRIPT",
		TargetSceneCount:  8,
		TargetDurationSec: 75,
		ThemeDescription:  "THEME",
	})
	if err != nil {
		t.Fatalf("renderTemplate err: %v", err)
	}
	for _, want := range []string{"dur=75", "count=8", "script=SCRIPT", "theme=THEME"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q: %s", want, out)
		}
	}
}
