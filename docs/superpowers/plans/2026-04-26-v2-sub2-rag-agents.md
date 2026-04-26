# Sub-Project 2: RAG + Multi-Agent Pipeline — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the autonomous content production pipeline — knowledge crawler, RAG engine, 4 AI agents (question, script, image, analytics), video producer (Kie.ai + FFmpeg), and orchestrator — so the system generates Q&A videos end-to-end without human intervention.

**Architecture:** Each agent is a Go service that calls Claude API with RAG context. The orchestrator runs weekly, invoking agents in sequence: Question → Script → Image → Video Producer. The knowledge crawler runs separately to keep the RAG store fresh. Video producer calls Kie.ai for images/voice then FFmpeg for assembly into 16:9 and 9:16 formats.

**Tech Stack:** Go 1.25, Claude API (HTTP), Kie.ai REST API, FFmpeg (subprocess), pgvector (Neon), existing chi router + pgx repos.

---

## New Files

```
internal/
├── config/
│   └── config.go              # MODIFY: add Claude/Kie.ai/embedding API keys
├── crawler/
│   └── crawler.go             # Web crawler + text chunker
├── rag/
│   └── rag.go                 # Embedding generation + vector search
├── agent/
│   ├── claude.go              # Claude API client (shared by all agents)
│   ├── question.go            # Question Agent
│   ├── script.go              # Script Agent
│   ├── image.go               # Image Agent (generates prompts, not images)
│   └── analytics.go           # Analytics Agent
├── producer/
│   ├── kieai.go               # Kie.ai API client (image gen + voice gen)
│   ├── ffmpeg.go              # FFmpeg video assembly
│   └── producer.go            # Orchestrates: images → voice → assembly
├── orchestrator/
│   └── orchestrator.go        # Weekly pipeline: agents → producer → save
└── handler/
    └── orchestrator.go        # HTTP trigger endpoints for manual runs
```

---

## Task 1: Expand Config for External APIs

**Files:**
- Modify: `internal/config/config.go`
- Modify: `.env.example`

- [ ] **Step 1: Update config.go**

```go
package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	Port           string
	APIKey         string
	ClaudeAPIKey   string
	KieAPIKey      string
	ElevenLabsVoice string
	FFmpegPath     string
}

func Load() *Config {
	godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ffmpeg := os.Getenv("FFMPEG_PATH")
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}

	voice := os.Getenv("ELEVENLABS_VOICE")
	if voice == "" {
		voice = "Adam"
	}

	return &Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		Port:           port,
		APIKey:         os.Getenv("API_KEY"),
		ClaudeAPIKey:   os.Getenv("CLAUDE_API_KEY"),
		KieAPIKey:      os.Getenv("KIE_API_KEY"),
		ElevenLabsVoice: voice,
		FFmpegPath:     ffmpeg,
	}
}
```

- [ ] **Step 2: Update .env.example**

```env
DATABASE_URL=postgresql://user:pass@host/dbname?sslmode=require
PORT=8080
API_KEY=your-api-key-here
CLAUDE_API_KEY=sk-ant-xxx
KIE_API_KEY=kie-xxx
ELEVENLABS_VOICE=Adam
FFMPEG_PATH=ffmpeg
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go .env.example
git commit -m "feat: expand config with Claude, Kie.ai, and FFmpeg settings"
```

---

## Task 2: Claude API Client

**Files:**
- Create: `internal/agent/claude.go`

- [ ] **Step 1: Create internal/agent/claude.go**

