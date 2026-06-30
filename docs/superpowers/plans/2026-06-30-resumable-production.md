# Resumable Production + Fast Auto-Retry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a failed clip resume from the last completed stage (reusing DB-persisted script/scenes/metadata) instead of rebuilding from scratch, and auto-retry it every ~15 minutes with a ≥10-minute cooldown — without changing the produced clip's content.

**Architecture:** Add a `clips.production_stage` checkpoint column set during forward production. `RetryClip` becomes a dispatcher: if the clip reached `content_ready`/`rendered`, it loads scenes from the DB and resumes at the render stage (skipping all LLM steps); otherwise it does the current full rebuild. The render+QA+finalize tail of `produceClipWithID` is extracted into a shared `renderAndFinalize` used by both paths. A new `retry_failed` schedule (`*/15`) drives fast auto-retry, with a cooldown added to `ListFailed`.

**Tech Stack:** Go, pgx, Postgres (Neon), robfig/cron (existing scheduler), auto-applied SQL migrations.

## Global Constraints

- Resume must NOT re-run the LLM stages (script/scene/critic) when the clip already reached `content_ready` — the clip's content stays identical across retries.
- Durable state is the DB only. No new media storage. Cross-restart resume regenerates media from saved prompts (existing `fileExists` reuse covers same-process).
- `production_stage` values: `''` (none), `content_ready`, `rendered`. `status` values (`producing`/`failed`/`ready`/`needs_review`) are unchanged and orthogonal.
- `needs_review` clips are a quality block, NOT failures — they must never be auto-retried (they already have `status='needs_review'`, so `ListFailed` (status='failed') already excludes them; do not change that).
- The retry path uses the existing production gate (`StartProduction`/`ErrProductionRunning`) — never render concurrently with a produce tick.
- `maxClipRetries` stays 2. Cooldown is `10` minutes. Retry tick cron is `*/15 * * * *`.
- Latest migration is `041`; the new one is `042`.
- Sandboxed shell: `go build`/`go test` write the Go build cache outside the project and fail with "operation not permitted" — re-run those commands with the Bash tool param `dangerouslyDisableSandbox: true`.
- Commit after every task; `go build ./...` + `gofmt` clean before each commit.

---

## Task 1: Migration — `production_stage` column + `retry_failed` schedule

**Files:**
- Create: `migrations/042_resumable_production.sql`

- [ ] **Step 1: Write the migration**

`migrations/042_resumable_production.sql`:

```sql
-- 042_resumable_production.sql
-- Resumable production: track the last completed production checkpoint per clip so
-- a retry can skip the LLM stages and resume at rendering, and add a fast retry
-- schedule (every 15 min) so a failed clip recovers in minutes, not the next 6h
-- produce slot. production_stage values: '' (none), 'content_ready', 'rendered'.
--
-- Rollback:
--   ALTER TABLE clips DROP COLUMN IF EXISTS production_stage;
--   DELETE FROM schedules WHERE action = 'retry_failed';

ALTER TABLE clips ADD COLUMN IF NOT EXISTS production_stage TEXT NOT NULL DEFAULT '';

INSERT INTO schedules (name, cron_expression, action, enabled)
SELECT 'Retry Failed', '*/15 * * * *', 'retry_failed', TRUE
WHERE NOT EXISTS (SELECT 1 FROM schedules WHERE action = 'retry_failed');
```

- [ ] **Step 2: Verify it parses against the live schema (Neon dev branch optional)**

The migration auto-applies on deploy. Locally, sanity-check the SQL is well-formed by eye (idempotent `IF NOT EXISTS` / `WHERE NOT EXISTS`, matches the style of `migrations/032_tiktok_publishing.sql` and `041_triple_daily_tiktok.sql`). No app code references the column yet, so `go build ./...` must still pass.

Run: `go build ./...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add migrations/042_resumable_production.sql
git commit -m "feat(db): production_stage column + retry_failed schedule (every 15m)"
```

---

## Task 2: Model + repo plumbing for `production_stage` and `ListFailed` cooldown

