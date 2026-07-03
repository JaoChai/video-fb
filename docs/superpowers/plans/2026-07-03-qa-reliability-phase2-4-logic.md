# QA Reliability Phase 2 + 4 (logic) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** (P2) make auto-review sample a different frame per scene than QA so it's a genuine independent second opinion; (P4a) make the hyperframes `inspect()` overflow check actually block publish instead of being logged-and-ignored; (P4c) guard the auto-review "approve" status write so a since-changed clip can't be forced back to `ready` (TOCTOU/duplicate-publish).

**Architecture:** All three are small backend Go changes in `internal/orchestrator`, `internal/producer`, `internal/repository`. No appearance change. Builds on Phase 1's `sceneAwareTimestamps`/`qaFrameTimestamps` (already merged).

**Tech Stack:** Go. Tests: `go test ./internal/orchestrator/`. Build: `go build ./...`.

## Global Constraints

- `go build`/`go test`/`go vet` FAIL in the default sandbox ("operation not permitted" on the go-build cache) — run them with the sandbox DISABLED.
- QA fail-open must be preserved: an absent/disabled QA or infra failure must never block a good clip nor crash the detached production goroutine.
- P4b (publish gate excluding `needs_review`) is ALREADY correct (`publisher.go:45,205` require `status='ready'`) — do NOT modify the publisher SELECT queries.
- `qaSceneFrac = 0.6` (QA). Auto-review's distinct offset: `autoReviewSceneFrac = 0.45`.

---

### Task 1: Auto-review samples a distinct frame offset (Phase 2)

**Files:**
- Modify: `internal/orchestrator/orchestrator.go` — `qaFrameTimestamps` (add `frac` param), its two callers (`extractQAFrames`, `autoReviewFrames`), add `autoReviewSceneFrac` const.
- Test: `internal/orchestrator/qa_frames_test.go`

**Interfaces:**
- Produces: `func (o *Orchestrator) qaFrameTimestamps(mp4Path string, durations []float64, frac float64) []float64`
- Produces: `const autoReviewSceneFrac = 0.45`

- [ ] **Step 1: Write the failing test** — append to `internal/orchestrator/qa_frames_test.go`:

```go
// QA and auto-review must sample DIFFERENT positions within a scene so auto-review
// is an independent second opinion (not the same frame QA already judged).
func TestSceneAwareTimestamps_QAandAutoReviewDiffer(t *testing.T) {
	durs := []float64{10, 10, 10}
	total := 30.0
	qa := sceneAwareTimestamps(durs, total, qaSceneFrac)
	ar := sceneAwareTimestamps(durs, total, autoReviewSceneFrac)
	if qaSceneFrac == autoReviewSceneFrac {
		t.Fatal("qaSceneFrac and autoReviewSceneFrac must differ for an independent second opinion")
	}
	for i := range qa {
		if qa[i] == ar[i] {
			t.Errorf("scene %d: QA and auto-review sampled the same timestamp %.3f", i, qa[i])
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run (sandbox disabled): `go test ./internal/orchestrator/ -run TestSceneAwareTimestamps_QAandAutoReviewDiffer -v`
Expected: FAIL — `undefined: autoReviewSceneFrac`.

- [ ] **Step 3: Implement** — in `internal/orchestrator/orchestrator.go`:

(a) Add the const next to `qaSceneFrac`:
```go
// autoReviewSceneFrac positions the auto-review sample at a DIFFERENT point in each
// scene than QA (qaSceneFrac), so the second-opinion judge inspects an independent
// frame and can overturn a QA false positive instead of re-confirming the same frame.
const autoReviewSceneFrac = 0.45
```

(b) Change `qaFrameTimestamps` to take `frac` and forward it:
```go
func (o *Orchestrator) qaFrameTimestamps(mp4Path string, durations []float64, frac float64) []float64 {
	probed, err := o.producer.FFmpeg().ProbeDurationSeconds(mp4Path)
	if err != nil || probed <= 0 {
		log.Printf("qa: probe duration unusable (err=%v, dur=%.3f); sampling from estimated scene durations", err, probed)
		probed = 0
	}
	if ts := sceneAwareTimestamps(durations, probed, frac); ts != nil {
		return ts
	}
	return evenFrameTimestamps(probed, len(durations))
}
```

(c) In `extractQAFrames`, change the call to pass `qaSceneFrac`:
```go
	mids := o.qaFrameTimestamps(mp4Path, durs, qaSceneFrac)
