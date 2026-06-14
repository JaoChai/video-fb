# Content Critic Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Insert an LLM "Content Critic" agent that reviews and revises generated content (scenes, image prompts, metadata) before render, with a fail-safe that guarantees output is never worse than today.

**Architecture:** A new `CriticAgent` (same pattern as `SceneAgent`) runs in `orchestrator.produceClipWithID` after the SceneAgent and before render. It revises content in place ("Approach A"). A pure `reconcileCritique` function enforces edit boundaries (content fields only — never timing/layout) and falls back to the original bundle on any anomaly. Score + changelog are appended to a new `clip_critiques` table (quality signal for a future Phase 3 learning loop).

**Tech Stack:** Go, pgx/Postgres (Neon), the existing `KieLLMClient.GenerateJSON` LLM wrapper, DB-driven `agent_configs`.

---

## File Structure

- **Create** `internal/agent/critic.go` — `CriticAgent`, input/output types, `Review`, pure `reconcileCritique`.
- **Create** `internal/agent/critic_test.go` — pure tests for `reconcileCritique` + schema unmarshal.
- **Create** `internal/repository/critiques.go` — `CritiquesRepo.Create` (append-only insert).
- **Create** `migrations/033_clip_critiques.sql` — the log table.
- **Create** `migrations/034_critic_agent_config.sql` — seed the `critic` agent_configs row.
- **Modify** `internal/orchestrator/orchestrator.go` — add `criticAgent` + `critiquesRepo` fields, extend `New(...)`, insert review block, move TTS sanitize to after the critic.
- **Modify** `cmd/server/main.go` — construct `NewCriticAgent` + `NewCritiquesRepo`, pass to `orchestrator.New`.

**Edit boundary (whitelist):** the critic may change only `VoiceText`, `OnScreenText`, `TextContent`, `ImagePrompt`, `EmphasisWords`, and the three metadata fields. Everything else on a scene (`SceneNumber`, `SceneType`, `LayoutVariant`, `Layout`, `Content`, `DurationSeconds`, `Beat`, `CaptionStyle`, `TextOverlays`) is copied from the original, so the structured/typed render path cannot break.

---

## Task 1: Migration — `clip_critiques` table

**Files:**
- Create: `migrations/033_clip_critiques.sql`

- [ ] **Step 1: Write the migration**

`clips.id` is `UUID` (see `migrations/001_initial_schema.sql`), so `clip_id` is a UUID FK matching the `scenes` table convention.

```sql
-- 033_clip_critiques.sql
-- Append-only log of Content Critic reviews. Phase 1 only writes this table;
-- a future Phase 3 (learning loop) reads it to find recurring low-score
-- patterns and tune upstream agents' skills. `applied` is FALSE when the
-- fail-safe kept the original content.
CREATE TABLE IF NOT EXISTS clip_critiques (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id    UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    score      JSONB NOT NULL DEFAULT '{}'::jsonb,
    changes    JSONB NOT NULL DEFAULT '[]'::jsonb,
    applied    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clip_critiques_clip_id ON clip_critiques (clip_id);
```

- [ ] **Step 2: Commit**

```bash
git add migrations/033_clip_critiques.sql
git commit -m "feat(db): clip_critiques log table for Content Critic"
```

(The migration is applied later with `make migrate` against the configured DB; no code depends on the table existing at build/test time.)

---

## Task 2: Migration — seed the `critic` agent config

**Files:**
- Create: `migrations/034_critic_agent_config.sql`

- [ ] **Step 1: Write the seed migration**

Column list mirrors the working precedent in `migrations/030_topic_pipeline_schema.sql` (`agent_name, system_prompt, prompt_template, model, temperature, enabled, skills`); other columns (`insights`, `config`) take their table defaults. `model` is `claude-sonnet-4-6` (routed by `KieLLMClient` prefix, same as the `scene` agent).

