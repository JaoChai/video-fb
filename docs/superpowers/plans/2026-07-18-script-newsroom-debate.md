# Script Newsroom Debate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
> (โปรเจกต์นี้ผู้ใช้ตั้งค่าให้ใช้ `glm-worker` Executor Mode แทนได้ — planner/reviewer อยู่ session หลัก)

**Goal:** ขั้น script เขียนแข่ง 3 เลนส์ขนานกัน → judge เลือกผู้ชนะ+ดึงจุดเด่น → ได้สคริปต์เดียวส่งต่อ pipeline เดิม โดย fail-open ทุกทางและปิดเปิดด้วย flag

**Architecture:** Debate อยู่ระดับ orchestrator (ไฟล์ใหม่ `script_debate.go`) ใช้ script agent เดิมตัวเดียวฉีด lens ผ่าน template field ใหม่ `{{.DebateLens}}`; judge เป็น agent ใหม่ `script_judge` (ไฟล์ `scriptjudge.go`); audit ลงตารางใหม่ `script_debates`; ทุกอย่าง seed ผ่าน migration 058

**Tech Stack:** Go 1.x, pgx/pgxpool, kie.ai LLM (`KieLLMClient.GenerateJSON`), custom `renderTemplate` (string-replace), Railway auto-migrate

**Spec:** `docs/superpowers/specs/2026-07-18-script-newsroom-debate-design.md`

## Global Constraints

- Flag `script_debate_enabled` default `'false'` — flag ปิด = path เดิม 100% ไม่มี LLM call เพิ่ม
- Fail-open ทุกทาง: นักเขียนเหลือ 1 → ข้าม judge; เหลือ 0 หรือ judge พัง → single-pass เดิม; ห้ามทำให้คลิป fail เพราะ debate
- `renderTemplate` เป็น string-replace ธรรมดา — prompt ใช้ได้แค่ `{{.Field}}` **ห้าม `{{if}}`** เด็ดขาด
- Migration ต้องหุ้ม `BEGIN; ... COMMIT;` เองทั้งไฟล์ (RunMigrations ไม่หุ้ม transaction) และ re-runnable (DELETE ก่อน INSERT / `ON CONFLICT DO NOTHING` / `IF NOT EXISTS`)
- ห้ามเขียนสตริง `-->` ในเนื้อ prompt/template ใดๆ (บทเรียน blank video regression)
- ทุก task: `go build ./... && go vet ./...` ต้องผ่านก่อน commit; commit message ภาษาอังกฤษ prefix `feat(debate):` / `test(debate):` / `migration:`

## File Structure

| ไฟล์ | บทบาท |
|---|---|
| Modify `internal/agent/script.go` | เพิ่ม field `DebateLens` + param `lens` ใน `Generate` |
| Create `internal/agent/scriptjudge.go` | `ScriptJudgeAgent` + types (`JudgeCandidate`, `JudgeVerdict` ฯลฯ) + validate |
| Create `internal/agent/scriptjudge_test.go` | tests ของ validate/candidate helper |
| Modify `internal/agent/script_test.go` | test render lens |
| Create `internal/repository/scriptdebates.go` | `ScriptDebatesRepo.Insert` (audit) |
| Create `internal/orchestrator/script_debate.go` | `parseDebateLenses` + `runScriptDebate` (pure, testable) + `generateScript` method + audit record |
| Create `internal/orchestrator/script_debate_test.go` | tests fallback ครบทุกแถว |
| Modify `internal/orchestrator/orchestrator.go` | struct + `New` + เปลี่ยน call site บรรทัด ~459 |
| Modify `cmd/server/main.go` | สร้าง repo + judge agent แล้วส่งเข้า `New` |
| Create `migrations/058_script_debate.sql` | ตาราง + settings + seed `script_judge` + ต่อท้าย `{{.DebateLens}}` |

---

### Task 1: Lens injection ใน ScriptAgent

**Files:**
- Modify: `internal/agent/script.go` (struct `ScriptTemplateData` บรรทัด 14-23, `Generate` บรรทัด 70)
- Modify: `internal/agent/script_test.go`
- Modify: `internal/orchestrator/orchestrator.go:459` (เติม arg `""` ให้ build ผ่าน — call site จริงย้ายใน Task 5)

**Interfaces:**
- Produces: `Generate(ctx, question, questionerName, category string, format *models.ContentFormat, persona, archetypeInstr, roleInstr, lens string, cfg *models.AgentConfig) (*GeneratedScript, error)` — param ใหม่ `lens` แทรกก่อน `cfg`; lens = ข้อความ block เต็ม (คนเรียกประกอบ header มาแล้ว) หรือ `""` เมื่อไม่ debate

- [ ] **Step 1: เขียน failing test** — เพิ่มท้าย `internal/agent/script_test.go`:

```go
// DebateLens renders into the script prompt; empty lens leaves no residue —
// the flag-off path must produce a byte-identical prompt to before the field
// existed (aside from the appended placeholder resolving to empty).
func TestScriptTemplateData_DebateLensRender(t *testing.T) {
	out, err := renderTemplate("base {{.DebateLens}}", ScriptTemplateData{DebateLens: "LENS"})
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	if out != "base LENS" {
		t.Errorf("got %q want %q", out, "base LENS")
	}
	out, err = renderTemplate("base {{.DebateLens}}", ScriptTemplateData{})
	if err != nil {
		t.Fatalf("renderTemplate empty: %v", err)
	}
	if out != "base " {
		t.Errorf("empty lens: got %q want %q", out, "base ")
	}
}
```

- [ ] **Step 2: รันให้ fail**

Run: `go test ./internal/agent/ -run TestScriptTemplateData_DebateLensRender -v`
Expected: FAIL (compile error: `unknown field DebateLens`)

- [ ] **Step 3: implement** — ใน `internal/agent/script.go`:

