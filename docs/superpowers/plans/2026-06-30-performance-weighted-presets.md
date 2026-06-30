# Performance-Weighted Style Presets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When enabled, bias per-clip style-preset selection toward higher-retention presets via epsilon-greedy, while always exploring; disabled (default) keeps today's uniform avoid-last selection.

**Architecture:** A new `AnalyticsRepo.PresetRetention` aggregates latest-per-clip retention by `style_preset` over a window. A pure `producer.PickPresetWeighted` runs epsilon-greedy over those scores (deterministic via an injected rng). The orchestrator calls it only when a NEW second flag `STYLE_PRESETS_PERFORMANCE_ENABLED` is on, falling back to the existing uniform `PickPreset` on any analytics error.

**Tech Stack:** Go, pgx/pgxpool, existing analytics tables (`clip_analytics`, `clips.style_preset`).

## Global Constraints

- Metric = `clip_analytics.retention_rate` (stored 0..1). Optimize for higher retention. — spec.
- Algorithm = epsilon-greedy. Defaults as package consts: `DefaultEpsilon = 0.30`, `DefaultMinClips = 3`, `DefaultWindowDays = 30`. — spec.
- Two flags: `STYLE_PRESETS_ENABLED` (existing) AND new `STYLE_PRESETS_PERFORMANCE_ENABLED`. Performance selection requires BOTH on. — spec.
- Disabled (`STYLE_PRESETS_PERFORMANCE_ENABLED` unset) ⇒ behavior byte-identical to today's uniform `PickPreset`; NO analytics query is issued. — spec.
- Fail-open: any `PresetRetention` error ⇒ log + fall back to uniform `PickPreset`. Production never blocks on analytics. — spec.
- Aggregate latest-per-clip (reuse the `latestAnalyticsCTE` pattern), across all platforms; do NOT average every daily snapshot row (that over-weights clips with more fetches). — analytics schema.
- `RetryClip` is unchanged: it reuses a clip's stored preset, never re-selects. — spec.
- The avoid-last rule (never return `lastKey` when other presets exist) is preserved in the weighted path. — prior preset design.

---

## File Structure

- **Modify** `internal/models/analytics.go` (or wherever analytics models live) — add `PresetScore` struct.
- **Modify** `internal/repository/analytics.go` — add `PresetRetention`.
- **Modify** `internal/producer/presets.go` — add `StylePresetsPerformanceEnabled`, the `Default*` consts, and `PickPresetWeighted`.
- **Modify** `internal/producer/presets_test.go` — unit tests for `PickPresetWeighted`.
- **Modify** `internal/orchestrator/orchestrator.go` — add `analyticsRepo` field + `New` param; wire weighted selection in `produceClip`.
- **Modify** `cmd/server/main.go` — pass the existing `analyticsRepo` into `orchestrator.New`.

---

## Task 1: `PresetRetention` analytics aggregate

**Files:**
- Modify: `internal/models/analytics.go` (add `PresetScore`; if no such file, add to the file that defines `ClipPerformance`)
- Modify: `internal/repository/analytics.go` (add method)

**Interfaces:**
- Produces:
  - `type PresetScore struct { Preset string; AvgRetention float64; N int }` (in `models`)
  - `func (r *AnalyticsRepo) PresetRetention(ctx context.Context, windowDays int) ([]models.PresetScore, error)`

- [ ] **Step 1: Find where `ClipPerformance` is defined and add `PresetScore` beside it**

Run: `grep -rn "type ClipPerformance" internal/models/`
Add to that file:

```go
// PresetScore is one style preset's measured retention over a recent window,
// used to bias preset selection toward better-performing looks.
type PresetScore struct {
	Preset       string  `json:"preset"`
	AvgRetention float64 `json:"avg_retention"` // mean of latest-per-clip retention_rate, 0..1
	N            int     `json:"n"`             // number of distinct clips counted
}
```

- [ ] **Step 2: Add `PresetRetention` to `analytics.go`**

Reuse the existing `latestAnalyticsCTE` (latest row per clip/platform/post_type) so daily
re-fetches of the same clip are not double-counted. Average the latest retention per clip,
grouped by the clip's `style_preset`, over the window.

