# QA Scene-Aware Frame Sampling — Implementation Plan (Phase 1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Visual QA and auto-review sample one frame per scene at the true middle of each scene (using real per-scene durations rescaled to the probed video length), so samples stop landing on scene transitions/entrance animations (blank frames) and stop pairing the wrong scene's frame with the wrong scene's text.

**Architecture:** Both the QA path (`extractQAFrames`) and the auto-review path (`autoReviewFrames`) already route frame-timestamp selection through `Orchestrator.qaFrameTimestamps`. Today that function ignores the per-scene midpoints it is handed and instead slices the *total* probed duration into N equal parts — which drifts off scene boundaries because scenes have unequal durations. We introduce one pure helper, `sceneAwareTimestamps`, that positions each sample a fixed fraction into its own scene using the real durations rescaled to the probed total, and rewire `qaFrameTimestamps` (and its two callers) to use it. This is the Phase 1 slice of `docs/superpowers/specs/2026-07-03-visual-qa-reliability-design.md`.

**Tech Stack:** Go, `internal/orchestrator`, existing `FFmpeg().ProbeDurationSeconds` / `ExtractFrameAt`.

## Global Constraints

- Language: Go; build check is `go build ./...`.
- Per-scene durations are already persisted (commit 85e6cb7) and available as
  `agent.GeneratedScene.DurationSeconds` (QA path) and `models.Scene.DurationSeconds`
  (auto-review path), both `float64` seconds.
- Fail-open behavior must be preserved: a missing/unusable probe must not crash; it
  falls back to sampling from estimated durations, and only when there are no usable
  durations at all does it fall back to the existing `evenFrameTimestamps`.
- Sample fraction into each scene is `qaSceneFrac = 0.6` (past entrance animation, well
  before the next transition).

---

### Task 1: `sceneAwareTimestamps` pure helper + tests

**Files:**
- Modify: `internal/orchestrator/orchestrator.go` (add helper + `qaSceneFrac` const near the existing `evenFrameTimestamps`, ~line 679–694)
- Test: `internal/orchestrator/qa_frames_test.go`

**Interfaces:**
- Produces: `func sceneAwareTimestamps(durations []float64, probedDur, frac float64) []float64` — returns one timestamp per input duration, each positioned `frac` into its scene, rescaled by `probedDur / sum(durations)`; returns `nil` when `sum(durations) <= 0`. `probedDur <= 0` means "no rescale" (scale = 1).
- Produces: `const qaSceneFrac = 0.6`

- [ ] **Step 1: Write the failing tests**

Edit `internal/orchestrator/qa_frames_test.go` — change the import line `import "testing"` to:

```go
import (
	"math"
	"testing"
)
```

Then append:

```go
// Each sample must fall strictly inside its own scene's [start,end) window even
// when scene durations are wildly unequal — this is what stops samples landing on
// scene transitions (blank crossfade frames).
func TestSceneAwareTimestamps_LandsInsideEachScene(t *testing.T) {
	durs := []float64{3.92, 10.68, 9.16, 9.72, 8.32, 12.56, 12, 12.88, 10.68, 16.8}
	var total float64
	for _, d := range durs {
		total += d
	}
	ts := sceneAwareTimestamps(durs, total, qaSceneFrac) // probed == sum → scale 1
	if len(ts) != len(durs) {
		t.Fatalf("want %d timestamps, got %d", len(durs), len(ts))
	}
	var start float64
	for i, d := range durs {
		end := start + d
		if ts[i] <= start || ts[i] >= end {
			t.Errorf("scene %d: ts %.3f not inside (%.3f, %.3f)", i, ts[i], start, end)
		}
		start = end
	}
}

// When the probed duration differs from the sum of estimates, every sample must
// scale by probed/sum so it still lands in the right place on the real encode.
func TestSceneAwareTimestamps_RescalesToProbed(t *testing.T) {
	durs := []float64{10, 30, 10} // sum 50
	ts := sceneAwareTimestamps(durs, 100, 0.6) // scale 2.0
	want := []float64{
		(0 + 10*0.6) * 2,  // 12
		(10 + 30*0.6) * 2, // 56
		(40 + 10*0.6) * 2, // 92
	}
	if len(ts) != len(want) {
		t.Fatalf("want %d, got %d", len(want), len(ts))
	}
	for i := range want {
		if math.Abs(ts[i]-want[i]) > 1e-6 {
			t.Errorf("i%d: want %.3f got %.3f", i, want[i], ts[i])
		}
	}
}

// No usable durations (all zero) → nil, so the caller can fall back.
func TestSceneAwareTimestamps_ZeroDurationsNil(t *testing.T) {
	if ts := sceneAwareTimestamps([]float64{0, 0, 0}, 30, 0.6); ts != nil {
		t.Errorf("want nil for zero durations, got %v", ts)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/orchestrator/ -run TestSceneAwareTimestamps -v`
