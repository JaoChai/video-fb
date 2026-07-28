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
	for _, format := range []string{"tutorial", "basic"} {
		if got := clipMode(format); got != producer.ModeTutorial {
			t.Errorf("%s format = %q, want tutorial mode", format, got)
		}
	}
	for _, format := range []string{"qa", "tips", "news", "case_story"} {
		if got := clipMode(format); got != producer.ModeCase {
			t.Errorf("%s format = %q, want case mode", format, got)
		}
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

func TestTutorialArchetypeRejectsConsultQA(t *testing.T) {
	consult := &models.TitleArchetype{ArchetypeName: "consult_qa", Instruction: "รบกวนปรึกษา..."}
	if got := tutorialArchetype(consult); got != (models.TitleArchetype{}) {
		t.Errorf("consult_qa archetype must fall back to empty, got %+v", got)
	}
	if got := tutorialArchetype(nil); got != (models.TitleArchetype{}) {
		t.Errorf("nil archetype must fall back to empty, got %+v", got)
	}
	other := &models.TitleArchetype{ArchetypeName: "how_to", Instruction: "วิธีทำ..."}
	if got := tutorialArchetype(other); got != *other {
		t.Errorf("non-consult_qa archetype must pass through unchanged, got %+v want %+v", got, *other)
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

// preset เป็นตัวตัดสินหน้าตาคลิปจริง คลิป basic ที่หลุดไปเข้าสาขาแฟ้มคดีจะ render
// เป็น data-format="case" กิน case number ของ 21:00 และไม่ได้ภาพ AI เลย ทั้งที่
// prompt เป็นคู่มือ — ต้องผูกกับ clipMode ที่เดียวเสมอ
func TestPresetForCoversBasicAndTutorial(t *testing.T) {
	for _, format := range []string{tutorialFormatName, basicFormatName} {
		if got := presetFor(format); got.Key != producer.TutorialPreset.Key {
			t.Errorf("presetFor(%q) = %q, want the tutorial preset", format, got.Key)
		}
	}
	for _, format := range []string{"qa", "tips", "news", "case_story"} {
		if got := presetFor(format); got.Key != producer.CaseFilePreset.Key {
			t.Errorf("presetFor(%q) = %q, want case-file", format, got.Key)
		}
	}
	// retry แบบ rebuild เรียก presetFor ด้วย content_format ล้วน คีย์ธีมเก่าที่ยัง
	// ค้างใน clips.style_preset จึงย้อนกลับมาไม่ได้ ต่อให้แถวนั้นเก่ากว่าสองฟอร์แมต
	if got := producer.PresetByKey("neon-techno"); got.Key != producer.CaseFilePreset.Key {
		t.Errorf("legacy stored theme resolves to %q, want case-file", got.Key)
	}
}

// ตะแกรง ui_vocab ทำงานได้ก็ต่อเมื่อ retry โหลดแถวคลังกลับมา — ถ้า feat เป็น nil
// tutorialGateFailure คืน "" (ประตูเปิดค้าง) และ TutorialBrief ก็คืน "" แปลว่า agent
// เขียนขั้นตอนเมนูจากชื่อคลิปล้วนๆ แล้วคลิปนั้น auto-publish ขึ้น YouTube
func TestNeedsCatalogFeatureCoversBasic(t *testing.T) {
	for _, format := range []string{tutorialFormatName, basicFormatName} {
		if !needsCatalogFeature(format) {
			t.Errorf("needsCatalogFeature(%q) = false, want true — the ui_vocab gate must never be skipped", format)
		}
	}
	for _, format := range []string{"qa", "tips", "news", "case_story"} {
		if needsCatalogFeature(format) {
			t.Errorf("needsCatalogFeature(%q) = true, want false — non-catalog slots must be unchanged", format)
		}
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

// คลิป basic ต้องหน้าตาเหมือนคลิป tutorial เป๊ะ (โหมดภาพเดียวกัน) แต่ใช้พรอมป์
// คนละชุด ถ้าสองอย่างนี้ผูกติดกันเมื่อไหร่ ต้องเลือกอย่างใดอย่างหนึ่งเสียเสมอ
func TestBasicClipLooksLikeTutorialButUsesItsOwnPrompts(t *testing.T) {
	if got := clipMode(basicFormatName); got != producer.ModeTutorial {
		t.Errorf("clipMode(basic) = %q, want tutorial — visuals must be identical", got)
	}
	if got := agentModeFor(basicFormatName); got != basicAgentMode {
		t.Errorf("agentModeFor(basic) = %q, want %q — basic needs its own voice", got, basicAgentMode)
	}
	if got := agentModeFor(tutorialFormatName); got != producer.ModeTutorial {
		t.Errorf("agentModeFor(tutorial) = %q, want tutorial — must not change", got)
	}
	if got := agentModeFor("qa"); got != clipMode("qa") {
		t.Errorf("agentModeFor(qa) = %q, want the same as clipMode — other formats unchanged", got)
	}
}

// โหมด tutorial ยังต้องได้ผลเดิมทุกอย่างหลังเพิ่ม basic เข้ามา
func TestTutorialModeUnchangedByBasic(t *testing.T) {
	t.Setenv("CASE_FORMAT_ENABLED", "true")
	if got := clipMode(tutorialFormatName); got != producer.ModeTutorial {
		t.Errorf("tutorial = %q, want tutorial even with the case flag on", got)
	}
	if got := clipMode(basicFormatName); got != producer.ModeTutorial {
		t.Errorf("basic = %q, want tutorial even with the case flag on", got)
	}
}
