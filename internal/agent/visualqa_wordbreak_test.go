package agent

import "testing"

func TestHasStickyCode(t *testing.T) {
	cases := []struct {
		name  string
		codes []string
		want  bool
	}{
		{"wordbreak ตรงตัว", []string{"wordbreak"}, true},
		{"wordbreak ปนกับรหัสอื่น", []string{"overflow", "wordbreak"}, true},
		{"ตัวพิมพ์ใหญ่/ช่องว่างต้องยังจับได้", []string{" WordBreak "}, true},
		{"รหัสที่ไม่ sticky", []string{"overflow", "ai_artifact"}, false},
		{"ไม่มีรหัสเลย", nil, false},
		{"รหัสว่าง", []string{""}, false},
	}
	for _, c := range cases {
		if got := HasStickyCode(c.codes); got != c.want {
			t.Errorf("%s: HasStickyCode(%v) = %v, want %v", c.name, c.codes, got, c.want)
		}
	}
}

// TestStickyCodesNonEmpty กันการลบรหัสทิ้งโดยไม่ตั้งใจ — ถ้า map ว่าง ConfirmMerge
// จะกลับไปเคลียร์ตำหนิ wordbreak ทิ้งเงียบๆ เหมือนก่อนแก้
func TestStickyCodesNonEmpty(t *testing.T) {
	if !stickyCodes["wordbreak"] {
		t.Fatal(`stickyCodes ต้องมี "wordbreak" — ไม่งั้นรอบยืนยันจะเคลียร์ตำหนิคำไทยถูกตัดกลางคำทิ้ง`)
	}
}
