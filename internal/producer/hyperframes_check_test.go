package producer

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestFindingsJSON_NilBecomesEmptyArray(t *testing.T) {
	got := string(CheckResult{}.FindingsJSON())
	if got != "[]" {
		t.Fatalf("FindingsJSON() = %s, want []", got)
	}
}

func TestFindingsJSON_EncodesLines(t *testing.T) {
	c := CheckResult{Findings: []string{"canvas_overflow: #cta", `has "quotes"`}}
	got := string(c.FindingsJSON())
	if !strings.Contains(got, "canvas_overflow") || !strings.HasPrefix(got, "[") {
		t.Fatalf("FindingsJSON() = %s, want a JSON array containing the finding", got)
	}
}

// exit != 0 แปลว่า CLI รันแล้วและ "ไม่ผ่าน" — เป็น finding จริงของเทมเพลต
func TestClassifyRunError_ExitErrorIsNotRunnerError(t *testing.T) {
	got := classifyRunError(&exec.ExitError{}, []string{"lint"})
	for _, line := range got {
		if strings.HasPrefix(line, "runner_error:") {
			t.Fatalf("exit error must not be classified as runner_error, got %v", got)
		}
	}
	if len(got) == 0 {
		t.Fatal("exit error must still produce a finding line")
	}
}

// รัน CLI ไม่ได้เลย (ไม่มี binary / timeout) เป็นคนละเรื่องกับเทมเพลตพัง
// เฟส 2 จะเปิด gate จาก finding จริงเท่านั้น จึงต้องแยกให้ออกตั้งแต่ตอนนี้
func TestClassifyRunError_RunnerErrorIsPrefixed(t *testing.T) {
	got := classifyRunError(errors.New("exec: \"hyperframes\": executable file not found"), []string{"lint"})
	if len(got) != 1 || !strings.HasPrefix(got[0], "runner_error:") {
		t.Fatalf("classifyRunError() = %v, want one runner_error: line", got)
	}
}

func TestClassifyRunError_NilIsEmpty(t *testing.T) {
	if got := classifyRunError(nil, []string{"lint"}); len(got) != 0 {
		t.Fatalf("classifyRunError(nil) = %v, want empty", got)
	}
}
