package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jaochai/video-fb/internal/rag"
)

type ScriptTemplateData struct {
	Question       string
	QuestionerName string
	Category       string
	RAGContext     string
}

type ScriptAgent struct {
	llm *LLMClient
	rag *rag.Engine
}

func NewScriptAgent(llm *LLMClient, ragEngine *rag.Engine) *ScriptAgent {
	return &ScriptAgent{llm: llm, rag: ragEngine}
}

type GeneratedScene struct {
	SceneNumber     int             `json:"scene_number"`
	SceneType       string          `json:"scene_type"`
	TextContent     string          `json:"text_content"`
	VoiceText       string          `json:"voice_text"`
	DurationSeconds float64         `json:"duration_seconds"`
	TextOverlays    json.RawMessage `json:"text_overlays"`
}

type GeneratedScript struct {
	Scenes             []GeneratedScene `json:"scenes"`
	TotalDuration      float64          `json:"total_duration_seconds"`
	YoutubeTitle       string           `json:"youtube_title"`
	YoutubeDescription string           `json:"youtube_description"`
	YoutubeTags        []string         `json:"youtube_tags"`
}

func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category, model, systemPrompt string, temperature float64, promptTemplate string) (*GeneratedScript, error) {
	ragResults, err := a.rag.Search(ctx, question, 5)
	if err != nil {
		return nil, fmt.Errorf("RAG search: %w", err)
	}

	var ragContext strings.Builder
	for _, r := range ragResults {
		ragContext.WriteString(r.Content)
		ragContext.WriteString("\n---\n")
	}

	userPrompt, err := renderTemplate(promptTemplate, ScriptTemplateData{
		Question:       question,
		QuestionerName: questionerName,
		Category:       category,
		RAGContext:     ragContext.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("render script template: %w", err)
	}

	var script GeneratedScript
	if err := a.llm.GenerateJSON(ctx, model, systemPrompt, userPrompt, temperature, &script); err != nil {
		return nil, fmt.Errorf("generate script: %w", err)
	}
	return &script, nil
}
