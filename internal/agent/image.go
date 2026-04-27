package agent

import (
	"context"
	"fmt"

	"github.com/jaochai/video-fb/internal/models"
)

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

func (a *ImageAgent) GeneratePrompts(ctx context.Context, scenes []GeneratedScene, theme *models.BrandTheme, questionerName, model, systemPrompt string, temperature float64) ([]SceneImagePrompts, error) {
	themeDesc := fmt.Sprintf(
		"Brand: primary=%s, secondary=%s, accent=%s, font=%s. Style: %s",
		theme.PrimaryColor, theme.SecondaryColor, theme.AccentColor, theme.FontName,
		safeStr(theme.ImageStyle))

	var sceneDescs string
	for _, s := range scenes {
		sceneDescs += fmt.Sprintf("Scene %d (%s): %s\n", s.SceneNumber, s.SceneType, s.TextContent)
	}

	userPrompt := fmt.Sprintf(`สร้าง image prompts สำหรับวิดีโอ Facebook Ads Q&A

Brand Theme: %s
คนถาม: %s

Scenes:
%s

ตอบเป็น JSON array ของ objects ที่มี:
- "scene_number": int
- "image_prompt_16_9": prompt ภาษาอังกฤษ สำหรับ 16:9 landscape. ใส่ Thai text content บนภาพ. ใช้สี brand. Scene type "question" ให้เป็น chat bubble style. Scene type "step" ให้เป็น infographic. Scene type "summary" ให้เป็น CTA card.
- "image_prompt_9_16": prompt เหมือนกันแต่สำหรับ 9:16 vertical format.

DO NOT include any logo, mascot, brand name, or brand text in the image.
ทุก prompt ต้องมี: dark gradient background (%s to darker), accent color %s, modern flat design.`, themeDesc, questionerName, sceneDescs, theme.PrimaryColor, theme.AccentColor)

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
