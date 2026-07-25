package agent

import (
	"context"
	"fmt"

	"github.com/jaochai/video-fb/internal/models"
)

// FreshnessVerdict is the tutorial verifier's answer: has this feature's menu
// path moved since the catalog row was written?
type FreshnessVerdict struct {
	Moved  bool   `json:"moved"`
	Reason string `json:"reason"`
}

// TutorialVerifier asks a cheap model whether fresh web research contradicts a
// catalog row's menu path. It is a safety net, never a gate: the caller treats
// every error as "not moved" so a flaky web search can never stop the daily clip.
type TutorialVerifier struct {
	llm *KieLLMClient
}

func NewTutorialVerifier(llm *KieLLMClient) *TutorialVerifier {
	return &TutorialVerifier{llm: llm}
}

const tutorialVerifierSystem = `คุณคือผู้ตรวจความสดของเมนู Facebook Ads
ตอบเป็น JSON object เดียวเท่านั้น: {"moved": true|false, "reason": "เหตุผลสั้นๆ"}

ตั้ง moved = true เฉพาะเมื่อ**มีหลักฐานชัดเจน**ในข้อมูลที่ให้มาว่าเมนูนี้
ถูกย้ายที่ เปลี่ยนชื่อ หรือยกเลิกไปแล้ว
ถ้าไม่แน่ใจ ถ้าข้อมูลไม่พูดถึง หรือถ้าข้อมูลกำกวม ให้ตั้ง moved = false เสมอ
การเตือนผิดทำให้ระบบหยุดผลิตคลิปโดยไม่จำเป็น จึงต้องมั่นใจจริงเท่านั้น`

// Verify returns the verdict for one feature. cfg supplies model/temperature
// (the caller passes the cheap `research` agent row).
func (v *TutorialVerifier) Verify(ctx context.Context, menuPath, featureName, research string, cfg *models.AgentConfig) (FreshnessVerdict, error) {
	user := fmt.Sprintf(`ฟีเจอร์: %s
เส้นทางเมนูที่บันทึกไว้: %s

ข้อมูลจากการค้นเว็บล่าสุด:
%s

เส้นทางเมนูที่บันทึกไว้ยังใช้ได้อยู่ไหม`, featureName, menuPath, research)

	var out FreshnessVerdict
	if err := v.llm.GenerateJSON(ctx, cfg.Model, tutorialVerifierSystem, user, cfg.Temperature, &out); err != nil {
		return FreshnessVerdict{}, fmt.Errorf("tutorial freshness verify: %w", err)
	}
	return out, nil
}
