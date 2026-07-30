package producer

import (
	"strings"
	"testing"
)

func mythParams() ScenesParams {
	mk := func(n int, layout string, c SceneContent) SceneSpec {
		c.SceneNumber, c.Layout = n, layout
		c.Start, c.End = float64(n-1)*5, float64(n)*5
		return SceneSpec{SceneNumber: n, StartSec: c.Start, EndSec: c.End,
			LayoutVariant: "hook_big", CaptionStyle: "phrase_block", Content: c}
	}
	return ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 30, Format: "myth", ThemeKey: "factcheck",
		MythVerdict: "half_true", MythSource: "Jon Loomer",
		Scenes: []SceneSpec{
			mk(1, "hook", SceneContent{Rows: []ContentRow{{Text: "ย้าย BM เสียเวลา 3 วัน"}},
				BackgroundImage: "assets/bg-scene1.png"}),
			mk(2, "belief", SceneContent{Kicker: "ที่เชื่อกัน",
				Title: "เปิด BM แล้วบัญชีแข็งกว่าบัญชีส่วนตัว",
				Sub:   "เพราะเอเจนซีทุกที่ใช้ BM"}),
			mk(3, "verdict", SceneContent{Title: "ไม่ได้แข็งกว่า",
				Stamp: "โมเดลแต่งเอง", Meter: "โมเดลแต่งเอง"}),
			mk(4, "proof", SceneContent{Title: "BM ให้ความเป็นเจ้าของ ไม่ใช่ภูมิคุ้มกัน",
				Rows:   []ContentRow{{Text: "สิทธิ์ทีมและ pixel อยู่กับธุรกิจ"}},
				Source: "โมเดลแต่งเอง"}),
			mk(5, "tip", SceneContent{Title: "ส่วนที่จริงคือ", Rows: []ContentRow{{Text: "ความเป็นเจ้าของสินทรัพย์"}}}),
			mk(6, "cta", SceneContent{Title: "ปิดท้าย", CTA: "ทักมาเช็ค", Brand: "ADS VANCE"}),
		},
		Segments: []TranscriptSegment{{Text: "เปิด BM", Start: 0, End: 2}},
	}
}

func TestRenderMythFormat(t *testing.T) {
	out, err := RenderCompositionScenes(mythParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{
		`data-format="myth"`,
		"const FORMAT_MYTH = true",
		`data-layout="belief"`,
		`data-layout="proof"`,
		"ที่เชื่อกัน",
		"เปิด BM แล้วบัญชีแข็งกว่าบัญชีส่วนตัว",
		"เพราะเอเจนซีทุกที่ใช้ BM",
		"BM ให้ความเป็นเจ้าของ ไม่ใช่ภูมิคุ้มกัน",
		".mb-card",  // CSS ของการ์ดต้องอยู่ในไฟล์ที่ส่งออก
		".mb-meter", // มิเตอร์ 3 ระดับ
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML โหมด myth ขาด %q", want)
		}
	}
}

// หัวใจของสเปก: คำตัดสินและแหล่งอ้างต้องมาจาก DB ไม่ใช่จาก LLM ค่าที่โมเดลส่งมา
// ต้องถูกเขียนทับทุกครั้ง — ถ้าข้อนี้พัง คลิปจะตัดสินความเชื่อเองได้
func TestMythVerdictAndSourceAreGoInjected(t *testing.T) {
	out, err := RenderCompositionScenes(mythParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if strings.Contains(html, "โมเดลแต่งเอง") {
		t.Error("ค่าที่ LLM ส่งมายังอยู่ในผลลัพธ์ — Go ต้องเขียนทับ Meter/Source/Stamp")
	}
	for _, want := range []string{`"meter":"half_true"`, "จริงครึ่งเดียว", "Jon Loomer"} {
		if !strings.Contains(html, want) {
			t.Errorf("ผลลัพธ์ขาดค่าที่ Go ต้องฉีดเข้าไป: %q", want)
		}
	}
}

// CSS ของ myth ต้องไม่ไปแตะ verdict ของแฟ้มคดี — สอง format ใช้ layout ชื่อเดียวกัน
func TestMythCSSIsScoped(t *testing.T) {
	out, _ := RenderCompositionScenes(mythParams())
	html := string(out)
	idx := strings.Index(html, ".mb-card")
	if idx < 0 {
		t.Fatal("ไม่พบ .mb-card")
	}
	block := html[max0(idx-400):idx]
	if !strings.Contains(block, "[data-format='myth']") {
		t.Error("บล็อก CSS ของ myth ต้อง scope ด้วย [data-format='myth']")
	}
}

func max0(i int) int {
	if i < 0 {
		return 0
	}
	return i
}

// verdict ที่ผู้เรียกไม่ส่งมา (หรือส่งค่าที่ระบบไม่รู้จัก) ต้องไม่กลายเป็นมิเตอร์เปล่า
// บนคลิปที่เผยแพร่: หนึ่งขีดแดงพร้อมป้ายสามระดับโดยไม่มีตรายางคือกราฟิกที่แปลไม่ได้
func TestMythUnknownVerdictRendersNoMeter(t *testing.T) {
	p := mythParams()
	p.MythVerdict = "garbage"
	out, err := RenderCompositionScenes(p)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if strings.Contains(html, `"meter":`) {
		t.Error("payload มี meter ทั้งที่ verdict ไม่ใช่ค่าที่ระบบรู้จัก")
	}
	if strings.Contains(html, `"stamp":"`) {
		t.Error("payload มี stamp ทั้งที่ verdict ไม่ใช่ค่าที่ระบบรู้จัก")
	}
	// ตัว builder ต้องมี guard ด้วย ไม่ใช่พึ่งฝั่ง Go อย่างเดียว
	if !strings.Contains(html, "if(sc.meter){") {
		t.Error("builder ไม่มี guard sc.meter — payload ที่หลุดมาจากทางอื่นจะวาดมิเตอร์เปล่า")
	}
}

// ค่าที่ Go ฉีดทีหลัง (Stamp/Source) ต้องผ่านตัวประกบคำทับศัพท์ด้วย — guardSceneContent
// ทำงานจบไปก่อนการฉีดค่าเหล่านี้ ถ้าไม่ประกบซ้ำที่จุดฉีด ชื่อแหล่งอ้างที่มีคำทับศัพท์
// จะถูกเบราว์เซอร์หั่นกลางคำบนคลิปที่เผยแพร่ (บั๊กตระกูลนี้ยังเปิดอยู่บน prod)
func TestMythInjectedTextIsWordBreakGuarded(t *testing.T) {
	p := mythParams()
	p.MythSource = "รายงานรีชของเมต้า"
	out, err := RenderCompositionScenes(p)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	guarded := GuardLoanWords("รายงานรีชของเมต้า")
	if guarded == "รายงานรีชของเมต้า" {
		t.Skip("ตัวประกบไม่แตะข้อความนี้ — เทสต์นี้ไม่มีอะไรยืนยัน")
	}
	if !strings.Contains(html, guarded) {
		t.Error("ชื่อแหล่งอ้างในผลลัพธ์ไม่ได้ผ่าน GuardLoanWords")
	}
}
