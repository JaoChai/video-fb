package producer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRenderSampleA1A4 renders a real MP4 exercising the phase-1 visual upgrades
// (A1 per-scene entrance speed + A4 emphasis-word pop/glow) with full scene
// content, so the effects are actually visible. Guarded by HF_SAMPLE=1 and
// HF_OUT=<path.mp4> so the normal suite stays green without Node/Chromium.
func TestRenderSampleA1A4(t *testing.T) {
	if os.Getenv("HF_SAMPLE") != "1" {
		t.Skip("set HF_SAMPLE=1 and HF_OUT=<path.mp4> to render the A1/A4 sample clip")
	}
	out := os.Getenv("HF_OUT")
	if out == "" {
		t.Fatal("HF_OUT (absolute .mp4 path) required")
	}

	preset := PresetByKey("editorial-bold")
	sc := func(n int, layout string, start, end float64, c SceneContent) SceneSpec {
		c.SceneNumber, c.Start, c.End = n, start, end
		c.Layout, c.Speed = layout, speedForLayout(layout)
		if c.CaptionStyle == "" {
			c.CaptionStyle = "phrase_block"
		}
		return SceneSpec{SceneNumber: n, StartSec: start, EndSec: end, BackgroundMode: "css",
			AccentColor: Brand.Orange, CaptionStyle: c.CaptionStyle, Content: c}
	}

	params := ScenesParams{
		AspectRatio: "9:16", BrandName: BrandName, CategoryLabel: "PIXEL",
		Kicker: "CAPI & PIXEL", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 31, ThemeKey: preset.Key, Motion: preset.Motion,
		BrandCSS: preset.BrandCSS(), Palette: preset.Palette,
		AudioMotion: true, // MOTION_UP branch (matches prod AUDIO_MOTION_ENABLED)
		CTAText:     "ทักแอดส์แวนซ์เลย",
		Scenes: []SceneSpec{
			sc(1, "hook", 0, 5, SceneContent{CaptionStyle: "word_pop",
				Rows: []ContentRow{{Text: "บัญชีโดนแบนถาวร", Bad: true}, {Text: "กู้คืนไม่ได้"}}}),
			sc(2, "stat", 5, 11, SceneContent{Stat: "92", Unit: "%", StatLabel: "ปลดล็อกได้ใน 7 วันถ้ายื่นถูกวิธี",
				Chips: []ContentChip{{N: "7", T: "วันเฉลี่ย"}, {N: "3", T: "ขั้นตอน"}}}),
			sc(3, "step", 11, 17, SceneContent{Num: "1", Of: "ขั้นที่ 1 / 3", Title: "เช็คเวลา UTC",
				Rows: []ContentRow{{Text: "ตั้งโซนเวลาให้ตรงระบบ"}, {Text: "ยืนยันตัวตนให้ครบ"}}}),
			sc(4, "tip", 17, 22, SceneContent{Pill: "ป้องกันระยะยาว",
				Rows: []ContentRow{{Text: "อย่ายิงแอดผิดนโยบาย"}, {Text: "สำรองบัญชีสำรองไว้"}}}),
			sc(5, "hero", 22, 27, SceneContent{Kicker: "ทางออก",
				Title: `ยื่นอุทธรณ์ให้ <span class="acc">ถูกวิธี</span>`, Sub: "แล้วบัญชีกลับมาได้"}),
			sc(6, "cta", 27, 31, SceneContent{Title: "อย่ารอให้สาย",
				CTA: "ทักแอดส์แวนซ์", Brand: "ADS VANCE", Sub: "ปรึกษาฟรี"}),
		},
		Segments: []TranscriptSegment{
			{Text: "บัญชีโฆษณาโดนแบนถาวร", Start: 0.4, End: 4.6, Emphasis: []string{"แบนถาวร"}},
			{Text: "เก้าสิบสองเปอร์เซ็นต์ปลดล็อกได้", Start: 5.2, End: 10.6, Emphasis: []string{"เก้าสิบสองเปอร์เซ็นต์"}},
			{Text: "ขั้นแรกเช็คเวลา UTC ให้ตรง", Start: 11.2, End: 16.6, Emphasis: []string{"UTC"}},
			{Text: "ป้องกันระยะยาวอย่ายิงผิดนโยบาย", Start: 17.2, End: 21.6, Emphasis: []string{"ป้องกัน"}},
			{Text: "ขอแค่ยื่นอุทธรณ์ให้ถูกวิธี", Start: 22.2, End: 26.6, Emphasis: []string{"ถูกวิธี"}},
			{Text: "อย่ารอให้สายทักแอดส์แวนซ์เลย", Start: 27.2, End: 30.6, Emphasis: []string{"แอดส์แวนซ์"}},
		},
	}

	htmlBytes, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render html: %v", err)
	}

	dir := t.TempDir()
	assets := filepath.Join(dir, "assets")
	if err := os.MkdirAll(filepath.Join(assets, "fonts"), 0o755); err != nil {
		t.Fatal(err)
	}
	// GSAP runtime — prod does this in BuildScenes; without it the timeline throws
	// "gsap is not defined" and every animation (incl. A1/A4) freezes.
	if err := writeGsapAsset(assets); err != nil {
		t.Fatalf("write gsap: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), htmlBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"package.json":     projectPackageJSON,
		"hyperframes.json": projectHyperframesJSON,
		"meta.json":        `{"id":"a1a4-sample","name":"a1a4-sample"}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyDir("assets/fonts", filepath.Join(assets, "fonts")); err != nil {
		t.Fatalf("copy fonts: %v", err)
	}
	if err := copyFile("../../hyperframes-poc/poc-video/assets/voice.wav", filepath.Join(assets, "voice.wav")); err != nil {
		t.Fatalf("copy voice: %v", err)
	}

	// Lint first so a broken template fails fast, then render the MP4.
	r := NewHyperframesRenderer()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := r.Lint(ctx, dir); err != nil {
		t.Fatalf("lint: %v", err)
	}
	// Inspect guards A2 repositioning against text overflow / caption collision.
	if err := r.Inspect(ctx, dir); err != nil {
		t.Fatalf("inspect (overflow/clip): %v", err)
	}
	if err := r.Render(ctx, dir, out); err != nil {
		t.Fatalf("render mp4: %v", err)
	}
	if fi, err := os.Stat(out); err != nil || fi.Size() == 0 {
		t.Fatalf("output not written: %v", err)
	}
	t.Logf("rendered %s", out)
}
