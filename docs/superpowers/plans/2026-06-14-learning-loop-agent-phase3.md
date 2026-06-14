# Learning Loop Agent (Phase 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the loop on Phase 1's `clip_critiques` quality signal: a meta-agent ("learner") reads accumulated critiques, finds recurring low-score / repeated-issue patterns, generates improved `skills` text for the relevant upstream agent, and **automatically applies** it to that agent's `agent_configs.skills` — behind hard guardrails so every change is bounded, logged, and revertable.

**Architecture:** A new `LearnerAgent` (same pattern as `SceneAgent`/`CriticAgent`) proposes improved skills text from aggregated critique patterns. A `Learner` service (`internal/learner`) orchestrates: for each allowlisted upstream agent (`scene`, `script`), it aggregates recent critiques via a new `CritiquesRepo.LowScorePatterns` read, applies a pure **strong-signal gate** (enough critiques AND a dimension below threshold), asks the `LearnerAgent` to propose, runs a pure `AcceptProposal` validation, and — only on accept — writes an append-only `skill_revisions` audit row (old+new+rationale) **then** calls the existing `AgentsRepo.UpdateSkillsByName`. Auto-apply is risky, so the design is: allowlist-only agents, never blank skills, strong-signal gate, append-only audit table that makes any change revertable, and a log line on every action and every skip. A `-learn` CLI flag runs the loop once; a weekly `schedules` row runs it on cron.

**Tech Stack:** Go, pgx/Postgres (Neon), the existing `KieLLMClient.GenerateJSON` LLM wrapper, DB-driven `agent_configs`, existing `CritiquesRepo` + `AgentsRepo`.

> **Migration numbers are provisional.** This plan uses `037` (Phase 2 reserves `035`/`036`). If implemented while the repo's latest migration has moved, renumber to the next unused number — migrations are tracked by filename, gaps are harmless.

---

## File Structure

- **Create** `migrations/037_skill_revisions.sql` — append-only audit table + seed the `learner` agent_configs row + weekly `learn` schedule.
- **Modify** `internal/repository/critiques.go` — add `LowScorePatterns` read/aggregation method + result structs.
- **Create** `internal/repository/critiques_test.go` — pure unit test for the in-Go post-processing helper.
- **Create** `internal/learner/mapping.go` — pure `agentForField(field string) string`.
- **Create** `internal/learner/mapping_test.go` — table-driven tests for `agentForField`.
- **Create** `internal/agent/learner.go` — `LearnerAgent`, input/output types, `Propose`, pure `AcceptProposal`.
- **Create** `internal/agent/learner_test.go` — pure tests for `AcceptProposal` + schema unmarshal.
- **Create** `internal/learner/learner.go` — `Learner` service, pure `strongSignal` gate, `RunOnce`.
- **Create** `internal/learner/learner_test.go` — pure tests for `strongSignal`.
- **Modify** `cmd/server/main.go` — add `-learn` flag, construct the `Learner`, dispatch.

**Guardrail boundary (locked):** the learner may only ever touch agents in `allowedAgents = {"scene", "script"}`. It never deletes or blanks skills (`AcceptProposal` rejects empty/blank `NewSkills`). Every applied change writes a `skill_revisions` row BEFORE the `UpdateSkillsByName`, so the audit table is a complete, append-only, revertable history.

---

## Task 1: Migration — `skill_revisions` audit table + `learner` agent + weekly schedule

**Files:**
- Create: `migrations/037_skill_revisions.sql`

- [ ] **Step 1: Write the migration**

`035` is the next number (latest on disk is `034_critic_agent_config.sql`). Migrations auto-run on every startup via `database.RunMigrations`; all statements are idempotent (`IF NOT EXISTS` / `WHERE NOT EXISTS`). The `agent_configs` INSERT mirrors migration 030's column set `(agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)`; `insights`/`config` take their table defaults. `model` is `claude-sonnet-4-6` (strong text, routed by `KieLLMClient` prefix). The `schedules` INSERT mirrors migration 032's pattern.

