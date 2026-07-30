package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
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

// ตะแกรงต้องจับตัวเลขที่แต่งขึ้นและคำอ้างลอย และต้องปล่อยผ่านคลิปที่สะอาด
func TestMythGateFailure(t *testing.T) {
	clean := []agent.GeneratedScene{{SceneNumber: 1, Layout: "hook",
		VoiceText: "เสียเวลา 3 วันโดยเปล่าประโยชน์", Content: json.RawMessage(`{}`)}}
	if msg := mythGateFailure(clean, gateBelief()); msg != "" {
		t.Errorf("mythGateFailure(สะอาด) = %q ต้องเป็น \"\"", msg)
	}

	invented := []agent.GeneratedScene{{SceneNumber: 1, Layout: "hook",
		VoiceText: "เสียเงิน 25000 บาท", Content: json.RawMessage(`{}`)}}
	if msg := mythGateFailure(invented, gateBelief()); !strings.Contains(msg, "25000") {
		t.Errorf("mythGateFailure(ตัวเลขแต่ง) = %q ต้องบอกตัวเลขที่ผิด", msg)
	}

	claim := []agent.GeneratedScene{{SceneNumber: 2, Layout: "proof",
		VoiceText: "บัญชีคุณอยู่ tier 2", Content: json.RawMessage(`{}`)}}
	if msg := mythGateFailure(claim, gateBelief()); msg == "" {
		t.Error("mythGateFailure(คำอ้างลอย) = \"\" ต้องจับได้")
	}

	// ตัวเลขที่แต่งขึ้นในเนื้อการ์ด (content) ต้องถูกจับเหมือนตัวเลขที่พูด — DOM ของ
	// เทมเพลตอ่านจาก content ไม่ใช่จาก on_screen_text
	inCard := []agent.GeneratedScene{{SceneNumber: 3, Layout: "proof",
		Content: json.RawMessage(`{"title":"เสียไป 99000 บาท"}`)}}
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
