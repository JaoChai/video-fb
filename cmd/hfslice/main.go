// Command hfslice is a vertical-slice runner for the Hyperframes pipeline.
// It pulls a real clip from the DB, lets the composition agent design the video,
// builds a Hyperframes project, and renders it to MP4 — proving the whole flow
// end-to-end before wiring it into the production producer.
//
// Usage: go run ./cmd/hfslice -clip <clipID>
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/config"
	"github.com/jaochai/video-fb/internal/database"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
)

// The composition agent's brain. In production these live in agent_configs +
// design skills; hardcoded here so the slice runs without a migration.
const compSystemPrompt = `คุณคือนักออกแบบวิดีโอสั้นแนวตั้ง (9:16) สำหรับช่อง "ADS VANCE" ที่ตอบคำถาม Facebook Ads ให้เจ้าของธุรกิจไทย
หน้าที่: ออกแบบ "หน้าตา" ของวิดีโอ Dynamic Karaoke จากบทพากย์ที่ให้มา
ตอบกลับเป็น JSON เท่านั้น ไม่มีคำอธิบายอื่น`

const compSkills = `แนวทางออกแบบ (design skills):
- accent_color เลือกตามอารมณ์เนื้อหา: ปัญหา/เตือน=#ff5a52, เทคนิค/ทั่วไป=#ff6b2b, อัปเดต/ข่าว=#3b82f6
- secondary_accent = สีการ์ดผลลัพธ์เชิงบวก ใช้ #2fd17a เสมอ
- highlight_words: เลือกคำสำคัญในหัวข้อ 1-2 คำ (คำที่สะดุดตาสุด)
- kicker: ป้ายหมวดสั้นๆ ตัวพิมพ์ใหญ่ เช่น "PIXEL & CAPI"
- cards: สร้าง 3-5 ใบไล่ประเด็นตามบทพากย์ — type "cause"(สาเหตุ/ปัญหา), "step"(ขั้นตอนแก้ ใส่ step 1,2,3), "win"(ผลลัพธ์)
  - แต่ละใบ start/end ต้อง sync กับช่วงเวลาในบทพากย์ที่พูดถึงเรื่องนั้น (ใช้ timestamp ที่ให้มา)
  - body สั้น ≤15 คำ
  - การ์ดห้ามเวลาทับกัน
- animation_speed: เนื้อหาเร่งด่วน=fast, เทคนิค=normal, เรื่องเล่า=slow`

const compPromptTemplate = `หัวข้อ: {{.Question}}
หมวด: {{.Category}}
ผู้ถาม: {{.QuestionerName}}
ความยาว: {{.DurationSeconds}} วินาที

บทพากย์เต็ม:
{{.VoiceText}}

ช่วงเวลาคำพูด (วินาที) สำหรับ sync การ์ด:
{{.SegmentsContext}}

ออกแบบวิดีโอ ตอบ JSON:
{
  "accent_color": "#rrggbb",
  "secondary_accent": "#2fd17a",
  "animation_speed": "fast|normal|slow",
  "kicker": "ป้ายหมวดสั้น",
  "highlight_words": ["คำ1","คำ2"],
  "cards": [
    {"type":"cause","start":13.7,"end":24.6,"kicker":"สาเหตุ","body":"...","step":0},
    {"type":"step","start":27.7,"end":35.2,"kicker":"วิธีแก้","body":"...","step":1}
  ]
}`

