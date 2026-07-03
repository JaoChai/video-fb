# Visual QA Reliability — Design Spec

**Date:** 2026-07-03
**Status:** Approved (design), pending implementation plan
**Owner:** (adsvance-v2)

## Problem

Users perceive that Visual QA flags "almost every video," forcing manual review on
every clip and defeating the point of the QA + auto-review automation.

## Root cause (evidenced)

Investigated via a 4-agent forensic sweep + prod-data queries (Neon
`snowy-grass-75448787`). Findings:

1. **Real fail rate is 34.8% (8/23 QA'd clips), not ~100%.** But of the flagged
   defects, ~67% are the *same* symptom: "scene renders as a blank/solid-color
   frame, no caption/content."

2. **The dominant cause is the QA frame sampler being scene-unaware — the videos
   are mostly fine, QA looks at the wrong frames.**
   - `extractQAFrames` (orchestrator.go:715) → `qaFrameTimestamps` →
     `evenFrameTimestamps(probedTotalDur, n)` slices the *total* duration into `n`
     **equal** parts and samples each midpoint.
   - Scenes have **very unequal** durations (3.9s–16.8s), so equal slices do **not**
     align to scene boundaries; the offset **drifts and accumulates**, so later
     scenes are sampled on **scene transitions** (crossfade → blank frame) or during
     **entrance animations** (content not yet visible → blank frame).
   - `extractQAFrames` also pairs `mids[i]` (an even-slice frame, often from a
     *neighboring* scene) with `scenes[i]`'s expected `on_screen_text` → produces
     "text on screen doesn't match expected" / "scenes out of order" flags. Verified
     example flag: *"ควรเป็น 'ห้ามใช้ Shared BIN' แต่ภาพแสดงเรื่อง OTP แยกเบอร์บัตร"* — the
     video is fine; QA compared the wrong frame to the wrong scene's text.
   - Concrete drift check on the latest failed clip (10 scenes, total 106.7s): the
     failing scenes were 7/8/9; even-slice samples i7≈80.0s and i8≈90.7s land almost
     exactly on the S8→S9 (79.2s) and S9→S10 (89.9s) transitions.

3. **The "fix" from earlier today (commit 69cf055, 12:40) did not resolve it.** It
   replaced a worse bug (all timestamps collapsing to t=0 because persisted
   `duration_seconds` was 0) with even-slicing — still scene-unaware. Per-scene
   durations are now persisted (commit 85e6cb7), and `sceneMidTimestamps()` already
   computes correct per-scene midpoints, but it is only used as an unreached
   fallback. Post-fix clips (07-02 18:18, 07-03 06:14) still fail.

4. **Auto-review is not an independent second opinion.** It samples via the *same*
   `qaFrameTimestamps` (orchestrator.go:797), so it inspects the same drifted/blank
   frames and "confirms" the QA false positives at ~0.95 confidence, marked
   `deterministic`. This is why held clips look high-confidence-bad.

5. **A minority of flags are genuine render defects** (separate from sampling):
   - **R1 — text overflow / CTA overlap:** no `max-width`/word-break/text-fit on
     `.title/.cta/.stat-label/.row .rt`; commit 2263109 narrowed the `stat` layout
     ~11% without compensating text-fitting. (layout_multi_scene.html.tmpl)
   - **R2 — Thai spelling/word-order corruption:** `captions.go` splits on
     whitespace then hard-splits at 42 runes mid-cluster; emphasis extraction uses
     boundary-unaware substring matching.
   - **R3 — generation length unbounded:** scene prompts (migration 046) only cap the
     hook scene ("<=7 words"); scenes 2–10 have no hard limit; `scene_adapter.go`
     does no length validation/truncation before render.

6. **Gate leaks (defective clips can still ship):**
   - `producer.go:395` — hyperframes `inspect()` detecting overflow is **logged and
     ignored** in production (rendered anyway); the same failure is fatal in tests.
   - All 8 QA-failed clips are `status=published` (5 had no auto-review row). Most
     predate today's block-publish gate (85e6cb7); needs verification that the
     current gate actually holds.

**Conclusion:** The QA *inspection* is misaligned (looks at wrong frames), which
explains most flags and the "review everything" pain. A minority are real render
defects. Separately, the publish gate has leaked. Fixing the sampler is the biggest
lever for the smallest, lowest-risk change.

## Design — 4 units, phased

Each unit is independently testable and shippable. Ship Phase 1 first, measure fail
rate for 1–2 days, then proceed.

### Phase 1 — Scene-aware QA frame sampling ⭐ (biggest lever, low risk)

- **Change:** Replace even-slice sampling with per-scene midpoints from the real
  persisted `duration_seconds`, **rescaled to the probed total** to correct
  estimate-vs-actual drift:
  `scale = probedDur / sum(sceneDur)` (when both > 0); `ts[i] = sceneStart_i*scale +
  sceneDur_i*scale*0.6` (sample ~60% into each scene — past entrance, far from the
  next transition).
- **Interface:** internal to `qaFrameTimestamps` / a new scene-aware helper in
  `internal/orchestrator/orchestrator.go`; callers (`extractQAFrames`, auto-review)
  unchanged in signature.
- **Fallback:** if `sum(sceneDur) == 0`, keep `evenFrameTimestamps` (last resort).
- **Verify:** unit test with unequal durations asserting each `ts[i]` falls inside
  `[start_i, end_i]` and past a configurable entrance window; re-run against the
  recently-failed clips and confirm the blank/mismatch flags disappear.

### Phase 2 — Make auto-review independent

- **Change:** Auto-review samples **multiple frames per scene** (e.g. 30/55/80% into
  each scene) at offsets distinct from QA, and only holds when a defect appears in a
  majority of a scene's frames. Goal: it can *overturn* a QA false positive.
- **Interface:** auto-review frame path in orchestrator (~line 797) gets its own
  timestamp helper; decision logic aggregates per-scene.
- **Verify:** a clip QA previously false-flagged is now auto-approved (no human).

### Phase 3 — Fix genuine render defects (largest effort, medium risk)

- **3a CSS (layout_multi_scene.html.tmpl):** add `max-width` / `word-break:break-word`
  / `overflow-wrap:anywhere` to main text elements; add font text-fit (shrink until it
  fits) for `.title/.cta/.stat-label`; restore/mitigate the `stat` layout width that
  2263109 narrowed.
- **3b Generation:** add per-layout hard character limits to the scene prompt
  (new migration) + truncate/validate in `scene_adapter.go` before render.
- **3c Thai text:** stop hard-splitting mid-cluster in `captions.go`; make emphasis
  matching boundary-aware.
- **Verify:** render sample tests with long real Thai strings; `inspect()` passes.

### Phase 4 — Close gate leaks

- **4a:** `producer.go:395` — on `inspect()` overflow, set the clip to `needs_review`
  (or retry) instead of silently rendering+publishing.
- **4b:** Audit why the 8 QA-failed clips reached `published`; confirm the current
  post-85e6cb7 gate blocks `needs_review`/held clips from publish (likely the 8 are
  pre-gate historical — verify, don't assume).

## Sequencing & risk

- **Phase 1 first**, deploy, observe fail rate for 1–2 days before investing in Phase 3.
- Low risk: 1, 2, 4a (small logic, unit-testable). Medium risk: 3 (touches template +
  prompt → changes clip appearance; requires eyeballing a real render).
- Every phase ships with tests + a verification step before deploy.

## Out of scope

- Redesigning the QA/auto-review prompts' judgement criteria (they are well-calibrated;
  the problem is which frames they see).
- Changing the video creative direction / themes.
- The kie vision HTTP-500 infra flakiness (2/30 flags) — noted, not addressed here.

## Open questions

- Phase 3a: CSS text-fit vs. a JS-based fit pass — decide during planning (start with
  CSS `word-break`/`max-width`; add JS shrink only if still overflowing).
- Phase 4b: exact mechanism by which `needs_review` clips became `published` — confirm
  during implementation (query + trace publish paths).
