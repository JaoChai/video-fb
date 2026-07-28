package producer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func warroomParams() ScenesParams {
	mk := func(n int, layout string, c SceneContent) SceneSpec {
		c.SceneNumber, c.Layout = n, layout
		c.Start, c.End = float64(n-1)*5, float64(n)*5
		return SceneSpec{SceneNumber: n, StartSec: c.Start, EndSec: c.End,
			LayoutVariant: "hook_big", CaptionStyle: "phrase_block", Content: c}
	}
	return ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 30, Format: "warroom", ThemeKey: "warroom",
		Scenes: []SceneSpec{
			mk(1, "dashboard", SceneContent{
				StatLabel:       "CPM 7 วันล่าสุด",
				Chips:           []ContentChip{{N: "+38%", T: "CPM", Bad: true}, {N: "1.4x", T: "ROAS"}},
				Callout:         "ค่าโฆษณาแพงขึ้น 38% ใน 3 วัน",
				BackgroundImage: "assets/bg-scene1.png"}),
			mk(2, "uistep", SceneContent{
				Num: "1", Of: "ขั้นที่ 1 / 2", StepTotal: 2, Title: "เปิดกฎอัตโนมัติ",
				Panel: &ContentUIPanel{
					Chrome: "Ads Manager", Breadcrumb: "Rules › Create new rule",
					Items: []ContentUIItem{
						{Label: "Campaigns", State: "normal"},
						{Label: "Automated Rules", State: "target"},
					},
					Field: &ContentUIField{Label: "Greater than", Value: "400 THB"},
				},
				Callout: "เลือก Automated Rules"}),
			mk(3, "alarm", SceneContent{Title: "ตั้งเพดานไว้ก่อนนอน",
				Rows: []ContentRow{{Text: "CPM ทะลุเพดาน = หยุดแอดอัตโนมัติ"},
					{Text: "ไม่ตั้ง = จ่ายทั้งคืน", Bad: true}}}),
		},
		Segments: []TranscriptSegment{{Text: "ค่าโฆษณาแพงขึ้น", Start: 0, End: 2}},
	}
}

func TestRenderWarRoomFormat(t *testing.T) {
	out, err := RenderCompositionScenes(warroomParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{
		`data-format="warroom"`,
		"const FORMAT_WARROOM = true",
		`data-layout="dashboard"`,
		`data-layout="alarm"`,
		"CPM 7 วันล่าสุด",
		"ค่าโฆษณาแพงขึ้น 38% ใน 3 วัน",
		"wr-kpi",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered warroom HTML missing %q", want)
		}
	}
}

// หัวใจของสเปก: ห้องควบคุมต้องยังเรนเดอร์ uistep ด้วย markup เดิม เพราะตะแกรง
// ui_vocab กับตัวนับขั้นตอนนับเฉพาะซีน uistep — ถ้าหายไปคลิปสอนผิดจะหลุดขึ้นช่อง
func TestWarRoomStillRendersUIStep(t *testing.T) {
	out, _ := RenderCompositionScenes(warroomParams())
	html := string(out)
	for _, want := range []string{`data-layout="uistep"`, "ui-panel", "Automated Rules", "ui-rail"} {
		if !strings.Contains(html, want) {
			t.Errorf("warroom must keep the uistep walkthrough intact — missing %q", want)
		}
	}
}

// KPI ที่แย่ต้องมีธง bad ติดไปถึง JSON ไม่งั้นเลขแดงหายทั้งโหมด
func TestWarRoomKPICarriesBadFlag(t *testing.T) {
	out, _ := RenderCompositionScenes(warroomParams())
	if !strings.Contains(string(out), `"bad":true`) {
		t.Error("SCENES JSON must carry the bad flag for a failing KPI")
	}
}

// TestDumpWarRoomHTML — ด่านสายตาเดียวกับฝั่งแชท ดูว่ากรอบจอมอนิเตอร์ไม่ทับ uistep
func TestDumpWarRoomHTML(t *testing.T) {
	dir := os.Getenv("HF_KEEP_DIR")
	if dir == "" {
		t.Skip("set HF_KEEP_DIR=<dir> to dump the war-room HTML for eyeballing")
	}
	out, err := RenderCompositionScenes(warroomParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "warroom.html"), out, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Logf("wrote %s/warroom.html", dir)
}
