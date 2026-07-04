package orchestrator

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func TestPickCategoryWeighted(t *testing.T) {
	categories := []string{"account", "payment", "campaign"}
	scores := []models.CategoryScore{
		{Category: "payment", AvgPercentile: 0.8, N: 4},
		{Category: "account", AvgPercentile: 0.3, N: 5},
		{Category: "retired-category", AvgPercentile: 0.99, N: 9}, // not configured → ignored
	}

	exploit := func(int) int { return 0 }  // rng(100)=0 < 50 → exploit
	explore := func(int) int { return 99 } // rng(100)=99 ≥ 50 → round-robin

	if got := PickCategoryWeighted(categories, scores, 7, exploit); got != "payment" {
		t.Errorf("exploit pick = %q, want payment (best configured category)", got)
	}
	if got := PickCategoryWeighted(categories, scores, 7, explore); got != categories[7%3] {
		t.Errorf("explore pick = %q, want round-robin %q", got, categories[7%3])
	}
	if got := PickCategoryWeighted(categories, nil, 7, exploit); got != categories[7%3] {
		t.Errorf("no-scores pick = %q, want round-robin fallback", got)
	}
}

func TestFormatTopicStats(t *testing.T) {
	if got := FormatTopicStats(nil); got != "" {
		t.Errorf("empty scores should render empty string, got %q", got)
	}
	out := FormatTopicStats([]models.CategoryScore{
		{Category: "payment", AvgPercentile: 0.8, AvgViews: 95, N: 4},
	})
	for _, want := range []string{"payment", "95", "80", "4", "ครึ่งหนึ่ง"} {
		if !strings.Contains(out, want) {
			t.Errorf("stats missing %q:\n%s", want, out)
		}
	}
}
