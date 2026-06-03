# Agent Prompt Templates — Dynamic Configuration

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ย้าย user prompt templates ที่ hardcode ใน Go code ไปเป็น `prompt_template` column ใน `agent_configs` table — ให้ config ได้จาก Dashboard โดยไม่ต้อง deploy ใหม่ รวมถึงย้าย categories และ brand mapping เข้า settings

**Architecture:** เพิ่ม `prompt_template` column (TEXT) ใน `agent_configs` ที่เก็บ Go `text/template` syntax พร้อม named variables (เช่น `{{.Question}}`, `{{.Category}}`). แต่ละ agent อ่าน template จาก DB แล้ว render ด้วย `text/template.Execute()` แทนที่ `fmt.Sprintf`. Business config (categories, brand aliases) ย้ายเข้า `settings` table เป็น JSON. เพิ่ม validation สำหรับ youtube_title เป็น safety net กัน LLM สลับ field

**Tech Stack:** Go text/template, pgx/v5, React/TanStack Query, SQL migration

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `migrations/011_agent_prompt_templates.sql` | Create | Add prompt_template column + seed templates + settings |
| `internal/agent/template.go` | Create | Shared template rendering function |
| `internal/agent/question.go` | Modify | Use template from DB instead of hardcoded prompt |
| `internal/agent/script.go` | Modify | Use template from DB instead of hardcoded prompt |
| `internal/agent/image.go` | Modify | Use template from DB instead of hardcoded prompt |
| `internal/models/models.go` | Modify | Add PromptTemplate field to AgentConfig |
| `internal/repository/agents.go` | Modify | Include prompt_template in queries + Update |
| `internal/handler/agents.go` | Modify | Accept prompt_template in PATCH |
| `internal/orchestrator/orchestrator.go` | Modify | Read categories + brand mapping from settings |
| `internal/publisher/publisher.go` | Modify | Add youtube_title validation |
| `frontend/src/pages/Agents.tsx` | Modify | Add prompt_template editor with variable hints |

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/011_agent_prompt_templates.sql`

- [ ] **Step 1: Write the migration file**

```sql
-- Add prompt_template column to agent_configs
ALTER TABLE agent_configs ADD COLUMN IF NOT EXISTS prompt_template TEXT NOT NULL DEFAULT '';

-- Seed prompt template for question agent
UPDATE agent_configs SET prompt_template = 'สร้าง {{.Count}} คำถามจากลูกค้าเกี่ยวกับ Facebook Ads หมวด "{{.Category}}"

ข้อมูลอ้างอิงจาก knowledge base:
{{.RAGContext}}
{{.PreviousTopics}}

ตอบเป็น JSON array เท่านั้น แต่ละ object มี:
- "question": คำถามภาษาไทย สั้น กระชับ เหมือนลูกค้าถามจริง
- "questioner_name": ชื่อไทย เช่น "คุณ สมชาย" "คุณ มานี"
- "category": "{{.Category}}"
- "pain_point": ปัญหาหลักเป็นภาษาอังกฤษ เช่น "account_banned" "payment_failed"

ห้ามสร้างคำถามที่แนะนำการทำผิดนโยบาย Facebook'
WHERE agent_name = 'question';

-- Seed prompt template for script agent
UPDATE agent_configs SET prompt_template = 'สร้าง voice script + ข้อมูล metadata สำหรับวิดีโอ Q&A สั้น

โครงสร้างวิดีโอ: ใช้ "ภาพเดียว" คงที่ตลอดทั้งคลิป + "เสียงพากย์เดียว" เล่าจบในตัว (ไม่มีการตัดฉาก ไม่มี multi-scene)

คำถาม: "{{.Question}}"
ถามโดย: {{.QuestionerName}}
หมวด: {{.Category}}

ข้อมูลอ้างอิง:
{{.RAGContext}}

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
- เรียกชื่อแบรนด์เป็นเสียงไทย (ห้ามเขียนชื่อแบรนด์เป็นภาษาอังกฤษ, ห้าม @handle ใน voice_text)
- ใช้ "..." สำหรับจังหวะหายใจระหว่างประโยค

