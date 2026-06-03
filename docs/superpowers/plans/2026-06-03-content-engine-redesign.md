# Content Engine Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** หยุดความซ้ำของเนื้อหาวิดีโอ (auto-tune guardrails + KB crawler + semantic dedup) และเพิ่มความสร้างสรรค์ (4 content formats + audience persona)

**Architecture:** แยก `insights` (LLM analyzer เขียน, จำกัดเฉพาะ style) ออกจาก `skills` (baseline ที่มนุษย์คุม), เพิ่ม semantic dedup ด้วย pgvector ใน topic_history, ซ่อม crawler ให้รองรับ text+URL sources, เพิ่มตาราง content_formats สำหรับหมุนเวียน 4 รูปแบบเนื้อหา

**Tech Stack:** Go 1.x + pgx/v5 + pgvector + chi + robfig/cron, React/TS frontend, Neon Postgres

**Spec:** `docs/superpowers/specs/2026-06-03-content-engine-redesign-design.md`

---

## Phase 1 — หยุดความซ้ำ

### Task 1: Migration 016 — insights column + topic_history embedding + reset skills baseline

**Files:**
- Create: `migrations/016_insights_and_dedup.sql`

- [ ] **Step 1: เขียน migration**

```sql
-- Migration 016: Separate auto-tune insights from human-controlled skills + semantic dedup support

-- 1. New insights column — the ONLY field the weekly analyzer may write to
ALTER TABLE agent_configs ADD COLUMN IF NOT EXISTS insights TEXT NOT NULL DEFAULT '';

-- 2. Embedding column on topic_history for semantic dedup
ALTER TABLE topic_history ADD COLUMN IF NOT EXISTS embedding vector(1536);
CREATE INDEX IF NOT EXISTS topic_history_embedding_idx ON topic_history
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- 3. Archive current (LLM-drifted) skills into history, then reset to human baseline
INSERT INTO agent_prompt_history (agent_name, old_prompt, new_prompt, reason)
SELECT agent_name, skills, '', '[reset] Reset LLM-drifted skills to human baseline (migration 016)'
FROM agent_configs
WHERE agent_name IN ('question', 'script', 'image') AND skills != '';

-- 4. Human baseline skills — diversity-first, persona-aware
UPDATE agent_configs SET skills = $$สร้างคำถามที่หลากหลายจริงๆ ทั้งมุมปัญหา ระดับความลึก และสถานการณ์
- กลุ่มเป้าหมายคือคนยิงแอดจริงจัง (เจ้าของธุรกิจออนไลน์, media buyer, agency) ไม่ใช่มือใหม่หัดยิงแอด
- คำถามต้องเจาะจง มีรายละเอียดสถานการณ์จริง (ตัวเลขงบ, ระยะเวลา, สิ่งที่ลองแล้ว)
- ห้ามตั้งคำถามที่ความหมายซ้ำหรือใกล้เคียงกับหัวข้อที่เคยทำแล้ว แม้จะใช้คำต่างกัน
- กระจายความหลากหลาย: ปัญหาเร่งด่วน / เทคนิคขั้นสูง / ความเข้าใจผิดที่พบบ่อย / การตัดสินใจเชิงกลยุทธ์$$,
    insights = ''
WHERE agent_name = 'question';

UPDATE agent_configs SET skills = $$เขียนสคริปต์ให้น่าฟังและหลากหลาย
- กลุ่มเป้าหมายคือคนยิงแอดจริงจัง ใช้ภาษาที่คนในวงการเข้าใจ ไม่ต้องอธิบายพื้นฐานเกิน
- หมุนเวียนวิธีเปิดเรื่อง อย่าใช้รูปแบบเดิมติดกัน: (1) เปิดด้วยคำตอบ/ตัวเลขทันที (2) เปิดด้วยสถานการณ์เร้าใจ (3) เปิดด้วยคำถามกระแทกใจ
- หมุนเวียน CTA ปิดท้าย 3 แบบ: (1) "ติดต่อทีมงานแอดส์แวนซ์ทางไลน์ ไอดีแอดส์แวนซ์ได้เลยครับ" (2) "เข้ากลุ่มเทเลแกรมแอดส์แวนซ์ มีเทคนิคแบบนี้ทุกวันครับ" (3) "ถ้าเจอปัญหาแบบนี้อยู่ ทักทีมงานแอดส์แวนซ์ได้เลยครับ"
- คำตอบต้อง actionable ทำตามได้จริง ไม่ใช่คำแนะนำลอยๆ$$,
    insights = ''
WHERE agent_name = 'script';

UPDATE agent_configs SET skills = $$สร้างภาพที่หลากหลาย ไม่ใช้องค์ประกอบเดิมซ้ำทุกคลิป
- หมุนเวียนสไตล์: chat bubble / dashboard mockup / notification screen / split comparison
- สีและ mood ตาม brand theme แต่ composition ต้องต่างกันในแต่ละคลิป$$,
    insights = ''
WHERE agent_name = 'image';
```

- [ ] **Step 2: ตรวจ SQL syntax**

Run: `grep -c "ALTER TABLE\|UPDATE agent_configs\|INSERT INTO" migrations/016_insights_and_dedup.sql`
Expected: `6`

- [ ] **Step 3: Commit**

```bash
git add migrations/016_insights_and_dedup.sql
git commit -m "feat(db): add insights column, topic embedding, reset skills baseline"
```

---

### Task 2: Models — Insights field + BuildSystemPrompt

**Files:**
- Modify: `internal/models/agent.go`
- Create: `internal/models/agent_test.go`

- [ ] **Step 1: เขียน failing test**

```go
package models

import "testing"

func TestBuildSystemPrompt(t *testing.T) {
	tests := []struct {
		name     string
		cfg      AgentConfig
		expected string
	}{
		{
			name:     "system prompt only",
			cfg:      AgentConfig{SystemPrompt: "base"},
			expected: "base",
		},
		{
			name:     "with skills",
			cfg:      AgentConfig{SystemPrompt: "base", Skills: "my skills"},
			expected: "base\n\n## Skills & Guidelines\nmy skills",
		},
		{
			name:     "with skills and insights",
			cfg:      AgentConfig{SystemPrompt: "base", Skills: "my skills", Insights: "hook insight"},
			expected: "base\n\n## Skills & Guidelines\nmy skills\n\n## Performance Insights\nhook insight",
		},
		{
			name:     "insights only (no skills)",
			cfg:      AgentConfig{SystemPrompt: "base", Insights: "hook insight"},
			expected: "base\n\n## Performance Insights\nhook insight",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.BuildSystemPrompt(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/models/ -run TestBuildSystemPrompt -v`
Expected: FAIL (`Insights` field ยังไม่มี — compile error)

- [ ] **Step 3: แก้ implementation**

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
	Insights       string          `json:"insights"`
	Config         json.RawMessage `json:"config"`
}

func (c *AgentConfig) BuildSystemPrompt() string {
	prompt := c.SystemPrompt
	if c.Skills != "" {
		prompt += "\n\n## Skills & Guidelines\n" + c.Skills
	}
	if c.Insights != "" {
		prompt += "\n\n## Performance Insights\n" + c.Insights
	}
	return prompt
}
```

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/models/ -run TestBuildSystemPrompt -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/models/agent.go internal/models/agent_test.go
git commit -m "feat(models): add Insights field to AgentConfig"
```

