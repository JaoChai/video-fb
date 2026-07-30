package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
)

// content_format "myth" ต้องแมปไปโหมดและ preset ของตัวเอง ไม่ใช่ตกไปที่ค่า default
// (แฟ้มคดี) — ถ้าพลาดตรงนี้ คลิปจะเรนเดอร์เป็นแฟ้มคดี กินเลขคดี และ CSS ใหม่
// ไม่ทำงานเลย (บั๊กเดียวกับที่เกือบเกิดกับคลิป basic)
func TestMythFormatMapsToMythPreset(t *testing.T) {
	if got := clipMode(mythFormatName); got != producer.ModeMyth {
		t.Errorf("clipMode(myth) = %q ต้องเป็น %q", got, producer.ModeMyth)
	}
	if got := presetFor(mythFormatName); got.Key != producer.MythPreset.Key {
		t.Errorf("presetFor(myth).Key = %q ต้องเป็น %q", got.Key, producer.MythPreset.Key)
	}
}

// คลิป myth ต้องโหลดแถวคลังกลับมาตอน retry ไม่งั้นตะแกรงข้อเท็จจริงปิดเงียบ
// แล้วคลิปที่แต่งตัวเลขเองจะ auto-publish (retryFull วิ่งทุก 15 นาที)
func TestNeedsMythBelief(t *testing.T) {
	if !needsMythBelief(mythFormatName) {
		t.Error("needsMythBelief(myth) = false ต้องเป็น true")
	}
	for _, f := range []string{"qa", "tips", "tutorial", "basic", "case_story", "news"} {
		if needsMythBelief(f) {
			t.Errorf("needsMythBelief(%q) = true ต้องเป็น false", f)
		}
	}
}

// คลิป myth ต้องไม่ถูกนับเป็นคลิปที่ต้องโหลดคลัง tutorial — ถ้าปนกัน retry จะไปหา
// tutorial_features ด้วยคีย์ที่ไม่มีอยู่แล้ว failClip ทั้งที่คลิปยังกู้ได้
func TestMythIsNotACatalogTutorial(t *testing.T) {
	if needsCatalogFeature(mythFormatName) {
		t.Error("needsCatalogFeature(myth) = true — คลิป myth ไม่ได้สอน UI จริง")
	}
}

func gateBelief() *models.MythBelief {
	return &models.MythBelief{
		BeliefKey: "k", BeliefTH: "b", WhyBelievedTH: "w",
		Verdict: models.MythVerdictHalfTrue, FactTH: "f", SourceLabel: "s",
		NuanceTH: "n", CostTH: "เสียเวลา 3 วัน",
	}
}

// withRequiredScenes ต่อซีน verdict+proof ที่ตะแกรงบังคับให้มี เข้ากับซีนที่กำลังทดสอบ
// เพื่อให้แต่ละเทสต์วัดสิ่งที่มันตั้งใจวัด (ตัวเลข/คำอ้าง) ไม่ใช่ไปติดเรื่องจำนวนซีน
func withRequiredScenes(scenes ...agent.GeneratedScene) []agent.GeneratedScene {
	out := append([]agent.GeneratedScene{}, scenes...)
	return append(out,
		agent.GeneratedScene{SceneNumber: 90, Layout: "verdict", Content: json.RawMessage(`{}`)},
		agent.GeneratedScene{SceneNumber: 91, Layout: "proof", Content: json.RawMessage(`{}`)},
	)
}

