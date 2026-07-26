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

// TestScenesNeedingConfirm ครอบ predicate ที่ orchestrator ใช้ตัดสินว่าจะยิงรอบยืนยัน
// ซีนไหน — ต้องไม่รวมซีนที่ตกด้วย sticky code เพราะรอบยืนยันตัดสินซ้ำไม่ได้อยู่แล้ว
func TestScenesNeedingConfirm(t *testing.T) {
	cases := []struct {
		name     string
		verdicts []SceneVerdict
		want     map[int]bool
	}{
		{
			"verdict ผ่านหมด",
			[]SceneVerdict{{SceneNumber: 1, OK: true}, {SceneNumber: 2, OK: true}},
			map[int]bool{},
		},
		{
			"ตกแบบ non-sticky ต้องอยู่ในชุด",
			[]SceneVerdict{{SceneNumber: 1, OK: false, Codes: []string{"overflow"}}},
			map[int]bool{1: true},
		},
		{
			"ตกแบบ sticky ต้องไม่อยู่ในชุด",
			[]SceneVerdict{{SceneNumber: 1, OK: false, Codes: []string{"wordbreak"}}},
			map[int]bool{},
		},
		{
			"ปนกัน — มีเฉพาะ non-sticky",
			[]SceneVerdict{
				{SceneNumber: 1, OK: true},
				{SceneNumber: 2, OK: false, Codes: []string{"overflow"}},
				{SceneNumber: 3, OK: false, Codes: []string{"wordbreak"}},
			},
			map[int]bool{2: true},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ScenesNeedingConfirm(c.verdicts)
			if len(got) != len(c.want) {
				t.Fatalf("ScenesNeedingConfirm(%+v) = %v, want %v", c.verdicts, got, c.want)
			}
			for scene := range c.want {
				if !got[scene] {
					t.Errorf("ScenesNeedingConfirm(%+v) = %v, want scene %d included", c.verdicts, got, scene)
				}
			}
		})
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