```go
// PresetRetention returns the mean latest retention_rate per style_preset over the
// last windowDays, across all platforms, for published clips that carry a preset
// and have analytics. Presets without qualifying analytics simply do not appear.
func (r *AnalyticsRepo) PresetRetention(ctx context.Context, windowDays int) ([]models.PresetScore, error) {
	rows, err := r.pool.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (clip_id, platform, post_type)
				clip_id, platform, post_type, retention_rate, fetched_at
			FROM clip_analytics
			WHERE fetched_at >= NOW() - make_interval(days => $1)
			ORDER BY clip_id, platform, post_type, fetched_at DESC
		)
		SELECT c.style_preset,
		       COALESCE(AVG(NULLIF(l.retention_rate, 0)), 0) AS avg_ret,
		       COUNT(DISTINCT l.clip_id) AS n
		FROM clips c
		JOIN latest l ON l.clip_id = c.id
		WHERE c.style_preset <> ''
		GROUP BY c.style_preset`, windowDays)
	if err != nil {
		return nil, fmt.Errorf("preset retention: %w", err)
	}
	defer rows.Close()

	var out []models.PresetScore
	for rows.Next() {
		var s models.PresetScore
		if err := rows.Scan(&s.Preset, &s.AvgRetention, &s.N); err != nil {
			return nil, fmt.Errorf("scan preset score: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate preset scores: %w", err)
	}
	return out, nil
}
```

- [ ] **Step 3: Build + vet**

Run: `go build ./... && go vet ./internal/repository/ ./internal/models/`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add internal/models/ internal/repository/analytics.go
git commit -m "feat(analytics): PresetRetention aggregate (latest-per-clip retention by style_preset)"
```

> **Controller note (not an implementer step):** the SQL is verified against a seeded Neon branch by the controller during review, not by a DB-less unit test.

---

## Task 2: `PickPresetWeighted` + performance flag

**Files:**
- Modify: `internal/producer/presets.go`
- Modify: `internal/producer/presets_test.go`

**Interfaces:**
- Consumes: `models.PresetScore` (Task 1); existing `Presets`, `StylePreset`.
- Produces:
  - `const DefaultEpsilon = 0.30`, `const DefaultMinClips = 3`, `const DefaultWindowDays = 30`
  - `func StylePresetsPerformanceEnabled() bool`
  - `func PickPresetWeighted(lastKey string, scores []models.PresetScore, epsilon float64, minClips int, rng func(int) int) StylePreset`

- [ ] **Step 1: Write the failing tests**

```go
// add to internal/producer/presets_test.go (also add "github.com/jaochai/video-fb/internal/models" to the test imports if not present)
func TestPickWeighted_ExploitPicksHighestEligible(t *testing.T) {
	// rng(100) -> 50 (>=30, no explore); no second rng call expected on exploit.
	rng := scriptedRng(t, 50)
	scores := []models.PresetScore{
		{Preset: "teal-coral", AvgRetention: 0.40, N: 5},
		{Preset: "purple-gold", AvgRetention: 0.60, N: 5}, // highest eligible
		{Preset: "charcoal-electric", AvgRetention: 0.90, N: 1}, // N<min, ignored for exploit
	}
	got := PickPresetWeighted("signature", scores, 0.30, 3, rng)
	if got.Key != "purple-gold" {
		t.Fatalf("exploit picked %q, want purple-gold", got.Key)
	}
}

func TestPickWeighted_ExploreRollUsesUniform(t *testing.T) {
	// rng(100) -> 10 (<30 explore); rng(len(candidates)) -> 0 -> first candidate.
	rng := scriptedRng(t, 10, 0)
	scores := []models.PresetScore{{Preset: "purple-gold", AvgRetention: 0.9, N: 9}}
	got := PickPresetWeighted("signature", scores, 0.30, 3, rng)
	candidates := candidateKeysExcluding("signature")
	if got.Key != candidates[0] {
		t.Fatalf("explore picked %q, want first candidate %q", got.Key, candidates[0])
	}
}

func TestPickWeighted_NoEligibleFallsBackToUniform(t *testing.T) {
	// All N<minClips → no exploit target → uniform even on a no-explore roll.
	// rng(100) -> 99 (no explore), then rng(len) -> 0.
	rng := scriptedRng(t, 99, 0)
	scores := []models.PresetScore{{Preset: "purple-gold", AvgRetention: 0.9, N: 1}}
	got := PickPresetWeighted("signature", scores, 0.30, 3, rng)
	if got.Key == "signature" {
		t.Fatal("must not return lastKey")
	}
	if PresetByKey(got.Key).Key != got.Key {
		t.Fatalf("returned unknown preset %q", got.Key)
	}
}

func TestPickWeighted_EmptyScoresUniform(t *testing.T) {
	rng := scriptedRng(t, 99, 1)
	got := PickPresetWeighted("signature", nil, 0.30, 3, rng)
	if got.Key == "signature" {
		t.Fatal("must not return lastKey")
	}
}

func TestPickWeighted_NeverReturnsLastKey(t *testing.T) {
	scores := []models.PresetScore{{Preset: "signature", AvgRetention: 0.99, N: 99}}
	// signature scores highest but is lastKey → excluded; no other candidate has scores,
	// so bestIdx == -1 → uniform path: rng(100) coin then rng(len) for the pick.
	got := PickPresetWeighted("signature", scores, 0.0, 1, scriptedRng(t, 50, 0))
	if got.Key == "signature" {
		t.Fatal("returned the avoided lastKey")
	}
}
```