```sql
-- 034_critic_agent_config.sql
-- Seed the Content Critic agent: reviews generated content (scenes, image
-- prompts, metadata) and revises it in place before render. Kill switch:
-- UPDATE agent_configs SET enabled = FALSE WHERE agent_name = 'critic';
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
SELECT
  'critic',
  'คุณคือ Content Critic ของ Ads Vance — บรรณาธิการวิดีโอสั้นภาษาไทยสายการเงิน/โฆษณา Meta. รับเนื้อหาที่ทีมสร้างมา (scenes, image_prompt, metadata) แล้วปรับให้ดีขึ้น "เท่าที่จำเป็น" โดยไม่รื้อโครงสร้าง.

เกณฑ์ตรวจ:
- Hook (scene แรก): ต้องดึงให้ดูต่อใน 2-3 วินาทีแรก (ตัวเลขช็อก/คำถามที่โดนความกลัว เช่น โดนแบน เสียเงิน บัญชีปิด).
- ภาษาไทยไหลลื่นแบบพูด ไม่แข็ง ไม่กำกวม.
- แต่ละ scene สื่อสารเรื่องเดียวจบ ชัดเจน.
- ตรงแบรนด์/persona Ads Vance (มืออาชีพ เป็นกันเอง).
- image_prompt: ตรงแบรนด์ (navy+ส้ม มาสคอตเสือดาว), ไม่มีตัวหนังสือในรูป, เข้ากับเนื้อ scene.
- metadata: title น่าคลิก ตรง search intent ไม่ clickbait เกินจริง.

ข้อห้ามเด็ดขาด:
- ห้ามเปลี่ยนจำนวน scene, scene_number, duration_seconds, layout, scene_type.
- ปรับได้เฉพาะ voice_text, on_screen_text, text_content, image_prompt, emphasis_words และ metadata.
- ถ้าเนื้อหาดีอยู่แล้ว ไม่ต้องแก้ คืนของเดิมได้ (changes ว่าง).
ตอบเป็น JSON object เท่านั้น.',
  'คำถามต้นทาง: {{.Question}}

บทพากย์รวม: {{.Narration}}

เนื้อหาที่ต้องตรวจ (JSON):
{{.InputJSON}}

จงคืน JSON object รูปแบบนี้เท่านั้น (ห้ามมีข้อความอื่นนอก JSON):
{
  "scenes": [ /* scene เดิมทุกตัว ใส่ค่าที่ปรับแล้ว คง scene_number/duration_seconds/layout/scene_type เดิม */ ],
  "metadata": { "youtube_title": "...", "youtube_description": "...", "youtube_tags": ["..."] },
  "score": { "hook": 8, "clarity": 7, "brand_fit": 9, "overall": 8 },
  "changes": [ { "field": "scene[0].voice_text", "reason": "hook ไม่ดึงใน 2 วิแรก" } ]
}',
  'claude-sonnet-4-6',
  0.3,
  TRUE,
  '- hook สายนี้: เปิดด้วยตัวเลขช็อกหรือคำถามที่โดนความกลัว (โดนแบน/เสียเงิน/บัญชีปิด).
- เลี่ยงศัพท์ทางการเกินไป ใช้คำที่ลูกค้าจริงพูด.
- CTA ปลายคลิป: ชวนทักเพจ ไม่ฮาร์ดเซล.'
WHERE NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'critic');
```

- [ ] **Step 2: Commit**

```bash
git add migrations/034_critic_agent_config.sql
git commit -m "feat(db): seed critic agent config"
```

---

## Task 3: CriticAgent types + `reconcileCritique` (pure, TDD)

**Files:**
- Create: `internal/agent/critic.go`
- Test: `internal/agent/critic_test.go`

- [ ] **Step 1: Write the types + the pure reconcile function**

Create `internal/agent/critic.go`. Task 3 only needs the `strings` import; Task 4
adds the rest when `Review` lands.

