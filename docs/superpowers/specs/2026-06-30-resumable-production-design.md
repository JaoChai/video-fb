# Resumable Production + Fast Auto-Retry — Design

**Date:** 2026-06-30
**Status:** Approved (brainstorming) — pending implementation plan

## Goal

When a clip's production fails partway, the system should **resume from where it stopped instead of rebuilding the whole clip**, and it should **retry automatically within ~15 minutes** (with a cooldown so a flaky upstream API can recover) rather than waiting for the next 6-hour produce slot.

Two failure scenarios must both resume:
1. A step fails mid-run (e.g. kie/TTS HTTP 500) without a process restart.
2. A deploy/restart happens while a clip is `producing`.

## The Problem (current behavior, verified)

The live pipeline `produceClipWithID` runs linearly: `script(LLM) → scene(LLM) → critic(LLM) → persist scenes+metadata → image-prompts(LLM) → ProduceHyperframes916 (per-scene images + voice + render + upload) → visual QA → ready`. Any step failure calls `failClip` → status `failed`.

`RetryClip` then calls `produceClipWithID` **from the top** — re-running script/scene/critic from scratch. Because the LLM is non-deterministic, **each retry produces a different clip**, wasting tokens and discarding work already persisted in the DB. Legacy `resumeFromImagePrompts`/`resumeFromProduction` functions exist but are dead code and target the old static `Produce` path, not `ProduceHyperframes916`.

Auto-retry currently only runs inside the produce ticks (06:00/12:00/18:00), so a failed clip waits up to 6 hours.

## Locked Decisions (from brainstorming)

| Topic | Decision |
|---|---|
| Resume scope | Both in-run failures AND deploy/restart |
| Durable state | DB only (script/scenes/critic output/image-prompts/metadata). **No new media storage** — regenerate media from the saved prompts on a cross-restart resume |
| Resume mechanism | Explicit `clips.production_stage` checkpoint column |
| Retry cadence | New `retry_failed` schedule every ~15 min, with a ≥10-min per-clip cooldown |

## Non-Goals (YAGNI)

- Persistent media storage (Railway volume / object store) for byte-exact image/voice reuse. Cross-restart resume regenerates media from saved prompts — clip content (script/scenes) is identical; background images may look slightly different (gpt-image-2 is non-deterministic).
- Fine-grained per-scene checkpoints that survive restart.
- Changes to Visual QA. A `needs_review` clip is a quality block, not a failure, and must NOT be auto-retried.
- Immediate in-process retry loops (cadence is the 15-min tick + 10-min cooldown).

## Design

### 1. Stage checkpointing — `clips.production_stage`

A new nullable column records the last completed checkpoint:

| value | meaning (what is durably in the DB) |
|---|---|
| `NULL` | nothing produced yet, or failed before content existed |
| `content_ready` | script + scenes (incl. critic revisions) + image-prompts + metadata all persisted |
| `rendered` | hyperframes video rendered + uploaded (video URL set) |

`ready` / `needs_review` are existing clip `status` values and remain the terminal states; `production_stage` complements `status`, it does not replace it. Forward production sets the stage at each checkpoint. `failClip` leaves `production_stage` untouched (it reflects the last *completed* checkpoint, so retry knows where to resume).

### 2. Stage-aware `RetryClip`

```
RetryClip(clip):
  if clip.production_stage == "content_ready" (or "rendered"):
      load scenes from DB (ScenesRepo.ListByClip)
      → resumeHyperframesProduction: ProduceHyperframes916(scenes, preset) + visual QA + mark ready
      (skips script/scene/critic/image-prompts entirely — SAME clip)
  else:
      full produceClipWithID from script (current behavior)
```

- `resumeHyperframesProduction` is the hyperframes counterpart of the dead legacy `resumeFromProduction`. It reuses existing helpers `scenesToGenerated`, `parseImagePrompts`, and the stored preset (`PresetByKey(clip.StylePreset)`).
- Inside `ProduceHyperframes916`, the existing `fileExists` checks already skip a `voice.wav` / `bg-scene*.png` that survived (same-process resume). After a restart the ephemeral files are gone, so it regenerates them from the saved prompts — content stays identical.
- The dead `resumeFromImagePrompts`/`resumeFromProduction`/`runProduction` (old `Produce` path) are removed as part of this change to avoid two conflicting resume paths.

### 3. Fast auto-retry tick

- New schedule row: `('Retry Failed', '*/15 * * * *', 'retry_failed', TRUE)`.
- New scheduler `handlerFor` case `retry_failed` → a retry-only handler that calls `RetryAllFailed` (now resume-aware via the new `RetryClip`) and `DeleteOldFailed`. It does **not** produce new clips.
- **Cooldown:** `ListFailed` gains a cooldown so a just-failed clip waits before retry: `status='failed' AND retry_count < $max AND updated_at < NOW() - interval '10 minutes'`. This gives a flaky upstream (kie 500) time to recover and is the user's requested delay.
- The retry handler uses the existing production gate (`StartProduction`/`ErrProductionRunning`), so a 15-min tick that lands during a 06/12/18 produce run skips instead of rendering concurrently.
- `maxClipRetries` stays 2 (3 attempts total). With 15-min ticks, the 3 attempts now complete in ~45 min instead of ~18 h; after that `DeleteOldFailed` removes the clip.

## Data Flow

```
produce tick (06/12/18) ──► produceClipWithID
   ├─ script/scene/critic ─► persist scenes+metadata ─► stage=content_ready
   ├─ ProduceHyperframes916 ─► video URL ─► stage=rendered
   └─ visual QA ─► status=ready|needs_review
        (any failure ─► failClip: status=failed, retry_count++, stage unchanged)

retry tick (*/15) ──► RetryAllFailed (cooldown ≥10m, retry_count<2, gate)
   └─ per failed clip ─► RetryClip
        ├─ stage>=content_ready ─► resumeHyperframesProduction (skip LLM)
        └─ stage NULL          ─► produceClipWithID (full)
   └─ DeleteOldFailed (retry_count>=2)
```

## Error Handling

- Resume that fails again → `failClip` increments `retry_count`; next 15-min tick retries until the cap, then delete.
- If `content_ready` is set but DB scenes are missing/corrupt (shouldn't happen) → `resumeHyperframesProduction` falls back to a full `produceClipWithID`.
- Concurrency is governed solely by the existing production gate; no new locking.

## Success Criteria

1. A clip that fails at render (stage `content_ready`) and is retried does **not** re-run script/scene/critic — verified by the same scenes (identical narration/titles) and no new LLM calls for those steps.
2. A failed clip is auto-retried by the `retry_failed` tick within ~15–25 min (≥10-min cooldown honored), not 6 h.
3. The retry tick never renders concurrently with a produce tick (gate respected).
4. A clip that exceeds `maxClipRetries` is deleted by the retry tick.
5. `needs_review` clips are never auto-retried.
6. Flag/behavior parity: produce ticks behave exactly as before aside from now writing `production_stage`.

## Files Touched (anticipated)

| File | Change |
|---|---|
| `migrations/042_resumable_production.sql` | add `clips.production_stage TEXT`; insert `retry_failed` schedule row (latest migration is 041) |
| `internal/models/request.go` | `UpdateClipRequest.ProductionStage *string` |
| `internal/repository/clips.go` | persist/scan `production_stage`; `ListFailed` cooldown clause |
| `internal/orchestrator/orchestrator.go` | set stage at checkpoints; stage-aware `RetryClip` + `resumeHyperframesProduction`; remove dead legacy resume funcs |
| `internal/scheduler/scheduler.go` | `retry_failed` case + retry-only handler |