Add these test helpers to the same file:

```go
// scriptedRng returns a deterministic rng(n) that yields the given values in order,
// clamped into [0,n). Fails the test if called more times than scripted.
func scriptedRng(t *testing.T, vals ...int) func(int) int {
	t.Helper()
	i := 0
	return func(n int) int {
		if i >= len(vals) {
			t.Fatalf("rng called more than the %d scripted times", len(vals))
		}
		v := vals[i]
		i++
		if n <= 0 {
			return 0
		}
		return v % n
	}
}

// candidateKeysExcluding mirrors PickPresetWeighted's candidate ordering: all
// Presets in declared order except lastKey.
func candidateKeysExcluding(lastKey string) []string {
	var ks []string
	for _, p := range Presets {
		if p.Key != lastKey {
			ks = append(ks, p.Key)
		}
	}
	return ks
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/producer/ -run TestPickWeighted -v`
Expected: FAIL (undefined: PickPresetWeighted).

- [ ] **Step 3: Implement in `presets.go`**

```go
// Performance-weighted selection tuning (spec defaults; callers pass them in).
const (
	DefaultEpsilon    = 0.30
	DefaultMinClips   = 3
	DefaultWindowDays = 30
)

// StylePresetsPerformanceEnabled reports whether per-clip selection should be biased
// by measured retention (epsilon-greedy). Requires STYLE_PRESETS_ENABLED as well;
// off ⇒ uniform avoid-last selection.
func StylePresetsPerformanceEnabled() bool {
	return os.Getenv("STYLE_PRESETS_PERFORMANCE_ENABLED") == "true"
}

// PickPresetWeighted chooses a preset with epsilon-greedy over retention scores,
// always excluding lastKey (the avoid-last rule). rng(n) must return an int in [0,n);
// pass rand.Intn in production. rng is consumed in a fixed order: first rng(100) for
// the explore/exploit coin, then — only when exploring or when no eligible exploit
// target exists — rng(len(candidates)) for the uniform pick.
//
//   - explore (prob epsilon) OR no candidate with N >= minClips: uniform among candidates.
//   - exploit: the candidate with the highest AvgRetention among those with N >= minClips.
//
// Candidates with N < minClips are "unknown": never chosen by exploit, only reachable
// via an explore roll — so new/under-sampled presets never starve.
func PickPresetWeighted(lastKey string, scores []models.PresetScore, epsilon float64, minClips int, rng func(int) int) StylePreset {
	candidates := make([]StylePreset, 0, len(Presets))
	for _, p := range Presets {
		if p.Key != lastKey {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return Presets[0]
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	scoreByKey := make(map[string]models.PresetScore, len(scores))
	for _, s := range scores {
		scoreByKey[s.Preset] = s
	}

	bestIdx := -1
	for i := range candidates {
		s, ok := scoreByKey[candidates[i].Key]
		if !ok || s.N < minClips {
			continue
		}
		if bestIdx == -1 || s.AvgRetention > scoreByKey[candidates[bestIdx].Key].AvgRetention {
			bestIdx = i
		}
	}

	explore := rng(100) < int(epsilon*100)
	if bestIdx == -1 || explore {
		return candidates[rng(len(candidates))]
	}
	return candidates[bestIdx]
}
```