แก้ struct (เพิ่มบรรทัดเดียวท้าย struct):

```go
type ScriptTemplateData struct {
	Question             string
	QuestionerName       string
	Category             string
	ArchetypeInstruction string
	RoleInstruction      string
	RAGContext           string
	FormatInstruction    string
	AudiencePersona      string
	DebateLens           string
}
```

แก้ signature `Generate` (บรรทัด 70) เพิ่ม `lens string` ก่อน `cfg`:

```go
func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category string, format *models.ContentFormat, persona string, archetypeInstr string, roleInstr string, lens string, cfg *models.AgentConfig) (*GeneratedScript, error) {
```

และใน call `renderTemplate` (บรรทัด ~95) เพิ่ม field:

```go
	userPrompt, err := renderTemplate(cfg.PromptTemplate, ScriptTemplateData{
		Question:             question,
		QuestionerName:       questionerName,
		Category:             category,
		ArchetypeInstruction: archetypeInstr,
		RoleInstruction:      roleInstr,
		RAGContext:           ragContext.String(),
		FormatInstruction:    format.ScriptInstruction,
		AudiencePersona:      persona,
		DebateLens:           lens,
	})
```

แก้ call site ใน `internal/orchestrator/orchestrator.go:459` (ชั่วคราว ให้ build ผ่าน):

```go
	script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, format, persona, archetype.Instruction, RoleInstruction(role), "", scriptCfg)
```

- [ ] **Step 4: รันให้ผ่าน**

Run: `go test ./internal/agent/ -run TestScriptTemplateData_DebateLensRender -v && go build ./...`
Expected: PASS + build สำเร็จ

- [ ] **Step 5: Commit**

```bash
git add internal/agent/script.go internal/agent/script_test.go internal/orchestrator/orchestrator.go
git commit -m "feat(debate): ScriptAgent accepts debate lens via {{.DebateLens}}"
```

---

### Task 2: ScriptJudgeAgent

**Files:**
- Create: `internal/agent/scriptjudge.go`
- Create: `internal/agent/scriptjudge_test.go`

**Interfaces:**
- Consumes: `GeneratedScript`, `validateGeneratedScript` (มีอยู่แล้วใน package), `KieLLMClient.GenerateJSON`, `renderTemplate`, `models.AgentConfig`
- Produces (Task 4/5 ใช้):
  - `type JudgeCandidate struct { Lens, AnswerScript, VoiceScript, YoutubeTitle, YoutubeDescription string; YoutubeTags []string }` (json tags ตามโค้ด)
  - `func NewJudgeCandidate(lens string, s *GeneratedScript) JudgeCandidate`
  - `type JudgeVerdict struct { Scores []JudgeScore; WinnerLens, Rationale string; Final GeneratedScript }`
  - `type JudgeInput struct { Question, AudiencePersona string; Candidates []JudgeCandidate }`
  - `func NewScriptJudgeAgent(llm *KieLLMClient) *ScriptJudgeAgent`
  - `func (a *ScriptJudgeAgent) Judge(ctx context.Context, in JudgeInput, cfg *models.AgentConfig) (*JudgeVerdict, error)` — validate verdict ในตัว (Final ต้องไม่ว่าง, WinnerLens ต้องตรง candidate) คืน error เมื่อใช้ไม่ได้ → คนเรียก fail-open เอง

- [ ] **Step 1: เขียน failing tests** — สร้าง `internal/agent/scriptjudge_test.go`:

```go
package agent

import "testing"

func judgeCands() []JudgeCandidate {
	return []JudgeCandidate{
		{Lens: "hook_maximalist", VoiceScript: "ฉบับ A"},
		{Lens: "skeptic_editor", VoiceScript: "ฉบับ B"},
	}
}

// Verdict whose final script is empty is unusable — the caller must fall back
// to a raw candidate, so validation rejects it here.
func TestValidateJudgeVerdict_EmptyFinal(t *testing.T) {
	v := &JudgeVerdict{WinnerLens: "hook_maximalist"}
	if err := validateJudgeVerdict(v, judgeCands()); err == nil {
		t.Fatal("expected error for empty final script, got nil")
	}
}

// winner_lens must name one of the actual candidates; a hallucinated lens key
// means the judge ignored the input.
func TestValidateJudgeVerdict_UnknownWinner(t *testing.T) {
	v := &JudgeVerdict{
		WinnerLens: "made_up",
		Final:      GeneratedScript{VoiceScript: "โอเค"},
	}
	if err := validateJudgeVerdict(v, judgeCands()); err == nil {
		t.Fatal("expected error for unknown winner_lens, got nil")
	}
}

func TestValidateJudgeVerdict_Valid(t *testing.T) {
	v := &JudgeVerdict{
		WinnerLens: "skeptic_editor",
		Final:      GeneratedScript{VoiceScript: "โอเค"},
	}
	if err := validateJudgeVerdict(v, judgeCands()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// NewJudgeCandidate carries only the fields the judge needs — scenes and
// timing stay out of the prompt.
func TestNewJudgeCandidate(t *testing.T) {
	s := &GeneratedScript{
		AnswerScript: "เต็ม", VoiceScript: "พากย์",
		YoutubeTitle: "T", YoutubeDescription: "D", YoutubeTags: []string{"a"},
	}
	c := NewJudgeCandidate("target_viewer", s)
	if c.Lens != "target_viewer" || c.AnswerScript != "เต็ม" || c.VoiceScript != "พากย์" ||
		c.YoutubeTitle != "T" || c.YoutubeDescription != "D" || len(c.YoutubeTags) != 1 {
		t.Errorf("candidate fields not copied: %+v", c)
	}
}
```

- [ ] **Step 2: รันให้ fail**

