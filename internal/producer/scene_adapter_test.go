package producer

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestBuildSceneSpecs_MapsFieldsAndTiming(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, LayoutVariant: "hook_big", OnScreenText: "บัญชีโดนแบน",
			EmphasisWords: []string{"แบน"}, CaptionStyle: "word_pop", ImagePrompt: "a banned ad account"},
		{SceneNumber: 2, LayoutVariant: "quote_cta", OnScreenText: "ทักแอดส์แวนซ์",
			CaptionStyle: "phrase_block", ImagePrompt: ""},
	}
	bounds := []sceneBound{{Start: 0, End: 8}, {Start: 8, End: 20}}

	specs := buildSceneSpecs(scenes, bounds)
	if len(specs) != 2 {
		t.Fatalf("len = %d, want 2", len(specs))
	}

	s0 := specs[0]
	if s0.SceneNumber != 1 || s0.LayoutVariant != "hook_big" || s0.CaptionStyle != "word_pop" {
		t.Errorf("scene 0 fields wrong: %+v", s0)
	}
	if s0.StartSec != 0 || s0.EndSec != 8 {
		t.Errorf("scene 0 timing = [%v,%v], want [0,8]", s0.StartSec, s0.EndSec)
	}
	if s0.AccentColor != Brand.Orange {
		t.Errorf("scene 0 accent = %q, want %q", s0.AccentColor, Brand.Orange)
	}
	if s0.AnimationSpeed != "normal" {
		t.Errorf("scene 0 speed = %q, want normal", s0.AnimationSpeed)
	}
	if s0.BackgroundMode != "image" { // has image_prompt
		t.Errorf("scene 0 bgMode = %q, want image", s0.BackgroundMode)
	}
	if len(s0.Slots) != 1 || s0.Slots[0].Role != "headline" {
		t.Fatalf("scene 0 slots = %+v, want one headline", s0.Slots)
	}
	if !strings.Contains(string(s0.Slots[0].HTML), `<span class="hl">แบน</span>`) {
		t.Errorf("scene 0 headline missing emphasis span: %q", s0.Slots[0].HTML)
	}

	if specs[1].BackgroundMode != "css" { // empty image_prompt
		t.Errorf("scene 1 bgMode = %q, want css", specs[1].BackgroundMode)
	}
}

func TestBuildSceneSpecs_NormalizesLayoutAndCaption(t *testing.T) {
	cases := map[string]string{
		"hook_big": "hook_big", "hook_punch": "hook_punch", "list_steps": "list_steps",
		"stat_reveal": "stat_reveal", "compare_two": "compare_two", "quote_cta": "quote_cta",
		"phrase_block": "hook_big", "word_pop": "hook_big", "static": "hook_big",
		"intro": "hook_big", "outro": "hook_big", "": "hook_big", "garbage": "hook_big",
	}
	for in, want := range cases {
		specs := buildSceneSpecs(
			[]agent.GeneratedScene{{SceneNumber: 1, LayoutVariant: in, OnScreenText: "x", CaptionStyle: "weird"}},
			[]sceneBound{{0, 5}},
		)
		if specs[0].LayoutVariant != want {
			t.Errorf("layout %q normalized to %q, want %q", in, specs[0].LayoutVariant, want)
		}
		if specs[0].CaptionStyle != "phrase_block" {
			t.Errorf("caption %q not clamped to phrase_block", specs[0].CaptionStyle)
		}
	}
}

func TestBuildSceneSpecs_LengthMismatchAndEmpty(t *testing.T) {
	if got := buildSceneSpecs(nil, nil); got != nil {
		t.Errorf("empty input = %v, want nil", got)
	}
	scenes := []agent.GeneratedScene{{SceneNumber: 1, OnScreenText: "a"}, {SceneNumber: 2, OnScreenText: "b"}}
	specs := buildSceneSpecs(scenes, []sceneBound{{0, 5}})
	if len(specs) != 1 {
		t.Errorf("len = %d, want 1 (min of 2 scenes, 1 bound)", len(specs))
	}
}

