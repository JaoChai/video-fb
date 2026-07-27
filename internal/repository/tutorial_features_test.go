package repository

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func tf(key string, weight int) models.TutorialFeature {
	return models.TutorialFeature{FeatureKey: key, Weight: weight, Enabled: true}
}

func TestPickTutorialFeatureLeastUsed(t *testing.T) {
	usages := []tutorialUsage{
		{Feat: tf("a", 1), UsedCount: 3},
		{Feat: tf("b", 1), UsedCount: 1},
		{Feat: tf("c", 1), UsedCount: 5},
	}
	if got := pickTutorialFeatureLeastUsed(usages, nil); got.FeatureKey != "b" {
		t.Errorf("picked %q, want b (lowest used/weight)", got.FeatureKey)
	}
}

func TestPickTutorialFeatureRespectsWeight(t *testing.T) {
	usages := []tutorialUsage{
		{Feat: tf("a", 1), UsedCount: 2}, // ratio 2.0
		{Feat: tf("b", 4), UsedCount: 4}, // ratio 1.0
	}
	if got := pickTutorialFeatureLeastUsed(usages, nil); got.FeatureKey != "b" {
		t.Errorf("picked %q, want b (weight lowers the ratio)", got.FeatureKey)
	}
}

func TestPickTutorialFeatureExcludes(t *testing.T) {
	usages := []tutorialUsage{
		{Feat: tf("a", 1), UsedCount: 0},
		{Feat: tf("b", 1), UsedCount: 9},
	}
	if got := pickTutorialFeatureLeastUsed(usages, []string{"a"}); got.FeatureKey != "b" {
		t.Errorf("picked %q, want b (a is excluded)", got.FeatureKey)
	}
}

// กันบทเรียน cooldown_deadlock: ถ้า exclude กินหมด ต้อง fail-open ไม่ใช่คืนค่าว่าง
func TestPickTutorialFeatureFailsOpenWhenAllExcluded(t *testing.T) {
	usages := []tutorialUsage{
		{Feat: tf("a", 1), UsedCount: 1},
		{Feat: tf("b", 1), UsedCount: 0},
	}
	got := pickTutorialFeatureLeastUsed(usages, []string{"a", "b"})
	if got.FeatureKey == "" {
		t.Fatal("must fail open with a pick, never an empty feature")
	}
}

// กันบั๊ก 2026-07-27 กลับมา: PickNext เคยกรองด้วย needs_verify ซึ่งตั้งเป็น TRUE
// ได้อย่างเดียว ไม่มีทางปลด คลังจึงหดลงรอบละไม่เกิน 2 แถวจนเหลือแถวเดียว
// แล้วคลิปสอนก็ซ้ำหัวข้อเดิมทุกวัน เงื่อนไข "หยิบได้ตอนนี้" ต้องหมดอายุเองตามเวลา
func TestAvailableFilterExpiresInsteadOfLatching(t *testing.T) {
	if strings.Contains(tutorialAvailableWhere, "needs_verify") {
		t.Error("availability must not depend on needs_verify — nothing ever sets it back to FALSE")
	}
	if !strings.Contains(tutorialAvailableWhere, "parked_until") {
		t.Error("availability must be time-based so a parked feature returns on its own")
	}
	if TutorialMinPool < 2 {
		t.Errorf("TutorialMinPool = %d — the floor has to leave room for a different clip tomorrow", TutorialMinPool)
	}
}

func TestPickTutorialFeatureEmptyPool(t *testing.T) {
	if got := pickTutorialFeatureLeastUsed(nil, nil); got.FeatureKey != "" {
		t.Errorf("empty pool must return the zero feature, got %q", got.FeatureKey)
	}
}
