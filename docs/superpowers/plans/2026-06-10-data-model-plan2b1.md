# Data Model + Migration 030 (Plan 2b-1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lay the additive data foundation for the topic-driven pipeline: new scene fields, a `ResearchBrief` type, new `scenes`/`clips` columns, and seeded `scene` + `metadata` agent rows — all additive so the existing flow keeps working and master stays green.

**Architecture:** Pure additive change. New struct fields default to empty; new DB columns are nullable/defaulted; new agent rows are inserted idempotently. Nothing is wired into the pipeline yet (the new `SceneAgent`/`MetadataAgent` and the orchestrator rewrite are Plans 2b-2 and 2b-3). This plan exists so those later plans have stable types and schema to build on.

**Tech Stack:** Go 1.25, pgx/pgxpool, Postgres (Neon), file-based migrations auto-applied on startup ordered by filename.

**Scope:** Models + repository persistence + one migration. No agent logic, no orchestrator, no frontend. `agent_configs` has `agent_name UNIQUE`, columns: `agent_name, system_prompt, prompt_template, model, temperature, enabled, skills, insights, config`.

---

## File Structure

- **Modify:** `internal/agent/script.go` — add Plan-2b fields to `GeneratedScene` (additive).
- **Create:** `internal/agent/research_brief.go` — `ResearchBrief`, `KeyPoint`, `Stat` types (used by Plan 2b-2).
- **Modify:** `internal/models/clip.go` — add the new columns to the `Scene` struct.
- **Modify:** `internal/models/request.go` — add the new columns to `CreateSceneRequest`.
- **Modify:** `internal/repository/scenes.go` — persist + read the new columns in `Create` and `ListByClip`.
- **Create:** `migrations/030_topic_pipeline_schema.sql` — new columns + `scene`/`metadata` agent rows.

---

## Task 1: Extend GeneratedScene + add ResearchBrief types

**Files:**
- Modify: `internal/agent/script.go`
- Create: `internal/agent/research_brief.go`

- [ ] **Step 1: Add fields to GeneratedScene**

In `internal/agent/script.go`, replace the `GeneratedScene` struct with (adds six fields; existing fields unchanged):

```go
type GeneratedScene struct {
	SceneNumber     int             `json:"scene_number"`
	SceneType       string          `json:"scene_type"`
	TextContent     string          `json:"text_content"`
	VoiceText       string          `json:"voice_text"`
	DurationSeconds float64         `json:"duration_seconds"`
	TextOverlays    json.RawMessage `json:"text_overlays"`
	// Plan 2b additions — populated by the new SceneAgent (Claude). The legacy
	// ScriptAgent leaves these empty, which is valid.
	LayoutVariant string   `json:"layout_variant"`
	OnScreenText  string   `json:"on_screen_text"`
	EmphasisWords []string `json:"emphasis_words"`
	Beat          string   `json:"beat"`
	CaptionStyle  string   `json:"caption_style"`
	ImagePrompt   string   `json:"image_prompt"`
}
```

- [ ] **Step 2: Create the ResearchBrief types**

Create `internal/agent/research_brief.go`:

```go
package agent

// ResearchBrief is the structured output of the topic-driven ResearchAgent:
// a sourced, angle-bearing summary the ScriptAgent turns into narration.
// Wired in Plan 2b-2; defined here so dependent plans share a stable type.
type ResearchBrief struct {
	Topic          string     `json:"topic"`
	CoreMessage    string     `json:"core_message"`
	NarrativeAngle string     `json:"narrative_angle"`
	KeyPoints      []KeyPoint `json:"key_points"`
	Stats          []Stat     `json:"stats"`
}

// KeyPoint is one sourced fact. UseAs is the narrative role (e.g. "hook",
// "problem", "proof") so the script agent knows where it belongs.
type KeyPoint struct {
	Claim      string `json:"claim"`
	SourceURL  string `json:"source_url"`
	Confidence string `json:"confidence"`
	UseAs      string `json:"use_as"`
}

type Stat struct {
	Value     string `json:"value"`
	Context   string `json:"context"`
	SourceURL string `json:"source_url"`
}
```

- [ ] **Step 3: Build the agent package**

Run: `go build ./internal/agent/`
Expected: PASS. (If the go-build cache errors with "operation not permitted", that is a sandbox restriction — rerun with sandbox disabled.)

- [ ] **Step 4: Commit**

```bash
git add internal/agent/script.go internal/agent/research_brief.go
git commit -m "feat(agent): add Plan-2b scene fields + ResearchBrief types"
```

---

## Task 2: Add new columns to Scene model + CreateSceneRequest

**Files:**
- Modify: `internal/models/clip.go`
- Modify: `internal/models/request.go`

- [ ] **Step 1: Extend the Scene struct**

