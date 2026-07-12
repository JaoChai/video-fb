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

// RoleInstruction แปล role ("reach"/"convert") ที่ได้จาก PickClipRole ให้เป็นประโยค
// คำสั่งภาษาไทยสำหรับ prompt ของ agent (question/script) — ชี้แนวทางเลือกหัวข้อ+CTA
// ให้ชัดกว่าการส่ง token เปล่าๆ ค่าอื่นนอกจาก "reach"/"convert" (รวมทั้ง "") → "" เพื่อรักษา
// พฤติกรรมเดิมตอน flag ปิด/role ว่าง (ไม่มีบรรทัด role ใน prompt เลย).
func RoleInstruction(role string) string {
	switch role {
	case "reach":
		return "บทบาทคลิปนี้คือ reach: เลือกหัวข้อที่กว้าง เข้าถึงคนยิงแอดทุกระดับ เน้นข่าวแรง ตัวเลขช็อก เตือนภัย หรือแฉความเชื่อผิดที่ทำให้คนหยุดดูทันที และปิดท้ายด้วย CTA ชวนติดตาม/ดูคลิปต่อไป ไม่ต้องขายของ"
	case "convert":
		return "บทบาทคลิปนี้คือ convert: เจาะลึก pain ของคนที่ถือหลายบัญชีโฆษณาหรือกำลังจะโดนแบน พูดตรงจุดที่เจ็บจริง และปิดท้ายด้วย CTA แบบ soft sell ชวนทักแชทหรือดูช่องทางใต้คลิปเรื่องบัญชีโฆษณา ห้ามขายบัญชีทั้งคลิป"
	default:
		return ""
	}
}

// PickPersona — สุ่ม 1 จาก personas (empty list → "")
func PickPersona(personas []string, rng *rand.Rand) string {
	if len(personas) == 0 {
		return ""
	}
	return personas[rng.Intn(len(personas))]
}
