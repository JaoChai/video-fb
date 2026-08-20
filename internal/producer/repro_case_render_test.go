package producer

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// สนามซ้อมสำหรับสืบสาเหตุคลิป 8885bcc8 ที่เรนเดอร์บน prod ไม่ผ่าน
// (Page.captureScreenshot timed out ทั้ง 3 worker, 662s/681s) เทียบกับคลิป
// bbb42f7b ที่ preset/รูปแบบเดียวกันแต่ผ่านใน 136s — สร้างโปรเจกต์จาก scene rows
// จริงของทั้งคู่แล้วเรนเดอร์ด้วยแฟล็กชุดเดียวกับ prod เพื่อดูว่าเป็นที่ composition
// หรือเป็นที่เครื่อง. รันด้วย HF_RENDER=1 เท่านั้น (ต้องมี Node + Chromium)
func writeSilentWAV(t *testing.T, path string, seconds float64) {
	t.Helper()
	const rate = 24000
	n := int(seconds * rate)
	data := make([]byte, 44+n*2)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(36+n*2))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], rate)
	binary.LittleEndian.PutUint32(data[28:32], rate*2)
	binary.LittleEndian.PutUint16(data[32:34], 2)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(n*2))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// reproCaseParams ประกอบ ScenesParams แบบเดียวกับ AssembleHyperframes916 โหมด case
func reproCaseParams(t *testing.T, fixture string, caseNumber int) ScenesParams {
	t.Helper()
	scenes := realClipScenes(t, fixture)
	bounds := realClipBounds(scenes)
	specs := buildSceneSpecs(scenes, bounds)
	if len(specs) == 0 {
		t.Fatal("buildSceneSpecs returned empty")
	}
	preset := CaseFilePreset
	return ScenesParams{
		AspectRatio:     "9:16",
		BrandName:       BrandName,
		CTAText:         BrandCTA,
		VoiceSrc:        "assets/voice.wav",
		DurationSeconds: bounds[len(bounds)-1].End,
		Scenes:          specs,
		Segments:        captionSegmentsFromScenes(scenes, bounds),
		Palette:         preset.Palette,
		BrandCSS:        preset.BrandCSS(),
		ThemeKey:        preset.Key,
		Motion:          preset.Motion,
		Format:          ModeCase,
		CaseNumber:      caseNumber,
		MotionV2:        SceneMotionV2Enabled(),
		Cover:           CoverSceneEnabled(),
	}
}

type reproFixture struct {
	name    string
	fixture string
	caseNum int
}

// reproFixtures คือคลิปคู่เทียบ: ตัวที่ prod เรนเดอร์ค้าง 20 ส.ค. 2026 กับตัวที่
// preset/รูปแบบเดียวกันแต่ผ่านปกติเมื่อ 19 ส.ค. — ต้องรันคู่กันเสมอ ตัวเลขของ
// ตัวเดียวไม่บอกอะไร
func reproFixtures() []reproFixture {
	return []reproFixture{
		{"fail_8885bcc8", "clip_8885bcc8_scenes.json", 247},
		{"ok_bbb42f7b", "clip_bbb42f7b_scenes.json", 246},
	}
}

func TestReproCaseRender(t *testing.T) {
	if os.Getenv("HF_RENDER") != "1" {
		t.Skip("set HF_RENDER=1 to run the render harness")
	}
	only := os.Getenv("HF_ONLY")
	for _, c := range reproFixtures() {
		if only != "" && only != c.name {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			params := reproCaseParams(t, c.fixture, c.caseNum)

			dir := t.TempDir()
			if keep := os.Getenv("HF_KEEP_DIR"); keep != "" {
				dir = filepath.Join(keep, c.name)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			voice := filepath.Join(t.TempDir(), "voice.wav")
			writeSilentWAV(t, voice, params.DurationSeconds)

			builder := NewCompositionBuilder("assets/fonts")
			if _, err := builder.BuildScenes(params, c.name, dir, voice, map[int]string{}); err != nil {
				t.Fatalf("build scenes: %v", err)
			}
			t.Logf("%s: project=%s duration=%.2fs scenes=%d segments=%d",
				c.name, dir, params.DurationSeconds, len(params.Scenes), len(params.Segments))

			started := time.Now()
			res, err := NewHyperframesRenderer().Render(context.Background(), dir, "output.mp4")
			t.Logf("%s: render took %s passed=%v findings=%v", c.name, time.Since(started), res.Passed, res.Findings)
			if err != nil {
				t.Errorf("%s: render failed: %v", c.name, err)
			}
		})
	}
}

// TestReproCaseChecks รันด่าน lint กับ inspect ชุดเดียวกับที่ prod รันก่อนเรนเดอร์
// แยกจาก TestReproCaseRender เพราะตอนอัปเวอร์ชัน CLI สิ่งที่ต้องเทียบไม่ใช่แค่
// "เรนเดอร์ออกไหม" แต่คือ "inspect ตีตกเพิ่มไหม" — inspect เป็นด่านที่ส่งคลิปไป
// needs_review ได้จริง การอัปเวอร์ชันที่เข้มขึ้นเงียบๆ จะกักคลิปทั้งสายโดยไม่มีใครรู้
func TestReproCaseChecks(t *testing.T) {
	if os.Getenv("HF_RENDER") != "1" {
		t.Skip("set HF_RENDER=1 to run the checks harness")
	}
	for _, c := range reproFixtures() {
		t.Run(c.name, func(t *testing.T) {
			params := reproCaseParams(t, c.fixture, c.caseNum)
			dir := t.TempDir()
			voice := filepath.Join(t.TempDir(), "voice.wav")
			writeSilentWAV(t, voice, params.DurationSeconds)

			builder := NewCompositionBuilder("assets/fonts")
			if _, err := builder.BuildScenes(params, c.name, dir, voice, map[int]string{}); err != nil {
				t.Fatalf("build scenes: %v", err)
			}

			r := NewHyperframesRenderer()
			lintRes, lintErr := r.Lint(context.Background(), dir)
			t.Logf("%s lint: passed=%v %dms findings=%v", c.name, lintRes.Passed, lintRes.DurationMS, lintRes.Findings)
			inspectRes, inspectErr := r.Inspect(context.Background(), dir)
			t.Logf("%s inspect: passed=%v %dms findings=%v", c.name, inspectRes.Passed, inspectRes.DurationMS, inspectRes.Findings)
			if lintErr != nil {
				t.Logf("%s lint error: %v", c.name, lintErr)
			}
			if inspectErr != nil {
				t.Errorf("%s inspect flagged (เทียบกับผลของเวอร์ชันเดิมก่อนสรุปว่าเป็น regression): %v", c.name, inspectErr)
			}
		})
	}
}
