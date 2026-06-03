# Autonomous Self-Healing Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the video production pipeline fully autonomous — no human intervention needed after initial setup. System validates config before spending credits, stops itself when broken, auto-retries recoverable failures, and cleans up dead clips.

**Architecture:** Four layers of self-healing added to the existing scheduler→orchestrator→producer pipeline: (1) pre-flight config validation before producing, (2) circuit breaker that pauses production after consecutive failures, (3) auto-retry of failed clips from the point of failure, (4) cleanup of unrecoverable clips. All state tracked in the existing `settings` table — no new tables needed.

**Tech Stack:** Go, pgx/v5, existing scheduler/orchestrator/producer packages

---

## File Structure

| File | Responsibility |
|------|---------------|
| Create: `internal/preflight/preflight.go` | Pre-flight config validation (voice, API keys, agents) |
| Modify: `internal/scheduler/scheduler.go` | Add circuit breaker + retry failed + cleanup logic |
| Modify: `internal/orchestrator/orchestrator.go` | Support retrying a specific failed clip by ID |
| Modify: `internal/repository/clips.go` | Add query for failed clips + delete old failed clips |
| Modify: `internal/producer/producer.go` | Export `validVoices` for use by preflight |
| Create: `migrations/008_clip_failure_tracking.sql` | Add `fail_reason` and `retry_count` columns to clips |

---

### Task 1: Migration — Add failure tracking columns to clips

**Files:**
- Create: `migrations/008_clip_failure_tracking.sql`

- [ ] **Step 1: Create the migration file**

```sql
ALTER TABLE clips ADD COLUMN IF NOT EXISTS fail_reason TEXT;
ALTER TABLE clips ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0;
```

- [ ] **Step 2: Verify migration file exists**

Run: `ls -la migrations/008_clip_failure_tracking.sql`
Expected: File exists

- [ ] **Step 3: Commit**

```bash
git add migrations/008_clip_failure_tracking.sql
git commit -m "feat: add fail_reason and retry_count columns to clips"
```

---

### Task 2: Repository — Add failed clips queries

**Files:**
- Modify: `internal/repository/clips.go`

- [ ] **Step 1: Add `ListFailed` method to ClipsRepo**

Add after the existing `Delete` method in `internal/repository/clips.go`:

```go
func (r *ClipsRepo) ListFailed(ctx context.Context, maxRetries int) ([]models.Clip, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+clipColumns+` FROM clips
		 WHERE status = 'failed' AND retry_count < $1
		 ORDER BY created_at ASC LIMIT 5`, maxRetries)
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

- [ ] **Step 2: Add `IncrementRetry` method**

```go
func (r *ClipsRepo) IncrementRetry(ctx context.Context, id, reason string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE clips SET retry_count = retry_count + 1, fail_reason = $2, updated_at = NOW()
		 WHERE id = $1`, id, reason)
	if err != nil {
		return fmt.Errorf("increment retry for clip %s: %w", id, err)
	}
	return nil
}
```

- [ ] **Step 3: Add `DeleteOldFailed` method**

```go
func (r *ClipsRepo) DeleteOldFailed(ctx context.Context, maxRetries int) (int, error) {
	result, err := r.pool.Exec(ctx,
		`DELETE FROM clips WHERE status = 'failed' AND retry_count >= $1
		 AND updated_at < NOW() - INTERVAL '24 hours'`, maxRetries)
	if err != nil {
		return 0, fmt.Errorf("delete old failed clips: %w", err)
	}
	return int(result.RowsAffected()), nil
}
```

- [ ] **Step 4: Add `CountConsecutiveFailed` method for circuit breaker**

```go
func (r *ClipsRepo) CountConsecutiveFailed(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM (
			SELECT status FROM clips ORDER BY created_at DESC LIMIT 5
		) recent WHERE status = 'failed'`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count consecutive failed: %w", err)
	}
	return count, nil
}
```

- [ ] **Step 5: Build check**

Run: `go build ./...`
Expected: Build succeeds with no errors

- [ ] **Step 6: Commit**

```bash
git add internal/repository/clips.go
git commit -m "feat: add failed clip queries for retry and cleanup"
```

---

### Task 3: Pre-flight config validation

**Files:**
- Create: `internal/preflight/preflight.go`
- Modify: `internal/producer/producer.go` (export validVoices)

- [ ] **Step 1: Export validVoices from producer**

In `internal/producer/producer.go`, rename the `validVoices` map to `ValidVoices` (exported):

Change:
```go
var validVoices = map[string]bool{
```
To:
```go
var ValidVoices = map[string]bool{
```

Then update all references in the same file: `validVoices[` → `ValidVoices[` (3 occurrences in `getVoice`).

- [ ] **Step 2: Create preflight package**

Create `internal/preflight/preflight.go`:

```go
package preflight

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/producer"
)

type CheckResult struct {
	OK     bool
	Errors []string
}

func Run(ctx context.Context, pool *pgxpool.Pool) CheckResult {
	var errors []string

	voice := checkVoice(ctx, pool)
	if voice != "" {
		errors = append(errors, voice)
	}

	keys := checkAPIKeys(ctx, pool)
	errors = append(errors, keys...)

	return CheckResult{
		OK:     len(errors) == 0,
		Errors: errors,
	}
}

func checkVoice(ctx context.Context, pool *pgxpool.Pool) string {
	var voice string
	err := pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'elevenlabs_voice'`).Scan(&voice)
	if err != nil || voice == "" {
		return ""
	}
	if !producer.ValidVoices[strings.ToLower(voice)] {
		return fmt.Sprintf("invalid elevenlabs_voice '%s' — update in Settings page", voice)
	}
	return ""
}

func checkAPIKeys(ctx context.Context, pool *pgxpool.Pool) []string {
	var errors []string
	required := []struct {
		key  string
		name string
	}{
		{"kie_api_key", "Kie AI API key"},
		{"zernio_api_key", "Zernio API key"},
	}

	for _, k := range required {
		var val string
		err := pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, k.key).Scan(&val)
		if err != nil || val == "" {
			errors = append(errors, fmt.Sprintf("%s not set — configure in Settings page", k.name))
		}
	}
	return errors
}
```

- [ ] **Step 3: Build check**

Run: `go build ./...`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add internal/preflight/preflight.go internal/producer/producer.go
git commit -m "feat: add pre-flight config validation before production"
```

---

### Task 4: Orchestrator — Support retry of a failed clip

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`

- [ ] **Step 1: Add `RetryClip` method**

Add this method after the existing `failClip` method in `internal/orchestrator/orchestrator.go`:

```go
func (o *Orchestrator) RetryClip(ctx context.Context, clip *models.Clip) error {
	log.Printf("Retrying failed clip %s: %s", clip.ID, clip.Title)

	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status})

	theme, err := o.themesRepo.GetActive(ctx)
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("get theme: %w", err))
	}

	scriptCfg, err := o.agentsRepo.GetByName(ctx, "script")
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("get script config: %w", err))
	}
	imageCfg, err := o.agentsRepo.GetByName(ctx, "image")
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("get image config: %w", err))
	}

	q := agent.GeneratedQuestion{
		Question:       clip.Question,
		QuestionerName: clip.QuestionerName,
		Category:       clip.Category,
	}

	// Delete old scenes before regenerating
	o.pool.Exec(ctx, `DELETE FROM scenes WHERE clip_id = $1`, clip.ID)
	o.pool.Exec(ctx, `DELETE FROM clip_metadata WHERE clip_id = $1`, clip.ID)

	return o.produceClipWithID(ctx, clip.ID, q, theme, scriptCfg, imageCfg)
}
```

- [ ] **Step 2: Extract `produceClipWithID` from existing `produceClip`**

Refactor `produceClip` to take an optional clipID. Replace the existing `produceClip` method with two methods:

```go
func (o *Orchestrator) produceClip(ctx context.Context, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig) error {
	today := time.Now().Format("2006-01-02")
	clip, err := o.clipsRepo.Create(ctx, models.CreateClipRequest{
		Title:          q.Question,
		Question:       q.Question,
		QuestionerName: q.QuestionerName,
		Category:       q.Category,
		PublishDate:    &today,
	})
	if err != nil {
		return fmt.Errorf("create clip: %w", err)
	}

	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status})

	return o.produceClipWithID(ctx, clip.ID, q, theme, scriptCfg, imageCfg)
}

func (o *Orchestrator) produceClipWithID(ctx context.Context, clipID string, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig) error {
	o.tracker.StartStep("script")
	script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, scriptCfg.Model, buildPrompt(scriptCfg), scriptCfg.Temperature)
	if err != nil {
		o.tracker.FailStep("script", err)
		return o.failClip(ctx, clipID, fmt.Errorf("script: %w", err))
	}
	o.tracker.CompleteStep("script")

	for _, scene := range script.Scenes {
		overlays := scene.TextOverlays
		if overlays == nil {
			overlays = []byte("[]")
		}
		o.scenesRepo.Create(ctx, models.CreateSceneRequest{
			ClipID:          clipID,
			SceneNumber:     scene.SceneNumber,
			SceneType:       scene.SceneType,
			TextContent:     scene.TextContent,
			VoiceText:       scene.VoiceText,
			DurationSeconds: scene.DurationSeconds,
			TextOverlays:    overlays,
		})
	}

	o.tracker.StartStep("image_prompts")
	imagePrompts, err := o.imageAgent.GeneratePrompts(ctx, script.Scenes, theme, q.QuestionerName, imageCfg.Model, buildPrompt(imageCfg), imageCfg.Temperature)
	if err != nil {
		o.tracker.FailStep("image_prompts", err)
		return o.failClip(ctx, clipID, fmt.Errorf("image prompts: %w", err))
	}
	o.tracker.CompleteStep("image_prompts")

	var fullVoice string
	for _, s := range script.Scenes {
		fullVoice += s.VoiceText + " "
	}

	result, err := o.producer.Produce(ctx, clipID, script.Scenes, imagePrompts, fullVoice)
	if err != nil {
		return o.failClip(ctx, clipID, fmt.Errorf("produce: %w", err))
	}

	readyStatus := "ready"
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{
		Status:       &readyStatus,
		Video169URL:  &result.Video169URL,
		Video916URL:  &result.Video916URL,
		ThumbnailURL: &result.ThumbnailURL,
		AnswerScript: &fullVoice,
		VoiceScript:  &fullVoice,
	})

	o.pool.Exec(ctx,
		`INSERT INTO clip_metadata (clip_id, youtube_title, youtube_description, youtube_tags)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (clip_id) DO UPDATE SET youtube_title=$2, youtube_description=$3, youtube_tags=$4`,
		clipID, script.YoutubeTitle, script.YoutubeDescription, script.YoutubeTags)

	log.Printf("Clip ready: %s — %s", clipID, q.Question)
	return nil
}
```

- [ ] **Step 3: Update `failClip` to store reason**

```go
func (o *Orchestrator) failClip(ctx context.Context, clipID string, err error) error {
	status := "failed"
	reason := err.Error()
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{Status: &status})
	o.clipsRepo.IncrementRetry(ctx, clipID, reason)
	return err
}
```

Note: This requires adding the `IncrementRetry` call. The `failClip` signature stays the same — it already returns `error`.

- [ ] **Step 4: Build check**

Run: `go build ./...`
Expected: Build succeeds

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go
git commit -m "feat: add RetryClip method for re-producing failed clips"
```

---

### Task 5: Scheduler — Circuit breaker + auto-retry + cleanup

**Files:**
- Modify: `internal/scheduler/scheduler.go`

This is the core task — adding self-healing to the scheduler loop.

- [ ] **Step 1: Add imports and constants**

Add these imports to `internal/scheduler/scheduler.go`:

```go
import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/analyzer"
	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/preflight"
	"github.com/jaochai/video-fb/internal/publisher"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/robfig/cron/v3"
)

const (
	maxClipRetries     = 2
	circuitBreakerLimit = 3
)
```

- [ ] **Step 2: Add clipsRepo field to Scheduler struct**

```go
type Scheduler struct {
	cron          *cron.Cron
	pool          *pgxpool.Pool
	publisher     *publisher.Publisher
	analyzer      *analyzer.Analyzer
	orchestrator  *orchestrator.Orchestrator
	schedulesRepo *repository.SchedulesRepo
	clipsRepo     *repository.ClipsRepo
}
```

Update `New` function signature to accept `*repository.ClipsRepo`:

```go
func New(pool *pgxpool.Pool, pub *publisher.Publisher, anlz *analyzer.Analyzer, orch *orchestrator.Orchestrator, schedRepo *repository.SchedulesRepo, clipsRepo *repository.ClipsRepo) *Scheduler {
```

And include `clipsRepo: clipsRepo` in both return paths of `New`.

- [ ] **Step 3: Replace `produceAndPublish` with self-healing version**

Replace the existing `produceAndPublish` method with:

```go
func (s *Scheduler) produceAndPublish(ctx context.Context) error {
	// Phase 0: Pre-flight validation
	check := preflight.Run(ctx, s.pool)
	if !check.OK {
		for _, e := range check.Errors {
			log.Printf("Scheduler PRE-FLIGHT FAIL: %s", e)
		}
		return fmt.Errorf("pre-flight failed: %v", check.Errors)
	}

	// Phase 1: Circuit breaker — skip if too many recent failures
	failCount, err := s.clipsRepo.CountConsecutiveFailed(ctx)
	if err != nil {
		log.Printf("Scheduler: circuit breaker check error: %v", err)
	} else if failCount >= circuitBreakerLimit {
		log.Printf("Scheduler CIRCUIT BREAKER: %d of last 5 clips failed, skipping production. Fix config and retry manually.", failCount)
		return fmt.Errorf("circuit breaker open: %d consecutive failures", failCount)
	}

	// Phase 2: Retry failed clips first (free — they already used question/script credits)
	failed, err := s.clipsRepo.ListFailed(ctx, maxClipRetries)
	if err != nil {
		log.Printf("Scheduler: list failed clips error: %v", err)
	}
	for _, clip := range failed {
		log.Printf("Scheduler: retrying failed clip %s (attempt %d)", clip.ID, clip.RetryCount+1)
		if err := s.orchestrator.RetryClip(ctx, &clip); err != nil {
			log.Printf("Scheduler: retry clip %s failed again: %v", clip.ID, err)
		} else {
			log.Printf("Scheduler: retry clip %s succeeded!", clip.ID)
		}
	}

	// Phase 3: Produce new clip
	log.Println("Scheduler: producing 1 new clip...")
	if err := s.orchestrator.ProduceWeekly(ctx, 1); err != nil {
		return fmt.Errorf("produce: %w", err)
	}

	// Phase 4: Publish ready clips
	log.Println("Scheduler: publishing ready clips...")
	if err := s.publisher.PublishReady(ctx); err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	// Phase 5: Cleanup unrecoverable clips
	deleted, err := s.clipsRepo.DeleteOldFailed(ctx, maxClipRetries)
	if err != nil {
		log.Printf("Scheduler: cleanup error: %v", err)
	} else if deleted > 0 {
		log.Printf("Scheduler: cleaned up %d unrecoverable clips", deleted)
	}

	return nil
}
```

- [ ] **Step 4: Build check**

Run: `go build ./...`
Expected: Will fail — need to update `models.Clip` struct and `New()` call site.

- [ ] **Step 5: Add `RetryCount` and `FailReason` to Clip model**

In `internal/models/models.go`, add two fields to the `Clip` struct:

```go
type Clip struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Question       string    `json:"question"`
	QuestionerName string    `json:"questioner_name"`
	AnswerScript   string    `json:"answer_script"`
	VoiceScript    string    `json:"voice_script"`
	Category       string    `json:"category"`
	Status         string    `json:"status"`
	Video169URL    *string   `json:"video_16_9_url"`
	Video916URL    *string   `json:"video_9_16_url"`
	ThumbnailURL   *string   `json:"thumbnail_url"`
	PublishDate    *string   `json:"publish_date"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	FailReason     *string   `json:"fail_reason,omitempty"`
	RetryCount     int       `json:"retry_count"`
}
```

- [ ] **Step 6: Update `clipColumns` and `scanClip` in repository**

In `internal/repository/clips.go`, update `clipColumns`:

```go
const clipColumns = `id, title, question, questioner_name, answer_script, voice_script,
	category, status, video_16_9_url, video_9_16_url, thumbnail_url,
	publish_date::text, created_at, updated_at, fail_reason, retry_count`
```

Update `scanClip` to include the new fields:

```go
func scanClip(scanner interface{ Scan(dest ...any) error }) (models.Clip, error) {
	var c models.Clip
	err := scanner.Scan(
		&c.ID, &c.Title, &c.Question, &c.QuestionerName,
		&c.AnswerScript, &c.VoiceScript, &c.Category, &c.Status,
		&c.Video169URL, &c.Video916URL, &c.ThumbnailURL,
		&c.PublishDate, &c.CreatedAt, &c.UpdatedAt,
		&c.FailReason, &c.RetryCount,
	)
	return c, err
}
```

- [ ] **Step 7: Update Scheduler `New()` call site in `cmd/server/main.go`**

In `cmd/server/main.go` at line 119, add `clipsRepo` parameter (`clipsRepo` is already created at line 84):

```go
// Before (line 119):
sched := scheduler.New(pool, pub, anlz, orch, schedRepo)

// After:
sched := scheduler.New(pool, pub, anlz, orch, schedRepo, clipsRepo)
```

- [ ] **Step 8: Build check**

Run: `go build ./...`
Expected: Build succeeds

- [ ] **Step 9: Commit**

```bash
git add internal/scheduler/scheduler.go internal/models/models.go internal/repository/clips.go cmd/server/main.go
git commit -m "feat: add circuit breaker, auto-retry, and cleanup to scheduler"
```

---

### Task 6: Integration — Wire everything together and verify

**Files:**
- Modify: `cmd/server/main.go` (if not already done in Task 5)

- [ ] **Step 1: Verify all imports resolve**

Run: `go build ./...`
Expected: Clean build

- [ ] **Step 2: Run tests**

Run: `make test` (or `go test ./...`)
Expected: All tests pass (or no tests exist yet — that's OK for now)

- [ ] **Step 3: Verify migration applies cleanly**

Run: `go run cmd/server/main.go -migrate` (locally or check that it will run on Railway startup)

- [ ] **Step 4: Final commit and push**

```bash
git add -A
git commit -m "feat: fully autonomous self-healing production pipeline

- Pre-flight validation: checks voice + API keys before spending credits
- Circuit breaker: pauses production after 3 consecutive failures
- Auto-retry: re-produces failed clips (max 2 retries) on next scheduler run
- Auto-cleanup: deletes unrecoverable clips after 24 hours"
```

---

## Behavior Summary

| Scenario | Before | After |
|----------|--------|-------|
| Voice setting is "Mark" (invalid) | Spends credits on Question+Script then fails at Voice every run | Pre-flight catches it → skips production → zero credits wasted |
| Kie AI has temporary 500 error | Clip fails, stays failed forever | Retried next scheduler run (max 2 times) |
| Config is fundamentally broken | Fails every run, burns credits each time | Circuit breaker triggers after 3 fails → stops producing until fixed |
| Failed clips pile up in DB | Stay forever, never cleaned | Deleted after 24h if max retries exceeded |
| Everything works normally | Same | Same — no overhead added |
