package analyzer

import (
	"strings"
	"testing"
)

func TestTrendLabel(t *testing.T) {
	cases := []struct {
		name  string
		views []int
		want  string
	}{
		{"too few points", []int{10, 20}, "unknown"},
		{"three points always looked rising — now unknown", []int{100, 200, 201}, "unknown"},
		{"no growth", []int{100, 100, 100, 100}, "steady"},
		{"rising: most growth is recent", []int{100, 110, 150, 220}, "rising"},
		{"peaked: growth stopped", []int{10, 80, 100, 102}, "peaked"},
		{"steady climb", []int{10, 40, 70, 100}, "rising"},
		{"noisy last snapshot on steady riser", []int{0, 1000, 2000, 3000, 4000, 5000, 6000, 6010}, "steady"},
	}
	for _, c := range cases {
		if got := TrendLabel(c.views); got != c.want {
			t.Errorf("%s: TrendLabel(%v) = %q, want %q", c.name, c.views, got, c.want)
		}
	}
}

func TestFillPercentiles(t *testing.T) {
	stats := []ClipStat{
		{ID: "a", Platform: "youtube", Views: 10},
		{ID: "b", Platform: "youtube", Views: 100},
		{ID: "c", Platform: "youtube", Views: 50},
		{ID: "d", Platform: "tiktok", Views: 5}, // alone on its platform → percentile 1.0
	}
	FillPercentiles(stats)
	if stats[1].Percentile != 1.0 {
		t.Errorf("top youtube percentile = %v, want 1.0", stats[1].Percentile)
	}
	if stats[0].Percentile != 0.0 {
		t.Errorf("bottom youtube percentile = %v, want 0.0", stats[0].Percentile)
	}
	if stats[2].Percentile != 0.5 {
		t.Errorf("mid youtube percentile = %v, want 0.5", stats[2].Percentile)
	}
	if stats[3].Percentile != 1.0 {
		t.Errorf("solo tiktok percentile = %v, want 1.0", stats[3].Percentile)
	}
}

func TestBuildAnalysisData(t *testing.T) {
	stats := []ClipStat{
		{ID: "aaaaaaaa-1111", Title: "คลิปหนึ่ง", Category: "payment", Hook: "เคยไหมโดนแบน?",
			Platform: "tiktok", Views: 120, Likes: 1, EngagementRate: 0.83,
			Percentile: 0.9, Trend: "rising"},
	}
	data, n := BuildAnalysisData(stats)
	if n != 1 {
		t.Errorf("clip count = %d, want 1", n)
	}
	for _, want := range []string{"payment", "เคยไหมโดนแบน?", "tiktok", "P90", "rising"} {
		if !strings.Contains(data, want) {
			t.Errorf("data missing %q:\n%s", want, data)
		}
	}
}
