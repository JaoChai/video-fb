# Scheduler Overhaul — Cron-based with Self-Improving Analytics

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the broken hardcoded `time.Ticker` scheduler with a real cron-based scheduler that reads config from the database, runs 2 jobs: Daily Publish (23:30) and Weekly Self-Improve (analytics → Claude analysis → auto-tune agent prompts).

**Architecture:** The scheduler uses `robfig/cron/v3` to run jobs at DB-configured times. Daily Publish calls the existing `publisher.PublishReady()`. Weekly Self-Improve creates a new `Analyzer` that fetches recent analytics from Zernio, sends them to Claude API (via the existing `LLMClient` using the `analytics` agent config from `agent_configs` table), and updates agent system prompts in the database. Old prompts are backed up in a new `agent_prompt_history` table for audit/rollback.

**Tech Stack:** Go 1.25, robfig/cron/v3, pgx/v5, OpenRouter API (via existing LLMClient)

---

### Task 1: Add robfig/cron dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add cron dependency**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go get github.com/robfig/cron/v3
```

- [ ] **Step 2: Verify it's in go.mod**

Run:
```bash
grep robfig /Users/jaochai/Code/video-fb/go.mod
```

Expected: `github.com/robfig/cron/v3 v3.x.x`

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add robfig/cron/v3 for real cron scheduling"
```

---

### Task 2: Migration — simplify schedules + add prompt history table

**Files:**
- Create: `migrations/006_scheduler_overhaul.sql`

- [ ] **Step 1: Write migration**

Create `migrations/006_scheduler_overhaul.sql`:

```sql
-- Prompt history for audit trail when analytics auto-tunes agents
CREATE TABLE IF NOT EXISTS agent_prompt_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_name TEXT NOT NULL,
    old_prompt TEXT NOT NULL,
    new_prompt TEXT NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Replace all schedules with the two we actually use
DELETE FROM schedules;
INSERT INTO schedules (name, cron_expression, action, enabled) VALUES
    ('Daily Publish', '30 23 * * *', 'publish_daily', TRUE),
    ('Weekly Self-Improve', '0 3 * * 1', 'analyze_and_improve', TRUE);
```

- [ ] **Step 2: Verify migration file exists**

Run:
```bash
ls -la /Users/jaochai/Code/video-fb/migrations/006_scheduler_overhaul.sql
```

- [ ] **Step 3: Commit**

```bash
git add migrations/006_scheduler_overhaul.sql
git commit -m "migration: add agent_prompt_history table, simplify schedules to 2 jobs"
```

---

### Task 3: Add `UpdateLastRun` to schedules repository

**Files:**
- Modify: `internal/repository/schedules.go`

- [ ] **Step 1: Add `UpdateLastRun` method**

Add this method to `internal/repository/schedules.go` after the existing `Update` method:

```go
func (r *SchedulesRepo) UpdateLastRun(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE schedules SET last_run_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("update last_run for schedule %s: %w", id, err)
	}
	return nil
}
```

- [ ] **Step 2: Add `ListEnabled` method**

Add this method to `internal/repository/schedules.go` after the `UpdateLastRun` method:

```go
func (r *SchedulesRepo) ListEnabled(ctx context.Context) ([]models.Schedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, cron_expression, action, enabled, last_run_at, next_run_at
		 FROM schedules WHERE enabled = TRUE ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query enabled schedules: %w", err)
	}
	defer rows.Close()

	var schedules []models.Schedule
	for rows.Next() {
		var s models.Schedule
		if err := rows.Scan(&s.ID, &s.Name, &s.CronExpression, &s.Action,
			&s.Enabled, &s.LastRunAt, &s.NextRunAt); err != nil {
			return nil, fmt.Errorf("scan schedule: %w", err)
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}
```

