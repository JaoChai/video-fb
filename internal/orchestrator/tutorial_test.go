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
	for _, tc := range []struct{ format, want string }{
		{"basic", producer.ModeTutorial},
		{"tutorial", producer.ModeWarRoom},
		{"qa", producer.ModeChat},
		{"tips", producer.ModeChat},
		{"case_story", producer.ModeCase},
		{"news", producer.ModeCase},
		{"ไม่รู้จัก", producer.ModeCase},
	} {
		if got := clipMode(tc.format); got != tc.want {
			t.Errorf("clipMode(%q) = %q, want %q", tc.format, got, tc.want)
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

// preset เป็นตัวตัดสินหน้าตาคลิปจริง ทุก content_format ต้องได้ preset ของโหมดตัวเอง
func TestPresetForCoversAllFourModes(t *testing.T) {
	for _, tc := range []struct{ format, wantKey string }{
		{"basic", "tutorial"},
		{"tutorial", "warroom"},
		{"qa", "chat"},
		{"tips", "chat"},
		{"case_story", "case-file"},
		{"news", "case-file"},
	} {
		if got := presetFor(tc.format); got.Key != tc.wantKey {
			t.Errorf("presetFor(%q) = %q, want %q", tc.format, got.Key, tc.wantKey)
		}
	}
}

// คลิปสอนทั้งสอง slot ต้องผ่านคลังหัวข้อเสมอ ไม่งั้นตะแกรง ui_vocab ไม่มีคำอนุญาตให้เทียบ
func TestNeedsCatalogFeatureCoversBothTeachingSlots(t *testing.T) {
	for _, format := range []string{tutorialFormatName, basicFormatName} {
		if !needsCatalogFeature(format) {
			t.Errorf("needsCatalogFeature(%q) = false, want true", format)
		}
	}
	for _, format := range []string{"qa", "tips", "news", "case_story"} {
		if needsCatalogFeature(format) {
			t.Errorf("needsCatalogFeature(%q) = true, want false", format)
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
	// 21:00 ย้ายไปโหมดห้องควบคุมแล้ว พรอมป์จึงต้องเป็นชุด warroom (migration 073)
	// ไม่ใช่ชุด tutorial ที่ตอนนี้เป็นของ 15:00 คนเดียว
	if got := agentModeFor(tutorialFormatName); got != producer.ModeWarRoom {
		t.Errorf("agentModeFor(tutorial) = %q, want warroom", got)
	}
	if got := agentModeFor("qa"); got != clipMode("qa") {
		t.Errorf("agentModeFor(qa) = %q, want the same as clipMode — other formats unchanged", got)
	}
}

// สอง slot สอนต้องแยกหน้าตากันจริง แต่ต้องผ่านคลังหัวข้อเหมือนกันทั้งคู่ —
// ถ้าวันหนึ่งมีคนทำให้สองอันนี้ไปโหมดเดียวกัน คลิปวันละ 4 รอบจะเหลือ 3 หน้าตา
func TestBothTeachingSlotsDifferInLookButShareTheGate(t *testing.T) {
	if got := clipMode(basicFormatName); got != producer.ModeTutorial {
		t.Errorf("clipMode(basic) = %q, want tutorial", got)
	}
	if got := clipMode(tutorialFormatName); got != producer.ModeWarRoom {
		t.Errorf("clipMode(tutorial) = %q, want warroom", got)
	}
	for _, f := range []string{basicFormatName, tutorialFormatName} {
		if !needsCatalogFeature(f) {
			t.Errorf("needsCatalogFeature(%q) = false — ตะแกรง ui_vocab จะเปิดค้างทั้งรอบ", f)
		}
	}
}
