# Auto-Review Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an autonomous "auto_review" agent that re-judges `needs_review` clips on a scheduler tick — approving Visual-QA false positives, re-rendering fixable defects, and holding genuine ones for the human.

**Architecture:** A new vision agent (`AutoReviewAgent`) makes one holistic multi-frame decision per clip. A new `Orchestrator.AutoReviewPending` method processes the `needs_review` queue and is invoked by a new `auto_review` scheduler tick every 10 min. Decisions are fail-closed on approve (any error/low-confidence → hold), audited in a new `auto_reviews` table, and gated by a single agent-enable flag. `renderAndFinalize` is untouched.

**Tech Stack:** Go, pgx/pgxpool, go-chi router, robfig/cron, kie.ai Claude vision (`GenerateVisionJSON`), custom SQL migrator.

## Global Constraints

- Migrations: plain idempotent SQL only (`CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, `INSERT ... WHERE NOT EXISTS`). NO goose syntax. Migrator runs the whole file and tracks by filename — never edit an applied migration; always add a new numbered file. Next number: **045**.
- New agent seeded `enabled=TRUE`, `model='claude-sonnet-5'`, `temperature=0.2`.
- Vision is Claude-only (`GenerateVisionJSON` rejects non-`claude-*` models).
- Import root: `github.com/jaochai/video-fb/internal/...`.
- Constants (code, adjustable): approve confidence threshold `0.8`, review retry cap `2`, per-tick batch `5`, tick cron `*/10 * * * *`.
- Fail-closed: the agent may NEVER auto-approve on error or low confidence — coerce to `hold`.
- Every task ends with `go build ./...` passing and a commit. Run tests with `dangerouslyDisableSandbox` if the go build cache is blocked.

---

## File Structure

- Create `internal/agent/autoreview.go` — the agent: input/output types, decision normalization (pure, unit-tested), `Judge()` (one multi-image vision call).
- Create `internal/agent/autoreview_test.go` — unit tests for decision normalization.
- Create `internal/repository/autoreviews.go` — `AutoReviewsRepo` (append-only `Create` + `GetByClip`).
- Create `internal/handler/autoreviews.go` — `GET /api/v1/clips/{clipId}/auto-review`.
- Create `migrations/045_auto_review.sql` — `auto_reviews` table, two `clips` columns, agent-config seed, schedule row.
- Modify `internal/models/clip.go` — add `AutoReviewHeld`, `ReviewRetryCount` fields to `Clip`.
- Modify `internal/repository/clips.go` — `clipColumns`, `scanClip`, + `ListNeedsReview`, `SetAutoReviewHeld`, `IncrementReviewRetry`.
- Modify `internal/orchestrator/orchestrator.go` — struct deps `autoReviewAgent`, `autoReviewsRepo`; `New(...)` signature; `AutoReviewPending`, `autoReviewOne`, `autoReviewFrames`, `downloadToTemp`, `sceneMids` helpers.
- Modify `internal/scheduler/scheduler.go` — add `case "auto_review"` to `handlerFor`.
- Modify `internal/router/router.go` — register the new route.
- Modify the composition root (where `orchestrator.New(...)` is called — `cmd/server/main.go` or `internal/router/router.go`) — construct + inject the new agent and repo.
- (Optional, Task 9) `frontend/src/components/ReviewDialog.tsx` + `frontend/src/api.ts` — show auto-review decision.

---

## Task 1: Migration — table, columns, seed, schedule

**Files:**
- Create: `migrations/045_auto_review.sql`

- [ ] **Step 1: Write the migration**

```sql
-- 045_auto_review.sql
-- Auto-review agent: append-only decision log + per-clip state for the
-- needs_review queue processor. Idempotent; no goose syntax.

CREATE TABLE IF NOT EXISTS auto_reviews (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id     UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    decision    TEXT NOT NULL,              -- approve | retry | hold
    confidence  DOUBLE PRECISION NOT NULL DEFAULT 0,
    defect_type TEXT NOT NULL DEFAULT 'none',-- none | stochastic | deterministic
    reasons     JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_auto_reviews_clip_id ON auto_reviews (clip_id);

-- Per-clip auto-review state. review_retry_count is SEPARATE from retry_count
-- (which counts crash/failed retries) so the two never interfere.
ALTER TABLE clips ADD COLUMN IF NOT EXISTS auto_review_held  BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE clips ADD COLUMN IF NOT EXISTS review_retry_count INT    NOT NULL DEFAULT 0;

-- Seed the agent config (enabled). Prompt mirrors the visual_qa style.
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
SELECT
  'auto_review',
  'คุณคือ Senior Reviewer ของ Ads Vance ผู้ตัดสินรอง (second opinion) ต่อจาก Visual QA. Visual QA (ซึ่ง fail-open) จับว่าคลิปนี้มีตำหนิ. หน้าที่คุณคือดู "เฟรมจริง" ทุกซีนแล้วตัดสินว่าคลิปนี้เผยแพร่ได้จริงไหม โดยแยกให้ออกระหว่าง (1) false positive — QA ระวังเกินไป ภาพจริงโอเค, (2) ตำหนิจริงแบบสุ่ม (AI artifact, มือ/หน้าเพี้ยน) ที่ re-render ใหม่น่าจะหาย, (3) ตำหนิจริงแบบ deterministic (caption ล้นกรอบ, สีหลุดแบรนด์, ตัวหนังสือ baked-in ผิด) ที่ re-render ไม่ช่วย. ตัดสิน approve เฉพาะเมื่อมั่นใจว่าเผยแพร่ได้จริง เพราะตำหนิที่หลุดไปกระทบแบรนด์ลูกค้า.',
  '',
  'claude-sonnet-5',
  0.2,
  TRUE,
  '- decision=approve เฉพาะเมื่อภาพจริงเผยแพร่ได้ (false positive ของ QA)
- decision=retry เมื่อเป็นตำหนิจริงแบบสุ่มที่ re-render น่าจะหาย → ตั้ง defect_type=stochastic
- decision=hold เมื่อเป็นตำหนิ deterministic หรือคุณไม่มั่นใจ → ตั้ง defect_type=deterministic หรือ none
- confidence 0-1 สะท้อนความมั่นใจใน decision
- reasons: ภาษาไทยสั้น ๆ อธิบายว่าทำไม'
WHERE NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'auto_review');

-- Seed the tick (every 10 min).
INSERT INTO schedules (name, cron_expression, action, enabled)
SELECT 'Auto Review', '*/10 * * * *', 'auto_review', TRUE
WHERE NOT EXISTS (SELECT 1 FROM schedules WHERE action = 'auto_review');
```

- [ ] **Step 2: Verify it builds/parses** — `go build ./...` (no code refs yet; this just confirms nothing broke). Expected: success.
- [ ] **Step 3: Commit**

```bash
git add migrations/045_auto_review.sql
git commit -m "feat(auto-review): migration — auto_reviews table, clip state, agent+schedule seed"
```

---

## Task 2: AutoReviewAgent — types + decision normalization (pure, TDD)

**Files:**
- Create: `internal/agent/autoreview.go`
- Test: `internal/agent/autoreview_test.go`

**Interfaces:**
- Produces: `AutoReviewDecision`, `AutoReviewInput`, `AutoReviewResult`, `NewAutoReviewAgent(llm *KieLLMClient) *AutoReviewAgent`, `(*AutoReviewAgent).Judge(ctx, AutoReviewInput, *models.AgentConfig) AutoReviewResult`, and the pure `normalizeAutoReview(raw AutoReviewDecision, threshold float64) AutoReviewResult`.
- Reuses `QAFrame` (from `visualqa.go`), `KieLLMClient.GenerateVisionJSON`, `AgentConfig.BuildSystemPrompt`.

- [ ] **Step 1: Write the failing test** (`internal/agent/autoreview_test.go`)

```go
package agent

import "testing"

func TestNormalizeAutoReview(t *testing.T) {
	cases := []struct {
		name      string
		raw       AutoReviewDecision
		threshold float64
		want      string
	}{
		{"approve high confidence", AutoReviewDecision{Decision: "approve", Confidence: 0.9}, 0.8, "approve"},
		{"approve below threshold -> hold", AutoReviewDecision{Decision: "approve", Confidence: 0.6}, 0.8, "hold"},
		{"retry passes through", AutoReviewDecision{Decision: "retry", Confidence: 0.9}, 0.8, "retry"},
		{"hold passes through", AutoReviewDecision{Decision: "hold", Confidence: 0.9}, 0.8, "hold"},
		{"unknown decision -> hold", AutoReviewDecision{Decision: "garbage", Confidence: 0.99}, 0.8, "hold"},
		{"empty decision -> hold", AutoReviewDecision{Decision: "", Confidence: 0.99}, 0.8, "hold"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeAutoReview(c.raw, c.threshold)
			if got.Decision != c.want {
				t.Fatalf("normalizeAutoReview(%+v) = %q, want %q", c.raw, got.Decision, c.want)
			}
		})
	}
}

