package config

import (
	"os"
	"testing"
)

// TestEnvBool verifies the SCHEDULER_ENABLED kill switch helper. An
// unparseable or missing value MUST fall back to the default — never silently
// become false, since false means the production/publish/retry cron never
// fires from this instance.
func TestEnvBool(t *testing.T) {
	val := func(s string) *string { return &s }

	tests := []struct {
		name string
		set  *string // nil = ไม่ตั้ง env เลย
		want bool
	}{
		{"unset env returns default true", nil, true},
		{"false string returns false", val("false"), false},
		{"zero returns false", val("0"), false},
		{"true string returns true", val("true"), true},
		// พิมพ์ผิดหรือว่าง ต้องคืนดีฟอลต์ ไม่ใช่ false — env ที่พิมพ์ผิด
		// ห้ามกลายเป็นการปิดสายการผลิตเงียบๆ
		{"unparseable value returns default not false", val("maybe"), true},
		{"empty string returns default not false", val(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ตั้งค่าอะไรก็ได้ก่อนเสมอ เพื่อให้ t.Setenv จดค่าเดิมไว้คืนหลังจบเทสต์
			// แล้วค่อยลบทิ้งในเคส unset — ไม่งั้นเคสนั้นจะลบ env ของ process ถาวร
			t.Setenv("X", "placeholder")
			if tt.set == nil {
				os.Unsetenv("X")
			} else {
				t.Setenv("X", *tt.set)
			}
			if got := envBool("X", true); got != tt.want {
				t.Errorf("envBool(\"X\", true) = %v, want %v", got, tt.want)
			}
		})
	}
}
