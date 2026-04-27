package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jaochai/video-fb/internal/rag"
)

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

func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category, model, systemPrompt string, temperature float64) (*GeneratedScript, error) {
	ragResults, err := a.rag.Search(ctx, question, 5)
	if err != nil {
		return nil, fmt.Errorf("RAG search: %w", err)
	}

	var ragContext strings.Builder
	for _, r := range ragResults {
		ragContext.WriteString(r.Content)
		ragContext.WriteString("\n---\n")
	}

	userPrompt := fmt.Sprintf(`สร้าง voice script สำหรับวิดีโอ Q&A (ภาพคำถามแสดงตลอดทั้งคลิป เสียงพากย์อธิบายคำตอบ)

คำถาม: "%s"
ถามโดย: %s
หมวด: %s

ข้อมูลอ้างอิง:
%s

ตอบเป็น JSON object มี:
- "scenes": array ของ scene objects:
  - scene 1: type "question" — เปิดด้วยคำถาม
  - scene 2-4: type "step" — อธิบายขั้นตอนแก้ปัญหา
  - scene 5: type "summary" — สรุป + CTA ติดต่อทีมงาน @adsvance
- แต่ละ scene มี: scene_number, scene_type, text_content (เท่ากับ voice_text), voice_text (ใช้ ... สำหรับจังหวะพัก), duration_seconds, text_overlays (array ว่าง [])
- "total_duration_seconds": รวม 30-55 วินาที (ห้ามเกิน 55 วินาที เพื่อให้เป็น YouTube Shorts ได้)
- "youtube_title": ดึงดูด สั้น ลงท้ายด้วย {Ads Vance} ไม่เกิน 70 ตัวอักษร
- "youtube_description": ต้องมีแค่ 2 บรรทัดนี้เท่านั้น ห้ามเพิ่มเนื้อหาอื่น:
  "ติดต่อทีมงาน line id : @adsvance\n\nเข้ากลุ่มเทเรแกรมเพื่อรับข่าวสาร : https://t.me/adsvancech"
- "youtube_tags": array tags ไทย+อังกฤษ

ห้ามแนะนำการทำผิดนโยบาย Facebook`, question, questionerName, category, ragContext.String())

	var script GeneratedScript
	if err := a.llm.GenerateJSON(ctx, model, systemPrompt, userPrompt, temperature, &script); err != nil {
		return nil, fmt.Errorf("generate script: %w", err)
	}
	return &script, nil
}