// ตะแกรงต้องจับตัวเลขที่แต่งขึ้นและคำอ้างลอย และต้องปล่อยผ่านคลิปที่สะอาด
func TestMythGateFailure(t *testing.T) {
	clean := withRequiredScenes(agent.GeneratedScene{SceneNumber: 1, Layout: "hook",
		VoiceText: "เสียเวลา 3 วันโดยเปล่าประโยชน์", Content: json.RawMessage(`{}`)})
	if msg := mythGateFailure(clean, gateBelief()); msg != "" {
		t.Errorf("mythGateFailure(สะอาด) = %q ต้องเป็น \"\"", msg)
	}

	invented := withRequiredScenes(agent.GeneratedScene{SceneNumber: 1, Layout: "hook",
		VoiceText: "เสียเงิน 25000 บาท", Content: json.RawMessage(`{}`)})
	if msg := mythGateFailure(invented, gateBelief()); !strings.Contains(msg, "25000") {
		t.Errorf("mythGateFailure(ตัวเลขแต่ง) = %q ต้องบอกตัวเลขที่ผิด", msg)
	}

	claim := withRequiredScenes(agent.GeneratedScene{SceneNumber: 2, Layout: "proof",
		VoiceText: "บัญชีคุณอยู่ tier 2", Content: json.RawMessage(`{}`)})
	if msg := mythGateFailure(claim, gateBelief()); msg == "" {
		t.Error("mythGateFailure(คำอ้างลอย) = \"\" ต้องจับได้")
	}

	// ตัวเลขที่แต่งขึ้นในเนื้อการ์ด (content) ต้องถูกจับเหมือนตัวเลขที่พูด — DOM ของ
	// เทมเพลตอ่านจาก content ไม่ใช่จาก on_screen_text
	inCard := withRequiredScenes(agent.GeneratedScene{SceneNumber: 3, Layout: "proof",
		Content: json.RawMessage(`{"title":"เสียไป 99000 บาท"}`)})
	if msg := mythGateFailure(inCard, gateBelief()); !strings.Contains(msg, "99000") {
		t.Errorf("mythGateFailure(ตัวเลขในการ์ด) = %q ต้องจับได้", msg)
	}

	if msg := mythGateFailure(invented, nil); msg != "" {
		t.Errorf("mythGateFailure(nil belief) = %q ต้องเป็น \"\" (คลิปโหมดอื่น)", msg)
	}
}

// จำนวนซีนของคลิป myth ต้องคงที่ 6 ซีน ตามโครงในสเปก §6.2 — ไม่ผูกกับจำนวนขั้น
// อย่างคลิปสอน
func TestMythSceneShape(t *testing.T) {
	n, dur := mythSceneShape()
	if n != 6 {
		t.Errorf("mythSceneShape sceneCount = %d ต้องเป็น 6", n)
	}
	if dur < 45 || dur > 90 {
		t.Errorf("mythSceneShape duration = %d วินาที ควรอยู่ในช่วง 45-90", dur)
	}
}

// ตะแกรงต้องห่อ ErrContentGateBlocked เพื่อให้ ProduceMyth แยก "เนื้อหาผิด" ออกจาก
// "ระบบพัง" ได้ — สองกรณีนี้จัดการคลังหัวข้อคนละแบบ (ตกตะแกรง = พักแถวนั้น,
// ระบบพัง = ไม่แตะเพราะหัวข้อยังไม่ถูกใช้จริง)
func TestContentGateErrorIsIdentifiable(t *testing.T) {
	wrapped := fmt.Errorf("myth gate: %s: %w", "ตัวเลขไม่มีในคลัง", ErrContentGateBlocked)
	if !errors.Is(wrapped, ErrContentGateBlocked) {
		t.Error("error ของตะแกรงต้องระบุตัวได้ด้วย errors.Is")
	}
	if errors.Is(errors.New("kie เครดิตหมด"), ErrContentGateBlocked) {
		t.Error("error ของระบบต้องไม่ถูกนับเป็นตะแกรงเนื้อหา")
	}
}

// คลิปที่ขาดซีน verdict หรือ proof คือคลิปที่ไม่มีคำตัดสิน/แหล่งอ้างบนจอ ทั้งที่นั่นคือ
// แกนของรูปแบบนี้ — ตะแกรงต้องจับ "ของที่ไม่มี" ไม่ใช่แค่ "ของที่ผิด"
func TestMythGateRequiresVerdictAndProofScenes(t *testing.T) {
	full := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "hook", Content: json.RawMessage(`{}`)},
		{SceneNumber: 2, Layout: "belief", Content: json.RawMessage(`{}`)},
		{SceneNumber: 3, Layout: "verdict", Content: json.RawMessage(`{}`)},
		{SceneNumber: 4, Layout: "proof", Content: json.RawMessage(`{}`)},
	}
	if msg := mythGateFailure(full, gateBelief()); msg != "" {
		t.Errorf("mythGateFailure(ครบ) = %q ต้องผ่าน", msg)
	}

	noVerdict := append([]agent.GeneratedScene{}, full[:2]...)
	noVerdict = append(noVerdict, full[3])
	if msg := mythGateFailure(noVerdict, gateBelief()); !strings.Contains(msg, "verdict") {
		t.Errorf("mythGateFailure(ไม่มี verdict) = %q ต้องบอกว่าขาด verdict", msg)
	}

	if msg := mythGateFailure(full[:2], gateBelief()); !strings.Contains(msg, "proof") ||
		!strings.Contains(msg, "verdict") {
		t.Errorf("mythGateFailure(ขาดทั้งคู่) = %q ต้องบอกทั้งสอง layout", msg)
	}
}

