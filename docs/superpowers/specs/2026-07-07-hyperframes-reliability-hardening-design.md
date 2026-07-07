# Hyperframes Reliability Hardening — Design

**Date:** 2026-07-07
**Status:** Approved (design), pending implementation plan
**Scope:** Reliability spec #1 of a two-spec set. The visual/engagement spec is separate and follows this one.

## Problem

An agent-team audit of the hyperframes video pipeline (LLM → images/TTS → HTML+GSAP render → visual QA → R2 upload → publish) found several places where a clip can render wrong, lose audio, or fail the whole clip — and in the worst cases **ship the defect silently** because visual QA only inspects a still frame per scene and cannot see motion, audio, or upload health.

This spec hardens six of those spots. All six are small, low-risk backend changes verifiable by unit test — **no render eyeball required**. Two adjacent items are deliberately excluded (see Non-goals).

## Goals

- No broken/frozen clip is ever published — it is retried, then routed to human review.
- An R2 outage never fails a whole clip's upload.
- A clip with missing/truncated narration is caught, not shipped.
- Visual QA gains an audio-presence check it currently lacks.
- Remove two known false-positive / budget-waste behaviors.

## Non-goals (explicitly excluded)

- **Title/stat vertical overflow** (`scene_adapter.go` uncapped Title/Stat/Num; `.scene-content` has no `max-height`). Fixing this needs a real render to tune `max-height`/auto-fit — it belongs to the **visual spec**, not this backend-only spec.
- **"Single scene TTS failure fails the whole clip"** (`scene_voice.go:38-40`). This is kept as-is: retrying the whole clip is correct and preferable to shipping a clip with one silent scene.
- No schema changes. Every fix reuses the existing `needs_review` queue and existing columns.

## The Six Fixes

Priority: 🔴 high, 🟡 medium, 🟢 low.

### Fix 1 🔴 — Broken/frozen render is detected but published anyway

**Current:** `HyperframesRenderer.run` (`internal/producer/hyperframes.go:29`) already calls `scanBrowserIssues` (`hyperframes.go:51`), which catches `[Browser:PAGEERROR]`, `Composition script failed`, and `is not defined` — the exact signatures of a GSAP/timeline JS error that freezes every scene on frame 1. But on a hit it only `log.Printf`s (`hyperframes.go:40-41`); the result is not returned, so `Render` (`hyperframes.go:101`) succeeds and the frozen clip publishes. Visual QA cannot catch it — a frozen clip still yields a valid-looking still frame at t≈60%.

**Fix:** Surface browser issues from `run`/`Render` as a render-suspect signal. When present:
1. Treat the render as a **failure** so the existing `retry_failed` tick (`scheduler.go:213`, `ClipsRepo.ListFailed` `clips.go:138`, `maxRetries`) re-renders it — a transient JS hiccup may pass on retry.
2. Once retries are exhausted and it still trips, route the clip to **`needs_review`** (reuse the `InspectFlagged` → `needs_review` path at `orchestrator.go:502-504`) instead of `failed`/publish.

**Flag:** `RENDER_ERROR_GATE_ENABLED` (env, `os.Getenv(...) == "true"` per existing pattern). Off → current behavior (log only).

**Verify (unit test):** mock renderer output containing `[Browser:PAGEERROR]` → assert render is treated as failure / clip is not published.

### Fix 2 🔴 — R2 upload failure fails the entire clip, with no runtime fallback

**Current:** `uploadPersistent` (`producer.go:415-419`) picks R2 **xor** kie up front via `p.r2.Enabled(ctx)` (`r2.go:67`). If R2 is enabled but `p.r2.Upload` errors at runtime (rotated creds, network, bucket issue), the error propagates and fails the clip (`producer.go:444-448`). The documented "kie fallback" only covers R2 being *disabled/unconfigured*, never an upload *error*. An R2 outage therefore fails every clip's upload.

**Fix:** In `uploadPersistent`, if the R2 `Upload` returns an error, log an alert and fall back to `p.kie.UploadFile(ctx, localPath, kieDir)` instead of returning the error.

**Flag:** none — pure safety, only ever helps.

**Trade-off (accepted):** kie fallback URLs are temporary (this is why R2 exists — see `project_r2_permanent_storage`). A clip uploaded via fallback during an R2 outage risks its URL expiring before a late publish. This is strictly better than losing the clip outright, and R2 outages are expected to be rare/short. The alert log makes the outage visible so it can be fixed before it matters.

**Verify (unit test):** mock R2 `Upload` returning an error → assert `kie.UploadFile` is called and no error propagates.

### Fix 3 🟡 — Silently truncated TTS ships a clip with missing narration