func TestNormalizeAutoReviewError(t *testing.T) {
	got := autoReviewError("vision failed")
	if got.Decision != "hold" {
		t.Fatalf("autoReviewError decision = %q, want hold (fail-closed)", got.Decision)
	}
	if len(got.Reasons) == 0 {
		t.Fatalf("autoReviewError should carry a reason")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestNormalizeAutoReview -v`
Expected: FAIL — `undefined: normalizeAutoReview` / `AutoReviewDecision`.

- [ ] **Step 3: Write the implementation** (`internal/agent/autoreview.go`)

```go
package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)

// AutoReviewDecision is the raw JSON the model returns for the whole clip.
type AutoReviewDecision struct {
	Decision   string   `json:"decision"`    // approve | retry | hold
	Confidence float64  `json:"confidence"`  // 0-1
	Reasons    []string `json:"reasons"`     // Thai, human-readable
	DefectType string   `json:"defect_type"` // none | stochastic | deterministic
}

// AutoReviewResult is the normalized, fail-closed outcome the orchestrator acts on.
type AutoReviewResult struct {
	Decision   string // approve | retry | hold  (never anything else)
	Confidence float64
	Reasons    []string
	DefectType string
}

// AutoReviewInput is one clip's judging request: topic context, one frame per
// scene, and the Visual QA issues that flagged it.
type AutoReviewInput struct {
	Question string
	Frames   []QAFrame
	QAIssues []string
}

// AutoReviewAgent is the second-opinion judge. Vision-capable Claude model.
type AutoReviewAgent struct {
	llm *KieLLMClient
}

func NewAutoReviewAgent(llm *KieLLMClient) *AutoReviewAgent {
	return &AutoReviewAgent{llm: llm}
}

// normalizeAutoReview enforces the fail-closed policy: approve only when the
// model said "approve" AND confidence >= threshold; every other case (unknown
// decision, low-confidence approve) becomes "hold". "retry" and "hold" pass
// through. Pure function — unit tested.
func normalizeAutoReview(raw AutoReviewDecision, threshold float64) AutoReviewResult {
	res := AutoReviewResult{Confidence: raw.Confidence, Reasons: raw.Reasons, DefectType: raw.DefectType}
	switch raw.Decision {
	case "approve":
		if raw.Confidence >= threshold {
			res.Decision = "approve"
		} else {
			res.Decision = "hold"
		}
	case "retry":
		res.Decision = "retry"
	default: // "hold", "", or anything unexpected
		res.Decision = "hold"
	}
	return res
}

// autoReviewError builds a fail-closed hold result carrying an error note.
func autoReviewError(note string) AutoReviewResult {
	return AutoReviewResult{Decision: "hold", Reasons: []string{note}}
}

// Judge makes ONE holistic multi-image vision call for the whole clip and
// returns the normalized, fail-closed result. Any error → hold.
func (a *AutoReviewAgent) Judge(ctx context.Context, in AutoReviewInput, cfg *models.AgentConfig, threshold float64) AutoReviewResult {
	if len(in.Frames) == 0 {
		return autoReviewError("no frames to review (fail-closed hold)")
	}
	pngs := make([][]byte, 0, len(in.Frames))
	var b strings.Builder
	fmt.Fprintf(&b, "หัวข้อคลิป: %s\n\nสิ่งที่ Visual QA จับได้ (ตำหนิที่ต้องพิจารณา):\n", in.Question)
	for _, iss := range in.QAIssues {
		fmt.Fprintf(&b, "- %s\n", iss)
	}
	b.WriteString("\nเฟรมจริงแต่ละซีน (เรียงตามลำดับภาพที่แนบ):\n")
	for _, f := range in.Frames {
		if len(f.PNG) == 0 {
			continue
		}
		pngs = append(pngs, f.PNG)
		fmt.Fprintf(&b, "- ซีน %d: ข้อความที่ควรขึ้นจอ = %q\n", f.SceneNumber, f.OnScreenText)
	}
	if len(pngs) == 0 {
		return autoReviewError("all frames empty (fail-closed hold)")
	}
	b.WriteString("\nตอบเป็น JSON เท่านั้น: {\"decision\":\"approve|retry|hold\",\"confidence\":0-1,\"reasons\":[\"...\"],\"defect_type\":\"none|stochastic|deterministic\"}")

	var raw AutoReviewDecision
	if err := a.llm.GenerateVisionJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), b.String(), cfg.Temperature, pngs, &raw); err != nil {
		log.Printf("autoreview: vision error (fail-closed hold): %v", err)
		return autoReviewError(fmt.Sprintf("vision error: %v", err))
	}
	return normalizeAutoReview(raw, threshold)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ -run TestNormalizeAutoReview -v && go test ./internal/agent/ -run TestNormalizeAutoReviewError -v`
Expected: PASS.

- [ ] **Step 5: Build + Commit**

```bash
go build ./... && git add internal/agent/autoreview.go internal/agent/autoreview_test.go
git commit -m "feat(auto-review): AutoReviewAgent with fail-closed decision normalization"
```

---

## Task 3: Clip model + ClipsRepo queue methods

**Files:**
- Modify: `internal/models/clip.go` (Clip struct)
- Modify: `internal/repository/clips.go` (`clipColumns`, `scanClip`, new methods)

**Interfaces:**
- Produces: `Clip.AutoReviewHeld bool`, `Clip.ReviewRetryCount int`; `(*ClipsRepo).ListNeedsReview(ctx, retryCap, limit int) ([]models.Clip, error)`; `(*ClipsRepo).SetAutoReviewHeld(ctx, id string) error`; `(*ClipsRepo).IncrementReviewRetry(ctx, id string) error`.

- [ ] **Step 1: Add fields to `Clip`** (`internal/models/clip.go`, in `type Clip struct`, next to `RetryCount`)

```go
	RetryCount       int    `json:"retry_count"`
	ReviewRetryCount int    `json:"review_retry_count"`
	AutoReviewHeld   bool   `json:"auto_review_held"`