```sql
-- 037_skill_revisions.sql
-- Phase 3 learning loop. Append-only audit of every automatic skills change the
-- learner makes to an upstream agent. Each row stores the FULL old + new skills
-- text plus a rationale and the critique window, so any auto-applied change is
-- fully revertable by hand:
--   UPDATE agent_configs SET skills = (SELECT old_skills FROM skill_revisions
--     WHERE agent_name = '<name>' ORDER BY created_at DESC LIMIT 1)
--   WHERE agent_name = '<name>';
-- Kill switch for the whole loop:
--   UPDATE agent_configs SET enabled = FALSE WHERE agent_name = 'learner';
CREATE TABLE IF NOT EXISTS skill_revisions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_name     TEXT NOT NULL,
    old_skills     TEXT NOT NULL,
    new_skills     TEXT NOT NULL,
    rationale      TEXT NOT NULL DEFAULT '',
    critique_window INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_skill_revisions_agent ON skill_revisions (agent_name, created_at DESC);

-- Seed the learner meta-agent. It reads recurring quality issues and rewrites an
-- upstream agent's skills guidelines. Output is the improved skills TEXT only.
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
SELECT
  'learner',
  'คุณคือ Learner ของ Ads Vance — โค้ชที่ปรับปรุง "skills guidelines" ของ agent ต้นทาง (เช่น scene, script) จากปัญหาคุณภาพที่เกิดซ้ำ ๆ ในงานจริง.

ระบบจะส่งให้คุณ:
- ชื่อ agent ที่กำลังปรับ
- skills guidelines ปัจจุบันของ agent นั้น
- สรุปปัญหาที่เกิดซ้ำ: คะแนนเฉลี่ยรายมิติ (hook/clarity/brand_fit/overall) + เหตุผลที่ critic แก้บ่อยที่สุด

หน้าที่ของคุณ:
- ออกแบบ skills guidelines ฉบับปรับปรุงที่ "แก้ที่ต้นเหตุ" ของปัญหาที่เกิดซ้ำ.
- เก็บของเดิมที่ยังดีไว้ เพิ่ม/แก้เฉพาะส่วนที่จำเป็น (ไม่รื้อทิ้งทั้งหมด).
- เขียนเป็น bullet สั้น กระชับ สั่งการได้จริง ภาษาไทยแบบที่ทีมใช้.

ข้อห้ามเด็ดขาด:
- ห้ามคืน skills ว่างหรือมีแต่ช่องว่าง.
- ถ้าของเดิมดีพออยู่แล้วและปัญหาไม่ชัด ให้ confident=false แล้วคืนของเดิม.
- ตอบเป็น JSON object เท่านั้น.',
  'agent ที่กำลังปรับ: {{.AgentName}}

skills ปัจจุบัน:
{{.CurrentSkills}}

สรุปปัญหาที่เกิดซ้ำ (จาก clip_critiques ช่วง {{.WindowDays}} วันล่าสุด):
{{.PatternSummary}}

จงคืน JSON object รูปแบบนี้เท่านั้น (ห้ามมีข้อความอื่นนอก JSON):
{
  "new_skills": "บทปรับปรุง skills แบบ bullet ภาษาไทย (ห้ามว่าง)",
  "rationale": "อธิบายสั้น ๆ ว่าแก้อะไรเพราะปัญหาอะไร",
  "confident": true
}',
  'claude-sonnet-4-6',
  0.3,
  TRUE,
  '- แก้ที่ต้นเหตุของปัญหาที่เกิดซ้ำ ไม่ใช่ปลายเหตุ.
- เก็บ guideline เดิมที่ยังได้ผลไว้ เพิ่มเฉพาะที่ขาด.
- เขียนสั้น สั่งการได้จริง วัดผลได้.'
WHERE NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'learner');

-- Weekly learning loop: every Monday 03:00 (DB server time). action = 'learn'.
INSERT INTO schedules (name, cron_expression, action, enabled)
SELECT 'Weekly Learn', '0 3 * * 1', 'learn', TRUE
WHERE NOT EXISTS (SELECT 1 FROM schedules WHERE action = 'learn');
```

> **verify at implementation time:** confirm the `schedules` table has exactly the columns `(name, cron_expression, action, enabled)` by reading `migrations/032_tiktok_publishing.sql` (it uses this same INSERT shape). If the scheduler that reads `schedules` does not yet dispatch the `'learn'` action to the `-learn` path, that wiring is out of scope here — the `-learn` flag (Task 6) is the guaranteed entry point; the cron row is best-effort and harmless if unread.

- [ ] **Step 2: Commit**

```bash
git add migrations/037_skill_revisions.sql
git commit -m "feat(db): skill_revisions audit table + learner agent + weekly schedule"
```

(The migration auto-applies on next startup via `database.RunMigrations`; no code depends on the table existing at build/test time.)

---

## Task 2: Aggregation read — `CritiquesRepo.LowScorePatterns`

**Files:**
- Modify: `internal/repository/critiques.go`
- Test: `internal/repository/critiques_test.go`

- [ ] **Step 1: Add the result structs + the query method**

`clip_critiques.score` is `{"hook":int,"clarity":int,"brand_fit":int,"overall":int}` and `changes` is an array of `{"field":string,"reason":string}`. The method returns the per-dimension averages and the overall critique count over the recent window, plus the top recurring `field`+`reason` pairs. Averages come from `jsonb ->> 'key'` cast to numeric; the top issues come from `jsonb_array_elements(changes)` unnested and grouped. Append to `internal/repository/critiques.go`:

```go
// FieldIssue is one recurring critic edit (a changes[] entry) and how often it
// occurred in the window.
type FieldIssue struct {
	Field  string
	Reason string
	Count  int
}

// ScorePatterns is the aggregated quality signal over a recent critique window.
// Avg* are the mean of each score dimension (0 when N == 0). N is how many
// critique rows fell in the window. TopIssues are the most common changes[]
// field+reason pairs, most frequent first.
type ScorePatterns struct {
	N           int
	AvgHook     float64
	AvgClarity  float64
	AvgBrandFit float64
	AvgOverall  float64
	TopIssues   []FieldIssue
}

// LowScorePatterns aggregates clip_critiques over the last sinceDays days:
// per-dimension score averages + count, plus the most common changes[] entries.
// topN caps how many recurring issues are returned (0 or negative -> 10).
func (r *CritiquesRepo) LowScorePatterns(ctx context.Context, sinceDays, topN int) (ScorePatterns, error) {
	if topN <= 0 {
		topN = 10
	}
	var p ScorePatterns

	// Per-dimension averages + count over the window.
	err := r.pool.QueryRow(ctx, `
SELECT
  COUNT(*)                                                       AS n,
  COALESCE(AVG((score->>'hook')::numeric),      0)              AS avg_hook,
  COALESCE(AVG((score->>'clarity')::numeric),   0)              AS avg_clarity,
  COALESCE(AVG((score->>'brand_fit')::numeric), 0)              AS avg_brand_fit,
  COALESCE(AVG((score->>'overall')::numeric),   0)              AS avg_overall
FROM clip_critiques
WHERE created_at >= NOW() - make_interval(days => $1)`,
		sinceDays,
	).Scan(&p.N, &p.AvgHook, &p.AvgClarity, &p.AvgBrandFit, &p.AvgOverall)
	if err != nil {
		return ScorePatterns{}, fmt.Errorf("aggregate score patterns: %w", err)
	}

	if p.N == 0 {
		return p, nil
	}

	// Most common changes[] field+reason pairs over the same window.
	rows, err := r.pool.Query(ctx, `
