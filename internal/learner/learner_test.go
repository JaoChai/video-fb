package learner

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/repository"
)

func TestStrongSignal_FiresOnLowDimWithEnoughData(t *testing.T) {
	p := repository.ScorePatterns{N: minCritiques, AvgHook: 4.5, AvgClarity: 7, AvgBrandFit: 8, AvgOverall: 6.5}
	ok, dim, val := strongSignal(p)
	if !ok {
		t.Fatalf("strongSignal = false, want true (n=%d, hook=4.5 < %.1f)", p.N, lowScoreThreshold)
	}
	if dim != "hook" || val != 4.5 {
		t.Errorf("weakest = (%q, %v), want (hook, 4.5)", dim, val)
	}
}

func TestStrongSignal_SkipsTooFewCritiques(t *testing.T) {
	p := repository.ScorePatterns{N: minCritiques - 1, AvgHook: 2.0, AvgClarity: 2, AvgBrandFit: 2, AvgOverall: 2}
	if ok, _, _ := strongSignal(p); ok {
		t.Fatal("strongSignal = true, want false when n < minCritiques")
	}
}

func TestStrongSignal_SkipsAllDimsHealthy(t *testing.T) {
	p := repository.ScorePatterns{N: 50, AvgHook: lowScoreThreshold, AvgClarity: 7, AvgBrandFit: 8, AvgOverall: 9}
	if ok, _, _ := strongSignal(p); ok {
		t.Fatal("strongSignal = true, want false when weakest >= threshold (boundary is exclusive)")
	}
}

func TestStrongSignal_EmptyWindow(t *testing.T) {
	if ok, _, _ := strongSignal(repository.ScorePatterns{N: 0}); ok {
		t.Fatal("strongSignal = true, want false on empty window")
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
