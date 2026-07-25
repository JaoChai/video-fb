package orchestrator

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
)

func TestClipModeFromContentFormat(t *testing.T) {
	t.Setenv("CASE_FORMAT_ENABLED", "true")
	if got := clipMode("tutorial"); got != producer.ModeTutorial {
		t.Errorf("tutorial format = %q, want tutorial mode even while the case flag is on", got)
	}
	if got := clipMode("qa"); got != producer.ModeCase {
		t.Errorf("qa with case flag on = %q, want case", got)
	}
	t.Setenv("CASE_FORMAT_ENABLED", "")
	if got := clipMode("qa"); got != producer.ModeClassic {
		t.Errorf("qa with case flag off = %q, want classic", got)
	}
	if got := clipMode("tutorial"); got != producer.ModeTutorial {
		t.Errorf("tutorial must not depend on the case flag, got %q", got)
	}
}

func TestTutorialSceneShape(t *testing.T) {
	for _, tc := range []struct{ steps, scenes, dur int }{
		{3, 8, 55}, {4, 9, 64}, {5, 10, 73},
	} {
		sc, d := tutorialSceneShape(tc.steps)
		if sc != tc.scenes || d != tc.dur {
			t.Errorf("steps=%d -> (%d scenes, %ds), want (%d, %d)", tc.steps, sc, d, tc.scenes, tc.dur)
		}
	}
}

func gateFeature() *models.TutorialFeature {
	return &models.TutorialFeature{
		FeatureKey: "f",
		UIVocab:    []string{"Ads Manager", "Rules", "Cost per result"},
		Steps: []models.TutorialStep{
			{N: 1, UITarget: "Rules"}, {N: 2, UITarget: "Cost per result"},
		},
	}
}

func gateScene(n int, label string) agent.GeneratedScene {
	return agent.GeneratedScene{SceneNumber: n, Layout: "uistep",
		Content: json.RawMessage(`{"panel":{"chrome":"Ads Manager","items":[{"label":"` + label + `","state":"target"}]}}`)}
}

func TestTutorialGatePassesCleanClip(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "hook"},
		gateScene(2, "Rules"), gateScene(3, "Cost per result"),
		{SceneNumber: 4, Layout: "cta"},
	}
	if msg := tutorialGateFailure(scenes, gateFeature()); msg != "" {
		t.Errorf("clean clip must pass, got %q", msg)
	}
}

func TestTutorialGateBlocksInventedMenu(t *testing.T) {
	scenes := []agent.GeneratedScene{
		gateScene(1, "Advanced Rules Manager"), gateScene(2, "Cost per result"),
	}
	msg := tutorialGateFailure(scenes, gateFeature())
	if !strings.Contains(msg, "Advanced Rules Manager") {
		t.Errorf("gate must name the invented label, got %q", msg)
	}
}

func TestTutorialGateBlocksWrongStepCount(t *testing.T) {
	scenes := []agent.GeneratedScene{gateScene(1, "Rules")} // 1 uistep, catalog says 2
	msg := tutorialGateFailure(scenes, gateFeature())
	if !strings.Contains(msg, "1") || !strings.Contains(msg, "2") {
		t.Errorf("gate must report got/want step counts, got %q", msg)
	}
}

func TestTutorialGateNilFeatureIsNoOp(t *testing.T) {
	if msg := tutorialGateFailure([]agent.GeneratedScene{{Layout: "hero"}}, nil); msg != "" {
		t.Errorf("non-tutorial clips must never be gated, got %q", msg)
	}
}

// retryPresetForCurrentMode ต้องเลือก preset ตาม content_format ของคลิป ไม่ใช่ตาม flag
// อย่างเดียว ไม่งั้น retry ของคลิป tutorial จะได้ prompt คู่มือ + หน้าตาแฟ้มคดี
func TestRetryPresetFollowsClipFormat(t *testing.T) {
	t.Setenv("CASE_FORMAT_ENABLED", "true")
	if got := retryPresetForCurrentMode("tutorial", "tutorial"); got.Key != producer.TutorialPreset.Key {
		t.Errorf("tutorial clip retry preset = %q, want tutorial even with the case flag on", got.Key)
	}
	if got := retryPresetForCurrentMode("case-file", "qa"); got.Key != producer.CaseFilePreset.Key {
		t.Errorf("case clip retry preset = %q, want case-file", got.Key)
	}
	t.Setenv("CASE_FORMAT_ENABLED", "")
	// flag rolled back mid-life: a stored case/tutorial preset must not leak into a classic clip
	if got := retryPresetForCurrentMode("case-file", "qa"); got.Key != producer.PresetByKey("").Key {
		t.Errorf("stored case preset with flag off = %q, want the classic default", got.Key)
	}
	if got := retryPresetForCurrentMode("tutorial", "qa"); got.Key != producer.PresetByKey("").Key {
		t.Errorf("stored tutorial preset on a non-tutorial clip = %q, want the classic default", got.Key)
	}
}

// การเช็คความสดเป็นตาข่ายกันพลาด ไม่ใช่ประตู — พังทางไหนก็ต้องแปลว่า "ไม่เก่า"
// ไม่งั้น LLM ล่มครั้งเดียวจะ park ฟีเจอร์ไปเรื่อยๆ จน catalog หมดแล้วผลิตไม่ได้
func TestFreshnessDecisionFailsOpen(t *testing.T) {
	if stale, _ := freshnessDecision(agent.FreshnessVerdict{Moved: true, Reason: "moved"},
		errors.New("llm down")); stale {
		t.Error("an error must never park a feature")
	}
	if stale, _ := freshnessDecision(agent.FreshnessVerdict{}, nil); stale {
		t.Error("moved=false must not park a feature")
	}
}

func TestFreshnessDecisionParksOnExplicitMove(t *testing.T) {
	stale, reason := freshnessDecision(
		agent.FreshnessVerdict{Moved: true, Reason: "Rules moved under Automated Rules"}, nil)
	if !stale || !strings.Contains(reason, "Automated Rules") {
		t.Errorf("stale=%v reason=%q, want parked with the reason kept", stale, reason)
	}
}

func TestFreshnessDecisionAlwaysHasAReason(t *testing.T) {
	stale, reason := freshnessDecision(agent.FreshnessVerdict{Moved: true}, nil)
	if !stale || strings.TrimSpace(reason) == "" {
		t.Errorf("a parked feature must carry a reason for the human who fixes it, got %q", reason)
	}
}
