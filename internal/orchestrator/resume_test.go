package orchestrator

import "testing"

func TestResumeAtRender(t *testing.T) {
	cases := map[string]bool{
		"":              false,
		"content_ready": true,
		"rendered":      true,
		"something":     false,
	}
	for stage, want := range cases {
		if got := resumeAtRender(stage); got != want {
			t.Errorf("resumeAtRender(%q) = %v, want %v", stage, got, want)
		}
	}
}
