package scoreboard

import (
	"fmt"
	"testing"
)

func sum(m map[string]int) int {
	t := 0
	for _, v := range m {
		t += v
	}
	return t
}

func TestCombinePlatformsWeightsByN(t *testing.T) {
	scores := []Score{
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30}, ScoreFinal: 0.8},
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "tiktok", N: 10}, ScoreFinal: 0.4},
		{Stat: Stat{Dimension: "category", Value: "payment", Platform: "youtube", N: 9}, ScoreFinal: 0.9},
	}
	got := CombinePlatforms(scores, "content_format")
	if len(got) != 1 {
		t.Fatalf("คืน %d รายการ, want 1 (กรองมิติอื่นออก)", len(got))
	}
	// (30*0.8 + 10*0.4) / 40 = 0.7
	if got[0].ScoreFinal < 0.699 || got[0].ScoreFinal > 0.701 {
		t.Errorf("ScoreFinal = %v, want 0.7", got[0].ScoreFinal)
	}
	if got[0].N != 40 {
		t.Errorf("N = %d, want 40", got[0].N)
	}
}

func TestTuneWeightsMovesOnlyAQuarterOfTheWay(t *testing.T) {
	current := map[string]int{"qa": 25, "news": 25, "tips": 25, "case_story": 25}
	combined := []Combined{
		{Value: "qa", ScoreFinal: 0.9, N: 30},
		{Value: "news", ScoreFinal: 0.3, N: 30},
		{Value: "tips", ScoreFinal: 0.5, N: 30},
		{Value: "case_story", ScoreFinal: 0.5, N: 30},
	}
	got := TuneWeights(current, combined, 8, 0.25)
	if sum(got) != 100 {
		t.Errorf("ผลรวม = %d, want 100", sum(got))
	}
	// qa ชนะ ต้องขึ้น แต่ห้ามพุ่งเกิน 25 + (เพดาน 50 - 25)*0.25 = 31.25 → ปัดแล้วไม่เกิน 32
	if got["qa"] <= 25 || got["qa"] > 32 {
		t.Errorf("qa = %d, ต้องมากกว่า 25 แต่ไม่เกิน 32 (เคลื่อน 25%%)", got["qa"])
	}
	if got["news"] >= 25 {
		t.Errorf("news = %d, ต้องลดลงจาก 25", got["news"])
	}
}

func TestTuneWeightsRespectsFloorAndCeiling(t *testing.T) {
	current := map[string]int{"a": 25, "b": 25, "c": 25, "d": 25}
	// a ชนะขาด อีกสามตัวศูนย์ — วิ่งซ้ำจนลู่เข้าที่ ต้องไม่มีใครหลุดพื้น 12.5%
	combined := []Combined{
		{Value: "a", ScoreFinal: 1.0, N: 50},
		{Value: "b", ScoreFinal: 0.01, N: 50},
		{Value: "c", ScoreFinal: 0.01, N: 50},
		{Value: "d", ScoreFinal: 0.01, N: 50},
	}
	w := current
	for i := 0; i < 40; i++ {
		w = TuneWeights(w, combined, 8, 0.25)
	}
	if sum(w) != 100 {
		t.Errorf("ผลรวมหลังลู่เข้า = %d, want 100", sum(w))
	}
	// พื้น = 0.5 * (1/4) = 12.5% → อย่างน้อย 12 หลังปัดเศษ
	for _, k := range []string{"b", "c", "d"} {
		if w[k] < 12 {
			t.Errorf("%s = %d หลุดพื้น 12.5%%", k, w[k])
		}
	}
	// เพดาน = 2 * 25% = 50%
	if w["a"] > 50 {
		t.Errorf("a = %d ทะลุเพดาน 50", w["a"])
	}
}

func TestTuneWeightsFreezesLowSampleValues(t *testing.T) {
	current := map[string]int{"qa": 40, "news": 40, "tutorial": 20}
	combined := []Combined{
		{Value: "qa", ScoreFinal: 0.9, N: 30},
		{Value: "news", ScoreFinal: 0.3, N: 30},
		{Value: "tutorial", ScoreFinal: 0.99, N: 2}, // n < 8 → ตรึง ห้ามได้ประโยชน์จากคะแนนสูง
	}
	got := TuneWeights(current, combined, 8, 0.25)
	if got["tutorial"] != 20 {
		t.Errorf("tutorial = %d, ต้องถูกตรึงไว้ที่ 20", got["tutorial"])
	}
	if sum(got) != 100 {
		t.Errorf("ผลรวม = %d, want 100", sum(got))
	}
}

func TestTuneWeightsEqualScoresStayUniform(t *testing.T) {
	current := map[string]int{"a": 33, "b": 33, "c": 34}
	combined := []Combined{
		{Value: "a", ScoreFinal: 0.5, N: 20},
		{Value: "b", ScoreFinal: 0.5, N: 20},
		{Value: "c", ScoreFinal: 0.5, N: 20},
	}
	got := TuneWeights(current, combined, 8, 0.25)
	for k, v := range got {
		if v < 32 || v > 35 {
			t.Errorf("%s = %d, คะแนนเท่ากันต้องอยู่ใกล้ uniform", k, v)
		}
	}
	if sum(got) != 100 {
		t.Errorf("ผลรวม = %d, want 100", sum(got))
	}
}