```

(d) In `autoReviewFrames`, change the call to pass `autoReviewSceneFrac`:
```go
	mids := o.qaFrameTimestamps(mp4Path, durs, autoReviewSceneFrac)
```

- [ ] **Step 4: Run to verify pass**

Run (sandbox disabled): `go build ./... && go test ./internal/orchestrator/ -v`
Expected: build OK; all tests PASS including the new one.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/orchestrator/qa_frames_test.go
git commit -m "feat(qa): auto-review samples a distinct frame offset (independent 2nd opinion)"
```

---

### Task 2: `inspect()` overflow flag blocks publish (Phase 4a)

**Files:**
- Modify: `internal/producer/producer.go` — `AssembleHyperframes916` (return an `inspectFlagged bool`), `ProduceHyperframes916` (capture it), `ProduceResult` struct (add field).
- Modify: `internal/orchestrator/orchestrator.go` — `renderAndFinalize` QA gate (OR the flag into `needs_review`).

**Interfaces:**
- Produces: `ProduceResult.InspectFlagged bool` — true when the hyperframes layout inspector flagged overflow/clip.
- Consumes (Task uses): existing `renderAndFinalize` status logic at the QA gate.

- [ ] **Step 1: Add the field to `ProduceResult`** (`internal/producer/producer.go`, struct at ~line 63):

```go
	SceneDurations []float64
	// InspectFlagged is true when the hyperframes layout inspector reported an
	// overflow/clip issue for this clip. Surfaced so the orchestrator can route
	// the clip to needs_review instead of publishing a visibly-broken layout.
	InspectFlagged bool
```

- [ ] **Step 2: Return the flag from `AssembleHyperframes916`**

Change the signature to add a `bool` before `error`:
```go
func (p *Producer) AssembleHyperframes916(ctx context.Context, clipID string, scenes []agent.GeneratedScene, preset StylePreset) (string, []float64, bool, error) {
```
Update EVERY return in that function to match the new arity:
- `return "", nil, false, fmt.Errorf("mkdir projectDir: %w", err)`
- `return "", nil, false, fmt.Errorf("build scenes: %w", err)`
- `return "", nil, false, fmt.Errorf("render: %w", err)`
- Change the inspect block to record the flag instead of only logging:
```go
	inspectFlagged := false
	if err := p.hf.renderer.Inspect(ctx, projectDir); err != nil {
		inspectFlagged = true
		log.Printf("hyperframes inspect flagged layout issues for clip %s: %v", clipID, err)
	}
```
- Success return: `return filepath.Join(projectDir, "output.mp4"), boundsToDurations(bounds), inspectFlagged, nil`

- [ ] **Step 3: Thread it through `ProduceHyperframes916`**

At the `AssembleHyperframes916` call (~line 411):
```go
	mp4Path, sceneDurations, inspectFlagged, err := p.AssembleHyperframes916(ctx, clipID, scenes, preset)
```
And in the final return (~line 438) add the field:
```go
	return &ProduceResult{Video916URL: video916URL, ThumbnailURL: thumbnailURL, LocalVideo916Path: mp4Path, SceneDurations: sceneDurations, InspectFlagged: inspectFlagged}, nil
```

- [ ] **Step 4: Route it to needs_review in `renderAndFinalize`** (`internal/orchestrator/orchestrator.go`)

Find the QA gate. Currently `status := "ready"` then a `visual_qa`-enabled block sets `needs_review` on `!qaRes.Passed`. Immediately AFTER that whole `if qaCfg... { ... }` QA block (and before status is used to persist), add:
```go
	// A hyperframes layout-inspector flag means visible overflow/clip — block publish
	// even if the vision QA gate passed or was disabled (fail-open QA can't catch it).
	if result.InspectFlagged && status == "ready" {
		status = "needs_review"
		log.Printf("clip %s: hyperframes inspect flagged layout — status=needs_review (publish blocked)", clipID)
	}
```
(If the QA block already set `needs_review`, this is a no-op; only promotes a would-be `ready` clip.)

