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

// TestRenderScrimDemo renders a multi-scene video in IMAGE background mode using
// a real AI-generated background (the poc background-9x16.png) so the Phase-1
// lighter scrim is visible: the image should now read as a hero behind the text
// instead of being buried under a heavy dark overlay. Captions come from the
// ground-truth VoiceText (Phase-0 fix). Guarded by HF_SCRIM_DEMO=1.
//
// NOTE: this proves the scrim/visibility change with a STAND-IN image. The
// Phase-1 content-relevance change (agent emits a concept from voice_text →
// buildScenePrompt) needs the live image API (OpenRouter key + DB) to show a
// content-matched image; that is verified at the code level, not rendered here.
//
// Run: HF_SCRIM_DEMO=1 go test ./internal/producer/ -run TestRenderScrimDemo -v -timeout 20m
func TestRenderScrimDemo(t *testing.T) {
	if os.Getenv("HF_SCRIM_DEMO") != "1" {
		t.Skip("set HF_SCRIM_DEMO=1 to render the image-mode scrim demo")
	}

	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "ติดตั้งทั้ง CAPI และ Pixel แล้วยอด Conversion นับซ้ำเป็น 2 เท่า ทั้งที่ตั้งค่า Deduplication หรือการกำจัดข้อมูลซ้ำด้วย Event ID ให้ตรงกันแล้ว แต่ทำไมยังพังอยู่"},
		{SceneNumber: 2, VoiceText: "ปัญหานี้ส่วนใหญ่เกิดจากเวลาครับ ต่อให้ ID ตรงกัน แต่ถ้า Event Time เวลาที่ส่งข้อมูลจากฝั่ง Server กับ Browser ห่างกันเกินไป ระบบจะมองว่าเป็นคนละเหตุการณ์ทันที"},
		{SceneNumber: 3, VoiceText: "วิธีแก้คือ ตรวจสอบว่าเวลาใน Server ตรงกับเวลามาตรฐานโลก UTC ไหม เช็คว่ามีการส่ง Event ซ้ำซ้อนในหน้า Thank You Page หรือเปล่า ลองใช้ Event Test Tool ใน Events Manager เพื่อดูแบบ Real Time ว่าข้อมูลตัวไหนถูกกำจัด"},
		{SceneNumber: 4, VoiceText: "ถ้าแก้ตรงนี้ได้ ค่า CPA จะกลับมาเป็นจริง และระบบจะนำส่งโฆษณาได้แม่นยำขึ้นครับ"},
	}
	bounds := []sceneBound{{0, 13.68}, {13.68, 27.68}, {27.68, 51.72}, {51.72, 59.96}}
	segments := captionSegmentsFromScenes(scenes, bounds)

	mkScene := func(n int, layout string, start, end float64, slots []SlotSpec) SceneSpec {
		return SceneSpec{
			SceneNumber: n, LayoutVariant: layout, AccentColor: "#ff6b2b", AnimationSpeed: "normal",
			StartSec: start, EndSec: end, BackgroundMode: "image",
			BackgroundImage: fmt.Sprintf("assets/bg-scene%d.png", n),
			Slots:           slots,
		}
	}
	params := ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", CategoryLabel: "PIXEL",
		QuestionerName: "คุณป๊อบ", Kicker: "CAPI & PIXEL", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 59.96,
		Scenes: []SceneSpec{
			mkScene(1, "hook_big", 0, 13.68, []SlotSpec{
				{Role: "badge", HTML: template.HTML("เคสจริง")},
				{Role: "headline", HTML: template.HTML(`Conversion <span class="acc">นับซ้ำ 2 เท่า</span>`)},
			}),
			mkScene(2, "hook_big", 13.68, 27.68, []SlotSpec{
				{Role: "badge", HTML: template.HTML("สาเหตุ")},
				{Role: "headline", HTML: template.HTML(`ปัญหาอยู่ที่ <span class="acc">Event Time</span>`)},
			}),
			mkScene(3, "list_steps", 27.68, 51.72, []SlotSpec{
				{Role: "headline", HTML: template.HTML("3 วิธีแก้")},
				{Role: "step", HTML: template.HTML("เช็คเวลา Server ให้ตรง UTC"), StepNum: 1},
				{Role: "step", HTML: template.HTML("เช็ค Event ซ้ำที่ Thank You Page"), StepNum: 2},
				{Role: "step", HTML: template.HTML("ใช้ Event Test Tool ดู Real Time"), StepNum: 3},
			}),
			mkScene(4, "quote_cta", 51.72, 59.96, []SlotSpec{
				{Role: "quote", HTML: template.HTML("ค่า CPA จะกลับมาแม่นยำ")},
				{Role: "cta", HTML: template.HTML("ทักแอดส์แวนซ์เลย")},
			}),
		},
		Segments: segments,
	}

	htmlBytes, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render html: %v", err)
	}

	dir := filepath.Join(os.TempDir(), "hf-scrim-demo")
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
		"meta.json":        `{"id":"scrim-demo","name":"scrim-demo"}`,
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
	// Same stand-in background for every scene (real pipeline gens one per scene).
	bgSrc := "../../hyperframes-poc/poc-video/assets/background-9x16.png"
	for n := 1; n <= 4; n++ {
		if err := copyFile(bgSrc, filepath.Join(assets, fmt.Sprintf("bg-scene%d.png", n))); err != nil {
			t.Fatalf("copy bg-scene%d: %v", n, err)
		}
	}

	run := func(args ...string) error {
		cmd := exec.Command("npx", append([]string{"--yes", "hyperframes@0.6.70"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		t.Logf("\n$ npx hyperframes %v\n%s", args, lastBytes(out, 1200))
		return err
	}
	_ = run("lint")
	if err := run("render", "--output", "output.mp4", "--quality", "standard", "--fps", "24", "-w", "12"); err != nil {
		t.Fatalf("render mp4: %v", err)
	}
	t.Logf("SCRIM DEMO MP4: %s", filepath.Join(dir, "output.mp4"))
}