SELECT
  c->>'field'  AS field,
  c->>'reason' AS reason,
  COUNT(*)     AS cnt
FROM clip_critiques cc,
     LATERAL jsonb_array_elements(cc.changes) AS c
WHERE cc.created_at >= NOW() - make_interval(days => $1)
  AND c->>'field' IS NOT NULL
GROUP BY c->>'field', c->>'reason'
ORDER BY cnt DESC, field ASC
LIMIT $2`,
		sinceDays, topN,
	)
	if err != nil {
		return ScorePatterns{}, fmt.Errorf("aggregate top issues: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var fi FieldIssue
		if err := rows.Scan(&fi.Field, &fi.Reason, &fi.Count); err != nil {
			return ScorePatterns{}, fmt.Errorf("scan top issue: %w", err)
		}
		p.TopIssues = append(p.TopIssues, fi)
	}
	if err := rows.Err(); err != nil {
		return ScorePatterns{}, fmt.Errorf("iterate top issues: %w", err)
	}
	return p, nil
}
```

Add `"fmt"` to the import block of `internal/repository/critiques.go` (currently it imports only `"context"` and the pgxpool package):

```go
import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)
```

> **verify at implementation time:** `make_interval(days => $1)` is standard Postgres (Neon supports it). The `score->>'key'` operator requires `score` to be a JSON object (it always is per the Phase 1 contract). `LowScorePatterns` needs a real DB to test end-to-end (no live DB at build/test time), so it has no Go unit test of its own — the pure unit test below covers the only in-Go post-processing helper.

- [ ] **Step 2: Add a pure post-processing helper + its failing test**

The strong-signal gate (Task 5) needs "the single lowest dimension and its value" from a `ScorePatterns`. Put that pure helper here next to the struct so it is unit-testable without a DB. Append to `internal/repository/critiques.go`:

```go
// LowestDimension returns the name and average of the weakest score dimension.
// Pure helper over already-aggregated data (no DB) so the strong-signal gate is
// testable. On N == 0 it returns ("", 0).
func (p ScorePatterns) LowestDimension() (string, float64) {
	if p.N == 0 {
		return "", 0
	}
	dims := []struct {
		name string
		val  float64
	}{
		{"hook", p.AvgHook},
		{"clarity", p.AvgClarity},
		{"brand_fit", p.AvgBrandFit},
		{"overall", p.AvgOverall},
	}
	lowName, lowVal := dims[0].name, dims[0].val
	for _, d := range dims[1:] {
		if d.val < lowVal {
			lowName, lowVal = d.name, d.val
		}
	}
	return lowName, lowVal
}
```

Create `internal/repository/critiques_test.go`:

```go
package repository

import "testing"

func TestLowestDimension_PicksWeakest(t *testing.T) {
	p := ScorePatterns{N: 5, AvgHook: 4.2, AvgClarity: 7.0, AvgBrandFit: 8.1, AvgOverall: 6.5}
	name, val := p.LowestDimension()
	if name != "hook" {
		t.Errorf("name = %q, want hook", name)
	}
	if val != 4.2 {
		t.Errorf("val = %v, want 4.2", val)
	}
}

func TestLowestDimension_EmptyWindow(t *testing.T) {
	name, val := ScorePatterns{N: 0}.LowestDimension()
	if name != "" || val != 0 {
		t.Errorf("empty window: got (%q, %v), want (\"\", 0)", name, val)
	}
}

func TestLowestDimension_TieKeepsFirst(t *testing.T) {
	p := ScorePatterns{N: 3, AvgHook: 5.0, AvgClarity: 5.0, AvgBrandFit: 5.0, AvgOverall: 5.0}
	name, _ := p.LowestDimension()
	if name != "hook" {
		t.Errorf("tie should keep first (hook), got %q", name)
	}
}
```

- [ ] **Step 3: Run the tests to verify they pass**

Run: `go test ./internal/repository/ -run TestLowestDimension -v`
Expected: PASS (3 tests). The aggregation SQL is not exercised here (needs a DB); only the pure helper is tested.

- [ ] **Step 4: Verify the package builds**

Run: `go build ./internal/repository/`
Expected: build OK.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/critiques.go internal/repository/critiques_test.go
git commit -m "feat(repo): CritiquesRepo.LowScorePatterns aggregation + LowestDimension helper"
```

---

## Task 3: Field→agent mapping (pure, TDD)

**Files:**
- Create: `internal/learner/mapping.go`
- Test: `internal/learner/mapping_test.go`

- [ ] **Step 1: Write the pure mapping function**

`changes[].field` looks like `"scene[0].voice_text"`, `"scene[2].image_prompt"`, `"metadata.youtube_title"`, etc. This pure function decides which upstream agent owns the field — and therefore whose skills the learner tunes. Scene-content fields map to `"scene"`; metadata/title/description/tags map to `"script"`; anything else maps to `""` (unowned, ignored by the gate). Create `internal/learner/mapping.go`:

```go
package learner

import "strings"

// agentForField maps a clip_critiques changes[].field path to the upstream
// agent whose skills own that field. Returns "" for fields no allowlisted agent
// owns (those are ignored by the strong-signal gate). Pure.
//
// Scene content (voice_text / on_screen_text / image_prompt / text_content /
// emphasis...) belongs to the `scene` agent. Metadata (youtube_title /
// description / tags, or any metadata.* path) belongs to the `script` agent.
func agentForField(field string) string {
	f := strings.ToLower(strings.TrimSpace(field))
	if f == "" {
		return ""
	}

	// Metadata-owned fields (script agent).
	if strings.HasPrefix(f, "metadata.") ||
		strings.Contains(f, "title") ||
		strings.Contains(f, "desc") ||
		strings.Contains(f, "tags") {
		return "script"
	}

	// Scene-content fields (scene agent). Match either the "scene[" prefix or any
	// of the known scene content sub-fields.
	if strings.HasPrefix(f, "scene[") || strings.HasPrefix(f, "scene.") || f == "scene" {
		return "scene"
	}
	for _, k := range []string{"voice_text", "on_screen_text", "image_prompt", "text_content", "emphasis"} {
		if strings.Contains(f, k) {
			return "scene"
		}
	}

	return ""
}
```

- [ ] **Step 2: Write the table-driven failing tests**

Create `internal/learner/mapping_test.go`:

```go
package learner

