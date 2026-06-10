package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestBuildSceneSpecs_MapsFieldsAndTiming(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, LayoutVariant: "hook_big", OnScreenText: "บัญชีโดนแบน",
			EmphasisWords: []string{"แบน"}, CaptionStyle: "word_pop", ImagePrompt: "a banned ad account"},
		{SceneNumber: 2, LayoutVariant: "quote_cta", OnScreenText: "ทักแอดส์แวนซ์",
			CaptionStyle: "phrase_block", ImagePrompt: ""},
	}
	bounds := []sceneBound{{Start: 0, End: 8}, {Start: 8, End: 20}}

	specs := buildSceneSpecs(scenes, bounds)
	if len(specs) != 2 {
		t.Fatalf("len = %d, want 2", len(specs))
	}

	s0 := specs[0]
	if s0.SceneNumber != 1 || s0.LayoutVariant != "hook_big" || s0.CaptionStyle != "word_pop" {
		t.Errorf("scene 0 fields wrong: %+v", s0)
	}
	if s0.StartSec != 0 || s0.EndSec != 8 {
		t.Errorf("scene 0 timing = [%v,%v], want [0,8]", s0.StartSec, s0.EndSec)
	}
	if s0.AccentColor != Brand.Orange {
		t.Errorf("scene 0 accent = %q, want %q", s0.AccentColor, Brand.Orange)
	}
	if s0.AnimationSpeed != "normal" {
		t.Errorf("scene 0 speed = %q, want normal", s0.AnimationSpeed)
	}
	if s0.BackgroundMode != "image" { // has image_prompt
		t.Errorf("scene 0 bgMode = %q, want image", s0.BackgroundMode)
	}
	if len(s0.Slots) != 1 || s0.Slots[0].Role != "headline" {
		t.Fatalf("scene 0 slots = %+v, want one headline", s0.Slots)
	}
	if !strings.Contains(string(s0.Slots[0].HTML), `<span class="hl">แบน</span>`) {
		t.Errorf("scene 0 headline missing emphasis span: %q", s0.Slots[0].HTML)
	}

	if specs[1].BackgroundMode != "css" { // empty image_prompt
		t.Errorf("scene 1 bgMode = %q, want css", specs[1].BackgroundMode)
	}
}

func TestBuildSceneSpecs_NormalizesLayoutAndCaption(t *testing.T) {
	cases := map[string]string{
		"hook_big": "hook_big", "hook_punch": "hook_punch", "list_steps": "list_steps",
		"stat_reveal": "stat_reveal", "compare_two": "compare_two", "quote_cta": "quote_cta",
		"phrase_block": "hook_big", "word_pop": "hook_big", "static": "hook_big",
		"intro": "hook_big", "outro": "hook_big", "": "hook_big", "garbage": "hook_big",
	}
	for in, want := range cases {
		specs := buildSceneSpecs(
			[]agent.GeneratedScene{{SceneNumber: 1, LayoutVariant: in, OnScreenText: "x", CaptionStyle: "weird"}},
			[]sceneBound{{0, 5}},
		)
		if specs[0].LayoutVariant != want {
			t.Errorf("layout %q normalized to %q, want %q", in, specs[0].LayoutVariant, want)
		}
		if specs[0].CaptionStyle != "phrase_block" {
			t.Errorf("caption %q not clamped to phrase_block", specs[0].CaptionStyle)
		}
	}
}

func TestBuildSceneSpecs_LengthMismatchAndEmpty(t *testing.T) {
	if got := buildSceneSpecs(nil, nil); got != nil {
		t.Errorf("empty input = %v, want nil", got)
	}
	scenes := []agent.GeneratedScene{{SceneNumber: 1, OnScreenText: "a"}, {SceneNumber: 2, OnScreenText: "b"}}
	specs := buildSceneSpecs(scenes, []sceneBound{{0, 5}})
	if len(specs) != 1 {
		t.Errorf("len = %d, want 1 (min of 2 scenes, 1 bound)", len(specs))
	}
}

func TestBuildSceneSpecs_EmptyOnScreenTextYieldsNoSlot(t *testing.T) {
	specs := buildSceneSpecs(
		[]agent.GeneratedScene{{SceneNumber: 1, OnScreenText: "  ", LayoutVariant: "hook_big"}},
		[]sceneBound{{0, 5}},
	)
	if len(specs) != 1 || len(specs[0].Slots) != 0 {
		t.Errorf("blank on_screen_text should yield 0 slots, got %+v", specs[0].Slots)
	}
}