**Files:**
- Modify: `internal/models/clip.go` (add field to `Clip`)
- Modify: `internal/models/request.go` (add field to `UpdateClipRequest`)
- Modify: `internal/repository/clips.go` (`clipColumns`, `scanClip`, `Update`, `ListFailed`)

**Interfaces:**
- Produces (used by Tasks 3-5):
  - `models.Clip.ProductionStage string`
  - `models.UpdateClipRequest.ProductionStage *string`
  - `ClipsRepo.Update(...)` persists `production_stage` via COALESCE
  - `ClipsRepo.ListFailed(ctx, maxRetries int, cooldownMinutes int) ([]models.Clip, error)`

- [ ] **Step 1: Add the model fields**

In `internal/models/clip.go`, add to the `Clip` struct after `ContentFormat`:

```go
	ProductionStage string    `json:"production_stage"`
```

In `internal/models/request.go`, add to `UpdateClipRequest` after `StylePreset`:

```go
	ProductionStage *string `json:"production_stage"`
```

- [ ] **Step 2: Thread the column through `clipColumns` + `scanClip`**

In `internal/repository/clips.go`, append `production_stage` to the `clipColumns` constant (last column):

```go
const clipColumns = `id, title, question, questioner_name, answer_script, voice_script,
	category, status, video_16_9_url, video_9_16_url, thumbnail_url,
	publish_date::text, created_at, updated_at, fail_reason, retry_count, style_preset, content_format,
	production_stage`
```

In `scanClip`, add `&c.ProductionStage` as the final scan target (after `&c.ContentFormat`):

```go
		&c.FailReason, &c.RetryCount, &c.StylePreset, &c.ContentFormat,
		&c.ProductionStage,
```

- [ ] **Step 3: Persist it in `Update`**

In `ClipsRepo.Update`, add a SET clause and the matching arg. Add after the `style_preset = COALESCE($13, style_preset),` line:

```go
			production_stage = COALESCE($14, production_stage),
```

and add `req.ProductionStage` as the final positional arg (after `req.StylePreset,`):

```go
		req.StylePreset,
		req.ProductionStage,
```

- [ ] **Step 4: Add the cooldown to `ListFailed`**

Change the signature and query so a just-failed clip waits `cooldownMinutes` before it is eligible. Replace the whole `ListFailed` function body's query + signature:

```go
func (r *ClipsRepo) ListFailed(ctx context.Context, maxRetries int, cooldownMinutes int) ([]models.Clip, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+clipColumns+` FROM clips
		 WHERE status = 'failed' AND retry_count < $1
		   AND updated_at < NOW() - make_interval(mins => $2)
		 ORDER BY created_at ASC LIMIT 5`, maxRetries, cooldownMinutes)
	if err != nil {
		return nil, fmt.Errorf("query failed clips: %w", err)
	}
	defer rows.Close()

	var clips []models.Clip
	for rows.Next() {
		c, err := scanClip(rows)
		if err != nil {
			return nil, fmt.Errorf("scan failed clip: %w", err)
		}
		clips = append(clips, c)
	}
	return clips, nil
}
```

> `make_interval(mins => $2)` takes the cooldown as a bound parameter (cleaner than string-concatenating an interval). The sole caller is `Orchestrator.RetryAllFailed` (updated in Task 5).

- [ ] **Step 5: Build (the `ListFailed` caller breaks until Task 5 — expected)**

Run: `go build ./internal/repository/ ./internal/models/`
Expected: these two packages compile. `go build ./...` will fail only at `internal/orchestrator` (old `ListFailed` call) until Task 5 — that is expected; do NOT "fix" it here by guessing Task 5's code.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/models/clip.go internal/models/request.go internal/repository/clips.go
git add internal/models/clip.go internal/models/request.go internal/repository/clips.go
git commit -m "feat(repo): production_stage column plumbing + ListFailed cooldown param"
```

---

## Task 3: Extract `renderAndFinalize`; set stages in `produceClipWithID`

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`