import "testing"

func TestAgentForField(t *testing.T) {
	cases := []struct {
		field string
		want  string
	}{
		// scene content
		{"scene[0].voice_text", "scene"},
		{"scene[2].image_prompt", "scene"},
		{"scene[1].on_screen_text", "scene"},
		{"scene[3].text_content", "scene"},
		{"scene[0].emphasis_words", "scene"},
		{"voice_text", "scene"},
		{"on_screen_text", "scene"},
		{"image_prompt", "scene"},
		{"text_content", "scene"},
		{"emphasis", "scene"},
		{"scene", "scene"},
		{"SCENE[0].VOICE_TEXT", "scene"}, // case-insensitive

		// metadata -> script
		{"metadata.youtube_title", "script"},
		{"metadata.youtube_description", "script"},
		{"metadata.youtube_tags", "script"},
		{"youtube_title", "script"},
		{"youtube_description", "script"},
		{"youtube_tags", "script"},
		{"title", "script"},
		{"desc", "script"},
		{"tags", "script"},

		// unknown / unowned
		{"", ""},
		{"   ", ""},
		{"duration_seconds", ""},
		{"layout_variant", ""},
		{"random.field", ""},
	}
	for _, c := range cases {
		if got := agentForField(c.field); got != c.want {
			t.Errorf("agentForField(%q) = %q, want %q", c.field, got, c.want)
		}
	}
}
```

- [ ] **Step 3: Run the tests to verify they pass**

Run: `go test ./internal/learner/ -run TestAgentForField -v`
Expected: PASS. `agentForField` is pure, no DB or LLM needed.

- [ ] **Step 4: Commit**

```bash
git add internal/learner/mapping.go internal/learner/mapping_test.go
git commit -m "feat(learner): pure agentForField mapping + table-driven tests"
```

---

## Task 4: LearnerAgent types + `Propose` + pure `AcceptProposal` (TDD)

**Files:**
- Create: `internal/agent/learner.go`
- Test: `internal/agent/learner_test.go`

- [ ] **Step 1: Write the types, the pure validation, and `Propose`**

Same agent pattern as `SceneAgent`/`CriticAgent`: `type LearnerAgent struct { llm *KieLLMClient }`, constructor, one method that calls `a.llm.GenerateJSON(...)` with `renderTemplate(cfg.PromptTemplate, data)`. The pure `AcceptProposal` is the guardrail (exported so the learner service can call it across packages): it rejects empty/blank `NewSkills`, a non-confident proposal, or skills identical to the current text (after trimming). Create `internal/agent/learner.go`:

```go
package agent

import (
	"context"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)

// LearnInput is one upstream agent's current skills plus a human-readable summary
// of the recurring critique patterns the learner should fix. PatternSummary is
// built by the Learner service from CritiquesRepo.LowScorePatterns; the agent
// package stays decoupled from the repository layer.
type LearnInput struct {
	AgentName     string
	CurrentSkills string
	Patterns      string // pre-rendered pattern summary (scores + top issues)
	WindowDays    int
}

// LearnOutput is the raw JSON the learner LLM returns.
type LearnOutput struct {
	NewSkills string `json:"new_skills"`
	Rationale string `json:"rationale"`
	Confident bool   `json:"confident"`
}

// learnerTemplateData fills the seeded `learner` prompt_template.
type learnerTemplateData struct {
	AgentName      string
	CurrentSkills  string
	PatternSummary string
	WindowDays     int
}

// LearnerAgent proposes improved skills guidelines for an upstream agent from
// recurring quality issues. Runs on Claude (cfg.Model is claude-sonnet-4-6).
type LearnerAgent struct {
	llm *KieLLMClient
}

func NewLearnerAgent(llm *KieLLMClient) *LearnerAgent {
	return &LearnerAgent{llm: llm}
}

// Propose asks the LLM for improved skills text. cfg is the `learner` AgentConfig
// fetched by the caller via GetByName. It returns the raw proposal; the caller
// MUST gate it through AcceptProposal before applying anything.
func (a *LearnerAgent) Propose(ctx context.Context, in LearnInput, cfg *models.AgentConfig) (*LearnOutput, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, learnerTemplateData{
		AgentName:      in.AgentName,
		CurrentSkills:  in.CurrentSkills,
		PatternSummary: in.Patterns,
		WindowDays:     in.WindowDays,
	})
	if err != nil {
		return nil, err
	}

	var out LearnOutput
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AcceptProposal is the guardrail deciding whether a proposal may be applied.
// Exported so the learner service (a different package) can gate on it. Pure.
// Rejects when: the LLM is not confident, the new skills are empty/blank, or the
// new skills are identical to the current text (after trimming). This is what
// guarantees the loop can never blank or no-op an agent's skills.
func AcceptProposal(in LearnInput, out *LearnOutput) bool {
	if out == nil || !out.Confident {
		return false
	}
	next := strings.TrimSpace(out.NewSkills)
	if next == "" {
		return false
	}
	if next == strings.TrimSpace(in.CurrentSkills) {
		return false
	}
	return true
}
```

- [ ] **Step 2: Write the failing tests**

Create `internal/agent/learner_test.go`:

```go
package agent