Run: `go test ./internal/agent/ -run 'TestValidateJudgeVerdict|TestNewJudgeCandidate' -v`
Expected: FAIL (compile error: undefined `JudgeCandidate` ฯลฯ)

- [ ] **Step 3: implement** — สร้าง `internal/agent/scriptjudge.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jaochai/video-fb/internal/models"
)

// JudgeCandidate is the slim view of one debate script the judge scores.
// Scenes/timing are deliberately excluded — the judge rules on content only.
type JudgeCandidate struct {
	Lens               string   `json:"lens"`
	AnswerScript       string   `json:"answer_script"`
	VoiceScript        string   `json:"voice_script"`
	YoutubeTitle       string   `json:"youtube_title"`
	YoutubeDescription string   `json:"youtube_description"`
	YoutubeTags        []string `json:"youtube_tags"`
}

func NewJudgeCandidate(lens string, s *GeneratedScript) JudgeCandidate {
	return JudgeCandidate{
		Lens:               lens,
		AnswerScript:       s.AnswerScript,
		VoiceScript:        s.VoiceScript,
		YoutubeTitle:       s.YoutubeTitle,
		YoutubeDescription: s.YoutubeDescription,
		YoutubeTags:        s.YoutubeTags,
	}
}

// JudgeScore is the judge's 1-10 rating of one candidate.
type JudgeScore struct {
	Lens        string `json:"lens"`
	Hook        int    `json:"hook"`
	Accuracy    int    `json:"accuracy"`
	AudienceFit int    `json:"audience_fit"`
}

// JudgeVerdict is the raw JSON the judge LLM returns. Final uses the same
// shape as a normal script-agent output so downstream stages see no difference.
type JudgeVerdict struct {
	Scores     []JudgeScore    `json:"scores"`
	WinnerLens string          `json:"winner_lens"`
	Rationale  string          `json:"rationale"`
	Final      GeneratedScript `json:"final"`
}

type JudgeInput struct {
	Question        string
	AudiencePersona string
	Candidates      []JudgeCandidate
}

type judgeTemplateData struct {
	Question        string
	AudiencePersona string
	CandidatesJSON  string
}

type ScriptJudgeAgent struct {
	llm *KieLLMClient
}

func NewScriptJudgeAgent(llm *KieLLMClient) *ScriptJudgeAgent {
	return &ScriptJudgeAgent{llm: llm}
}

// Judge scores the candidates and returns the merged final script. Any error
// (LLM, parse, invalid verdict) is returned as-is; the caller is responsible
// for the fail-open fallback to a raw candidate.
func (a *ScriptJudgeAgent) Judge(ctx context.Context, in JudgeInput, cfg *models.AgentConfig) (*JudgeVerdict, error) {
	candJSON, err := json.Marshal(in.Candidates)
	if err != nil {
		return nil, fmt.Errorf("marshal candidates: %w", err)
	}
	userPrompt, err := renderTemplate(cfg.PromptTemplate, judgeTemplateData{
		Question:        in.Question,
		AudiencePersona: in.AudiencePersona,
		CandidatesJSON:  string(candJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("render judge template: %w", err)
	}
	var v JudgeVerdict
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &v); err != nil {
		return nil, fmt.Errorf("judge llm: %w", err)
	}
	if err := validateJudgeVerdict(&v, in.Candidates); err != nil {
		return nil, fmt.Errorf("judge verdict invalid: %w", err)
	}
	return &v, nil
}

// validateJudgeVerdict rejects verdicts the pipeline cannot safely use: an
// empty final script (same rule as validateGeneratedScript) or a winner_lens
// that names no actual candidate.
func validateJudgeVerdict(v *JudgeVerdict, cands []JudgeCandidate) error {
	if err := validateGeneratedScript(&v.Final); err != nil {
		return err
	}
	for _, c := range cands {
		if c.Lens == v.WinnerLens {
			return nil
		}
	}
	return fmt.Errorf("winner_lens %q not among candidates", v.WinnerLens)
}
```

- [ ] **Step 4: รันให้ผ่าน**

Run: `go test ./internal/agent/ -run 'TestValidateJudgeVerdict|TestNewJudgeCandidate' -v && go build ./...`
Expected: PASS ทั้ง 4 + build สำเร็จ

- [ ] **Step 5: Commit**

```bash
git add internal/agent/scriptjudge.go internal/agent/scriptjudge_test.go
git commit -m "feat(debate): ScriptJudgeAgent scores candidates and merges final script"
```

---

### Task 3: ScriptDebatesRepo (audit)

**Files:**
- Create: `internal/repository/scriptdebates.go`

**Interfaces:**
- Produces: `func NewScriptDebatesRepo(pool *pgxpool.Pool) *ScriptDebatesRepo`, `func (r *ScriptDebatesRepo) Insert(ctx context.Context, clipID string, candidates, verdict []byte, source string) error` — `verdict` nil ได้ (กรณีข้าม judge) → เก็บ NULL
- หมายเหตุ: ไม่มี DB test harness ในโปรเจกต์ → task นี้ตรวจด้วย compile + vet (pattern ตาม `autoreviews.go`)

- [ ] **Step 1: implement** — สร้าง `internal/repository/scriptdebates.go`:

