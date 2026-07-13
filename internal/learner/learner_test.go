package learner

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/repository"
)

func TestStrongSignal_SkipsTooFewCritiques(t *testing.T) {
	p := repository.ScorePatterns{N: minCritiques - 1, AvgHook: 2.0, AvgClarity: 2, AvgBrandFit: 2, AvgOverall: 2}
	if ok, _, _, gate := strongSignal(p, repository.ScorePatterns{}); ok {
		t.Fatalf("strongSignal = true, want false when n < minCritiques (gate=%s)", gate)
	}
}

func TestStrongSignal_EmptyWindow(t *testing.T) {
	if ok, _, _, gate := strongSignal(repository.ScorePatterns{N: 0}, repository.ScorePatterns{}); ok {
		t.Fatalf("strongSignal = true, want false on empty window (gate=%s)", gate)
	}
}

func TestStrongSignalRelativeGates(t *testing.T) {
	base := repository.ScorePatterns{N: 40, AvgHook: 8.0, AvgClarity: 8.2, AvgBrandFit: 8.8, AvgOverall: 8.1}

	cases := []struct {
		name string
		p    repository.ScorePatterns
		base repository.ScorePatterns
		want bool
		gate string
	}{
		{
			name: "too few critiques never fires",
			p:    repository.ScorePatterns{N: 5, AvgHook: 3.0, AvgClarity: 8, AvgBrandFit: 8, AvgOverall: 8},
			base: base, want: false, gate: "insufficient",
		},
		{
			name: "regression: weakest dim 0.6 below its own 90d baseline fires",
			p:    repository.ScorePatterns{N: 12, AvgHook: 7.4, AvgClarity: 8.1, AvgBrandFit: 8.7, AvgOverall: 8.0},
			base: base, want: true, gate: "regression",
		},
		{
			name: "flat scores near baseline do not fire",
			p:    repository.ScorePatterns{N: 12, AvgHook: 7.8, AvgClarity: 8.1, AvgBrandFit: 8.8, AvgOverall: 8.0},
			base: base, want: false, gate: "no_gate",
		},
		{
			name: "frequency: top issue in >=40% of critiques fires even without regression",
			p: repository.ScorePatterns{N: 10, AvgHook: 7.9, AvgClarity: 8.1, AvgBrandFit: 8.8, AvgOverall: 8.0,
				TopIssues: []repository.FieldIssue{{Field: "scene[0].voice_text", Reason: "hook อืด", Count: 5}}},
			base: base, want: true, gate: "frequency",
		},
		{
			name: "insufficient baseline disables regression gate (frequency can still fire)",
			p:    repository.ScorePatterns{N: 12, AvgHook: 6.0, AvgClarity: 8.1, AvgBrandFit: 8.8, AvgOverall: 8.0},
			base: repository.ScorePatterns{N: 3, AvgHook: 8.0}, want: false, gate: "no_gate",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, _, gate := strongSignal(tc.p, tc.base)
			if got != tc.want {
				t.Errorf("fire = %v, want %v (gate=%s)", got, tc.want, gate)
			}
			if tc.want && gate != tc.gate {
				t.Errorf("gate = %q, want %q", gate, tc.gate)
			}
		})
	}
}

func TestFormatPatterns_IncludesScoresAndIssues(t *testing.T) {
	p := repository.ScorePatterns{
		N: 12, AvgHook: 4.5, AvgClarity: 7, AvgBrandFit: 8, AvgOverall: 6,
		TopIssues: []repository.FieldIssue{
			{Field: "scene[0].voice_text", Reason: "hook อ่อน", Count: 5},
		},
	}
	s := formatPatterns(p)
	for _, want := range []string{"12", "4.50", "scene[0].voice_text", "hook อ่อน", "x5"} {
		if !strings.Contains(s, want) {
			t.Errorf("formatPatterns output missing %q\n--- got ---\n%s", want, s)
		}
	}
}

func TestFormatPatterns_NoIssues(t *testing.T) {
	s := formatPatterns(repository.ScorePatterns{N: 9, AvgHook: 5, AvgClarity: 5, AvgBrandFit: 5, AvgOverall: 5})
	if !strings.Contains(s, "ไม่มีรายการ") {
		t.Errorf("expected empty-issues marker, got:\n%s", s)
	}
}

// TestAgentIssueFiltering verifies that only TopIssues attributed to a given
// agent survive after filtering via agentForField. This is the core correctness
// property of FIX 2: a metadata issue must NOT appear in the scene agent's slice.
func TestAgentIssueFiltering(t *testing.T) {
	allIssues := []repository.FieldIssue{
		{Field: "scene[0].voice_text", Reason: "hook อ่อน", Count: 5},
		{Field: "metadata.youtube_title", Reason: "title ซ้ำ", Count: 3},
		{Field: "scene[1].on_screen_text", Reason: "ตัวหนังสือเยอะ", Count: 2},
		{Field: "metadata.description", Reason: "desc สั้น", Count: 1},
	}

	filterFor := func(name string) []repository.FieldIssue {
		var out []repository.FieldIssue
		for _, fi := range allIssues {
			if agentForField(fi.Field) == name {
				out = append(out, fi)
			}
		}
		return out
	}

	sceneIssues := filterFor("scene")
	if len(sceneIssues) != 2 {
		t.Errorf("scene: want 2 issues, got %d: %v", len(sceneIssues), sceneIssues)
	}
	for _, fi := range sceneIssues {
		if agentForField(fi.Field) != "scene" {
			t.Errorf("scene filter leaked non-scene issue: %q", fi.Field)
		}
	}

	scriptIssues := filterFor("script")
	if len(scriptIssues) != 2 {
		t.Errorf("script: want 2 issues, got %d: %v", len(scriptIssues), scriptIssues)
	}
	for _, fi := range scriptIssues {
		if agentForField(fi.Field) != "script" {
			t.Errorf("script filter leaked non-script issue: %q", fi.Field)
		}
	}
}
