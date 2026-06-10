package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/rag"
)

type ScriptTemplateData struct {
	Question          string
	QuestionerName    string
	Category          string
	RAGContext        string
	FormatInstruction string
	AudiencePersona   string
}

type ScriptAgent struct {
	llm      *KieLLMClient
	rag      *rag.Engine
	research *ResearchAgent
}

func NewScriptAgent(llm *KieLLMClient, ragEngine *rag.Engine, research *ResearchAgent) *ScriptAgent {
	return &ScriptAgent{llm: llm, rag: ragEngine, research: research}
}

type GeneratedScene struct {
	SceneNumber     int             `json:"scene_number"`
	SceneType       string          `json:"scene_type"`
	TextContent     string          `json:"text_content"`
	VoiceText       string          `json:"voice_text"`
	DurationSeconds float64         `json:"duration_seconds"`
	TextOverlays    json.RawMessage `json:"text_overlays"`
	// Plan 2b additions — populated by the new SceneAgent (Claude). The legacy
	// ScriptAgent leaves these empty, which is valid.
	LayoutVariant string   `json:"layout_variant"`
	OnScreenText  string   `json:"on_screen_text"`
	EmphasisWords []string `json:"emphasis_words"`
	Beat          string   `json:"beat"`
	CaptionStyle  string   `json:"caption_style"`
	ImagePrompt   string   `json:"image_prompt"`
	// Style-B structured content (Plan 2b-6). Emitted by the upgraded SceneAgent.
	Layout  string          `json:"layout"`  // hook|hero|stat|step|tip|cta
	Content json.RawMessage `json:"content"` // typed per layout; parsed in scene_adapter.go
}

type GeneratedScript struct {
	Scenes             []GeneratedScene `json:"scenes"`
	TotalDuration      float64          `json:"total_duration_seconds"`
	YoutubeTitle       string           `json:"youtube_title"`
	YoutubeDescription string           `json:"youtube_description"`
	YoutubeTags        []string         `json:"youtube_tags"`
}

func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category string, format *models.ContentFormat, persona string, cfg *models.AgentConfig) (*GeneratedScript, error) {
	var ragContext strings.Builder

	if format.FormatName == "news" {
		// News format: research the specific story for accurate, current facts
		researchContext, err := a.research.Research(ctx, question)
		if err != nil {
			log.Printf("ScriptAgent: research failed, continuing with KB only: %v", err)
		}
		if researchContext != "" {
			ragContext.WriteString(researchContext)
			ragContext.WriteString("\n---\n")
		}
	}

	// Business knowledge + brand context from the hand-written Thai KB (all formats)
	ragResults, err := a.rag.Search(ctx, question, 5)
	if err != nil {
		return nil, fmt.Errorf("RAG search: %w", err)
	}
	for _, r := range ragResults {
		ragContext.WriteString(r.Content)
		ragContext.WriteString("\n---\n")
	}

	userPrompt, err := renderTemplate(cfg.PromptTemplate, ScriptTemplateData{
		Question:          question,
		QuestionerName:    questionerName,
		Category:          category,
		RAGContext:        ragContext.String(),
		FormatInstruction: format.ScriptInstruction,
		AudiencePersona:   persona,
	})
	if err != nil {
		return nil, fmt.Errorf("render script template: %w", err)
	}

	var script GeneratedScript
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &script); err != nil {
		return nil, fmt.Errorf("generate script: %w", err)
	}
	return &script, nil
}