Expected: FAIL — `undefined: sceneAwareTimestamps` (and `undefined: qaSceneFrac`).

- [ ] **Step 3: Write the implementation**

In `internal/orchestrator/orchestrator.go`, immediately after `evenFrameTimestamps` (ends ~line 694), add:

```go
// qaSceneFrac positions each QA/auto-review sample this fraction into its scene —
// far enough past the entrance animation that content is visible, and far enough
// before the next scene that it never lands on a transition/crossfade frame.
const qaSceneFrac = 0.6

// sceneAwareTimestamps returns one timestamp per scene, each positioned `frac` into
// its own scene using the real per-scene durations, then rescaled so the estimated
// total maps onto the probed video duration. This keeps every sample inside its
// intended scene even when scene durations are unequal (unlike naive even slicing,
// which drifts onto transitions). Returns nil when durations sum to <= 0 so the
// caller can fall back. probedDur <= 0 means "don't rescale" (scale = 1).
func sceneAwareTimestamps(durations []float64, probedDur, frac float64) []float64 {
	var total float64
	for _, d := range durations {
		if d > 0 {
			total += d
		}
	}
	if total <= 0 {
		return nil
	}
	scale := 1.0
	if probedDur > 0 {
		scale = probedDur / total
	}
	ts := make([]float64, len(durations))
	var acc float64
	for i, d := range durations {
		if d < 0 {
			d = 0
		}
		ts[i] = (acc + d*frac) * scale
		acc += d
	}
	return ts
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/orchestrator/ -run TestSceneAwareTimestamps -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/orchestrator/qa_frames_test.go
git commit -m "feat(qa): add scene-aware frame timestamp helper"
```

---

### Task 2: Rewire `qaFrameTimestamps` and both callers to be scene-aware

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`
  - `qaFrameTimestamps` (~line 700–710) — new signature + use `sceneAwareTimestamps`
  - `extractQAFrames` (~line 716) — build durations, call new signature
  - `autoReviewFrames` (~line 797) — build durations, call new signature
  - Remove now-dead `sceneMidTimestamps` (~line 665–677) and `sceneMids` (~line 749–758)

**Interfaces:**
- Consumes: `sceneAwareTimestamps`, `qaSceneFrac` (Task 1); existing
  `o.producer.FFmpeg().ProbeDurationSeconds(mp4Path) (float64, error)` and
  `evenFrameTimestamps(duration float64, n int) []float64`.
- Produces: `func (o *Orchestrator) qaFrameTimestamps(mp4Path string, durations []float64) []float64`.

- [ ] **Step 1: Replace `qaFrameTimestamps`**

Replace the whole existing `qaFrameTimestamps` func (the one calling `evenFrameTimestamps(dur, n)`) with:

```go
// qaFrameTimestamps returns one timestamp per scene for frame extraction, each
// positioned qaSceneFrac into its scene via real per-scene durations rescaled to
// the probed video length. Falls back to naive even slicing only when per-scene
// durations are unavailable (all zero).
func (o *Orchestrator) qaFrameTimestamps(mp4Path string, durations []float64) []float64 {
	probed, err := o.producer.FFmpeg().ProbeDurationSeconds(mp4Path)
	if err != nil || probed <= 0 {
		log.Printf("qa: probe duration unusable (err=%v, dur=%.3f); sampling from estimated scene durations", err, probed)
		probed = 0
	}
	if ts := sceneAwareTimestamps(durations, probed, qaSceneFrac); ts != nil {
		return ts
	}
	return evenFrameTimestamps(probed, len(durations))
}
```

- [ ] **Step 2: Update `extractQAFrames` caller**

In `extractQAFrames`, replace:

```go
	mids := o.qaFrameTimestamps(mp4Path, len(scenes), sceneMidTimestamps(scenes))
