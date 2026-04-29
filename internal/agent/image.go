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
	llm *LLMClient
}

func NewImageAgent(llm *LLMClient) *ImageAgent {
	return &ImageAgent{llm: llm}
}

type SceneImagePrompts struct {
	SceneNumber    int    `json:"scene_number"`
	ImagePrompt169 string `json:"image_prompt_16_9"`
	ImagePrompt916 string `json:"image_prompt_9_16"`
}

func (a *ImageAgent) GeneratePrompts(ctx context.Context, scenes []GeneratedScene, theme *models.BrandTheme, questionerName, model, systemPrompt string, temperature float64, promptTemplate string) ([]SceneImagePrompts, error) {
	themeDesc := fmt.Sprintf(
		"Brand: primary=%s, secondary=%s, accent=%s, font=%s. Style: %s",
		theme.PrimaryColor, theme.SecondaryColor, theme.AccentColor, theme.FontName,
		safeStr(theme.ImageStyle))

	var questionText string
	if len(scenes) > 0 {
		questionText = scenes[0].TextContent
	}

	userPrompt, err := renderTemplate(promptTemplate, ImageTemplateData{
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
	if err := a.llm.GenerateJSON(ctx, model, systemPrompt, userPrompt, temperature, &prompts); err != nil {
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
