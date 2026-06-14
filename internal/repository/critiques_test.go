package repository

import "testing"

func TestLowestDimension_PicksWeakest(t *testing.T) {
	p := ScorePatterns{N: 5, AvgHook: 4.2, AvgClarity: 7.0, AvgBrandFit: 8.1, AvgOverall: 6.5}
	name, val := p.LowestDimension()
	if name != "hook" {
		t.Errorf("name = %q, want hook", name)
	}
	if val != 4.2 {
		t.Errorf("val = %v, want 4.2", val)
	}
}

func TestLowestDimension_EmptyWindow(t *testing.T) {
	name, val := ScorePatterns{N: 0}.LowestDimension()
	if name != "" || val != 0 {
		t.Errorf("empty window: got (%q, %v), want (\"\", 0)", name, val)
	}
}

func TestLowestDimension_TieKeepsFirst(t *testing.T) {
	p := ScorePatterns{N: 3, AvgHook: 5.0, AvgClarity: 5.0, AvgBrandFit: 5.0, AvgOverall: 5.0}
	name, _ := p.LowestDimension()
	if name != "hook" {
		t.Errorf("tie should keep first (hook), got %q", name)
	}
}
