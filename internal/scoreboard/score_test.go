package scoreboard

import (
	"math"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestMinMaxNorm(t *testing.T) {
	got := MinMaxNorm([]float64{0.2, 0.4, 0.6})
	want := []float64{0, 0.5, 1}
	for i := range want {
		if !almost(got[i], want[i]) {
			t.Errorf("MinMaxNorm[%d] = %v, want %v", i, got[i], want[i])
		}
	}
	// ทุกค่าเท่ากัน → 0.5 ทั้งหมด (ห้ามหาร 0)
	flat := MinMaxNorm([]float64{0.3, 0.3})
	for i, v := range flat {
		if !almost(v, 0.5) {
			t.Errorf("flat[%d] = %v, want 0.5", i, v)
		}
	}
	if len(MinMaxNorm(nil)) != 0 {
		t.Error("MinMaxNorm(nil) ต้องคืน slice ว่าง")
	}
}

func TestScoreRawWithRetention(t *testing.T) {
	// 0.5*0.8 + 0.3*1.0 + 0.2*(1-0.25) = 0.4 + 0.3 + 0.15 = 0.85
	got := ScoreRaw(0.8, 1.0, 0.25, true)
	if !almost(got, 0.85) {
		t.Errorf("ScoreRaw = %v, want 0.85", got)
	}
}

func TestScoreRawWithoutRetentionRenormalizes(t *testing.T) {
	// ไม่มี retention → น้ำหนักเหลือ 0.5/0.7 และ 0.2/0.7
	// (0.5*0.8 + 0.2*(1-0.25)) / 0.7 = (0.4 + 0.15)/0.7 = 0.7857142857...
	got := ScoreRaw(0.8, 0, 0.25, false)
	want := 0.55 / 0.7
	if !almost(got, want) {
		t.Errorf("ScoreRaw without retention = %v, want %v", got, want)
	}
	// ห้ามเท่ากับกรณีใส่ retention = 0 (นั่นคือบั๊กที่ทำให้ case-file ตกรอบ)
	if almost(got, ScoreRaw(0.8, 0, 0.25, true)) {
		t.Error("ไม่มี retention ต้องไม่เท่ากับ retention = 0")
	}
}

func TestShrink(t *testing.T) {
	// n=1: (1*1.0 + 5*0.5)/(1+5) = 3.5/6 = 0.58333...
	if got := Shrink(1.0, 1); !almost(got, 3.5/6) {
		t.Errorf("Shrink(1.0, 1) = %v, want %v", got, 3.5/6)
	}
	// n=75: เกือบเท่าเดิม
	if got := Shrink(1.0, 75); math.Abs(got-1.0) > 0.07 {
		t.Errorf("Shrink(1.0, 75) = %v, ควรใกล้ 1.0", got)
	}
	// n=0 ต้องได้ 0.5 พอดี ไม่ใช่ NaN
	if got := Shrink(0.9, 0); !almost(got, 0.5) {
		t.Errorf("Shrink(_, 0) = %v, want 0.5", got)
	}
}

func TestScoreAllNormalizesWithinDimensionAndPlatform(t *testing.T) {
	stats := []Stat{
		{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30, MedianPct: 0.53, MedianRetention: 0.71, FlopRate: 0.31},
		{Dimension: "content_format", Value: "news", Platform: "youtube", N: 25, MedianPct: 0.53, MedianRetention: 0.75, FlopRate: 0.16},
		// คนละแพลตฟอร์ม: ต้อง normalize แยกกลุ่ม ไม่ปนกับ youtube
		{Dimension: "content_format", Value: "tips", Platform: "tiktok", N: 17, MedianPct: 0.58, MedianRetention: 0, FlopRate: 0.18},
	}
	got := ScoreAll(stats)
	if len(got) != 3 {
		t.Fatalf("ScoreAll คืน %d แถว, want 3", len(got))
	}
	byKey := map[string]Score{}
	for _, s := range got {
		byKey[s.Value+"|"+s.Platform] = s
	}
	// news flop ต่ำกว่าและ retention สูงกว่า qa ที่ median_pct เท่ากัน → ต้องชนะ
	if byKey["news|youtube"].ScoreFinal <= byKey["qa|youtube"].ScoreFinal {
		t.Errorf("news (%v) ต้องมากกว่า qa (%v)", byKey["news|youtube"].ScoreFinal, byKey["qa|youtube"].ScoreFinal)
	}
	// tiktok อยู่กลุ่มเดียวโดดๆ และไม่มี retention → ต้องไม่ NaN และอยู่ในช่วง 0..1
	tips := byKey["tips|tiktok"].ScoreFinal
	if math.IsNaN(tips) || tips < 0 || tips > 1 {
		t.Errorf("tips score = %v ต้องอยู่ใน 0..1 และไม่ NaN", tips)
	}
}