- [ ] **Step 5: Build + test**

Run (sandbox disabled): `go build ./... && go vet ./internal/producer/ ./internal/orchestrator/ && go test ./internal/producer/ ./internal/orchestrator/ -v`
Expected: build OK; vet clean; tests PASS. (If a producer test constructs `AssembleHyperframes916`'s return, update it to the new arity.)

- [ ] **Step 6: Commit**

```bash
git add internal/producer/producer.go internal/orchestrator/orchestrator.go
git commit -m "fix(qa): hyperframes inspect overflow flag routes clip to needs_review (block publish)"
```

---

### Task 3: Guard the auto-review approve status write (Phase 4c, TOCTOU)

**Files:**
- Modify: `internal/repository/clips.go` — add `ApproveFromNeedsReview(ctx, id) (bool, error)`.
- Modify: `internal/orchestrator/orchestrator.go` — the auto-review `case "approve"` path uses the guarded method.
- Test: `internal/repository/clips_test.go` if it exists; otherwise no unit test (DB-backed) — rely on build/vet and a focused reasoning note.

**Interfaces:**
- Produces: `func (r *ClipsRepo) ApproveFromNeedsReview(ctx context.Context, id string) (bool, error)` — sets `status='ready'` ONLY when the clip is still `needs_review`; returns whether a row was updated.

- [ ] **Step 1: Add the guarded repo method** (`internal/repository/clips.go`, near `SetAutoReviewHeld`):

```go
// ApproveFromNeedsReview promotes a clip to 'ready' ONLY if it is still
// 'needs_review', so a clip whose status changed since the auto-reviewer snapshotted
// it (already published, manually handled) is left untouched. Returns true if a row
// was updated. Prevents a late auto-review "approve" from forcing a published clip
// back to 'ready' (which would re-publish it next tick).
func (r *ClipsRepo) ApproveFromNeedsReview(ctx context.Context, id string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE clips SET status = 'ready', updated_at = NOW() WHERE id = $1 AND status = 'needs_review'`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
```

- [ ] **Step 2: Use it in the auto-review approve path** (`internal/orchestrator/orchestrator.go`, `case "approve":`)

Replace the existing approve body:
```go
	case "approve":
		ready := "ready"
		if _, err := o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &ready}); err != nil {
			log.Printf("autoreview: clip %s approve->ready failed: %v", clip.ID, err)
			return
		}
		log.Printf("autoreview: clip %s APPROVED (conf %.2f) — now ready", clip.ID, res.Confidence)
```
with:
```go
	case "approve":
		updated, err := o.clipsRepo.ApproveFromNeedsReview(ctx, clip.ID)
		if err != nil {
			log.Printf("autoreview: clip %s approve->ready failed: %v", clip.ID, err)
			return
		}
		if !updated {
			log.Printf("autoreview: clip %s no longer needs_review — skipping stale approve", clip.ID)
			return
		}
		log.Printf("autoreview: clip %s APPROVED (conf %.2f) — now ready", clip.ID, res.Confidence)
```

- [ ] **Step 3: Build + test**

Run (sandbox disabled): `go build ./... && go vet ./internal/repository/ ./internal/orchestrator/ && go test ./internal/... 2>&1 | grep -vE "no test files"`
Expected: build OK; vet clean; existing tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/repository/clips.go internal/orchestrator/orchestrator.go
git commit -m "fix(qa): guard auto-review approve with WHERE status=needs_review (fix TOCTOU re-publish)"
```

---

## Self-review checklist (controller, after all tasks)
- P2: two distinct fracs; both callers updated; qaFrameTimestamps 3-arg everywhere.
- P4a: all `AssembleHyperframes916` returns updated to 4-arity; InspectFlagged plumbed; gate only promotes ready→needs_review (fail-open preserved).
- P4c: guarded write returns bool; approve path skips on 0-rows; no other caller of the old blind approve.

## Out of scope
- P4b publisher SELECT queries (already correct). Phase 3 render/appearance (separate branch/plan).