```

- [ ] **Step 2: Extend `clipColumns` and `scanClip`** (`internal/repository/clips.go`)

Add `review_retry_count, auto_review_held` to the `clipColumns` constant (append at the end, before any trailing whitespace), and scan them in `scanClip` in the SAME order. Example — locate the `clipColumns` const and append:

```go
// clipColumns: append these two to the existing column list (keep order in sync with scanClip)
// ... existing columns ..., retry_count, production_stage, review_retry_count, auto_review_held
```

In `scanClip`, add the two destinations at the matching position:

```go
	// ... existing &c.RetryCount, &c.ProductionStage ...,
	&c.ReviewRetryCount, &c.AutoReviewHeld,
```

- [ ] **Step 3: Add the three methods** (`internal/repository/clips.go`), modeled on `ListFailed`

```go
// ListNeedsReview returns needs_review clips eligible for auto-review: not held
// and under the review-retry cap, oldest first, capped at `limit`.
func (r *ClipsRepo) ListNeedsReview(ctx context.Context, retryCap, limit int) ([]models.Clip, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+clipColumns+` FROM clips
		 WHERE status = 'needs_review' AND auto_review_held = FALSE AND review_retry_count < $1
		 ORDER BY created_at ASC LIMIT $2`, retryCap, limit)
	if err != nil {
		return nil, fmt.Errorf("query needs_review clips: %w", err)
	}
	defer rows.Close()
	var clips []models.Clip
	for rows.Next() {
		c, err := scanClip(rows)
		if err != nil {
			return nil, fmt.Errorf("scan needs_review clip: %w", err)
		}
		clips = append(clips, c)
	}
	return clips, nil
}

