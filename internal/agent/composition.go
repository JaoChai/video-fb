package agent

import (
	"context"
	"fmt"
	"strings"

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

// Slot is one semantic content slot the composition agent fills for a scene.
// The agent chooses role + text + emphasis; the template (engine) owns geometry,
// so the agent never sends pixel positions — this is how overlap is prevented.
type Slot struct {
	Role     string   `json:"role"`     // "headline" | "body" | "badge" | "step"
	Text     string   `json:"text"`
	Emphasis []string `json:"emphasis"` // words inside Text to accent
}

// SceneDesign is the per-scene visual design (no pixel coordinates).
type SceneDesign struct {
	SceneNumber    int    `json:"scene_number"`
	LayoutVariant  string `json:"layout_variant"`
	Slots          []Slot `json:"slots"`
	AccentColor    string `json:"accent_color"`  // sanitized later in producer
	BgArtPrompt    string `json:"bg_art_prompt"` // text-free background art prompt
	AnimationSpeed string `json:"animation_speed"`
}

// ScenesDecision is the multi-scene design DecideScenes returns.
type ScenesDecision struct {
	Scenes         []SceneDesign `json:"scenes"`
	Kicker         string        `json:"kicker"`
	HighlightWords []string      `json:"highlight_words"`
}

// Layout variants the template library implements (Phase 3).
const (
	LayoutHookBig    = "hook_big"
	LayoutListSteps  = "list_steps"
	LayoutStatReveal = "stat_reveal"
	LayoutQuoteCTA   = "quote_cta"
)

const defaultLayoutVariant = LayoutListSteps

var validLayoutVariants = map[string]bool{
	LayoutHookBig: true, LayoutListSteps: true, LayoutStatReveal: true, LayoutQuoteCTA: true,
}

const defaultSlotRole = "body"

var validSlotRoles = map[string]bool{
	"headline": true, "body": true, "badge": true, "step": true,
}

// Normalize keeps LLM scene designs safe: defaults unknown layout_variant /
// animation_speed, drops empty-text slots, and defaults unknown slot roles.
// Accent-color sanitization is left to the producer (which owns sanitizeHexColor).
func (d *ScenesDecision) Normalize() {
	for i := range d.Scenes {
		if !validLayoutVariants[d.Scenes[i].LayoutVariant] {
			d.Scenes[i].LayoutVariant = defaultLayoutVariant
		}
		if d.Scenes[i].AnimationSpeed != "fast" && d.Scenes[i].AnimationSpeed != "slow" {
			d.Scenes[i].AnimationSpeed = "normal"
		}
		kept := d.Scenes[i].Slots[:0]
		for _, s := range d.Scenes[i].Slots {
			if strings.TrimSpace(s.Text) == "" {
				continue
			}
			if !validSlotRoles[s.Role] {
				s.Role = defaultSlotRole
			}
			kept = append(kept, s)
		}
		d.Scenes[i].Slots = kept
	}
}

// ScenesTemplateData feeds the composition_scenes prompt. ScenesJSON is the
// script's scenes (number, type, headline, voice_text, bg_hint) marshaled to JSON
// so the agent designs one SceneDesign per script scene.
type ScenesTemplateData struct {
	ScenesJSON      string
	Category        string
	QuestionerName  string
	DurationSeconds float64
}

// DecideScenes asks the LLM to design every scene as semantic slots. cfg must be
// the 'composition_scenes' agent config. The result is Normalized before return.
// Not wired into the producer yet (Phase 4).
func (a *CompositionAgent) DecideScenes(ctx context.Context, data ScenesTemplateData, cfg *models.AgentConfig) (*ScenesDecision, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("render composition_scenes template: %w", err)
	}

	var decision ScenesDecision
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &decision); err != nil {
		return nil, fmt.Errorf("generate scenes decision: %w", err)
	}
	decision.Normalize()
	return &decision, nil
}
