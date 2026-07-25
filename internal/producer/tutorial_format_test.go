package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestTutorialPresetNotInRandomPool(t *testing.T) {
	for _, p := range Presets {
		if p.Key == "tutorial" {
			t.Fatal("tutorial must NOT be in Presets (random pool)")
		}
	}
	if PresetByKey("tutorial").Key != "tutorial" {
		t.Error("PresetByKey must resolve tutorial (resume path)")
	}
	if TutorialPreset.BrandCSS() == "" {
		t.Error("tutorial BrandCSS must render")
	}
	if TutorialPreset.Palette != Brand {
		t.Error("tutorial must keep the brand navy+orange palette")
	}
}

func TestTutorialCoverPromptIsEditorialNotForensic(t *testing.T) {
	out := buildTutorialCoverPrompt("a phone showing a night alert", TutorialPreset, "clip-x")
	if !strings.Contains(out, "a phone showing a night alert") {
		t.Errorf("cover prompt lost the subject: %s", out)
	}
	if !strings.Contains(out, "clip-x") {
		t.Error("cover prompt must keep the cohesion style-set token")
	}
	if strings.Contains(strings.ToLower(out), "forensic") {
		t.Error("tutorial cover must not inherit the case-file forensic anchor")
	}
}

func TestImageScenesForModeTutorialCapsAtOne(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "hook", ImagePrompt: "a phone at night"},
		{SceneNumber: 2, Layout: "uistep", ImagePrompt: "should be ignored"},
		{SceneNumber: 3, Layout: "hero", ImagePrompt: "also ignored"},
	}
	allowed := imageScenesForMode(scenes, ModeTutorial)
	if len(allowed) != 1 || !allowed[1] {
		t.Errorf("tutorial must allow exactly scene 1, got %v", allowed)
	}
}

func TestImageScenesForModeTutorialNoPrompts(t *testing.T) {
	scenes := []agent.GeneratedScene{{SceneNumber: 1, Layout: "hook"}}
	allowed := imageScenesForMode(scenes, ModeTutorial)
	if allowed == nil {
		t.Fatal("tutorial must return an (empty) allow-set, never nil = unrestricted")
	}
	if len(allowed) != 0 {
		t.Errorf("no image prompts must yield an empty allow-set, got %v", allowed)
	}
}

func TestImageScenesForModeCaseUnchanged(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "casefile", ImagePrompt: "a desk"},
		{SceneNumber: 2, Layout: "evidence", ImagePrompt: "a jar"},
		{SceneNumber: 3, Layout: "evidence", ImagePrompt: "a card"},
	}
	allowed := imageScenesForMode(scenes, ModeCase)
	if len(allowed) != 2 || !allowed[1] || !allowed[2] {
		t.Errorf("case cap of 2 regressed: %v", allowed)
	}
}
