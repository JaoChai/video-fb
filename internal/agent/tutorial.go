package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)

// UIPanelItem is one row inside the simulated Ads Manager panel of a uistep scene.
type UIPanelItem struct {
	Label string `json:"label"`
	State string `json:"state"` // normal|target|done
}

// UIPanelField is the highlighted input of a uistep scene ("Greater than: 400 THB").
type UIPanelField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// UIPanel is the simulated screen of a uistep scene. Every English string in it
// must come from the catalog's ui_vocab — see UIVocabViolations.
type UIPanel struct {
	Chrome     string        `json:"chrome"`
	Breadcrumb string        `json:"breadcrumb"`
	Items      []UIPanelItem `json:"items"`
	Field      *UIPanelField `json:"field,omitempty"`
}

// uistepContent is the parse target for a uistep scene's content object.
type uistepContent struct {
	Panel   *UIPanel `json:"panel"`
	Callout string   `json:"callout"`
	Title   string   `json:"title"`
	Num     string   `json:"num"`
	Of      string   `json:"of"`
}

// ClampUIState clamps an LLM panel-item state to the three the template styles;
// anything else (including "") becomes "normal".
func ClampUIState(s string) string {
	switch s {
	case "target", "done", "normal":
		return s
	default:
		return "normal"
	}
}

// normalizeUILabel makes vocabulary comparison forgiving about case and spacing
// but nothing else — "cost  per result" matches "Cost per result", while
// "Advanced Rules Manager" never matches "Rules".
func normalizeUILabel(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// breadcrumbParts splits "Rules › Create new rule" into its individual labels.
// Both the typographic and the ASCII separators are accepted.
func breadcrumbParts(s string) []string {
	f := strings.FieldsFunc(s, func(r rune) bool { return r == '›' || r == '>' || r == '/' })
	out := make([]string, 0, len(f))
	for _, p := range f {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// UIVocabViolations returns a human-readable description of every UI string in
// the scenes' uistep panels that is NOT in vocab. An empty result means every
// menu name on screen came from the catalog.
//
// This is the deterministic half of the tutorial gate: it does not ask an LLM
// whether a menu exists, it checks against the catalog a human curated. A
// uistep scene whose content JSON does not parse is itself a violation — a
// silently-dropped panel would ship a tutorial with a blank step.
func UIVocabViolations(scenes []GeneratedScene, vocab []string) []string {
	allowed := make(map[string]bool, len(vocab))
	for _, v := range vocab {
		if n := normalizeUILabel(v); n != "" {
			allowed[n] = true
		}
	}

	var out []string
	check := func(sceneNo int, field, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		if !allowed[normalizeUILabel(value)] {
			out = append(out, fmt.Sprintf("scene %d %s: %q not in ui_vocab", sceneNo, field, value))
		}
	}

	for _, s := range scenes {
		if ClampLayout(s.Layout) != "uistep" {
			continue
		}
		var c uistepContent
		if err := json.Unmarshal(s.Content, &c); err != nil {
			out = append(out, fmt.Sprintf("scene %d: uistep content is not valid JSON: %v", s.SceneNumber, err))
			continue
		}
		if c.Panel == nil {
			out = append(out, fmt.Sprintf("scene %d: uistep has no panel object", s.SceneNumber))
			continue
		}
		check(s.SceneNumber, "chrome", c.Panel.Chrome)
		for _, p := range breadcrumbParts(c.Panel.Breadcrumb) {
			check(s.SceneNumber, "breadcrumb", p)
		}
		for _, it := range c.Panel.Items {
			check(s.SceneNumber, "item", it.Label)
		}
		if c.Panel.Field != nil {
			check(s.SceneNumber, "field", c.Panel.Field.Label)
		}
	}
	return out
}

// CountUIStepScenes reports how many scenes render the uistep layout. The
// tutorial gate requires this to equal the catalog's step count, so a clip can
// never silently drop or invent a step.
func CountUIStepScenes(scenes []GeneratedScene) int {
	n := 0
	for _, s := range scenes {
		if ClampLayout(s.Layout) == "uistep" {
			n++
		}
	}
	return n
}

// TutorialBrief renders the catalog row as the Thai prompt block the script and
// scene agents read. Returns "" for a nil feature so non-tutorial modes render
// an empty {{.TutorialBrief}} substitution.
func TutorialBrief(f *models.TutorialFeature) string {
	if f == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## ข้อมูลฟีเจอร์ที่ต้องสอน (ใช้ได้เฉพาะข้อมูลในบล็อกนี้ ห้ามเพิ่มชื่อเมนูเอง)\n")
	b.WriteString("ชื่อฟีเจอร์: " + f.DisplayNameTH + "\n")
	b.WriteString("ความเสียหายที่ฟีเจอร์นี้กันได้ (ใช้เป็นวัตถุดิบของ hook): " + f.WhyMattersTH + "\n")
	b.WriteString("เส้นทางเมนูจริง: " + strings.Join(f.MenuPath, " › ") + "\n")
	b.WriteString(fmt.Sprintf("จำนวนขั้นตอน: %d (ต้องมีซีน layout \"uistep\" เท่ากับจำนวนนี้พอดี)\n", len(f.Steps)))
	b.WriteString("\nขั้นตอน:\n")
	for _, s := range f.Steps {
		line := fmt.Sprintf("%d) %s — %s [ไฮไลต์: %s]", s.N, s.TitleTH, s.ActionTH, s.UITarget)
		if s.ValueTH != "" {
			line += " [ค่าที่ใส่: " + s.ValueTH + "]"
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\nกับดักที่คนพลาด (ใช้เป็นซีน re-hook กลางคลิป): " + f.TrapTH + "\n")
	b.WriteString("\nคำศัพท์ UI ที่อนุญาตให้ปรากฏบนจอได้เท่านั้น (ห้ามใช้คำอื่นเด็ดขาด):\n- ")
	b.WriteString(strings.Join(f.UIVocab, "\n- "))
	b.WriteString("\n")
	return b.String()
}