import (
	"encoding/json"
	"testing"
)

func TestAcceptProposal_Accepts(t *testing.T) {
	in := LearnInput{CurrentSkills: "- เดิม"}
	out := &LearnOutput{NewSkills: "- เดิม\n- เพิ่ม hook ตัวเลขช็อก", Rationale: "hook ต่ำ", Confident: true}
	if !AcceptProposal(in, out) {
		t.Fatal("AcceptProposal = false, want true for a confident, non-empty, changed proposal")
	}
}

func TestAcceptProposal_RejectsNotConfident(t *testing.T) {
	in := LearnInput{CurrentSkills: "- เดิม"}
	out := &LearnOutput{NewSkills: "- ใหม่", Confident: false}
	if AcceptProposal(in, out) {
		t.Fatal("AcceptProposal = true, want false when not confident")
	}
}

func TestAcceptProposal_RejectsEmpty(t *testing.T) {
	in := LearnInput{CurrentSkills: "- เดิม"}
	for _, blank := range []string{"", "   ", "\n\t  "} {
		out := &LearnOutput{NewSkills: blank, Confident: true}
		if AcceptProposal(in, out) {
			t.Fatalf("AcceptProposal = true, want false for blank new_skills %q", blank)
		}
	}
}

func TestAcceptProposal_RejectsIdentical(t *testing.T) {
	in := LearnInput{CurrentSkills: "  - เดิม  "}
	out := &LearnOutput{NewSkills: "- เดิม", Confident: true}
	if AcceptProposal(in, out) {
		t.Fatal("AcceptProposal = true, want false when new == current (after trim)")
	}
}

func TestAcceptProposal_RejectsNil(t *testing.T) {
	if AcceptProposal(LearnInput{CurrentSkills: "x"}, nil) {
		t.Fatal("AcceptProposal = true, want false for nil output")
	}
}

// Locks the prompt<->struct contract: the JSON the learner is told to emit must
// unmarshal cleanly into LearnOutput.
func TestLearnOutputParsesSchema(t *testing.T) {
	raw := `{ "new_skills": "- ปรับ hook", "rationale": "hook ต่ำสุด", "confident": true }`
	var out LearnOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("LearnOutput did not unmarshal: %v", err)
	}
	if !out.Confident || out.NewSkills != "- ปรับ hook" || out.Rationale != "hook ต่ำสุด" {
		t.Errorf("unexpected parse: %+v", out)
	}
}
```

- [ ] **Step 3: Run the tests to verify they pass**

Run: `go test ./internal/agent/ -run 'TestAcceptProposal|TestLearnOutput' -v`
Expected: PASS (6 tests). `AcceptProposal` is pure; no LLM mock needed.

- [ ] **Step 4: Verify the package builds**

Run: `go build ./internal/agent/`
Expected: build OK.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/learner.go internal/agent/learner_test.go
git commit -m "feat(agent): LearnerAgent.Propose + pure AcceptProposal guardrail"
```

---

## Task 5: Guardrailed apply — `Learner` service + pure strong-signal gate (TDD)

**Files:**
- Create: `internal/learner/learner.go`
- Test: `internal/learner/learner_test.go`

- [ ] **Step 1: Write the strong-signal gate (pure) + the service**

The gate and the constants live with the service. `RunOnce` loops over the allowlist, aggregates, gates, proposes, validates, and — only on accept — writes the audit row BEFORE `UpdateSkillsByName`. It logs every action and every skip reason. Create `internal/learner/learner.go`:

```go
package learner

import (
	"context"
	"fmt"
	"log"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

// Guardrail constants. The loop only acts on a STRONG signal: enough critiques in
// the window AND a score dimension averaging below the threshold. Tuned
// conservatively because changes auto-apply.
const (
	// windowDays is how far back LowScorePatterns aggregates.
	windowDays = 30
	// minCritiques is the minimum critique rows in the window before we act.
	minCritiques = 8
	// lowScoreThreshold: a dimension must average strictly below this (1-10
	// scale) to count as a real, recurring weakness worth a skills change.
	lowScoreThreshold = 6.0
	// topIssuesN caps how many recurring issues feed the pattern summary.
	topIssuesN = 8
)

// allowedAgents is the FIXED allowlist of agents the learner may ever touch.
// Auto-apply is restricted to these names; nothing else can be modified.
var allowedAgents = []string{"scene", "script"}

// SkillRevisionsWriter is the append-only audit sink. Implemented in Task 6 by a
// tiny repo; declared here so the service depends on a narrow interface.
type SkillRevisionsWriter interface {
	Record(ctx context.Context, agentName, oldSkills, newSkills, rationale string, critiqueWindow int) error
}

// agentsRepoIface is the subset of *repository.AgentsRepo the Learner needs.
// *repository.AgentsRepo already satisfies this exactly.
type agentsRepoIface interface {
	GetByName(ctx context.Context, name string) (*models.AgentConfig, error)
	UpdateSkillsByName(ctx context.Context, agentName, newSkills string) error
}

// critiquesRepoIface is the subset of *repository.CritiquesRepo the Learner needs.
type critiquesRepoIface interface {
	LowScorePatterns(ctx context.Context, sinceDays, topN int) (repository.ScorePatterns, error)
}

// learnerAgentIface is the subset of *agent.LearnerAgent the Learner needs.
type learnerAgentIface interface {
	Propose(ctx context.Context, in agent.LearnInput, cfg *models.AgentConfig) (*agent.LearnOutput, error)
}

// strongSignal is the pure gate: act only when there are enough critiques AND the
// weakest dimension is below threshold. Returns (ok, weakest-dimension-name,
// weakest-value) so the caller can log exactly why it acted or skipped.
func strongSignal(p repository.ScorePatterns) (bool, string, float64) {
	name, val := p.LowestDimension()
	if p.N < minCritiques {
		return false, name, val
	}
	if val >= lowScoreThreshold {
		return false, name, val
	}
	return true, name, val
}
```

