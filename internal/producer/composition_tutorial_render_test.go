package producer

import (
	"os"
	"strings"
	"testing"
)

func tutorialParams() ScenesParams {
	mk := func(n int, layout string, c SceneContent) SceneSpec {
		c.SceneNumber, c.Layout = n, layout
		c.Start, c.End = float64(n-1)*5, float64(n)*5
		return SceneSpec{SceneNumber: n, StartSec: c.Start, EndSec: c.End,
			LayoutVariant: "hook_big", CaptionStyle: "phrase_block", Content: c}
	}
	step := func(num string) SceneContent {
		return SceneContent{
			Num: num, Of: "ขั้นที่ " + num + " / 3", StepTotal: 3,
			Title: "ตั้งเงื่อนไขให้ตัดงบ",
			Panel: &ContentUIPanel{
				Chrome: "Ads Manager", Breadcrumb: "Rules › Create new rule",
				Items: []ContentUIItem{
					{Label: "Campaigns", State: "normal"},
					{Label: "Ad sets", State: "done"},
					{Label: "Cost per result", State: "target"},
				},
				Field: &ContentUIField{Label: "Greater than", Value: "400 THB"},
			},
			Callout: "เลือก Cost per result แล้วใส่ 400 บาท",
		}
	}
	return ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 30, Format: "tutorial", ThemeKey: "tutorial",
		Scenes: []SceneSpec{
			mk(1, "hook", SceneContent{Rows: []ContentRow{{Text: "ตีสามบัญชีโดนปิด งบยังวิ่ง"}},
				BackgroundImage: "assets/bg-scene1.png"}),
			mk(2, "uistep", step("1")),
			mk(3, "uistep", step("2")),
			mk(4, "uistep", step("3")),
			mk(5, "cta", SceneContent{Title: "ทำครั้งเดียวจบ", CTA: "ทักมาเช็ค", Brand: "ADS VANCE"}),
		},
		Segments: []TranscriptSegment{{Text: "ตีสามบัญชีโดนปิด", Start: 0, End: 2}},
	}
}

func TestRenderTutorialFormat(t *testing.T) {
	out, err := RenderCompositionScenes(tutorialParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{
		`data-format="tutorial"`,
		"const FORMAT_TUTORIAL = true",
		"ui-panel", "ui-chrome", "ui-crumb", "ui-item", "ui-field", "ui-rail", "ui-callout",
		"Cost per result", "Ads Manager", "400 THB",
		"เลือก Cost per result แล้วใส่ 400 บาท",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q", want)
		}
	}
	// กันบทเรียน blank-video regression: "-->" ใน <script> ทำให้ JS พังทั้งคลิป
	if strings.Contains(html[strings.Index(html, "<script>"):], "-->") {
		t.Error("inline script must never contain the sequence minus-minus-gt")
	}
	// กัน tofu: ห้ามใช้ glyph ที่ฟอนต์ bundle ไม่มีเป็น CSS content
	for _, glyph := range []string{`content:"✓"`, `content:"◀"`, `content:"●"`} {
		if strings.Contains(html, glyph) {
			t.Errorf("template must not use %s — bundled fonts render it as tofu", glyph)
		}
	}
}

func TestRenderCaseFormatUnaffectedByTutorialBlock(t *testing.T) {
	out, err := RenderCompositionScenes(caseParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if !strings.Contains(html, `data-format="case"`) {
		t.Error("case format regressed")
	}
	if strings.Contains(html, "const FORMAT_TUTORIAL = true") {
		t.Error("case clips must not enable the tutorial renderer")
	}
}

// TestDumpTutorialHTML เขียนผลลัพธ์ไว้ให้คนเปิดดูด้วยตา ไม่ใช่ assertion
func TestDumpTutorialHTML(t *testing.T) {
	out, err := RenderCompositionScenes(tutorialParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := os.MkdirAll("../../render-test-out", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile("../../render-test-out/tutorial-preview.html", out, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Log("wrote render-test-out/tutorial-preview.html")
}
