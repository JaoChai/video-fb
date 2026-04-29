package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/rag"
)

type QuestionTemplateData struct {
	Count          int
	Category       string
	RAGContext     string
	PreviousTopics string
	PreviousNames  string
}

type QuestionAgent struct {
	llm  *LLMClient
	rag  *rag.Engine
	pool *pgxpool.Pool
}

func NewQuestionAgent(llm *LLMClient, ragEngine *rag.Engine, pool *pgxpool.Pool) *QuestionAgent {
	return &QuestionAgent{llm: llm, rag: ragEngine, pool: pool}
}

type GeneratedQuestion struct {
	Question       string `json:"question"`
	QuestionerName string `json:"questioner_name"`
	Category       string `json:"category"`
	PainPoint      string `json:"pain_point"`
}

func (a *QuestionAgent) Generate(ctx context.Context, count int, category, model, systemPrompt string, temperature float64, promptTemplate string) ([]GeneratedQuestion, error) {
	ragResults, err := a.rag.Search(ctx, fmt.Sprintf("Facebook Ads %s problems common issues", category), 5)
	if err != nil {
		return nil, fmt.Errorf("RAG search: %w", err)
	}

	var ragContext strings.Builder
	for _, r := range ragResults {
		ragContext.WriteString(r.Content)
		ragContext.WriteString("\n---\n")
	}

	recentRows, err := a.pool.Query(ctx,
		`SELECT title FROM topic_history WHERE created_at > NOW() - INTERVAL '60 days' ORDER BY created_at DESC LIMIT 30`)
	if err != nil {
		return nil, fmt.Errorf("query recent topics: %w", err)
	}
	defer recentRows.Close()

	var recent []string
	for recentRows.Next() {
		var t string
		recentRows.Scan(&t)
		recent = append(recent, t)
	}

	previousList := ""
	if len(recent) > 0 {
		previousList = "\n\nห้ามซ้ำกับหัวข้อเหล่านี้:\n- " + strings.Join(recent, "\n- ")
	}

	nameRows, err := a.pool.Query(ctx,
		`SELECT DISTINCT questioner_name FROM clips WHERE created_at > NOW() - INTERVAL '60 days' AND questioner_name != '' ORDER BY questioner_name LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("query recent names: %w", err)
	}
	defer nameRows.Close()

	var recentNames []string
	for nameRows.Next() {
		var n string
		nameRows.Scan(&n)
		recentNames = append(recentNames, n)
	}

	previousNames := ""
	if len(recentNames) > 0 {
		previousNames = "\n\nห้ามใช้ชื่อซ้ำกับเหล่านี้: " + strings.Join(recentNames, ", ")
	}

	userPrompt, err := renderTemplate(promptTemplate, QuestionTemplateData{
		Count:          count,
		Category:       category,
		RAGContext:     ragContext.String(),
		PreviousTopics: previousList,
		PreviousNames:  previousNames,
	})
	if err != nil {
		return nil, fmt.Errorf("render question template: %w", err)
	}

	var questions []GeneratedQuestion
	if err := a.llm.GenerateJSON(ctx, model, systemPrompt, userPrompt, temperature, &questions); err != nil {
		return nil, fmt.Errorf("generate questions: %w", err)
	}

	for _, q := range questions {
		a.pool.Exec(ctx,
			`INSERT INTO topic_history (title, category) VALUES ($1, $2)`,
			q.Question, q.Category)
	}

	return questions, nil
}
