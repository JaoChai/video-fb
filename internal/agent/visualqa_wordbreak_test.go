package agent

import (
	"os"
	"strings"
	"testing"
)

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

// TestMigration066DeclaresStickyCodes กันความพังที่เงียบที่สุดของฟีเจอร์นี้: ถ้า prompt
// สะกดรหัสไม่ตรงกับ stickyCodes ใน Go (เช่น "word_break" กับ "wordbreak") ตำหนิจะไหลผ่าน
// รอบยืนยันไปเผยแพร่โดยไม่มีใครรู้ว่าสาเหตุอยู่ที่การสะกด ไม่ใช่ที่โมเดล
func TestMigration066DeclaresStickyCodes(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/066_visual_qa_thai_wordbreak.sql")
	if err != nil {
		t.Fatalf("อ่าน migration 066 ไม่ได้: %v", err)
	}
	sql := string(raw)

	for code := range stickyCodes {
		if !strings.Contains(sql, `"`+code+`"`) {
			t.Errorf("migration 066 ไม่ได้ประกาศรหัส %q ให้โมเดล — ConfirmMerge จะไม่มีวันเห็นมัน", code)
		}
	}
	if !strings.Contains(sql, "BEGIN;") || !strings.Contains(sql, "COMMIT;") {
		t.Error("migration ต้องหุ้ม BEGIN/COMMIT เอง — RunMigrations ไม่หุ้มให้")
	}
	if !strings.Contains(sql, "auto_review") {
		t.Error("migration ต้องแก้ auto_review ด้วย ไม่งั้นมัน approve ทับตำหนิที่ visual_qa จับได้")
	}
}
