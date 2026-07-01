# Auto-Review Agent — Design Spec

**Date:** 2026-07-01
**Status:** Approved (design), pending implementation plan

## Context

ADS VANCE auto-produces short videos (clips) with AI agents and publishes them to YouTube/TikTok. After render, a **Visual QA agent** inspects one rendered frame per scene and — being *fail-open* (passes when uncertain) — either lets the clip through (`ready`) or, when it is **confident it sees a defect**, marks it `needs_review`. `needs_review` clips block publishing and wait for a human to Approve/Reject in the dashboard's ReviewDialog.

**Problem:** When the operator is away from the screen, flagged clips sit in `needs_review` indefinitely and never publish — even though publishing itself is already automated (daily cron 17:00). The operator wants an agent to review flagged clips autonomously so nothing gets stuck.

**Outcome wanted:** A second-opinion agent that clears Visual QA false-positives (approve → publish) and self-heals real-but-fixable defects (retry), only leaving the genuine hard cases for the human.

## Current-state anchors (verified)

- **Statuses** (`migrations/001_initial_schema.sql`): `draft` → `ready` | `needs_review` → `published`; `failed` (retriable).
- **needs_review set at** `orchestrator.go:~446-463` inside `renderAndFinalize()`: only when `visual_qa` agent enabled AND render produced a video AND `!qaRes.Passed`.
- **Visual QA** (`internal/agent/visualqa.go`): vision via kie.ai (`GenerateVisionJSON`, Claude + GPT-5 fallback); one PNG per scene at scene midpoint (`orchestrator.go:~667-690`); fail-open; result stored append-only in `visual_qa` table (`migrations/035`).
- **Manual review** (`frontend/src/components/ReviewDialog.tsx`): Approve = PATCH `/api/v1/clips/{id}` `{status:'ready'}` (`handler/clips.go:~53`); Reject = DELETE clip.
- **Publish gate** (`publisher.go:~40`): only `status='ready' AND publish_date <= CURRENT_DATE`. `needs_review` excluded. Publishing automated via cron (`publish_daily`, 17:00). No re-QA at publish.
- **Retry** — stage-aware `RetryClip` resumes at render (git `2d6f688`); `retry_count` capped at 2 for crash/failed clips; UI shows `(retry_count/2)`.
- **Scheduler** (`scheduler.go` `handlerFor`): existing ticks incl. `retry_failed` (every 15m). Adding an action = add a case + seed a schedule row.
- **No notification infra** exists in code.

## Goals / Non-goals

**Goals**
- Autonomously clear Visual-QA false-positives (approve → publish) with high confidence.
- Self-heal real-but-stochastic defects via bounded re-render retries.
- Leave only genuine/uncertain cases for the human, visible in the dashboard.
- Fully reversible via a single enable flag.

**Non-goals (v1)**
- External notifications (Telegram/LINE/email). Residual `needs_review` clips are dashboard-visible only. (Deferred.)
- Replacing Visual QA or the human ReviewDialog (both stay).
- Re-producing content from scratch (retry = re-render existing scenes, not new topic/script).

## Approach — Scheduler-tick queue processor (Approach A)

`renderAndFinalize()` is unchanged and still sets `needs_review`. A new out-of-band tick picks up the queue. Chosen over inline (adds load to the fragile detached production goroutine; harder rollback) and over merging into Visual QA (conflates detection with policy).

### Flow

```
render → Visual QA → ready ──────────────────────→ publish (cron 17:00)
                   └ needs_review
                        │  ← auto_review tick every 10 min
                        ▼
              auto_review agent (vision, second-opinion judge)
                ├ approve → status=ready ─────────→ publish
                ├ retry (review_retry_count<2) → RetryClip (re-render) → back to QA
                └ hold / retry exhausted / agent error → stays needs_review (dashboard)
```

## Components

### 1. Agent `auto_review` — `internal/agent/autoreview.go`
- Vision via kie.ai (`GenerateVisionJSON`, Claude + GPT-5 fallback) — same path as `visualqa.go`.
- **Input:** rendered frame per scene + the Visual QA `issues` for the clip + per-scene expected `OnScreenText`/`VoiceText` + brand context (colors, mascot).
- **Prompt framing:** "Visual QA flagged: [issues]. You are the senior reviewer. From the actual frames decide whether the clip is genuinely publishable. Separate false-positives (QA over-cautious) from real defects. For real defects, judge whether a re-render would likely fix it (stochastic AI artifact) or not (deterministic, e.g. caption overflow)."
- **Output (StructuredOutput schema):**
  ```json
  { "decision": "approve|retry|hold",
    "confidence": 0.0,
    "reasons": ["ไทย, สั้น"],
    "defect_type": "none|stochastic|deterministic" }
  ```