```

with:

```go
	durs := make([]float64, len(scenes))
	for i, s := range scenes {
		durs[i] = s.DurationSeconds
	}
	mids := o.qaFrameTimestamps(mp4Path, durs)
```

- [ ] **Step 3: Update `autoReviewFrames` caller**

In `autoReviewFrames`, replace:

```go
	mids := o.qaFrameTimestamps(mp4Path, len(scenes), sceneMids(scenes))
```

with:

```go
	durs := make([]float64, len(scenes))
	for i, s := range scenes {
		durs[i] = s.DurationSeconds
	}
	mids := o.qaFrameTimestamps(mp4Path, durs)
```

- [ ] **Step 4: Delete the now-dead helpers**

Delete the entire `sceneMidTimestamps` function (the `[]agent.GeneratedScene` mid helper, ~line 665–677) and the entire `sceneMids` function (the `[]models.Scene` mid helper, ~line 749–758). They have no remaining callers after Steps 2–3.

- [ ] **Step 5: Build, vet, test**

Run: `go build ./... && go vet ./internal/orchestrator/ && go test ./internal/orchestrator/ -v`
Expected: build OK; vet clean; all tests PASS (no `declared and not used` / `undefined` errors — confirms the dead helpers were fully removed and both callers compile).

- [ ] **Step 6: Commit**

```bash
git add internal/orchestrator/orchestrator.go
git commit -m "fix(qa): sample one frame per scene via scene-aware timestamps

Both the QA and auto-review frame paths sliced total duration into equal
parts, which drifts off unequal scene boundaries onto transitions (blank
frames) and pairs the wrong scene's frame with the wrong scene's text.
Use real per-scene durations positioned 60% into each scene, rescaled to
the probed total. Removes the now-dead sceneMidTimestamps/sceneMids."
```

---

## Post-implementation verification (before declaring Phase 1 done)

1. `go build ./...` and `go test ./internal/orchestrator/` pass.
2. Deploy (push to master → Railway auto-deploys backend). Do NOT deploy while a clip
   is producing (see project memory).
3. After the next scheduled clips render (06:00/12:00/18:00), query prod
   (`snowy-grass-75448787`):
   ```sql
   SELECT vq.created_at AT TIME ZONE 'Asia/Bangkok', vq.passed,
          (SELECT count(*) FROM jsonb_array_elements(vq.issues) e WHERE (e->>'ok')::bool=false) AS failed_scenes
   FROM visual_qa vq ORDER BY vq.created_at DESC LIMIT 6;
   ```
   Expected: post-deploy clips show a markedly lower fail rate; the "blank/solid frame"
   and "text doesn't match on_screen_text" issue strings should largely disappear.
4. If fail rate stays high on genuinely-broken frames, that isolates the remaining
   real-defect work (spec Phases 3–4).

## Out of scope (later phases, separate plans)

- Phase 2 (multi-frame independent auto-review), Phase 3 (render text-fit + generation
  length limits + Thai text), Phase 4 (close inspect()/publish gate leaks). Each gets
  its own plan after Phase 1 is measured.
