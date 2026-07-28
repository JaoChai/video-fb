package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestChatPresetResolvesAndKeepsBrand(t *testing.T) {
	if PresetByKey("chat").Key != "chat" {
		t.Error("PresetByKey must resolve chat (resume path)")
	}
	if ChatPreset.Palette != Brand {
		t.Error("chat must keep the brand navy+orange palette")
	}
	if ChatPreset.BrandCSS() == "" {
		t.Error("chat BrandCSS must render")
	}
	if ChatPreset.HeadingFont.HeadingFamily == "" {
		t.Error("chat must set a heading font")
	}
}

func TestChatLayoutsAreAccepted(t *testing.T) {
	for _, l := range []string{"chat_in", "chat_out", "recap"} {
		if agent.ClampLayout(l) != l {
			t.Errorf("ClampLayout(%q) = %q, want it accepted", l, agent.ClampLayout(l))
		}
	}
}

func TestImageScenesForModeChatCapsAtOne(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "hook", ImagePrompt: "a marketer at night"},
		{SceneNumber: 2, Layout: "chat_in", ImagePrompt: "a phone screen"},
		{SceneNumber: 3, Layout: "chat_out", ImagePrompt: "another shot"},
	}
	got := imageScenesForMode(scenes, ModeChat)
	if len(got) != 1 || !got[1] {
		t.Errorf("chat mode must allow exactly scene 1, got %v", got)
	}
}

func TestChatCoverPromptIsPortraitNotForensic(t *testing.T) {
	out := promptForScene(
		agent.GeneratedScene{SceneNumber: 1, Layout: "hook", ImagePrompt: "a worried shop owner holding a phone"},
		ChatPreset, "clip-x", ModeChat)
	if !strings.Contains(out, "a worried shop owner holding a phone") {
		t.Errorf("cover prompt lost the subject: %s", out)
	}
	if !strings.Contains(out, "clip-x") {
		t.Error("cover prompt must keep the cohesion style-set token")
	}
	if strings.Contains(strings.ToLower(out), "forensic") {
		t.Error("chat cover must not inherit the case-file forensic anchor")
	}
}
