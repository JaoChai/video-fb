package analyzer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/repository"
)

const historyPrefixInsights = "[insights] "

type Analyzer struct {
	pool       *pgxpool.Pool
	llm        *agent.KieLLMClient
	agentsRepo *repository.AgentsRepo
}

func New(pool *pgxpool.Pool, llm *agent.KieLLMClient, agentsRepo *repository.AgentsRepo) *Analyzer {
	return &Analyzer{pool: pool, llm: llm, agentsRepo: agentsRepo}
}

type improvementResult struct {
	Agents []agentImprovement `json:"agents"`
}

type agentImprovement struct {
	AgentName   string `json:"agent_name"`
	NewInsights string `json:"new_insights"`
	Reason      string `json:"reason"`
}

func (a *Analyzer) AnalyzeAndImprove(ctx context.Context) error {
	stats, err := a.gatherData(ctx)
	if err != nil {
		return fmt.Errorf("gather analytics data: %w", err)
	}

	data, clipCount := BuildAnalysisData(stats)
	// Small-sample gate: below 8 measurable clips the signal is noise.
	if clipCount < 8 {
		log.Printf("Analyzer: only %d measurable clips in window (need 8), skipping", clipCount)
		return nil
	}

	analyticsAgent, err := a.agentsRepo.GetByName(ctx, "analytics")
	if err != nil {
		return fmt.Errorf("get analytics agent config: %w", err)
	}

	userPrompt := fmt.Sprintf(`Here is the performance data from our YouTube Shorts + TikTok posts for the last 14 days (n=%d clips — a small sample; calibrate your confidence accordingly):

%s

Notes on the data:
- "P<n> within platform" is the views percentile compared to other clips on the SAME platform (P90 = top 10%%).
- "Trend: rising" means most view growth happened in the last 2 days (likely entering the recommendation feed); "peaked" means growth stopped; "steady" means growth continues at an ordinary pace; "unknown" means too little data.
- TikTok has no watch-time/retention data — judge TikTok clips by views percentile, shares, engagement, and trend.

Current agent configurations:
%s

Analyze BOTH of these dimensions:
1. STORYTELLING STYLE — openings, hooks (the "Hook" field is the clip's real first line), pacing, tone, length. Which styles earn high view percentiles and "rising" trends?
2. TOPICS — which categories and question angles earn high views/shares on each platform?

Requirements for your insights:
- Preserve content diversity: recommend leaning into winning topics for roughly HALF of future clips, never exclusively. Say this explicitly in the question agent's insights.
- Ground every recommendation in the data (cite the pattern: views percentile, shares, or trend).
- Each insight must be under 1000 characters, written in Thai.

Return JSON only:
{
  "agents": [
    {"agent_name": "question", "new_insights": "...", "reason": "..."},
    {"agent_name": "script", "new_insights": "...", "reason": "..."},
    {"agent_name": "image", "new_insights": "...", "reason": "..."}
  ]
}`, clipCount, data, a.currentPrompts(ctx))

	var result improvementResult
	err = a.llm.GenerateJSON(ctx, analyticsAgent.Model, analyticsAgent.BuildSystemPrompt(), userPrompt, analyticsAgent.Temperature, &result)
	if err != nil {
		return fmt.Errorf("LLM analysis: %w", err)
	}

	if len(result.Agents) == 0 {
		log.Println("Analyzer: LLM returned no improvements")
		return nil
	}

	agents, err := a.agentsRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list agents: %w", err)
	}
	agentMap := make(map[string]string, len(agents))
	for _, ag := range agents {
		agentMap[ag.AgentName] = ag.Insights
	}

	for _, imp := range result.Agents {
		if imp.AgentName == "analytics" || imp.NewInsights == "" {
			continue
		}

		oldInsights, exists := agentMap[imp.AgentName]
		if !exists {
			log.Printf("Analyzer: skip unknown agent %s", imp.AgentName)
			continue
		}

		if err := ValidateInsights(imp.NewInsights); err != nil {
			log.Printf("Analyzer: REJECTED insights for %s: %v", imp.AgentName, err)
			continue
		}

		if err := a.agentsRepo.SaveInsightsWithHistory(ctx, imp.AgentName, oldInsights, imp.NewInsights, historyPrefixInsights+imp.Reason); err != nil {
			log.Printf("Analyzer: failed to save insights for %s: %v", imp.AgentName, err)
			continue
		}

		log.Printf("Analyzer: updated %s insights — reason: %s", imp.AgentName, imp.Reason)
	}

	return nil
}

