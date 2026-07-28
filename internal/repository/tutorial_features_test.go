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
	if TutorialMinPool < 10 {
		t.Errorf("TutorialMinPool = %d — must be >= 10, the floor guards against a catalog that shrinks until it repeats too often", TutorialMinPool)
	}
}

// กันบทเรียนกลับมา: verdict "เมนูย้าย" ครั้งเดียวพักฟีเจอร์ทันที ข้อมูลจริงพัก
// 7 ครั้งจาก ~5 รอบผลิต — ParkOutcome 3 ค่าต้องแยกแยะได้และไม่มีค่าไหนว่าง
func TestParkOutcomesAreDistinct(t *testing.T) {
	outcomes := []ParkOutcome{ParkFirstStrike, ParkedForVerify, ParkRefusedFloor}
	seen := map[ParkOutcome]bool{}
	for _, o := range outcomes {
		if o == "" {
			t.Errorf("ParkOutcome value must not be empty, got %q", o)
		}
		if seen[o] {
			t.Errorf("ParkOutcome %q is not distinct from another outcome", o)
		}
		seen[o] = true
	}
}

// เทสต์ยืนยันโครงสร้าง SQL ของ Park — strike แรกต้องอ้าง flagged_at, strike ที่สอง
// ต้องมีทั้ง parked_until และ subquery นับพื้นคลัง มิฉะนั้น "พักครั้งเดียว" หรือ
// "พักโดยไม่นับพื้น" จะกลับมาเงียบๆ โดยไม่มีเทสต์จับ
func TestParkStrikeSQLShape(t *testing.T) {
	if !strings.Contains(tutorialFirstStrikeSQL, "flagged_at") {
		t.Error("first-strike statement must reference flagged_at")
	}
	if strings.Contains(tutorialFirstStrikeSQL, "parked_until") {
		t.Error("first-strike statement must not touch parked_until — it only records the flag")
	}
	if !strings.Contains(tutorialSecondStrikeSQL, "parked_until") {
		t.Error("second-strike statement must set parked_until — this is the statement that actually parks")
	}
	if !strings.Contains(tutorialSecondStrikeSQL, "COUNT(*)") {
		t.Error("second-strike statement must count the available floor in the same statement as the park")
	}
	if !strings.Contains(tutorialSecondStrikeSQL, "flagged_at = NULL") {
		t.Error("second-strike statement must clear flagged_at so the next window starts fresh")
	}
}

// $3 หมายถึงคนละค่ากันระหว่าง 2 statement (30 วันหน้าต่าง vs 14 วันพัก) —
// เทสต์นี้กันไม่ให้ใครมาลดรูปเป็นค่าเดียวกันเพราะดู $3 เหมือนกันแล้วเข้าใจผิด
func TestStrikeWindowAndParkDaysAreDifferent(t *testing.T) {
	if tutorialStrikeWindowDays == tutorialParkDays {
		t.Errorf("tutorialStrikeWindowDays (%d) and tutorialParkDays (%d) collapsed to the same value — "+
			"they mean different things: the strike window vs how long a park lasts",
			tutorialStrikeWindowDays, tutorialParkDays)
	}
}

// คลัง basic กับ advanced อยู่ตารางเดียวกันแต่เป็นคนละคิว ถ้าพื้นนับรวมกัน
// การพักแถว basic จะผ่านตลอดเพราะไปนับแถว advanced ที่ยังว่าง แล้วคลัง basic
// ก็ยุบจนคลิป 15:00 วนซ้ำทุกไม่กี่วัน — อาการเดียวกับบั๊กเดิมแต่คนละคลัง
func TestParkFloorCountsOnlyTheSameLevel(t *testing.T) {
	if !strings.Contains(tutorialSecondStrikeSQL, "t2.level = tutorial_features.level") {
		t.Error("park floor subquery must be scoped to the row's own level")
	}
	if !strings.Contains(tutorialPickSQL, "level = $1") {
		t.Error("PickNext must filter by level — one table, two independent queues")
	}
}

func TestTutorialLevelsAreDistinct(t *testing.T) {
	if TutorialLevelAdvanced == TutorialLevelBasic {
		t.Fatal("levels must differ")
	}
	for _, l := range []string{TutorialLevelAdvanced, TutorialLevelBasic} {
		if strings.TrimSpace(l) == "" {
			t.Error("level constants must not be empty — an empty level would match no rows")
		}
	}
}

func TestPickTutorialFeatureEmptyPool(t *testing.T) {
	if got := pickTutorialFeatureLeastUsed(nil, nil); got.FeatureKey != "" {
		t.Errorf("empty pool must return the zero feature, got %q", got.FeatureKey)
	}
}
