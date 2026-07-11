package orchestrator

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)

// PickCategoryWeighted picks the production category. Half the time (rng(100) < 50)
// it exploits the best-performing configured category; otherwise — and whenever no
// scores are available — it keeps the legacy week-based round-robin so topic
// coverage stays diverse. Scores for categories no longer configured are ignored.
func PickCategoryWeighted(categories []string, scores []models.CategoryScore, weekNum int, rng func(int) int) string {
	fallback := categories[weekNum%len(categories)]
	if len(scores) == 0 {
		return fallback
	}
	configured := make(map[string]bool, len(categories))
	for _, c := range categories {
		configured[c] = true
	}
	best := ""
	bestPct := -1.0
	for _, s := range scores {
		if configured[s.Category] && s.AvgPercentile > bestPct {
			best, bestPct = s.Category, s.AvgPercentile
		}
	}
	if best == "" || rng(100) >= 50 {
		return fallback
	}
	return best
}

// FormatTopicStats renders category performance as a Thai prompt block for the
// question agent, or "" when there is nothing to show.
func FormatTopicStats(scores []models.CategoryScore) string {
	if len(scores) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## ผลงานหัวข้อ 30 วันล่าสุด (ยอดจริงจาก YouTube Shorts + TikTok)\n")
	for _, s := range scores {
		b.WriteString(fmt.Sprintf("- หมวด %s: ยอดวิวเฉลี่ย %.0f ต่อคลิป (percentile เฉลี่ย %.0f จาก 100, วัดจาก %d คลิป)\n",
			s.Category, s.AvgViews, s.AvgPercentile*100, s.N))
	}
	b.WriteString("\nใช้ข้อมูลนี้เป็นบริบท: เลือกประเด็น/มุมที่ใกล้เคียงหมวดผลงานดีราวครึ่งหนึ่ง ที่เหลือกระจายมุมใหม่เพื่อความหลากหลาย และห้ามซ้ำกับหัวข้อเดิมตามรายการห้ามซ้ำ")
	return b.String()
}

// PickClipRole — "reach" (prob 1-convertRatio) / "convert" (prob convertRatio).
// Pure: caller ส่ง *rand.Rand เพื่อให้ทดสอบได้.
func PickClipRole(convertRatio float64, rng *rand.Rand) string {
	if convertRatio <= 0 {
		return "reach"
	}
	if convertRatio >= 1 {
		return "convert"
	}
	if rng.Float64() < convertRatio {
		return "convert"
	}
	return "reach"
}

// PickPersona — สุ่ม 1 จาก personas (empty list → "")
func PickPersona(personas []string, rng *rand.Rand) string {
	if len(personas) == 0 {
		return ""
	}
	return personas[rng.Intn(len(personas))]
}
