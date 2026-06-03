package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/rag"
)

// ErrNoFreshNews is returned when the news format finds no reliable fresh
// information — callers must switch to another format rather than let the
// LLM fabricate news from stale knowledge.
var ErrNoFreshNews = errors.New("no fresh news found from research")

type QuestionTemplateData struct {
	Count             int
	Category          string
	RAGContext        string
	PreviousTopics    string
	PreviousNames     string
	FormatInstruction string
	AudiencePersona   string
}

type QuestionAgent struct {
	llm      *LLMClient
	rag      *rag.Engine
	pool     *pgxpool.Pool
	deduper  *Deduper
	research *ResearchAgent
}

func NewQuestionAgent(llm *LLMClient, ragEngine *rag.Engine, pool *pgxpool.Pool, research *ResearchAgent) *QuestionAgent {
	return &QuestionAgent{llm: llm, rag: ragEngine, pool: pool, deduper: NewDeduper(pool, ragEngine), research: research}
}

type GeneratedQuestion struct {
	Question       string `json:"question"`
	QuestionerName string `json:"questioner_name"`
	Category       string `json:"category"`
	PainPoint      string `json:"pain_point"`
}

func (a *QuestionAgent) Generate(ctx context.Context, count int, category string, format *models.ContentFormat, persona string, cfg *models.AgentConfig) ([]GeneratedQuestion, error) {
	var ragContext strings.Builder
	if format.FormatName == "news" {
		// News format: live web search for fresh, reliable updates.
		// Never fall back to stale KB here — that produces fabricated news.
		researchContext, err := a.research.Research(ctx, "Facebook Ads หรือ Meta ที่กระทบผู้ลงโฆษณาในไทย")
		if err != nil {
			log.Printf("QuestionAgent: research failed: %v", err)
		}
		if researchContext == "" {
			return nil, ErrNoFreshNews
		}
		ragContext.WriteString(researchContext)
		ragContext.WriteString("\n---\n")
	} else {
		// Business knowledge from the hand-written Thai KB
		ragResults, err := a.rag.Search(ctx, fmt.Sprintf("Facebook Ads %s %s", category, format.FormatName), 5)
		if err != nil {
			return nil, fmt.Errorf("RAG search: %w", err)
		}
		for _, r := range ragResults {
			ragContext.WriteString(r.Content)
			ragContext.WriteString("\n---\n")
		}
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

	userPrompt, err := renderTemplate(cfg.PromptTemplate, QuestionTemplateData{
		Count:             count,
		Category:          category,
		RAGContext:        ragContext.String(),
		PreviousTopics:    previousList,
		PreviousNames:     previousNames,
		FormatInstruction: format.QuestionInstruction,
		AudiencePersona:   persona,
	})
	if err != nil {
		return nil, fmt.Errorf("render question template: %w", err)
	}

	var questions []GeneratedQuestion
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &questions); err != nil {
		return nil, fmt.Errorf("generate questions: %w", err)
	}

	// Semantic dedup: reject questions too similar to past topics, retry up to 2 times.
	const maxDedupRetries = 2
	var accepted []GeneratedQuestion
	allEmbeddings := make(map[string][]float64)

	for attempt := 0; ; attempt++ {
		similarities, embeddings, err := a.deduper.CheckQuestions(ctx, questions)
		if err != nil {
			// Embedding service down — accept as-is rather than block production.
			log.Printf("QuestionAgent: dedup check failed, accepting without dedup: %v", err)
			accepted = append(accepted, questions...)
			break
		}
		for k, v := range embeddings {
			allEmbeddings[k] = v
		}

		passed, rejected := filterBySimilarity(questions, similarities, similarityThreshold)
		accepted = append(accepted, passed...)

		if len(rejected) == 0 || len(accepted) >= count || attempt >= maxDedupRetries {
			for _, r := range rejected {
				log.Printf("QuestionAgent: rejected duplicate %q (%.0f%% similar to %q)",
					r.Question.Question, r.Match.Similarity*100, r.Match.MatchedTitle)
			}
			break
		}

		// Ask the LLM to regenerate replacements, telling it what was rejected and why.
		var rejectedInfo strings.Builder
		for _, r := range rejected {
			rejectedInfo.WriteString(fmt.Sprintf("- %q ซ้ำกับ %q\n", r.Question.Question, r.Match.MatchedTitle))
		}
		retryPrompt := userPrompt + fmt.Sprintf(
			"\n\nคำถามต่อไปนี้ถูกปฏิเสธเพราะความหมายซ้ำกับคลิปเก่า สร้างใหม่ %d ข้อที่เป็นมุมมองใหม่จริงๆ:\n%s",
			len(rejected), rejectedInfo.String())

		questions = nil
		if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), retryPrompt, cfg.Temperature, &questions); err != nil {
			log.Printf("QuestionAgent: dedup retry generation failed: %v", err)
			break
		}
	}

	// Cap to the requested count so unused questions don't pollute topic_history.
	if len(accepted) > count {
		accepted = accepted[:count]
	}

	// Store accepted questions with embeddings for future dedup checks.
	for _, q := range accepted {
		if emb, ok := allEmbeddings[q.Question]; ok {
			a.pool.Exec(ctx,
				`INSERT INTO topic_history (title, category, embedding) VALUES ($1, $2, $3::vector)`,
				q.Question, q.Category, rag.FormatVector(emb))
		} else {
			a.pool.Exec(ctx,
				`INSERT INTO topic_history (title, category) VALUES ($1, $2)`,
				q.Question, q.Category)
		}
	}

	return accepted, nil
}
