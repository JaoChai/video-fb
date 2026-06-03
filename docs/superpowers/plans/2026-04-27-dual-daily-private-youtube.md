# Dual Daily Schedule + YouTube Private Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Change the video pipeline to produce 2 clips per day (noon + midnight Bangkok time) and post them to YouTube as "private" via Zernio, so the user can manually set visibility and scheduling on YouTube.

**Architecture:** The scheduler gains a new `produce_and_publish` action that chains orchestrator.ProduceWeekly(ctx, 1) → publisher.PublishReady(ctx). The old `publish_daily` (23:30) schedule is replaced by two new entries: noon (12:00) and midnight (00:00). The Zernio PostRequest changes from `isDraft: true` (Zernio draft, never hits YouTube) to `visibility: "private"` (actually posts to YouTube but as private video). The orchestrator also sets `publish_date = today` so the publish step can find the just-produced clip.

**Tech Stack:** Go 1.25, robfig/cron/v3, pgx/v5, Zernio API

---

### Task 1: Add Visibility field to Zernio PostRequest

**Files:**
- Modify: `internal/publisher/zernio.go:37-44`

- [ ] **Step 1: Add Visibility to PostRequest struct**

In `internal/publisher/zernio.go`, add the `Visibility` field to `PostRequest`:

```go
type PostRequest struct {
	Title      string           `json:"title,omitempty"`
	Content    string           `json:"content"`
	Platforms  []PlatformTarget `json:"platforms"`
	MediaItems []MediaItem      `json:"mediaItems,omitempty"`
	IsDraft    bool             `json:"isDraft,omitempty"`
	PublishNow bool             `json:"publishNow,omitempty"`
	Visibility string           `json:"visibility,omitempty"`
}
```

The `Visibility` field accepts: `"public"`, `"private"`, or `"unlisted"`. When set, Zernio passes this to YouTube's upload API. When omitted (empty string + omitempty), the Zernio default applies.

- [ ] **Step 2: Verify build**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./internal/publisher/...
```

Expected: no errors (this is a struct-only change, no logic affected)

- [ ] **Step 3: Commit**

```bash
git add internal/publisher/zernio.go
git commit -m "feat: add Visibility field to Zernio PostRequest for YouTube privacy control"
```

---

### Task 2: Change Publisher from Zernio draft to YouTube private

**Files:**
- Modify: `internal/publisher/publisher.go:59-88`

This is the core behavior change. Currently `IsDraft: true` means the post stays as a draft inside Zernio and never reaches YouTube. We change to `IsDraft: false` (omitted) + `Visibility: "private"` so the video actually gets uploaded to YouTube but as a private video.

- [ ] **Step 1: Update 16:9 post to use Visibility instead of IsDraft**

In `internal/publisher/publisher.go`, change the 16:9 post request (around line 60-65).

Old code:
```go
		result169, err := p.zernio.Post(ctx, PostRequest{
			Content:    title + "\n\n" + desc,
			Platforms:  platforms,
			MediaItems: []MediaItem{{Type: "video", URL: *video169}},
			IsDraft:    true,
		})
```

New code:
```go
		result169, err := p.zernio.Post(ctx, PostRequest{
			Content:    title,
			Platforms:  platforms,
			MediaItems: []MediaItem{{Type: "video", URL: *video169}},
			Visibility: "private",
		})
```

Note: `Content` is changed to `title` only (Zernio uses `content` as YouTube title, the description is handled separately by Zernio from the video metadata).

- [ ] **Step 2: Update 9:16 Shorts post to use Visibility instead of IsDraft**

In the same file, change the 9:16 post request (around line 78-83).

Old code:
```go
			result916, err := p.zernio.Post(ctx, PostRequest{
				Content:    shortsTitle + " #Shorts\n\n" + desc,
				Platforms:  platforms,
				MediaItems: []MediaItem{{Type: "video", URL: *video916}},
				IsDraft:    true,
			})
```

New code:
```go
			result916, err := p.zernio.Post(ctx, PostRequest{
				Content:    shortsTitle + " #Shorts",
				Platforms:  platforms,
				MediaItems: []MediaItem{{Type: "video", URL: *video916}},
				Visibility: "private",
			})
```

- [ ] **Step 3: Update log messages to reflect private posting**

Change log messages to reflect the new behavior.

Old:
```go
		log.Printf("Posted 16:9 draft for clip %s → %s", clipID, result169.Post.ID)
```

New:
```go
		log.Printf("Posted 16:9 private for clip %s → %s", clipID, result169.Post.ID)
```

Old:
```go
				log.Printf("Posted 9:16 Shorts draft for clip %s → %s", clipID, result916.Post.ID)
