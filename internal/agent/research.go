package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jaochai/video-fb/internal/repository"
)

// ResearchAgent finds fresh, reliable information via Gemini googleSearch
// grounding (kie.ai). No crawler or embedding pipeline needed.
//
// Design note: the prompt is deliberately simple. Strict source whitelists or
// explicit "answer empty if unsure" escape hatches make search models bail out
// and return nothing.
type ResearchAgent struct {
	llm        *KieLLMClient
	agentsRepo *repository.AgentsRepo
}

func NewResearchAgent(llm *KieLLMClient, agentsRepo *repository.AgentsRepo) *ResearchAgent {
	return &ResearchAgent{llm: llm, agentsRepo: agentsRepo}
}

// Research searches the web for recent, reliable information about the topic.
// Returns a Thai-language context block for content generation, or "" when no
// substantial information was found (callers decide the fallback).
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

	userPrompt := fmt.Sprintf(`ค้นหาข่าวล่าสุดเกี่ยวกับ: %s

สรุป 3 ข่าวล่าสุด แต่ละข่าวบอก: เกิดอะไรขึ้น / มีผลเมื่อไหร่ / กระทบคนยิงแอดยังไง / URL แหล่งที่มา
ตอบเป็นภาษาไทย อย่าใช้ข้อมูลจากเว็บที่ขายบริการกู้บัญชี ปลดแบน หรือขาย/เช่าบัญชีโฆษณา`, topic)

	text, err := a.llm.GenerateWithSearch(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature)
	if err != nil {
		return "", fmt.Errorf("research search: %w", err)
	}

	if !isSubstantialResearch(text) {
		log.Printf("ResearchAgent: no substantial information found for %q", topic)
		return "", nil
	}
	return strings.TrimSpace(text), nil
}

// minResearchLength: real news summaries run far longer than refusals or
// "nothing found" style responses.
const minResearchLength = 200

// isSubstantialResearch reports whether a research response contains real
// findings rather than a refusal/empty answer. Pure function — testable.
func isSubstantialResearch(text string) bool {
	t := strings.TrimSpace(text)
	if utf8.RuneCountInString(t) < minResearchLength {
		return false
	}
	if strings.Contains(t, "NO_NEWS_FOUND") {
		return false
	}
	return true
}
