package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jaochai/video-fb/internal/repository"
)

// ResearchAgent finds fresh, reliable information via live web search.
// Its configured model must have native web search (e.g. perplexity/sonar) —
// no crawler or embedding pipeline needed.
type ResearchAgent struct {
	llm        *LLMClient
	agentsRepo *repository.AgentsRepo
}

func NewResearchAgent(llm *LLMClient, agentsRepo *repository.AgentsRepo) *ResearchAgent {
	return &ResearchAgent{llm: llm, agentsRepo: agentsRepo}
}

type researchResult struct {
	Summary  string   `json:"summary"`
	KeyFacts []string `json:"key_facts"`
	Sources  []string `json:"sources"`
}

// Research searches the web for recent, reliable information about the topic.
// Returns a Thai-language context block for content generation, or "" when no
// trustworthy information was found (callers should fall back to the KB).
func (a *ResearchAgent) Research(ctx context.Context, topic string) (string, error) {
	cfg, err := a.agentsRepo.GetByName(ctx, "research")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Config not seeded yet — treat as "no research available", not an error.
			log.Printf("ResearchAgent: config row not found, skipping research")
			return "", nil
		}
		return "", fmt.Errorf("get research agent config: %w", err)
	}
	if !cfg.Enabled {
		return "", nil
	}

	userPrompt := fmt.Sprintf(`ค้นหาข้อมูลล่าสุดที่เชื่อถือได้เกี่ยวกับ: %s

กฎการคัดกรองแหล่งข้อมูล:
- เชื่อถือได้: ประกาศจาก Meta โดยตรง (Meta Newsroom, Meta for Business), สื่อวงการโฆษณา (Search Engine Land, ppc.land), ประกาศหน่วยงานราชการไทย (ETDA, กสทช.)
- ห้ามใช้: บล็อกของ agency ที่ขายบริการกู้บัญชี/ปลดแบน, โพสต์ Reddit/forum, ข้อมูลที่เก่ากว่า 6 เดือน
- ถ้าข้อมูลจากหลายแหล่งขัดแย้งกัน ให้เชื่อแหล่งทางการของ Meta

ตอบเป็น JSON object:
{
  "summary": "สรุปข้อมูลภาษาไทย พร้อมระบุวันที่ของข้อมูล",
  "key_facts": ["ข้อเท็จจริงสำคัญพร้อมตัวเลข/วันที่", "..."],
  "sources": ["URL ของแหล่งที่ใช้อ้างอิง"]
}

สำคัญมาก: ถ้าหาข้อมูลที่เชื่อถือได้ไม่เจอ ให้ตอบ summary เป็นสตริงว่าง ห้ามแต่งข้อมูลขึ้นเองเด็ดขาด`, topic)

	var result researchResult
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &result); err != nil {
		return "", fmt.Errorf("research search: %w", err)
	}

	context := buildResearchContext(result)
	if context == "" {
		log.Printf("ResearchAgent: no reliable information found for %q", topic)
	}
	return context, nil
}

// buildResearchContext formats a research result into a prompt context block.
// Pure function — testable without network/DB.
func buildResearchContext(result researchResult) string {
	if strings.TrimSpace(result.Summary) == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(result.Summary)
	if len(result.KeyFacts) > 0 {
		sb.WriteString("\n\nข้อเท็จจริงสำคัญ:\n")
		for _, f := range result.KeyFacts {
			sb.WriteString("- " + f + "\n")
		}
	}
	if len(result.Sources) > 0 {
		sb.WriteString("\nแหล่งอ้างอิง: " + strings.Join(result.Sources, ", "))
	}
	return sb.String()
}