```

New:
```go
				log.Printf("Posted 9:16 Shorts private for clip %s → %s", clipID, result916.Post.ID)
```

- [ ] **Step 4: Verify build**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./internal/publisher/...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/publisher/publisher.go
git commit -m "feat: change Zernio posting from draft to YouTube private visibility"
```

---

### Task 3: Set publish_date = today when producing clips

**Files:**
- Modify: `internal/orchestrator/orchestrator.go:119-126`

Currently `produceClip` creates clips without `publish_date` (nil). This means `PublishReady()` can never find them because it filters `publish_date <= CURRENT_DATE` and NULL fails that comparison. We need to set `publish_date = today` so the publish step can pick up the clip.

- [ ] **Step 1: Add today's date to clip creation**

In `internal/orchestrator/orchestrator.go`, modify the `produceClip` method (around line 120-126).

Old code:
```go
	clip, err := o.clipsRepo.Create(ctx, models.CreateClipRequest{
		Title:          q.Question,
		Question:       q.Question,
		QuestionerName: q.QuestionerName,
		Category:       q.Category,
	})
```

New code:
```go
	today := time.Now().Format("2006-01-02")
	clip, err := o.clipsRepo.Create(ctx, models.CreateClipRequest{
		Title:          q.Question,
		Question:       q.Question,
		QuestionerName: q.QuestionerName,
		Category:       q.Category,
		PublishDate:    &today,
	})
```

`time` is already imported in this file (line 7).