// SetAutoReviewHeld marks a clip as held so the auto-review tick stops re-judging it.
func (r *ClipsRepo) SetAutoReviewHeld(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE clips SET auto_review_held = TRUE, updated_at = NOW() WHERE id = $1`, id)
	return err
}

// IncrementReviewRetry bumps the review-retry counter (separate from retry_count).
func (r *ClipsRepo) IncrementReviewRetry(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE clips SET review_retry_count = review_retry_count + 1, updated_at = NOW() WHERE id = $1`, id)
	return err
}
```

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: success (scan order must match `clipColumns`; if it fails to compile or the column count mismatches, fix the order before proceeding).

- [ ] **Step 5: Commit**

```bash
git add internal/models/clip.go internal/repository/clips.go
git commit -m "feat(auto-review): clip review state fields + ClipsRepo queue methods"
```

---

## Task 4: AutoReviewsRepo (append-only log + read)

**Files:**
- Create: `internal/repository/autoreviews.go`

**Interfaces:**
- Produces: `NewAutoReviewsRepo(pool *pgxpool.Pool) *AutoReviewsRepo`; `(*AutoReviewsRepo).Create(ctx, clipID, decision, defectType string, confidence float64, reasons []byte) error`; `(*AutoReviewsRepo).GetByClip(ctx, clipID string) (*models.AutoReview, error)` (returns nil, nil when none).

- [ ] **Step 1: Add the `AutoReview` model** (`internal/models/clip.go` or a new `internal/models/autoreview.go`)

```go
// AutoReview is one append-only auto-review decision row.
type AutoReview struct {
	ID         string          `json:"id"`
	ClipID     string          `json:"clip_id"`
	Decision   string          `json:"decision"`
	Confidence float64         `json:"confidence"`
	DefectType string          `json:"defect_type"`
	Reasons    json.RawMessage `json:"reasons"`
	CreatedAt  time.Time       `json:"created_at"`
}
```

- [ ] **Step 2: Write the repo** (`internal/repository/autoreviews.go`), modeled on `visualqa.go`

```go
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type AutoReviewsRepo struct{ pool *pgxpool.Pool }

