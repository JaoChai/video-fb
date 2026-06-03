package producer

import (
	"os"
	"strings"
	"testing"
)

func sampleParams() CompositionParams {
	return CompositionParams{
		Title:           "CAPI + Pixel นับยอดซ้ำ แก้ยังไง?",
		HighlightWords:  []string{"ซ้ำ"},
		Kicker:          "CAPI & PIXEL",
		BrandName:       "ADS VANCE",
		CategoryLabel:   "PIXEL",
		QuestionerName:  "คุณป๊อบ",
		LayoutVariant:   "dynamic_karaoke",
		AccentColor:     "#ff6b2b",
		SecondaryAccent: "#2fd17a",
		AnimationSpeed:  "normal",
		BackgroundMode:  "css",
		VoiceSrc:        "assets/voice.wav",
		DurationSeconds: 60.0,
		Cards: []CardSpec{
			{ID: "cardCause", Type: "cause", StartSec: 13.7, EndSec: 24.6, Kicker: "สาเหตุของปัญหา", Body: "สาเหตุอยู่ที่ เวลา — event_time ฝั่ง Server กับ Browser ห่างกันเกินไป"},
			{ID: "card1", Type: "step", StartSec: 27.7, EndSec: 35.2, Kicker: "วิธีแก้ที่หนึ่ง", Body: "เช็คเวลาใน Server ให้ตรง UTC", StepNum: 1},
			{ID: "card2", Type: "step", StartSec: 35.4, EndSec: 40.2, Kicker: "วิธีแก้ที่สอง", Body: "เช็ค event ซ้ำในหน้า Thank You Page", StepNum: 2},
			{ID: "card3", Type: "step", StartSec: 40.4, EndSec: 51.5, Kicker: "วิธีแก้ที่สาม", Body: "ใช้ Event Test Tool ดูแบบ Real-time", StepNum: 3},
			{ID: "cardWin", Type: "win", StartSec: 51.7, EndSec: 55.8, Kicker: "ผลลัพธ์", Body: "ค่า CPA กลับมาแม่นยำ นำส่งโฆษณาได้ตรงกลุ่มขึ้น"},
		},
		Segments: []TranscriptSegment{
			{Text: "ติดตั้งทั้ง CAPI และ Pixel แล้วยอด Conversion นับซ้ำเป็น 2 เท่า", Start: 0.0, End: 5.72},
			{Text: "ทั้งที่ตั้งค่า Deduplication หรือการกำจัดข้อมูลซ้ำด้วย Event ID ให้ตรงกันแล้ว", Start: 5.72, End: 11.68},
			{Text: "แต่ทำไมยังพังอยู่", Start: 11.68, End: 13.68},
			{Text: "ปัญหานี้ส่วนใหญ่เกิดจากเวลาครับ", Start: 13.68, End: 17.08},
			{Text: "ต่อให้ ID ตรงกัน แต่ถ้า Event Time เวลาที่ส่งข้อมูลจากฝั่ง Server กับ Browser ห่างกันเกินไป", Start: 17.08, End: 24.76},
			{Text: "ระบบจะมองว่าเป็นคนละเหตุการณ์ทันที", Start: 24.76, End: 27.68},
			{Text: "วิธีแก้คือ 1. ตรวจสอบว่าเวลาใน Server ตรงกับเวลามาตรฐานโลก หรือ UTC ไหม", Start: 27.68, End: 35.4},
			{Text: "2. เช็คว่ามีการส่ง Event ซ้ำซ้อนในหน้า Thank You Page หรือเปล่า", Start: 35.4, End: 40.36},
			{Text: "3. ลองใช้ Event Test Tool ใน Events Manager เพื่อดูแบบ Real Time ว่าข้อมูลตัวไหนถูกกำจัด", Start: 40.36, End: 51.72},
			{Text: "ถ้าแก้ตรงนี้ได้ ค่า CPA จะกลับมาเป็นจริง และระบบจะนำส่งโฆษณาได้แม่นยำขึ้นครับ", Start: 51.72, End: 59.96},
		},
	}
}

func TestRenderComposition(t *testing.T) {
	html, err := RenderComposition(sampleParams())
	if err != nil {
		t.Fatalf("RenderComposition: %v", err)
	}
	s := string(html)

	mustContain := []string{
		`data-composition-id="main"`,
		`data-width="1080"`,
		`data-height="1920"`,
		`--orange: #ff6b2b;`,
		`--green: #2fd17a;`,
		`<div class="badge-brand">ADS VANCE</div>`,
		`<div class="badge-cat">PIXEL</div>`,
		`คำถามจาก <b>คุณป๊อบ</b>`,
		`<span class="hl">ซ้ำ</span>`,
		`src="assets/voice.wav"`,
		`window.__timelines["main"] = tl;`,
		`tl.set({}, {}, TOTAL);`,
		`const CARDS = [`,
		`const SEGMENTS = [`,
		`fitTextFontSize`,
		`ADS <span>VANCE</span>`,
	}
	for _, m := range mustContain {
		if !strings.Contains(s, m) {
			t.Errorf("output missing %q", m)
		}
	}

	// Template delimiters must be fully consumed.
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		t.Errorf("unrendered template delimiter present in output")
	}

	// JSON content must survive into the script (Thai + step numbers).
	if !strings.Contains(s, `"type":"cause"`) || !strings.Contains(s, `"step":3`) {
		t.Errorf("cards JSON not embedded correctly")
	}
	if !strings.Contains(s, `"text":"แต่ทำไมยังพังอยู่"`) {
		t.Errorf("segments JSON not embedded correctly")
	}

	// Outro derived from duration: 60 - 4 = 56.
	if !strings.Contains(s, `data-start="56"`) {
		t.Errorf("outro start not derived to 56s; got none")
	}
}

func TestRenderComposition_ImageBackground(t *testing.T) {
	p := sampleParams()
	p.BackgroundMode = "image"
	p.BackgroundImage = "assets/background-9x16.png"
	out, err := RenderComposition(p)
	if err != nil {
		t.Fatalf("RenderComposition: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `<img src="assets/background-9x16.png"`) {
		t.Errorf("image background not rendered")
	}
	if strings.Contains(s, `class="bg-dots"`) {
		t.Errorf("css-mode background present in image mode")
	}
}

func TestRenderComposition_SanitizesBadColor(t *testing.T) {
	p := sampleParams()
	p.AccentColor = "red; } body { display:none } .x{color:blue"
	out, err := RenderComposition(p)
	if err != nil {
		t.Fatalf("RenderComposition: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `--orange: #ff6b2b;`) {
		t.Errorf("invalid color not replaced with fallback")
	}
	if strings.Contains(s, "display:none") {
		t.Errorf("CSS injection leaked into output")
	}
}

// TestWriteSampleHTML writes the rendered composition for manual `npm run check`
// when HF_WRITE_SAMPLE is set, e.g.:
//
//	HF_WRITE_SAMPLE=../../hyperframes-poc/poc-video/index.html go test -run TestWriteSampleHTML ./internal/producer/
func TestWriteSampleHTML(t *testing.T) {
	out := os.Getenv("HF_WRITE_SAMPLE")
	if out == "" {
		t.Skip("set HF_WRITE_SAMPLE=<path> to write rendered HTML")
	}
	html, err := RenderComposition(sampleParams())
	if err != nil {
		t.Fatalf("RenderComposition: %v", err)
	}
	if err := os.WriteFile(out, html, 0o644); err != nil {
		t.Fatalf("write %s: %v", out, err)
	}
	t.Logf("wrote %d bytes to %s", len(html), out)
}
