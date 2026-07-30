package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func mythFixture() *models.MythBelief {
	return &models.MythBelief{
		BeliefKey:     "bm_stronger_than_personal",
		BeliefTH:      "เปิด BM แล้วบัญชีแข็งกว่าใช้บัญชีส่วนตัว",
		WhyBelievedTH: "เพราะเอเจนซีทุกที่ใช้ BM",
		Verdict:       models.MythVerdictHalfTrue,
		FactTH:        "BM ให้ความเป็นเจ้าของสินทรัพย์และสิทธิ์ทีม ไม่ได้ลดโอกาสถูกแบน",
		SourceLabel:   "Jon Loomer",
		NuanceTH:      "ที่จริงคือเรื่องความเป็นเจ้าของเพจและ pixel",
		CostTH:        "ย้ายบัญชีเสียเวลา 3 วันโดยไม่ได้อะไรเพิ่ม",
	}
}

func sceneWithVoice(n int, layout, voice, onScreen string) GeneratedScene {
	return GeneratedScene{SceneNumber: n, Layout: layout, VoiceText: voice, OnScreenText: onScreen,
		Content: json.RawMessage(`{}`)}
}

// ตัวเลขข้อเท็จจริงที่ไม่มีในแถวคลังคือสิ่งที่โมเดลแต่งขึ้น — ต้องจับได้ ไม่งั้นคลิป
// จะเผยแพร่ตัวเลขที่ไม่มีใครยืนยัน
func TestFactNumberViolationsCatchesInventedNumber(t *testing.T) {
	scenes := []GeneratedScene{
		sceneWithVoice(1, "hook", "เสียเงินฟรี 25000 บาททุกเดือน", "เสียเงินฟรี"),
	}
	v := FactNumberViolations(scenes, mythFixture())
	if len(v) == 0 {
		t.Fatal("FactNumberViolations = ว่าง แต่ 25000 ไม่มีในแถวคลัง")
	}
	if !strings.Contains(v[0], "25000") {
		t.Errorf("ข้อความตำหนิควรบอกตัวเลขที่ผิด ได้ %q", v[0])
	}
}

// ตัวเลขที่มาจากแถวคลังต้องผ่าน ไม่งั้นตะแกรงจะตีคลิปที่ถูกต้องตกทุกคืน
func TestFactNumberViolationsAllowsCatalogNumber(t *testing.T) {
	scenes := []GeneratedScene{
		sceneWithVoice(1, "hook", "ย้ายบัญชีเสียเวลา 3 วันโดยเปล่าประโยชน์", "เสียเวลา 3 วัน"),
	}
	if v := FactNumberViolations(scenes, mythFixture()); len(v) > 0 {
		t.Errorf("FactNumberViolations = %v แต่ 3 มาจาก cost_th", v)
	}
}

// เลขลำดับขั้น/ข้อ ไม่ใช่ตัวเลขข้อเท็จจริง ตะแกรงต้องไม่จับ
func TestFactNumberViolationsIgnoresOrdinals(t *testing.T) {
	scenes := []GeneratedScene{
		sceneWithVoice(1, "proof", "ข้อ 2 ที่ต้องจำ", "ข้อ 2"),
	}
	if v := FactNumberViolations(scenes, mythFixture()); len(v) > 0 {
		t.Errorf("FactNumberViolations = %v แต่ \"ข้อ 2\" เป็นเลขลำดับ", v)
	}
}

// คลิป myth ที่ไม่มีแถวคลัง (nil) = คลิปโหมดอื่น ตะแกรงต้องไม่ทำงาน
func TestFactNumberViolationsNilBelief(t *testing.T) {
	if v := FactNumberViolations([]GeneratedScene{sceneWithVoice(1, "hook", "999", "")}, nil); v != nil {
		t.Errorf("FactNumberViolations(nil) = %v ต้องเป็น nil", v)
	}
}

// คำอ้างลอย (trust score/tier/HiVA) ใช้ได้เฉพาะแถวที่มี source_url จริง
func TestDisallowedClaimViolations(t *testing.T) {
	b := mythFixture() // SourceURL ว่าง
	scenes := []GeneratedScene{sceneWithVoice(1, "proof", "บัญชีคุณอยู่ tier 2", "tier 2")}
	if v := DisallowedClaimViolations(scenes, b); len(v) == 0 {
		t.Fatal("DisallowedClaimViolations = ว่าง แต่พูด tier โดยไม่มี source_url")
	}
	b.SourceURL = "https://www.facebook.com/business/help/xxxx"
	if v := DisallowedClaimViolations(scenes, b); len(v) > 0 {
		t.Errorf("DisallowedClaimViolations = %v แต่แถวนี้มี source_url แล้ว", v)
	}
}

// brief ต้องขนข้อมูลครบทุกฟิลด์ที่ซีนต้องใช้ ขาดฟิลด์ใดฟิลด์หนึ่ง = ซีนนั้นว่าง
func TestMythBriefCarriesEveryField(t *testing.T) {
	got := MythBrief(mythFixture())
	for _, want := range []string{
		"เปิด BM แล้วบัญชีแข็งกว่าใช้บัญชีส่วนตัว",
		"เพราะเอเจนซีทุกที่ใช้ BM",
		"BM ให้ความเป็นเจ้าของสินทรัพย์",
		"Jon Loomer",
		"ที่จริงคือเรื่องความเป็นเจ้าของเพจ",
		"ย้ายบัญชีเสียเวลา 3 วัน",
		"จริงครึ่งเดียว",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("MythBrief ขาด %q", want)
		}
	}
	if MythBrief(nil) != "" {
		t.Error("MythBrief(nil) ต้องเป็น \"\"")
	}
}

