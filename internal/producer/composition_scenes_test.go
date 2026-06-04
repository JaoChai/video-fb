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
		Scenes: []SceneSpec{
			{SceneNumber: 1, LayoutVariant: "hook_big", AccentColor: "#ff6b2b", AnimationSpeed: "normal",
				StartSec: 0, EndSec: 15, BackgroundMode: "css",
				Slots: []SlotSpec{{Role: "headline", HTML: template.HTML("บัญชีโดนแบน")}}},
			{SceneNumber: 2, LayoutVariant: "list_steps", AccentColor: "#ff6b2b", AnimationSpeed: "normal",
				StartSec: 15, EndSec: 32, BackgroundMode: "css",
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

func TestRenderCompositionScenes_RejectsEmpty(t *testing.T) {
	if _, err := RenderCompositionScenes(ScenesParams{AspectRatio: "9:16", DurationSeconds: 0}); err == nil {
		t.Error("expected error for DurationSeconds<=0")
	}
	if _, err := RenderCompositionScenes(ScenesParams{AspectRatio: "9:16", DurationSeconds: 10}); err == nil {
		t.Error("expected error for empty Scenes")
	}
}
