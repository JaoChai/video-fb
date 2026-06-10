package producer

import (
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// richScenesParams builds a sample exercising all 6 layout variants and every
// slot role (incl. stat + callout) with realistic Thai text so
// `hyperframes inspect` can catch overflow/clipping on each.
func richScenesParams(aspect string) ScenesParams {
	return ScenesParams{
		AspectRatio: aspect, BrandName: "ADS VANCE", CategoryLabel: "PIXEL",
		QuestionerName: "คุณป๊อบ", Kicker: "CAPI & PIXEL", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 38,
		Scenes: []SceneSpec{
			{SceneNumber: 1, LayoutVariant: "hook_punch", AccentColor: "#ff6b2b", AnimationSpeed: "fast",
				StartSec: 0, EndSec: 5, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "badge", HTML: template.HTML("เคสจริง")},
					{Role: "headline", HTML: template.HTML(`บัญชีโดน<span class="acc">แบนถาวร</span>`)},
				}},
			{SceneNumber: 2, LayoutVariant: "hook_big", AccentColor: "#ff6b2b", AnimationSpeed: "normal",
				StartSec: 5, EndSec: 11, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "badge", HTML: template.HTML("คำถามจากลูกค้า")},
					{Role: "headline", HTML: template.HTML(`บัญชีโฆษณาโดน<span class="acc">แบนถาวร</span>เพราะอะไร`)},
				}},
			{SceneNumber: 3, LayoutVariant: "list_steps", AccentColor: "#ff6b2b", AnimationSpeed: "fast",
				StartSec: 11, EndSec: 18, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "headline", HTML: template.HTML("3 ขั้นตอนกู้บัญชีคืน")},
					{Role: "step", HTML: template.HTML("เช็คเวลา UTC ของระบบให้ตรงกับโซนจริง"), StepNum: 1},
					{Role: "step", HTML: template.HTML("ยื่นอุทธรณ์พร้อมเอกสารยืนยันตัวตน"), StepNum: 2},
					{Role: "step", HTML: template.HTML("ตั้งค่า CAPI ใหม่ให้ครบทุก event"), StepNum: 3},
				}},
			{SceneNumber: 4, LayoutVariant: "stat_reveal", AccentColor: "#2fd17a", AnimationSpeed: "slow",
				StartSec: 18, EndSec: 24, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "stat", HTML: template.HTML("92%")},
					{Role: "body", HTML: template.HTML("ของบัญชีที่ยื่นถูกวิธี ได้รับการปลดล็อกภายใน 7 วัน")},
					{Role: "callout", HTML: template.HTML(`ปลดล็อกเฉลี่ยภายใน <span class="acc">7 วัน</span>`)},
				}},
			{SceneNumber: 5, LayoutVariant: "compare_two", AccentColor: "#2fd17a", AnimationSpeed: "normal",
				StartSec: 24, EndSec: 31, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "headline", HTML: template.HTML("ทำเองvsทักแอดส์แวนซ์")},
					{Role: "body", HTML: template.HTML("ยื่นถูกวิธี ปลดล็อกใน 7 วัน")},
					{Role: "body", HTML: template.HTML("ลองเองมั่ว เสี่ยงโดนแบนซ้ำ")},
				}},
			{SceneNumber: 6, LayoutVariant: "quote_cta", AccentColor: "#ff6b2b", AnimationSpeed: "normal",
				StartSec: 31, EndSec: 38, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "quote", HTML: template.HTML("อย่ารอให้บัญชีโดนแบนก่อนค่อยแก้")},
					{Role: "cta", HTML: template.HTML("ทักแอดส์แวนซ์เลย")},
				}},
		},
		Segments: []TranscriptSegment{
			{Text: "บัญชีโฆษณาโดนแบนถาวร", Start: 0.5, End: 4.5},
			{Text: "แบนถาวรเพราะอะไรกันแน่", Start: 5.2, End: 10.5},
			{Text: "วันนี้เรามีสามขั้นตอนกู้คืนมาฝาก", Start: 11.2, End: 17.5},
			{Text: "เก้าสิบสองเปอร์เซ็นต์ปลดล็อกได้ใน เจ็ดวัน", Start: 18.3, End: 23.5},
			{Text: "ทำเองกับให้มืออาชีพต่างกันมาก", Start: 24.3, End: 30.5},
			{Text: "อย่ารอให้สายเกินไป ทักแอดส์แวนซ์ได้เลย", Start: 31.3, End: 37.5},
		},
	}
}

// TestManualRenderMultiScene builds a real Hyperframes project from the
// multi-scene template and runs `lint` + `inspect`. Guarded by HF_RENDER=1 so
// the normal `go test ./...` stays green without Node/Chromium.
func TestManualRenderMultiScene(t *testing.T) {
	if os.Getenv("HF_RENDER") != "1" {
		t.Skip("set HF_RENDER=1 to run the Hyperframes lint+inspect render harness")
	}
	aspect := os.Getenv("HF_ASPECT")
	if aspect == "" {
		aspect = "9:16"
	}

	htmlBytes, err := RenderCompositionScenes(richScenesParams(aspect))
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	dir := t.TempDir()
	if keep := os.Getenv("HF_KEEP_DIR"); keep != "" {
		dir = keep
		_ = os.MkdirAll(dir, 0o755)
	}
	assets := filepath.Join(dir, "assets")
	fontsDst := filepath.Join(assets, "fonts")
	if err := os.MkdirAll(fontsDst, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "index.html"), htmlBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"package.json":     projectPackageJSON,
		"hyperframes.json": projectHyperframesJSON,
		"meta.json":        `{"id":"multi-scene-harness","name":"multi-scene-harness"}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// fonts — repo-relative default (tests run from internal/producer/), overridable.
	fontSrc := os.Getenv("HF_FONT_SRC")
	if fontSrc == "" {
		fontSrc = "assets/fonts"
	}
	if err := copyDir(fontSrc, fontsDst); err != nil {
		t.Fatalf("copy fonts: %v", err)
	}
	// voice (reuse poc voice; CSS bg mode means no image needed)
	voiceSrc := os.Getenv("HF_VOICE_SRC")
	if voiceSrc == "" {
		voiceSrc = "../../hyperframes-poc/poc-video/assets/voice.wav"
	}
	if err := copyFile(voiceSrc, filepath.Join(assets, "voice.wav")); err != nil {
		t.Fatalf("copy voice: %v", err)
	}

	t.Logf("project dir: %s (aspect %s)", dir, aspect)

	run := func(args ...string) {
		cmd := exec.Command("npx", append([]string{"--yes", "hyperframes@0.6.70"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		t.Logf("\n$ npx hyperframes %v\n%s", args, out)
		if err != nil {
			t.Errorf("hyperframes %v failed: %v", args, err)
		}
	}
	run("lint")
	run("inspect", "--json")
}