// verdict "outdated" มีตรายางยาวสุด (25 ตัวอักษร) — ต้องผ่านทั้งตะแกรงและได้คำแปล
func TestMythOutdatedVerdictHasLabel(t *testing.T) {
	if got := agent.MythVerdictLabelTH(models.MythVerdictOutdated); got == "" {
		t.Error("verdict outdated ไม่มีคำแปลไทย — ตรายางจะว่าง")
	}
}

// ตัวตรวจ title/description ของ YouTube ส่งข้อความมาในรูปซีนสมมุติหนึ่งตัว — ต้องไม่ติด
// กฎ "ต้องมีซีน verdict/proof" ไม่งั้นคลิปทุกตัวถูกบล็อกที่ขั้น metadata = ช่องตายสนิท
func TestMythTextGateIgnoresSceneShape(t *testing.T) {
	metaAsScene := []agent.GeneratedScene{{
		SceneNumber: 0, VoiceText: "เปิด BM แล้วบัญชีแข็งกว่าจริงไหม",
		OnScreenText: "คำตอบอยู่ในคลิป", Content: json.RawMessage(`{}`),
	}}
	if msg := mythTextGateFailure(metaAsScene, gateBelief()); msg != "" {
		t.Errorf("mythTextGateFailure(metadata สะอาด) = %q ต้องผ่าน", msg)
	}
	// แต่ยังต้องจับตัวเลขที่แต่งขึ้นใน title ได้
	metaAsScene[0].VoiceText = "เสียฟรี 88000 บาทเพราะเชื่อเรื่องนี้"
	if msg := mythTextGateFailure(metaAsScene, gateBelief()); !strings.Contains(msg, "88000") {
		t.Errorf("mythTextGateFailure(ตัวเลขแต่งใน title) = %q ต้องจับได้", msg)
	}
	// และตัวตรวจชุดซีนเต็มยังต้องบังคับรูปร่างคลิปอยู่ (ใช้ซีนสะอาด ไม่ให้ติดตัวเลขก่อน)
	oneClean := []agent.GeneratedScene{{SceneNumber: 0, Layout: "hook",
		VoiceText: "เปิดคลิปแบบไม่มีตัวเลข", Content: json.RawMessage(`{}`)}}
	if msg := mythGateFailure(oneClean, gateBelief()); !strings.Contains(msg, "verdict") {
		t.Errorf("mythGateFailure ต้องยังบังคับซีนที่ต้องมี ได้ %q", msg)
	}
}

// เส้นทาง metadata ต้องเรียก mythTextGateFailure ไม่ใช่ mythGateFailure — สลับกันแล้ว
// คลิปทุกตัวถูกบล็อกที่ขั้น metadata (ช่องตายสนิท) แต่เทสต์ระดับฟังก์ชันจับไม่ได้เพราะ
// มันไม่รู้ว่า call site เรียกตัวไหน
//
// เทสต์นี้อ่านซอร์สจริง ไม่ใช่รันพฤติกรรม (เส้นทางนั้นต้องมี DB + LLM ครบ) — อ่อนกว่า
// เทสต์พฤติกรรม แต่เป็นสิ่งเดียวที่กัน regression ตรงจุดนี้ได้โดยไม่ต้องมีของจริง
func TestMetadataGateUsesTextOnlyChecker(t *testing.T) {
	src, err := os.ReadFile("orchestrator.go")
	if err != nil {
		t.Fatalf("อ่าน orchestrator.go: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, "mythTextGateFailure(metaAsScene") {
		t.Error("เส้นทาง metadata ไม่ได้ใช้ mythTextGateFailure — คลิปจะถูกบล็อกทุกตัว")
	}
	if strings.Contains(s, "mythGateFailure(metaAsScene") {
		t.Error("เส้นทาง metadata ใช้ตัวตรวจที่บังคับรูปร่างซีน = ช่องตายสนิท")
	}
}
