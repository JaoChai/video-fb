package agent

import (
	"context"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)

// LearnInput is one upstream agent's current skills plus a human-readable summary
// of the recurring critique patterns the learner should fix. PatternSummary is
// built by the Learner service from CritiquesRepo.LowScorePatterns; the agent
// package stays decoupled from the repository layer.
type LearnInput struct {
	AgentName     string
	CurrentSkills string
	Patterns      string // pre-rendered pattern summary (scores + top issues)
	WindowDays    int
}

// LearnOutput is the raw JSON the learner LLM returns.
type LearnOutput struct {
	NewSkills string `json:"new_skills"`
	Rationale string `json:"rationale"`
	Confident bool   `json:"confident"`
}

// learnerTemplateData fills the seeded `learner` prompt_template.
type learnerTemplateData struct {
	AgentName      string
	CurrentSkills  string
	PatternSummary string
	WindowDays     int
}

// LearnerAgent proposes improved skills guidelines for an upstream agent from
// recurring quality issues. Runs on Claude (cfg.Model is claude-sonnet-5).
type LearnerAgent struct {
	llm *KieLLMClient
}

func NewLearnerAgent(llm *KieLLMClient) *LearnerAgent {
	return &LearnerAgent{llm: llm}
}

// Propose asks the LLM for improved skills text. cfg is the `learner` AgentConfig
// fetched by the caller via GetByName. It returns the raw proposal; the caller
// MUST gate it through AcceptProposal before applying anything.
func (a *LearnerAgent) Propose(ctx context.Context, in LearnInput, cfg *models.AgentConfig) (*LearnOutput, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, learnerTemplateData{
		AgentName:      in.AgentName,
		CurrentSkills:  in.CurrentSkills,
		PatternSummary: in.Patterns,
		WindowDays:     in.WindowDays,
	})
	if err != nil {
		return nil, err
	}

	var out LearnOutput
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AcceptProposal is the guardrail deciding whether a proposal may be applied.
// Exported so the learner service (a different package) can gate on it. Pure.
// Rejects when: the LLM is not confident, the new skills are empty/blank, or the
// new skills are identical to the current text (after trimming). This is what
// guarantees the loop can never blank or no-op an agent's skills.
func AcceptProposal(in LearnInput, out *LearnOutput) bool {
	if out == nil || !out.Confident {
		return false
	}
	next := strings.TrimSpace(out.NewSkills)
	if next == "" {
		return false
	}
	if next == strings.TrimSpace(in.CurrentSkills) {
		return false
	}
	return true
}