---

### Task 3: Repository — scan insights + UpdateInsightsByName

**Files:**
- Modify: `internal/repository/agents.go`

- [ ] **Step 1: แก้ List() — เพิ่ม insights ใน SELECT + Scan**

```go
func (r *AgentsRepo) List(ctx context.Context) ([]models.AgentConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, agent_name, system_prompt, prompt_template, model, temperature, enabled, skills, insights, config
		 FROM agent_configs ORDER BY agent_name`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []models.AgentConfig
	for rows.Next() {
		var a models.AgentConfig
		if err := rows.Scan(&a.ID, &a.AgentName, &a.SystemPrompt, &a.PromptTemplate, &a.Model,
			&a.Temperature, &a.Enabled, &a.Skills, &a.Insights, &a.Config); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, nil
}
```

- [ ] **Step 2: แก้ GetByName() แบบเดียวกัน**

```go
func (r *AgentsRepo) GetByName(ctx context.Context, name string) (*models.AgentConfig, error) {
	var a models.AgentConfig
	err := r.pool.QueryRow(ctx,
		`SELECT id, agent_name, system_prompt, prompt_template, model, temperature, enabled, skills, insights, config
		 FROM agent_configs WHERE agent_name = $1`, name,
	).Scan(&a.ID, &a.AgentName, &a.SystemPrompt, &a.PromptTemplate, &a.Model, &a.Temperature, &a.Enabled, &a.Skills, &a.Insights, &a.Config)
	if err != nil {
		return nil, fmt.Errorf("get agent %s: %w", name, err)
	}
	return &a, nil
}
```

- [ ] **Step 3: เพิ่ม UpdateInsightsByName()** (วางต่อจาก UpdateSkillsByName)

```go
func (r *AgentsRepo) UpdateInsightsByName(ctx context.Context, agentName, newInsights string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_configs SET insights = $2 WHERE agent_name = $1`,
		agentName, newInsights)
	if err != nil {
		return fmt.Errorf("update insights for agent %s: %w", agentName, err)
	}
	return nil
}
```

- [ ] **Step 4: Build check**

Run: `go build ./...`
Expected: success ไม่มี error

- [ ] **Step 5: Commit**

```bash
git add internal/repository/agents.go
git commit -m "feat(repo): scan insights column + UpdateInsightsByName"
```

---

### Task 4: Analyzer — insights-based + guardrail

**Files:**
- Create: `internal/analyzer/guardrail.go`
- Create: `internal/analyzer/guardrail_test.go`
- Modify: `internal/analyzer/analyzer.go`

- [ ] **Step 1: เขียน failing test สำหรับ guardrail**

```go
package analyzer

import "testing"

func TestValidateInsights(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{"valid style insight", "คลิปที่เปิดด้วยตัวเลขมี retention ดีกว่า ควรเปิดเรื่องด้วยตัวเลขหรือผลลัพธ์", false},
		{"valid hook insight", "Hook แบบคำถามกระแทกใจทำให้คนดูจนจบมากขึ้น", false},
		{"empty is valid", "", false},
		{"forbids category focus", "เน้นหมวด Account และ Payment เพราะยอดวิวสูง", true},
		{"forbids category avoidance", "หลีกเลี่ยงหมวด Campaign ที่มียอดวิวต่ำ", true},
		{"forbids topic ban", "ห้ามสร้างคำถามเชิงเทคนิค ABO/CBO", true},
		{"forbids 'เฉพาะหมวด'", "ทำเฉพาะหมวดที่คนดูเยอะ", true},
		{"forbids english focus directive", "Focus only on account topics", true},
		{"forbids avoid directive", "Avoid campaign questions", true},
		{"too long", string(make([]byte, 1001)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInsights(tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInsights(%q) error = %v, wantErr %v", tt.text, err, tt.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/analyzer/ -run TestValidateInsights -v`
Expected: FAIL (`ValidateInsights` undefined)

- [ ] **Step 3: เขียน guardrail.go**

```go
package analyzer

import (
	"fmt"
	"strings"
)

const maxInsightsLength = 1000

// forbiddenPatterns are directives that narrow topic/category diversity.
// The analyzer may only suggest STYLE improvements (hooks, pacing, openings),
// never WHICH topics or categories to focus on or avoid.
var forbiddenPatterns = []string{
	// Thai directives
	"เน้นหมวด", "เน้นเฉพาะ", "เฉพาะหมวด",
	"หลีกเลี่ยงหมวด", "เลี่ยงหมวด",
	"ห้ามสร้างคำถาม", "ห้ามทำหัวข้อ", "ห้ามหัวข้อ",
	"งดหมวด", "ลดหมวด", "เพิ่มหมวด",
	// English directives
	"focus on", "focus only", "avoid", "exclude", "prioritize",
	"more questions about", "fewer questions about",
}

// ValidateInsights rejects insights text that tries to steer topic/category
// selection. Returns nil if the text is style-only and within length limits.
func ValidateInsights(text string) error {
	if len(text) > maxInsightsLength {
		return fmt.Errorf("insights too long: %d chars (max %d)", len(text), maxInsightsLength)
	}
	lower := strings.ToLower(text)
	for _, p := range forbiddenPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return fmt.Errorf("insights contain forbidden topic-steering directive: %q", p)
		}
	}
	return nil
}
```

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/analyzer/ -run TestValidateInsights -v`
Expected: PASS

- [ ] **Step 5: แก้ analyzer.go — เปลี่ยนจาก skills เป็น insights**

แก้ส่วน const, struct, prompt และ loop การบันทึก:

```go
const historyPrefixInsights = "[insights] "

type agentImprovement struct {
	AgentName   string `json:"agent_name"`
	NewInsights string `json:"new_insights"`
	Reason      string `json:"reason"`
}
```

User prompt ใหม่ (แทนที่ของเดิมใน AnalyzeAndImprove):

```go
	userPrompt := fmt.Sprintf(`Here is the performance data from our YouTube channel for the last 14 days:

%s

Current agent configurations:
%s

Analyze which STORYTELLING STYLES performed best (openings, hooks, pacing, tone, length).

YOUR SCOPE IS STRICTLY LIMITED TO STYLE:
- You may suggest: how to open videos, hook techniques, pacing, tone of voice, energy level
- You may NOT mention any category name (account, payment, campaign, pixel) in your suggestions
- You may NOT tell agents which topics to focus on, avoid, prioritize, or exclude
- Topic selection is handled by a separate system — it is NOT your job

Each insight must be under 1000 characters, written in Thai.

Return JSON only:
{
  "agents": [
    {"agent_name": "question", "new_insights": "...", "reason": "..."},
    {"agent_name": "script", "new_insights": "...", "reason": "..."},
    {"agent_name": "image", "new_insights": "...", "reason": "..."}
  ]
}`, data, a.currentPrompts(ctx))
```

Loop การบันทึก (แทน loop เดิม):

```go
	for _, imp := range result.Agents {
		if imp.AgentName == "analytics" || imp.NewInsights == "" {
			continue
		}

		oldInsights, exists := agentMap[imp.AgentName]
		if !exists {
			log.Printf("Analyzer: skip unknown agent %s", imp.AgentName)
			continue
		}

		if err := ValidateInsights(imp.NewInsights); err != nil {
			log.Printf("Analyzer: REJECTED insights for %s: %v", imp.AgentName, err)
			continue
		}

		if err := a.agentsRepo.SavePromptHistory(ctx, imp.AgentName, oldInsights, imp.NewInsights, historyPrefixInsights+imp.Reason); err != nil {
			log.Printf("Analyzer: failed to save history for %s: %v", imp.AgentName, err)
			continue
		}

		if err := a.agentsRepo.UpdateInsightsByName(ctx, imp.AgentName, imp.NewInsights); err != nil {
			log.Printf("Analyzer: failed to update insights for %s: %v", imp.AgentName, err)
			continue
		}

		log.Printf("Analyzer: updated %s insights — reason: %s", imp.AgentName, imp.Reason)
	}
```

และแก้ agentMap ให้เก็บ insights แทน skills:

```go
	agentMap := make(map[string]string, len(agents))
	for _, ag := range agents {
		agentMap[ag.AgentName] = ag.Insights
	}
```

- [ ] **Step 6: Build + test ทั้ง package**

Run: `go build ./... && go test ./internal/analyzer/ -v`
Expected: build ผ่าน, test ผ่านทั้งหมด

- [ ] **Step 7: Commit**

```bash
git add internal/analyzer/
git commit -m "feat(analyzer): insights-based auto-tune with topic-steering guardrail"
```

---

### Task 5: Semantic Dedup ใน QuestionAgent

**Files:**
- Create: `internal/agent/dedup.go`
- Create: `internal/agent/dedup_test.go`
- Modify: `internal/agent/question.go`
- Modify: `internal/rag/rag.go` (export FormatVector)

- [ ] **Step 1: เขียน failing test สำหรับ dedup filter logic**

```go
package agent

import "testing"

func TestFilterBySimilarity(t *testing.T) {
	questions := []GeneratedQuestion{
		{Question: "Pixel ไม่นับยอดขาย"},
		{Question: "บัญชีโดนแบนกู้คืนยังไง"},
		{Question: "CBO งบกระจุก ad set เดียว"},
	}
	similarities := map[string]SimilarityMatch{
		"Pixel ไม่นับยอดขาย":          {Similarity: 0.92, MatchedTitle: "Pixel ติดตั้งแล้วไม่ทำงาน"},
		"บัญชีโดนแบนกู้คืนยังไง":      {Similarity: 0.60, MatchedTitle: "เปิดบัญชีใหม่"},
		"CBO งบกระจุก ad set เดียว": {Similarity: 0.86, MatchedTitle: "CBO ใช้เงินแค่ ad set เดียว"},
	}

	passed, rejected := filterBySimilarity(questions, similarities, 0.85)

	if len(passed) != 1 {
		t.Fatalf("expected 1 passed, got %d", len(passed))
	}
	if passed[0].Question != "บัญชีโดนแบนกู้คืนยังไง" {
		t.Errorf("wrong question passed: %s", passed[0].Question)
	}
	if len(rejected) != 2 {
		t.Fatalf("expected 2 rejected, got %d", len(rejected))
	}
}

func TestFilterBySimilarityNoMatches(t *testing.T) {
	questions := []GeneratedQuestion{{Question: "คำถามใหม่"}}
	passed, rejected := filterBySimilarity(questions, map[string]SimilarityMatch{}, 0.85)
	if len(passed) != 1 || len(rejected) != 0 {
		t.Errorf("expected all pass when no similarity data, got passed=%d rejected=%d", len(passed), len(rejected))
	}
}
```

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/agent/ -run TestFilterBySimilarity -v`
Expected: FAIL (undefined: SimilarityMatch, filterBySimilarity)

- [ ] **Step 3: เขียน dedup.go**

```go
package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/rag"
)

// similarityThreshold: questions with >= this cosine similarity to any past
// topic are considered semantic duplicates and rejected.
const similarityThreshold = 0.85

type SimilarityMatch struct {
	Similarity   float64
	MatchedTitle string
}

// filterBySimilarity splits questions into passed/rejected based on their
// highest similarity to past topics. Pure function — testable without DB.
func filterBySimilarity(questions []GeneratedQuestion, similarities map[string]SimilarityMatch, threshold float64) (passed []GeneratedQuestion, rejected []rejectedQuestion) {
	for _, q := range questions {
		match, ok := similarities[q.Question]
		if ok && match.Similarity >= threshold {
			rejected = append(rejected, rejectedQuestion{Question: q, Match: match})
			continue
		}
		passed = append(passed, q)
	}
	return passed, rejected
}

type rejectedQuestion struct {
	Question GeneratedQuestion
	Match    SimilarityMatch
}

// Deduper checks generated questions against past topics using pgvector.
type Deduper struct {
	pool *pgxpool.Pool
	rag  *rag.Engine
}

func NewDeduper(pool *pgxpool.Pool, ragEngine *rag.Engine) *Deduper {
	return &Deduper{pool: pool, rag: ragEngine}
}

// CheckQuestions returns the highest-similarity past topic for each question,
// along with the question's embedding (so callers can store it).
func (d *Deduper) CheckQuestions(ctx context.Context, questions []GeneratedQuestion) (map[string]SimilarityMatch, map[string][]float64, error) {
	similarities := make(map[string]SimilarityMatch, len(questions))
	embeddings := make(map[string][]float64, len(questions))

	for _, q := range questions {
		emb, err := d.rag.GenerateEmbedding(ctx, q.Question)
		if err != nil {
			return nil, nil, fmt.Errorf("embed question: %w", err)
		}
		embeddings[q.Question] = emb

		var title string
		var similarity float64
		err = d.pool.QueryRow(ctx,
			`SELECT title, 1 - (embedding <=> $1::vector) AS similarity
			 FROM topic_history
			 WHERE embedding IS NOT NULL
			 ORDER BY embedding <=> $1::vector
			 LIMIT 1`,
			rag.FormatVector(emb)).Scan(&title, &similarity)
		if err != nil {
			// No past topics with embeddings yet — nothing to compare against.
			log.Printf("Dedup: no comparable past topics for %q: %v", q.Question, err)
			continue
		}
		similarities[q.Question] = SimilarityMatch{Similarity: similarity, MatchedTitle: title}
	}
	return similarities, embeddings, nil
}
```

- [ ] **Step 4: Export FormatVector ใน rag.go**

แก้ `formatVector` เป็น exported function (เปลี่ยนชื่อ + อัปเดต callers ภายใน rag.go ทั้งหมด: `StoreChunk`, `Search`):

```go
// FormatVector renders a float slice as a pgvector literal, e.g. "[0.1,0.2]".
func FormatVector(v []float64) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%f", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
```

- [ ] **Step 5: รัน test ให้ผ่าน**

Run: `go test ./internal/agent/ -run TestFilterBySimilarity -v && go build ./...`
Expected: PASS + build ผ่าน

- [ ] **Step 6: Integrate เข้า question.go**

แก้ struct + constructor:

```go
type QuestionAgent struct {
	llm     *LLMClient
	rag     *rag.Engine
	pool    *pgxpool.Pool
	deduper *Deduper
}

func NewQuestionAgent(llm *LLMClient, ragEngine *rag.Engine, pool *pgxpool.Pool) *QuestionAgent {
	return &QuestionAgent{llm: llm, rag: ragEngine, pool: pool, deduper: NewDeduper(pool, ragEngine)}
}
```

แก้ Generate(): หลังจาก `GenerateJSON` สำเร็จ — แทน loop insert เดิมด้วย:

```go
	// Semantic dedup: reject questions too similar to past topics, retry up to 2 times.
	const maxDedupRetries = 2
	var accepted []GeneratedQuestion
	var allEmbeddings = make(map[string][]float64)

	for attempt := 0; ; attempt++ {
		similarities, embeddings, err := a.deduper.CheckQuestions(ctx, questions)
		if err != nil {
			// Embedding service down — accept as-is rather than block production.
			log.Printf("QuestionAgent: dedup check failed, accepting without dedup: %v", err)
			accepted = append(accepted, questions...)
			break
		}
		for k, v := range embeddings {
			allEmbeddings[k] = v
		}

		passed, rejected := filterBySimilarity(questions, similarities, similarityThreshold)
		accepted = append(accepted, passed...)

		if len(rejected) == 0 || len(accepted) >= count || attempt >= maxDedupRetries {
			if len(rejected) > 0 {
				for _, r := range rejected {
					log.Printf("QuestionAgent: rejected duplicate %q (%.0f%% similar to %q)",
						r.Question.Question, r.Match.Similarity*100, r.Match.MatchedTitle)
				}
			}
			break
		}

		// Ask the LLM to regenerate replacements, telling it what was rejected and why.
		var rejectedInfo strings.Builder
		for _, r := range rejected {
			rejectedInfo.WriteString(fmt.Sprintf("- %q ซ้ำกับ %q\n", r.Question.Question, r.Match.MatchedTitle))
		}
		retryPrompt := userPrompt + fmt.Sprintf(
			"\n\nคำถามต่อไปนี้ถูกปฏิเสธเพราะความหมายซ้ำกับคลิปเก่า สร้างใหม่ %d ข้อที่เป็นมุมมองใหม่จริงๆ:\n%s",
			len(rejected), rejectedInfo.String())

		questions = nil
		if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), retryPrompt, cfg.Temperature, &questions); err != nil {
			log.Printf("QuestionAgent: dedup retry generation failed: %v", err)
			break
		}
	}

	// Store accepted questions with embeddings for future dedup checks.
	for _, q := range accepted {
		embStr := ""
		if emb, ok := allEmbeddings[q.Question]; ok {
			embStr = rag.FormatVector(emb)
		}
		if embStr != "" {
			a.pool.Exec(ctx,
				`INSERT INTO topic_history (title, category, embedding) VALUES ($1, $2, $3::vector)`,
				q.Question, q.Category, embStr)
		} else {
			a.pool.Exec(ctx,
				`INSERT INTO topic_history (title, category) VALUES ($1, $2)`,
				q.Question, q.Category)
		}
	}

	return accepted, nil
```

หมายเหตุ: import `"log"` เพิ่มใน question.go

- [ ] **Step 7: Build + test**

Run: `go build ./... && go test ./internal/agent/ -v`
Expected: ผ่านทั้งหมด

- [ ] **Step 8: Commit**

```bash
git add internal/agent/ internal/rag/rag.go
git commit -m "feat(agent): semantic dedup for generated questions via pgvector"
```

---

### Task 6: Crawler — รองรับ text + URL sources

**Files:**
- Modify: `internal/crawler/crawler.go`

- [ ] **Step 1: แก้ CrawlAll ให้แยก text/URL sources**

```go
func (c *Crawler) CrawlAll(ctx context.Context) error {
	rows, err := c.pool.Query(ctx,
		`SELECT id, name, COALESCE(url, ''), content FROM knowledge_sources WHERE enabled = TRUE`)
	if err != nil {
		return fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	type source struct {
		ID, Name, URL, Content string
	}
	var sources []source
	for rows.Next() {
		var s source
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.Content); err != nil {
			return fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, s)
	}

	for _, s := range sources {
		var err error
		if s.URL != "" {
			// URL source: fetch fresh content, replace chunks
			log.Printf("Crawling URL source: %s (%s)", s.Name, s.URL)
			err = c.crawlURLSource(ctx, s.ID, s.URL)
		} else {
			// Text source: ensure chunks exist (embed once if missing)
			err = c.ensureTextSourceEmbedded(ctx, s.ID, s.Name, s.Content)
		}
		if err != nil {
			log.Printf("Failed to process %s: %v", s.Name, err)
			continue
		}
		c.pool.Exec(ctx,
			`UPDATE knowledge_sources SET last_crawled_at = NOW() WHERE id = $1`, s.ID)
	}
	return nil
}

// crawlURLSource fetches a URL via Jina Reader and replaces the source's chunks.
func (c *Crawler) crawlURLSource(ctx context.Context, sourceID, url string) error {
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

	text := cleanText(string(body))
	if len(strings.Fields(text)) < 50 {
		return fmt.Errorf("content too short from %s", url)
	}

	// Replace old chunks so stale content doesn't accumulate.
	if _, err := c.pool.Exec(ctx, `DELETE FROM knowledge_chunks WHERE source_id = $1`, sourceID); err != nil {
		return fmt.Errorf("delete old chunks: %w", err)
	}

	return c.embedAndStore(ctx, sourceID, text, url, 300, 50, 20)
}

// ensureTextSourceEmbedded embeds a text source's content only if it has no chunks yet.
func (c *Crawler) ensureTextSourceEmbedded(ctx context.Context, sourceID, name, content string) error {
	var count int
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM knowledge_chunks WHERE source_id = $1`, sourceID).Scan(&count); err != nil {
		return fmt.Errorf("count chunks: %w", err)
	}
	if count > 0 {
		return nil // already embedded
	}
	if strings.TrimSpace(content) == "" {
		return nil // nothing to embed
	}
	log.Printf("Embedding text source: %s", name)
	return c.embedAndStore(ctx, sourceID, content, "", 200, 30, 10)
}