- [ ] **Step 3: Verify build**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./internal/repository/...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/repository/schedules.go
git commit -m "feat: add UpdateLastRun and ListEnabled to schedules repo"
```

---

### Task 4: Add `UpdatePromptByName` and `SavePromptHistory` to agents repository

**Files:**
- Modify: `internal/repository/agents.go`

- [ ] **Step 1: Add `UpdatePromptByName` method**

Add to `internal/repository/agents.go` after the existing `Update` method:

```go
func (r *AgentsRepo) UpdatePromptByName(ctx context.Context, agentName, newPrompt string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_configs SET system_prompt = $2 WHERE agent_name = $1`,
		agentName, newPrompt)
	if err != nil {
		return fmt.Errorf("update prompt for agent %s: %w", agentName, err)
	}
	return nil
}
```

- [ ] **Step 2: Add `SavePromptHistory` method**

Add to `internal/repository/agents.go`:

```go
func (r *AgentsRepo) SavePromptHistory(ctx context.Context, agentName, oldPrompt, newPrompt, reason string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO agent_prompt_history (agent_name, old_prompt, new_prompt, reason)
		 VALUES ($1, $2, $3, $4)`,
		agentName, oldPrompt, newPrompt, reason)
	if err != nil {
		return fmt.Errorf("save prompt history for %s: %w", agentName, err)
	}
	return nil
}
```

- [ ] **Step 3: Verify build**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./internal/repository/...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/repository/agents.go
git commit -m "feat: add UpdatePromptByName and SavePromptHistory to agents repo"
```

---

### Task 5: Create the Analyzer — analytics-driven agent self-improvement

**Files:**
- Create: `internal/analyzer/analyzer.go`

This is the core new component. It:
1. Fetches analytics for clips published in the last 7 days
2. Loads the `analytics` agent config from DB (model, temperature, system_prompt)
3. Sends all data to Claude via the existing `LLMClient`
4. Receives improvement recommendations as JSON
5. Updates agent system prompts in DB with backup to `agent_prompt_history`

- [ ] **Step 1: Create analyzer package**

Create `internal/analyzer/analyzer.go`:

```go
package analyzer

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/repository"
)

type Analyzer struct {
	pool       *pgxpool.Pool
	llm        *agent.LLMClient
	agentsRepo *repository.AgentsRepo
	analytics  *repository.AnalyticsRepo
}

func New(pool *pgxpool.Pool, llm *agent.LLMClient, agentsRepo *repository.AgentsRepo, analytics *repository.AnalyticsRepo) *Analyzer {
	return &Analyzer{pool: pool, llm: llm, agentsRepo: agentsRepo, analytics: analytics}
}

type improvementResult struct {
	Agents []agentImprovement `json:"agents"`
}

type agentImprovement struct {
	AgentName string `json:"agent_name"`
	NewPrompt string `json:"new_prompt"`
	Reason    string `json:"reason"`
}

func (a *Analyzer) AnalyzeAndImprove(ctx context.Context) error {
	data, err := a.gatherData(ctx)
	if err != nil {
		return fmt.Errorf("gather analytics data: %w", err)
	}

	if data == "" {
		log.Println("Analyzer: not enough data to analyze (need at least 3 published clips with analytics)")
		return nil
	}

	analyticsAgent, err := a.agentsRepo.GetByName(ctx, "analytics")
	if err != nil {
		return fmt.Errorf("get analytics agent config: %w", err)
	}

	userPrompt := fmt.Sprintf(`Here is the performance data from our YouTube channel for the last 7 days:

%s

Current agent system prompts:
%s

Based on this data, analyze which videos performed best and worst. Then improve each agent's system_prompt to produce more viral, engaging content.