// Fix รอบ 1 (Critical): ตัวเลขที่โมเดลแต่งแล้วยัดเข้า content.* ขึ้นจอเหมือนกัน
// ตะแกรงที่ตรวจแค่ voice/on_screen จะมองไม่เห็น — ต้องสแกน Content ด้วย
func TestFactNumberViolationsScansContent(t *testing.T) {
	s := sceneWithVoice(2, "proof", "นี่คือคำอธิบายสะอาด", "")
	s.Content = json.RawMessage(`{"title":"เสียเงิน 25000 บาท","rows":[{"t":"งบ 40000 หายไป"}]}`)
	v := FactNumberViolations([]GeneratedScene{s}, mythFixture())
	if len(v) != 2 {
		t.Fatalf("ต้องได้ตำหนิ 2 รายการ ได้ %d: %v", len(v), v)
	}
	joined := strings.Join(v, " ")
	if !strings.Contains(joined, "25000") || !strings.Contains(joined, "40000") {
		t.Errorf("ต้องมีทั้ง 25000 และ 40000 ได้ %v", v)
	}
	for _, msg := range v {
		if !strings.Contains(msg, "content") {
			t.Errorf("ข้อความตำหนิต้องระบุ field content: %q", msg)
		}
	}
}

// เลขลำดับที่อยู่ใน content ต้องถูกปล่อยเหมือน voice/on_screen
func TestFactNumberViolationsContentOrdinalOK(t *testing.T) {
	s := sceneWithVoice(1, "proof", "", "")
	s.Content = json.RawMessage(`{"title":"ข้อ 2 ที่ต้องจำ"}`)
	if v := FactNumberViolations([]GeneratedScene{s}, mythFixture()); len(v) > 0 {
		t.Errorf("ต้องไม่มีตำหนิ (เลขลำดับ) ได้ %v", v)
	}
}

// content ที่ไม่ใช่ JSON ต้องไม่ทำให้ตะแกรง panic และไม่อ้างตัวเลขจากมัน
func TestFactNumberViolationsBadContentJSON(t *testing.T) {
	s := sceneWithVoice(1, "proof", "", "")
	s.Content = json.RawMessage("not json")
	v := FactNumberViolations([]GeneratedScene{s}, mythFixture())
	if len(v) != 0 {
		t.Errorf("content ไม่ใช่ JSON ต้องไม่มีตำหนิ ได้ %v", v)
	}
}

// Fix รอบ 1 (Critical): คำห้ามใน content ต้องถูกจับเหมือนคำพูด
func TestDisallowedClaimViolationsScansContent(t *testing.T) {
	b := mythFixture() // SourceURL ว่าง
	s := sceneWithVoice(1, "proof", "", "")
	s.Content = json.RawMessage(`{"title":"บัญชีอยู่ tier 2"}`)
	if v := DisallowedClaimViolations([]GeneratedScene{s}, b); len(v) == 0 {
		t.Fatal("ต้องติดเพราะ content อ้าง tier โดยไม่มี source_url")
	}
}

// Fix รอบ 1 (Important): "ที่" เดี่ยวๆ กว้างเกินไป — "งบที่ 40000" ไม่ใช่ลำดับ
func TestFactNumberViolationsBareThiIsNotOrdinal(t *testing.T) {
	scenes := []GeneratedScene{
		sceneWithVoice(1, "hook", "งบที่ 40000 หายไป", ""),
	}
	if v := FactNumberViolations(scenes, mythFixture()); len(v) == 0 {
		t.Fatal("40000 ต้องติดเพราะ \"งบที่\" ไม่ใช่คำนำหน้าลำดับ")
	}
}

// ตัวเลขที่เขียนต่างรูปแต่ค่าเท่ากันต้องนับเป็นตัวเดียวกัน ไม่งั้นตะแกรงตีคลิปที่ถูกต้อง
// เข้า needs_review เพราะโมเดลเขียน "3.0" ขณะที่คลังเขียน "3 วัน" — เสียแรงคนตรวจฟรี
func TestFactNumberViolationsNormalizesDecimals(t *testing.T) {
	scenes := []GeneratedScene{
		sceneWithVoice(1, "hook", "ย้ายบัญชีเสียเวลา 3.0 วัน", ""),
	}
	if v := FactNumberViolations(scenes, mythFixture()); len(v) > 0 {
		t.Errorf("FactNumberViolations = %v แต่ 3.0 มีค่าเท่ากับ 3 ใน cost_th", v)
	}
	// คนละค่ากันต้องยังจับได้
	scenes[0].VoiceText = "ย้ายบัญชีเสียเวลา 3.5 วัน"
	if v := FactNumberViolations(scenes, mythFixture()); len(v) == 0 {
		t.Error("FactNumberViolations = ว่าง แต่ 3.5 ไม่มีในแถวคลัง")
	}
}