```go
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const claudeAPI = "https://api.anthropic.com/v1/messages"

type ClaudeClient struct {
	apiKey string
	model  string
	client *http.Client
}

func NewClaudeClient(apiKey, model string) *ClaudeClient {
	return &ClaudeClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

type claudeRequest struct {
	Model       string           `json:"model"`
	MaxTokens   int              `json:"max_tokens"`
	System      string           `json:"system,omitempty"`
	Messages    []claudeMessage  `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *ClaudeClient) Generate(ctx context.Context, systemPrompt, userPrompt string, temperature float64) (string, error) {
	reqBody := claudeRequest{
		Model:       c.model,
		MaxTokens:   8000,
		System:      systemPrompt,
		Temperature: temperature,
		Messages: []claudeMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", claudeAPI, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result claudeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("claude error: %s", result.Error.Message)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}

	return result.Content[0].Text, nil
}

func (c *ClaudeClient) GenerateJSON(ctx context.Context, systemPrompt, userPrompt string, temperature float64, target any) error {
	text, err := c.Generate(ctx, systemPrompt, userPrompt, temperature)
	if err != nil {
		return err
	}

	cleaned := text
	if idx := bytes.IndexByte([]byte(cleaned), '['); idx > 0 {
		cleaned = cleaned[idx:]
	} else if idx := bytes.IndexByte([]byte(cleaned), '{'); idx > 0 {
		cleaned = cleaned[idx:]
	}
	if last := bytes.LastIndexByte([]byte(cleaned), ']'); last >= 0 {
		cleaned = cleaned[:last+1]
	} else if last := bytes.LastIndexByte([]byte(cleaned), '}'); last >= 0 {
		cleaned = cleaned[:last+1]
	}

	if err := json.Unmarshal([]byte(cleaned), target); err != nil {
		return fmt.Errorf("parse JSON from Claude: %w\nraw: %s", err, text[:min(len(text), 200)])
	}
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/agent/claude.go
git commit -m "feat: Claude API client with JSON extraction"
```

---

## Task 3: RAG Engine (Embeddings + Vector Search)

**Files:**
- Create: `internal/rag/rag.go`

- [ ] **Step 1: Create internal/rag/rag.go**

```go
package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Engine struct {
	pool     *pgxpool.Pool
	apiKey   string
	client   *http.Client
}

func NewEngine(pool *pgxpool.Pool, claudeAPIKey string) *Engine {
	return &Engine{
		pool:   pool,
		apiKey: claudeAPIKey,
		client: &http.Client{},
	}
}

type voyageRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type voyageResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (e *Engine) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	reqBody := voyageRequest{
		Input: []string{text},
		Model: "voyage-3",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.voyageai.com/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send embedding request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	var result voyageResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("embedding error: %s", result.Error.Message)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Data[0].Embedding, nil
}

func (e *Engine) StoreChunk(ctx context.Context, sourceID, content, url string, embedding []float64) error {
	embStr := formatVector(embedding)
	_, err := e.pool.Exec(ctx,
		`INSERT INTO knowledge_chunks (source_id, content, url, embedding)
		 VALUES ($1, $2, $3, $4::vector)`,
		sourceID, content, url, embStr)
	if err != nil {
		return fmt.Errorf("store chunk: %w", err)
	}
	return nil
}

type SearchResult struct {
	Content    string  `json:"content"`
	URL        string  `json:"url"`
	Similarity float64 `json:"similarity"`
}

func (e *Engine) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	embedding, err := e.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	embStr := formatVector(embedding)
	rows, err := e.pool.Query(ctx,
		`SELECT content, COALESCE(url, ''), 1 - (embedding <=> $1::vector) AS similarity
		 FROM knowledge_chunks
		 ORDER BY embedding <=> $1::vector
		 LIMIT $2`,
		embStr, topK)
	if err != nil {
		return nil, fmt.Errorf("search chunks: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Content, &r.URL, &r.Similarity); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		results = append(results, r)
	}
	return results, nil
}

func formatVector(v []float64) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%f", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func ChunkText(text string, maxChunkSize int, overlap int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []string
	start := 0
	for start < len(words) {
		end := start + maxChunkSize
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[start:end], " ")
		chunks = append(chunks, chunk)
		start = end - overlap
		if start >= end {
			break
		}
	}
	return chunks
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/rag/rag.go
git commit -m "feat: RAG engine with Voyage embeddings and pgvector search"
```

---

## Task 4: Knowledge Crawler

**Files:**
- Create: `internal/crawler/crawler.go`

- [ ] **Step 1: Create internal/crawler/crawler.go**

```go
package crawler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/rag"
)

type Crawler struct {
	pool   *pgxpool.Pool
	engine *rag.Engine
	client *http.Client
}

