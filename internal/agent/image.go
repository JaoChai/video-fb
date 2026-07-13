package agent

import (
	"context"
	"fmt"

	"github.com/jaochai/video-fb/internal/models"
)

type ImageTemplateData struct {
	ThemeDescription string
	QuestionerName   string
	QuestionText     string
	PrimaryColor     string
	AccentColor      string
}

type ImageAgent struct {
	llm *KieLLMClient
}

func NewImageAgent(llm *KieLLMClient) *ImageAgent {
	return &ImageAgent{llm: llm}
}

type SceneImagePrompts struct {
	SceneNumber    int    `json:"scene_number"`
	ImagePrompt169 string `json:"image_prompt_16_9"`
	ImagePrompt916 string `json:"image_prompt_9_16"`
}

func (a *ImageAgent) GeneratePrompts(ctx context.Context, scenes []GeneratedScene, theme *models.BrandTheme, questionerName string, cfg *models.AgentConfig) ([]SceneImagePrompts, error) {
	themeDesc := fmt.Sprintf(
		"Brand: primary=%s, secondary=%s, accent=%s, font=%s. Style: %s",
		theme.PrimaryColor, theme.SecondaryColor, theme.AccentColor, theme.FontName,
		safeStr(theme.ImageStyle))

	var questionText string
	if len(scenes) > 0 {
		questionText = scenes[0].TextContent
	}

	userPrompt, err := renderTemplate(cfg.PromptTemplate, ImageTemplateData{
		ThemeDescription: themeDesc,
		QuestionerName:   questionerName,
		QuestionText:     questionText,
		PrimaryColor:     theme.PrimaryColor,
		AccentColor:      theme.AccentColor,
	})
	if err != nil {
		return nil, fmt.Errorf("render image template: %w", err)
	}

	var prompts []SceneImagePrompts
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &prompts); err != nil {
		return nil, fmt.Errorf("generate image prompts: %w", err)
	}
	return prompts, nil
}

func safeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// CoverImageTemplateData fills the migration-054 `image` prompt_template, which
// is dedicated to the clip's cover (frame-0) background.
type CoverImageTemplateData struct {
	QuestionText string
	Category     string
	HookText     string
}

// coverPromptOut is the JSON the cover template asks the LLM to return.
type coverPromptOut struct {
	ImagePrompt string `json:"image_prompt"`
}

// GenerateCoverPrompt produces one English image prompt for the cover scene's
// background. The render pipeline applies theme styling itself (buildScenePrompt),
// so the prompt describes objects/scene only. cfg is the `image` AgentConfig.
func (a *ImageAgent) GenerateCoverPrompt(ctx context.Context, question, category, hookText string, cfg *models.AgentConfig) (string, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, CoverImageTemplateData{
		QuestionText: question,
		Category:     category,
		HookText:     hookText,
	})
	if err != nil {
		return "", fmt.Errorf("render cover image template: %w", err)
	}
	var out coverPromptOut
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &out); err != nil {
		return "", fmt.Errorf("generate cover prompt: %w", err)
	}
	return out.ImagePrompt, nil
}
