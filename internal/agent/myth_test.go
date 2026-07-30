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
