package agent

import (
	"context"
	"fmt"

	"github.com/jaochai/video-fb/internal/models"
)

// CompositionAgent is the LLM that designs each video's look: it picks a layout
// variant, accent colors, animation speed, which title words to highlight, and
// the point cards (synced to the audio timeline). This is what lets the system
// "create new designs" instead of using one fixed template.
type CompositionAgent struct {
	llm *LLMClient
}

func NewCompositionAgent(llm *LLMClient) *CompositionAgent {
	return &CompositionAgent{llm: llm}
}

// CompositionCard is one point card the agent decides to show during a time window.
type CompositionCard struct {
	Type     string  `json:"type"`  // "cause" | "step" | "win"
	StartSec float64 `json:"start"` // synced to the transcript timeline
	EndSec   float64 `json:"end"`
	Kicker   string  `json:"kicker"`
	Body     string  `json:"body"`
	StepNum  int     `json:"step"`
}

// CompositionDecision is the design the agent returns. Media paths, brand, the
// title text and caption segments are filled in by the orchestrator — the agent
// only decides the creative/design layer.
type CompositionDecision struct {
	LayoutVariant   string            `json:"layout_variant"`
	AccentColor     string            `json:"accent_color"`
	SecondaryAccent string            `json:"secondary_accent"`
	AnimationSpeed  string            `json:"animation_speed"`
	Kicker          string            `json:"kicker"`
	HighlightWords  []string          `json:"highlight_words"`
	Cards           []CompositionCard `json:"cards"`
}

// CompositionTemplateData feeds the agent's prompt template.
type CompositionTemplateData struct {
	Question        string
	VoiceText       string
	Category        string
	QuestionerName  string
	FormatName      string
	DurationSeconds float64
	SegmentsContext string // transcript phrases+timestamps so the agent times cards to the audio
}

// Decide asks the LLM to design the composition for one clip.
func (a *CompositionAgent) Decide(ctx context.Context, data CompositionTemplateData, cfg *models.AgentConfig) (*CompositionDecision, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("render composition template: %w", err)
	}

	var decision CompositionDecision
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &decision); err != nil {
		return nil, fmt.Errorf("generate composition decision: %w", err)
	}
	return &decision, nil
}