func main() {
	clipID := flag.String("clip", "9d318572-fc10-4dae-9245-72ab1e2e34ff", "clip UUID to render")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	// 1. Pull the real clip + scene.
	var title, category, questioner, textContent, voiceText string
	var dbDuration float64
	err = pool.QueryRow(ctx,
		`SELECT c.title, c.category, c.questioner_name, s.text_content, s.voice_text, s.duration_seconds
		 FROM clips c JOIN scenes s ON s.clip_id = c.id WHERE c.id = $1`, *clipID).
		Scan(&title, &category, &questioner, &textContent, &voiceText, &dbDuration)
	if err != nil {
		log.Fatalf("query clip %s: %v", *clipID, err)
	}
	log.Printf("clip: %s | %s | %s", textContent, category, questioner)

	// 2. Load existing Thai transcript (from local whisper large-v3; production
	//    will call the transcriber). Duration comes from the audio, not the DB.
	pocAssets := "hyperframes-poc/poc-video/assets"
	segData, err := os.ReadFile(filepath.Join(pocAssets, "transcript.json"))
	if err != nil {
		log.Fatalf("read transcript: %v", err)
	}
	var segments []producer.TranscriptSegment
	if err := json.Unmarshal(segData, &segments); err != nil {
		log.Fatalf("parse transcript: %v", err)
	}
	duration := dbDuration
	if n := len(segments); n > 0 && segments[n-1].End > duration {
		duration = segments[n-1].End
	}
	log.Printf("transcript: %d segments, duration %.1fs", len(segments), duration)

	// 3. Composition agent designs the video.
	llm := agent.NewLLMClient(pool)
	var model string
	if err := pool.QueryRow(ctx, `SELECT model FROM agent_configs WHERE agent_name='script'`).Scan(&model); err != nil {
		log.Fatalf("get script model: %v", err)
	}
	compCfg := &models.AgentConfig{
		AgentName: "composition", Model: model, Temperature: 0.7,
		SystemPrompt: compSystemPrompt, PromptTemplate: compPromptTemplate, Skills: compSkills,
	}
	decision, err := agent.NewCompositionAgent(llm).Decide(ctx, agent.CompositionTemplateData{
		Question: textContent, VoiceText: voiceText, Category: category,
		QuestionerName: questioner, FormatName: "qa", DurationSeconds: duration,
		SegmentsContext: segmentsContext(segments),
	}, compCfg)
	if err != nil {
		log.Fatalf("composition decide: %v", err)
	}
	log.Printf("design: accent=%s speed=%s highlights=%v cards=%d",
		decision.AccentColor, decision.AnimationSpeed, decision.HighlightWords, len(decision.Cards))

	// 4. Map the agent's decision → render params.
	params := producer.CompositionParams{
		Title:           textContent,
		HighlightWords:  decision.HighlightWords,
		Kicker:          decision.Kicker,
		BrandName:       "ADS VANCE",
		CategoryLabel:   strings.ToUpper(category),
		QuestionerName:  questioner,
		LayoutVariant:   "dynamic_karaoke",
		AccentColor:     decision.AccentColor,
		SecondaryAccent: decision.SecondaryAccent,
		AnimationSpeed:  decision.AnimationSpeed,
		BackgroundMode:  "css",
		VoiceSrc:        "assets/voice.wav",
		DurationSeconds: duration,
		Segments:        segments,
		Cards:           mapCards(decision.Cards),
	}

	// 5. Build the Hyperframes project.
	projectDir, err := filepath.Abs(filepath.Join("/tmp/hfslice", *clipID))
	if err != nil {
		log.Fatalf("abs path: %v", err)
	}
	_ = os.RemoveAll(projectDir)
	builder := producer.NewCompositionBuilder("internal/producer/templates", filepath.Join(pocAssets, "fonts"))
	if _, err := builder.Build(params, projectDir, filepath.Join(pocAssets, "voice.wav"), ""); err != nil {
		log.Fatalf("build composition: %v", err)
	}
	log.Printf("built project: %s", projectDir)

	// 6. Lint guardrail, then render.
	renderer := producer.NewHyperframesRenderer()
	if err := renderer.Lint(ctx, projectDir); err != nil {
		log.Fatalf("lint failed: %v", err)
	}
	log.Println("lint: passed")
	if err := renderer.Render(ctx, projectDir, "output.mp4"); err != nil {
		log.Fatalf("render failed: %v", err)
	}
	log.Printf("DONE → %s", filepath.Join(projectDir, "output.mp4"))
}

func segmentsContext(segs []producer.TranscriptSegment) string {
	var b strings.Builder
	for _, s := range segs {
		fmt.Fprintf(&b, "[%.1f-%.1f] %s\n", s.Start, s.End, s.Text)
	}
	return b.String()
}

func mapCards(cards []agent.CompositionCard) []producer.CardSpec {
	out := make([]producer.CardSpec, 0, len(cards))
	for i, c := range cards {
		out = append(out, producer.CardSpec{
			ID:       fmt.Sprintf("card%d", i+1),
			Type:     c.Type,
			StartSec: c.StartSec,
			EndSec:   c.EndSec,
			Kicker:   c.Kicker,
			Body:     c.Body,
			StepNum:  c.StepNum,
		})
	}
	return out
}