```go
package agent

import "strings"

// CriticMetadata mirrors the YouTube metadata fields the critic may revise.
type CriticMetadata struct {
	YoutubeTitle       string   `json:"youtube_title"`
	YoutubeDescription string   `json:"youtube_description"`
	YoutubeTags        []string `json:"youtube_tags"`
}

// CriticInput is everything the critic reviews for one clip.
type CriticInput struct {
	Question  string
	Narration string
	Scenes    []GeneratedScene
	Metadata  CriticMetadata
}

// CriticScore is the per-dimension quality score (each 1-10).
type CriticScore struct {
	Hook     int `json:"hook"`
	Clarity  int `json:"clarity"`
	BrandFit int `json:"brand_fit"`
	Overall  int `json:"overall"`
}

// CriticChange is one human-readable edit the critic made.
type CriticChange struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// CriticOutput is the raw JSON the LLM returns.
type CriticOutput struct {
	Scenes   []GeneratedScene `json:"scenes"`
	Metadata CriticMetadata   `json:"metadata"`
	Score    CriticScore      `json:"score"`
	Changes  []CriticChange   `json:"changes"`
}

// CriticResult is what Review hands back to the orchestrator: the content to
// actually use (the original bundle when the fail-safe triggered) plus the
// score/changes to log. Applied is false when the original was kept.
type CriticResult struct {
	Scenes   []GeneratedScene
	Metadata CriticMetadata
	Score    CriticScore
	Changes  []CriticChange
	Applied  bool
}

// scoreInRange reports whether every score dimension is within 1-10.
func scoreInRange(s CriticScore) bool {
	for _, v := range []int{s.Hook, s.Clarity, s.BrandFit, s.Overall} {
		if v < 1 || v > 10 {
			return false
		}
	}
	return true
}

// reconcileCritique merges the critic's output onto the original content and
// enforces the edit boundary. It returns the original bundle (Applied=false) on
// ANY anomaly, so the pipeline can never render something worse than the input.
// Only content fields are taken from the critic; all structural/timing/layout
// fields are copied from the original scene.
func reconcileCritique(in CriticInput, out CriticOutput) CriticResult {
	fail := CriticResult{
		Scenes:   in.Scenes,
		Metadata: in.Metadata,
		Score:    out.Score,
		Changes:  out.Changes,
		Applied:  false,
	}

	if len(out.Scenes) != len(in.Scenes) || !scoreInRange(out.Score) {
		return fail
	}

	critByNum := make(map[int]GeneratedScene, len(out.Scenes))
	for _, cs := range out.Scenes {
		if _, dup := critByNum[cs.SceneNumber]; dup {
			return fail // duplicate scene_number
		}
		critByNum[cs.SceneNumber] = cs
	}

	merged := make([]GeneratedScene, len(in.Scenes))
	for i, orig := range in.Scenes {
		cs, ok := critByNum[orig.SceneNumber]
		if !ok {
			return fail // critic dropped or renumbered a scene
		}
		if strings.TrimSpace(cs.VoiceText) == "" {
			return fail // never ship an empty narration
		}
		m := orig // copy keeps every structural/timing/layout field
		m.VoiceText = cs.VoiceText
		m.OnScreenText = cs.OnScreenText
		m.TextContent = cs.TextContent
		m.ImagePrompt = cs.ImagePrompt
		m.EmphasisWords = cs.EmphasisWords
		merged[i] = m
	}

	meta := in.Metadata
	if s := strings.TrimSpace(out.Metadata.YoutubeTitle); s != "" {
		meta.YoutubeTitle = s
	}
	if s := strings.TrimSpace(out.Metadata.YoutubeDescription); s != "" {
		meta.YoutubeDescription = s
	}
	if len(out.Metadata.YoutubeTags) > 0 {
		meta.YoutubeTags = out.Metadata.YoutubeTags
	}

	return CriticResult{
		Scenes:   merged,
		Metadata: meta,
		Score:    out.Score,
		Changes:  out.Changes,
		Applied:  true,
	}
}

// (Review is added in Task 4.)
```

The file compiles standalone in Task 3: `reconcileCritique` and the types use only the `strings` import.

- [ ] **Step 2: Write the failing tests**

Create `internal/agent/critic_test.go`:

