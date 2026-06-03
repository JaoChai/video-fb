package analyzer

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxInsightsLength = 1000

// forbiddenPatterns are directives that narrow topic/category diversity.
// The analyzer may only suggest STYLE improvements (hooks, pacing, openings),
// never WHICH topics or categories to focus on or avoid.
var forbiddenPatterns = []string{
	// Thai directives
	"เน้นหมวด", "เน้นเฉพาะ", "เฉพาะหมวด",
	"หลีกเลี่ยงหมวด", "เลี่ยงหมวด",
	"ห้ามสร้างคำถาม", "ห้ามทำหัวข้อ", "ห้ามหัวข้อ",
	"งดหมวด", "ลดหมวด", "เพิ่มหมวด",
	"ควรเน้น", "ข้ามหัวข้อ", "ข้ามหมวด", "ไม่ต้องทำหมวด",
	"หัวข้อที่ควร", "หัวข้อที่ไม่ควร", "หลีกเลี่ยงหัวข้อ",
	// English directives
	"focus on", "focus only", "avoid", "exclude", "prioritize",
	"more questions about", "fewer questions about",
	"stay away", "concentrate on", "don't cover", "never cover",
	"skip topics", "only cover", "don't create questions",
}

// ValidateInsights rejects insights text that tries to steer topic/category
// selection. Returns nil if the text is style-only and within length limits.
func ValidateInsights(text string) error {
	if utf8.RuneCountInString(text) > maxInsightsLength {
		return fmt.Errorf("insights too long: %d chars (max %d)", utf8.RuneCountInString(text), maxInsightsLength)
	}
	lower := strings.ToLower(text)
	for _, p := range forbiddenPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return fmt.Errorf("insights contain forbidden topic-steering directive: %q", p)
		}
	}
	return nil
}