**Interfaces:**
- Produces (used by Task 4):
  - `const stageContentReady = "content_ready"`, `const stageRendered = "rendered"`
  - `func (o *Orchestrator) renderAndFinalize(ctx context.Context, clipID string, q agent.GeneratedQuestion, scenes []agent.GeneratedScene, preset producer.StylePreset, narration string) error`

- [ ] **Step 1: Add the stage constants**

Near the top of `internal/orchestrator/orchestrator.go` (after the existing `const` block with `targetSceneCount`), add:

```go
// production_stage checkpoint values (see migration 042). A clip at
// stageContentReady or later has scenes+metadata persisted, so a retry can skip
// the LLM stages and resume at rendering.
const (
	stageContentReady = "content_ready"
	stageRendered     = "rendered"
)
```

- [ ] **Step 2: Extract the render+QA+finalize tail into `renderAndFinalize`**

In `produceClipWithID`, the block from `result, err := o.producer.ProduceHyperframes916(...)` through the final `return nil` (currently lines ~418-457) moves verbatim into a new method, with the final `Update` also setting `ProductionStage`. Add this new method (place it right after `produceClipWithID`):

```go
// renderAndFinalize runs the media/render/upload + visual-QA tail shared by the
// full produce path and the resume path. It assumes scenes + metadata are already
// persisted. On render failure it fails the clip (retriable); on success it marks
// the clip ready/needs_review and records stage=rendered.
func (o *Orchestrator) renderAndFinalize(ctx context.Context, clipID string, q agent.GeneratedQuestion, scenes []agent.GeneratedScene, preset producer.StylePreset, narration string) error {
	result, err := o.producer.ProduceHyperframes916(ctx, clipID, scenes, preset)
	if err != nil {
		return o.failClip(ctx, clipID, fmt.Errorf("produce hyperframes: %w", err))
	}

	status := "ready"
	if qaCfg, qErr := o.agentsRepo.GetByName(ctx, "visual_qa"); qErr == nil && qaCfg.Enabled && result.LocalVideo916Path != "" {
		o.tracker.StartStep("visual_qa")
		frames := o.extractQAFrames(clipID, result.LocalVideo916Path, scenes)
		qaRes := o.visualQAAgent.Review(ctx, agent.VisualQAInput{
			Question: q.Question,
			Frames:   frames,
		}, qaCfg)
		if wErr := o.visualQARepo.Create(ctx, clipID, qaRes.Passed, agent.MarshalVerdicts(qaRes.Verdicts)); wErr != nil {
			log.Printf("visualqa: persist result failed (non-fatal): %v", wErr)
		}
		if !qaRes.Passed {
			status = "needs_review"
			log.Printf("visualqa: clip %s FAILED — status=needs_review (publish blocked); verdicts=%s",
				clipID, string(agent.MarshalVerdicts(qaRes.Verdicts)))
		}
		o.tracker.CompleteStep("visual_qa")
	}

	renderedStage := stageRendered
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{
		Status:          &status,
		Video916URL:     &result.Video916URL,
		ThumbnailURL:    &result.ThumbnailURL,
		VoiceScript:     &narration,
		AnswerScript:    &narration,
		ProductionStage: &renderedStage,
	})
	o.clipsRepo.ClearFailReason(ctx, clipID)
	if status == "ready" {
		log.Printf("Clip ready (hyperframes): %s", clipID)
	}
	return nil
}
```

- [ ] **Step 3: Replace that tail in `produceClipWithID` with a stage write + call**

In `produceClipWithID`, after the metadata upsert block (`o.clipsRepo.UpsertMetadata(...)`, ~line 416), DELETE everything from `// ── Assemble the multi-scene...` / `result, err := o.producer.ProduceHyperframes916(...)` down to the function's closing `return nil`, and replace with:

```go
	// Content (script/scenes/critic/metadata) is now durably persisted — checkpoint
	// so a later failure resumes at rendering instead of regenerating content.
	contentStage := stageContentReady
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{ProductionStage: &contentStage})

	return o.renderAndFinalize(ctx, clipID, q, scenes, preset, narration)
}
```

