package producer

import (
	"html/template"
	"strings"
	"testing"
)

func sampleScenesParams(aspect string) ScenesParams {
	return ScenesParams{
		AspectRatio: aspect, BrandName: "ADS VANCE", CategoryLabel: "PIXEL",
		QuestionerName: "คุณป๊อบ", Kicker: "CAPI & PIXEL", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 45,
		IntroMascot:     "assets/mascot/rocket.png",
		OutroMascot:     "assets/mascot/wave.png",
		CTAText:         "กดติดตาม ไม่พลาดทุกอัปเดตแอด",
		Scenes: []SceneSpec{
			{SceneNumber: 1, LayoutVariant: "hook_big", AccentColor: "#F0A030", AnimationSpeed: "normal",
				StartSec: 0, EndSec: 15, BackgroundMode: "css", CaptionStyle: "word_pop",
				Slots:   []SlotSpec{{Role: "headline", HTML: template.HTML("บัญชีโดนแบน")}},
				Content: SceneContent{SceneNumber: 1, Start: 0, End: 15, Layout: "hero", CaptionStyle: "word_pop", Title: "บัญชีโดนแบน"}},
			{SceneNumber: 2, LayoutVariant: "list_steps", AccentColor: "#F0A030", AnimationSpeed: "normal",
				StartSec: 15, EndSec: 32, BackgroundMode: "css",
				MascotPose: "assets/mascot/thumbs_up.png", CaptionStyle: "phrase_block",
				Slots:   []SlotSpec{{Role: "step", HTML: template.HTML("เช็คเวลา UTC"), StepNum: 1}},
				Content: SceneContent{SceneNumber: 2, Start: 15, End: 32, Layout: "step", CaptionStyle: "phrase_block", Num: "1", Of: "ขั้นที่ 1", Title: "เช็คเวลา UTC"}},
			{SceneNumber: 3, LayoutVariant: "quote_cta", AccentColor: "#2fd17a", AnimationSpeed: "normal",
				StartSec: 32, EndSec: 45, BackgroundMode: "css",
				Slots:   []SlotSpec{{Role: "headline", HTML: template.HTML("ทักแอดส์แวนซ์")}},
				Content: SceneContent{SceneNumber: 3, Start: 32, End: 45, Layout: "cta", CaptionStyle: "phrase_block", Title: "ทักแอดส์แวนซ์", CTA: "ทักเลย", Brand: "ADS VANCE"}},
		},
		Segments: []TranscriptSegment{{Text: "บัญชีโดนแบนเพราะอะไร", Start: 0, End: 4}},
	}
}

func TestRenderCompositionScenes_9x16(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	// Thai on-screen text now flows in via the structured ScenesJSON (Content),
	// not the legacy Slots markup; SEGMENTS/SCENES are the in-page builders.
	for _, m := range []string{`data-width="1080"`, `data-height="1920"`, "บัญชีโดนแบน", "เช็คเวลา UTC", "ทักแอดส์แวนซ์", "const SEGMENTS", "const SCENES"} {
		if !strings.Contains(s, m) {
			t.Errorf("output missing %q", m)
		}
	}
	// Go template actions must all be rendered. (The Style-B JS legitimately
	// contains literal "}}" in its arrow-function bodies, so a bare "}}" scan
	// would false-positive; check for an unrendered ".Field" action instead.)
	if strings.Contains(s, "{{.") || strings.Contains(s, "{{if") || strings.Contains(s, "{{end") {
		t.Errorf("unrendered template action")
	}
}

func TestRenderCompositionScenes_16x9(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("16:9"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `data-width="1920"`) || !strings.Contains(s, `data-height="1080"`) {
		t.Errorf("16:9 dimensions wrong")
	}
}

// Style-B has no mascot bumpers; the persistent chrome is the brand/category
// badge row and the in-page scene-wrapper builder. This asserts that chrome.
func TestRenderCompositionScenes_BadgesAndScaffold(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	for _, m := range []string{
		`class="badge-brand">ADS VANCE`, `class="badge-cat">PIXEL`,
		`id="progress"`, `id="capStage"`, `id="badges"`,
		`w.id = "scene-" + sc.scene`, `data-i="`,
	} {
		if !strings.Contains(s, m) {
			t.Errorf("output missing %q", m)
		}
	}
}

func TestRenderCompositionScenes_NoLegacyLiterals(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	legacy := []string{"#0a1428", "#0f1d35", "#16284a", "#ff6b2b",
		"rgba(15, 29, 53", "rgba(15,29,53", "rgba(255, 107, 43", "rgba(255,107,43",
		"rgba(10, 20, 40", "rgba(8, 16, 32"}
	for _, bad := range legacy {
		if strings.Contains(s, bad) {
			t.Errorf("rendered multi-scene HTML still contains legacy literal %q", bad)
		}
	}
}

func TestRenderCompositionScenes_RejectsEmpty(t *testing.T) {
	if _, err := RenderCompositionScenes(ScenesParams{AspectRatio: "9:16", DurationSeconds: 0}); err == nil {
		t.Error("expected error for DurationSeconds<=0")
	}
	if _, err := RenderCompositionScenes(ScenesParams{AspectRatio: "9:16", DurationSeconds: 10}); err == nil {
		t.Error("expected error for empty Scenes")
	}
}

func TestRenderCompositionScenes_CaptionStyleInJSON(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	for _, m := range []string{`"caption_style":"word_pop"`, `"caption_style":"phrase_block"`} {
		if !strings.Contains(s, m) {
			t.Errorf("ScenesJSON missing %q", m)
		}
	}
}

func TestRenderCompositionScenes_StyleB(t *testing.T) {
	params := ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", CategoryLabel: "การเงิน",
		VoiceSrc: "assets/voice.wav", DurationSeconds: 10,
		Scenes: []SceneSpec{
			{SceneNumber: 1, StartSec: 0, EndSec: 5, BackgroundMode: "image", BackgroundImage: "assets/bg-scene1.png",
				Content: SceneContent{SceneNumber: 1, Start: 0, End: 5, Layout: "stat", Stat: "2026", StatLabel: "ปีบังคับใช้",
					Chips: []ContentChip{{N: "90%", T: "ยังไม่รองรับ"}}}},
			{SceneNumber: 2, StartSec: 5, EndSec: 10, BackgroundMode: "css",
				Content: SceneContent{SceneNumber: 2, Start: 5, End: 10, Layout: "step", Num: "1", Of: "ขั้นที่ 1",
					Title: "โทรธนาคาร", Rows: []ContentRow{{Text: "ขอเปิดต่างประเทศ"}}}},
		},
		Segments: []TranscriptSegment{{Text: "ทดสอบ", Start: 0, End: 10}},
	}
	out, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{`const SCENES =`, `"type":"stat"`, `"stat":"2026"`, `"type":"step"`, `letter-spacing:0`, `--amber`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, emo := range []string{"❌", "✅", "📞", "💳", "🛡️", "👇", "⏰"} {
		if strings.Contains(html, emo) {
			t.Errorf("emoji leaked: %q", emo)
		}
	}
}
