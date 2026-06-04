package producer

import (
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// TestRenderCaptionDemo proves the new caption pipeline on REAL audio: it feeds
// the poc voice.wav's real transcript (the exact spoken text) as ground-truth
// scene VoiceText, uses the real per-scene audio windows, lets
// captionSegmentsFromScenes() build the captions, and renders an actual MP4.
// Guarded by HF_CAPTION_DEMO=1 so the normal suite stays offline.
//
// Run: HF_CAPTION_DEMO=1 go test ./internal/producer/ -run TestRenderCaptionDemo -v -timeout 20m
func TestRenderCaptionDemo(t *testing.T) {
	if os.Getenv("HF_CAPTION_DEMO") != "1" {
		t.Skip("set HF_CAPTION_DEMO=1 to render the real-audio caption demo")
	}

	// Ground truth = the actual words spoken in hyperframes-poc voice.wav,
	// grouped into 4 scenes with their REAL audio time windows.
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "ติดตั้งทั้ง CAPI และ Pixel แล้วยอด Conversion นับซ้ำเป็น 2 เท่า ทั้งที่ตั้งค่า Deduplication หรือการกำจัดข้อมูลซ้ำด้วย Event ID ให้ตรงกันแล้ว แต่ทำไมยังพังอยู่"},
		{SceneNumber: 2, VoiceText: "ปัญหานี้ส่วนใหญ่เกิดจากเวลาครับ ต่อให้ ID ตรงกัน แต่ถ้า Event Time เวลาที่ส่งข้อมูลจากฝั่ง Server กับ Browser ห่างกันเกินไป ระบบจะมองว่าเป็นคนละเหตุการณ์ทันที"},
		{SceneNumber: 3, VoiceText: "วิธีแก้คือ ตรวจสอบว่าเวลาใน Server ตรงกับเวลามาตรฐานโลก UTC ไหม เช็คว่ามีการส่ง Event ซ้ำซ้อนในหน้า Thank You Page หรือเปล่า ลองใช้ Event Test Tool ใน Events Manager เพื่อดูแบบ Real Time ว่าข้อมูลตัวไหนถูกกำจัด"},
		{SceneNumber: 4, VoiceText: "ถ้าแก้ตรงนี้ได้ ค่า CPA จะกลับมาเป็นจริง และระบบจะนำส่งโฆษณาได้แม่นยำขึ้นครับ"},
	}
	bounds := []sceneBound{
		{Start: 0.0, End: 13.68},
		{Start: 13.68, End: 27.68},
		{Start: 27.68, End: 51.72},
		{Start: 51.72, End: 59.96},
	}

	// THE fix under test: captions from ground-truth text, not ASR.
	segments := captionSegmentsFromScenes(scenes, bounds)
	for _, s := range segments {
		t.Logf("[%5.2f→%5.2f] %s", s.Start, s.End, s.Text)
	}

	params := ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", CategoryLabel: "PIXEL",
		QuestionerName: "คุณป๊อบ", Kicker: "CAPI & PIXEL", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 59.96,
		Scenes: []SceneSpec{
			{SceneNumber: 1, LayoutVariant: "hook_big", AccentColor: "#ff6b2b", AnimationSpeed: "normal",
				StartSec: 0, EndSec: 13.68, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "badge", HTML: template.HTML("เคสจริง")},
					{Role: "headline", HTML: template.HTML(`Conversion <span class="acc">นับซ้ำ 2 เท่า</span>`)},
				}},
			{SceneNumber: 2, LayoutVariant: "hook_big", AccentColor: "#ff6b2b", AnimationSpeed: "normal",
				StartSec: 13.68, EndSec: 27.68, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "badge", HTML: template.HTML("สาเหตุ")},
					{Role: "headline", HTML: template.HTML(`ปัญหาอยู่ที่ <span class="acc">Event Time</span>`)},
				}},
			{SceneNumber: 3, LayoutVariant: "list_steps", AccentColor: "#ff6b2b", AnimationSpeed: "fast",
				StartSec: 27.68, EndSec: 51.72, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "headline", HTML: template.HTML("3 วิธีแก้")},
					{Role: "step", HTML: template.HTML("เช็คเวลา Server ให้ตรง UTC"), StepNum: 1},
					{Role: "step", HTML: template.HTML("เช็ค Event ซ้ำที่ Thank You Page"), StepNum: 2},
					{Role: "step", HTML: template.HTML("ใช้ Event Test Tool ดู Real Time"), StepNum: 3},
				}},
			{SceneNumber: 4, LayoutVariant: "quote_cta", AccentColor: "#2fd17a", AnimationSpeed: "normal",
				StartSec: 51.72, EndSec: 59.96, BackgroundMode: "css",
				Slots: []SlotSpec{
					{Role: "quote", HTML: template.HTML("ค่า CPA จะกลับมาแม่นยำ")},
					{Role: "cta", HTML: template.HTML("ทักแอดส์แวนซ์เลย")},
				}},
		},
		Segments: segments,
	}

	htmlBytes, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render html: %v", err)
	}

	dir := filepath.Join(os.TempDir(), "hf-caption-demo")
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
		"meta.json":        `{"id":"caption-demo","name":"caption-demo"}`,
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
	t.Logf("DEMO MP4: %s", filepath.Join(dir, "output.mp4"))
}