```go
package agent

import (
	"encoding/json"
	"testing"
)

// twoScenes builds a minimal valid CriticInput with two scenes.
func twoScenesInput() CriticInput {
	return CriticInput{
		Question:  "บัญชีโฆษณาโดนแบนทำไง",
		Narration: "...",
		Scenes: []GeneratedScene{
			{SceneNumber: 1, SceneType: "hook", Layout: "hook", LayoutVariant: "hook_big",
				DurationSeconds: 4.5, VoiceText: "ของเดิม 1", ImagePrompt: "img1"},
			{SceneNumber: 2, SceneType: "cta", Layout: "cta", LayoutVariant: "phrase_block",
				DurationSeconds: 6, VoiceText: "ของเดิม 2", ImagePrompt: "img2"},
		},
		Metadata: CriticMetadata{YoutubeTitle: "เดิม", YoutubeDescription: "เดิม", YoutubeTags: []string{"a"}},
	}
}

func goodScore() CriticScore { return CriticScore{Hook: 8, Clarity: 7, BrandFit: 9, Overall: 8} }

func TestReconcile_HappyPath_AppliesContentEdits(t *testing.T) {
	in := twoScenesInput()
	out := CriticOutput{
		Scenes: []GeneratedScene{
			{SceneNumber: 1, VoiceText: "ปรับแล้ว 1", ImagePrompt: "img1-better",
				DurationSeconds: 999, SceneType: "WRONG", Layout: "WRONG"},
			{SceneNumber: 2, VoiceText: "ปรับแล้ว 2", ImagePrompt: "img2-better"},
		},
		Metadata: CriticMetadata{YoutubeTitle: "ใหม่", YoutubeTags: []string{"x", "y"}},
		Score:    goodScore(),
		Changes:  []CriticChange{{Field: "scene[0].voice_text", Reason: "คมขึ้น"}},
	}
	got := reconcileCritique(in, out)

	if !got.Applied {
		t.Fatal("Applied = false, want true")
	}
	if got.Scenes[0].VoiceText != "ปรับแล้ว 1" {
		t.Errorf("VoiceText not applied: %q", got.Scenes[0].VoiceText)
	}
	// Immutable fields must come from the ORIGINAL, not the critic.
	if got.Scenes[0].DurationSeconds != 4.5 {
		t.Errorf("DurationSeconds = %v, want 4.5 (original)", got.Scenes[0].DurationSeconds)
	}
	if got.Scenes[0].SceneType != "hook" || got.Scenes[0].Layout != "hook" {
		t.Errorf("layout/type drifted: %q / %q", got.Scenes[0].SceneType, got.Scenes[0].Layout)
	}
	if got.Metadata.YoutubeTitle != "ใหม่" {
		t.Errorf("title not applied: %q", got.Metadata.YoutubeTitle)
	}
	// Empty description in output must keep the original.
	if got.Metadata.YoutubeDescription != "เดิม" {
		t.Errorf("description should keep original, got %q", got.Metadata.YoutubeDescription)
	}
}

func TestReconcile_CountMismatch_FailsSafe(t *testing.T) {
	in := twoScenesInput()
	out := CriticOutput{
		Scenes: []GeneratedScene{{SceneNumber: 1, VoiceText: "x"}}, // only 1
		Score:  goodScore(),
	}
	got := reconcileCritique(in, out)
	if got.Applied {
		t.Fatal("Applied = true, want false on count mismatch")
	}
	if got.Scenes[0].VoiceText != "ของเดิม 1" {
		t.Errorf("did not return original scenes")
	}
}

func TestReconcile_EmptyVoice_FailsSafe(t *testing.T) {
	in := twoScenesInput()
	out := CriticOutput{
		Scenes: []GeneratedScene{
			{SceneNumber: 1, VoiceText: "   "},
			{SceneNumber: 2, VoiceText: "ok"},
		},
		Score: goodScore(),
	}
	if reconcileCritique(in, out).Applied {
		t.Fatal("Applied = true, want false on empty voice_text")
	}
}

func TestReconcile_ScoreOutOfRange_FailsSafe(t *testing.T) {
	in := twoScenesInput()
	out := CriticOutput{
		Scenes: []GeneratedScene{{SceneNumber: 1, VoiceText: "a"}, {SceneNumber: 2, VoiceText: "b"}},
		Score:  CriticScore{Hook: 11, Clarity: 5, BrandFit: 5, Overall: 5},
	}
	if reconcileCritique(in, out).Applied {
		t.Fatal("Applied = true, want false on score out of range")
	}
}

func TestReconcile_UnknownSceneNumber_FailsSafe(t *testing.T) {
	in := twoScenesInput()
	out := CriticOutput{
		Scenes: []GeneratedScene{{SceneNumber: 1, VoiceText: "a"}, {SceneNumber: 9, VoiceText: "b"}},
		Score:  goodScore(),
	}
	if reconcileCritique(in, out).Applied {
		t.Fatal("Applied = true, want false on unknown scene_number")
	}
}

// Locks the prompt↔struct contract: the JSON the critic is told to emit must
// unmarshal cleanly into CriticOutput.
func TestCriticOutputParsesSchema(t *testing.T) {
	raw := `{
	  "scenes": [ { "scene_number": 1, "voice_text": "hi", "image_prompt": "p" } ],
	  "metadata": { "youtube_title": "t", "youtube_description": "d", "youtube_tags": ["a","b"] },
	  "score": { "hook": 8, "clarity": 7, "brand_fit": 9, "overall": 8 },
	  "changes": [ { "field": "scene[0].voice_text", "reason": "r" } ]
	}`
	var out CriticOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("CriticOutput did not unmarshal: %v", err)
	}
	if out.Score.Hook != 8 || out.Metadata.YoutubeTitle != "t" || len(out.Changes) != 1 {
		t.Errorf("unexpected parse: %+v", out)
	}
}
```