func TestTuneWeightsNoScoresIsNoOp(t *testing.T) {
	current := map[string]int{"a": 60, "b": 40}
	got := TuneWeights(current, nil, 8, 0.25)
	if got["a"] != 60 || got["b"] != 40 {
		t.Errorf("ไม่มีคะแนน ต้องคืนค่าเดิม ได้ %v", got)
	}
}

func TestTuneWeightsFloorCeilingWithVariousAlpha(t *testing.T) {
	// ทดสอบเสถียรภาพของ floor/ceiling ข้ามค่า alpha ต่าง ๆ
	// การจัดสรรใหม่อย่างรุนแรง (alpha สูง) ต้องไม่ทะลุขอบ
	for _, alpha := range []float64{0.25, 0.3, 0.4, 0.5} {
		t.Run(fmt.Sprintf("alpha=%.2f", alpha), func(t *testing.T) {
			current := map[string]int{"a": 25, "b": 25, "c": 25, "d": 25}
			combined := []Combined{
				{Value: "a", ScoreFinal: 1.0, N: 50},
				{Value: "b", ScoreFinal: 0.01, N: 50},
				{Value: "c", ScoreFinal: 0.01, N: 50},
				{Value: "d", ScoreFinal: 0.01, N: 50},
			}
			w := current
			for i := 0; i < 40; i++ {
				w = TuneWeights(w, combined, 8, alpha)
			}
			if sum(w) != 100 {
				t.Errorf("ผลรวม = %d, want 100", sum(w))
			}
			// พื้น = 0.5 * (1/4) = 12.5% → อย่างน้อย 12 หลังปัดเศษ
			for _, k := range []string{"b", "c", "d"} {
				if w[k] < 12 {
					t.Errorf("%s = %d หลุดพื้น 12.5%% ที่ alpha=%.2f", k, w[k], alpha)
				}
			}
			// เพดาน = 2 * 25% = 50%
			if w["a"] > 50 {
				t.Errorf("a = %d ทะลุเพดาน 50 ที่ alpha=%.2f", w["a"], alpha)
			}
		})
	}
}

func TestTuneWeightsMutationBug(t *testing.T) {
	// ตรวจว่า TuneWeights ไม่ทำให้ caller's map เปลี่ยนแปลง
	current := map[string]int{"a": 0, "b": 0}
	originalA := current["a"]
	originalB := current["b"]
	combined := []Combined{
		{Value: "a", ScoreFinal: 0.5, N: 20},
		{Value: "b", ScoreFinal: 0.5, N: 20},
	}
	TuneWeights(current, combined, 8, 0.25)
	if current["a"] != originalA || current["b"] != originalB {
		t.Errorf("TuneWeights ไม่ควรแตะ caller's map — เปลี่ยนจาก {%d,%d} เป็น {%d,%d}",
			originalA, originalB, current["a"], current["b"])
	}
}

func TestTuneWeightsSaturatedRemainderTerminates(t *testing.T) {
	// ทดสอบ no-progress guard: ออกแบบให้ truncation loop ปล่อยเศษ assigned < 100
	// และบางค่า movable นั้นแล้วอยู่ที่ ceiling (ceilInt) ทำให้ remainder loop
	// skip บ่อยแล้ว no progress คร่ำครวญ
	//
	// With 4 values: uniform = 0.25, low = 0.125, high = 0.5 (ceilInt = 50)
	// ตั้งค่าเบื้องต้นให้ a มีคะแนนสูงมาก ผลักมันเข้า high ผ่าน water-filling
	// หลังจาก truncation: a→50, b,c,d→12 (floor) = 86 < 100
	// remainder loop เห็นว่า a ≥ ceilInt (50) แล้ว → skip ต่อเนื่องจนกว่า
	// no-progress guard ตรวจจับและใช้ fallback เพื่อแจกเศษให้ frozen
	//
	// ทำการจัดสรรซ้ำหลาย ๆ ครั้ง (alpha=0.25) เพื่อให้ a ลู่เข้าเพดาน
	current := map[string]int{"a": 40, "b": 20, "c": 20, "d": 20}
	combined := []Combined{
		{Value: "a", ScoreFinal: 0.99, N: 30}, // a ชนะขาด
		{Value: "b", ScoreFinal: 0.01, N: 30},
		{Value: "c", ScoreFinal: 0.01, N: 30},
		{Value: "d", ScoreFinal: 0.01, N: 30},
	}
	w := current
	// วิ่ง 10 รอบให้ a ลู่เข้าเพดาน 50
	for i := 0; i < 10; i++ {
		w = TuneWeights(w, combined, 8, 0.25)
	}
	// หลังจาก 10 รอบ a ควรอยู่ใกล้เพดาน แล้ว remainder loop ต้องจัดการ
	// ต้องแน่ใจว่า function terminate (ไม่ hang) และ sum=100, bounds ถูก
	if sum(w) != 100 {
		t.Errorf("ผลรวม = %d, want 100 หลังลู่เข้าเพดาน", sum(w))
	}
	// เพดาน = 2 * 0.25 = 0.5 = 50
	if w["a"] > 50 {
		t.Errorf("a = %d เกินเพดาน 50", w["a"])
	}
	// พื้น = 0.5 * 0.25 = 0.125 = 12.5% ≈ 12 หลังปัดเศษ
	if w["b"] < 12 || w["c"] < 12 || w["d"] < 12 {
		t.Errorf("b,c,d ต้องอย่างน้อย 12 (floor 12.5%%), ได้ b=%d c=%d d=%d",
			w["b"], w["c"], w["d"])
	}
}