> **Type note:** the interfaces use the real `*models.AgentConfig` (the type `AgentsRepo.GetByName` returns and `LearnerAgent.Propose` consumes). `*repository.AgentsRepo`, `*repository.CritiquesRepo`, and `*agent.LearnerAgent` each satisfy their respective interface method set exactly, so the concrete types are passed directly with no adapter. There is no import cycle: `internal/learner` imports `agent`, `models`, and `repository`; none of those import `learner`.

Append the service struct + `RunOnce` to `internal/learner/learner.go`:

```go
// Learner runs the guardrailed auto-apply loop.
type Learner struct {
	agents    agentsRepoIface
	critiques critiquesRepoIface
	llmAgent  learnerAgentIface
	audit     SkillRevisionsWriter
}

func New(
	agents agentsRepoIface,
	critiques critiquesRepoIface,
	llmAgent learnerAgentIface,
	audit SkillRevisionsWriter,
) *Learner {
	return &Learner{agents: agents, critiques: critiques, llmAgent: llmAgent, audit: audit}
}

// RunOnce executes one pass: for each allowlisted agent, aggregate recent
// critiques, apply the strong-signal gate, ask the learner to propose, validate,
// and — only on accept — write an audit row THEN update the agent's skills. Never
// fatal: a failure on one agent is logged and the loop continues.
func (l *Learner) RunOnce(ctx context.Context) error {
	learnerCfg, err := l.agents.GetByName(ctx, "learner")
	if err != nil {
		return fmt.Errorf("learner agent config: %w", err)
	}
	if !learnerCfg.Enabled {
		log.Printf("learner: disabled (agent_configs['learner'].enabled = false); skipping run")
		return nil
	}

	for _, name := range allowedAgents {
		patterns, err := l.critiques.LowScorePatterns(ctx, windowDays, topIssuesN)
		if err != nil {
			log.Printf("learner: [%s] aggregate failed (skip): %v", name, err)
			continue
		}

		ok, lowDim, lowVal := strongSignal(patterns)
		if !ok {
			log.Printf("learner: [%s] skip — weak signal (n=%d weakest=%s avg=%.2f; need n>=%d and avg<%.1f)",
				name, patterns.N, lowDim, lowVal, minCritiques, lowScoreThreshold)
			continue
		}

		target, err := l.agents.GetByName(ctx, name)
		if err != nil {
			log.Printf("learner: [%s] config not found (skip): %v", name, err)
			continue
		}

		in := agent.LearnInput{
			AgentName:     name,
			CurrentSkills: target.Skills,
			Patterns:      formatPatterns(patterns),
			WindowDays:    windowDays,
		}
		out, err := l.llmAgent.Propose(ctx, in, learnerCfg)
		if err != nil {
			log.Printf("learner: [%s] propose failed (skip): %v", name, err)
			continue
		}

		if !agent.AcceptProposal(in, out) {
			log.Printf("learner: [%s] skip — proposal rejected by guardrail (confident=%v, empty=%v)",
				name, out != nil && out.Confident, out == nil || out.NewSkills == "")
			continue
		}

		// Audit FIRST (append-only, revertable), then apply. If the audit write
		// fails we do NOT apply — the change must always be recorded.
		if err := l.audit.Record(ctx, name, target.Skills, out.NewSkills, out.Rationale, patterns.N); err != nil {
			log.Printf("learner: [%s] audit write failed — NOT applying: %v", name, err)
			continue
		}
		if err := l.agents.UpdateSkillsByName(ctx, name, out.NewSkills); err != nil {
			log.Printf("learner: [%s] apply failed AFTER audit (revert from skill_revisions if needed): %v", name, err)
			continue
		}
		log.Printf("learner: [%s] APPLIED new skills (weakest=%s avg=%.2f n=%d) — rationale: %s",
			name, lowDim, lowVal, patterns.N, out.Rationale)
	}
	return nil
}
```

> **Note:** `agent.AcceptProposal` is exported in Task 4 precisely so this cross-package call works; the gate runs here on every proposal before any audit/apply. `formatPatterns` is defined in Step 2 below.

- [ ] **Step 2: Add the pure pattern formatter**

Renders a `ScorePatterns` into the Thai summary the prompt expects. Pure → unit-testable. Append to `internal/learner/learner.go`:

```go
// formatPatterns renders aggregated patterns into the Thai summary the learner
// prompt_template consumes. Pure.
func formatPatterns(p repository.ScorePatterns) string {
	lowDim, lowVal := p.LowestDimension()
	s := fmt.Sprintf(
		"จำนวน critique: %d\nคะแนนเฉลี่ย — hook: %.2f, clarity: %.2f, brand_fit: %.2f, overall: %.2f\nมิติที่อ่อนสุด: %s (%.2f)\n\nปัญหาที่ critic แก้บ่อยสุด:\n",
		p.N, p.AvgHook, p.AvgClarity, p.AvgBrandFit, p.AvgOverall, lowDim, lowVal,
	)
	if len(p.TopIssues) == 0 {
		s += "- (ไม่มีรายการ)\n"
		return s
	}
	for _, fi := range p.TopIssues {
		s += fmt.Sprintf("- %s — %s (x%d)\n", fi.Field, fi.Reason, fi.Count)
	}
	return s
}
```

- [ ] **Step 3: Write the failing tests for the gate**

Create `internal/learner/learner_test.go`. These test the pure `strongSignal` gate and `formatPatterns` only — `RunOnce` does I/O and is verified by the build + a live `-learn` run, not a unit test.

