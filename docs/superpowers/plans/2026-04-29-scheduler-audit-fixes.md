# Scheduler Audit Fixes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 5 issues found in system audit — wire missing scheduler handlers, enable analyzer, clean up redundant/broken data.

**Architecture:** Tasks 1-2 modify the Go scheduler to register handlers for `fetch_analytics` and `crawl_knowledge`. Task 2 requires injecting the crawler dependency into the scheduler. Tasks 3-5 are DB-only fixes via Neon MCP.

**Tech Stack:** Go 1.25, robfig/cron, pgx/v5, Neon PostgreSQL

---

## File Structure

### Modified Files
| File | Changes |
|------|---------|
| `internal/scheduler/scheduler.go` | Add `crawler` field, add 2 cases to `handlerFor()` |
| `cmd/server/main.go` | Pass `crawl` to `scheduler.New()` |

### No New Files

---

## Task 1: Add `fetch_analytics` and `crawl_knowledge` handlers to scheduler

**Files:**
- Modify: `internal/scheduler/scheduler.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add crawler field to Scheduler struct**

In `internal/scheduler/scheduler.go`, add the import and field:

Add to imports:
```go
"github.com/jaochai/video-fb/internal/crawler"
```

Change the struct (line 23-31):
```go
type Scheduler struct {
	cron          *cron.Cron
	pool          *pgxpool.Pool
	publisher     *publisher.Publisher
	analyzer      *analyzer.Analyzer
	orchestrator  *orchestrator.Orchestrator
	crawler       *crawler.Crawler
	schedulesRepo *repository.SchedulesRepo
	clipsRepo     *repository.ClipsRepo
}
```

- [ ] **Step 2: Update New() constructor to accept crawler**

Change the `New()` function signature (line 33):
```go
func New(pool *pgxpool.Pool, pub *publisher.Publisher, anlz *analyzer.Analyzer, orch *orchestrator.Orchestrator, crawl *crawler.Crawler, schedRepo *repository.SchedulesRepo, clipsRepo *repository.ClipsRepo) *Scheduler {
```

Add `crawler: crawl,` to both return blocks:
```go
return &Scheduler{
	cron:          cron.New(),
	pool:          pool,
	publisher:     pub,
	analyzer:      anlz,
	orchestrator:  orch,
	crawler:       crawl,
	schedulesRepo: schedRepo,
	clipsRepo:     clipsRepo,
}
```

And the same for the second return block (with `cron.WithLocation(loc)`).

- [ ] **Step 3: Add 2 cases to handlerFor()**

Change `handlerFor()` (line 152-163):
```go
func (s *Scheduler) handlerFor(action string) func(context.Context) error {
	switch action {
	case "publish_daily":
		return s.publisher.PublishReady
	case "produce_and_publish":
		return s.produceAndPublish
	case "analyze_and_improve":
		return s.analyzer.AnalyzeAndImprove
	case "fetch_analytics":
		return s.publisher.FetchAnalytics
	case "crawl_knowledge":
		return s.crawler.CrawlAll
	default:
		return nil
	}
}
```

- [ ] **Step 4: Update main.go to pass crawler to scheduler**

In `cmd/server/main.go`, find line 123:
```go
sched := scheduler.New(pool, pub, anlz, orch, schedRepo, clipsRepo)
```

Change to:
```go
sched := scheduler.New(pool, pub, anlz, orch, crawl, schedRepo, clipsRepo)
```

Note: `crawl` is already created at line 61 (`crawl := crawler.NewCrawler(pool, ragEngine)`), it just wasn't being passed to the scheduler.

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: Build succeeds with 0 errors.

- [ ] **Step 6: Commit**

```bash
git add internal/scheduler/scheduler.go cmd/server/main.go
git commit -m "fix: wire fetch_analytics and crawl_knowledge handlers to scheduler"
```

---

## Task 2: Enable Weekly Self-Improve in DB

**Files:**
- DB: Neon project `snowy-grass-75448787`

- [ ] **Step 1: Enable Weekly Self-Improve schedule**

Run via Neon MCP:
```sql
UPDATE schedules SET enabled = TRUE WHERE action = 'analyze_and_improve'
```

- [ ] **Step 2: Verify**

Run via Neon MCP:
```sql
SELECT name, action, enabled FROM schedules WHERE action = 'analyze_and_improve'
```

Expected: `enabled = true`

---

## Task 3: Delete redundant Weekly Production schedule

**Files:**
- DB: Neon project `snowy-grass-75448787`

- [ ] **Step 1: Delete produce_weekly schedule**

This schedule is redundant — Noon Produce & Publish and Midnight Produce & Publish already produce clips daily. The `produce_weekly` action also has no handler (now unnecessary since daily production covers it).

Run via Neon MCP:
```sql
DELETE FROM schedules WHERE action = 'produce_weekly'
```

- [ ] **Step 2: Verify remaining schedules**

Run via Neon MCP:
```sql
SELECT name, action, enabled, cron_expression FROM schedules ORDER BY name
```

Expected: 6 schedules (Weekly Production removed), all enabled except none should be disabled.

---

## Task 4: Fix youtube_title format on existing clips

**Files:**
- DB: Neon project `snowy-grass-75448787`

- [ ] **Step 1: Check affected clips**

Run via Neon MCP:
```sql
SELECT clip_id, youtube_title FROM clip_metadata WHERE youtube_title LIKE '%{Ads Vance}%'
```

- [ ] **Step 2: Fix titles — replace {Ads Vance} with | Ads Vance**

Run via Neon MCP:
```sql
UPDATE clip_metadata
SET youtube_title = REPLACE(youtube_title, '{Ads Vance}', '| Ads Vance')
WHERE youtube_title LIKE '%{Ads Vance}%'
```

- [ ] **Step 3: Verify fix**

Run via Neon MCP:
```sql
SELECT clip_id, youtube_title FROM clip_metadata ORDER BY clip_id
```

Expected: All titles end with `| Ads Vance` instead of `{Ads Vance}`

---

## Task 5: Clean up duplicate scenes for ROAS clip

**Files:**
- DB: Neon project `snowy-grass-75448787`

- [ ] **Step 1: Find the ROAS clip with duplicate scenes**

Run via Neon MCP:
```sql
SELECT c.id, c.title, COUNT(s.id) as scene_count
FROM clips c
JOIN scenes s ON c.id = s.clip_id
GROUP BY c.id, c.title
HAVING COUNT(s.id) > 1
ORDER BY scene_count DESC
```

- [ ] **Step 2: Keep only the first scene per clip, delete duplicates**

For any clip with duplicate scenes, keep the one with the lowest scene_number and delete the rest:

```sql
DELETE FROM scenes
WHERE id NOT IN (
  SELECT DISTINCT ON (clip_id) id
  FROM scenes
  ORDER BY clip_id, scene_number, id
)
AND clip_id IN (
  SELECT clip_id FROM scenes GROUP BY clip_id HAVING COUNT(*) > 1
)
```

- [ ] **Step 3: Verify no duplicates remain**

Run via Neon MCP:
```sql
SELECT c.id, c.title, COUNT(s.id) as scene_count
FROM clips c
JOIN scenes s ON c.id = s.clip_id
GROUP BY c.id, c.title
HAVING COUNT(s.id) > 1
```

Expected: 0 rows (no clips with duplicate scenes)

---

## Verification Checklist

After all tasks complete:

- [ ] `go build ./...` passes
- [ ] `handlerFor()` handles 5 actions: `publish_daily`, `produce_and_publish`, `analyze_and_improve`, `fetch_analytics`, `crawl_knowledge`
- [ ] Weekly Self-Improve is enabled in DB
- [ ] Weekly Production (produce_weekly) is removed from DB
- [ ] All youtube_titles use `| Ads Vance` format
- [ ] No clips have duplicate scenes
- [ ] Server deploys and starts without "unknown action" warnings for any enabled schedule
