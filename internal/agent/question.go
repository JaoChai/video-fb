package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/rag"
)

// ErrNoFreshNews is returned when the news format finds no reliable fresh
// information — callers must switch to another format rather than let the
// LLM fabricate news from stale knowledge.
var ErrNoFreshNews = errors.New("no fresh news found from research")

type QuestionTemplateData struct {
	Count                int
	Category             string
	CategoryAngle        string
	ArchetypeInstruction string
	RoleInstruction      string
	TopicStats           string
	RAGContext           string
	PreviousTopics       string
	PreviousNames        string
	FormatInstruction    string
	AudiencePersona      string
}

type QuestionAgent struct {
	llm              *KieLLMClient
	rag              *rag.Engine
	pool             *pgxpool.Pool
	deduper          *Deduper
	research         *ResearchAgent
	painCooldownDays int // 0 = skip cooldown (legacy); set จาก setting pain_point_cooldown_days
}

func NewQuestionAgent(llm *KieLLMClient, ragEngine *rag.Engine, pool *pgxpool.Pool, research *ResearchAgent) *QuestionAgent {
	return &QuestionAgent{llm: llm, rag: ragEngine, pool: pool, deduper: NewDeduper(pool, ragEngine), research: research}
}

// SetPainCooldownDays — orchestrator เรียกเมื่อ flag on (ค่าจาก setting pain_point_cooldown_days).
func (a *QuestionAgent) SetPainCooldownDays(days int) { a.painCooldownDays = days }

// Deduper — expose ให้ orchestrator set threshold ตอน flag on.
func (a *QuestionAgent) Deduper() *Deduper { return a.deduper }

type GeneratedQuestion struct {
	Question       string `json:"question"`
	QuestionerName string `json:"questioner_name"`
	Category       string `json:"category"`
	PainPoint      string `json:"pain_point"`
}

// cooldownFilterRetry ทิ้งคำถามที่ pain_point ติด cooldown แล้วขอ regen มาแทน
// (เลี่ยง pain_point ที่ทิ้งไป) สูงสุด maxRetries รอบ เพื่อไม่ให้ batch ที่ตัวแรก
// ชน cooldown คืน 0 คำถาม. fail-open: ถ้าทุกตัวติด cooldown จนครบ retry จะคืน
// คำถามตัวสุดท้ายที่ถูกทิ้งแทนการคืน empty — คลิปซ้ำนิดหน่อยยังดีกว่า produce 0 คลิป
// เงียบๆ. inCooldown error ถือว่า "ไม่ติด cooldown" (fail-open ตาม pattern เดิม)
func cooldownFilterRetry(
	ctx context.Context,
	initial []GeneratedQuestion,
	count, maxRetries int,
	inCooldown func(context.Context, string) (bool, error),
	regen func(ctx context.Context, avoid []string, n int) ([]GeneratedQuestion, error),
) []GeneratedQuestion {
	kept := make([]GeneratedQuestion, 0, len(initial))
	dropped := map[string]bool{}
	var lastDropped *GeneratedQuestion

	filter := func(qs []GeneratedQuestion) {
		for i := range qs {
			cd, err := inCooldown(ctx, qs[i].PainPoint)
			if err != nil {
				log.Printf("QuestionAgent: pain_point cooldown check error (fail-open): %v", err)
				kept = append(kept, qs[i])
				continue
			}
			if !cd {
				kept = append(kept, qs[i])
				continue
			}
			dropped[qs[i].PainPoint] = true
			lastDropped = &qs[i]
			log.Printf("QuestionAgent: pain_point %q in cooldown, dropped", qs[i].PainPoint)
		}
	}

	filter(initial)

	for attempt := 0; len(kept) < count && len(dropped) > 0 && attempt < maxRetries; attempt++ {
		avoid := make([]string, 0, len(dropped))
		for pp := range dropped {
			avoid = append(avoid, pp)
		}
		sort.Strings(avoid) // prompt/ลำดับ deterministic
		fresh, err := regen(ctx, avoid, count-len(kept))
		if err != nil {
			log.Printf("QuestionAgent: cooldown regen failed (attempt %d): %v", attempt+1, err)
			break
		}
		filter(fresh)
	}

	if len(kept) == 0 && lastDropped != nil {
		log.Printf("QuestionAgent: all pain_points in cooldown after %d retries, accepting %q to avoid 0-clip stall",
			maxRetries, lastDropped.PainPoint)
		kept = append(kept, *lastDropped)
	}

	if len(kept) > count {
		kept = kept[:count]
	}
	return kept
}

