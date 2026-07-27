package scoreboard

import "testing"

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