func NewAutoReviewsRepo(pool *pgxpool.Pool) *AutoReviewsRepo { return &AutoReviewsRepo{pool: pool} }

// Create appends one decision row. reasons is JSON-encoded []string.
func (r *AutoReviewsRepo) Create(ctx context.Context, clipID, decision, defectType string, confidence float64, reasons []byte) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO auto_reviews (clip_id, decision, confidence, defect_type, reasons)
		 VALUES ($1, $2, $3, $4, $5)`,
		clipID, decision, confidence, defectType, reasons)
	return err
}

// GetByClip returns the most recent decision for a clip, or (nil, nil) if none.
func (r *AutoReviewsRepo) GetByClip(ctx context.Context, clipID string) (*models.AutoReview, error) {
	var a models.AutoReview
	err := r.pool.QueryRow(ctx,
		`SELECT id, clip_id, decision, confidence, defect_type, reasons, created_at
		 FROM auto_reviews WHERE clip_id = $1 ORDER BY created_at DESC LIMIT 1`, clipID,
	).Scan(&a.ID, &a.ClipID, &a.Decision, &a.Confidence, &a.DefectType, &a.Reasons, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get auto_review for clip %s: %w", clipID, err)
	}
	return &a, nil
}
```

> NOTE (implementer): if the import block differs, copy it verbatim from `internal/repository/visualqa.go`.

- [ ] **Step 3: Build** — `go build ./...`. Expected: success.
- [ ] **Step 4: Commit**

```bash
git add internal/models/*.go internal/repository/autoreviews.go
git commit -m "feat(auto-review): AutoReviewsRepo append-only log + read"
```

---

## Task 5: Orchestrator — AutoReviewPending processing loop

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`

**Interfaces:**
- Consumes: `AutoReviewAgent.Judge`, `AutoReviewsRepo.Create`, `ClipsRepo.ListNeedsReview/SetAutoReviewHeld/IncrementReviewRetry`, `ScenesRepo.ListByClip`, `VisualQARepo.GetByClip`, `producer.FFmpeg().ExtractFrameAt`, existing `RetryClip`.
- Produces: `(*Orchestrator).AutoReviewPending(ctx context.Context) error`.

- [ ] **Step 1: Add struct fields + constructor params**

In `type Orchestrator struct` add:

```go
	autoReviewAgent *agent.AutoReviewAgent
	autoReviewsRepo *repository.AutoReviewsRepo
```

In `func New(...)` add two params (place next to `vqa`/`visualqa` for readability) and assign them in the returned struct literal:

```go
// signature: add after vqa *agent.VisualQAAgent,
	ara *agent.AutoReviewAgent,
// signature: add after visualqa *repository.VisualQARepo,
	autoreviews *repository.AutoReviewsRepo,
// struct literal: add
	autoReviewAgent: ara, autoReviewsRepo: autoreviews,
```

- [ ] **Step 2: Add the constants + helpers** (near the other orchestrator consts/helpers)

```go
const (
	autoReviewApproveThreshold = 0.8
	autoReviewRetryCap         = 2
	autoReviewBatch            = 5
)

// sceneMids returns each scene's midpoint timestamp (seconds) from persisted scenes.
func sceneMids(scenes []models.Scene) []float64 {
	mids := make([]float64, len(scenes))
	var acc float64
	for i, s := range scenes {
		mids[i] = acc + s.DurationSeconds/2
		acc += s.DurationSeconds
	}
	return mids
}