```go
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ScriptDebatesRepo struct{ pool *pgxpool.Pool }

func NewScriptDebatesRepo(pool *pgxpool.Pool) *ScriptDebatesRepo {
	return &ScriptDebatesRepo{pool: pool}
}

// Insert appends one debate audit row. candidates/verdict are JSON-encoded;
// verdict may be nil when the judge was skipped (single candidate) or failed.
func (r *ScriptDebatesRepo) Insert(ctx context.Context, clipID string, candidates, verdict []byte, source string) error {
	if verdict == nil {
		_, err := r.pool.Exec(ctx,
			`INSERT INTO script_debates (clip_id, candidates, verdict, source)
			 VALUES ($1, $2, NULL, $3)`,
			clipID, candidates, source)
		return err
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO script_debates (clip_id, candidates, verdict, source)
		 VALUES ($1, $2, $3, $4)`,
		clipID, candidates, verdict, source)
	return err
}
```

- [ ] **Step 2: ตรวจ compile**

Run: `go build ./... && go vet ./internal/repository/`
Expected: ผ่านทั้งคู่

- [ ] **Step 3: Commit**

```bash
git add internal/repository/scriptdebates.go
git commit -m "feat(debate): script_debates audit repo"
```

---

### Task 4: Debate core — `runScriptDebate` + `parseDebateLenses`

**Files:**
- Create: `internal/orchestrator/script_debate.go` (เฉพาะส่วน pure — method ที่แตะ Orchestrator เพิ่มใน Task 5 ไฟล์เดียวกัน)
- Create: `internal/orchestrator/script_debate_test.go`

**Interfaces:**
- Consumes: `agent.GeneratedScript`, `agent.JudgeCandidate`, `agent.JudgeVerdict`, `agent.NewJudgeCandidate` (Task 2)
- Produces (Task 5 ใช้):
  - `type debateLens struct { Key, Name, Instruction string }` (json tags: `key`, `name`, `instruction`)
  - `func parseDebateLenses(raw string) []debateLens` — คืน nil เมื่อ JSON พัง/ข้อมูลใช้ได้ <2 เลนส์
  - `type scriptGenFn func(lensInstruction string) (*agent.GeneratedScript, error)`
  - `type scriptJudgeFn func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error)`
  - `func runScriptDebate(lenses []debateLens, gen scriptGenFn, judge scriptJudgeFn) (final *agent.GeneratedScript, cands []agent.JudgeCandidate, verdict *agent.JudgeVerdict, source string, err error)` — source ∈ `judge` | `single_candidate` | `judge_failed`; err != nil เฉพาะเมื่อนักเขียน fail หมด (คนเรียกไป single-pass ต่อ)

- [ ] **Step 1: เขียน failing tests** — สร้าง `internal/orchestrator/script_debate_test.go`:

```go
package orchestrator

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func testLenses() []debateLens {
	return []debateLens{
		{Key: "hook_maximalist", Name: "Hook", Instruction: "แรงสุด"},
		{Key: "skeptic_editor", Name: "Skeptic", Instruction: "แม่นสุด"},
		{Key: "target_viewer", Name: "Viewer", Instruction: "ตรง pain สุด"},
	}
}

// Happy path: all writers succeed, judge succeeds → final comes from the
// judge's merged script, all candidates recorded.
func TestRunScriptDebate_JudgeWins(t *testing.T) {
	gen := func(lens string) (*agent.GeneratedScript, error) {
		return &agent.GeneratedScript{VoiceScript: "ฉบับ:" + lens}, nil
	}
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) {
		if len(cands) != 3 {
			t.Fatalf("judge got %d candidates, want 3", len(cands))
		}
		return &agent.JudgeVerdict{
			WinnerLens: "skeptic_editor",
			Final:      agent.GeneratedScript{VoiceScript: "ฉบับรวมร่าง"},
		}, nil
	}
	final, cands, verdict, source, err := runScriptDebate(testLenses(), gen, judge)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if source != "judge" || verdict == nil || final.VoiceScript != "ฉบับรวมร่าง" || len(cands) != 3 {
		t.Errorf("got source=%s verdict=%v final=%q cands=%d", source, verdict, final.VoiceScript, len(cands))
	}
}

// The lens block each writer receives must contain that lens's instruction —
// this is the "blind, independent angles" property the whole feature rests on.
func TestRunScriptDebate_LensInstructionReachesWriters(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	gen := func(lens string) (*agent.GeneratedScript, error) {
		mu.Lock()
		seen = append(seen, lens)
		mu.Unlock()
		return &agent.GeneratedScript{VoiceScript: "x"}, nil
	}
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) {
		return &agent.JudgeVerdict{WinnerLens: "hook_maximalist", Final: agent.GeneratedScript{VoiceScript: "y"}}, nil
	}
	if _, _, _, _, err := runScriptDebate(testLenses(), gen, judge); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	joined := strings.Join(seen, "|")
	for _, want := range []string{"แรงสุด", "แม่นสุด", "ตรง pain สุด"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no writer received instruction %q (got %q)", want, joined)
		}
	}
}

// Judge failure is fail-open: fall back to the first successful candidate,
// verdict nil, no error surfaced.
func TestRunScriptDebate_JudgeFails_FirstCandidate(t *testing.T) {
	gen := func(lens string) (*agent.GeneratedScript, error) {
		return &agent.GeneratedScript{VoiceScript: "ฉบับ:" + lens}, nil
	}
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) {
		return nil, errors.New("judge exploded")
	}
	final, cands, verdict, source, err := runScriptDebate(testLenses(), gen, judge)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if source != "judge_failed" || verdict != nil || len(cands) != 3 {
		t.Errorf("got source=%s verdict=%v cands=%d", source, verdict, len(cands))
	}
	if !strings.Contains(final.VoiceScript, "แรงสุด") {
		t.Errorf("expected first (hook) candidate, got %q", final.VoiceScript)
	}
}

// Exactly one writer succeeds → judge must be skipped entirely.
func TestRunScriptDebate_SingleCandidateSkipsJudge(t *testing.T) {
	gen := func(lens string) (*agent.GeneratedScript, error) {
		if !strings.Contains(lens, "แม่นสุด") {
			return nil, errors.New("writer down")
		}
		return &agent.GeneratedScript{VoiceScript: "ฉบับเดียวที่รอด"}, nil
	}
	judgeCalled := false
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) {
		judgeCalled = true
		return nil, nil
	}
	final, cands, verdict, source, err := runScriptDebate(testLenses(), gen, judge)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if judgeCalled {
		t.Error("judge must not be called with a single candidate")
	}
	if source != "single_candidate" || verdict != nil || len(cands) != 1 || final.VoiceScript != "ฉบับเดียวที่รอด" {
		t.Errorf("got source=%s verdict=%v cands=%d final=%q", source, verdict, len(cands), final.VoiceScript)
	}
}