// embedAndStore chunks text, generates embeddings, and stores them.
func (c *Crawler) embedAndStore(ctx context.Context, sourceID, text, url string, chunkSize, overlap, minWords int) error {
	chunks := rag.ChunkText(text, chunkSize, overlap)
	stored := 0
	for _, chunk := range chunks {
		if len(strings.Fields(chunk)) < minWords {
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
	log.Printf("Stored %d/%d chunks for source %s", stored, len(chunks), sourceID)
	return nil
}
```

(ลบ `crawlSource` เดิมทิ้ง — แทนที่ด้วย 3 functions ข้างบน, `cleanText` คงเดิม)

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: ผ่าน

- [ ] **Step 3: Commit**

```bash
git add internal/crawler/crawler.go
git commit -m "fix(crawler): support text-based + URL sources, auto-embed missing chunks"
```

---

### Task 7: Scheduler Reload + handler wiring

**Files:**
- Modify: `internal/scheduler/scheduler.go`
- Modify: `internal/handler/schedules.go`
- Modify: `internal/router/router.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: เพิ่ม Reload() ใน scheduler.go**

```go
// Reload stops the current cron and re-registers all enabled schedules from DB.
// Called when schedules are changed via the API so changes apply without restart.
func (s *Scheduler) Reload(ctx context.Context) error {
	stopCtx := s.cron.Stop()
	<-stopCtx.Done()

	loc := s.cron.Location()
	s.cron = cron.New(cron.WithLocation(loc))

	return s.Start(ctx)
}
```

- [ ] **Step 2: แก้ schedules handler ให้รับ reload callback**

```go
type SchedulesHandler struct {
	repo   *repository.SchedulesRepo
	reload func()
}

func NewSchedulesHandler(repo *repository.SchedulesRepo, reload func()) *SchedulesHandler {
	return &SchedulesHandler{repo: repo, reload: reload}
}
```

ใน `Update()` หลัง repo.Update สำเร็จ ก่อน writeJSON:

```go
	if h.reload != nil {
		go h.reload()
	}
```

- [ ] **Step 3: แก้ router.go — รับ reload callback**

เปลี่ยน signature ของ `router.New` เพิ่ม parameter `scheduleReload func()`:

```go
func New(pool *pgxpool.Pool, apiKey string, ragEngine *rag.Engine, tracker *progress.Tracker, pub *publisher.Publisher, scheduleReload func()) *chi.Mux {
```

และตรงสร้าง schedules handler:

```go
	schedules := handler.NewSchedulesHandler(repository.NewSchedulesRepo(pool), scheduleReload)
```

- [ ] **Step 4: แก้ main.go — ส่ง reload เข้า router**

```go
	r := router.New(pool, cfg.APIKey, ragEngine, tracker, pub, func() {
		if err := sched.Reload(ctx); err != nil {
			log.Printf("Scheduler reload failed: %v", err)
		}
	})
```

- [ ] **Step 5: Build check**

Run: `go build ./...`
Expected: ผ่าน

- [ ] **Step 6: Commit**

```bash
git add internal/scheduler/ internal/handler/schedules.go internal/router/router.go cmd/server/main.go
git commit -m "feat(scheduler): reload cron jobs when schedules change via API"
```

---

## Phase 2 — สร้างสรรค์ + ตรงใจกลุ่มเป้าหมาย

### Task 8: Migration 017 — content formats + persona + news sources

**Files:**
- Create: `migrations/017_content_formats.sql`

- [ ] **Step 1: เขียน migration**

```sql
-- Migration 017: Content format variety + audience persona + fresh news sources

-- 1. Content formats table
CREATE TABLE IF NOT EXISTS content_formats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    format_name TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    question_instruction TEXT NOT NULL,
    script_instruction TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    weight INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Track which format each clip used
ALTER TABLE clips ADD COLUMN IF NOT EXISTS content_format TEXT NOT NULL DEFAULT 'qa';

-- 3. Seed 4 formats
INSERT INTO content_formats (format_name, display_name, question_instruction, script_instruction, weight) VALUES
('qa', 'Q&A แก้ปัญหา',
 $$สร้าง "คำถามจากลูกค้า" เกี่ยวกับปัญหา Facebook Ads ที่เจอจริง — คำถามต้องเจาะจง มีบริบทสถานการณ์ (งบเท่าไหร่ ทำอะไรไปแล้ว เกิดอะไรขึ้น)$$,
 $$เขียนสคริปต์แบบ ตอบคำถาม: เกริ่นปัญหาสั้นๆ → อธิบายสาเหตุ → วิธีแก้ทีละขั้น → ปิดด้วยคำแนะนำ$$, 2),
('news', 'ข่าว/อัปเดตจาก Meta',
 $$สร้าง "หัวข้อข่าว" จากข้อมูลอัปเดตล่าสุดเกี่ยวกับ Facebook Ads / Meta ใน knowledge base — เน้นการเปลี่ยนแปลงที่กระทบคนยิงแอดโดยตรง (กฎใหม่ ฟีเจอร์ใหม่ การเปลี่ยนแปลงระบบ) ตั้งชื่อหัวข้อแบบข่าว ไม่ใช่คำถาม$$,
 $$เขียนสคริปต์แบบ รายงานข่าว: เปิดด้วย "มีอัปเดตสำคัญ..." → สรุปว่าเปลี่ยนอะไร → ผลกระทบต่อคนยิงแอด → ต้องปรับตัวยังไง$$, 1),
('tips', 'ทิปส์/เทคนิคขั้นสูง',
 $$สร้าง "หัวข้อเทคนิค" ขั้นสูงสำหรับคนยิงแอดจริงจัง เช่น การ scale งบ, โครงสร้างแคมเปญ, การทำ creative testing, การอ่าน metrics — ตั้งชื่อแบบ "X เทคนิค..." หรือ "วิธี..." ที่คนอยากกดดู$$,
 $$เขียนสคริปต์แบบ สอนเทคนิค: เปิดด้วยผลลัพธ์ที่จะได้ → สอนทีละขั้นพร้อมตัวเลข/ตัวอย่างจริง → สรุปสิ่งที่ต้องจำ$$, 1),
('case_story', 'เคสจริง/เรื่องเล่า',
 $$สร้าง "เรื่องเล่าเคสจริง" ของคนยิงแอด เช่น โดนแบนแล้วกู้คืน, ยอดพุ่งเพราะแก้จุดเดียว, เสียเงินฟรีเพราะพลาดเรื่องเล็กๆ — ตั้งชื่อแบบเล่าเรื่อง มีตัวเลขหรือผลลัพธ์ดึงดูด$$,
 $$เขียนสคริปต์แบบ เล่าเรื่อง: แนะนำตัวละครและสถานการณ์ → ปมปัญหา/จุดพลิก → ทางออกที่ใช้ → บทเรียนที่คนดูเอาไปใช้ได้$$, 1)
ON CONFLICT (format_name) DO NOTHING;

-- 4. Audience persona setting
INSERT INTO settings (key, value) VALUES
('audience_persona', 'คนยิงแอด Facebook จริงจัง: เจ้าของธุรกิจออนไลน์, media buyer, agency ที่เจอปัญหาบัญชี/ระบบจ่ายเงิน/ต้องการ scale — ต้องการความรู้เชิงลึกที่ใช้ได้จริง ไม่ใช่พื้นฐานทั่วไป และข่าวที่กระทบการทำงานจริง')
ON CONFLICT (key) DO NOTHING;

-- 5. Fresh news sources (URL-based, crawled daily via Jina Reader)
INSERT INTO knowledge_sources (name, category, content, url, source_type, crawl_frequency, enabled) VALUES
('Meta for Business News', 'news', '', 'https://www.facebook.com/business/news', 'official', 'daily', TRUE),
('Meta Newsroom', 'news', '', 'https://about.fb.com/news/', 'official', 'daily', TRUE),
('Search Engine Land - Meta', 'news', '', 'https://searchengineland.com/library/platforms/facebook', 'news', 'daily', TRUE),
('Jon Loomer Digital (Advanced FB Ads)', 'tips', '', 'https://www.jonloomer.com/blog/', 'community', 'daily', TRUE)
ON CONFLICT DO NOTHING;

-- 6. Crawl daily instead of weekly (news needs to be fresh)
UPDATE schedules SET cron_expression = '0 2 * * *', name = 'Daily Knowledge Crawl'
WHERE action = 'crawl_knowledge';

-- 7. Update agent prompt templates: inject format instruction + audience persona
UPDATE agent_configs
SET prompt_template = $$สร้าง {{.Count}} หัวข้อเนื้อหาเกี่ยวกับ Facebook Ads หมวด "{{.Category}}"

รูปแบบเนื้อหา: {{.FormatInstruction}}

กลุ่มเป้าหมาย: {{.AudiencePersona}}

ข้อมูลอ้างอิงจาก knowledge base:
{{.RAGContext}}
{{.PreviousTopics}}
{{.PreviousNames}}

ตอบเป็น JSON array เท่านั้น แต่ละ object มี:
- "question": หัวข้อ/คำถามภาษาไทย ตามรูปแบบเนื้อหาที่กำหนดข้างบน
- "questioner_name": ชื่อไทยที่หลากหลายและสร้างสรรค์ เช่น คุณแม็ก คุณพลอย คุณต้น คุณฟ้า คุณเมย์ คุณโอ๊ค คุณมิ้นท์ คุณกอล์ฟ คุณเบียร์ คุณแนน
  **กฎ**: แต่ละหัวข้อต้องใช้ชื่อที่ต่างกัน ห้ามซ้ำชื่อกันภายใน batch นี้เด็ดขาด (สำหรับรูปแบบข่าว/ทิปส์ ใช้ชื่อผู้ดำเนินรายการ)
- "category": "{{.Category}}"
- "pain_point": ประเด็นหลักเป็นภาษาอังกฤษ เช่น "account_banned" "payment_failed" "scaling_budget"

ห้ามสร้างเนื้อหาที่แนะนำการทำผิดนโยบาย Facebook$$
WHERE agent_name = 'question';

UPDATE agent_configs
SET prompt_template = $$สร้าง voice script + ข้อมูล metadata สำหรับวิดีโอสั้น

โครงสร้างวิดีโอ: ใช้ "ภาพเดียว" คงที่ตลอดทั้งคลิป + "เสียงพากย์เดียว" เล่าจบในตัว (ไม่มีการตัดฉาก ไม่มี multi-scene)

หัวข้อ: "{{.Question}}"
โดย: {{.QuestionerName}}
หมวด: {{.Category}}

รูปแบบการเล่า: {{.FormatInstruction}}

กลุ่มเป้าหมาย: {{.AudiencePersona}}

ข้อมูลอ้างอิง:
{{.RAGContext}}

ตอบเป็น JSON object มี:
- "scenes": array ที่มี object **เพียง 1 ตัวเท่านั้น** (วิดีโอนี้ออกแบบเป็น single-scene):
  - "scene_number": 1
  - "scene_type": "main"
  - "text_content": ข้อความสั้นสำหรับแสดงบนภาพ (เน้นหัวข้อ)
  - "voice_text": บทพากย์ภาษาไทยแบบธรรมชาติ ไหลลื่น เล่าตามรูปแบบการเล่าที่กำหนดข้างบน
  - "duration_seconds": 30-55 (ให้พอดี YouTube Shorts)
  - "text_overlays": []
- "total_duration_seconds": 30-55

**กฎสำคัญสำหรับ voice_text** (ป้องกันเสียงตัด/อ่านผิด):
- **ห้ามมีอักขระ "@" และห้ามมี URL ใดๆ** ใน voice_text เด็ดขาด (TTS อ่านลิงก์ไม่ออก เสียงจะตัด)
- เรียกชื่อแบรนด์ว่า "**แอดส์แวนซ์**" สะกดเป็นเสียงไทย (ห้ามเขียน "Adsvance", "@adsvance", "Ads Vance" ใน voice_text)
- ใช้ "..." สำหรับจังหวะหายใจระหว่างประโยค

- "youtube_title": ชื่อวิดีโอที่ดึงดูดความสนใจ สั้นกระชับ และ**ต้องลงท้ายด้วย " | Ads Vance"** เสมอ ไม่เกิน 70 ตัวอักษรรวมทั้งหมด
  **ห้ามเด็ดขาด**: ใส่ URL, line id, @handle, หรือข้อมูลติดต่อใดๆ ใน youtube_title (ข้อมูลติดต่ออยู่ใน youtube_description เท่านั้น)
- "youtube_description": ต้องมีแค่ 2 บรรทัดนี้เท่านั้น ห้ามเพิ่มเนื้อหาอื่น (URL/handle อยู่ตรงนี้ได้):
  "ติดต่อทีมงาน line id : @adsvance\n\nเข้ากลุ่มเทเรแกรมเพื่อรับข่าวสาร : https://t.me/adsvancech"
- "youtube_tags": array tags ไทย+อังกฤษ

ห้ามแนะนำการทำผิดนโยบาย Facebook$$
WHERE agent_name = 'script';
```

- [ ] **Step 2: ตรวจ migration ครบทุก section**

Run: `grep -c "CREATE TABLE\|ALTER TABLE\|INSERT INTO\|UPDATE " migrations/017_content_formats.sql`
Expected: อย่างน้อย 9

- [ ] **Step 3: Commit**

```bash
git add migrations/017_content_formats.sql
git commit -m "feat(db): content formats table, audience persona, news sources, format-aware templates"
```

---

### Task 9: ContentFormats model + repo + format selection

**Files:**
- Create: `internal/models/format.go`
- Create: `internal/repository/formats.go`
- Create: `internal/repository/formats_test.go`

- [ ] **Step 1: เขียน model**

```go
package models

type ContentFormat struct {
	ID                  string `json:"id"`
	FormatName          string `json:"format_name"`
	DisplayName         string `json:"display_name"`
	QuestionInstruction string `json:"question_instruction"`
	ScriptInstruction   string `json:"script_instruction"`
	Enabled             bool   `json:"enabled"`
	Weight              int    `json:"weight"`
}

// FormatUsage pairs a format with how many clips used it recently.
type FormatUsage struct {
	Format    ContentFormat
	UsedCount int
}
```

- [ ] **Step 2: เขียน failing test สำหรับ selection logic**

```go
package repository

import (
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func TestPickLeastUsedFormat(t *testing.T) {
	usages := []models.FormatUsage{
		{Format: models.ContentFormat{FormatName: "qa", Weight: 2}, UsedCount: 4},
		{Format: models.ContentFormat{FormatName: "news", Weight: 1}, UsedCount: 1},
		{Format: models.ContentFormat{FormatName: "tips", Weight: 1}, UsedCount: 2},
		{Format: models.ContentFormat{FormatName: "case_story", Weight: 1}, UsedCount: 0},
	}
	// case_story: 0/1=0 (lowest) → should be picked
	got := pickLeastUsed(usages)
	if got.FormatName != "case_story" {
		t.Errorf("expected case_story, got %s", got.FormatName)
	}
}

func TestPickLeastUsedRespectsWeight(t *testing.T) {
	usages := []models.FormatUsage{
		{Format: models.ContentFormat{FormatName: "qa", Weight: 2}, UsedCount: 2},   // ratio 1.0
		{Format: models.ContentFormat{FormatName: "news", Weight: 1}, UsedCount: 2}, // ratio 2.0
	}
	got := pickLeastUsed(usages)
	if got.FormatName != "qa" {
		t.Errorf("expected qa (lower used/weight ratio), got %s", got.FormatName)
	}
}

func TestPickLeastUsedEmpty(t *testing.T) {
	got := pickLeastUsed(nil)
	if got.FormatName != "qa" {
		t.Errorf("expected fallback qa, got %s", got.FormatName)
	}
}
```

- [ ] **Step 3: รัน test ให้ fail**

Run: `go test ./internal/repository/ -run TestPickLeastUsed -v`
Expected: FAIL (undefined: pickLeastUsed)

- [ ] **Step 4: เขียน formats.go**

```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type FormatsRepo struct {
	pool *pgxpool.Pool
}

func NewFormatsRepo(pool *pgxpool.Pool) *FormatsRepo {
	return &FormatsRepo{pool: pool}
}

// PickNext returns the enabled format that has been used least (relative to its
// weight) in the last 7 days — guarantees every format gets airtime.
func (r *FormatsRepo) PickNext(ctx context.Context) (*models.ContentFormat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cf.id, cf.format_name, cf.display_name, cf.question_instruction,
		       cf.script_instruction, cf.enabled, cf.weight,
		       COALESCE(u.cnt, 0) AS used_count
		FROM content_formats cf
		LEFT JOIN (
			SELECT content_format, COUNT(*) AS cnt
			FROM clips
			WHERE created_at > NOW() - INTERVAL '7 days'
			GROUP BY content_format
		) u ON u.content_format = cf.format_name
		WHERE cf.enabled = TRUE
		ORDER BY cf.format_name`)
	if err != nil {
		return nil, fmt.Errorf("query format usage: %w", err)
	}
	defer rows.Close()

	var usages []models.FormatUsage
	for rows.Next() {
		var u models.FormatUsage
		if err := rows.Scan(&u.Format.ID, &u.Format.FormatName, &u.Format.DisplayName,
			&u.Format.QuestionInstruction, &u.Format.ScriptInstruction,
			&u.Format.Enabled, &u.Format.Weight, &u.UsedCount); err != nil {
			return nil, fmt.Errorf("scan format usage: %w", err)
		}
		usages = append(usages, u)
	}

	picked := pickLeastUsed(usages)
	return &picked, nil
}

