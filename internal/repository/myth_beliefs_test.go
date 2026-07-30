package repository

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

// ตัวเลือกต้องหยิบแถวที่ใช้น้อยที่สุด ไม่ใช่แถวแรกตามตัวอักษร ไม่งั้นคลิปวนซ้ำ
// หัวข้อเดิมทั้งที่คลังมีของอีกเป็นสิบแถว
func TestPickMythLeastUsed(t *testing.T) {
	got := pickMythLeastUsed([]mythUsage{
		{Belief: models.MythBelief{BeliefKey: "a"}, UsedCount: 5},
		{Belief: models.MythBelief{BeliefKey: "b"}, UsedCount: 1},
		{Belief: models.MythBelief{BeliefKey: "c"}, UsedCount: 3},
	})
	if got.BeliefKey != "b" {
		t.Errorf("pickMythLeastUsed = %q, ต้องเป็น \"b\" (ใช้น้อยสุด)", got.BeliefKey)
	}
}

// weight สูงต้องมีโอกาสถูกหยิบมากกว่าเมื่อ used_count เท่ากัน — เทสต์ระดับที่จับได้
// จริงคือ "แถว weight 0 ต้องไม่ถูกหยิบก่อนแถว weight 3 ที่ใช้เท่ากัน"
func TestPickMythPrefersWeight(t *testing.T) {
	got := pickMythLeastUsed([]mythUsage{
		{Belief: models.MythBelief{BeliefKey: "low", Weight: 0}, UsedCount: 2},
		{Belief: models.MythBelief{BeliefKey: "high", Weight: 3}, UsedCount: 2},
	})
	if got.BeliefKey != "high" {
		t.Errorf("pickMythLeastUsed = %q, ต้องเป็น \"high\"", got.BeliefKey)
	}
}

// เงื่อนไข "แถวที่ใช้ได้ตอนนี้" ต้องกันแถวที่ยังไม่ได้ยืนยันข้อเท็จจริงออกไป —
// แถว needs_verify คือแถวที่ยังหาแหล่งอ้างไม่ได้ ปล่อยให้หลุดไปเท่ากับคลิปพูดสิ่งที่
// ไม่มีใครยืนยัน
func TestMythPickSQLExcludesUnverified(t *testing.T) {
	for _, want := range []string{"needs_verify = FALSE", "enabled = TRUE", "parked_until"} {
		if !strings.Contains(mythPickSQL, want) {
			t.Errorf("mythPickSQL ขาดเงื่อนไข %q", want)
		}
	}
}

// พื้นกันคลังยุบต้องอยู่ใน UPDATE เดียวกับการ park ไม่ใช่แยกคำสั่ง — ไม่งั้นสองรอบ
// ที่รันชนกันจะเห็น "เหลือแถวสุดท้าย" พร้อมกันแล้ว park ทิ้งทั้งคู่
func TestMythMarkUsedHasFloorInSameStatement(t *testing.T) {
	if !strings.Contains(mythMarkUsedSQL, "used_count = used_count + 1") {
		t.Error("mythMarkUsedSQL ต้องบวก used_count")
	}
	if !strings.Contains(mythMarkUsedSQL, "SELECT COUNT(*)") {
		t.Error("mythMarkUsedSQL ต้องนับคลังในคำสั่งเดียวกัน (พื้นกันคลังยุบ)")
	}
}