// All writers fail → error out so the caller can run the plain single-pass.
func TestRunScriptDebate_AllWritersFail(t *testing.T) {
	gen := func(lens string) (*agent.GeneratedScript, error) { return nil, errors.New("down") }
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) { return nil, nil }
	if _, _, _, _, err := runScriptDebate(testLenses(), gen, judge); err == nil {
		t.Fatal("expected error when all writers fail, got nil")
	}
}

func TestParseDebateLenses(t *testing.T) {
	good := `[{"key":"a","name":"A","instruction":"ก"},{"key":"b","name":"B","instruction":"ข"}]`
	if got := parseDebateLenses(good); len(got) != 2 {
		t.Errorf("valid JSON: got %d lenses, want 2", len(got))
	}
	if got := parseDebateLenses("not json"); got != nil {
		t.Errorf("invalid JSON must return nil, got %v", got)
	}
	// blank key/instruction rows are dropped; fewer than 2 usable → nil
	oneUsable := `[{"key":"a","name":"A","instruction":"ก"},{"key":"","name":"B","instruction":"ข"}]`
	if got := parseDebateLenses(oneUsable); got != nil {
		t.Errorf("<2 usable lenses must return nil, got %v", got)
	}
}
```

- [ ] **Step 2: รันให้ fail**

Run: `go test ./internal/orchestrator/ -run 'TestRunScriptDebate|TestParseDebateLenses' -v`
Expected: FAIL (compile error: undefined `debateLens` ฯลฯ)

- [ ] **Step 3: implement** — สร้าง `internal/orchestrator/script_debate.go`:

```go
package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/jaochai/video-fb/internal/agent"
)

// debateLens is one writing angle in the script newsroom debate, loaded from
// the script_debate_lenses setting.
type debateLens struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Instruction string `json:"instruction"`
}

// parseDebateLenses parses the setting JSON. Returns nil (caller falls back to
// single-pass) on bad JSON or fewer than 2 usable lenses — a one-writer
// "debate" is just a slower single pass.
func parseDebateLenses(raw string) []debateLens {
	var lenses []debateLens
	if err := json.Unmarshal([]byte(raw), &lenses); err != nil {
		return nil
	}
	usable := make([]debateLens, 0, len(lenses))
	for _, l := range lenses {
		if strings.TrimSpace(l.Key) != "" && strings.TrimSpace(l.Instruction) != "" {
			usable = append(usable, l)
		}
	}
	if len(usable) < 2 {
		return nil
	}
	return usable
}

type scriptGenFn func(lensInstruction string) (*agent.GeneratedScript, error)
type scriptJudgeFn func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error)

// runScriptDebate runs one writer per lens in parallel, then the judge.
// Fail-open ladder: judge error → first candidate; one candidate → skip judge;
// zero candidates → error (caller runs the plain single-pass generate).
func runScriptDebate(lenses []debateLens, gen scriptGenFn, judge scriptJudgeFn) (*agent.GeneratedScript, []agent.JudgeCandidate, *agent.JudgeVerdict, string, error) {
	type slot struct {
		script *agent.GeneratedScript
		err    error
	}
	slots := make([]slot, len(lenses))
	var wg sync.WaitGroup
	for i, l := range lenses {
		wg.Add(1)
		go func(i int, l debateLens) {
			defer wg.Done()
			s, err := gen("## มุมมองการเขียนรอบนี้ (" + l.Name + ")\n" + l.Instruction)
			slots[i] = slot{script: s, err: err}
		}(i, l)
	}
	wg.Wait()

	var cands []agent.JudgeCandidate
	var scripts []*agent.GeneratedScript
	for i, s := range slots {
		if s.err != nil || s.script == nil {
			continue
		}
		cands = append(cands, agent.NewJudgeCandidate(lenses[i].Key, s.script))
		scripts = append(scripts, s.script)
	}

	switch len(scripts) {
	case 0:
		return nil, nil, nil, "", fmt.Errorf("all %d debate writers failed", len(lenses))
	case 1:
		return scripts[0], cands, nil, "single_candidate", nil
	}

	verdict, err := judge(cands)
	if err != nil {
		return scripts[0], cands, nil, "judge_failed", nil
	}
	return &verdict.Final, cands, verdict, "judge", nil
}
```

- [ ] **Step 4: รันให้ผ่าน**

Run: `go test ./internal/orchestrator/ -run 'TestRunScriptDebate|TestParseDebateLenses' -v && go build ./...`
Expected: PASS ทั้ง 6 + build สำเร็จ

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/script_debate.go internal/orchestrator/script_debate_test.go
git commit -m "feat(debate): runScriptDebate core with fail-open ladder"
```

---

### Task 5: Wire เข้า Orchestrator + main.go

**Files:**
- Modify: `internal/orchestrator/script_debate.go` (เพิ่ม method `generateScript` + `recordScriptDebate`)
- Modify: `internal/orchestrator/orchestrator.go` (struct บรรทัด 61-84, `New` บรรทัด 86-119, call site บรรทัด ~459)
- Modify: `cmd/server/main.go` (บล็อกสร้าง repo ~บรรทัด 115-130)