```go
package learner

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/repository"
)

func TestStrongSignal_FiresOnLowDimWithEnoughData(t *testing.T) {
	p := repository.ScorePatterns{N: minCritiques, AvgHook: 4.5, AvgClarity: 7, AvgBrandFit: 8, AvgOverall: 6.5}
	ok, dim, val := strongSignal(p)
	if !ok {
		t.Fatalf("strongSignal = false, want true (n=%d, hook=4.5 < %.1f)", p.N, lowScoreThreshold)
	}
	if dim != "hook" || val != 4.5 {
		t.Errorf("weakest = (%q, %v), want (hook, 4.5)", dim, val)
	}
}

func TestStrongSignal_SkipsTooFewCritiques(t *testing.T) {
	p := repository.ScorePatterns{N: minCritiques - 1, AvgHook: 2.0, AvgClarity: 2, AvgBrandFit: 2, AvgOverall: 2}
	if ok, _, _ := strongSignal(p); ok {
		t.Fatal("strongSignal = true, want false when n < minCritiques")
	}
}

func TestStrongSignal_SkipsAllDimsHealthy(t *testing.T) {
	p := repository.ScorePatterns{N: 50, AvgHook: lowScoreThreshold, AvgClarity: 7, AvgBrandFit: 8, AvgOverall: 9}
	if ok, _, _ := strongSignal(p); ok {
		t.Fatal("strongSignal = true, want false when weakest >= threshold (boundary is exclusive)")
	}
}

func TestStrongSignal_EmptyWindow(t *testing.T) {
	if ok, _, _ := strongSignal(repository.ScorePatterns{N: 0}); ok {
		t.Fatal("strongSignal = true, want false on empty window")
	}
}

func TestFormatPatterns_IncludesScoresAndIssues(t *testing.T) {
	p := repository.ScorePatterns{
		N: 12, AvgHook: 4.5, AvgClarity: 7, AvgBrandFit: 8, AvgOverall: 6,
		TopIssues: []repository.FieldIssue{
			{Field: "scene[0].voice_text", Reason: "hook อ่อน", Count: 5},
		},
	}
	s := formatPatterns(p)
	for _, want := range []string{"12", "4.50", "scene[0].voice_text", "hook อ่อน", "x5"} {
		if !strings.Contains(s, want) {
			t.Errorf("formatPatterns output missing %q\n--- got ---\n%s", want, s)
		}
	}
}

func TestFormatPatterns_NoIssues(t *testing.T) {
	s := formatPatterns(repository.ScorePatterns{N: 9, AvgHook: 5, AvgClarity: 5, AvgBrandFit: 5, AvgOverall: 5})
	if !strings.Contains(s, "ไม่มีรายการ") {
		t.Errorf("expected empty-issues marker, got:\n%s", s)
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/learner/ -run 'TestStrongSignal|TestFormatPatterns' -v`
Expected: PASS (6 tests).

> Note: these tests need the full `internal/learner` package (including `RunOnce` from Step 1) to compile. Make sure Task 4 exported `AcceptProposal` and the `learner.go` import block includes `models`, `agent`, and `repository` before running.

- [ ] **Step 5: Verify the package builds**

Run: `go build ./internal/learner/`
Expected: build OK.

- [ ] **Step 6: Commit**

```bash
git add internal/learner/learner.go internal/learner/learner_test.go
git commit -m "feat(learner): guardrailed RunOnce + pure strongSignal gate"
```

---

## Task 6: Audit repo + wire `-learn` into main.go

**Files:**
- Create: `internal/repository/skill_revisions.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Write the append-only audit repo**

This implements the `learner.SkillRevisionsWriter` interface. Same shape as the other repos (`*pgxpool.Pool` via `NewXRepo`). Create `internal/repository/skill_revisions.go`:

```go
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SkillRevisionsRepo struct {
	pool *pgxpool.Pool
}

func NewSkillRevisionsRepo(pool *pgxpool.Pool) *SkillRevisionsRepo {
	return &SkillRevisionsRepo{pool: pool}
}

// Record appends one audit row capturing the full old + new skills, the
// rationale, and the critique window. Append-only: never updates or deletes, so
// the table is a complete, revertable history of every auto-applied change.
func (r *SkillRevisionsRepo) Record(ctx context.Context, agentName, oldSkills, newSkills, rationale string, critiqueWindow int) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO skill_revisions (agent_name, old_skills, new_skills, rationale, critique_window)
		 VALUES ($1, $2, $3, $4, $5)`,
		agentName, oldSkills, newSkills, rationale, critiqueWindow)
	return err
}
```

`*SkillRevisionsRepo` satisfies `learner.SkillRevisionsWriter` (same `Record` signature).

- [ ] **Step 2: Add the `-learn` flag and construct + dispatch the Learner**

In `cmd/server/main.go`, after the `analyticsFlag := flag.Bool(...)` line (before `flag.Parse()`), add:

```go
	learnFlag := flag.Bool("learn", false, "Run the learning loop once (auto-tune upstream agent skills from critiques)")
```

After the existing `critiquesRepo := repository.NewCritiquesRepo(pool)` line, add:

```go
	skillRevisionsRepo := repository.NewSkillRevisionsRepo(pool)
	learnerAgent := agent.NewLearnerAgent(llm)
	learnerSvc := learner.New(agentsRepo, critiquesRepo, learnerAgent, skillRevisionsRepo)
```

Add the dispatch block next to the other flag handlers (e.g. after the `*analyticsFlag` block):

```go
	if *learnFlag {
		if err := learnerSvc.RunOnce(ctx); err != nil {
			log.Fatalf("Learning loop failed: %v", err)
		}
		return
	}
```

Add the import `"github.com/jaochai/video-fb/internal/learner"` to the import block of `cmd/server/main.go`.