func NewCrawler(pool *pgxpool.Pool, engine *rag.Engine) *Crawler {
	return &Crawler{
		pool:   pool,
		engine: engine,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Crawler) CrawlAll(ctx context.Context) error {
	rows, err := c.pool.Query(ctx,
		`SELECT id, name, url, source_type FROM knowledge_sources WHERE enabled = TRUE`)
	if err != nil {
		return fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	type source struct {
		ID, Name, URL, Type string
	}
	var sources []source
	for rows.Next() {
		var s source
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.Type); err != nil {
			return fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, s)
	}

	for _, s := range sources {
		log.Printf("Crawling: %s (%s)", s.Name, s.URL)
		if err := c.crawlSource(ctx, s.ID, s.URL); err != nil {
			log.Printf("Failed to crawl %s: %v", s.Name, err)
			continue
		}
		c.pool.Exec(ctx,
			`UPDATE knowledge_sources SET last_crawled_at = NOW() WHERE id = $1`, s.ID)
		log.Printf("Crawled: %s", s.Name)
	}
	return nil
}

func (c *Crawler) crawlSource(ctx context.Context, sourceID, url string) error {
	readerURL := "https://r.jina.ai/" + url
	req, err := http.NewRequestWithContext(ctx, "GET", readerURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "adsvance-crawler/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	text := string(body)
	text = cleanText(text)

	if len(strings.Fields(text)) < 50 {
		return fmt.Errorf("content too short from %s", url)
	}

	chunks := rag.ChunkText(text, 300, 50)
	stored := 0
	for _, chunk := range chunks {
		if len(strings.Fields(chunk)) < 20 {
			continue
		}
		embedding, err := c.engine.GenerateEmbedding(ctx, chunk)
		if err != nil {
			log.Printf("Embedding failed for chunk: %v", err)
			continue
		}
		if err := c.engine.StoreChunk(ctx, sourceID, chunk, url, embedding); err != nil {
			log.Printf("Store failed: %v", err)
			continue
		}
		stored++
	}
	log.Printf("Stored %d/%d chunks from %s", stored, len(chunks), url)
	return nil
}

func cleanText(text string) string {
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "![") || strings.HasPrefix(line, "<") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/crawler/crawler.go
git commit -m "feat: knowledge crawler with Jina Reader and chunking"
```

---

## Task 5: Question Agent

**Files:**
- Create: `internal/agent/question.go`

- [ ] **Step 1: Create internal/agent/question.go**

```go
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/rag"
)

type QuestionAgent struct {
	claude *ClaudeClient
	rag    *rag.Engine
	pool   *pgxpool.Pool
}

func NewQuestionAgent(claude *ClaudeClient, ragEngine *rag.Engine, pool *pgxpool.Pool) *QuestionAgent {
	return &QuestionAgent{claude: claude, rag: ragEngine, pool: pool}
}

type GeneratedQuestion struct {
	Question       string `json:"question"`
	QuestionerName string `json:"questioner_name"`
	Category       string `json:"category"`
	PainPoint      string `json:"pain_point"`
}

func (a *QuestionAgent) Generate(ctx context.Context, count int, category string, systemPrompt string) ([]GeneratedQuestion, error) {
	ragResults, err := a.rag.Search(ctx, fmt.Sprintf("Facebook Ads %s problems common issues", category), 5)
	if err != nil {
		return nil, fmt.Errorf("RAG search: %w", err)
	}

	var ragContext strings.Builder
	for _, r := range ragResults {
		ragContext.WriteString(r.Content)
		ragContext.WriteString("\n---\n")
	}

	recentRows, err := a.pool.Query(ctx,
		`SELECT title FROM topic_history WHERE created_at > NOW() - INTERVAL '60 days' ORDER BY created_at DESC LIMIT 30`)
	if err != nil {
		return nil, fmt.Errorf("query recent topics: %w", err)
	}
	defer recentRows.Close()

	var recent []string
	for recentRows.Next() {
		var t string
		recentRows.Scan(&t)
		recent = append(recent, t)
	}

	previousList := ""
	if len(recent) > 0 {
		previousList = "\n\nห้ามซ้ำกับหัวข้อเหล่านี้:\n- " + strings.Join(recent, "\n- ")
	}

	userPrompt := fmt.Sprintf(`สร้าง %d คำถามจากลูกค้าเกี่ยวกับ Facebook Ads หมวด "%s"

ข้อมูลอ้างอิงจาก knowledge base:
%s
%s

ตอบเป็น JSON array เท่านั้น แต่ละ object มี:
- "question": คำถามภาษาไทย สั้น กระชับ เหมือนลูกค้าถามจริง
- "questioner_name": ชื่อไทย เช่น "คุณ สมชาย" "คุณ มานี"
- "category": "%s"
- "pain_point": ปัญหาหลักเป็นภาษาอังกฤษ เช่น "account_banned" "payment_failed"

ห้ามสร้างคำถามที่แนะนำการทำผิดนโยบาย Facebook`, count, category, ragContext.String(), previousList, category)

	var questions []GeneratedQuestion
	if err := a.claude.GenerateJSON(ctx, systemPrompt, userPrompt, 0.8, &questions); err != nil {
		return nil, fmt.Errorf("generate questions: %w", err)
	}

	for _, q := range questions {
		a.pool.Exec(ctx,
			`INSERT INTO topic_history (title, category) VALUES ($1, $2)`,
			q.Question, q.Category)
	}

	return questions, nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/agent/question.go
git commit -m "feat: question agent with RAG context and topic dedup"
```

---

## Task 6: Script Agent

**Files:**
- Create: `internal/agent/script.go`

- [ ] **Step 1: Create internal/agent/script.go**

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jaochai/video-fb/internal/rag"
)

type ScriptAgent struct {
	claude *ClaudeClient
	rag    *rag.Engine
}

func NewScriptAgent(claude *ClaudeClient, ragEngine *rag.Engine) *ScriptAgent {
	return &ScriptAgent{claude: claude, rag: ragEngine}
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

func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category, systemPrompt string) (*GeneratedScript, error) {
	ragResults, err := a.rag.Search(ctx, question, 5)
	if err != nil {
		return nil, fmt.Errorf("RAG search: %w", err)
	}

	var ragContext strings.Builder
	for _, r := range ragResults {
		ragContext.WriteString(r.Content)
		ragContext.WriteString("\n---\n")
	}

	userPrompt := fmt.Sprintf(`สร้าง script วิดีโอ Q&A สำหรับคำถามนี้:

คำถาม: "%s"
ถามโดย: %s
หมวด: %s

ข้อมูลอ้างอิง:
%s

ตอบเป็น JSON object มี:
- "scenes": array ของ scene objects (5 scenes):
  - scene 1: type "question" — แสดงคำถาม (8 วินาที)
  - scene 2-4: type "step" — ขั้นตอนแก้ปัญหา (10-15 วินาทีต่อ scene)
  - scene 5: type "summary" — สรุป + CTA ติดต่อซื้อบัญชี @adsvance (8 วินาที)
- แต่ละ scene มี: scene_number, scene_type, text_content (ข้อความบนภาพ), voice_text (script เสียงพูด ใช้ ... สำหรับพัก — สำหรับเน้น), duration_seconds, text_overlays (array ว่าง [])
- "total_duration_seconds": รวมทั้งหมด 30-90 วินาที
- "youtube_title": ชื่อ YouTube ดึงดูด ลงท้ายด้วย {{Ads Vance}} ไม่เกิน 70 ตัวอักษร
- "youtube_description": คำอธิบาย รวม "ติดต่อซื้อบัญชี line id : @adsvance\nเข้ากลุ่มเทเลแกรม: https://t.me/adsvancech"
- "youtube_tags": array ของ tags ไทย+อังกฤษ

ห้ามแนะนำการทำผิดนโยบาย Facebook
CTA ให้แนะนำซื้อบัญชีสำรองจาก @adsvance เป็น business continuity`, question, questionerName, category, ragContext.String())

	var script GeneratedScript
	if err := a.claude.GenerateJSON(ctx, systemPrompt, userPrompt, 0.7, &script); err != nil {
		return nil, fmt.Errorf("generate script: %w", err)
	}
	return &script, nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/agent/script.go
git commit -m "feat: script agent generates multi-scene Q&A scripts with RAG"
```

---

## Task 7: Image Prompt Agent

**Files:**
- Create: `internal/agent/image.go`

- [ ] **Step 1: Create internal/agent/image.go**

```go
package agent

import (
	"context"
	"fmt"

	"github.com/jaochai/video-fb/internal/models"
)

type ImageAgent struct {
	claude *ClaudeClient
}

func NewImageAgent(claude *ClaudeClient) *ImageAgent {
	return &ImageAgent{claude: claude}
}

type SceneImagePrompts struct {
	SceneNumber     int    `json:"scene_number"`
	ImagePrompt169  string `json:"image_prompt_16_9"`
	ImagePrompt916  string `json:"image_prompt_9_16"`
}

func (a *ImageAgent) GeneratePrompts(ctx context.Context, scenes []GeneratedScene, theme *models.BrandTheme, questionerName, systemPrompt string) ([]SceneImagePrompts, error) {
	themeDesc := fmt.Sprintf(
		"Brand: primary=%s, secondary=%s, accent=%s, font=%s. Mascot: %s. Style: %s",
		theme.PrimaryColor, theme.SecondaryColor, theme.AccentColor, theme.FontName,
		safeStr(theme.MascotDescription), safeStr(theme.ImageStyle))

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
- "image_prompt_9_16": prompt เหมือนกันแต่สำหรับ 9:16 vertical format. ปรับ layout ให้เหมาะกับแนวตั้ง.

ทุก prompt ต้องมี: dark gradient background (%s to darker), accent color %s, brand text 'Ads Vance' bottom-right, modern flat design.`, themeDesc, questionerName, sceneDescs, theme.PrimaryColor, theme.AccentColor)

	var prompts []SceneImagePrompts
	if err := a.claude.GenerateJSON(ctx, systemPrompt, userPrompt, 0.7, &prompts); err != nil {
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
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/agent/image.go
git commit -m "feat: image prompt agent generates per-scene prompts in 2 formats"
```

---

## Task 8: Kie.ai Client (Image + Voice Generation)

**Files:**
- Create: `internal/producer/kieai.go`

- [ ] **Step 1: Create internal/producer/kieai.go**

```go
package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const kieAPI = "https://api.kie.ai/api/v1"

type KieClient struct {
	apiKey string
	client *http.Client
}

func NewKieClient(apiKey string) *KieClient {
	return &KieClient{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type kieTaskRequest struct {
	Model string         `json:"model"`
	Input map[string]any `json:"input"`
}

type kieTaskResponse struct {
	Code int `json:"code"`
	Data struct {
		TaskID string `json:"taskId"`
	} `json:"data"`
}

type kieStatusResponse struct {
	Code int `json:"code"`
	Data struct {
		Status string         `json:"status"`
		Output map[string]any `json:"output"`
	} `json:"data"`
}

func (k *KieClient) GenerateImage(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	taskID, err := k.createTask(ctx, "gpt-image-2-text-to-image", map[string]any{
		"prompt":       prompt,
		"aspect_ratio": aspectRatio,
		"resolution":   "2K",
	})
	if err != nil {
		return fmt.Errorf("create image task: %w", err)
	}

	result, err := k.pollTask(ctx, taskID, 120*time.Second)
	if err != nil {
		return fmt.Errorf("poll image task: %w", err)
	}

	imageURL, ok := result["image_url"].(string)
	if !ok {
		return fmt.Errorf("no image_url in result")
	}

	return k.downloadFile(ctx, imageURL, outputPath)
}

func (k *KieClient) GenerateVoice(ctx context.Context, text, voice, outputPath string) error {
	taskID, err := k.createTask(ctx, "elevenlabs/text-to-dialogue-v3", map[string]any{
		"dialogue":      []map[string]string{{"text": text, "voice": voice}},
		"language_code": "th",
		"stability":     0.5,
	})
	if err != nil {
		return fmt.Errorf("create voice task: %w", err)
	}

	result, err := k.pollTask(ctx, taskID, 120*time.Second)
	if err != nil {
		return fmt.Errorf("poll voice task: %w", err)
	}

	audioURL, ok := result["audio_url"].(string)
	if !ok {
		return fmt.Errorf("no audio_url in result")
	}

	return k.downloadFile(ctx, audioURL, outputPath)
}

func (k *KieClient) createTask(ctx context.Context, model string, input map[string]any) (string, error) {
	reqBody := kieTaskRequest{Model: model, Input: input}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", kieAPI+"/jobs/createTask", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.apiKey)

	resp, err := k.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result kieTaskResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Data.TaskID == "" {
		return "", fmt.Errorf("no taskId returned")
	}
	return result.Data.TaskID, nil
}

func (k *KieClient) pollTask(ctx context.Context, taskID string, timeout time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/jobs/getTaskDetail?taskId=%s", kieAPI, taskID), nil)
		req.Header.Set("Authorization", "Bearer "+k.apiKey)

		resp, err := k.client.Do(req)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		var result kieStatusResponse
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		switch result.Data.Status {
		case "completed", "success":
			return result.Data.Output, nil
		case "failed", "error":
			return nil, fmt.Errorf("task failed: %v", result.Data.Output)
		}
		time.Sleep(3 * time.Second)
	}
	return nil, fmt.Errorf("task %s timed out after %v", taskID, timeout)
}

func (k *KieClient) downloadFile(ctx context.Context, url, outputPath string) error {
	dir := filepath.Dir(outputPath)
	os.MkdirAll(dir, 0755)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := k.client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/producer/kieai.go
git commit -m "feat: Kie.ai client for GPT Image 2 and ElevenLabs V3"
```

---

## Task 9: FFmpeg Video Assembler

**Files:**
- Create: `internal/producer/ffmpeg.go`

- [ ] **Step 1: Create internal/producer/ffmpeg.go**

```go
package producer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FFmpegAssembler struct {
	ffmpegPath string
	fontPath   string
}

func NewFFmpegAssembler(ffmpegPath, fontPath string) *FFmpegAssembler {
	return &FFmpegAssembler{ffmpegPath: ffmpegPath, fontPath: fontPath}
}

type AssemblyScene struct {
	ImagePath       string
	DurationSeconds float64
	TextOverlay     string
}

func (f *FFmpegAssembler) Assemble(scenes []AssemblyScene, audioPath, outputPath string) error {
	dir := filepath.Dir(outputPath)
	os.MkdirAll(dir, 0755)

	concatFile := outputPath + ".concat.txt"
	defer os.Remove(concatFile)

	var concat strings.Builder
	var inputs []string
	var filterParts []string

	for i, scene := range scenes {
		inputs = append(inputs, "-loop", "1", "-t", fmt.Sprintf("%.1f", scene.DurationSeconds), "-i", scene.ImagePath)

		scale := fmt.Sprintf("[%d:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1[v%d]", i, i)
		filterParts = append(filterParts, scale)
		concat.WriteString(fmt.Sprintf("[v%d]", i))
	}

	audioIdx := len(scenes)
	inputs = append(inputs, "-i", audioPath)

	filter := strings.Join(filterParts, ";") + ";" +
		concat.String() + fmt.Sprintf("concat=n=%d:v=1:a=0[vout]", len(scenes))

	args := append(inputs,
		"-filter_complex", filter,
		"-map", "[vout]",
		"-map", fmt.Sprintf("%d:a", audioIdx),
		"-c:v", "libx264", "-preset", "medium", "-crf", "23",
		"-c:a", "aac", "-b:a", "128k",
		"-pix_fmt", "yuv420p",
		"-shortest",
		"-y", outputPath,
	)

	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/producer/ffmpeg.go
git commit -m "feat: FFmpeg assembler concatenates scene images with audio"
```

---

## Task 10: Video Producer (ties images + voice + assembly)

**Files:**
- Create: `internal/producer/producer.go`

- [ ] **Step 1: Create internal/producer/producer.go**

```go
package producer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaochai/video-fb/internal/agent"
)

type Producer struct {
	kie     *KieClient
	ffmpeg  *FFmpegAssembler
	voice   string
	workDir string
}

func NewProducer(kie *KieClient, ffmpeg *FFmpegAssembler, voice, workDir string) *Producer {
	os.MkdirAll(workDir, 0755)
	return &Producer{kie: kie, ffmpeg: ffmpeg, voice: voice, workDir: workDir}
}

type ProduceResult struct {
	Video169Path  string
	Video916Path  string
	ThumbnailPath string
}

func (p *Producer) Produce(ctx context.Context, clipID string, scenes []agent.GeneratedScene, imagePrompts []agent.SceneImagePrompts, voiceScript string) (*ProduceResult, error) {
	clipDir := filepath.Join(p.workDir, clipID)
	os.MkdirAll(clipDir, 0755)

	log.Printf("Generating voice for %s", clipID)
	voicePath := filepath.Join(clipDir, "voice.mp3")
	if err := p.kie.GenerateVoice(ctx, voiceScript, p.voice, voicePath); err != nil {
		return nil, fmt.Errorf("generate voice: %w", err)
	}

	var scenes169 []AssemblyScene
	var scenes916 []AssemblyScene

	for i, prompt := range imagePrompts {
		log.Printf("Generating images for scene %d of %s", prompt.SceneNumber, clipID)

		img169 := filepath.Join(clipDir, fmt.Sprintf("scene-%d-16x9.png", prompt.SceneNumber))
		if err := p.kie.GenerateImage(ctx, prompt.ImagePrompt169, "16:9", img169); err != nil {
			return nil, fmt.Errorf("generate 16:9 image scene %d: %w", prompt.SceneNumber, err)
		}

		img916 := filepath.Join(clipDir, fmt.Sprintf("scene-%d-9x16.png", prompt.SceneNumber))
		if err := p.kie.GenerateImage(ctx, prompt.ImagePrompt916, "9:16", img916); err != nil {
			return nil, fmt.Errorf("generate 9:16 image scene %d: %w", prompt.SceneNumber, err)
		}

		dur := 10.0
		if i < len(scenes) {
			dur = scenes[i].DurationSeconds
		}

		scenes169 = append(scenes169, AssemblyScene{ImagePath: img169, DurationSeconds: dur})
		scenes916 = append(scenes916, AssemblyScene{ImagePath: img916, DurationSeconds: dur})
	}

	video169 := filepath.Join(clipDir, "video-16x9.mp4")
	log.Printf("Assembling 16:9 video for %s", clipID)
	if err := p.ffmpeg.Assemble(scenes169, voicePath, video169); err != nil {
		return nil, fmt.Errorf("assemble 16:9: %w", err)
	}

	video916 := filepath.Join(clipDir, "video-9x16.mp4")
	log.Printf("Assembling 9:16 video for %s", clipID)
	if err := p.ffmpeg.Assemble(scenes916, voicePath, video916); err != nil {
		return nil, fmt.Errorf("assemble 9:16: %w", err)
	}

	thumbPath := filepath.Join(clipDir, "thumbnail.png")
	if len(imagePrompts) > 0 {
		thumbPrompt := strings.Replace(imagePrompts[0].ImagePrompt169,
			"chat bubble", "YouTube thumbnail, large bold text, eye-catching", 1)
		if err := p.kie.GenerateImage(ctx, thumbPrompt, "16:9", thumbPath); err != nil {
			log.Printf("Thumbnail generation failed: %v (using scene 1 image)", err)
			thumbPath = scenes169[0].ImagePath
		}
	}

	return &ProduceResult{
		Video169Path:  video169,
		Video916Path:  video916,
		ThumbnailPath: thumbPath,
	}, nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/producer/producer.go
git commit -m "feat: video producer orchestrates images, voice, and FFmpeg assembly"
```

---

## Task 11: Orchestrator + HTTP Trigger

**Files:**
- Create: `internal/orchestrator/orchestrator.go`
- Create: `internal/handler/orchestrator.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1: Create internal/orchestrator/orchestrator.go**

```go
package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/producer"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/jaochai/video-fb/internal/models"
)

var categories = []string{"account", "payment", "campaign", "pixel"}

type Orchestrator struct {
	pool         *pgxpool.Pool
	questionAgent *agent.QuestionAgent
	scriptAgent   *agent.ScriptAgent
	imageAgent    *agent.ImageAgent
	producer      *producer.Producer
	clipsRepo     *repository.ClipsRepo
	scenesRepo    *repository.ScenesRepo
	themesRepo    *repository.ThemesRepo
	agentsRepo    *repository.AgentsRepo
}

func New(
	pool *pgxpool.Pool,
	qa *agent.QuestionAgent,
	sa *agent.ScriptAgent,
	ia *agent.ImageAgent,
	prod *producer.Producer,
	clips *repository.ClipsRepo,
	scenes *repository.ScenesRepo,
	themes *repository.ThemesRepo,
	agents *repository.AgentsRepo,
) *Orchestrator {
	return &Orchestrator{
		pool: pool, questionAgent: qa, scriptAgent: sa, imageAgent: ia,
		producer: prod, clipsRepo: clips, scenesRepo: scenes,
		themesRepo: themes, agentsRepo: agents,
	}
}

func (o *Orchestrator) ProduceWeekly(ctx context.Context, count int) error {
	weekNum := int(time.Now().Unix() / (7 * 24 * 3600))
	category := categories[weekNum%len(categories)]
	log.Printf("Producing %d clips for category: %s", count, category)

	qaCfg, err := o.agentsRepo.GetByName(ctx, "question")
	if err != nil {
		return fmt.Errorf("get question agent config: %w", err)
	}

	questions, err := o.questionAgent.Generate(ctx, count, category, qaCfg.SystemPrompt)
	if err != nil {
		return fmt.Errorf("generate questions: %w", err)
	}
	log.Printf("Generated %d questions", len(questions))

	theme, err := o.themesRepo.GetActive(ctx)
	if err != nil {
		return fmt.Errorf("get active theme: %w", err)
	}

	scriptCfg, _ := o.agentsRepo.GetByName(ctx, "script")
	imageCfg, _ := o.agentsRepo.GetByName(ctx, "image")

	for i, q := range questions {
		log.Printf("[%d/%d] Processing: %s", i+1, len(questions), q.Question)
		if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg); err != nil {
			log.Printf("Failed to produce clip: %v", err)
			continue
		}
	}

	log.Println("Weekly production complete")
	return nil
}

func (o *Orchestrator) produceClip(ctx context.Context, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig) error {
	clip, err := o.clipsRepo.Create(ctx, models.CreateClipRequest{
		Title:          q.Question,
		Question:       q.Question,
		QuestionerName: q.QuestionerName,
		Category:       q.Category,
	})
	if err != nil {
		return fmt.Errorf("create clip: %w", err)
	}

	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status})

	script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, scriptCfg.SystemPrompt)
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("script: %w", err))
	}

	for _, scene := range script.Scenes {
		overlays := scene.TextOverlays
		if overlays == nil {
			overlays = []byte("[]")
		}
		o.scenesRepo.Create(ctx, models.CreateSceneRequest{
			ClipID:          clip.ID,
			SceneNumber:     scene.SceneNumber,
			SceneType:       scene.SceneType,
			TextContent:     scene.TextContent,
			VoiceText:       scene.VoiceText,
			DurationSeconds: scene.DurationSeconds,
			TextOverlays:    overlays,
		})
	}

	imagePrompts, err := o.imageAgent.GeneratePrompts(ctx, script.Scenes, theme, q.QuestionerName, imageCfg.SystemPrompt)
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("image prompts: %w", err))
	}

	var fullVoice string
	for _, s := range script.Scenes {
		fullVoice += s.VoiceText + " "
	}

	result, err := o.producer.Produce(ctx, clip.ID, script.Scenes, imagePrompts, fullVoice)
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("produce: %w", err))
	}

	readyStatus := "ready"
	ytTitle := script.YoutubeTitle
	ytDesc := script.YoutubeDescription
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{
		Status:      &readyStatus,
		Video169URL: &result.Video169Path,
		Video916URL: &result.Video916Path,
		ThumbnailURL: &result.ThumbnailPath,
		AnswerScript: &fullVoice,
		VoiceScript:  &fullVoice,
	})

	o.pool.Exec(ctx,
		`INSERT INTO clip_metadata (clip_id, youtube_title, youtube_description, youtube_tags)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (clip_id) DO UPDATE SET youtube_title=$2, youtube_description=$3, youtube_tags=$4`,
		clip.ID, ytTitle, ytDesc, script.YoutubeTags)

	log.Printf("Clip ready: %s — %s", clip.ID, q.Question)
	return nil
}

func (o *Orchestrator) failClip(ctx context.Context, clipID string, err error) error {
	status := "failed"
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{Status: &status})
	return err
}
```

- [ ] **Step 2: Create internal/handler/orchestrator.go**

```go
package handler

import (
	"net/http"
	"strconv"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/orchestrator"
)

type OrchestratorHandler struct {
	orch *orchestrator.Orchestrator
}

func NewOrchestratorHandler(orch *orchestrator.Orchestrator) *OrchestratorHandler {
	return &OrchestratorHandler{orch: orch}
}

func (h *OrchestratorHandler) TriggerWeekly(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count := 7
	if countStr != "" {
		if n, err := strconv.Atoi(countStr); err == nil && n > 0 {
			count = n
		}
	}

	go func() {
		if err := h.orch.ProduceWeekly(r.Context(), count); err != nil {
			writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		}
	}()

	writeJSON(w, http.StatusAccepted, models.APIResponse{
		Message: "Weekly production started in background",
	})
}
```

- [ ] **Step 3: Add route to router.go**

Add this to the end of the `New` function in `internal/router/router.go`, before the return statement:

```go
	// Orchestrator triggers (passed in via parameter)
	// Route will be registered by main.go after orchestrator is built
```

Actually, we need to modify the router to accept the orchestrator handler. Add to the end of the `New` function body, before `return r`:

```go
r.Post("/api/v1/orchestrator/produce", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"message":"use SetOrchestrator to enable"}`))
})
```

And add a method:

```go
func SetOrchestrator(r *chi.Mux, h *handler.OrchestratorHandler) {
	r.Post("/api/v1/orchestrator/produce", h.TriggerWeekly)
}
```

- [ ] **Step 4: Update cmd/server/main.go to wire orchestrator**

Add the orchestrator wiring after the router setup. Import all new packages and build the dependency graph:

```go
package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/config"
	"github.com/jaochai/video-fb/internal/crawler"
	"github.com/jaochai/video-fb/internal/database"
	"github.com/jaochai/video-fb/internal/handler"
	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/producer"
	"github.com/jaochai/video-fb/internal/rag"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/jaochai/video-fb/internal/router"
)

