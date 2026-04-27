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

	var questionText string
	for _, s := range scenes {
		if s.SceneType == "question" {
			questionText = s.TextContent
			break
		}
	}
	if questionText == "" && len(scenes) > 0 {
		questionText = scenes[0].TextContent
	}

	userPrompt := fmt.Sprintf(`สร้าง image prompt 1 ภาพ สำหรับวิดีโอ Facebook Ads Q&A

Brand Theme: %s
คนถาม: %s
คำถาม: %s

สร้างภาพสไตล์ chat bubble / Facebook-like UI แสดงคำถามเด่นชัด พร้อม icon คำถาม
ภาพนี้จะใช้เป็นพื้นหลังตลอดทั้งคลิปในขณะที่เสียงพากย์อธิบายคำตอบ

ตอบเป็น JSON array ที่มี object เดียว:
- "scene_number": 1
- "image_prompt_16_9": prompt ภาษาอังกฤษ สำหรับ 16:9 landscape. ใส่ Thai text คำถามบนภาพ.
- "image_prompt_9_16": prompt เหมือนกันแต่สำหรับ 9:16 vertical format.

DO NOT include any logo, mascot, brand name, or brand text in the image.
ภาพต้องมี: dark gradient background (%s to darker), accent color %s, modern flat design, chat bubble with question text.`, themeDesc, questionerName, questionText, theme.PrimaryColor, theme.AccentColor)

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