> **verify at implementation time:** `agentsRepo`, `critiquesRepo`, `llm`, and `ctx` are all already in scope at that point in `main.go` (see lines ~76–109). `learner.New` takes the concrete `*repository.AgentsRepo`, `*repository.CritiquesRepo`, `*agent.LearnerAgent`, `*repository.SkillRevisionsRepo` — all satisfy the Task 5 interfaces directly (no adapters).

- [ ] **Step 3: Verify the whole project builds**

Run: `go build ./...`
Expected: build OK (exit 0).

- [ ] **Step 4: Commit**

```bash
git add internal/repository/skill_revisions.go cmd/server/main.go
git commit -m "feat(main): SkillRevisionsRepo + wire -learn flag for the learning loop"
```

---

## Task 7: Full verification

- [ ] **Step 1: Build, vet, and run the new pure tests**

Run:
```bash
go build ./... && \
go vet ./internal/agent/ ./internal/learner/ ./internal/repository/ && \
go test ./internal/agent/ ./internal/learner/ ./internal/repository/ -v
```
Expected: build OK, vet clean, all tests PASS — including the new pure tests:
- repository: `TestLowestDimension_*` (3)
- learner mapping: `TestAgentForField` (1, many sub-cases)
- agent: `TestAcceptProposal_*` + `TestLearnOutputParsesSchema` (6)
- learner: `TestStrongSignal_*` + `TestFormatPatterns_*` (6)
- plus the existing Phase 1 critic tests still pass.

- [ ] **Step 2: Sanity-check the migration applies (requires DB)**

Only if a dev/staging DB is configured. The app auto-runs migrations on startup; you can also trigger directly:
```bash
go run ./cmd/server -migrate
```
Expected: `037_skill_revisions.sql` applies with no error; re-running is a no-op (all guards idempotent). Verify the seed + schedule:
```bash
psql "$DATABASE_URL" -c "SELECT agent_name, enabled FROM agent_configs WHERE agent_name='learner';"
psql "$DATABASE_URL" -c "SELECT name, action, enabled FROM schedules WHERE action='learn';"
```
Expected: one `learner` row (enabled = t), one `Weekly Learn` row (action = learn, enabled = t).

- [ ] **Step 3: Dry-run the loop (requires DB with accumulated critiques)**

```bash
go run ./cmd/server -learn
```
Expected: it logs one decision per allowlisted agent — either `skip — weak signal (...)` (when there aren't yet `minCritiques` critiques or no dimension is below threshold) or `APPLIED new skills (...)`. On a fresh DB with few critiques, expect skips. If it applies, confirm the audit trail:
```bash
psql "$DATABASE_URL" -c "SELECT agent_name, critique_window, created_at, left(rationale,60) FROM skill_revisions ORDER BY created_at DESC LIMIT 5;"
```
Expected: a row per applied change, capturing old+new skills + rationale (revertable).

- [ ] **Step 4: Final commit (if anything outstanding)**

```bash
git status
```
Expected: clean working tree; all changes committed across Tasks 1–6.

---

## Notes / guardrails

This phase **auto-applies** LLM-proposed changes to a production agent's `skills`. The whole design is built so that is safe and reversible:

- **Append-only audit + full revertability.** Every applied change writes a `skill_revisions` row capturing the complete `old_skills`, `new_skills`, `rationale`, and `critique_window` BEFORE the `UpdateSkillsByName` runs. The table is never updated or deleted. Any change is revertable by hand:
  `UPDATE agent_configs SET skills = (SELECT old_skills FROM skill_revisions WHERE agent_name='<name>' ORDER BY created_at DESC LIMIT 1) WHERE agent_name='<name>';`
- **Audit-before-apply ordering.** If the audit write fails, the skills are NOT updated (the change must always be recorded). If the update fails after the audit, the audit row still exists so you can revert/inspect.
- **Allowlist-only.** `allowedAgents = {"scene","script"}` is the only set the loop can ever touch. No code path lets it modify any other agent (including `learner`, `critic`, `question`, etc.).
- **Strong-signal gate.** It acts only when there are at least `minCritiques` (8) critiques in the `windowDays` (30-day) window AND the weakest score dimension averages strictly below `lowScoreThreshold` (6.0). These are named constants — tune in one place. The threshold boundary is exclusive (avg == 6.0 is a skip).
- **Never blanks skills.** `AcceptProposal` rejects empty/blank `NewSkills`, a non-confident proposal, or skills identical to the current text. The loop can only ever replace skills with non-empty, changed, improved text.
- **Observable.** Every action and every skip logs a reason (weak signal, rejected proposal, propose error, audit error, applied). One `-learn` run is auditable from logs alone.
- **Kill switches.** Set `agent_configs['learner'].enabled = FALSE` to stop the whole loop (checked at the top of `RunOnce`). The weekly `schedules` row can be disabled independently.

### Assumptions to verify at implementation time (called out inline above)

1. **`schedules` columns** are `(name, cron_expression, action, enabled)` — confirmed against `migrations/032`. The scheduler dispatching the `'learn'` action to `-learn` is **out of scope**; the `-learn` flag is the guaranteed entry point and the cron row is harmless if unread (Task 1 note).
2. **`make_interval(days => $1)`** and the `score->>'key'::numeric` casts are standard Postgres/Neon; verified shape against the Phase 1 `score`/`changes` JSON contract, but the SQL itself needs a real DB to exercise (Task 2 note). `LowScorePatterns` therefore has no Go unit test — only its pure `LowestDimension` helper does.
3. **No import cycle.** `internal/learner` imports `agent`, `models`, `repository`; none import `learner`. The Task 5 interfaces use the real `*models.AgentConfig` and are satisfied by the concrete `*repository.AgentsRepo` / `*repository.CritiquesRepo` / `*agent.LearnerAgent` directly — verify by compiling (`go build ./internal/learner/`).
```