- [ ] **Step 3: Run the tests to verify they pass**

Run: `go test ./internal/agent/ -run 'TestReconcile|TestCriticOutput' -v`
Expected: PASS (all 6 tests). `reconcileCritique` is pure, so no LLM mock is needed.

- [ ] **Step 4: Commit**

```bash
git add internal/agent/critic.go internal/agent/critic_test.go
git commit -m "feat(agent): CriticAgent types + reconcileCritique with fail-safe"
```

---

## Task 4: CriticAgent.Review method

**Files:**
- Modify: `internal/agent/critic.go`

- [ ] **Step 1: Widen the import block and add Review**

Change the top of `internal/agent/critic.go` from `import "strings"` to:

```go
import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)
```

Append to `internal/agent/critic.go`:

```go
// CriticTemplateData fills the seeded `critic` prompt_template.
type CriticTemplateData struct {
	Question  string
	Narration string
	InputJSON string
}

// CriticAgent reviews a clip's generated content and revises it in place. Runs
// on Claude (cfg.Model), same KieLLMClient path as the other agents.
type CriticAgent struct {
	llm *KieLLMClient
}

func NewCriticAgent(llm *KieLLMClient) *CriticAgent {
	return &CriticAgent{llm: llm}
}

// Review never returns an error: on any failure it returns the ORIGINAL content
// (Applied=false) so the caller can render unchanged. cfg is the `critic`
// AgentConfig fetched by the caller via GetByName.
func (a *CriticAgent) Review(ctx context.Context, in CriticInput, cfg *models.AgentConfig) CriticResult {
	orig := CriticResult{Scenes: in.Scenes, Metadata: in.Metadata, Applied: false}

	inputJSON, err := json.Marshal(struct {
		Scenes   []GeneratedScene `json:"scenes"`
		Metadata CriticMetadata   `json:"metadata"`
	}{in.Scenes, in.Metadata})
	if err != nil {
		return orig
	}

	userPrompt, err := renderTemplate(cfg.PromptTemplate, CriticTemplateData{
		Question:  in.Question,
		Narration: in.Narration,
		InputJSON: string(inputJSON),
	})
	if err != nil {
		return orig
	}

	var out CriticOutput
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &out); err != nil {
		return orig
	}
	return reconcileCritique(in, out)
}
```

`renderTemplate` is the shared helper already used by `SceneAgent` (see `internal/agent/scene.go` / `template.go`).

- [ ] **Step 2: Verify it builds and existing tests still pass**

Run: `go build ./internal/agent/ && go test ./internal/agent/ -run 'TestReconcile|TestCriticOutput' -v`
Expected: build OK, 6 tests PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/agent/critic.go
git commit -m "feat(agent): CriticAgent.Review (fail-safe LLM review)"
```

---

## Task 5: CritiquesRepo

**Files:**
- Create: `internal/repository/critiques.go`

- [ ] **Step 1: Write the repo**

Takes pre-marshalled JSON bytes so the repository stays decoupled from the `agent` package (the orchestrator does the marshalling).

```go
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CritiquesRepo struct {
	pool *pgxpool.Pool
}