**youtube_title** — ต้องเป็นชื่อวิดีโอที่ดึงดูด ไม่เกิน 70 ตัวอักษร
  - ห้ามเป็น contact info, ห้ามมี @, ห้ามมี line id, ห้ามมี URL
  - ตัวอย่างที่ถูก: "ทำไมแคมเปญโดน Ad Disapproval? วิธีแก้ไขง่ายๆ | Ads Vance"
  - ตัวอย่างที่ผิด: "ติดต่อทีมงาน line id : @adsvance"

**youtube_description** — ข้อมูลติดต่อ ใส่แค่ 2 บรรทัดนี้เท่านั้น:
  "ติดต่อทีมงาน line id : @adsvance\n\nเข้ากลุ่มเทเรแกรมเพื่อรับข่าวสาร : https://t.me/adsvancech"

**youtube_tags** — array ของ tag ภาษาไทย+อังกฤษ

ห้ามแนะนำการทำผิดนโยบาย Facebook'
WHERE agent_name = 'script';

-- Seed prompt template for image agent
UPDATE agent_configs SET prompt_template = 'สร้าง image prompt 1 ภาพ สำหรับวิดีโอ Facebook Ads Q&A

Brand Theme: {{.ThemeDescription}}
คนถาม: {{.QuestionerName}}
คำถาม: {{.QuestionText}}

สร้างภาพสไตล์ chat bubble / Facebook-like UI แสดงคำถามเด่นชัด พร้อม icon คำถาม
ภาพนี้จะใช้เป็นพื้นหลังตลอดทั้งคลิปในขณะที่เสียงพากย์อธิบายคำตอบ

ตอบเป็น JSON array ที่มี object เดียว:
- "scene_number": 1
- "image_prompt_16_9": prompt ภาษาอังกฤษ สำหรับ 16:9 landscape. ใส่ Thai text คำถามบนภาพ.
- "image_prompt_9_16": prompt เหมือนกันแต่สำหรับ 9:16 vertical format.

DO NOT include any logo, mascot, brand name, or brand text in the image.
ภาพต้องมี: dark gradient background ({{.PrimaryColor}} to darker), accent color {{.AccentColor}}, modern flat design, chat bubble with question text.'
WHERE agent_name = 'image';

-- Add business config to settings
INSERT INTO settings (key, value) VALUES
  ('categories', '["account","payment","campaign","pixel"]'),
  ('brand_aliases', '{"AdsVance":"แอดส์แวนซ์","Adsvance":"แอดส์แวนซ์","adsvance":"แอดส์แวนซ์","Ads Vance":"แอดส์แวนซ์","@adsvance":"แอดส์แวนซ์","@AdsVance":"แอดส์แวนซ์","@Adsvance":"แอดส์แวนซ์"}')
ON CONFLICT (key) DO NOTHING;
```

- [ ] **Step 2: Run migration**

Run: `go run cmd/server/main.go -migrate`
Expected: migration 011 applied successfully

- [ ] **Step 3: Commit**

```bash
git add migrations/011_agent_prompt_templates.sql
git commit -m "feat: add prompt_template column and seed templates (migration 011)"
```

---

### Task 2: Template Renderer (shared utility)

**Files:**
- Create: `internal/agent/template.go`

- [ ] **Step 1: Create template renderer**

```go
package agent

import (
	"bytes"
	"fmt"
	"text/template"
)