func main() {
	migrateFlag := flag.Bool("migrate", false, "Run database migrations")
	crawlFlag := flag.Bool("crawl", false, "Run knowledge crawler")
	produceFlag := flag.Int("produce", 0, "Produce N clips")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("Connected to database")

	if *migrateFlag {
		if err := database.RunMigrations(ctx, pool, "migrations"); err != nil {
			log.Fatalf("Migrations failed: %v", err)
		}
		log.Println("Migrations complete")
		return
	}

	ragEngine := rag.NewEngine(pool, cfg.ClaudeAPIKey)

	if *crawlFlag {
		c := crawler.NewCrawler(pool, ragEngine)
		if err := c.CrawlAll(ctx); err != nil {
			log.Fatalf("Crawl failed: %v", err)
		}
		return
	}

	claude := agent.NewClaudeClient(cfg.ClaudeAPIKey, "claude-sonnet-4-6-20250514")
	questionAgent := agent.NewQuestionAgent(claude, ragEngine, pool)
	scriptAgent := agent.NewScriptAgent(claude, ragEngine)
	imageAgent := agent.NewImageAgent(claude)

	kie := producer.NewKieClient(cfg.KieAPIKey)
	ffmpeg := producer.NewFFmpegAssembler(cfg.FFmpegPath, "/tmp/fonts/NotoSansThai-Bold.ttf")
	prod := producer.NewProducer(kie, ffmpeg, cfg.ElevenLabsVoice, "/tmp/adsvance-output")

	clipsRepo := repository.NewClipsRepo(pool)
	scenesRepo := repository.NewScenesRepo(pool)
	themesRepo := repository.NewThemesRepo(pool)
	agentsRepo := repository.NewAgentsRepo(pool)

	orch := orchestrator.New(pool, questionAgent, scriptAgent, imageAgent, prod,
		clipsRepo, scenesRepo, themesRepo, agentsRepo)

	if *produceFlag > 0 {
		if err := orch.ProduceWeekly(ctx, *produceFlag); err != nil {
			log.Fatalf("Production failed: %v", err)
		}
		return
	}

	r := router.New(pool, cfg.APIKey)
	orchHandler := handler.NewOrchestratorHandler(orch)
	router.SetOrchestrator(r, orchHandler)

	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Update internal/router/router.go — add SetOrchestrator**

Add this function at the bottom of router.go:

```go
func SetOrchestrator(r *chi.Mux, h *handler.OrchestratorHandler) {
	r.Post("/api/v1/orchestrator/produce", h.TriggerWeekly)
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./...`

- [ ] **Step 7: Commit**

```bash
git add internal/orchestrator/ internal/handler/orchestrator.go internal/router/router.go cmd/server/main.go
git commit -m "feat: orchestrator wires all agents + producer, CLI flags for crawl/produce"
```

---

## Task Summary

| Task | Component | Key Outcome |
|------|-----------|------------|
| 1 | Config expansion | Claude, Kie.ai, FFmpeg API keys |
| 2 | Claude API client | Shared HTTP client with JSON extraction |
| 3 | RAG Engine | Voyage embeddings + pgvector cosine search |
| 4 | Knowledge Crawler | Jina Reader + text chunking + embedding storage |
| 5 | Question Agent | RAG-backed question generation with dedup |
| 6 | Script Agent | Multi-scene Q&A script generation |
| 7 | Image Agent | Per-scene image prompts in 2 formats |
| 8 | Kie.ai Client | GPT Image 2 + ElevenLabs V3 integration |
| 9 | FFmpeg Assembler | Scene concatenation with audio |
| 10 | Video Producer | Orchestrates images → voice → assembly |
| 11 | Orchestrator + HTTP | Weekly pipeline + CLI flags + HTTP trigger |
