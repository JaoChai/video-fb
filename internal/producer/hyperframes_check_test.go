package producer

import (
	"errors"
	"os/exec"
	"slices"
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

// ด่านผ่าน = CLI จบดี และไม่มีสัญญาณพังในหน้าเพจ. render ที่ค้างจะ exit 0
// พร้อม [Browser:PAGEERROR] — ถ้านับว่า passed สถิติจะบอกว่า render ไม่เคยพัง
func TestCheckPassed(t *testing.T) {
	if !checkPassed(nil, nil) {
		t.Error("ไม่มี error ไม่มี issue ต้องผ่าน")
	}
	if checkPassed(nil, []string{"[Browser:PAGEERROR] boom"}) {
		t.Error("exit 0 แต่มี browser issue = เรนเดอร์ค้าง ต้องไม่ผ่าน")
	}
	if checkPassed(errors.New("boom"), nil) {
		t.Error("มี error ต้องไม่ผ่าน")
	}
}

func TestWithDetail(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		err  error
		want []string
	}{
		{"ไม่มี error ⇒ ไม่แตะ findings", []string{"a"}, nil, []string{"a"}},
		{
			"ต่อข้อความเต็มท้าย finding เดิม",
			[]string{"failed: hyperframes [inspect] exited non-zero"},
			errors.New("canvas_overflow: #cta ล้นกรอบฉาก 3"),
			[]string{"failed: hyperframes [inspect] exited non-zero", "detail: canvas_overflow: #cta ล้นกรอบฉาก 3"},
		},
		{"findings ว่างก็ยังได้ detail", nil, errors.New("boom"), []string{"detail: boom"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := withDetail(CheckResult{Stage: "inspect", Findings: c.in}, c.err).Findings
			if !slices.Equal(got, c.want) {
				t.Errorf("findings = %v, want %v", got, c.want)
			}
		})
	}
}
