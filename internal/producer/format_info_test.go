package producer

import (
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestFormatInfoModes(t *testing.T) {
	var zero FormatInfo
	if zero.IsCase() || zero.IsTutorial() {
		t.Error("zero FormatInfo must be classic (neither case nor tutorial)")
	}
	if !(FormatInfo{Mode: ModeCase}).IsCase() {
		t.Error("Mode=case must report IsCase")
	}
	if !(FormatInfo{Mode: ModeTutorial}).IsTutorial() {
		t.Error("Mode=tutorial must report IsTutorial")
	}
	if (FormatInfo{Mode: ModeTutorial}).IsCase() {
		t.Error("tutorial must never report IsCase")
	}
}

func TestImageScenesForModeClassicUnrestricted(t *testing.T) {
	scenes := []agent.GeneratedScene{{SceneNumber: 1, Layout: "hero", ImagePrompt: "a graph"}}
	if imageScenesForMode(scenes, ModeClassic) != nil {
		t.Error("classic mode must return nil = no restriction")
	}
}