// downloadToTemp fetches url into a temp .mp4 and returns its path (caller removes it).
func downloadToTemp(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	f, err := os.CreateTemp("", "autoreview-*.mp4")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// autoReviewFrames downloads the clip's rendered 9:16 video and extracts one
// PNG per scene at its midpoint, paired with scene text. Returns nil on any
// failure (caller treats missing frames as a fail-closed hold).
func (o *Orchestrator) autoReviewFrames(ctx context.Context, videoURL string, scenes []models.Scene) []agent.QAFrame {
	mp4Path, err := downloadToTemp(ctx, videoURL)
	if err != nil {
		log.Printf("autoreview: download video failed: %v", err)
		return nil
	}
	defer os.Remove(mp4Path)

	mids := sceneMids(scenes)
	frames := make([]agent.QAFrame, 0, len(scenes))
	for i, s := range scenes {
		outPath := filepath.Join(filepath.Dir(mp4Path), fmt.Sprintf("ar-scene%d.png", s.SceneNumber))
		if err := o.producer.FFmpeg().ExtractFrameAt(mp4Path, outPath, mids[i]); err != nil {
			log.Printf("autoreview: scene %d frame extract failed (skip): %v", s.SceneNumber, err)
			continue
		}
		png, err := os.ReadFile(outPath)
		os.Remove(outPath)
		if err != nil {
			continue
		}
		frames = append(frames, agent.QAFrame{
			SceneNumber:  s.SceneNumber,
			PNG:          png,
			OnScreenText: s.OnScreenText,
			VoiceText:    s.VoiceText,
		})
	}
	return frames
}
```

> Ensure `net/http`, `io`, `os`, `path/filepath` are imported (some already are).

- [ ] **Step 3: Add `AutoReviewPending` + `autoReviewOne`**

```go
// AutoReviewPending runs the second-opinion judge over the needs_review queue.
// Disabled agent → no-op (behavior identical to manual review). Called by the
// scheduler's auto_review tick.
func (o *Orchestrator) AutoReviewPending(ctx context.Context) error {
	cfg, err := o.agentsRepo.GetByName(ctx, "auto_review")
	if err != nil || cfg == nil || !cfg.Enabled {
		return nil
	}
	clips, err := o.clipsRepo.ListNeedsReview(ctx, autoReviewRetryCap, autoReviewBatch)
	if err != nil {
		return fmt.Errorf("auto-review list: %w", err)
	}
	for i := range clips {
		o.autoReviewOne(ctx, &clips[i], cfg)
	}
	return nil
}

// autoReviewOne judges one clip and applies the decision. Every path is logged;
// a per-clip failure never aborts the batch.
func (o *Orchestrator) autoReviewOne(ctx context.Context, clip *models.Clip, cfg *models.AgentConfig) {
	if clip.Video916URL == nil || *clip.Video916URL == "" {
		log.Printf("autoreview: clip %s has no video URL — holding", clip.ID)
		o.recordAndHold(ctx, clip, agent.AutoReviewResult{Decision: "hold", Reasons: []string{"no video url"}})
		return
	}
	scenes, err := o.scenesRepo.ListByClip(ctx, clip.ID)
	if err != nil || len(scenes) == 0 {
		log.Printf("autoreview: clip %s scenes unavailable — holding: %v", clip.ID, err)
		o.recordAndHold(ctx, clip, agent.AutoReviewResult{Decision: "hold", Reasons: []string{"scenes unavailable"}})
		return
	}

	var qaIssues []string
	if qa, err := o.visualQARepo.GetByClip(ctx, clip.ID); err == nil && qa != nil {
		qaIssues = flattenQAIssues(qa) // see helper below
	}

	frames := o.autoReviewFrames(ctx, *clip.Video916URL, scenes)
	res := o.autoReviewAgent.Judge(ctx, agent.AutoReviewInput{
		Question: clip.Question,
		Frames:   frames,
		QAIssues: qaIssues,
	}, cfg, autoReviewApproveThreshold)

	reasons, _ := json.Marshal(res.Reasons)
	if err := o.autoReviewsRepo.Create(ctx, clip.ID, res.Decision, res.DefectType, res.Confidence, reasons); err != nil {
		log.Printf("autoreview: clip %s log write failed: %v", clip.ID, err)
	}

	switch res.Decision {
	case "approve":
		ready := "ready"
		if _, err := o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &ready}); err != nil {
			log.Printf("autoreview: clip %s approve->ready failed: %v", clip.ID, err)
			return
		}
		log.Printf("autoreview: clip %s APPROVED (conf %.2f) — now ready", clip.ID, res.Confidence)
	case "retry":
		if err := o.clipsRepo.IncrementReviewRetry(ctx, clip.ID); err != nil {
			log.Printf("autoreview: clip %s retry counter bump failed: %v", clip.ID, err)
		}
		if err := o.RetryClip(ctx, clip); err != nil {
			log.Printf("autoreview: clip %s retry render failed: %v", clip.ID, err)
		} else {
			log.Printf("autoreview: clip %s RETRY re-render triggered (review_retry now %d)", clip.ID, clip.ReviewRetryCount+1)
		}
	default: // hold
		o.recordHeld(ctx, clip)
	}
}