**Interfaces:**
- Consumes: ทุกอย่างจาก Task 1-4; `o.settingsRepo.Get`, `o.agentsRepo.GetByName`
- Produces: `func (o *Orchestrator) generateScript(ctx context.Context, clipID string, q agent.GeneratedQuestion, format *models.ContentFormat, persona, archetypeInstr, roleInstr string, scriptCfg *models.AgentConfig) (*agent.GeneratedScript, error)`
- Orchestrator field ใหม่: `scriptJudgeAgent *agent.ScriptJudgeAgent`, `scriptDebatesRepo *repository.ScriptDebatesRepo`; `New` รับ param เพิ่ม 2 ตัว: `sja *agent.ScriptJudgeAgent` (ต่อจาก `ara`), `scriptDebates *repository.ScriptDebatesRepo` (ต่อจาก `autoreviews`)

- [ ] **Step 1: เพิ่ม method ใน `internal/orchestrator/script_debate.go`** (ต่อท้ายไฟล์; เพิ่ม import `"context"`, `"log"`, `"github.com/jaochai/video-fb/internal/models"`):

```go
// generateScript is the script-stage entry point. Flag off (or any config
// gap) → the plain single-pass path, byte-for-byte the old behavior. Flag on →
// newsroom debate with the fail-open ladder in runScriptDebate; if even that
// errors, fall back to single-pass. The debate can never fail a clip.
func (o *Orchestrator) generateScript(ctx context.Context, clipID string, q agent.GeneratedQuestion, format *models.ContentFormat, persona, archetypeInstr, roleInstr string, scriptCfg *models.AgentConfig) (*agent.GeneratedScript, error) {
	single := func() (*agent.GeneratedScript, error) {
		return o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, format, persona, archetypeInstr, roleInstr, "", scriptCfg)
	}

	if raw, _ := o.settingsRepo.Get(ctx, "script_debate_enabled"); raw != "true" {
		return single()
	}

	lensRaw, _ := o.settingsRepo.Get(ctx, "script_debate_lenses")
	lenses := parseDebateLenses(lensRaw)
	judgeCfg, jerr := o.agentsRepo.GetByName(ctx, "script_judge")
	if lenses == nil || jerr != nil || judgeCfg == nil || !judgeCfg.Enabled {
		log.Printf("script debate: config unavailable (lenses=%d, judgeErr=%v) — single-pass fallback", len(lenses), jerr)
		return single()
	}

	gen := func(lensInstruction string) (*agent.GeneratedScript, error) {
		return o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, format, persona, archetypeInstr, roleInstr, lensInstruction, scriptCfg)
	}
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) {
		return o.scriptJudgeAgent.Judge(ctx, agent.JudgeInput{
			Question:        q.Question,
			AudiencePersona: persona,
			Candidates:      cands,
		}, judgeCfg)
	}

	final, cands, verdict, source, err := runScriptDebate(lenses, gen, judge)
	if err != nil {
		log.Printf("script debate: %v — single-pass fallback", err)
		return single()
	}
	log.Printf("script debate: source=%s candidates=%d clip=%s", source, len(cands), clipID)
	o.recordScriptDebate(ctx, clipID, cands, verdict, source)
	return final, nil
}

// recordScriptDebate persists the audit row; failures only log — audit must
// never block production.
func (o *Orchestrator) recordScriptDebate(ctx context.Context, clipID string, cands []agent.JudgeCandidate, verdict *agent.JudgeVerdict, source string) {
	if o.scriptDebatesRepo == nil {
		return
	}
	candJSON, err := json.Marshal(cands)
	if err != nil {
		log.Printf("script debate: marshal candidates for audit failed (non-fatal): %v", err)
		return
	}
	var verdictJSON []byte
	if verdict != nil {
		verdictJSON, err = json.Marshal(verdict)
		if err != nil {
			log.Printf("script debate: marshal verdict for audit failed (non-fatal): %v", err)
			verdictJSON = nil
		}
	}
	if err := o.scriptDebatesRepo.Insert(ctx, clipID, candJSON, verdictJSON, source); err != nil {
		log.Printf("script debate: audit insert failed (non-fatal): %v", err)
	}
}
```

- [ ] **Step 2: แก้ `internal/orchestrator/orchestrator.go`**

Struct — เพิ่ม 2 field (หลัง `autoReviewAgent` และหลัง `autoReviewsRepo` ตามลำดับ):

```go
	autoReviewAgent *agent.AutoReviewAgent
	scriptJudgeAgent *agent.ScriptJudgeAgent
```
```go
	autoReviewsRepo *repository.AutoReviewsRepo
	scriptDebatesRepo *repository.ScriptDebatesRepo
```

`New` — เพิ่ม param `sja *agent.ScriptJudgeAgent` ต่อจาก `ara *agent.AutoReviewAgent` และ `scriptDebates *repository.ScriptDebatesRepo` ต่อจาก `autoreviews *repository.AutoReviewsRepo` แล้วเพิ่มใน struct literal:

```go
	autoReviewsRepo: autoreviews, scriptJudgeAgent: sja, scriptDebatesRepo: scriptDebates,
```

Call site (บรรทัด ~459 — แทนบรรทัดที่ Task 1 แก้ไว้):

```go
	script, err := o.generateScript(ctx, clipID, q, format, persona, archetype.Instruction, RoleInstruction(role), scriptCfg)
```

- [ ] **Step 3: แก้ `cmd/server/main.go`** — ในบล็อกสร้าง repos (~บรรทัด 119) เพิ่ม:

```go
	scriptDebatesRepo := repository.NewScriptDebatesRepo(pool)
	scriptJudgeAgent := agent.NewScriptJudgeAgent(llm)
```

และแก้ call `orchestrator.New(...)` ให้แทรก arg ตามตำแหน่งใหม่:

```go
	orch := orchestrator.New(questionAgent, scriptAgent, imageAgent, metadataAgent, sceneAgent, criticAgent, visualQAAgent, autoReviewAgent, scriptJudgeAgent, prod,
		clipsRepo, scenesRepo, critiquesRepo, visualQARepo, autoReviewsRepo, scriptDebatesRepo, themesRepo, agentsRepo, analyticsRepo, settingsRepo, formatsRepo,
		repository.NewTopicCategoriesRepo(pool), repository.NewTitleArchetypesRepo(pool), tracker)
```

