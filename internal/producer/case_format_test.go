package producer

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestCaseFormatEnabled(t *testing.T) {
	t.Setenv("CASE_FORMAT_ENABLED", "")
	if CaseFormatEnabled() {
		t.Error("must be off by default")
	}
	t.Setenv("CASE_FORMAT_ENABLED", "true")
	if !CaseFormatEnabled() {
		t.Error("must be on when env=true")
	}
}

func TestCaseFilePresetNotInRandomPool(t *testing.T) {
	for _, p := range Presets {
		if p.Key == "case-file" {
			t.Fatal("case-file must NOT be in Presets (random pool)")
		}
	}
	if PresetByKey("case-file").Key != "case-file" {
		t.Error("PresetByKey must resolve case-file (resume path)")
	}
	if PresetByKey("unknown-key").Key != "editorial-bold" {
		t.Error("unknown key must still fall back to editorial-bold")
	}
	if CaseFilePreset.BrandCSS() == "" {
		t.Error("case-file BrandCSS must render")
	}
}

func TestBuildEvidencePrompt(t *testing.T) {
	out := buildEvidencePrompt("a cream jar", CaseFilePreset, "clip-x")
	if !strings.Contains(out, "a cream jar") || !strings.Contains(out, "centered") {
		t.Errorf("evidence prompt missing subject/composition: %s", out)
	}
	if strings.Contains(out, "UPPER 55%") {
		t.Error("evidence prompt must not reserve lower frame (image sits in polaroid)")
	}
	if !strings.Contains(out, "clip-x") {
		t.Error("evidence prompt must keep the cohesion style-set token")
	}
}

func evScene(n int, layout, imgPrompt string) agent.GeneratedScene {
	return agent.GeneratedScene{SceneNumber: n, Layout: layout, ImagePrompt: imgPrompt,
		Content: json.RawMessage(`{}`)}
}

func TestEvidenceImageScenes(t *testing.T) {
	scenes := []agent.GeneratedScene{
		evScene(1, "casefile", "should be ignored"),
		evScene(2, "evidence", "a cream jar"),
		evScene(3, "hero", "should be ignored"),
		evScene(4, "evidence", "a phone"),
		evScene(5, "evidence", "a third thing - over cap"),
	}
	if evidenceImageScenes(scenes, false) != nil {
		t.Error("classic mode must return nil (no restriction)")
	}
	allowed := evidenceImageScenes(scenes, true)
	if len(allowed) != 2 || !allowed[2] || !allowed[4] || allowed[5] {
		t.Errorf("allowed = %v, want scenes 2 and 4 only (cap 2)", allowed)
	}
}