func (a *Analyzer) gatherData(ctx context.Context) ([]ClipStat, error) {
	rows, err := a.pool.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (ca.clip_id, ca.platform)
				ca.clip_id, ca.platform, ca.views, ca.likes, ca.comments, ca.shares,
				ca.engagement_rate, ca.avg_view_percentage, ca.subscribers_gained
			FROM clip_analytics ca
			WHERE ca.fetched_at >= NOW() - INTERVAL '14 days'
			  AND ca.platform IN ('youtube', 'tiktok')
			ORDER BY ca.clip_id, ca.platform, ca.fetched_at DESC
		)
		SELECT c.id, c.title, c.category, COALESCE(s.voice_text, ''),
		       l.platform, l.views, l.likes, l.comments, l.shares,
		       l.engagement_rate, l.avg_view_percentage, l.subscribers_gained
		FROM latest l
		JOIN clips c ON c.id = l.clip_id
		LEFT JOIN LATERAL (
			SELECT voice_text FROM scenes WHERE clip_id = c.id ORDER BY scene_number ASC LIMIT 1
		) s ON true
		WHERE c.status = 'published'
		  AND NOT EXISTS (
			SELECT 1 FROM clip_publish_status ps
			WHERE ps.clip_id = l.clip_id AND ps.platform = l.platform AND ps.status = 'failed')
		ORDER BY l.platform, l.views DESC
		LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("query recent analytics: %w", err)
	}
	defer rows.Close()

	var stats []ClipStat
	for rows.Next() {
		var s ClipStat
		if err := rows.Scan(&s.ID, &s.Title, &s.Category, &s.Hook,
			&s.Platform, &s.Views, &s.Likes, &s.Comments, &s.Shares,
			&s.EngagementRate, &s.AvgViewPct, &s.SubsGained); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate analytics: %w", err)
	}

	trends, err := a.fetchTrends(ctx)
	if err != nil {
		log.Printf("Analyzer: trend query failed (labels default to unknown): %v", err)
	}
	for i := range stats {
		if label, ok := trends[stats[i].ID+"|"+stats[i].Platform]; ok {
			stats[i].Trend = label
		} else {
			stats[i].Trend = "unknown"
		}
	}
	FillPercentiles(stats)
	return stats, nil
}

// fetchTrends derives a trend label per clip+platform from daily snapshot maxima
// over the last 8 days (the daily 04:00 fetch gives one snapshot per day).
func (a *Analyzer) fetchTrends(ctx context.Context) (map[string]string, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT clip_id, platform, DATE_TRUNC('day', fetched_at)::date AS day, MAX(views)
		FROM clip_analytics
		WHERE fetched_at >= NOW() - INTERVAL '8 days'
		  AND platform IN ('youtube', 'tiktok')
		GROUP BY clip_id, platform, day
		ORDER BY clip_id, platform, day ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	series := map[string][]int{}
	for rows.Next() {
		var clipID, platform string
		var day time.Time
		var views int
		if err := rows.Scan(&clipID, &platform, &day, &views); err != nil {
			return nil, err
		}
		key := clipID + "|" + platform
		series[key] = append(series[key], views)
	}
	out := map[string]string{}
	for key, views := range series {
		out[key] = TrendLabel(views)
	}
	return out, rows.Err()
}

func (a *Analyzer) currentPrompts(ctx context.Context) string {
	agents, err := a.agentsRepo.List(ctx)
	if err != nil {
		return "(failed to load current prompts)"
	}
	var lines []string
	for _, ag := range agents {
		if ag.AgentName == "analytics" {
			continue
		}
		section := fmt.Sprintf("### %s", ag.AgentName)
		if ag.Skills != "" {
			section += fmt.Sprintf("\n**Skills:**\n%s", ag.Skills)
		}
		if ag.Insights != "" {
			section += fmt.Sprintf("\n**Current Insights:**\n%s", ag.Insights)
		} else {
			section += "\n**Current Insights:** (none yet)"
		}
		lines = append(lines, section)
	}
	return strings.Join(lines, "\n\n---\n\n")
}
