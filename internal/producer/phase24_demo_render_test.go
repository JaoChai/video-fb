package producer

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// TestRenderPhase24Demo renders a full Phase 2-4 showcase: all 6 layout variants
// (hook_punch, hook_big, compare_two, list_steps, stat_reveal+callout, quote_cta),
// the C-B motion layer (kinetic captions, punch-in, parallax, branded transitions),
// image backgrounds, and ground-truth captions. Uses the poc voice.wav + a stand-in
// background. Guarded by HF_P24_DEMO=1.
//
// Run: HF_P24_DEMO=1 go test ./internal/producer/ -run TestRenderPhase24Demo -v -timeout 20m
func TestRenderPhase24Demo(t *testing.T) {
	if os.Getenv("HF_P24_DEMO") != "1" {
		t.Skip("set HF_P24_DEMO=1 to render the Phase 2-4 showcase")
	}

	// Scenes mapped to the poc voice.wav transcript timeline (~60s) so captions
	// stay synced to the real audio; layouts chosen to exercise every variant.
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "ติดตั้งทั้ง CAPI และ Pixel แล้วยอด Conversion นับซ้ำเป็น 2 เท่า ทั้งที่ตั้งค่า Deduplication ด้วย Event ID ให้ตรงกันแล้ว"},
		{SceneNumber: 2, VoiceText: "แต่ทำไมยังพังอยู่ ปัญหานี้ส่วนใหญ่เกิดจากเวลาครับ"},
		{SceneNumber: 3, VoiceText: "ต่อให้ ID ตรงกัน แต่ถ้า Event Time ฝั่ง Server กับ Browser ห่างกันเกินไป ระบบจะมองว่าเป็นคนละเหตุการณ์ทันที"},
		{SceneNumber: 4, VoiceText: "วิธีแก้คือ ตรวจเวลา Server ให้ตรง UTC เช็ค Event ซ้ำที่ Thank You Page และใช้ Event Test Tool ดูแบบ Real Time"},
		{SceneNumber: 5, VoiceText: "ถ้าแก้ตรงนี้ได้ ค่า CPA จะกลับมาเป็นจริง"},
		{SceneNumber: 6, VoiceText: "และระบบจะนำส่งโฆษณาได้แม่นยำขึ้นครับ"},
	}
	bounds := []sceneBound{
		{0, 11.68}, {11.68, 17.08}, {17.08, 27.68}, {27.68, 51.72}, {51.72, 56.20}, {56.20, 59.96},
	}
	segments := captionSegmentsFromScenes(scenes, bounds)

	aspect := os.Getenv("HF_ASPECT")
	if aspect == "" {
		aspect = "9:16"
	}
	img := func(n int) string { return fmt.Sprintf("assets/bg-scene%d.png", n) }
	params := ScenesParams{
		AspectRatio: aspect, BrandName: "ADS VANCE", CategoryLabel: "PIXEL",
		QuestionerName: "คุณป๊อบ", Kicker: "CAPI & PIXEL", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 59.96,
		Scenes: []SceneSpec{
			{SceneNumber: 1, LayoutVariant: "hook_punch", AccentColor: "#ff5a52", AnimationSpeed: "fast",
				StartSec: 0, EndSec: 11.68, BackgroundMode: "image", BackgroundImage: img(1),
				Slots: []SlotSpec{
					{Role: "badge", HTML: template.HTML("เคสจริง")},
					{Role: "headline", HTML: template.HTML(`Conversion <span class="acc">นับซ้ำ 2 เท่า</span>`)},
				}},
			{SceneNumber: 2, LayoutVariant: "hook_big", AccentColor: "#ff6b2b", AnimationSpeed: "normal",
				StartSec: 11.68, EndSec: 17.08, BackgroundMode: "image", BackgroundImage: img(2),
				Slots: []SlotSpec{
					{Role: "headline", HTML: template.HTML(`ปัญหาจริงอยู่ที่ <span class="acc">เวลา</span>`)},
				}},
			{SceneNumber: 3, LayoutVariant: "compare_two", AccentColor: "#3b82f6", AnimationSpeed: "normal",
				StartSec: 17.08, EndSec: 27.68, BackgroundMode: "image", BackgroundImage: img(3),
				Slots: []SlotSpec{
					{Role: "headline", HTML: template.HTML("Server vs Browser")},
					{Role: "body", HTML: template.HTML("Server Time")},
					{Role: "body", HTML: template.HTML("Browser Time")},
				}},
			{SceneNumber: 4, LayoutVariant: "list_steps", AccentColor: "#ff6b2b", AnimationSpeed: "fast",
				StartSec: 27.68, EndSec: 51.72, BackgroundMode: "image", BackgroundImage: img(4),
				Slots: []SlotSpec{
					{Role: "headline", HTML: template.HTML("3 วิธีแก้")},
					{Role: "step", HTML: template.HTML("ตรวจเวลา Server ให้ตรง UTC"), StepNum: 1},
					{Role: "step", HTML: template.HTML("เช็ค Event ซ้ำที่ Thank You Page"), StepNum: 2},
					{Role: "step", HTML: template.HTML("ใช้ Event Test Tool ดู Real Time"), StepNum: 3},
				}},
			{SceneNumber: 5, LayoutVariant: "stat_reveal", AccentColor: "#2fd17a", AnimationSpeed: "slow",
				StartSec: 51.72, EndSec: 56.20, BackgroundMode: "image", BackgroundImage: img(5),
				Slots: []SlotSpec{
					{Role: "stat", HTML: template.HTML("92%")},
					{Role: "body", HTML: template.HTML("ของบัญชีกลับมาแม่นยำ")},
					{Role: "callout", HTML: template.HTML(`ค่า <span class="acc">CPA</span> ลดลงจริง`)},
				}},
			{SceneNumber: 6, LayoutVariant: "quote_cta", AccentColor: "#ff6b2b", AnimationSpeed: "normal",
				StartSec: 56.20, EndSec: 59.96, BackgroundMode: "image", BackgroundImage: img(6),
				Slots: []SlotSpec{
					{Role: "quote", HTML: template.HTML("อย่ารอให้สายเกินไป")},
					{Role: "cta", HTML: template.HTML("ทักแอดส์แวนซ์เลย")},
				}},
		},
		Segments: segments,
	}

	htmlBytes, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render html: %v", err)
	}

	dir := filepath.Join(os.TempDir(), "hf-p24-demo")
	_ = os.RemoveAll(dir)
	assets := filepath.Join(dir, "assets")
	if err := os.MkdirAll(filepath.Join(assets, "fonts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), htmlBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"package.json":     projectPackageJSON,
		"hyperframes.json": projectHyperframesJSON,
		"meta.json":        `{"id":"p24-demo","name":"p24-demo"}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyDir("../../hyperframes-poc/poc-video/assets/fonts", filepath.Join(assets, "fonts")); err != nil {
		t.Fatalf("copy fonts: %v", err)
	}
	if err := copyFile("../../hyperframes-poc/poc-video/assets/voice.wav", filepath.Join(assets, "voice.wav")); err != nil {
		t.Fatalf("copy voice: %v", err)
	}
	bgSrc := "../../hyperframes-poc/poc-video/assets/background-9x16.png"
	for n := 1; n <= 6; n++ {
		if err := copyFile(bgSrc, filepath.Join(assets, fmt.Sprintf("bg-scene%d.png", n))); err != nil {
			t.Fatalf("copy bg-scene%d: %v", n, err)
		}
	}

	run := func(args ...string) error {
		cmd := exec.Command("npx", append([]string{"--yes", "hyperframes@0.6.70"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		t.Logf("\n$ npx hyperframes %v\n%s", args, lastBytes(out, 1500))
		return err
	}
	_ = run("lint")
	if err := run("render", "--output", "output.mp4", "--quality", "standard", "--fps", "24", "-w", "6"); err != nil {
		t.Fatalf("render mp4: %v", err)
	}
	t.Logf("PHASE 2-4 DEMO MP4: %s", filepath.Join(dir, "output.mp4"))
}