func (o *Orchestrator) recordAndHold(ctx context.Context, clip *models.Clip, res agent.AutoReviewResult) {
	reasons, _ := json.Marshal(res.Reasons)
	_ = o.autoReviewsRepo.Create(ctx, clip.ID, "hold", res.DefectType, res.Confidence, reasons)
	o.recordHeld(ctx, clip)
}

func (o *Orchestrator) recordHeld(ctx context.Context, clip *models.Clip) {
	if err := o.clipsRepo.SetAutoReviewHeld(ctx, clip.ID); err != nil {
		log.Printf("autoreview: clip %s set-held failed: %v", clip.ID, err)
	} else {
		log.Printf("autoreview: clip %s HELD for human review", clip.ID)
	}
}

// flattenQAIssues pulls the human-readable issue strings out of the stored
// visual_qa verdicts JSON (array of {scene_number, ok, issues}).
func flattenQAIssues(qa *models.VisualQA) []string {
	var verdicts []agent.SceneVerdict
	if err := json.Unmarshal(qa.Issues, &verdicts); err != nil {
		return nil
	}
	var out []string
	for _, v := range verdicts {
		if !v.OK {
			out = append(out, v.Issues...)
		}
	}
	return out
}
```

> NOTE (implementer): confirm the `VisualQARepo.GetByClip` return type and the `models.VisualQA` field holding the JSON verdicts (`Issues json.RawMessage`). If the getter returns a different shape, adapt `flattenQAIssues`. Read `internal/repository/visualqa.go` + `internal/handler/visualqa.go` first. If `GetByClip` doesn't exist, pass `qaIssues=nil` (the agent still judges from frames) and note it.

- [ ] **Step 4: Build** — `go build ./...`. Expected: FAIL until Task 6 updates the `orchestrator.New(...)` call sites (new params). That is expected; proceed to Task 6, then this compiles. (If you prefer green-between-tasks, do Task 6 Step 1 wiring now.)
- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go
git commit -m "feat(auto-review): orchestrator AutoReviewPending queue processor"
```

---

## Task 6: Wire dependencies (composition root) + scheduler tick

**Files:**
- Modify: the file that calls `orchestrator.New(...)` (search: `grep -rn "orchestrator.New(" cmd/ internal/`) — likely `cmd/server/main.go`.
- Modify: `internal/scheduler/scheduler.go`

**Interfaces:**
- Consumes: `agent.NewAutoReviewAgent`, `repository.NewAutoReviewsRepo`, `Orchestrator.AutoReviewPending`.

- [ ] **Step 1: Construct + inject at the composition root**

Where the `KieLLMClient` and other agents/repos are built and passed to `orchestrator.New(...)`, add:

```go
autoReviewAgent := agent.NewAutoReviewAgent(llm)              // reuse the existing *KieLLMClient var
autoReviewsRepo := repository.NewAutoReviewsRepo(pool)
```

Then add these two args to the `orchestrator.New(...)` call in the SAME positions the Task 5 signature defined (after the visual-QA agent arg, and after the visual-QA repo arg).

- [ ] **Step 2: Add the scheduler case** (`internal/scheduler/scheduler.go`, in `handlerFor`)

```go
	case "auto_review":
		return s.orchestrator.AutoReviewPending
```

- [ ] **Step 3: Build + test**

Run: `go build ./... && go test ./internal/agent/ ./internal/orchestrator/ ./internal/scheduler/`
Expected: build success; tests pass.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(auto-review): wire agent+repo into orchestrator and scheduler auto_review tick"
```

---

## Task 7: Read endpoint — GET /clips/{clipId}/auto-review

**Files:**
- Create: `internal/handler/autoreviews.go`
- Modify: `internal/router/router.go`

**Interfaces:**
- Consumes: `AutoReviewsRepo.GetByClip`. Mirrors `CritiquesHandler`.

- [ ] **Step 1: Write the handler** (`internal/handler/autoreviews.go`)

```go
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type AutoReviewsHandler struct{ repo *repository.AutoReviewsRepo }

