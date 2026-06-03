package analyzer

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/repository"
)

const historyPrefixInsights = "[insights] "

type Analyzer struct {
	pool       *pgxpool.Pool
	llm        *agent.LLMClient
	agentsRepo *repository.AgentsRepo
}

func New(pool *pgxpool.Pool, llm *agent.LLMClient, agentsRepo *repository.AgentsRepo) *Analyzer {
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
	data, err := a.gatherData(ctx)
	if err != nil {
		return fmt.Errorf("gather analytics data: %w", err)
	}

	if data == "" {
		log.Println("Analyzer: not enough data to analyze (need at least 3 published clips with analytics)")
		return nil
	}

	analyticsAgent, err := a.agentsRepo.GetByName(ctx, "analytics")
	if err != nil {
		return fmt.Errorf("get analytics agent config: %w", err)
	}

	userPrompt := fmt.Sprintf(`Here is the performance data from our YouTube channel for the last 14 days:

%s

Current agent configurations:
%s

Analyze which STORYTELLING STYLES performed best (openings, hooks, pacing, tone, length).

YOUR SCOPE IS STRICTLY LIMITED TO STYLE:
- You may suggest: how to open videos, hook techniques, pacing, tone of voice, energy level
- You may NOT mention any category name (account, payment, campaign, pixel) in your suggestions
- You may NOT tell agents which topics to focus on, avoid, prioritize, or exclude
- Topic selection is handled by a separate system — it is NOT your job

Each insight must be under 1000 characters, written in Thai.

Return JSON only:
{
  "agents": [
    {"agent_name": "question", "new_insights": "...", "reason": "..."},
    {"agent_name": "script", "new_insights": "...", "reason": "..."},
    {"agent_name": "image", "new_insights": "...", "reason": "..."}
  ]
}`, data, a.currentPrompts(ctx))

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

		if err := a.agentsRepo.SavePromptHistory(ctx, imp.AgentName, oldInsights, imp.NewInsights, historyPrefixInsights+imp.Reason); err != nil {
			log.Printf("Analyzer: failed to save history for %s: %v", imp.AgentName, err)
			continue
		}

		if err := a.agentsRepo.UpdateInsightsByName(ctx, imp.AgentName, imp.NewInsights); err != nil {
			log.Printf("Analyzer: failed to update insights for %s: %v", imp.AgentName, err)
			continue
		}

		log.Printf("Analyzer: updated %s insights — reason: %s", imp.AgentName, imp.Reason)
	}

	return nil
}

func (a *Analyzer) gatherData(ctx context.Context) (string, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT c.id, c.title, c.category,
		       cm.youtube_title,
		       ca.views, ca.likes, ca.comments, ca.shares,
		       ca.watch_time_seconds, ca.retention_rate
		FROM clips c
		JOIN clip_metadata cm ON c.id = cm.clip_id
		JOIN clip_analytics ca ON c.id = ca.clip_id
		WHERE c.status = 'published'
		  AND ca.platform = 'youtube'
		  AND ca.fetched_at >= NOW() - INTERVAL '14 days'
		ORDER BY ca.fetched_at DESC
		LIMIT 100`)
	if err != nil {
		return "", fmt.Errorf("query recent analytics: %w", err)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var id, title, category string
		var ytTitle *string
		var views, likes, comments, shares int
		var watchTime, retention float64

		if err := rows.Scan(&id, &title, &category,
			&ytTitle,
			&views, &likes, &comments, &shares,
			&watchTime, &retention); err != nil {
			return "", fmt.Errorf("scan: %w", err)
		}

		yt := ""
		if ytTitle != nil {
			yt = *ytTitle
		}

		lines = append(lines, fmt.Sprintf(
			"- Clip: %s | Title: %s | YT Title: %s | Category: %s | Views: %d | Likes: %d | Comments: %d | Shares: %d | Watch Time: %.0fs | Retention: %.1f%%",
			id[:8], title, yt, category, views, likes, comments, shares, watchTime, retention*100))
	}

	if len(lines) < 3 {
		return "", nil
	}

	return strings.Join(lines, "\n"), nil
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
		section := fmt.Sprintf("### %s\n**System Prompt:**\n%s", ag.AgentName, ag.SystemPrompt)
		if ag.Skills != "" {
			section += fmt.Sprintf("\n\n**Skills:**\n%s", ag.Skills)
		}
		if ag.Insights != "" {
			section += fmt.Sprintf("\n\n**Current Insights:**\n%s", ag.Insights)
		}
		if ag.PromptTemplate != "" {
			section += fmt.Sprintf("\n\n**Prompt Template:**\n%s", ag.PromptTemplate)
		}
		lines = append(lines, section)
	}
	return strings.Join(lines, "\n\n---\n\n")
}