func TestBuildSceneSpecs_EmptyOnScreenTextYieldsNoSlot(t *testing.T) {
	specs := buildSceneSpecs(
		[]agent.GeneratedScene{{SceneNumber: 1, OnScreenText: "  ", LayoutVariant: "hook_big"}},
		[]sceneBound{{0, 5}},
	)
	if len(specs) != 1 || len(specs[0].Slots) != 0 {
		t.Errorf("blank on_screen_text should yield 0 slots, got %+v", specs[0].Slots)
	}
}

func TestEntranceForSceneRotates(t *testing.T) {
	got := make([]string, 0, 7)
	for i := 0; i < 7; i++ {
		got = append(got, entranceForScene(i))
	}
	want := []string{"punch", "rise", "slide", "punch", "rise", "slide", "punch"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("idx %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEntranceForSceneNoConsecutiveRepeat(t *testing.T) {
	for i := 1; i < 30; i++ {
		if entranceForScene(i) == entranceForScene(i-1) {
			t.Fatalf("idx %d and %d have the same entrance %q", i-1, i, entranceForScene(i))
		}
	}
}

// ข้อความบนจอต้องถูก guard ทุกช่อง — Title ผ่าน highlightTitleStr มาก่อน
// จึงต้องยืนยันว่าแท็ก <span class="acc"> ยังอยู่ครบ
func TestBuildSceneContent_GuardsOnScreenText(t *testing.T) {
	s := agent.GeneratedScene{
		SceneNumber: 1,
		Layout:      "hero",
		Content:     json.RawMessage(`{"title":"เปิดแอดมินให้ครบ","cta":"ทักไอดี","sub":"เช็คเฟซบุ๊กก่อน"}`),
	}
	c := buildSceneContent(s, sceneBound{Start: 0, End: 5})

	if !strings.Contains(c.Title, zwsp+"แอดมิน"+zwsp) {
		t.Errorf("Title ไม่ถูก guard: %q", c.Title)
	}
	if !strings.Contains(c.CTA, zwsp+"ไอดี"+zwsp) {
		t.Errorf("CTA ไม่ถูก guard: %q", c.CTA)
	}
	if !strings.Contains(c.Sub, zwsp+"เฟซบุ๊ก"+zwsp) {
		t.Errorf("Sub ไม่ถูก guard: %q", c.Sub)
	}
}

// ป้ายเมนูใน uistep ต้องคงค่าเดิมทุกไบต์ — gate UIVocabViolations เทียบกับ
// catalog แบบตรงตัว ถ้าแทรก ZWSP เข้าไปคลิปจะถูกบล็อกทั้งใบ
func TestBuildSceneContent_LeavesUIPanelUntouched(t *testing.T) {
	s := agent.GeneratedScene{
		SceneNumber: 1,
		Layout:      "uistep",
		Content: json.RawMessage(`{"title":"เปิดแอดมิน",
			"panel":{"chrome":"Meta Business Suite","breadcrumb":"Settings",
			"items":[{"label":"Business settings","state":"target"}],
			"field":{"label":"ชื่อบัญชี","value":"Ads Vance"}}}`),
	}
	c := buildSceneContent(s, sceneBound{Start: 0, End: 5})

	if c.Panel == nil || len(c.Panel.Items) == 0 {
		t.Fatal("panel หายไป")
	}
	if got := c.Panel.Items[0].Label; got != "Business settings" {
		t.Errorf("ป้ายเมนูถูกแก้: %q", got)
	}
	if c.Panel.Field == nil {
		t.Fatal("field หายไป")
	}
	if got := c.Panel.Field.Value; got != "Ads Vance" {
		t.Errorf("ค่าในช่องถูกแก้: %q", got)
	}
}

// คำที่ไม่ได้อยู่ในรายการต้องไม่ถูกแตะ — กัน guard ไปยุ่งกับข้อความปกติ
func TestBuildSceneContent_LeavesPlainThaiUntouched(t *testing.T) {
	s := agent.GeneratedScene{
		SceneNumber: 1,
		Layout:      "hero",
		Content:     json.RawMessage(`{"title":"เปลี่ยนโดเมนแล้วอัดงบต่อทันที"}`),
	}
	c := buildSceneContent(s, sceneBound{Start: 0, End: 5})
	if strings.Contains(c.Title, zwsp) {
		t.Errorf("แทรก ZWSP ในข้อความที่ไม่มีคำทับศัพท์: %q", c.Title)
	}
}
