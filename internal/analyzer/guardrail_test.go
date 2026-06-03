package analyzer

import (
	"strings"
	"testing"
)

func TestValidateInsights(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{"valid style insight", "คลิปที่เปิดด้วยตัวเลขมี retention ดีกว่า ควรเปิดเรื่องด้วยตัวเลขหรือผลลัพธ์", false},
		{"valid hook insight", "Hook แบบคำถามกระแทกใจทำให้คนดูจนจบมากขึ้น", false},
		{"empty is valid", "", false},
		{"forbids category focus", "เน้นหมวด Account และ Payment เพราะยอดวิวสูง", true},
		{"forbids category avoidance", "หลีกเลี่ยงหมวด Campaign ที่มียอดวิวต่ำ", true},
		{"forbids topic ban", "ห้ามสร้างคำถามเชิงเทคนิค ABO/CBO", true},
		{"forbids exclusive category", "ทำเฉพาะหมวดที่คนดูเยอะ", true},
		{"forbids english focus directive", "Focus only on account topics", true},
		{"forbids avoid directive", "Avoid campaign questions", true},
		{"too long", strings.Repeat("ก", 1001), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInsights(tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInsights(%q) error = %v, wantErr %v", tt.text, err, tt.wantErr)
			}
		})
	}
}