func (a *QuestionAgent) Generate(ctx context.Context, count int, category string, categoryAngle string, format *models.ContentFormat, persona string, archetypeInstr string, roleInstr string, topicStats string, cfg *models.AgentConfig) ([]GeneratedQuestion, error) {
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
		ragContext.WriteString("ข้อมูลข่าว/งานวิจัยสด (อ้างอิงได้):\n")
		ragContext.WriteString(researchContext)
		ragContext.WriteString("\n---\n")
	}
	// non-news: ไม่ ground ด้วย KB — gemini คิดจากเมนู pain_point ใน prompt (ตัด rag.Search ออก)

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
		Count:                count,
		Category:             category,
		CategoryAngle:        categoryAngle,
		ArchetypeInstruction: archetypeInstr,
		RoleInstruction:      roleInstr,
		TopicStats:           topicStats,
		RAGContext:           ragContext.String(),
		PreviousTopics:       previousList,
		PreviousNames:        previousNames,
		FormatInstruction:    format.QuestionInstruction,
		AudiencePersona:      persona,
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
			// Embedding ล่ม → retry 1 ครั้ง ก่อน fallback lexical
			log.Printf("QuestionAgent: dedup embedding error, retrying once: %v", err)
			time.Sleep(500 * time.Millisecond)
			similarities, embeddings, err = a.deduper.CheckQuestions(ctx, questions)
		}
		if err != nil {
			// ยังล่ม → lexical guard (pg_trgm) แทน ห้ามรับมั่ว
			log.Printf("QuestionAgent: dedup still failing, using lexical fallback: %v", err)
			blocked, lexErr := a.deduper.LexicalCheck(ctx, questions)
			if lexErr != nil {
				log.Printf("QuestionAgent: lexical fallback also failed, accepting all (last resort): %v", lexErr)
				accepted = append(accepted, questions...)
			} else {
				for _, q := range questions {
					if !blocked[q.Question] {
						accepted = append(accepted, q)
					}
				}
			}
			break
		}
		for k, v := range embeddings {
			allEmbeddings[k] = v
		}

		passed, rejected := filterBySimilarity(questions, similarities, a.deduper.threshold)
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

	// pain_point cooldown: กันหัวข้อเดิมเปลี่ยนมุม (flag on เท่านั้น — painCooldownDays > 0)
	// ถ้า pain_point ที่ได้ติด cooldown ให้ generate ใหม่โดยเลี่ยง pain_point นั้น
	// fail-open ถ้าทุกอันติด cooldown เพื่อไม่ให้ produce คืน 0 คลิป (ดู cooldownFilterRetry)
	if a.painCooldownDays > 0 && len(accepted) > 0 {
		const maxCooldownRetries = 2
		regen := func(ctx context.Context, avoid []string, n int) ([]GeneratedQuestion, error) {
			p := userPrompt + fmt.Sprintf(
				"\n\npain_point เหล่านี้ติด cooldown ห้ามใช้ ให้เลือก pain_point อื่นในหมวด %s:\n- %s\nสร้าง %d ข้อ",
				category, strings.Join(avoid, "\n- "), n)
			var qs []GeneratedQuestion
			if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), p, cfg.Temperature, &qs); err != nil {
				return nil, err
			}
			sims, embs, derr := a.deduper.CheckQuestions(ctx, qs)
			if derr != nil {
				return qs, nil // dedup ล่ม → รับ fresh batch ไปก่อน (สอดคล้อง fallback lexical เดิม)
			}
			for k, v := range embs {
				allEmbeddings[k] = v // เก็บ embedding ของคำถามที่ regen ไว้ dedup รอบหน้า
			}
			passed, _ := filterBySimilarity(qs, sims, a.deduper.threshold)
			return passed, nil
		}
		accepted = cooldownFilterRetry(ctx, accepted, count, maxCooldownRetries,
			func(ctx context.Context, pp string) (bool, error) {
				return a.deduper.PainPointInCooldown(ctx, pp, a.painCooldownDays)
			}, regen)
	}

	// Store accepted questions with embeddings for future dedup checks.
	for _, q := range accepted {
		if emb, ok := allEmbeddings[q.Question]; ok {
			a.pool.Exec(ctx,
				`INSERT INTO topic_history (title, category, pain_point, embedding) VALUES ($1, $2, $3, $4::vector)`,
				q.Question, q.Category, q.PainPoint, rag.FormatVector(emb))
		} else {
			a.pool.Exec(ctx,
				`INSERT INTO topic_history (title, category, pain_point) VALUES ($1, $2, $3)`,
				q.Question, q.Category, q.PainPoint)
		}
	}

	return accepted, nil
}