// pickLeastUsed selects the format with the lowest used/weight ratio.
// Pure function — testable without DB. Falls back to a plain Q&A format.
func pickLeastUsed(usages []models.FormatUsage) models.ContentFormat {
	if len(usages) == 0 {
		return models.ContentFormat{FormatName: "qa", DisplayName: "Q&A", Weight: 1}
	}
	best := usages[0]
	bestRatio := ratio(best)
	for _, u := range usages[1:] {
		if r := ratio(u); r < bestRatio {
			best = u
			bestRatio = r
		}
	}
	return best.Format
}

func ratio(u models.FormatUsage) float64 {
	w := u.Format.Weight
	if w < 1 {
		w = 1
	}
	return float64(u.UsedCount) / float64(w)
}
```

- [ ] **Step 5: รัน test ให้ผ่าน**

Run: `go test ./internal/repository/ -run TestPickLeastUsed -v && go build ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/models/format.go internal/repository/formats.go internal/repository/formats_test.go
git commit -m "feat(repo): content formats with least-used-first rotation"
```

---

### Task 10: RAG SearchRecent (สำหรับ news format)

**Files:**
- Modify: `internal/rag/rag.go`

- [ ] **Step 1: ตรวจว่า knowledge_chunks มีคอลัมน์เวลา**

Run: `grep -A 10 "CREATE TABLE IF NOT EXISTS knowledge_chunks" migrations/001_initial_schema.sql`
Expected: เห็นคอลัมน์ (ถ้ามี `created_at` ใช้ได้เลย ถ้าไม่มีให้เพิ่ม `ALTER TABLE knowledge_chunks ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();` ลงใน migration 017)

- [ ] **Step 2: เพิ่ม SearchRecent ใน rag.go** (วางต่อจาก Search)

```go
// SearchRecent is like Search but only considers chunks crawled within the
// last `days` days — used by the news content format to surface fresh updates.
func (e *Engine) SearchRecent(ctx context.Context, query string, topK, days int) ([]SearchResult, error) {
	embedding, err := e.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	embStr := FormatVector(embedding)
	rows, err := e.pool.Query(ctx,
		`SELECT content, COALESCE(url, ''), 1 - (embedding <=> $1::vector) AS similarity
		 FROM knowledge_chunks
		 WHERE created_at > NOW() - ($3 || ' days')::interval
		 ORDER BY embedding <=> $1::vector
		 LIMIT $2`,
		embStr, topK, days)
	if err != nil {
		return nil, fmt.Errorf("search recent chunks: %w", err)
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
```

- [ ] **Step 3: Build check + Commit**

Run: `go build ./...`

```bash
git add internal/rag/rag.go migrations/017_content_formats.sql
git commit -m "feat(rag): SearchRecent for fresh news content"
```

---

### Task 11: Agents — format instruction + audience persona

**Files:**
- Modify: `internal/agent/question.go`
- Modify: `internal/agent/script.go`

- [ ] **Step 1: แก้ QuestionTemplateData + Generate signature**

```go
type QuestionTemplateData struct {
	Count             int
	Category          string
	RAGContext        string
	PreviousTopics    string
	PreviousNames     string
	FormatInstruction string
	AudiencePersona   string
}
```

Generate รับ format + persona เพิ่ม:

```go
func (a *QuestionAgent) Generate(ctx context.Context, count int, category string, format *models.ContentFormat, persona string, cfg *models.AgentConfig) ([]GeneratedQuestion, error) {
```

RAG search แยกตาม format (แทนบรรทัด `ragResults, err := a.rag.Search(...)` เดิม):

```go
	var ragResults []rag.SearchResult
	var err error
	if format.FormatName == "news" {
		// News format: only use chunks crawled in the last 7 days
		ragResults, err = a.rag.SearchRecent(ctx, fmt.Sprintf("Facebook Ads Meta update news %s", category), 5, 7)
	} else {
		ragResults, err = a.rag.Search(ctx, fmt.Sprintf("Facebook Ads %s %s", category, format.FormatName), 5)
	}
	if err != nil {
		return nil, fmt.Errorf("RAG search: %w", err)
	}
```

Template data:

```go
	userPrompt, err := renderTemplate(cfg.PromptTemplate, QuestionTemplateData{
		Count:             count,
		Category:          category,
		RAGContext:        ragContext.String(),
		PreviousTopics:    previousList,
		PreviousNames:     previousNames,
		FormatInstruction: format.QuestionInstruction,
		AudiencePersona:   persona,
	})
```

(import `"github.com/jaochai/video-fb/internal/models"` มีอยู่แล้ว)

- [ ] **Step 2: แก้ ScriptTemplateData + Generate signature**

```go
type ScriptTemplateData struct {
	Question          string
	QuestionerName    string
	Category          string
	RAGContext        string
	FormatInstruction string
	AudiencePersona   string
}
```

```go
func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category string, format *models.ContentFormat, persona string, cfg *models.AgentConfig) (*GeneratedScript, error) {
```

Template data:

```go
	userPrompt, err := renderTemplate(cfg.PromptTemplate, ScriptTemplateData{
		Question:          question,
		QuestionerName:    questionerName,
		Category:          category,
		RAGContext:        ragContext.String(),
		FormatInstruction: format.ScriptInstruction,
		AudiencePersona:   persona,
	})
```

- [ ] **Step 3: Build (จะ fail ที่ orchestrator — แก้ใน Task 12)**

Run: `go build ./internal/agent/`
Expected: agent package ผ่าน (orchestrator ยัง fail — เป็นเรื่องปกติ แก้ task ถัดไป)

- [ ] **Step 4: Commit**

```bash
git add internal/agent/question.go internal/agent/script.go
git commit -m "feat(agent): format-aware generation with audience persona"
```

---

### Task 12: Orchestrator — เลือก format + persona + ส่งให้ agents

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`
- Modify: `internal/models/request.go`
- Modify: `internal/repository/clips.go`
- Modify: `cmd/server/main.go`
- Modify: `internal/scheduler/scheduler.go` (ถ้าจำเป็น)

- [ ] **Step 1: เพิ่ม formatsRepo ใน Orchestrator struct + constructor**

```go
type Orchestrator struct {
	settingsRepo  *repository.SettingsRepo
	formatsRepo   *repository.FormatsRepo
	questionAgent *agent.QuestionAgent
	scriptAgent   *agent.ScriptAgent
	imageAgent    *agent.ImageAgent
	producer      *producer.Producer
	clipsRepo     *repository.ClipsRepo
	scenesRepo    *repository.ScenesRepo
	themesRepo    *repository.ThemesRepo
	agentsRepo    *repository.AgentsRepo
	tracker       *progress.Tracker
}
```

constructor เพิ่ม parameter `formats *repository.FormatsRepo` และ field `formatsRepo: formats`

- [ ] **Step 2: แก้ ProduceWeekly — เลือก format + ดึง persona**

หลังจากเลือก category (บรรทัด `category := categories[weekNum%len(categories)]`):

```go
	format, err := o.formatsRepo.PickNext(ctx)
	if err != nil {
		return fmt.Errorf("pick content format: %w", err)
	}

	persona, err := o.settingsRepo.Get(ctx, "audience_persona")
	if err != nil {
		log.Printf("Warning: audience_persona not set, using empty: %v", err)
		persona = ""
	}

	log.Printf("Producing %d clips — category: %s, format: %s", count, category, format.DisplayName)
```

แก้การเรียก questionAgent:

```go
	questions, err := o.questionAgent.Generate(ctx, count, category, format, persona, qaCfg)
```

แก้การเรียก produceClip ให้ส่ง format + persona ต่อ:

```go
		if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format, persona); err != nil {
```

- [ ] **Step 3: แก้ produceClip / produceClipWithID signatures**

```go
func (o *Orchestrator) produceClip(ctx context.Context, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string, format *models.ContentFormat, persona string) error {
	today := time.Now().Format("2006-01-02")
	clip, err := o.clipsRepo.Create(ctx, models.CreateClipRequest{
		Title:          q.Question,
		Question:       q.Question,
		QuestionerName: q.QuestionerName,
		Category:       q.Category,
		PublishDate:    &today,
		ContentFormat:  format.FormatName,
	})
	...
	return o.produceClipWithID(ctx, clip.ID, q, theme, scriptCfg, imageCfg, brandAliases, format, persona)
}

func (o *Orchestrator) produceClipWithID(ctx context.Context, clipID string, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string, format *models.ContentFormat, persona string) error {
	o.tracker.StartStep("script")
	script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, format, persona, scriptCfg)
	...
```

**สำคัญ:** หา callers อื่นของ `produceClipWithID` / `produceClip` (เช่น `RetryClip`, `RetryAllFailed`, handler) ด้วย `grep -rn "produceClipWithID\|produceClip(" internal/` แล้วแก้ให้ส่ง format default + persona:

```go
	// In RetryClip and other callers that lack format context, fall back to qa:
	format, err := o.formatsRepo.PickNext(ctx)
	if err != nil {
		format = &models.ContentFormat{FormatName: "qa"}
	}
	persona, _ := o.settingsRepo.Get(ctx, "audience_persona")
```

- [ ] **Step 4: เพิ่ม ContentFormat ใน CreateClipRequest + clips repo**

`internal/models/request.go`:

```go
type CreateClipRequest struct {
	Title          string  `json:"title"`
	Question       string  `json:"question"`
	QuestionerName string  `json:"questioner_name"`
	Category       string  `json:"category"`
	PublishDate    *string `json:"publish_date"`
	ContentFormat  string  `json:"content_format"`
}
```

`internal/repository/clips.go` Create():

```go
	c, err := scanClip(r.pool.QueryRow(ctx,
		`INSERT INTO clips (title, question, questioner_name, category, publish_date, content_format)
		 VALUES ($1, $2, $3, $4, $5::date, COALESCE(NULLIF($6, ''), 'qa'))
		 RETURNING `+clipColumns,
		req.Title, req.Question, req.QuestionerName, req.Category, req.PublishDate, req.ContentFormat,
	))
```

(ไม่ต้องเพิ่ม content_format ใน clipColumns/scanClip/Clip model — YAGNI จนกว่า frontend ต้องแสดง)

- [ ] **Step 5: แก้ main.go wiring**

```go
	formatsRepo := repository.NewFormatsRepo(pool)

	orch := orchestrator.New(questionAgent, scriptAgent, imageAgent, prod,
		clipsRepo, scenesRepo, themesRepo, agentsRepo, settingsRepo, formatsRepo, tracker)
```

- [ ] **Step 6: Build + test ทั้งหมด**

Run: `go build ./... && go test ./...`
Expected: ผ่านทั้งหมด

- [ ] **Step 7: Commit**

```bash
git add internal/ cmd/ migrations/
git commit -m "feat(orchestrator): content format rotation + audience persona in production flow"
```

---

### Task 13: Frontend — แสดง insights ใน Agents page

**Files:**
- Modify: `frontend/src/pages/Agents.tsx` (อ่านโครงสร้างก่อนแก้)

- [ ] **Step 1: อ่าน Agents.tsx เพื่อหา pattern การแสดง skills**

Run: `grep -n "skills" frontend/src/pages/Agents.tsx`

- [ ] **Step 2: เพิ่มการแสดง insights แบบ read-only**

ตาม pattern ที่เจอ — เพิ่ม section "Performance Insights (auto-tuned)" แสดง `agent.insights` แบบ read-only (ไม่มี input แก้ไข) ใต้ส่วน skills พร้อม badge บอกว่า "ปรับอัตโนมัติโดย weekly analyzer"

- [ ] **Step 3: Build frontend**

Run: `cd frontend && npm run build`
Expected: ผ่าน

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Agents.tsx
git commit -m "feat(ui): show auto-tuned insights read-only on Agents page"
```

---

### Task 14: Final verification + deploy prep

- [ ] **Step 1: รัน build + test ทั้งหมด**

Run: `go build ./... && go test ./... && cd frontend && npm run build && cd ..`
Expected: ทุกอย่างผ่าน

- [ ] **Step 2: ตรวจว่า migration เรียงถูกและ idempotent**

Run: `ls migrations/ | tail -5`
Expected: 016, 017 อยู่ท้ายสุด

- [ ] **Step 3: Agent team review** (ตาม global-agents.md: Go + React + DB → go review + typescript review + database review)

- [ ] **Step 4: สรุปผลให้ user + แนะนำขั้นตอน deploy**

หมายเหตุ: migrations จะรันอัตโนมัติตอน server start (`database.RunMigrations` ใน main.go) — deploy ไป Railway แล้ว migration 016+017 จะ apply เอง