> `narration` is already in scope in `produceClipWithID` (from `scriptNarration(script)`). `q`, `scenes`, `preset` are parameters/locals already in scope.

- [ ] **Step 4: Build + run existing orchestrator/producer tests**

Run: `go build ./... && go test ./internal/orchestrator/ ./internal/producer/`
Expected: build clean; tests pass (no behavior change yet — the produce path is identical aside from writing `production_stage`).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/orchestrator/orchestrator.go
git add internal/orchestrator/orchestrator.go
git commit -m "refactor(orchestrator): extract renderAndFinalize; checkpoint production_stage"
```

---

## Task 4: Stage-aware `RetryClip` + `resumeHyperframesProduction`

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`
- Test: `internal/orchestrator/resume_test.go` (create)

**Interfaces:**
- Consumes: `stageContentReady`/`stageRendered`, `renderAndFinalize` (Task 3); `scenesRepo.ListByClip`, `scenesToGenerated`, `buildVoiceScript`, `producer.PresetByKey` (existing).
- Produces: `func resumeAtRender(stage string) bool` (pure, unit-tested).

- [ ] **Step 1: Write the failing test for the pure resume decision**

`internal/orchestrator/resume_test.go`:

```go
package orchestrator

import "testing"

func TestResumeAtRender(t *testing.T) {
	cases := map[string]bool{
		"":              false,
		"content_ready": true,
		"rendered":      true,
		"something":     false,
	}
	for stage, want := range cases {
		if got := resumeAtRender(stage); got != want {
			t.Errorf("resumeAtRender(%q) = %v, want %v", stage, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator/ -run TestResumeAtRender -v`
Expected: FAIL — `undefined: resumeAtRender`.

- [ ] **Step 3: Implement `resumeAtRender` + make `RetryClip` a dispatcher**

Add the pure helper near the stage constants:

```go
// resumeAtRender reports whether a failed clip has enough persisted state
// (scenes + metadata) to skip the LLM stages and resume at rendering.
func resumeAtRender(stage string) bool {
	return stage == stageContentReady || stage == stageRendered
}
```

Rename the EXISTING `RetryClip` method body to `retryFull` (it keeps the current full-rebuild logic verbatim — only the method name changes):

```go
// retryFull rebuilds a clip from scratch (script onward). Used when the clip has
// no resumable content checkpoint.
func (o *Orchestrator) retryFull(ctx context.Context, clip *models.Clip) error {
	// ... existing RetryClip body, unchanged ...
}
```

Add the new dispatcher `RetryClip` (the name `RetryAllFailed` already calls `RetryClip`):

```go
func (o *Orchestrator) RetryClip(ctx context.Context, clip *models.Clip) error {
	if resumeAtRender(clip.ProductionStage) {
		return o.resumeHyperframesProduction(ctx, clip)
	}
	return o.retryFull(ctx, clip)
}

// resumeHyperframesProduction reuses the DB-persisted scenes (no LLM re-run) and
// resumes at the render stage. Falls back to a full rebuild if the scenes are
// missing (shouldn't happen for a content_ready clip).
func (o *Orchestrator) resumeHyperframesProduction(ctx context.Context, clip *models.Clip) error {
	scenes, err := o.scenesRepo.ListByClip(ctx, clip.ID)
	if err != nil || len(scenes) == 0 {
		log.Printf("resume %s: scenes unavailable (%v) — full rebuild", clip.ID, err)
		return o.retryFull(ctx, clip)
	}
	log.Printf("Resuming clip %s at render stage (%d scenes, stage=%s)", clip.ID, len(scenes), clip.ProductionStage)

	brandAliases, err := o.settingsRepo.GetBrandAliases(ctx)
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("read brand aliases: %w", err))
	}
	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status})

	gen := scenesToGenerated(scenes)
	narration := buildVoiceScript(scenes, brandAliases)
	preset := producer.PresetByKey(clip.StylePreset)
	q := agent.GeneratedQuestion{
		Question:       clip.Question,
		QuestionerName: clip.QuestionerName,
		Category:       clip.Category,
	}
	return o.renderAndFinalize(ctx, clip.ID, q, gen, preset, narration)
}
```