- [ ] **Step 2: Verify build**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./internal/orchestrator/...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/orchestrator/orchestrator.go
git commit -m "fix: set publish_date to today when producing clips so PublishReady can find them"
```

---

### Task 4: Add orchestrator to scheduler + produce_and_publish action

**Files:**
- Modify: `internal/scheduler/scheduler.go`

The scheduler needs the orchestrator as a dependency to trigger clip production. A new `produce_and_publish` action chains: produce 1 clip → publish it.

- [ ] **Step 1: Add orchestrator import**

In `internal/scheduler/scheduler.go`, add the orchestrator import:

```go
import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/analyzer"
	"github.com/jaochai/video-fb/internal/orchestrator"
	"github.com/jaochai/video-fb/internal/publisher"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/robfig/cron/v3"
)
```

- [ ] **Step 2: Add orchestrator to Scheduler struct**

Old struct (line 16-22):
```go
type Scheduler struct {
	cron          *cron.Cron
	pool          *pgxpool.Pool
	publisher     *publisher.Publisher
	analyzer      *analyzer.Analyzer
	schedulesRepo *repository.SchedulesRepo
}
```

New struct:
```go
type Scheduler struct {
	cron          *cron.Cron
	pool          *pgxpool.Pool
	publisher     *publisher.Publisher
	analyzer      *analyzer.Analyzer
	orchestrator  *orchestrator.Orchestrator
	schedulesRepo *repository.SchedulesRepo
}
```

- [ ] **Step 3: Update constructor to accept orchestrator**

Old constructor (line 24):
```go
func New(pool *pgxpool.Pool, pub *publisher.Publisher, anlz *analyzer.Analyzer, schedRepo *repository.SchedulesRepo) *Scheduler {
```

New constructor:
```go
func New(pool *pgxpool.Pool, pub *publisher.Publisher, anlz *analyzer.Analyzer, orch *orchestrator.Orchestrator, schedRepo *repository.SchedulesRepo) *Scheduler {
```

Update both return statements in the constructor (lines 28-34 and 36-42) to include `orchestrator: orch`:

First return (fallback UTC):
```go
		return &Scheduler{
			cron:          cron.New(),
			pool:          pool,
			publisher:     pub,
			analyzer:      anlz,
			orchestrator:  orch,
			schedulesRepo: schedRepo,
		}
```

Second return (normal path):
```go
	return &Scheduler{
		cron:          cron.New(cron.WithLocation(loc)),
		pool:          pool,
		publisher:     pub,
		analyzer:      anlz,
		orchestrator:  orch,
		schedulesRepo: schedRepo,
	}
```

- [ ] **Step 4: Add produceAndPublish method**

Add this new method after the `Stop` method (after line 87):

```go
func (s *Scheduler) produceAndPublish(ctx context.Context) error {
	log.Println("Scheduler: producing 1 clip...")
	if err := s.orchestrator.ProduceWeekly(ctx, 1); err != nil {
		return fmt.Errorf("produce: %w", err)
	}
	log.Println("Scheduler: publishing latest ready clip...")
	if err := s.publisher.PublishReady(ctx); err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Add produce_and_publish to handlerFor switch**

Old `handlerFor` (line 89-98):
```go
func (s *Scheduler) handlerFor(action string) func(context.Context) error {
	switch action {
	case "publish_daily":
		return s.publisher.PublishReady
	case "analyze_and_improve":
		return s.analyzer.AnalyzeAndImprove
	default:
		return nil
	}
}
```

New:
```go
func (s *Scheduler) handlerFor(action string) func(context.Context) error {
	switch action {
	case "publish_daily":
		return s.publisher.PublishReady
	case "produce_and_publish":
		return s.produceAndPublish
	case "analyze_and_improve":
		return s.analyzer.AnalyzeAndImprove
	default:
		return nil
	}
}
```

- [ ] **Step 6: Verify build**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./internal/scheduler/...
```

Expected: build error — `cmd/server/main.go` still passes old constructor args. That's fine, we fix it in Task 5.

Run this to verify only the package compiles:
```bash
cd /Users/jaochai/Code/video-fb && go vet ./internal/scheduler/...
```

Expected: no vet errors (vet checks syntax/types within the package)

- [ ] **Step 7: Commit**

```bash
git add internal/scheduler/scheduler.go
git commit -m "feat: add orchestrator to scheduler with produce_and_publish action"
```

---

### Task 5: Wire orchestrator into scheduler in main.go

**Files:**
- Modify: `cmd/server/main.go:119`

- [ ] **Step 1: Update scheduler.New() call to include orchestrator**

In `cmd/server/main.go`, find the scheduler creation (line 119):

Old:
```go
	sched := scheduler.New(pool, pub, anlz, schedRepo)
```

New:
```go
	sched := scheduler.New(pool, pub, anlz, orch, schedRepo)
```

`orch` is already defined on line 90-91 as `orchestrator.New(...)`.

- [ ] **Step 2: Verify full build**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./...
```

Expected: no errors — all packages compile together

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire orchestrator into scheduler for produce_and_publish support"
```

---

### Task 6: Migration — Replace schedules with dual daily + keep weekly self-improve

**Files:**
- Create: `migrations/007_dual_daily_schedule.sql`

This migration replaces the old "Daily Publish" (23:30) schedule with two new "produce_and_publish" schedules at noon and midnight. The "Weekly Self-Improve" schedule is preserved.

- [ ] **Step 1: Create migration file**

Create `migrations/007_dual_daily_schedule.sql`:

```sql
-- Remove old daily publish (23:30) — replaced by noon + midnight produce_and_publish
DELETE FROM schedules WHERE action = 'publish_daily';

-- Add dual daily schedule: produce 1 clip + publish as YouTube private
INSERT INTO schedules (name, cron_expression, action, enabled) VALUES
    ('Noon Produce & Publish', '0 12 * * *', 'produce_and_publish', TRUE),
    ('Midnight Produce & Publish', '0 0 * * *', 'produce_and_publish', TRUE);
```

This keeps `Weekly Self-Improve` (`0 3 * * 1`, `analyze_and_improve`) untouched.

After migration, the `schedules` table will have 3 rows:
| name | cron | action |
|------|------|--------|
| Noon Produce & Publish | `0 12 * * *` | produce_and_publish |
| Midnight Produce & Publish | `0 0 * * *` | produce_and_publish |
| Weekly Self-Improve | `0 3 * * 1` | analyze_and_improve |

- [ ] **Step 2: Verify file exists**

Run:
```bash
ls -la /Users/jaochai/Code/video-fb/migrations/007_dual_daily_schedule.sql
```

- [ ] **Step 3: Commit**

```bash
git add migrations/007_dual_daily_schedule.sql
git commit -m "migration: replace daily publish with noon+midnight produce_and_publish"
```

---

### Task 7: Update frontend Schedules page labels

**Files:**
- Modify: `frontend/src/pages/Schedules.tsx:14-22`

The Schedules page has hardcoded label mappings for actions and cron expressions. We need to add the new `produce_and_publish` action and the new cron times.

- [ ] **Step 1: Add produce_and_publish to ACTION_LABELS**

In `frontend/src/pages/Schedules.tsx`, update `ACTION_LABELS` (line 14-17):

Old:
```tsx
const ACTION_LABELS: Record<string, string> = {
  publish_daily: 'Post คลิปขึ้น YouTube',
  analyze_and_improve: 'วิเคราะห์ + ปรับปรุง Agent',
};
```

New:
```tsx
const ACTION_LABELS: Record<string, string> = {
  publish_daily: 'Post คลิปขึ้น YouTube',
  produce_and_publish: 'ผลิตคลิป + Post YouTube (Private)',
  analyze_and_improve: 'วิเคราะห์ + ปรับปรุง Agent',
};
```

- [ ] **Step 2: Add new cron expressions to CRON_LABELS**

Update `CRON_LABELS` (line 19-22):

Old:
```tsx
const CRON_LABELS: Record<string, string> = {
  '30 23 * * *': 'ทุกวัน 23:30 น.',
  '0 3 * * 1': 'ทุกวันจันทร์ 03:00 น.',
};
```

New:
```tsx
const CRON_LABELS: Record<string, string> = {
  '0 12 * * *': 'ทุกวัน เที่ยงวัน (12:00 น.)',
  '0 0 * * *': 'ทุกวัน เที่ยงคืน (00:00 น.)',
  '30 23 * * *': 'ทุกวัน 23:30 น.',
  '0 3 * * 1': 'ทุกวันจันทร์ 03:00 น.',
};
```

- [ ] **Step 3: Verify frontend build**

Run:
```bash
cd /Users/jaochai/Code/video-fb/frontend && npm run build
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Schedules.tsx
git commit -m "feat: add produce_and_publish labels to Schedules page"
```

---

### Task 8: Run migration + full integration verify

- [ ] **Step 1: Full Go build**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./...
```

Expected: no errors

- [ ] **Step 2: Full frontend build**

Run:
```bash
cd /Users/jaochai/Code/video-fb/frontend && npm run build
```

Expected: no errors

- [ ] **Step 3: Run migration**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go run cmd/server/main.go -migrate
```

Expected: `Migrations complete`

- [ ] **Step 4: Start server and verify scheduler logs**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go run cmd/server/main.go
```

Expected logs should include:
```
Connected to database
Scheduler [Midnight Produce & Publish]: registered cron "0 0 * * *"
Scheduler [Noon Produce & Publish]: registered cron "0 12 * * *"
Scheduler [Weekly Self-Improve]: registered cron "0 3 * * 1"
Scheduler started with 3 jobs
Starting server on :8080
```

Key checks:
- 3 jobs registered (not 2)
- Both noon and midnight schedules appear
- No "unknown action" warnings for `produce_and_publish`

- [ ] **Step 5: Verify schedules API returns correct data**

Run (in another terminal):
```bash
curl -s -H "Authorization: Bearer $API_KEY" http://localhost:8080/api/v1/schedules | python3 -m json.tool
```

Expected: 3 schedules — Noon Produce & Publish, Midnight Produce & Publish, Weekly Self-Improve — all enabled.

- [ ] **Step 6: Stop server (Ctrl+C) and final commit if fixups needed**

```bash
git add -A
git commit -m "chore: verify dual daily schedule + YouTube private — integration complete"
```

---

## Summary of Changes

| File | Change | Why |
|------|--------|-----|
| `internal/publisher/zernio.go` | Add `Visibility` field to `PostRequest` | Enable YouTube privacy control via Zernio API |
| `internal/publisher/publisher.go` | `IsDraft: true` → `Visibility: "private"` | Post to YouTube as private (was Zernio draft) |
| `internal/orchestrator/orchestrator.go` | Set `publish_date = today` in `produceClip` | So `PublishReady()` can find newly produced clips |
| `internal/scheduler/scheduler.go` | Add orchestrator + `produce_and_publish` action | Enable scheduled production + publishing |
| `cmd/server/main.go` | Pass `orch` to `scheduler.New()` | Wire dependencies |
| `migrations/007_dual_daily_schedule.sql` | Replace `publish_daily` with 2x `produce_and_publish` | Noon + midnight schedule |
| `frontend/src/pages/Schedules.tsx` | Add action + cron labels | Dashboard shows correct Thai descriptions |

## New Daily Flow

```
00:00 (เที่ยงคืน) → produce 1 clip → set publish_date=today → publish to YouTube (private)
12:00 (เที่ยงวัน) → produce 1 clip → set publish_date=today → publish to YouTube (private)
03:00 (จันทร์)    → analyze_and_improve (unchanged)
```

User goes to YouTube Studio → sees 2 private videos per day → sets public + schedule manually.

## Assumptions & Risks

1. **Zernio `visibility` field** — This plan assumes Zernio's POST `/api/v1/posts` accepts a `"visibility"` field at the post level with values `"public"`, `"private"`, `"unlisted"`. If Zernio's API uses a different field name or structure (e.g., nested under `platformSettings`), the `PostRequest` struct and publisher code will need adjustment. **Verify with Zernio API docs before deploying.**

2. **Concurrent production** — If the API endpoint triggers production while a scheduled production is running, the progress tracker may conflict. The API handler already checks `tracker.GetStatus().Active`, but the scheduler does not. Low risk since scheduled runs are 12 hours apart and take ~5 minutes.

3. **Server restart required** — After running the migration, the server must be restarted for the scheduler to pick up new cron entries (documented gotcha in CLAUDE.md).