(ตรวจชื่อตัวแปร llm client ใน main.go ให้ตรงของจริง — agent ตัวอื่นใช้ `llm` เช่น `agent.NewLearnerAgent(llm)`)

- [ ] **Step 4: build + test ทั้งหมด**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: ผ่านทั้งหมด (test เดิมทุกไฟล์ต้องยังเขียว)

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/script_debate.go internal/orchestrator/orchestrator.go cmd/server/main.go
git commit -m "feat(debate): wire script newsroom debate into produce pipeline (flag-gated)"
```

---

### Task 6: Migration 058

**Files:**
- Create: `migrations/058_script_debate.sql`

**Interfaces:**
- Produces: ตาราง `script_debates`; settings `script_debate_enabled` (='false'), `script_debate_lenses`; agent_configs row `script_judge`; ต่อท้าย `{{.DebateLens}}` ใน prompt_template ของ `script`

- [ ] **Step 1: เขียน migration** — สร้าง `migrations/058_script_debate.sql`:

```sql
-- 058 Script newsroom debate: 3 lens writers + judge (flag-gated, fail-open)
-- Spec: docs/superpowers/specs/2026-07-18-script-newsroom-debate-design.md
-- renderTemplate = string-replace (custom) NOT text/template — ใช้ได้แค่ {{.Field}} ห้าม {{if}}
-- Re-runnable ในไฟล์ (IF NOT EXISTS / ON CONFLICT / DELETE ก่อน INSERT / NOT LIKE guard)
-- หุ้ม BEGIN/COMMIT ให้ atomic: RunMigrations ทำ pool.Exec ไฟล์เดียวไม่หุ้ม transaction
BEGIN;

-- 1) Audit table: หนึ่งแถวต่อการ debate หนึ่งครั้ง
CREATE TABLE IF NOT EXISTS script_debates (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id    UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    candidates JSONB NOT NULL DEFAULT '[]'::jsonb,
    verdict    JSONB,                      -- NULL เมื่อข้าม judge (เหลือ candidate เดียว) หรือ judge พัง
    source     TEXT NOT NULL,              -- judge | single_candidate | judge_failed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_script_debates_clip_id ON script_debates (clip_id);

-- 2) Flag + lenses (แก้จากหน้า Settings ได้โดยไม่ต้อง deploy)
INSERT INTO settings (key, value) VALUES
('script_debate_enabled', 'false'),
('script_debate_lenses', '[{"key":"hook_maximalist","name":"Hook Maximalist","instruction":"โฟกัสพลังหยุดนิ้วสูงสุด: 3 วินาทีแรกต้องแรงจนคนยิงแอดต้องหยุดดู เปิดด้วยตัวเลข/ความเสียหาย/คำถามที่แทงใจกลุ่มเป้าหมาย กล้าตัดเนื้อหาที่ไม่เสริม hook ทิ้ง ทุกประโยคต้องสร้างเหตุผลให้ดูต่ออีก 3 วินาที ห้ามเปิดด้วยการอธิบายพื้นหลังหรือทักทาย"},{"key":"skeptic_editor","name":"Skeptic Editor","instruction":"โฟกัสความแม่นและความน่าเชื่อถือ: ทุก claim ต้องเป็นสิ่งที่คนวงในยืนยันได้จริง ห้าม oversell ห้ามตัวเลขมั่ว ห้ามสัญญาผลลัพธ์ ถ้าไม่ชัวร์ให้พูดแบบมีเงื่อนไข เนื้อหาต้องลึกระดับที่มือใหม่เขียนไม่ได้ และห้ามแนะนำอะไรที่ขัดนโยบาย Meta ตรงๆ"},{"key":"target_viewer","name":"Target Viewer","instruction":"เขียนจากมุมคนดูเป้าหมายของรอบนี้ตรงๆ: ใช้ภาษาและศัพท์ที่คนกลุ่มนี้พิมพ์คุยกันจริง เล่าสถานการณ์ที่เขากำลังเจออยู่ให้รู้สึกว่านี่คือเรื่องของเขาเอง ตอบ pain ให้จบในคลิปเดียว ไม่อวดฉลาดเกินคนดู"}]')
ON CONFLICT (key) DO NOTHING;

-- 3) Judge agent (enabled=TRUE — การเปิดใช้จริงคุมด้วย script_debate_enabled)
DELETE FROM agent_configs WHERE agent_name = 'script_judge';
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
VALUES (
  'script_judge',
  'คุณคือหัวหน้ากองบรรณาธิการ (Editor-in-Chief) ของ Ads Vance ช่องความรู้เรื่องบัญชีโฆษณา Facebook สำหรับคนยิงแอดตัวจริง หน้าที่คุณคือตัดสินสคริปต์วิดีโอสั้นที่นักเขียน 3 คนเขียนแข่งกันคนละมุมมอง แล้วประกอบร่างฉบับที่ดีที่สุดฉบับเดียว คุณให้น้ำหนัก retention เหนือความครบถ้วน: hook 3 วินาทีแรกสำคัญที่สุด รองลงมาคือความแม่นของข้อมูล (เลขมั่ว/ขัดนโยบาย = ตกทันที) และความตรง pain ของผู้ชม',
  'คำถามของคลิป: {{.Question}}
ผู้ชมเป้าหมายรอบนี้: {{.AudiencePersona}}

สคริปต์จากนักเขียนแต่ละมุมมอง (JSON array, field lens บอกมุมมอง):
{{.CandidatesJSON}}

ขั้นตอน:
1) ให้คะแนนแต่ละฉบับ 1-10 สามด้าน: hook (พลังหยุดนิ้ว 3 วิแรก), accuracy (ความแม่น ไม่ oversell), audience_fit (ตรง pain และภาษาผู้ชม)
2) เลือกผู้ชนะ 1 ฉบับ — winner_lens ต้องเป็นค่า lens ของฉบับนั้นเป๊ะๆ
3) สร้างฉบับสุดท้าย (final): ใช้ฉบับผู้ชนะเป็นฐาน ยก hook หรือประโยคเด็ดจากฉบับอื่นมาเสริมได้เฉพาะจุดที่ทำให้ดีขึ้นจริง ห้ามเขียนเนื้อหาใหม่เอง ห้ามเพิ่ม claim ที่ไม่ปรากฏในฉบับใดเลย ความยาวใกล้เคียงฉบับผู้ชนะ

