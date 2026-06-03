package agent

import (
	"strings"
	"testing"
)

func TestIsSubstantialResearch(t *testing.T) {
	realNews := strings.Repeat("Meta บังคับยืนยันตัวตนผู้ลงโฆษณาในไทย มีผลตั้งแต่เดือนกันยายน ", 10)

	tests := []struct {
		name string
		text string
		want bool
	}{
		{"real news summary", realNews, true},
		{"empty response", "", false},
		{"whitespace only", "   \n  ", false},
		{"short refusal", "ไม่พบข่าวที่เกี่ยวข้องในขณะนี้", false},
		{"sentinel marker", "NO_NEWS_FOUND", false},
		{"sentinel inside long text", realNews + " NO_NEWS_FOUND", false},
		{"short but real-looking", "Meta ออกกฎใหม่", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSubstantialResearch(tt.text); got != tt.want {
				t.Errorf("isSubstantialResearch(%q) = %v, want %v", tt.text[:min(len(tt.text), 50)], got, tt.want)
			}
		})
	}
}