func NewCritiquesRepo(pool *pgxpool.Pool) *CritiquesRepo {
	return &CritiquesRepo{pool: pool}
}

// Create appends one critique row. score and changes are JSON-encoded bytes.
func (r *CritiquesRepo) Create(ctx context.Context, clipID string, score, changes []byte, applied bool) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_critiques (clip_id, score, changes, applied) VALUES ($1, $2, $3, $4)`,
		clipID, score, changes, applied)
	return err
}
```

Confirm the import path matches the other repos: open `internal/repository/clips.go` and copy its `pgxpool` import line verbatim if it differs.

- [ ] **Step 2: Verify it builds**

Run: `go build ./internal/repository/`
Expected: build OK.

- [ ] **Step 3: Commit**

```bash
git add internal/repository/critiques.go
git commit -m "feat(repo): CritiquesRepo append-only Create"
```

---

## Task 6: Wire the critic into the orchestrator

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`

- [ ] **Step 1: Add the two struct fields**

In the `Orchestrator` struct, after the `sceneAgent *agent.SceneAgent` line add:

```go
	criticAgent   *agent.CriticAgent
```

and after `scenesRepo    *repository.ScenesRepo` add:

```go
	critiquesRepo *repository.CritiquesRepo
```

- [ ] **Step 2: Extend the `New(...)` constructor**

Change the signature to add `ca` after `sca` and `critiques` after `scenes`:

```go
func New(
	qa *agent.QuestionAgent,
	sa *agent.ScriptAgent,
	ia *agent.ImageAgent,
	sca *agent.SceneAgent,
	ca *agent.CriticAgent,
	prod *producer.Producer,
	clips *repository.ClipsRepo,
	scenes *repository.ScenesRepo,
	critiques *repository.CritiquesRepo,
	themes *repository.ThemesRepo,
	agents *repository.AgentsRepo,
	settings *repository.SettingsRepo,
	formats *repository.FormatsRepo,
	tracker *progress.Tracker,
) *Orchestrator {
	return &Orchestrator{
		settingsRepo: settings, formatsRepo: formats, questionAgent: qa, scriptAgent: sa, imageAgent: ia,
		sceneAgent: sca, criticAgent: ca,
		producer: prod, clipsRepo: clips, scenesRepo: scenes, critiquesRepo: critiques,
		themesRepo: themes, agentsRepo: agents, tracker: tracker,
	}
}
```

- [ ] **Step 3: Insert the critic block + move the sanitize loop**

In `produceClipWithID`, find this existing block:

```go
	narration := scriptNarration(script)
	scenes, err := o.sceneAgent.Generate(ctx, narration, targetSceneCount, targetDurationSec, theme, sceneCfg)
	if err != nil {
		o.tracker.FailStep("scene", err)
		return o.failClip(ctx, clipID, fmt.Errorf("scene breakdown: %w", err))
	}
	// Sanitize each scene's narration for TTS (brand aliases, strip URLs/@handles).
	for i := range scenes {
		scenes[i].VoiceText = sanitizeVoiceText(scenes[i].VoiceText, brandAliases)
	}
	o.tracker.CompleteStep("scene")