Confirm `presets.go` imports `"github.com/jaochai/video-fb/internal/models"` (it already does, for `AsTheme`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/producer/ -run 'TestPickWeighted|Preset' -v`
Expected: PASS. Then `go build ./... && go vet ./internal/producer/`.

- [ ] **Step 5: Commit**

```bash
git add internal/producer/presets.go internal/producer/presets_test.go
git commit -m "feat(producer): epsilon-greedy PickPresetWeighted + performance flag"
```

---

## Task 3: Orchestrator wiring

**Files:**
- Modify: `internal/orchestrator/orchestrator.go` (struct field, `New` param, `produceClip` selection)
- Modify: `cmd/server/main.go` (pass `analyticsRepo` into `orchestrator.New`)

**Interfaces:**
- Consumes: `producer.StylePresetsPerformanceEnabled`, `producer.PickPresetWeighted`, `producer.Default*`, `(*repository.AnalyticsRepo).PresetRetention`.

- [ ] **Step 1: Add the dependency**

In `orchestrator.go`: add `analyticsRepo *repository.AnalyticsRepo` to the `Orchestrator`
struct; add `analytics *repository.AnalyticsRepo` as a `New` parameter (place it right
after `agents *repository.AgentsRepo` to minimize call-site churn) and set
`analyticsRepo: analytics` in the returned struct literal.

In `cmd/server/main.go`: the `analyticsRepo` variable already exists (line ~120). Add it
to the `orchestrator.New(...)` argument list in the SAME position you added the param.

- [ ] **Step 2: Wire weighted selection in `produceClip`**

Read the current selection block in `produceClip` (added by the prior preset feature):

```go
preset := producer.PresetByKey("signature")
if producer.StylePresetsEnabled() {
	last, _ := o.clipsRepo.LastStylePreset(ctx)
	preset = producer.PickPreset(last)
}
```

Replace with:

```go
preset := producer.PresetByKey("signature")
if producer.StylePresetsEnabled() {
	last, _ := o.clipsRepo.LastStylePreset(ctx)
	preset = producer.PickPreset(last)
	if producer.StylePresetsPerformanceEnabled() {
		scores, err := o.analyticsRepo.PresetRetention(ctx, producer.DefaultWindowDays)
		if err != nil {
			log.Printf("preset perf: scores unavailable (%v); uniform pick %s", err, preset.Key)
		} else {
			preset = producer.PickPresetWeighted(last, scores,
				producer.DefaultEpsilon, producer.DefaultMinClips, rand.Intn)
			log.Printf("preset perf: picked %s (window=%dd, %d preset scores)",
				preset.Key, producer.DefaultWindowDays, len(scores))
		}
	}
}
```

Add `"math/rand"` to the orchestrator imports if not present.

- [ ] **Step 3: Build + vet + tests**

Run: `go build ./... && go vet ./internal/orchestrator/ && go test ./internal/orchestrator/... ./internal/producer/...`
Expected: clean / PASS.

- [ ] **Step 4: Self-check the no-op guarantee**

Confirm by reading: with `STYLE_PRESETS_PERFORMANCE_ENABLED` unset,
`StylePresetsPerformanceEnabled()` is false, the inner block is skipped, no
`PresetRetention` call is made, and `preset` is exactly what `PickPreset` returned —
identical to today.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go cmd/server/main.go
git commit -m "feat(orchestrator): performance-weighted preset selection behind 2nd flag"
```

---

## Self-Review checklist (run after writing all tasks)

- Spec coverage: PresetRetention (Task 1) ✓; epsilon-greedy + flag + defaults (Task 2) ✓;
  orchestrator wiring + fail-open + logging (Task 3) ✓; retry untouched (not modified) ✓;
  flag-off no-op (Task 3 Step 4) ✓.
- Type consistency: `models.PresetScore{Preset,AvgRetention,N}` used identically in Tasks
  1, 2, 3; `PickPresetWeighted` signature identical in plan text and test calls.
- No placeholders: all code blocks complete.

## Rollout (after all tasks)

1. Merge + deploy with `STYLE_PRESETS_PERFORMANCE_ENABLED` unset → zero behavior change.
2. Let `fetch_analytics` accumulate per-preset retention for ~2 weeks.
3. Set `STYLE_PRESETS_PERFORMANCE_ENABLED=true`; watch deploy logs for `preset perf: picked …`.
4. Rollback = unset the flag.