func renderTemplate(tmplStr string, data any) (string, error) {
	tmpl, err := template.New("prompt").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/agent/template.go
git commit -m "feat: add shared prompt template renderer"
```

---

### Task 3: Update AgentConfig model + repository + handler

**Files:**
- Modify: `internal/models/models.go:93-102` (AgentConfig struct)
- Modify: `internal/repository/agents.go` (all queries)
- Modify: `internal/handler/agents.go:29-47` (Update handler)

- [ ] **Step 1: Add PromptTemplate to AgentConfig**

In `internal/models/models.go`, add the field to the AgentConfig struct:

```go
type AgentConfig struct {
	ID             string          `json:"id"`
	AgentName      string          `json:"agent_name"`
	SystemPrompt   string          `json:"system_prompt"`
	PromptTemplate string          `json:"prompt_template"`
	Model          string          `json:"model"`
	Temperature    float64         `json:"temperature"`
	Enabled        bool            `json:"enabled"`
	Skills         string          `json:"skills"`
	Config         json.RawMessage `json:"config"`
}
```

- [ ] **Step 2: Update repository queries to include prompt_template**

In `internal/repository/agents.go`:

**List method** — add `prompt_template` to SELECT and Scan:

```go
func (r *AgentsRepo) List(ctx context.Context) ([]models.AgentConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, agent_name, system_prompt, prompt_template, model, temperature, enabled, skills, config
		 FROM agent_configs ORDER BY agent_name`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []models.AgentConfig
	for rows.Next() {
		var a models.AgentConfig
		if err := rows.Scan(&a.ID, &a.AgentName, &a.SystemPrompt, &a.PromptTemplate, &a.Model,
			&a.Temperature, &a.Enabled, &a.Skills, &a.Config); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, nil
}
```

**GetByName method** — add `prompt_template` to SELECT and Scan:

```go
func (r *AgentsRepo) GetByName(ctx context.Context, name string) (*models.AgentConfig, error) {
	var a models.AgentConfig
	err := r.pool.QueryRow(ctx,
		`SELECT id, agent_name, system_prompt, prompt_template, model, temperature, enabled, skills, config
		 FROM agent_configs WHERE agent_name = $1`, name,
	).Scan(&a.ID, &a.AgentName, &a.SystemPrompt, &a.PromptTemplate, &a.Model, &a.Temperature, &a.Enabled, &a.Skills, &a.Config)
	if err != nil {
		return nil, fmt.Errorf("get agent %s: %w", name, err)
	}
	return &a, nil
}
```

**Update method** — add `prompt_template` parameter:

```go
func (r *AgentsRepo) Update(ctx context.Context, id string, prompt, promptTemplate, model string, temp float64, enabled bool, skills string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_configs SET system_prompt=$2, prompt_template=$3, model=$4, temperature=$5, enabled=$6, skills=$7 WHERE id=$1`,
		id, prompt, promptTemplate, model, temp, enabled, skills)
	if err != nil {
		return fmt.Errorf("update agent %s: %w", id, err)
	}
	return nil
}
```

- [ ] **Step 3: Update handler to accept prompt_template**

In `internal/handler/agents.go`:

```go
func (h *AgentsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		SystemPrompt   string  `json:"system_prompt"`
		PromptTemplate string  `json:"prompt_template"`
		Model          string  `json:"model"`
		Temperature    float64 `json:"temperature"`
		Enabled        bool    `json:"enabled"`
		Skills         string  `json:"skills"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	if err := h.repo.Update(r.Context(), id, req.SystemPrompt, req.PromptTemplate, req.Model, req.Temperature, req.Enabled, req.Skills); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}
```

- [ ] **Step 4: Build check**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add internal/models/models.go internal/repository/agents.go internal/handler/agents.go
git commit -m "feat: add prompt_template to AgentConfig model, repo, and handler"
```

---

### Task 4: Refactor QuestionAgent to use DB template

**Files:**
- Modify: `internal/agent/question.go:29-86`

- [ ] **Step 1: Define template data struct and use renderTemplate**

Replace the `Generate` method to accept and use the prompt template:

```go
type QuestionTemplateData struct {
	Count          int
	Category       string
	RAGContext     string
	PreviousTopics string
}

func (a *QuestionAgent) Generate(ctx context.Context, count int, category, model, systemPrompt string, temperature float64, promptTemplate string) ([]GeneratedQuestion, error) {
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

	userPrompt, err := renderTemplate(promptTemplate, QuestionTemplateData{
		Count:          count,
		Category:       category,
		RAGContext:     ragContext.String(),
		PreviousTopics: previousList,
	})
	if err != nil {
		return nil, fmt.Errorf("render prompt template: %w", err)
	}

	var questions []GeneratedQuestion
	if err := a.llm.GenerateJSON(ctx, model, systemPrompt, userPrompt, temperature, &questions); err != nil {
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

- [ ] **Step 2: Update caller in orchestrator.go**

In `internal/orchestrator/orchestrator.go`, update the `ProduceWeekly` call (around line 103):

```go
questions, err := o.questionAgent.Generate(ctx, count, category, qaCfg.Model, buildPrompt(qaCfg), qaCfg.Temperature, qaCfg.PromptTemplate)
```

- [ ] **Step 3: Build check**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/agent/question.go internal/orchestrator/orchestrator.go
git commit -m "refactor: QuestionAgent uses prompt_template from DB"
```

---

### Task 5: Refactor ScriptAgent to use DB template

**Files:**
- Modify: `internal/agent/script.go:38-89`

- [ ] **Step 1: Define template data struct and use renderTemplate**

```go
type ScriptTemplateData struct {
	Question       string
	QuestionerName string
	Category       string
	RAGContext     string
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
		return nil, fmt.Errorf("render prompt template: %w", err)
	}

	var script GeneratedScript
	if err := a.llm.GenerateJSON(ctx, model, systemPrompt, userPrompt, temperature, &script); err != nil {
		return nil, fmt.Errorf("generate script: %w", err)
	}
	return &script, nil
}
```

- [ ] **Step 2: Update caller in orchestrator.go**

In `internal/orchestrator/orchestrator.go`, update `produceClipWithID` (around line 171):

```go
script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, scriptCfg.Model, buildPrompt(scriptCfg), scriptCfg.Temperature, scriptCfg.PromptTemplate)
```

- [ ] **Step 3: Build check**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/agent/script.go internal/orchestrator/orchestrator.go
git commit -m "refactor: ScriptAgent uses prompt_template from DB"
```

---

### Task 6: Refactor ImageAgent to use DB template

**Files:**
- Modify: `internal/agent/image.go:24-57`

- [ ] **Step 1: Define template data struct and use renderTemplate**

```go
type ImageTemplateData struct {
	ThemeDescription string
	QuestionerName   string
	QuestionText     string
	PrimaryColor     string
	AccentColor      string
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
		return nil, fmt.Errorf("render prompt template: %w", err)
	}

	var prompts []SceneImagePrompts
	if err := a.llm.GenerateJSON(ctx, model, systemPrompt, userPrompt, temperature, &prompts); err != nil {
		return nil, fmt.Errorf("generate image prompts: %w", err)
	}
	return prompts, nil
}
```

- [ ] **Step 2: Update callers in orchestrator.go**

In `produceClipWithID` (around line 195):

```go
imagePrompts, err := o.imageAgent.GeneratePrompts(ctx, script.Scenes, theme, q.QuestionerName, imageCfg.Model, buildPrompt(imageCfg), imageCfg.Temperature, imageCfg.PromptTemplate)
```

In `resumeFromImagePrompts` (around line 274):

```go
imagePrompts, err := o.imageAgent.GeneratePrompts(ctx, genScenes, theme, questionerName, imageCfg.Model, buildPrompt(imageCfg), imageCfg.Temperature, imageCfg.PromptTemplate)
```

- [ ] **Step 3: Build check**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/agent/image.go internal/orchestrator/orchestrator.go
git commit -m "refactor: ImageAgent uses prompt_template from DB"
```

---

### Task 7: Move categories + brand aliases to settings

**Files:**
- Modify: `internal/orchestrator/orchestrator.go:20-45,86-89`

- [ ] **Step 1: Read categories from settings instead of hardcoded var**

Remove the hardcoded `categories` var and read from DB in `ProduceWeekly`:

```go
// Remove this line:
// var categories = []string{"account", "payment", "campaign", "pixel"}

func (o *Orchestrator) ProduceWeekly(ctx context.Context, count int) error {
	weekNum := int(time.Now().Unix() / (7 * 24 * 3600))

	var categoriesJSON string
	err := o.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'categories'`).Scan(&categoriesJSON)
	if err != nil {
		return fmt.Errorf("read categories setting: %w", err)
	}
	var categories []string
	if err := json.Unmarshal([]byte(categoriesJSON), &categories); err != nil {
		return fmt.Errorf("parse categories: %w", err)
	}
	if len(categories) == 0 {
		return fmt.Errorf("no categories configured in settings")
	}

	category := categories[weekNum%len(categories)]
	log.Printf("Producing %d clips for category: %s", count, category)
	// ... rest unchanged
```

- [ ] **Step 2: Read brand aliases from settings for sanitizeVoiceText**

Replace hardcoded `sanitizeVoiceText` to read aliases from DB. Since this is called per-clip and the aliases rarely change, accept them as a parameter loaded once in `ProduceWeekly`:

```go
func sanitizeVoiceText(s string, brandAliases map[string]string) string {
	for eng, thai := range brandAliases {
		s = strings.ReplaceAll(s, eng, thai)
	}
	s = urlRegex.ReplaceAllString(s, "")
	s = atHandleRgx.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	return s
}
```

In `ProduceWeekly`, load brand aliases once before the loop:

```go
var aliasesJSON string
if err := o.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'brand_aliases'`).Scan(&aliasesJSON); err != nil {
	log.Printf("No brand_aliases setting, using empty: %v", err)
	aliasesJSON = "{}"
}
var brandAliases map[string]string
json.Unmarshal([]byte(aliasesJSON), &brandAliases)
```

Update all callers of `sanitizeVoiceText` to pass `brandAliases`.

In `produceClipWithID`, the brandAliases needs to be passed through. Add it as a parameter:

```go
func (o *Orchestrator) produceClipWithID(ctx context.Context, clipID string, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string) error {
	// ... existing code ...
	fullVoice = sanitizeVoiceText(fullVoice, brandAliases)
	// ...
}
```

Similarly update `produceClip`, `buildVoiceScript` → `sanitizeVoiceText` calls in `resumeFromImagePrompts` and `resumeFromProduction`.

- [ ] **Step 3: Add "categories" and "brand_aliases" to allowed settings in handler**

In `internal/handler/settings.go`, add to the allowed keys so they can be edited from the Settings page. Find the allowed keys map/slice and add `"categories"` and `"brand_aliases"`.

- [ ] **Step 4: Build check**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/handler/settings.go
git commit -m "refactor: read categories and brand aliases from settings table"
```

---

### Task 8: Add youtube_title validation (safety net)

**Files:**
- Modify: `internal/publisher/publisher.go:40-55`

- [ ] **Step 1: Add validation after reading title from DB**

In `PublishReady`, after scanning the title, validate it isn't contact info:

```go
for rows.Next() {
	var clipID, title string
	var description *string
	var video169, video916, thumb *string
	if err := rows.Scan(&clipID, &title, &description, &video169, &video916, &thumb); err != nil {
		return fmt.Errorf("scan clip: %w", err)
	}

	if video169 == nil {
		continue
	}

	// Safety net: if LLM put contact info in title, fall back to clip question
	if isContactInfo(title) {
		var clipTitle string
		if err := p.pool.QueryRow(ctx, `SELECT title FROM clips WHERE id = $1`, clipID).Scan(&clipTitle); err == nil && clipTitle != "" {
			log.Printf("Title validation failed for clip %s: '%s' looks like contact info, using clip question instead", clipID, title)
			title = clipTitle
		}
	}

	// ... rest unchanged
```

- [ ] **Step 2: Add the isContactInfo helper function**

Add at the top of `publisher.go`:

```go
func isContactInfo(title string) bool {
	lower := strings.ToLower(title)
	return strings.Contains(lower, "line id") ||
		strings.Contains(lower, "@adsvance") ||
		strings.Contains(lower, "ติดต่อทีมงาน") ||
		strings.Contains(lower, "t.me/") ||
		strings.Contains(lower, "https://")
}
```

Add `"strings"` to imports.

- [ ] **Step 3: Build check**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add internal/publisher/publisher.go
git commit -m "fix: validate youtube_title is not contact info before publishing"
```

---

### Task 9: Frontend — Add prompt template editor

**Files:**
- Modify: `frontend/src/pages/Agents.tsx`

- [ ] **Step 1: Add prompt_template to Agent interface**

```typescript
interface Agent {
  id: string;
  agent_name: string;
  system_prompt: string;
  prompt_template: string;
  model: string;
  temperature: number;
  enabled: boolean;
  skills: string;
}
```

- [ ] **Step 2: Add prompt_template to edit state initialization**

In the `useEffect` that initializes edits:

```typescript
useEffect(() => {
  if (agents) {
    const initial: Record<string, Partial<Agent>> = {};
    agents.forEach((a) => {
      initial[a.id] = {
        system_prompt: a.system_prompt,
        prompt_template: a.prompt_template ?? '',
        skills: a.skills ?? '',
        temperature: a.temperature,
        enabled: a.enabled,
        model: a.model,
      };
    });
    setEdits(initial);
  }
}, [agents]);
```

- [ ] **Step 3: Add prompt_template to mutation body**

In the `update` mutation:

```typescript
body: JSON.stringify({
  system_prompt: e.system_prompt ?? agent.system_prompt,
  prompt_template: e.prompt_template ?? agent.prompt_template ?? '',
  model: e.model ?? agent.model,
  temperature: e.temperature ?? agent.temperature,
  enabled: e.enabled ?? agent.enabled,
  skills: e.skills ?? agent.skills ?? '',
}),
```

- [ ] **Step 4: Add Prompt Template textarea in the UI**

Add after the System Prompt section (after line 134), before the Model section:

```tsx
{/* Prompt Template */}
<div className="grid gap-2">
  <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
    Prompt Template
  </label>
  <p className="text-xs text-muted-foreground">
    ใช้ {'{{.VariableName}}'} สำหรับตัวแปร — ระบบจะแทนที่ให้อัตโนมัติตอนรัน
  </p>
  <Textarea
    rows={12}
    className="font-mono text-xs"
    value={e.prompt_template ?? agent.prompt_template ?? ''}
    onChange={(ev) =>
      handleEdit(agent.id, 'prompt_template', ev.target.value)
    }
  />
  <div className="flex flex-wrap gap-1.5">
    {getTemplateVars(agent.agent_name).map((v) => (
      <span key={v} className="px-1.5 py-0.5 rounded bg-muted text-[10px] font-mono text-muted-foreground">
        {`{{.${v}}}`}
      </span>
    ))}
  </div>
</div>
```

- [ ] **Step 5: Add getTemplateVars helper function**

Add before the component:

```typescript
const TEMPLATE_VARS: Record<string, string[]> = {
  question: ['Count', 'Category', 'RAGContext', 'PreviousTopics'],
  script: ['Question', 'QuestionerName', 'Category', 'RAGContext'],
  image: ['ThemeDescription', 'QuestionerName', 'QuestionText', 'PrimaryColor', 'AccentColor'],
};

function getTemplateVars(agentName: string): string[] {
  return TEMPLATE_VARS[agentName] ?? [];
}
```

- [ ] **Step 6: Build check**

Run: `cd frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/Agents.tsx
git commit -m "feat: add prompt template editor to Agents page with variable hints"
```

---

### Task 10: Final build + integration verification

- [ ] **Step 1: Full backend build**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 2: Full frontend build**

Run: `cd frontend && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 3: Run migration on local/dev**

Run: `go run cmd/server/main.go -migrate`
Expected: All migrations applied

- [ ] **Step 4: Start server and verify agents page shows prompt templates**

Run: `make run` (or `go run cmd/server/main.go`)
Then open browser → Agents page → expand any agent → verify:
- Prompt Template textarea visible with seeded content
- Variable badges shown below textarea
- Save works (edit → save → refresh → content persists)

- [ ] **Step 5: Final commit (if any remaining changes)**

```bash
git add -A
git commit -m "chore: final integration verification"
```