- **Decision normalization (fail-closed on approve):** approve only if `decision=="approve" && confidence >= APPROVE_THRESHOLD` (config, default 0.8); any error / parse failure / low-confidence approve → coerce to `hold`. The agent may never auto-approve on uncertainty.
- **Config:** seeded row in `agent_configs` (`enabled=true`, `model='claude-sonnet-5'`, `temperature=0.2`). Disable = full off switch.

### 2. Tick `auto_review` — `internal/scheduler/scheduler.go` + handler
- Seed schedule row `*/10 * * * *`, enabled.
- Handler `AutoReview(ctx)`:
  1. Query candidates: `status='needs_review' AND auto_review_held=false AND review_retry_count < 2`, ordered oldest-first, **LIMIT ~5** (per-tick batch cap for cost).
  2. For each candidate:
     - Acquire the frames (see Frame sourcing below) + the clip's latest `visual_qa.issues` + scenes.
     - Run the agent; normalize decision.
     - **approve** → `UPDATE clips SET status='ready'`; insert `auto_reviews` row.
     - **retry** → call existing `RetryClip` (resumes at render); `review_retry_count++`; insert `auto_reviews` row.
     - **hold** → `UPDATE clips SET auto_review_held=true`; insert `auto_reviews` row.
- If agent disabled → handler returns immediately (no-op) → behavior identical to today.

### 3. Persistence — new migration
- Table `auto_reviews` (append-only, mirrors `visual_qa`/`clip_critiques`):
  `id, clip_id, decision, confidence, reasons (jsonb), defect_type, created_at`.
- Columns on `clips`:
  - `auto_review_held BOOLEAN NOT NULL DEFAULT FALSE` — stops endless re-judging of held clips.
  - `review_retry_count INT NOT NULL DEFAULT 0` — **separate** from `retry_count` (crash/failed) so review retries and failure retries do not interfere.
- All statements idempotent (`ADD COLUMN IF NOT EXISTS`, `CREATE TABLE IF NOT EXISTS`); no goose syntax (custom migrator runs whole file, tracks by filename).

### 4. Frame sourcing (design detail — important)
At tick time the local rendered MP4 from production is likely gone (goroutine ended / container restarted). The clip has `video_9_16_url` (kie.ai temp storage, ~1-day TTL; the tick runs within minutes, so fresh). The handler **downloads the MP4 from `video_9_16_url` and re-extracts per-scene frames** with the existing ffmpeg midpoint-extraction logic (refactor `orchestrator.go:~667-690` into a shared helper reused by both QA and auto-review). If download/extract fails → fail-closed → `hold`.

### 5. Guardrails
- **Kill switch:** `agent_configs.auto_review.enabled=false` (or disable the schedule row) → tick no-op → reverts to manual review, no data loss.
- fail-closed approve; `APPROVE_THRESHOLD` config; per-tick batch cap; full `auto_reviews` audit trail.
- `review_retry_count` cap (2) bounds re-render cost per clip.

### 6. Frontend (minimal, optional in v1)
- New read endpoint `GET /api/v1/clips/{id}/auto-review` (latest `auto_reviews` row), mirroring the critique endpoint.
- ReviewDialog shows an "Auto-review" section (decision + reasons) so the human understands why a held clip is still waiting. Low priority; can ship v1.1.

## Testing
- Unit: decision normalization (approve gating by threshold, error → hold, retry cap → hold), tick query selects only eligible clips (not held, under cap).
- Integration-ish: mock agent returning approve/retry/hold → assert status transitions and `auto_reviews` inserts.
- Manual: seed a `needs_review` clip with a known false-positive, run the tick, confirm → `ready`; seed a deterministic-defect clip, confirm → `hold`.
- `go build ./...` and `go test ./...` pass.

## Rollback
Set `agent_configs.auto_review.enabled=false` (or disable the `auto_review` schedule row). Tick becomes a no-op; existing clips stay as-is; manual review unchanged. New columns/table are additive and harmless.

## Decisions (locked)
- Agent authority: **second-opinion judge** — may approve-and-publish false-positives.
- On confirmed/uncertain defect: **auto-retry** (re-render) up to 2, then **hold** for human.
- Notifications: **none in v1** (dashboard-visible only).
- Placement: **scheduler tick**, every 10 min.
- Defaults: retry cap 2, approve confidence threshold 0.8, batch 5/tick (all config-adjustable).