In `internal/models/clip.go`, replace the `Scene` struct with (adds five fields after `TextOverlays`):

```go
type Scene struct {
	ID              string          `json:"id"`
	ClipID          string          `json:"clip_id"`
	SceneNumber     int             `json:"scene_number"`
	SceneType       string          `json:"scene_type"`
	TextContent     string          `json:"text_content"`
	ImagePrompt     string          `json:"image_prompt"`
	Image169URL     *string         `json:"image_16_9_url"`
	Image916URL     *string         `json:"image_9_16_url"`
	VoiceText       string          `json:"voice_text"`
	DurationSeconds float64         `json:"duration_seconds"`
	TextOverlays    json.RawMessage `json:"text_overlays"`
	LayoutVariant   string          `json:"layout_variant"`
	OnScreenText    string          `json:"on_screen_text"`
	EmphasisWords   json.RawMessage `json:"emphasis_words"`
	Beat            string          `json:"beat"`
	CaptionStyle    string          `json:"caption_style"`
}
```

- [ ] **Step 2: Extend CreateSceneRequest**

In `internal/models/request.go`, replace the `CreateSceneRequest` struct with:

```go
type CreateSceneRequest struct {
	ClipID          string          `json:"clip_id"`
	SceneNumber     int             `json:"scene_number"`
	SceneType       string          `json:"scene_type"`
	TextContent     string          `json:"text_content"`
	ImagePrompt     string          `json:"image_prompt"`
	VoiceText       string          `json:"voice_text"`
	DurationSeconds float64         `json:"duration_seconds"`
	TextOverlays    json.RawMessage `json:"text_overlays"`
	LayoutVariant   string          `json:"layout_variant"`
	OnScreenText    string          `json:"on_screen_text"`
	EmphasisWords   json.RawMessage `json:"emphasis_words"`
	Beat            string          `json:"beat"`
	CaptionStyle    string          `json:"caption_style"`
}
```

- [ ] **Step 3: Build models**