**Current:** `openrouter.go:189-190` only logs `WARNING: TTS audio unusually short` when audio < 5s for > 100 chars. It does not retry or fail. The short scene gets a short duration bound (`scene_voice.go:71` `computeBounds`) → missing narration + captions, and the clip still renders and publishes.

**Fix:** When TTS output is detected too-short-for-input, **retry the TTS call once**. If it is still too short, treat the scene's TTS as failed (return an error like `scene_voice.go:38-40` does), which fails the clip → existing retry tick re-produces it.

**Flag:** `TTS_LENGTH_GATE_ENABLED`. Off → current behavior (warning only).

**Verify (unit test):** mock a short-audio result → assert one retry happens, then a scene-TTS error is returned when still short.

### Fix 4 🟡 — Visual QA is blind to audio

**Current:** QA inspects one still frame per scene at ~60% depth (`extractQAFrames` `orchestrator.go:767`, `qaFrameTimestamps` `orchestrator.go:752`). It never checks that voice audio is present, of expected length, or non-silent. A silent or absent-voice clip passes QA.

**Fix:** Add an audio probe to the QA path: measure the combined `voice.wav` duration and near-silence ratio. Scenes/clips with silent or absent voice → flag to **`needs_review`** (reuse the existing `status = "needs_review"` gate at `orchestrator.go:493`).

**Flag:** `QA_AUDIO_CHECK_ENABLED`. Off → no audio check.

**Verify (unit test):** feed a silent/near-empty wav → assert the clip is flagged `needs_review`.

### Fix 5 🟢 — Empty-VoiceText scene creates a QA false positive

**Current:** a scene with empty `VoiceText` gets a zero-duration placeholder (`scene_voice.go:27-33`) → zero-width `[start,end)` bound → `sceneAwareTimestamps` (`orchestrator.go:721`) samples `acc + 0*frac` = exactly the transition boundary → blank frame → QA flags it (re-introducing the Phase-1 class of false positive that scene-aware sampling was meant to remove).

**Fix:** In frame extraction / `sceneAwareTimestamps`, skip QA frame extraction for scenes whose duration is zero (or clamp the sample off the transition boundary).

**Flag:** none — pure correctness.

**Verify (unit test):** durations array containing a `0` → assert no frame is sampled at the transition boundary for that scene.

### Fix 6 🟢 — Deploy mid-render burns a clip's retry budget

**Current:** `ResetStaleProducing` (`clips.go:124`) recovers clips orphaned in `producing` (e.g. by a deploy) but does `retry_count = retry_count + 1` (`clips.go:129`). Two deploys landing during the same clip's render exhaust `maxRetries` → a good clip gets permanently stuck `failed`.

**Fix:** Do **not** increment `retry_count` in `ResetStaleProducing`. An interrupted render is an infrastructure event, not a content failure. (`fail_reason`/status handling stays.)

**Flag:** none — pure correctness.

**Verify (unit test):** run recovery on a stale-producing clip → assert `retry_count` is unchanged.

## Feature-flag summary

| Fix | Flag | Off behavior |
|-----|------|--------------|
| 1 broken-render gate | `RENDER_ERROR_GATE_ENABLED` | log only (current) |
| 3 TTS-too-short gate | `TTS_LENGTH_GATE_ENABLED` | warning only (current) |
| 4 QA audio check | `QA_AUDIO_CHECK_ENABLED` | no audio check |
| 2, 5, 6 | none | ship unconditionally — only ever help |

Flags follow the existing env pattern (`presets.go:93` `StylePresetsEnabled`, `audio.go:17` `AudioMotionEnabled`).

## Rollout

1. Implement each fix TDD (test fails first) per the Verify note above.
2. `go test ./...` green (regression guard).
3. Smoke test one real clip on a branch/live-test: confirm a **healthy** clip passes all new gates (no false block) and R2 upload still works.
4. `/simplify` the diff.
5. `/pre-deploy-checklist`.
6. Push master → Railway auto-deploys (monorepo, no migration).

## Monitoring (24–48h post-deploy)

- `needs_review` queue volume — a spike means gate #1 or #4 is over-triggering → flip its flag off.
- `failed` clip rate — a spike means gate #3 is over-triggering → flip its flag off.
- R2-vs-kie upload logs — confirms fallback #2 isn't firing abnormally often (R2 health).

## Rollback

1. First line: flip the offending flag off (immediate, no redeploy).
2. Second line: revert the commit.

## Verification checklist (definition of done)

- [ ] Six unit tests (one per fix) written and passing.
- [ ] `go test ./...` green.
- [ ] One real clip produced end-to-end without a false block from any new gate.
- [ ] Three flags default-off in code until deliberately enabled.
