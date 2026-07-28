package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestWarRoomPresetResolvesAndKeepsBrand(t *testing.T) {
	if PresetByKey("warroom").Key != "warroom" {
		t.Error("PresetByKey must resolve warroom (resume path)")
	}
	if WarRoomPreset.Palette != Brand {
		t.Error("warroom must keep the brand navy+orange palette")
	}
	if WarRoomPreset.BrandCSS() == "" {
		t.Error("warroom BrandCSS must render")
	}
}

func TestWarRoomLayoutsAreAccepted(t *testing.T) {
	for _, l := range []string{"dashboard", "alarm"} {
		if agent.ClampLayout(l) != l {
			t.Errorf("ClampLayout(%q) = %q, want it accepted", l, agent.ClampLayout(l))
		}
	}
}

// ห้องควบคุมเป็นคลิปสอน — uistep ต้องยังใช้ได้ ไม่งั้นตะแกรง ui_vocab เปิดค้าง
func TestWarRoomKeepsUIStepLayout(t *testing.T) {
	if agent.ClampLayout("uistep") != "uistep" {
		t.Fatal("uistep must remain a valid layout — the tutorial gate counts it")
	}
}

func TestImageScenesForModeWarRoomCapsAtOne(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "dashboard", ImagePrompt: "a night control room"},
		{SceneNumber: 2, Layout: "uistep", ImagePrompt: "a menu"},
	}
	got := imageScenesForMode(scenes, ModeWarRoom)
	if len(got) != 1 || !got[1] {
		t.Errorf("warroom mode must allow exactly scene 1, got %v", got)
	}
}

func TestWarRoomCoverPromptHasNoReadableScreens(t *testing.T) {
	out := promptForScene(
		agent.GeneratedScene{SceneNumber: 1, Layout: "dashboard", ImagePrompt: "a desk with three glowing monitors"},
		WarRoomPreset, "clip-y", ModeWarRoom)
	if !strings.Contains(out, "a desk with three glowing monitors") {
		t.Errorf("cover prompt lost the subject: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "no readable") {
		t.Error("warroom cover must forbid readable screen content (AI renders fake Thai as garbage)")
	}
}