```

Replace it with (sanitize moves to AFTER the critic so it cleans whatever the critic produced):

```go
	narration := scriptNarration(script)
	scenes, err := o.sceneAgent.Generate(ctx, narration, targetSceneCount, targetDurationSec, theme, sceneCfg)
	if err != nil {
		o.tracker.FailStep("scene", err)
		return o.failClip(ctx, clipID, fmt.Errorf("scene breakdown: %w", err))
	}
	o.tracker.CompleteStep("scene")

	// ── Content Critic: review + revise content before render. Optional gate;
	//    on disable/error/anomaly it returns the original content unchanged. ──
	if criticCfg, cErr := o.agentsRepo.GetByName(ctx, "critic"); cErr == nil && criticCfg.Enabled {
		o.tracker.StartStep("critic")
		res := o.criticAgent.Review(ctx, agent.CriticInput{
			Question:  q.Question,
			Narration: narration,
			Scenes:    scenes,
			Metadata: agent.CriticMetadata{
				YoutubeTitle:       script.YoutubeTitle,
				YoutubeDescription: script.YoutubeDescription,
				YoutubeTags:        script.YoutubeTags,
			},
		}, criticCfg)
		scenes = res.Scenes
		script.YoutubeTitle = res.Metadata.YoutubeTitle
		script.YoutubeDescription = res.Metadata.YoutubeDescription
		script.YoutubeTags = res.Metadata.YoutubeTags
		if scoreJSON, mErr := json.Marshal(res.Score); mErr == nil {
			changesJSON, _ := json.Marshal(res.Changes)
			if cErr := o.critiquesRepo.Create(ctx, clipID, scoreJSON, changesJSON, res.Applied); cErr != nil {
				log.Printf("critic: persist critique failed (non-fatal): %v", cErr)
			}
		}
		o.tracker.CompleteStep("critic")
	}

	// Sanitize each scene's narration for TTS (brand aliases, strip URLs/@handles).
	// Runs after the critic so any rewritten voice_text is cleaned too.
	for i := range scenes {
		scenes[i].VoiceText = sanitizeVoiceText(scenes[i].VoiceText, brandAliases)
	}
```

`json` and `log` are already imported in this file. No new imports needed.

- [ ] **Step 4: Verify it builds**

Run: `go build ./internal/orchestrator/`
Expected: PASS — this package compiles on its own. (`go build ./...` will still fail because `cmd/server/main.go` calls the old `New(...)` signature; that caller is fixed in Task 7.)

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go
git commit -m "feat(orchestrator): run Content Critic before render"
```

---

## Task 7: Wire construction in main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Construct the new agent + repo and pass them in**

After the line `sceneAgent := agent.NewSceneAgent(llm)` add:

```go
	criticAgent := agent.NewCriticAgent(llm)
```

After the line `scenesRepo := repository.NewScenesRepo(pool)` add:

```go
	critiquesRepo := repository.NewCritiquesRepo(pool)
```

Change the `orchestrator.New(...)` call to insert `criticAgent` after `sceneAgent` and `critiquesRepo` after `scenesRepo`:

```go
	orch := orchestrator.New(questionAgent, scriptAgent, imageAgent, sceneAgent, criticAgent, prod,
		clipsRepo, scenesRepo, critiquesRepo, themesRepo, agentsRepo, settingsRepo, formatsRepo, tracker)
```

- [ ] **Step 2: Verify the whole project builds**

Run: `go build ./...`
Expected: build OK (exit 0).

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(main): construct and wire CriticAgent + CritiquesRepo"
```

---

## Task 8: Full verification

- [ ] **Step 1: Build, vet, and run the full test suite**

Run:
```bash
go build ./... && go vet ./internal/agent/ ./internal/orchestrator/ ./internal/repository/ && go test ./internal/agent/ -v
```
Expected: build OK, vet clean, all agent tests PASS (including the 6 new critic tests).

- [ ] **Step 2: Sanity-check the migrations apply (requires DB)**

Only if a dev/staging DB is configured:
```bash
make migrate
```
Expected: `033_clip_critiques.sql` and `034_critic_agent_config.sql` apply with no error; re-running is a no-op (guards are idempotent).

- [ ] **Step 3: Final commit (if anything outstanding)**

```bash
git status
```
Expected: clean working tree; all changes committed across Tasks 1-7.

---

## Notes / deliberate Phase 1 boundaries

- **`Content` (structured per-layout JSON) is NOT editable by the critic.** On-screen text for Style-B scenes largely lives in `Content`, which `scene_adapter.go` parses per layout type. Letting the critic rewrite it risks breaking that typed parse. Phase 1 keeps the critic to `voice_text` (the narration + captions, the highest-value surface), `image_prompt`, and metadata. Editing structured `Content` can be a later increment.
- **`clip_critiques` is written even when `Applied=false`** (fail-safe kept the original), so Phase 3 can distinguish "review ran, kept original" from "never reviewed".
- **No re-render**: the critic runs before the single render, so it adds one LLM call and zero extra renders.
```
