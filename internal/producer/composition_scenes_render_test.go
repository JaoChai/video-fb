package producer

import (
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// richScenesParams builds a sample exercising all 4 layout variants with
// realistic Thai text so `hyperframes inspect` can catch overflow/clipping.
func richScenesParams(aspect string) ScenesParams {
	return ScenesParams{
		AspectRatio: aspect, BrandName: "ADS VANCE", CategoryLabel: "PIXEL",
		QuestionerName: "คุณป๊อบ", Kicker: "CAPI & PIXEL", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 24,
		Scenes: []SceneSpec{
			{SceneNumber: 1, LayoutVariant: "hook_big", AccentColor: "#ff6b2b", AnimationSpeed: "normal",
				StartSec: 0, EndSec: 6, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "badge", HTML: template.HTML("เคสจริง")},
					{Role: "headline", HTML: template.HTML(`บัญชีโฆษณาโดน<span class="acc">แบนถาวร</span>เพราะอะไร`)},
				}},
			{SceneNumber: 2, LayoutVariant: "list_steps", AccentColor: "#ff6b2b", AnimationSpeed: "fast",
				StartSec: 6, EndSec: 13, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "headline", HTML: template.HTML("3 ขั้นตอนกู้บัญชีคืน")},
					{Role: "step", HTML: template.HTML("เช็คเวลา UTC ของระบบให้ตรงกับโซนจริง"), StepNum: 1},
					{Role: "step", HTML: template.HTML("ยื่นอุทธรณ์พร้อมเอกสารยืนยันตัวตน"), StepNum: 2},
					{Role: "step", HTML: template.HTML("ตั้งค่า CAPI ใหม่ให้ครบทุก event"), StepNum: 3},
				}},
			{SceneNumber: 3, LayoutVariant: "stat_reveal", AccentColor: "#2fd17a", AnimationSpeed: "slow",
				StartSec: 13, EndSec: 19, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "stat", HTML: template.HTML("92%")},
					{Role: "body", HTML: template.HTML("ของบัญชีที่ยื่นถูกวิธี ได้รับการปลดล็อกภายใน 7 วัน")},
				}},
			{SceneNumber: 4, LayoutVariant: "quote_cta", AccentColor: "#ff6b2b", AnimationSpeed: "normal",
				StartSec: 19, EndSec: 24, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "quote", HTML: template.HTML("อย่ารอให้บัญชีโดนแบนก่อนค่อยแก้")},
					{Role: "cta", HTML: template.HTML("ทักแอดส์แวนซ์เลย")},
				}},
		},
		Segments: []TranscriptSegment{
			{Text: "บัญชีโฆษณาโดนแบนถาวรเพราะอะไรกันแน่", Start: 0.5, End: 5.5},
			{Text: "วันนี้เรามีสามขั้นตอนกู้คืนมาฝาก", Start: 6.2, End: 12.5},
			{Text: "เก้าสิบสองเปอร์เซ็นต์ปลดล็อกได้ใน เจ็ดวัน", Start: 13.3, End: 18.5},
			{Text: "อย่ารอให้สายเกินไป ทักแอดส์แวนซ์ได้เลย", Start: 19.3, End: 23.5},
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

	// fonts
	fontSrc := "/Users/jaochai/Code/video-fb/hyperframes-poc/poc-video/assets/fonts"
	if err := copyDir(fontSrc, fontsDst); err != nil {
		t.Fatalf("copy fonts: %v", err)
	}
	// voice (reuse poc voice; CSS bg mode means no image needed)
	if err := copyFile("/Users/jaochai/Code/video-fb/hyperframes-poc/poc-video/assets/voice.wav",
		filepath.Join(assets, "voice.wav")); err != nil {
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