Return JSON only:
{
  "agents": [
    {"agent_name": "question", "new_prompt": "...", "reason": "..."},
    {"agent_name": "script", "new_prompt": "...", "reason": "..."},
    {"agent_name": "image", "new_prompt": "...", "reason": "..."}
  ]
}`, data, a.currentPrompts(ctx))

	var result improvementResult
	err = a.llm.GenerateJSON(ctx, analyticsAgent.Model, analyticsAgent.SystemPrompt, userPrompt, analyticsAgent.Temperature, &result)
	if err != nil {
		return fmt.Errorf("LLM analysis: %w", err)
	}

	if len(result.Agents) == 0 {
		log.Println("Analyzer: LLM returned no improvements")
		return nil
	}

	for _, imp := range result.Agents {
		if imp.AgentName == "analytics" {
			continue
		}
		if imp.NewPrompt == "" {
			continue
		}

		current, err := a.agentsRepo.GetByName(ctx, imp.AgentName)
		if err != nil {
			log.Printf("Analyzer: skip unknown agent %s: %v", imp.AgentName, err)
			continue
		}

		if err := a.agentsRepo.SavePromptHistory(ctx, imp.AgentName, current.SystemPrompt, imp.NewPrompt, imp.Reason); err != nil {
			log.Printf("Analyzer: failed to save history for %s: %v", imp.AgentName, err)
			continue
		}

		if err := a.agentsRepo.UpdatePromptByName(ctx, imp.AgentName, imp.NewPrompt); err != nil {
			log.Printf("Analyzer: failed to update prompt for %s: %v", imp.AgentName, err)
			continue
		}

		log.Printf("Analyzer: updated %s prompt — reason: %s", imp.AgentName, imp.Reason)
	}

	return nil
}

func (a *Analyzer) gatherData(ctx context.Context) (string, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT c.id, c.title, c.question, c.category,
		       cm.youtube_title, cm.youtube_video_id,
		       ca.views, ca.likes, ca.comments, ca.shares,
		       ca.watch_time_seconds, ca.retention_rate, ca.fetched_at
		FROM clips c
		JOIN clip_metadata cm ON c.id = cm.clip_id
		JOIN clip_analytics ca ON c.id = ca.clip_id
		WHERE c.status = 'published'
		  AND ca.platform = 'youtube'
		  AND ca.fetched_at >= NOW() - INTERVAL '14 days'
		ORDER BY ca.fetched_at DESC`)
	if err != nil {
		return "", fmt.Errorf("query recent analytics: %w", err)
	}
	defer rows.Close()

	var lines []string
	count := 0
	for rows.Next() {
		var id, title, question, category string
		var ytTitle, ytVideoID *string
		var views, likes, comments, shares int
		var watchTime, retention float64
		var fetchedAt interface{}

		if err := rows.Scan(&id, &title, &question, &category,
			&ytTitle, &ytVideoID,
			&views, &likes, &comments, &shares,
			&watchTime, &retention, &fetchedAt); err != nil {
			return "", fmt.Errorf("scan: %w", err)
		}

		yt := ""
		if ytTitle != nil {
			yt = *ytTitle
		}

		lines = append(lines, fmt.Sprintf(
			"- Clip: %s | Title: %s | YT Title: %s | Category: %s | Views: %d | Likes: %d | Comments: %d | Shares: %d | Watch Time: %.0fs | Retention: %.1f%%",
			id[:8], title, yt, category, views, likes, comments, shares, watchTime, retention*100))
		count++
	}

	if count < 3 {
		return "", nil
	}

	return strings.Join(lines, "\n"), nil
}

func (a *Analyzer) currentPrompts(ctx context.Context) string {
	agents, err := a.agentsRepo.List(ctx)
	if err != nil {
		return "(failed to load current prompts)"
	}
	var lines []string
	for _, ag := range agents {
		if ag.AgentName == "analytics" {
			continue
		}
		lines = append(lines, fmt.Sprintf("### %s\n%s", ag.AgentName, ag.SystemPrompt))
	}
	return strings.Join(lines, "\n\n")
}
```

- [ ] **Step 2: Verify build**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./internal/analyzer/...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/analyzer/analyzer.go
git commit -m "feat: add analyzer package for analytics-driven agent self-improvement"
```

---

### Task 6: Rewrite scheduler with robfig/cron

**Files:**
- Modify: `internal/scheduler/scheduler.go` (full rewrite)

- [ ] **Step 1: Rewrite scheduler.go**

Replace the entire content of `internal/scheduler/scheduler.go`:

```go
package scheduler

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/analyzer"
	"github.com/jaochai/video-fb/internal/publisher"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron          *cron.Cron
	pool          *pgxpool.Pool
	publisher     *publisher.Publisher
	analyzer      *analyzer.Analyzer
	schedulesRepo *repository.SchedulesRepo
}

func New(pool *pgxpool.Pool, pub *publisher.Publisher, anlz *analyzer.Analyzer, schedRepo *repository.SchedulesRepo) *Scheduler {
	loc, err := loadBangkokTimezone()
	if err != nil {
		log.Printf("Scheduler: failed to load Asia/Bangkok, using UTC: %v", err)
		return &Scheduler{
			cron:          cron.New(),
			pool:          pool,
			publisher:     pub,
			analyzer:      anlz,
			schedulesRepo: schedRepo,
		}
	}
	return &Scheduler{
		cron:          cron.New(cron.WithLocation(loc)),
		pool:          pool,
		publisher:     pub,
		analyzer:      anlz,
		schedulesRepo: schedRepo,
	}
}

func loadBangkokTimezone() (*time.Location, error) {
	return time.LoadLocation("Asia/Bangkok")
}

func (s *Scheduler) Start(ctx context.Context) error {
	schedules, err := s.schedulesRepo.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("load schedules: %w", err)
	}

	for _, sched := range schedules {
		schedule := sched
		handler := s.handlerFor(schedule.Action)
		if handler == nil {
			log.Printf("Scheduler: unknown action %q for schedule %q, skipping", schedule.Action, schedule.Name)
			continue
		}

		_, err := s.cron.AddFunc(schedule.CronExpression, func() {
			log.Printf("Scheduler [%s]: executing", schedule.Name)
			if err := handler(ctx); err != nil {
				log.Printf("Scheduler [%s]: failed: %v", schedule.Name, err)
			} else {
				log.Printf("Scheduler [%s]: completed", schedule.Name)
			}
			if err := s.schedulesRepo.UpdateLastRun(ctx, schedule.ID); err != nil {
				log.Printf("Scheduler [%s]: failed to update last_run: %v", schedule.Name, err)
			}
		})
		if err != nil {
			log.Printf("Scheduler: invalid cron %q for %q: %v", schedule.CronExpression, schedule.Name, err)
			continue
		}

		log.Printf("Scheduler [%s]: registered cron %q", schedule.Name, schedule.CronExpression)
	}

	s.cron.Start()
	log.Printf("Scheduler started with %d jobs", len(s.cron.Entries()))
	return nil
}

func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("Scheduler stopped")
}

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

- [ ] **Step 2: Add missing imports**

Make sure the import block at top includes:

```go
import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/analyzer"
	"github.com/jaochai/video-fb/internal/publisher"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/robfig/cron/v3"
)
```

- [ ] **Step 3: Verify build**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./internal/scheduler/...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/scheduler/scheduler.go
git commit -m "feat: rewrite scheduler with robfig/cron, reads config from DB"
```

---

### Task 7: Update main.go — new scheduler wiring + graceful shutdown

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add imports for analyzer and signal handling**

Add these to the import block in `cmd/server/main.go`:

```go
"os"
"os/signal"
"syscall"

"github.com/jaochai/video-fb/internal/analyzer"
```

Remove this import (no longer needed):

```go
"github.com/jaochai/video-fb/internal/crawler"
```

- [ ] **Step 2: Remove crawl flag and crawler initialization**

Remove the `crawlFlag` declaration from the flag block:
```go
// DELETE: crawlFlag := flag.Bool("crawl", false, "Run knowledge crawler")
```

Remove the crawler initialization and crawlFlag handling:
```go
// DELETE: crawl := crawler.NewCrawler(pool, ragEngine)
// DELETE: if *crawlFlag { ... }
```

Note: Keep the `crawler` package import ONLY if other code still uses it. In this case, the orchestrator doesn't use crawler, so it can be fully removed from main.go. The crawler package remains in the codebase for potential manual use but is no longer wired into main.go.

- [ ] **Step 3: Create analyzer and update scheduler initialization**

Replace the scheduler creation block (around line 112-113):

```go
// OLD:
// sched := scheduler.New(orch, pub, crawl)
// sched.Start(ctx)

// NEW:
anlz := analyzer.New(pool, llm, agentsRepo, analyticsRepo)
schedRepo := repository.NewSchedulesRepo(pool)
sched := scheduler.New(pool, pub, anlz, schedRepo)
if err := sched.Start(ctx); err != nil {
    log.Printf("Warning: scheduler start failed: %v", err)
}
```

- [ ] **Step 4: Add graceful shutdown**

Replace the server start block at the bottom:

```go
// OLD:
// addr := ":" + cfg.Port
// log.Printf("Starting server on %s", addr)
// if err := http.ListenAndServe(addr, r); err != nil {
//     log.Fatal(err)
// }

// NEW:
addr := ":" + cfg.Port
srv := &http.Server{Addr: addr, Handler: r}

go func() {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    log.Println("Shutting down...")
    sched.Stop()
    srv.Close()
}()

log.Printf("Starting server on %s", addr)
if err := srv.ListenAndServe(); err != http.ErrServerClosed {
    log.Fatal(err)
}
```