- [ ] **Step 4: Delete the dead legacy resume functions**

Remove `resumeFromImagePrompts`, `resumeFromProduction`, and `runProduction` (they target the old static `Produce` path, are not called by the new dispatcher, and would be a second, conflicting resume path). Verify with grep they have no remaining callers before deleting:

Run: `grep -rn "resumeFromImagePrompts\|resumeFromProduction\|runProduction" internal/ --include=*.go`
Expected after deletion: only matches inside any test you may have, otherwise none. If `runProduction` is referenced elsewhere, STOP and report — do not delete a referenced function.

> If `parseImagePrompts` / `scenesToGenerated` / `buildVoiceScript` become unused after the deletion, the Go compiler flags unused *functions* only if they are unexported AND unused — package-level funcs don't error on unused. Leave `scenesToGenerated`/`buildVoiceScript` (used by resume). Remove `parseImagePrompts` only if grep shows zero remaining callers.

- [ ] **Step 5: Run tests + build**

Run: `go test ./internal/orchestrator/ -run TestResumeAtRender -v && go build ./... && go test ./internal/orchestrator/`
Expected: PASS; build clean.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/orchestrator/orchestrator.go
git add internal/orchestrator/orchestrator.go internal/orchestrator/resume_test.go
git commit -m "feat(orchestrator): stage-aware RetryClip resumes at render; drop dead legacy resume"
```

---

## Task 5: Scheduler `retry_failed` tick + cooldown wiring

**Files:**
- Modify: `internal/orchestrator/orchestrator.go` (`RetryAllFailed` cooldown arg)
- Modify: `internal/scheduler/scheduler.go` (cooldown const, `retryFailed` handler, `handlerFor` case)
- Test: `internal/scheduler/scheduler_handler_test.go` (create)

**Interfaces:**
- Consumes: `ClipsRepo.ListFailed(ctx, maxRetries, cooldownMinutes)` (Task 2), `RetryClip` (Task 4).
- Produces: scheduler action `"retry_failed"` mapped in `handlerFor`.

- [ ] **Step 1: Update `RetryAllFailed` to pass the cooldown**

In `internal/orchestrator/orchestrator.go`, `RetryAllFailed` currently calls `o.clipsRepo.ListFailed(ctx, maxRetries)`. Add a cooldown parameter and forward it. Change the signature and the call:

```go
func (o *Orchestrator) RetryAllFailed(ctx context.Context, maxRetries int, cooldownMinutes int) error {
	failed, err := o.clipsRepo.ListFailed(ctx, maxRetries, cooldownMinutes)
	// ... rest unchanged ...
```

- [ ] **Step 2: Update the existing produce-tick caller**

In `internal/scheduler/scheduler.go`, the produce path (`produceAndPublish`) calls `s.orchestrator.RetryAllFailed(ctx, maxClipRetries)`. Add the cooldown const and pass it. First add the const next to `maxClipRetries`:

```go
	retryCooldownMinutes = 10
```

Then update the call in `produceAndPublish`:

```go
	if err := s.orchestrator.RetryAllFailed(ctx, maxClipRetries, retryCooldownMinutes); err != nil {
```

- [ ] **Step 3: Add the `retryFailed` handler**

In `internal/scheduler/scheduler.go`, add a retry-only handler (mirrors the retry portion of `produceAndPublish`, but does NOT produce a new clip):

```go
// retryFailed resumes/retries failed clips on a fast cadence (every 15 min). It
// never produces a new clip. The production gate ensures it skips when a produce
// tick is already rendering. Cooldown gives a flaky upstream time to recover.
func (s *Scheduler) retryFailed(ctx context.Context) error {
	if err := s.orchestrator.RetryAllFailed(ctx, maxClipRetries, retryCooldownMinutes); err != nil {
		if errors.Is(err, orchestrator.ErrProductionRunning) {
			log.Println("Scheduler: production running, skipping retry tick")
			return nil
		}
		return fmt.Errorf("retry failed clips: %w", err)
	}
	if deleted, err := s.clipsRepo.DeleteOldFailed(ctx, maxClipRetries); err != nil {
		log.Printf("Scheduler: retry cleanup error: %v", err)
	} else if deleted > 0 {
		log.Printf("Scheduler: cleaned up %d unrecoverable clips", deleted)
	}
	return nil
}
```

> `errors`, `fmt`, `log`, and the `orchestrator` package are already imported in this file (used by `produceAndPublish`).

- [ ] **Step 4: Map the action in `handlerFor`**

In `handlerFor`, add the case (before `default`):

```go
	case "retry_failed":
		return s.retryFailed
```

- [ ] **Step 5: Write + run the handler-mapping test**

`internal/scheduler/scheduler_handler_test.go`:

```go
package scheduler

import "testing"

func TestHandlerForRetryFailed(t *testing.T) {
	s := &Scheduler{}
	if s.handlerFor("retry_failed") == nil {
		t.Error(`handlerFor("retry_failed") returned nil; expected the retryFailed handler`)
	}
	if s.handlerFor("does_not_exist") != nil {
		t.Error("handlerFor(unknown) should be nil")
	}
}
```

> `handlerFor` returns a bound method value without invoking it, so a zero-value `&Scheduler{}` is safe here (no dependencies are dereferenced).

Run: `go test ./internal/scheduler/ -run TestHandlerForRetryFailed -v`
Expected: PASS.

- [ ] **Step 6: Full build + suite**

Run: `go build ./... && go test ./internal/scheduler/ ./internal/orchestrator/ ./internal/repository/`
Expected: build clean (the Task 2 `ListFailed` caller is now fixed); tests pass.

- [ ] **Step 7: Commit**

```bash
gofmt -w internal/orchestrator/orchestrator.go internal/scheduler/scheduler.go
git add internal/orchestrator/orchestrator.go internal/scheduler/scheduler.go internal/scheduler/scheduler_handler_test.go
git commit -m "feat(scheduler): retry_failed tick (every 15m) with 10m cooldown"
```

---

## Self-Review

**Spec coverage:**
- `production_stage` checkpoint column → Task 1 (migration) + Task 2 (plumbing) + Task 3 (set at checkpoints). ✅
- Stage-aware resume skipping LLM → Task 4 (`RetryClip`/`resumeHyperframesProduction`). ✅
- `retry_failed` schedule every 15m → Task 1 (row) + Task 5 (handler/mapping). ✅
- ≥10-min cooldown → Task 2 (`ListFailed` clause) + Task 5 (const wiring). ✅
- Production gate respected → Task 5 (`retryFailed` handles `ErrProductionRunning`). ✅
- `needs_review` never retried → unchanged `ListFailed` status filter (Global Constraints). ✅
- No new media storage; regen from prompts → no task adds storage; resume calls `ProduceHyperframes916` which keeps `fileExists` reuse. ✅
- Remove dead legacy resume → Task 4 Step 4. ✅
- Exhausted clips cleaned up by retry tick → Task 5 Step 3 (`DeleteOldFailed`). ✅

**Placeholder scan:** No TBD/"handle errors"/"similar to" — every code step shows full code. The two `>` notes are explicit guidance, not deferred work. ✅

**Type consistency:** `ListFailed(ctx, maxRetries, cooldownMinutes)` defined Task 2, called Task 5 (both produce-tick and retry handler). `RetryAllFailed(ctx, maxRetries, cooldownMinutes)` defined/updated Task 5, both callers updated. `renderAndFinalize(ctx, clipID, q, scenes []agent.GeneratedScene, preset, narration)` defined Task 3, called by Task 3 (`produceClipWithID`) and Task 4 (`resumeHyperframesProduction`) with matching types. `resumeAtRender(string) bool` defined+used Task 4. `stageContentReady`/`stageRendered` defined Task 3, used Tasks 3-4. `ProductionStage` field types: `models.Clip.ProductionStage string`, `UpdateClipRequest.ProductionStage *string` — consistent across Tasks 2-4. ✅