Run: `go build ./internal/models/`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/models/clip.go internal/models/request.go
git commit -m "feat(models): add topic-pipeline columns to Scene + CreateSceneRequest"
```

---

## Task 3: Persist new columns in the scenes repository

**Files:**
- Modify: `internal/repository/scenes.go`

The new columns are appended at the end of both queries so existing positional scans for other columns are unaffected. `EmphasisWords` is a `json.RawMessage`; when empty it must be written as `[]` (valid JSONB), not an empty byte slice.

- [ ] **Step 1: Update Create to insert + return the new columns**

In `internal/repository/scenes.go`, replace the `Create` method with:

```go
func (r *ScenesRepo) Create(ctx context.Context, req models.CreateSceneRequest) (*models.Scene, error) {
	emphasis := req.EmphasisWords
	if len(emphasis) == 0 {
		emphasis = []byte("[]")
	}
	var s models.Scene
	err := r.pool.QueryRow(ctx,
		`INSERT INTO scenes (clip_id, scene_number, scene_type, text_content, image_prompt, voice_text, duration_seconds, text_overlays,
		                     layout_variant, on_screen_text, emphasis_words, beat, caption_style)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id, clip_id, scene_number, scene_type, text_content, image_prompt,
		           image_16_9_url, image_9_16_url, voice_text, duration_seconds, text_overlays,
		           layout_variant, on_screen_text, emphasis_words, beat, caption_style`,
		req.ClipID, req.SceneNumber, req.SceneType, req.TextContent,
		req.ImagePrompt, req.VoiceText, req.DurationSeconds, req.TextOverlays,
		req.LayoutVariant, req.OnScreenText, emphasis, req.Beat, req.CaptionStyle,
	).Scan(&s.ID, &s.ClipID, &s.SceneNumber, &s.SceneType,
		&s.TextContent, &s.ImagePrompt, &s.Image169URL, &s.Image916URL,
		&s.VoiceText, &s.DurationSeconds, &s.TextOverlays,
		&s.LayoutVariant, &s.OnScreenText, &s.EmphasisWords, &s.Beat, &s.CaptionStyle)
	if err != nil {
		return nil, fmt.Errorf("create scene: %w", err)
	}
	return &s, nil
}
```

- [ ] **Step 2: Update ListByClip to select + scan the new columns**

In the same file, replace the `ListByClip` method with:

```go
func (r *ScenesRepo) ListByClip(ctx context.Context, clipID string) ([]models.Scene, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, clip_id, scene_number, scene_type, text_content, image_prompt,
		        image_16_9_url, image_9_16_url, voice_text, duration_seconds, text_overlays,
		        layout_variant, on_screen_text, emphasis_words, beat, caption_style
		 FROM scenes WHERE clip_id = $1 ORDER BY scene_number`, clipID)
	if err != nil {
		return nil, fmt.Errorf("query scenes: %w", err)
	}
	defer rows.Close()

	var scenes []models.Scene
	for rows.Next() {
		var s models.Scene
		if err := rows.Scan(&s.ID, &s.ClipID, &s.SceneNumber, &s.SceneType,
			&s.TextContent, &s.ImagePrompt, &s.Image169URL, &s.Image916URL,
			&s.VoiceText, &s.DurationSeconds, &s.TextOverlays,
			&s.LayoutVariant, &s.OnScreenText, &s.EmphasisWords, &s.Beat, &s.CaptionStyle); err != nil {
			return nil, fmt.Errorf("scan scene: %w", err)
		}
		scenes = append(scenes, s)
	}
	return scenes, nil
}
```

- [ ] **Step 3: Build the repository package**

Run: `go build ./internal/repository/`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/repository/scenes.go
git commit -m "feat(repo): persist topic-pipeline columns in scenes Create/ListByClip"
```

---

## Task 4: Migration 030 — columns + scene/metadata agent rows

**Files:**
- Create: `migrations/030_topic_pipeline_schema.sql`

Adds the columns the repo now reads/writes, and seeds the two new agents (`scene` Claude, `metadata` Gemini) with system prompt + prompt template + skills per spec §4.6/§4.7. Idempotent via `IF NOT EXISTS` and `ON CONFLICT (agent_name) DO UPDATE`.

- [ ] **Step 1: Write the migration**

Create `migrations/030_topic_pipeline_schema.sql`:

```sql
-- Migration 030: topic-driven pipeline schema.
-- Additive columns for the SceneAgent output + research brief, and seed rows
-- for the new `scene` (Claude) and `metadata` (Gemini) agents.

ALTER TABLE scenes ADD COLUMN IF NOT EXISTS layout_variant TEXT NOT NULL DEFAULT '';
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS on_screen_text TEXT NOT NULL DEFAULT '';
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS emphasis_words JSONB NOT NULL DEFAULT '[]';
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS beat TEXT NOT NULL DEFAULT '';
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS caption_style TEXT NOT NULL DEFAULT '';

ALTER TABLE clips ADD COLUMN IF NOT EXISTS core_message TEXT NOT NULL DEFAULT '';
ALTER TABLE clips ADD COLUMN IF NOT EXISTS narrative_angle TEXT NOT NULL DEFAULT '';
ALTER TABLE clips ADD COLUMN IF NOT EXISTS research_brief JSONB NOT NULL DEFAULT '{}';

-- Seed the SceneAgent (Claude): script -> 6-10 scene JSON.
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
VALUES (
  'scene',
  'คุณคือ Director ที่แตกสคริปวิดีโอเป็นซีนสำหรับ explainer แนวตั้ง 9:16 ภาษาไทย ให้คนดูเข้าใจง่ายและน่าสนใจ ตอบเป็น JSON เท่านั้น',
  $$แตกสคริปนี้ออกเป็น 6-10 ซีน สำหรับวิดีโอแนวตั้ง 9:16 ยาว {{.TargetDurationSec}} วินาที

สคริป:
{{.Script}}

ธีมแบรนด์: {{.ThemeDescription}}

ตอบเป็น JSON array เท่านั้น แต่ละ object มี:
- "scene_number": ลำดับซีน (เริ่มที่ 1)
- "beat": บทบาทในเรื่อง — หนึ่งใน "hook" | "problem" | "payoff" | "cta"
- "voice_text": ประโยคพากย์ไทยของซีนนี้ (สั้น พูดได้ลื่น)
- "on_screen_text": ข้อความบนจอ สั้น อ่านรู้เรื่องตอนปิดเสียง
- "emphasis_words": array คำที่ต้องไฮไลต์ (1-3 คำ)
- "layout_variant": หนึ่งใน "hook_big" | "hook_punch" | "phrase_block" | "stat_reveal" | "quote_cta" | "word_pop" | "static" | "intro" | "outro"
- "caption_style": "phrase_block" หรือ "word_pop"
- "duration_seconds": ความยาวซีนโดยประมาณ (วินาที)
- "image_prompt": คำอธิบายภาพประกอบซีนนี้แบบสั้น (อังกฤษ) หรือ "" ถ้าซีนนี้ไม่ต้องใช้ภาพ

หนึ่งซีนหนึ่งไอเดีย ห้ามยัดสองความคิดในซีนเดียว$$,
  'claude-sonnet-4-6',
  0.6,
  TRUE,
  'แตกเป็น 6-10 ซีน หนึ่งซีนหนึ่งไอเดีย, เลือก layout_variant ให้เข้าจังหวะ, กำหนด emphasis_words, on_screen_text สั้นอ่านรู้เรื่องตอนปิดเสียง, output JSON ตาม schema'
)
ON CONFLICT (agent_name) DO UPDATE SET
  system_prompt = EXCLUDED.system_prompt,
  prompt_template = EXCLUDED.prompt_template,
  model = EXCLUDED.model,
  temperature = EXCLUDED.temperature,
  skills = EXCLUDED.skills;

-- Seed the MetadataAgent (Gemini): script -> youtube title/desc/tags.
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
VALUES (
  'metadata',
  'คุณสร้าง metadata ยูทูบภาษาไทยแบบ search-intent ตอบเป็น JSON เท่านั้น',
  $$สร้าง metadata สำหรับวิดีโอเรื่อง "{{.Topic}}" หมวด "{{.Category}}"

สคริป:
{{.Script}}

กลุ่มผู้ชม: {{.AudiencePersona}}

ตอบเป็น JSON object เท่านั้น:
- "youtube_title": หัวข้อแบบ search-intent ภาษาไทย ไม่เกิน 55 ตัวอักษร (ระบบจะต่อท้าย " | Ads Vance" ให้เอง ห้ามใส่ชื่อแบรนด์เอง)
- "youtube_description": คำอธิบายกระชับ 2-3 ประโยค
- "youtube_tags": array แท็กภาษาไทย 5-8 คำ$$,
  'gemini-3-5-flash',
  0.6,
  TRUE,
  'title แบบ search-intent ภาษาไทย, ไม่ใส่ชื่อแบรนด์เอง, desc กระชับ, tags ตรงหัวข้อ'
)
ON CONFLICT (agent_name) DO UPDATE SET
  system_prompt = EXCLUDED.system_prompt,
  prompt_template = EXCLUDED.prompt_template,
  model = EXCLUDED.model,
  temperature = EXCLUDED.temperature,
  skills = EXCLUDED.skills;
```

- [ ] **Step 2: Verify the SQL applies cleanly**

If the Neon MCP is available, apply this file's SQL against a temporary branch of project `adsvance-v2` (`snowy-grass-75448787`), then confirm:
```sql
SELECT column_name FROM information_schema.columns
WHERE table_name='scenes' AND column_name IN ('layout_variant','on_screen_text','emphasis_words','beat','caption_style');
SELECT agent_name, model FROM agent_configs WHERE agent_name IN ('scene','metadata');
```
Expected: 5 scene columns present; `scene=claude-sonnet-4-6`, `metadata=gemini-3-5-flash`. Delete the temp branch afterward. If the Neon MCP is unavailable, skip the live check (the migration auto-applies on next deploy) and instead eyeball the SQL for balanced `$$` quoting and matching parentheses.

- [ ] **Step 3: Commit**

```bash
git add migrations/030_topic_pipeline_schema.sql
git commit -m "feat(db): migration 030 — topic-pipeline columns + scene/metadata agents"
```

---

## Task 5: Full verification

- [ ] **Step 1: Build, vet, test the module**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS. The new struct fields and columns are additive; no existing test or call site needs to change. (Sandbox go-build cache error → rerun with sandbox disabled.)

- [ ] **Step 2: Confirm the additive contract held**

Run: `git diff 030..HEAD --stat` is not meaningful; instead run `git diff <base>..HEAD --stat` where `<base>` is the commit before Task 1, and confirm only these files changed: `internal/agent/script.go`, `internal/agent/research_brief.go`, `internal/models/clip.go`, `internal/models/request.go`, `internal/repository/scenes.go`, `migrations/030_topic_pipeline_schema.sql`. No agent logic, orchestrator, or frontend file should appear.

---

## Self-Review Notes

- **Spec coverage:** Implements spec §5 (DB: scene columns `layout_variant`/`on_screen_text`/`emphasis_words`/`beat`/`caption_style`, clip columns `core_message`/`narrative_angle`/`research_brief`; `agent_configs` seed of `scene` + `metadata`) and the type foundation for §4.2 agents. The `image`→`imageprompt` rename, `question` deletion, research/script model rewrites, orchestrator, and frontend are deliberately in Plans 2b-2/2b-3 to keep this increment additive and green.
- **No placeholders:** every struct, SQL statement, and prompt template is written in full.
- **Type consistency:** `GeneratedScene.EmphasisWords` is `[]string` (LLM JSON output convenience); `Scene.EmphasisWords` and `CreateSceneRequest.EmphasisWords` are `json.RawMessage` (DB JSONB passthrough). The orchestrator conversion between them is a Plan 2b-3 concern and is intentionally not added here. `Create` defaults empty `EmphasisWords` to `[]` so JSONB never receives invalid input.
- **Idempotency:** columns use `IF NOT EXISTS`; agent rows use `ON CONFLICT (agent_name) DO UPDATE`, so re-running the migration is safe.
- **Honesty flag:** the `scene`/`metadata` prompt templates are seeded now but not invoked until Plan 2b-2 wires the agents — they will get real-call tuning then.