- [ ] **Step 5: Verify build**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./cmd/server/...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire analyzer into scheduler, add graceful shutdown"
```

---

### Task 8: Update frontend Schedules page

**Files:**
- Modify: `frontend/src/pages/Schedules.tsx`

The current page is display-only. After this change it will also show `next_run_at` which the cron library can help populate in the future.

- [ ] **Step 1: Add `next_run_at` to the interface and display**

Replace the content of `frontend/src/pages/Schedules.tsx`:

```tsx
import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Schedule {
  id: string;
  name: string;
  cron_expression: string;
  action: string;
  enabled: boolean;
  last_run_at: string | null;
  next_run_at: string | null;
}

const ACTION_LABELS: Record<string, string> = {
  publish_daily: 'Post คลิปขึ้น YouTube',
  analyze_and_improve: 'วิเคราะห์ + ปรับปรุง Agent',
};

const CRON_LABELS: Record<string, string> = {
  '30 23 * * *': 'ทุกวัน 23:30 น.',
  '0 3 * * 1': 'ทุกวันจันทร์ 03:00 น.',
};

export default function SchedulesPage() {
  const { data: schedules, isLoading } = useQuery({
    queryKey: ['schedules'],
    queryFn: () => apiFetch<Schedule[]>('/api/v1/schedules'),
  });

  return (
    <div>
      <h1 style={{ fontSize: 20, fontWeight: 600, marginBottom: 32 }}>Schedules</h1>
      {isLoading ? (
        <p style={{ color: '#555' }}>Loading...</p>
      ) : (
        <div style={{ display: 'grid', gap: 12 }}>
          {schedules?.map((s) => (
            <div
              key={s.id}
              style={{
                background: '#111',
                borderRadius: 8,
                padding: '16px 24px',
                border: '1px solid #1a1a1a',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                transition: 'border-color 0.15s',
              }}
              onMouseEnter={(e) => (e.currentTarget.style.borderColor = '#333')}
              onMouseLeave={(e) => (e.currentTarget.style.borderColor = '#1a1a1a')}
            >
              <div>
                <div style={{ fontSize: 15, fontWeight: 500, marginBottom: 4 }}>{s.name}</div>
                <div style={{ fontSize: 13, color: '#888', marginBottom: 6 }}>
                  {ACTION_LABELS[s.action] || s.action}
                </div>
                <div style={{ display: 'flex', gap: 16, fontSize: 12, color: '#555' }}>
                  <span style={{ fontFamily: 'monospace' }}>
                    {CRON_LABELS[s.cron_expression] || s.cron_expression}
                  </span>
                  {s.last_run_at && (
                    <span>Last: {new Date(s.last_run_at).toLocaleString('th-TH')}</span>
                  )}
                </div>
              </div>
              <span
                style={{
                  fontSize: 11,
                  fontWeight: 500,
                  padding: '3px 10px',
                  borderRadius: 4,
                  background: s.enabled ? 'rgba(34,197,94,0.15)' : 'rgba(239,68,68,0.15)',
                  color: s.enabled ? '#22c55e' : '#ef4444',
                }}
              >
                {s.enabled ? 'Active' : 'Paused'}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify build**

Run:
```bash
cd /Users/jaochai/Code/video-fb/frontend && npm run build
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/Schedules.tsx
git commit -m "feat: update Schedules page with Thai labels and last_run display"
```

---

### Task 9: Run migration and full integration verify

- [ ] **Step 1: Run migration on database**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go run cmd/server/main.go -migrate
```

Expected: `Migrations complete`

- [ ] **Step 2: Full build check**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go build ./... && cd frontend && npm run build
```

Expected: no errors in both

- [ ] **Step 3: Start server and verify scheduler logs**

Run:
```bash
cd /Users/jaochai/Code/video-fb && go run cmd/server/main.go
```

Expected logs:
```
Connected to database
Scheduler [Daily Publish]: registered cron "30 23 * * *"
Scheduler [Weekly Self-Improve]: registered cron "0 3 * * 1"
Scheduler started with 2 jobs
Starting server on :8080
```

- [ ] **Step 4: Verify API returns correct schedules**

Run:
```bash
curl -s -H "Authorization: Bearer $API_KEY" http://localhost:8080/api/v1/schedules | jq
```

Expected: 2 schedules — Daily Publish and Weekly Self-Improve, both enabled

- [ ] **Step 5: Final commit (if any fixups needed)**

```bash
git add -A
git commit -m "chore: integration verify — scheduler overhaul complete"
```