func NewAutoReviewsHandler(repo *repository.AutoReviewsRepo) *AutoReviewsHandler {
	return &AutoReviewsHandler{repo: repo}
}

func (h *AutoReviewsHandler) GetByClip(w http.ResponseWriter, r *http.Request) {
	clipID := chi.URLParam(r, "clipId")
	a, err := h.repo.GetByClip(r.Context(), clipID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: a}) // a==nil → data:null
}
```

- [ ] **Step 2: Register the route** (`internal/router/router.go`, next to the critiques route ~line 107)

```go
autoReviews := handler.NewAutoReviewsHandler(repository.NewAutoReviewsRepo(pool))
r.Get("/api/v1/clips/{clipId}/auto-review", autoReviews.GetByClip)
```

- [ ] **Step 3: Build** — `go build ./...`. Expected: success.
- [ ] **Step 4: Commit**

```bash
git add internal/handler/autoreviews.go internal/router/router.go
git commit -m "feat(auto-review): GET /clips/{id}/auto-review read endpoint"
```

---

## Task 8: Full verification pass

**Files:** none (verification only)

- [ ] **Step 1: Build + full test**

Run: `go build ./... && go test ./...`
Expected: all pass.

- [ ] **Step 2: Migration sanity** — confirm `migrations/045_auto_review.sql` is the highest number and idempotent (re-running is a no-op). Confirm no `-- +goose` markers.

- [ ] **Step 3: Manual smoke (staging/prod DB via Neon MCP or psql)** — after deploy, verify the seed rows exist:

```sql
SELECT agent_name, enabled, model FROM agent_configs WHERE agent_name='auto_review';
SELECT name, cron_expression, action, enabled FROM schedules WHERE action='auto_review';
```
Expected: one enabled agent row (`claude-sonnet-5`) and one enabled schedule row (`*/10 * * * *`).

- [ ] **Step 4: Commit (if any fixups)**

```bash
git commit -am "test(auto-review): verification fixups" || true
```

---

## Task 9 (OPTIONAL, v1.1): Surface auto-review in ReviewDialog

**Files:**
- Modify: `frontend/src/api.ts` (add `getClipAutoReview(clipId)` mirroring `getClipCritique`)
- Modify: `frontend/src/components/ReviewDialog.tsx` (fetch + render decision/reasons)

- [ ] **Step 1: Add API fn** (`frontend/src/api.ts`) mirroring the critique fetch:

```ts
export const getClipAutoReview = (clipId: string) =>
  apiGet<AutoReview | null>(`/api/v1/clips/${clipId}/auto-review`)
```

(Define an `AutoReview` type: `{ decision: string; confidence: number; defect_type: string; reasons: string[]; created_at: string }`.)

- [ ] **Step 2: Render a small "Auto-review" section** in `ReviewDialog.tsx` next to the critic section (only when data present): show `decision`, `confidence`, and `reasons`.

- [ ] **Step 3: Build** — `cd frontend && npm run build`. Expected: success.
- [ ] **Step 4: Commit**

```bash
git add frontend/src/api.ts frontend/src/components/ReviewDialog.tsx
git commit -m "feat(auto-review): show auto-review decision in ReviewDialog"
```

---

## Self-Review (author checklist — completed)

- **Spec coverage:** second-opinion judge → Task 2; approve/retry/hold + fail-closed → Task 2 (`normalizeAutoReview`) + Task 5 (apply); retry cap 2 & separate counter → Tasks 1,3,5; scheduler tick */10 → Tasks 1,6; frame re-download from URL → Task 5; audit table + held/retry columns → Tasks 1,4; kill switch (enabled flag no-op) → Task 5; read endpoint → Task 7; optional frontend → Task 9; rollback (disable flag) → covered by Task 5 no-op path.
- **Placeholders:** none — every code step shows real code. Two explicit implementer NOTES (pgxpool import path in Task 4; `VisualQARepo.GetByClip`/`models.VisualQA` shape in Task 5) flag facts to confirm against the actual file rather than fabricating; both have a defined fallback.
- **Type consistency:** `AutoReviewResult.Decision` ∈ {approve,retry,hold} used consistently in Tasks 2 & 5; `Judge(ctx, in, cfg, threshold)` signature matches its call in Task 5; new `ClipsRepo` methods match their calls; `orchestrator.New` param additions match the wiring in Task 6.

## Rollback

`UPDATE agent_configs SET enabled=FALSE WHERE agent_name='auto_review';` (or disable the `auto_review` schedule row). Tick becomes a no-op; columns/table are additive and harmless.
