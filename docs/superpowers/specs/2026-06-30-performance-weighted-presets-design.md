# Performance-Weighted Style Presets — Design

**Date:** 2026-06-30
**Status:** Approved design, pre-implementation
**Builds on:** `2026-06-30-video-style-presets-design.md` (per-clip presets, already live).
This is sub-project **A** of the "system chooses design itself" roadmap; B (content-aware),
C (layout variants), D (fonts), E (AI-generated themes) remain separate future specs.

## Problem

Per-clip style presets are live, but selection is **uniform random (avoid-last)** — the
system doesn't learn which looks perform. We want it to bias toward presets that hold
viewers, while still exploring, so the channel's design slowly optimizes itself.

## Honest constraints (shape the design)

- ~3 clips/day ÷ 5 presets ≈ 4 clips/preset/week → statistically meaningful per-preset
  signal takes **many weeks**.
- A clip's performance is driven mostly by topic / hook / post timing; **theme is a small,
  confounded factor**. The optimizer must therefore keep strong exploration and never lock
  onto early noise.
- Conclusion: this is a slow, modest optimizer, not a fast win. The design favors safety
  (exploration, cold-start fallback, a kill-switch flag) over aggressive exploitation.

## Goal

When enabled, bias per-clip preset selection toward higher **retention_rate** presets via
**epsilon-greedy** selection, while guaranteeing every preset keeps getting impressions.
Disabled (default) ⇒ today's uniform avoid-last behavior, unchanged.

Locked decisions: metric = `retention_rate`; algorithm = epsilon-greedy; ε = 0.30;
window = 30 days; minClips = 3; gated behind a NEW second flag.

## Architecture

### 1. Aggregate: `AnalyticsRepo.PresetRetention`

```go
// PresetScore is one preset's measured performance over the window.
type PresetScore struct {
    Preset        string  // clips.style_preset
    AvgRetention  float64 // AVG(clip_analytics.retention_rate), 0..1
    N             int     // number of distinct clips with analytics
}

// PresetRetention returns avg retention_rate per style_preset over the last
// windowDays, aggregated across all platforms, for clips that have analytics.
// Presets with no analytics rows simply don't appear in the result.
func (r *AnalyticsRepo) PresetRetention(ctx context.Context, windowDays int) ([]PresetScore, error)
```

SQL shape: join `clip_analytics ca` → `clips c ON ca.clip_id = c.id`, filter
`c.style_preset <> ''` and `ca.fetched_at >= NOW() - windowDays`, `GROUP BY c.style_preset`,
select `AVG(ca.retention_rate)` and `COUNT(DISTINCT ca.clip_id)`. (Aggregating across
platforms is intentional simplicity; per-platform weighting is YAGNI for now.)

### 2. Selection: `producer.PickPresetWeighted`

```go
// PickPresetWeighted chooses a preset using epsilon-greedy over retention scores,
// always excluding lastKey. scores may be partial/empty.
//   - With probability epsilon (explore) OR when no candidate has N >= minClips:
//     uniform random among candidates (today's behavior).
//   - Otherwise (exploit): the candidate with the highest AvgRetention among those
//     with N >= minClips. Candidates with N < minClips are "unknown" and are only
//     reachable on an explore roll — so new/under-sampled presets never starve.
// epsilon and minClips are passed in (config-free callers use the package defaults).
func PickPresetWeighted(lastKey string, scores []PresetScore, epsilon float64, minClips int, rng func(int) int) StylePreset
```

- `rng` is injected (`rand.Intn` in prod) so tests are deterministic. The explore/exploit
  coin flip uses the same injected source.
- Candidate set = all `Presets` except `lastKey` (unchanged avoid-last rule). If that
  leaves one preset, return it.
- Defaults live as package consts: `DefaultEpsilon = 0.30`, `DefaultMinClips = 3`,
  `DefaultWindowDays = 30`.

### 3. Orchestrator wiring (`produceClip`)

Replace the current selection block. With `StylePresetsEnabled()`:
```go
preset := producer.PickPreset(last)                  // uniform avoid-last (unchanged path)
if producer.StylePresetsPerformanceEnabled() {
    scores, err := o.analyticsRepo.PresetRetention(ctx, producer.DefaultWindowDays)
    if err != nil {
        log.Printf("preset perf: scores unavailable, uniform pick: %v", err) // fail-open
    } else {
        preset = producer.PickPresetWeighted(last, scores,
            producer.DefaultEpsilon, producer.DefaultMinClips, rand.Intn)
    }
}
```
- Logs the decision every time: chosen preset, its score+N, and explore-vs-exploit, so the
  optimizer is observable in deploy logs.
- The orchestrator already has the clips repo; it gains an `analyticsRepo` dependency
  (wired in `cmd/server/main.go` — the repo already exists).

### 4. Feature flags (two layers)

| Flag | Off (default) | On |
|---|---|---|
| `STYLE_PRESETS_ENABLED` | signature only (no variety) | per-clip presets |
| `STYLE_PRESETS_PERFORMANCE_ENABLED` | uniform avoid-last | epsilon-greedy by retention |

Performance selection only takes effect when BOTH are on. New flag added as
`producer.StylePresetsPerformanceEnabled()` mirroring the existing `StylePresetsEnabled()`.

## Error handling

- `PresetRetention` error → log + fall back to uniform `PickPreset` (fail-open; production
  never blocks on analytics).
- Empty/partial scores → epsilon-greedy degrades to uniform automatically (no candidate
  meets minClips).
- Retry path (`RetryClip`) is unchanged — it reuses the clip's stored preset, never
  re-selects, so performance logic doesn't touch retries.

## Testing

- Unit `PickPresetWeighted` (deterministic via injected `rng`):
  - exploit roll → returns highest-retention candidate with N≥minClips
  - explore roll → returns the uniform-selected candidate
  - all candidates N<minClips → uniform regardless of roll
  - empty scores → uniform
  - excludes `lastKey` in every branch; single remaining preset returned directly
- Unit/integration `PresetRetention`: verify the SQL on a Neon branch seeded with a few
  `clips` (varied `style_preset`) + `clip_analytics` rows; confirm per-preset AVG and
  DISTINCT-clip counts and the 30-day window cutoff.
- Flag-off: with `STYLE_PRESETS_PERFORMANCE_ENABLED` unset, selection is byte-identical to
  the current uniform path (no analytics query issued).

## Rollout

1. Ship with `STYLE_PRESETS_PERFORMANCE_ENABLED` unset → zero behavior change.
2. Let presets accumulate analytics for a couple of weeks (needs the daily fetch_analytics
   to populate retention per clip).
3. Enable the flag; watch deploy logs for explore/exploit decisions; compare per-preset
   retention in `PresetRetention` over time.
4. Rollback = unset the flag (back to uniform). No code revert.

## Out of scope (future sub-projects)

B content-aware selection, C layout variants, D font variety, E AI-generated themes, and
any admin UI to tune ε/window/minClips (they ship as code constants for now).
