package analyzer

import (
	"fmt"
	"sort"
	"strings"
)

// ClipStat is one clip's latest performance on one platform, assembled for the
// weekly LLM analysis.
type ClipStat struct {
	ID, Title, Category, Hook      string
	Platform                       string
	Views, Likes, Comments, Shares int
	EngagementRate                 float64
	AvgViewPct                     float64
	SubsGained                     int
	Percentile                     float64 // 0..1 within platform, filled by FillPercentiles
	Trend                          string  // rising | peaked | steady | unknown
}

// TrendLabel classifies a clip's daily cumulative view counts (oldest→newest)
// by comparing the most recent day's growth to the average daily growth over
// the whole window. "rising" = the last day grew at or above the average
// pace; "peaked" = the window grew overall but the last day's growth fell to
// under 20% of the average pace (i.e. it flattened out); otherwise "steady".
func TrendLabel(dailyViews []int) string {
	n := len(dailyViews)
	if n < 3 {
		return "unknown"
	}
	total := dailyViews[n-1] - dailyViews[0]
	if total <= 0 {
		return "steady"
	}
	avgDelta := float64(total) / float64(n-1)
	recent := float64(dailyViews[n-1] - dailyViews[n-2])
	switch {
	case recent >= avgDelta:
		return "rising"
	case recent <= avgDelta/5:
		return "peaked"
	default:
		return "steady"
	}
}

// FillPercentiles sets each stat's within-platform views percentile (0 = worst,
// 1 = best). A platform with a single clip gets 1.0.
func FillPercentiles(stats []ClipStat) {
	byPlatform := map[string][]int{}
	for i := range stats {
		byPlatform[stats[i].Platform] = append(byPlatform[stats[i].Platform], i)
	}
	for _, idxs := range byPlatform {
		sort.Slice(idxs, func(a, b int) bool { return stats[idxs[a]].Views < stats[idxs[b]].Views })
		n := len(idxs)
		for rank, i := range idxs {
			if n == 1 {
				stats[i].Percentile = 1.0
			} else {
				stats[i].Percentile = float64(rank) / float64(n-1)
			}
		}
	}
}

// BuildAnalysisData renders stats as one line per clip-platform for the LLM
// and returns the number of distinct clips.
func BuildAnalysisData(stats []ClipStat) (string, int) {
	seen := map[string]bool{}
	var lines []string
	for _, s := range stats {
		seen[s.ID] = true
		id := s.ID
		if len(id) > 8 {
			id = id[:8]
		}
		line := fmt.Sprintf(
			"- Clip %s | Platform: %s | Category: %s | Title: %s | Hook: %s | Views: %d (P%.0f within platform) | Likes: %d | Comments: %d | Shares: %d | Engagement: %.2f%% | Trend: %s",
			id, s.Platform, s.Category, s.Title, s.Hook,
			s.Views, s.Percentile*100, s.Likes, s.Comments, s.Shares, s.EngagementRate, s.Trend)
		if s.Platform == "youtube" {
			line += fmt.Sprintf(" | AvgViewPct: %.0f%% | SubsGained: %d", s.AvgViewPct*100, s.SubsGained)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), len(seen)
}
