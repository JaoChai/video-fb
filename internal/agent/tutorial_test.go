package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func uistepScene(n int, panel string) GeneratedScene {
	return GeneratedScene{SceneNumber: n, Layout: "uistep", Content: json.RawMessage(panel)}
}

func TestUIStepIsASupportedLayout(t *testing.T) {
	if ClampLayout("uistep") != "uistep" {
		t.Error("uistep must survive ClampLayout")
	}
	if ClampLayout("ui_step") != "hero" {
		t.Error("unknown layouts must still clamp to hero")
	}
}

func TestClampUIState(t *testing.T) {
	for in, want := range map[string]string{
		"target": "target", "done": "done", "normal": "normal",
		"": "normal", "TARGET": "normal", "highlighted": "normal",
	} {
		if got := ClampUIState(in); got != want {
			t.Errorf("ClampUIState(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUIVocabViolationsAcceptsCatalogLabels(t *testing.T) {
	scenes := []GeneratedScene{uistepScene(1, `{"panel":{"chrome":"Ads Manager",
		"breadcrumb":"Rules › Create new rule",
		"items":[{"label":"Campaigns","state":"normal"},{"label":"Rules","state":"target"}],
		"field":{"label":"Cost per result","value":"400 THB"}}}`)}
	vocab := []string{"Ads Manager", "Rules", "Create new rule", "Campaigns", "Cost per result"}
	if v := UIVocabViolations(scenes, vocab); len(v) != 0 {
		t.Errorf("expected no violations, got %v", v)
	}
}

func TestUIVocabViolationsCatchesInventedLabel(t *testing.T) {
	scenes := []GeneratedScene{uistepScene(1, `{"panel":{"chrome":"Ads Manager",
		"items":[{"label":"Advanced Rules Manager","state":"target"}]}}`)}
	v := UIVocabViolations(scenes, []string{"Ads Manager", "Rules"})
	if len(v) != 1 || !strings.Contains(v[0], "Advanced Rules Manager") {
		t.Errorf("expected the invented label to be reported, got %v", v)
	}
}

func TestUIVocabViolationsNormalizesCaseAndSpace(t *testing.T) {
	scenes := []GeneratedScene{uistepScene(1, `{"panel":{"items":[{"label":"  cost  per result "}]}}`)}
	if v := UIVocabViolations(scenes, []string{"Cost per result"}); len(v) != 0 {
		t.Errorf("normalized match must pass, got %v", v)
	}
}

func TestUIVocabViolationsIgnoresNonUIStepScenes(t *testing.T) {
	scenes := []GeneratedScene{{SceneNumber: 1, Layout: "hero",
		Content: json.RawMessage(`{"title":"อะไรก็เขียนได้"}`)}}
	if v := UIVocabViolations(scenes, []string{"Rules"}); len(v) != 0 {
		t.Errorf("non-uistep scenes must not be validated, got %v", v)
	}
}

func TestUIVocabViolationsMalformedContentIsAViolation(t *testing.T) {
	scenes := []GeneratedScene{uistepScene(1, `{"panel": broken`)}
	if v := UIVocabViolations(scenes, []string{"Rules"}); len(v) == 0 {
		t.Error("malformed uistep content must be reported, not silently accepted")
	}
}

func TestUIVocabViolationsMissingPanelIsAViolation(t *testing.T) {
	scenes := []GeneratedScene{uistepScene(1, `{"title":"ขั้นที่ 1"}`)}
	if v := UIVocabViolations(scenes, []string{"Rules"}); len(v) == 0 {
		t.Error("a uistep scene with no panel must be reported")
	}
}

func TestCountUIStepScenes(t *testing.T) {
	scenes := []GeneratedScene{
		{Layout: "hook"}, uistepScene(2, `{}`), {Layout: "tip"}, uistepScene(4, `{}`), {Layout: "cta"},
	}
	if got := CountUIStepScenes(scenes); got != 2 {
		t.Errorf("CountUIStepScenes = %d, want 2", got)
	}
}

func TestTutorialBriefContainsEverythingTheModelNeeds(t *testing.T) {
	f := &models.TutorialFeature{
		DisplayNameTH: "ตั้ง Automated Rules",
		MenuPath:      []string{"Ads Manager", "Rules"},
		UIVocab:       []string{"Ads Manager", "Rules", "Cost per result"},
		Steps: []models.TutorialStep{
			{N: 1, TitleTH: "เปิดหน้าสร้างกฎ", ActionTH: "กด Rules", UITarget: "Rules"},
		},
		TrapTH:       "อย่าตั้ง Lifetime",
		WhyMattersTH: "งบวิ่งตอนบัญชีพัง",
	}
	b := TutorialBrief(f)
	for _, want := range []string{
		"ตั้ง Automated Rules", "Ads Manager", "Cost per result",
		"เปิดหน้าสร้างกฎ", "อย่าตั้ง Lifetime", "งบวิ่งตอนบัญชีพัง",
	} {
		if !strings.Contains(b, want) {
			t.Errorf("brief missing %q", want)
		}
	}
}

func TestTutorialBriefNilIsEmpty(t *testing.T) {
	if TutorialBrief(nil) != "" {
		t.Error("nil feature must render an empty brief (non-tutorial modes)")
	}
}

func TestSceneTemplateDataSubstitutesTutorialBrief(t *testing.T) {
	out, err := renderTemplate("script:{{.Script}}|brief:{{.TutorialBrief}}|dur:{{.TargetDurationSec}}",
		SceneTemplateData{Script: "S", TutorialBrief: "B", TargetDurationSec: 55})
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	if out != "script:S|brief:B|dur:55" {
		t.Errorf("got %q", out)
	}
}

// โหมดอื่นต้องได้ค่าว่าง ไม่ใช่ตัว placeholder ค้างใน prompt
func TestSceneTemplateDataEmptyBriefLeavesNoPlaceholder(t *testing.T) {
	out, _ := renderTemplate("A{{.TutorialBrief}}B", SceneTemplateData{})
	if out != "AB" {
		t.Errorf("got %q, want %q", out, "AB")
	}
}

func TestTutorialAvoidRepeatBlockEmpty(t *testing.T) {
	if got := TutorialAvoidRepeatBlock(nil); got != "" {
		t.Errorf("nil list must render empty block, got %q", got)
	}
	if got := TutorialAvoidRepeatBlock([]string{}); got != "" {
		t.Errorf("empty list must render empty block, got %q", got)
	}
}

func TestTutorialAvoidRepeatBlockListsAngles(t *testing.T) {
	angles := []string{
		"บัญชีโดนแบนเพราะไม่ตั้ง Automated Rules",
		"เจ้าของร้านคนนี้เกือบเสียเงินฟรีเพราะไม่รู้ทริคนี้",
	}
	b := TutorialAvoidRepeatBlock(angles)
	for _, want := range angles {
		if !strings.Contains(b, want) {
			t.Errorf("block missing previous angle %q, got %q", want, b)
		}
	}
	if !strings.Contains(b, "ขั้นตอน") || !strings.Contains(b, "ชื่อเมนู") {
		t.Errorf("block must warn against changing steps/menu names, got %q", b)
	}
}

func TestTutorialAvoidRepeatBlockNotMixedIntoTutorialBrief(t *testing.T) {
	f := &models.TutorialFeature{
		DisplayNameTH: "ตั้ง Automated Rules",
		MenuPath:      []string{"Ads Manager", "Rules"},
		UIVocab:       []string{"Ads Manager", "Rules"},
		Steps: []models.TutorialStep{
			{N: 1, TitleTH: "เปิดหน้าสร้างกฎ", ActionTH: "กด Rules", UITarget: "Rules"},
		},
		TrapTH:       "อย่าตั้ง Lifetime",
		WhyMattersTH: "งบวิ่งตอนบัญชีพัง",
	}
	priorAngle := "บัญชีโดนแบนเพราะไม่ตั้ง Automated Rules มุมเดิม"
	brief := TutorialBrief(f)
	if strings.Contains(brief, priorAngle) {
		t.Errorf("TutorialBrief must never contain avoid-repeat content, got %q", brief)
	}
	if strings.Contains(brief, "มุมที่เคยใช้กับฟีเจอร์นี้แล้ว") {
		t.Errorf("TutorialBrief must not include the avoid-repeat block on its own, got %q", brief)
	}
}
