package agent

import (
	"strings"
	"testing"
)

func TestBuildResearchContext(t *testing.T) {
	tests := []struct {
		name     string
		result   researchResult
		wantEmpty bool
		contains []string
	}{
		{
			name: "full result",
			result: researchResult{
				Summary:  "Meta ประกาศบังคับ verify ตัวตน มีผล 1 พ.ย. 2026",
				KeyFacts: []string{"deadline 7 วัน", "กระทบโฆษณาที่ยิงถึงคนไทย"},
				Sources:  []string{"https://ppc.land/example"},
			},
			contains: []string{"Meta ประกาศ", "ข้อเท็จจริงสำคัญ", "deadline 7 วัน", "แหล่งอ้างอิง", "ppc.land"},
		},
		{
			name:      "empty summary means no reliable info",
			result:    researchResult{Summary: "", KeyFacts: []string{"fact"}},
			wantEmpty: true,
		},
		{
			name:      "whitespace summary treated as empty",
			result:    researchResult{Summary: "   "},
			wantEmpty: true,
		},
		{
			name:     "summary only",
			result:   researchResult{Summary: "ข่าวสั้น"},
			contains: []string{"ข่าวสั้น"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildResearchContext(tt.result)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("expected empty context, got %q", got)
				}
				return
			}
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("context missing %q\ngot: %s", want, got)
				}
			}
		})
	}
}
