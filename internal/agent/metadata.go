package agent

import (
	"context"
	"fmt"

	"github.com/jaochai/video-fb/internal/models"
)

// MetadataTemplateData fills the seeded `metadata` prompt template. Field names
// match the design §4.6 registry exactly.
type MetadataTemplateData struct {
	Topic           string
	Script          string
	Category        string
	AudiencePersona string
}

// GeneratedMetadata is the search-intent YouTube metadata the MetadataAgent
// produces. The brand suffix (" | Ads Vance") is appended later by the
// orchestrator's validateScript, not requested in the prompt.
type GeneratedMetadata struct {
	YoutubeTitle       string   `json:"youtube_title"`
	YoutubeDescription string   `json:"youtube_description"`
	YoutubeTags        []string `json:"youtube_tags"`
}

// MetadataAgent generates Thai search-intent YouTube metadata from a finished
// script. Runs on Gemini Flash (cfg.Model is gemini-3-5-flash, routed by prefix).
type MetadataAgent struct {
	llm *KieLLMClient
}

func NewMetadataAgent(llm *KieLLMClient) *MetadataAgent {
	return &MetadataAgent{llm: llm}
}

// Generate produces title/description/tags for one clip. cfg is the `metadata`
// AgentConfig (fetched by the caller via GetByName).
func (a *MetadataAgent) Generate(ctx context.Context, topic, script, category, persona string, cfg *models.AgentConfig) (*GeneratedMetadata, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, MetadataTemplateData{
		Topic:           topic,
		Script:          script,
		Category:        category,
		AudiencePersona: persona,
	})
	if err != nil {
		return nil, fmt.Errorf("render metadata template: %w", err)
	}

	var md GeneratedMetadata
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &md); err != nil {
		return nil, fmt.Errorf("generate metadata: %w", err)
	}
	return &md, nil
}