ตอบเป็น JSON เท่านั้น ห้ามมีข้อความอื่นนอก JSON:
{"scores":[{"lens":"...","hook":1,"accuracy":1,"audience_fit":1}],"winner_lens":"...","rationale":"เหตุผลสั้นๆ","final":{"answer_script":"...","voice_script":"...","youtube_title":"...","youtube_description":"...","youtube_tags":["..."]}}',
  'claude-sonnet-5',
  0.3,
  TRUE,
  '- final.voice_script ต้องพร้อมพากย์: ไม่มี URL, ไม่มี emoji, ไม่มี markdown
- ถ้าทุกฉบับ accuracy แย่ ให้เลือกฉบับที่เสี่ยงน้อยสุดเป็นผู้ชนะ ห้ามคืนค่าว่าง'
);

-- 4) ต่อท้าย placeholder ใน prompt ของ script (flag ปิด/ไม่มีเลนส์ → แทนด้วยสตริงว่าง ไม่กระทบ)
UPDATE agent_configs
SET prompt_template = prompt_template || E'\n\n{{.DebateLens}}'
WHERE agent_name = 'script'
  AND prompt_template NOT LIKE '%{{.DebateLens}}%';

COMMIT;
```

- [ ] **Step 2: ตรวจไฟล์**

Run: `grep -c 'BEGIN;\|COMMIT;' migrations/058_script_debate.sql && grep -n '{{if' migrations/058_script_debate.sql; go build ./...`
Expected: นับได้ 2 (BEGIN+COMMIT), grep `{{if` ไม่เจอ (exit 1), build ผ่าน
หมายเหตุ: ไม่มี psql local — syntax จริงพิสูจน์ตอน deploy (RunMigrations auto-apply) แล้วตรวจผลผ่าน Neon MCP

- [ ] **Step 3: Commit**

```bash
git add migrations/058_script_debate.sql
git commit -m "migration: 058 script newsroom debate (table + settings + script_judge seed)"
```

---

### Task 7: Final verification + simplify

- [ ] **Step 1: รันทุกอย่าง**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: ผ่านทั้งหมด

- [ ] **Step 2: /simplify diff ทั้ง feature** (ตาม preference ผู้ใช้ — รัน simplify ก่อนขั้น commit สุดท้าย/PR): รัน skill `simplify` บน diff ของ feature นี้ (`git diff 3fe4a34..HEAD`) แล้ว re-run `go test ./...` หลังแก้

- [ ] **Step 3: Commit ผลลัพธ์ simplify (ถ้ามีการแก้)**

```bash
git add -u && git commit -m "refactor(debate): simplify pass"
```

---

## Deploy & Post-Deploy Verification (ทำหลัง merge/push — มีคนอยู่ด้วย)

1. ก่อน push: เช็ค clip ไม่มีตัวไหนค้าง `producing` (production non-durable — ห้าม deploy กลาง production)
2. Push master → Railway auto-deploy ทั้ง backend+frontend → migration 058 auto-apply
3. ตรวจผ่าน Neon MCP (project snowy-grass-75448787): `SELECT agent_name, model, enabled FROM agent_configs WHERE agent_name='script_judge';` และ `SELECT key, LEFT(value,50) FROM settings WHERE key LIKE 'script_debate%';` และ `SELECT COUNT(*) FROM script_debates;` (=0)
4. Produce 1 คลิปโดย flag ยังปิด → ยืนยัน path เดิมปกติ (script_debates ยังว่าง)
5. เปิด flag: `UPDATE settings SET value='true' WHERE key='script_debate_enabled';` → `/produce` 1 คลิป (ระบุ count=1! ไม่ใส่ = 7 คลิป)
6. ตรวจ `SELECT source, jsonb_array_length(candidates), verdict->>'winner_lens', verdict->'scores' FROM script_debates ORDER BY created_at DESC LIMIT 1;` — ต้องมี 3 candidates + verdict สมเหตุสมผล
7. Eyeball คลิปจริง (hook 3 วิแรก + เนื้อหาไม่หลุดแบรนด์) ก่อนปล่อยรัน schedule
8. Rollback: ปิด flag (ไม่ต้อง revert code)

## Self-Review Notes

- Spec coverage: flag+lenses (Task 6), lens injection (Task 1), judge (Task 2+6), fail-open ladder ทุกแถวของ spec (Task 4 tests ครบ: ≥2→judge, judge fail→first candidate, 1→skip judge, 0→single-pass ใน Task 5 `generateScript`), audit (Task 3+5), migration BEGIN/COMMIT (Task 6), verification (Task 7 + Deploy section) — ครบ
- Retry path (`produceClipWithID` จาก retry) วิ่งผ่าน `generateScript` เดียวกัน → debate ใช้กับ retry ด้วยเมื่อ flag เปิด (ตั้งใจ)
- `validateScript(script)` บรรทัด 464 เดิมยังทำงานกับ output ของ debate เหมือนเดิม (title suffix ฯลฯ) — ไม่ต้องแก้
