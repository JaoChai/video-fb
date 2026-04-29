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

const historyPrefixSkills = "[skills] "

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
	AgentName string `json:"agent_name"`
	NewSkills string `json:"new_skills"`
	Reason    string `json:"reason"`
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

Current agent configurations (system_prompt + skills + prompt_template):
%s

Based on this data, analyze which videos performed best and worst.
Then improve each agent's "skills" field to produce more engaging content.

CRITICAL RULES — you MUST preserve these in every new skills text:
- Script: "ห้ามมีอักขระ @ และห้ามมี URL ใดๆ ใน voice_text" must remain
- Script: "เรียกแบรนด์ว่า แอดส์แวนซ์ ใน voice_text" must remain
- Script: JSON output format rules (scenes array, youtube_title, etc.) must remain
- Image: "ห้ามใส่ logo, mascot, ชื่อแบรนด์" must remain
- Question: "ห้ามสร้างคำถามที่แนะนำการทำผิดนโยบาย" must remain
- All agents: JSON output format instructions must remain

You may ONLY change the "skills" field. Do NOT change system_prompt or prompt_template.
Focus on: content quality, engagement hooks, audience targeting, variety.

Return JSON only:
{
  "agents": [
    {"agent_name": "question", "new_skills": "...", "reason": "..."},
    {"agent_name": "script", "new_skills": "...", "reason": "..."},
    {"agent_name": "image", "new_skills": "...", "reason": "..."}
  ]
}`, data, a.currentPrompts(ctx))

	var result improvementResult
	err = a.llm.GenerateJSON(ctx, analyticsAgent.Model, analyticsAgent.SystemPrompt, userPrompt, analyticsAgent.Temperature, &result)
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
		agentMap[ag.AgentName] = ag.Skills
	}

	for _, imp := range result.Agents {
		if imp.AgentName == "analytics" || imp.NewSkills == "" {
			continue
		}

		oldSkills, exists := agentMap[imp.AgentName]
		if !exists {
			log.Printf("Analyzer: skip unknown agent %s", imp.AgentName)
			continue
		}

		if err := a.agentsRepo.SavePromptHistory(ctx, imp.AgentName, oldSkills, imp.NewSkills, historyPrefixSkills+imp.Reason); err != nil {
			log.Printf("Analyzer: failed to save history for %s: %v", imp.AgentName, err)
			continue
		}

		if err := a.agentsRepo.UpdateSkillsByName(ctx, imp.AgentName, imp.NewSkills); err != nil {
			log.Printf("Analyzer: failed to update skills for %s: %v", imp.AgentName, err)
			continue
		}

		log.Printf("Analyzer: updated %s skills — reason: %s", imp.AgentName, imp.Reason)
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
		if ag.PromptTemplate != "" {
			section += fmt.Sprintf("\n\n**Prompt Template:**\n%s", ag.PromptTemplate)
		}
		lines = append(lines, section)
	}
	return strings.Join(lines, "\n\n---\n\n")
}
