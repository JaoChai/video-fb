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
				Slots: []SlotSpec{{Role: "headline", HTML: template.HTML("บัญชีโดนแบน")}}},
			{SceneNumber: 2, LayoutVariant: "list_steps", AccentColor: "#F0A030", AnimationSpeed: "normal",
				StartSec: 15, EndSec: 32, BackgroundMode: "css",
				MascotPose: "assets/mascot/thumbs_up.png", CaptionStyle: "phrase_block",
				Slots: []SlotSpec{{Role: "step", HTML: template.HTML("เช็คเวลา UTC"), StepNum: 1}}},
			{SceneNumber: 3, LayoutVariant: "quote_cta", AccentColor: "#2fd17a", AnimationSpeed: "normal",
				StartSec: 32, EndSec: 45, BackgroundMode: "css",
				Slots: []SlotSpec{{Role: "headline", HTML: template.HTML("ทักแอดส์แวนซ์")}}},
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
	for _, m := range []string{`data-width="1080"`, `data-height="1920"`, "บัญชีโดนแบน", "เช็คเวลา UTC", "ทักแอดส์แวนซ์", "const SEGMENTS"} {
		if !strings.Contains(s, m) {
			t.Errorf("output missing %q", m)
		}
	}
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		t.Errorf("unrendered template delimiter")
	}
	if strings.Contains(s, `<span class="hl"><span`) {
		t.Errorf("nested highlight span")
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

func TestRenderCompositionScenes_Bumper(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	for _, m := range []string{`id="introBumper"`, "assets/mascot/rocket.png", `id="outroBumper"`, "assets/mascot/wave.png", "กดติดตาม", "assets/mascot/thumbs_up.png"} {
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
