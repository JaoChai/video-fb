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

func TestCaseImageScenes(t *testing.T) {
	scenes := []agent.GeneratedScene{
		evScene(1, "casefile", "a dark desk with a handbag"),
		evScene(2, "evidence", "a cream jar"),
		evScene(3, "hero", "should be ignored"),
		evScene(4, "evidence", "a phone - over cap"),
	}
	if caseImageScenes(scenes, false) != nil {
		t.Error("classic mode must return nil (no restriction)")
	}
	allowed := caseImageScenes(scenes, true)
	if len(allowed) != 2 || !allowed[1] || !allowed[2] || allowed[4] {
		t.Errorf("allowed = %v, want cover scene 1 + evidence scene 2 (cap 2)", allowed)
	}
}

func TestBuildCoverPrompt(t *testing.T) {
	out := buildCoverPrompt("a designer handbag on a desk", CaseFilePreset, "clip-x")
	if !strings.Contains(out, "a designer handbag on a desk") || !strings.Contains(out, "UPPER half") {
		t.Errorf("cover prompt missing subject/upper-half rule: %s", out)
	}
	if strings.Contains(out, "single subject centered") {
		t.Error("cover prompt must not use the evidence centered composition")
	}
}

func TestPromptForSceneRouting(t *testing.T) {
	cover := promptForScene(evScene(1, "casefile", "a desk"), CaseFilePreset, "c", true)
	ev := promptForScene(evScene(2, "evidence", "a jar"), CaseFilePreset, "c", true)
	classic := promptForScene(evScene(3, "hero", "a graph"), CaseFilePreset, "c", false)
	if !strings.Contains(cover, "UPPER half") {
		t.Error("casefile scene must get the cover prompt")
	}
	if !strings.Contains(ev, "centered") {
		t.Error("evidence scene must get the evidence prompt")
	}
	if !strings.Contains(classic, "UPPER 55%") {
		t.Error("classic mode must keep the scene prompt")
	}
}

func TestCaseInfoZeroValueIsClassic(t *testing.T) {
	var ci CaseInfo
	if ci.Enabled || ci.CaseNumber != 0 {
		t.Error("zero CaseInfo must mean classic format")
	}
}
