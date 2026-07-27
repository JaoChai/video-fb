package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jaochai/video-fb/internal/scoreboard"
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
// by comparing growth over the last TWO intervals to the average daily growth
// over the whole window (two-day smoothing so one lagging/anomalous snapshot
// can't flip the label). "rising" = the last two days grew at 2x+ the average
// pace; "peaked" = the window grew overall but the last two days' growth fell
// to at or under the average pace (i.e. it flattened out); otherwise "steady".
// At least 4 daily snapshots (3 intervals) are needed to judge a trend — with
// only 3 points the "recent" and "average" windows overlap completely, so any
// growth always looks "rising".
func TrendLabel(dailyViews []int) string {
	if len(dailyViews) < 4 {
		return "unknown"
	}
	n := len(dailyViews)
	total := dailyViews[n-1] - dailyViews[0]
	if total <= 0 {
		return "steady"
	}
	// Compare the last two intervals' growth against the window's average
	// daily pace — two-day smoothing so one lagging snapshot can't flip the label.
	avgDelta := float64(total) / float64(n-1)
	recent := float64(dailyViews[n-1] - dailyViews[n-3])
	switch {
	case recent >= 2*avgDelta:
		return "rising"
	case recent <= avgDelta:
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

// TopBottom คัดเฉพาะคลิปที่ดีที่สุดและแย่ที่สุด k ตัวของแต่ละแพลตฟอร์ม
// ตัวกลางไม่ได้สอนอะไรโมเดล และการส่งคลิปทั้งหมดไปทำให้โมเดลเสียความสนใจ
// กับรายละเอียดแทนที่จะอธิบายว่าอะไรทำให้ตัวบนชนะ
func TopBottom(stats []ClipStat, k int) []ClipStat {
	byPlatform := map[string][]ClipStat{}
	for _, s := range stats {
		byPlatform[s.Platform] = append(byPlatform[s.Platform], s)
	}
	var out []ClipStat
	for _, group := range byPlatform {
		sort.Slice(group, func(a, b int) bool { return group[a].Percentile > group[b].Percentile })
		n := len(group)
		if n <= 2*k {
			out = append(out, group...)
			continue
		}
		out = append(out, group[:k]...)
		out = append(out, group[n-k:]...)
	}
	return out
}

// BuildScoreboardSection เรนเดอร์กระดานคะแนนเป็นข้อความสำหรับใส่ในพรอมป์
// ตัวเลขถูกคำนวณมาแล้ว โมเดลจึงไม่ต้องเดาว่าสูตรไหนชนะ — หน้าที่มันคืออธิบายว่าทำไม
func BuildScoreboardSection(scores []scoreboard.Score) string {
	if len(scores) == 0 {
		return "(ยังไม่มีกระดานคะแนน)"
	}
	byDim := map[string][]scoreboard.Score{}
	var dims []string
	for _, s := range scores {
		if _, seen := byDim[s.Dimension]; !seen {
			dims = append(dims, s.Dimension)
		}
		byDim[s.Dimension] = append(byDim[s.Dimension], s)
	}
	sort.Strings(dims)

	var b strings.Builder
	for _, d := range dims {
		group := byDim[d]
		sort.Slice(group, func(i, j int) bool { return group[i].ScoreFinal > group[j].ScoreFinal })
		fmt.Fprintf(&b, "\n### %s\n", d)
		for _, s := range group {
			fmt.Fprintf(&b, "- %s (%s): คะแนน %.2f | n=%d | median วิว P%.0f | flop %.0f%%",
				s.Value, s.Platform, s.ScoreFinal, s.N, s.MedianPct*100, s.FlopRate*100)
			if s.MedianRetention > 0 {
				fmt.Fprintf(&b, " | retention %.2f", s.MedianRetention)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
