package producer

import (
	"strings"
	"testing"
)

func caseParams() ScenesParams {
	mk := func(n int, layout string, c SceneContent) SceneSpec {
		c.SceneNumber, c.Layout = n, layout
		c.Start, c.End = float64(n-1)*4, float64(n)*4
		return SceneSpec{SceneNumber: n, StartSec: c.Start, EndSec: c.End,
			LayoutVariant: "hook_big", CaptionStyle: "phrase_block", Content: c}
	}
	return ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 20, Format: "case", CaseNumber: 91,
		ThemeKey: "case-file",
		Scenes: []SceneSpec{
			mk(1, "casefile", SceneContent{Title: "คดีบัญชีฟาร์มปลิว",
				Rows: []ContentRow{{Text: "ความเสียหาย: 30,000"}}, Stamp: "ด่วนที่สุด"}),
			mk(2, "comic", SceneContent{Panels: []ContentPanel{
				{Time: "วันที่ 1", T: "เปิดแอด"}, {Time: "คืนวันที่ 2", T: "ถูกปิด", Dark: true}}}),
			mk(3, "evidence", SceneContent{Kicker: "หลักฐานชิ้นที่ 1",
				Stamp: "REJECTED", Sub: "ครีเอทีฟชุดเดิม", BackgroundImage: "assets/bg-scene3.png"}),
			mk(4, "board", SceneContent{Kicker: "ผังสาเหตุ",
				Rows: []ContentRow{{Text: "บัญชีใหม่"}, {Text: "งบกระโดด"}}}),
			mk(5, "verdict", SceneContent{Title: "ออกแบบใหม่ + วอร์มบัญชี",
				Stamp: "ปิดคดี - รอดได้", CTA: "ส่งเคสมาเลย", Brand: "ADS VANCE"}),
		},
		Segments: []TranscriptSegment{{Text: "เปิดแฟ้มคดี", Start: 0, End: 2}},
	}
}

func TestRenderCaseFormat(t *testing.T) {
	out, err := RenderCompositionScenes(caseParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{
		`data-format="case"`,
		"cf-folder", "cf-panel", "cf-polaroid", "cf-note", "cf-stamp-green",
		`คดีที่ 91`, // Go-injected case number
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(html[strings.Index(html, "<script>"):], "-->") {
		// "-->" หลัง <script> แรก = อันตราย (html/template ตัดบรรทัด)
		t.Error("inline script must never contain the sequence minus-minus-gt")
	}
}

func TestRenderClassicFormatUnchanged(t *testing.T) {
	p := caseParams()
	p.Format, p.CaseNumber = "", 0
	out, err := RenderCompositionScenes(p)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(out), `data-format="case"`) {
		t.Error("classic render must not carry data-format=case")
	}
	if strings.Contains(string(out), "คดีที่ 91") {
		t.Error("classic render must not inject a case number")
	}
}
