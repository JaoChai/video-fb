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

	userPrompt := fmt.Sprintf(`สร้าง voice script + ข้อมูล metadata สำหรับวิดีโอ Q&A สั้น

โครงสร้างวิดีโอ: ใช้ "ภาพเดียว" คงที่ตลอดทั้งคลิป + "เสียงพากย์เดียว" เล่าจบในตัว (ไม่มีการตัดฉาก ไม่มี multi-scene)

คำถาม: "%s"
ถามโดย: %s
หมวด: %s

ข้อมูลอ้างอิง:
%s

ตอบเป็น JSON object มี:
- "scenes": array ที่มี object **เพียง 1 ตัวเท่านั้น** (วิดีโอนี้ออกแบบเป็น single-scene):
  - "scene_number": 1
  - "scene_type": "main"
  - "text_content": ข้อความสั้นสำหรับแสดงบนภาพ (เน้นคำถาม)
  - "voice_text": บทพากย์ภาษาไทยแบบธรรมชาติ ไหลลื่นเป็นเรื่องเล่าเดียว ลำดับ: เกริ่นคำถาม → อธิบายคำตอบเป็นขั้นตอน → ปิดด้วย CTA
  - "duration_seconds": 30-55 (ให้พอดี YouTube Shorts)
  - "text_overlays": []
- "total_duration_seconds": 30-55

**กฎสำคัญสำหรับ voice_text** (ป้องกันเสียงตัด/อ่านผิด):
- **ห้ามมีอักขระ "@" และห้ามมี URL ใดๆ** ใน voice_text เด็ดขาด (TTS อ่านลิงก์ไม่ออก เสียงจะตัด)
- เรียกชื่อแบรนด์ว่า "**แอดส์แวนซ์**" สะกดเป็นเสียงไทย (ห้ามเขียน "Adsvance", "@adsvance", "Ads Vance" ใน voice_text)
- CTA ปิดท้ายให้พูดทำนองนี้: "ติดต่อทีมงานแอดส์แวนซ์ทางไลน์ ไอดีแอดส์แวนซ์ หรือเข้ากลุ่มเทเลแกรมแอดส์แวนซ์ได้เลยครับ"
- ใช้ "..." สำหรับจังหวะหายใจระหว่างประโยค

- "youtube_title": ดึงดูด สั้น ลงท้ายด้วย {Ads Vance} ไม่เกิน 70 ตัวอักษร
- "youtube_description": ต้องมีแค่ 2 บรรทัดนี้เท่านั้น ห้ามเพิ่มเนื้อหาอื่น (URL/handle อยู่ตรงนี้ได้):
  "ติดต่อทีมงาน line id : @adsvance\n\nเข้ากลุ่มเทเรแกรมเพื่อรับข่าวสาร : https://t.me/adsvancech"
- "youtube_tags": array tags ไทย+อังกฤษ

ห้ามแนะนำการทำผิดนโยบาย Facebook`, question, questionerName, category, ragContext.String())

	var script GeneratedScript
	if err := a.llm.GenerateJSON(ctx, model, systemPrompt, userPrompt, temperature, &script); err != nil {
		return nil, fmt.Errorf("generate script: %w", err)
	}
	return &script, nil
}
