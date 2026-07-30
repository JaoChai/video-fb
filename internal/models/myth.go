package models

// ค่า verdict ของแถวคลัง — ต้องตรงกับ CHECK constraint ของ myth_beliefs.verdict
// (migration 075) เป๊ะทุกตัวอักษร เพราะ Go เอาค่านี้ไปเป็น SceneContent.Meter ตรงๆ
const (
	MythVerdictFalse    = "false"     // ไม่จริง
	MythVerdictHalfTrue = "half_true" // จริงครึ่งเดียว
	MythVerdictOutdated = "outdated"  // เคยจริง วันนี้ไม่จริงแล้ว
)

// ValidMythVerdict บอกว่าค่านี้เป็น verdict ที่ระบบรู้จักไหม ใช้ตรวจแถวคลังที่
// อ่านมาจาก DB ก่อนส่งต่อให้ template — แถวที่ค่าเพี้ยนต้องหยุดที่นี่ ไม่ใช่ไป
// โผล่เป็นมิเตอร์เปล่าในคลิปที่เผยแพร่แล้ว
func ValidMythVerdict(v string) bool {
	switch v {
	case MythVerdictFalse, MythVerdictHalfTrue, MythVerdictOutdated:
		return true
	}
	return false
}

// MythBelief คือหนึ่งแถวของคลังความเชื่อผิด = หัวข้อของคลิปหนึ่งตัว
// ทุกข้อเท็จจริงที่คลิปพูดได้ต้องมาจากฟิลด์ในนี้เท่านั้น (ดูตะแกรงใน internal/agent/myth.go)
type MythBelief struct {
	ID            string `json:"id"`
	BeliefKey     string `json:"belief_key"`
	BeliefTH      string `json:"belief_th"`
	WhyBelievedTH string `json:"why_believed_th"`
	Verdict       string `json:"verdict"`
	FactTH        string `json:"fact_th"`
	SourceLabel   string `json:"source_label"`
	SourceURL     string `json:"source_url"`
	NuanceTH      string `json:"nuance_th"`
	CostTH        string `json:"cost_th"`
	Audience      string `json:"audience"`
	Weight        int    `json:"weight"`
	Enabled       bool   `json:"enabled"`
}
