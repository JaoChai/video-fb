# Hyperframes Motion v2 — Design

**Date:** 2026-07-07
**Status:** Approved (design), pending implementation plan
**Scope:** Visual/engagement spec #1 of 2. The content spec (hook/CTA de-dup) is separate and follows later.

## Problem

An agent-team audit found hyperframes clips look static and same-y: after each scene's ~0.6s entrance the headline/cards/stat freeze for the rest of the 5–11s scene (only the background ken-burns), every scene enters identically (fade + slide-up), and stat numbers just pop in. This spec adds three motion upgrades to raise retention, all in the render template, all behind one kill-switch flag.

Out of scope (separate content spec): hook authored upstream, hook/CTA rotation + de-dup. That is LLM/content-pipeline work with different files and verification.

## Goals

- Kill the "frozen middle" of each scene with subtle continuous motion.
- Make consecutive scenes enter differently so the clip doesn't feel repetitive.
- Animate stat numbers counting up.
- Ship dark: one flag, default off, so we render + eyeball before enabling on prod.
- Flag off ⇒ byte-for-byte current behavior.

## The three changes

All live in `internal/producer/templates/layout_multi_scene.html.tmpl` (one paused, seek-safe GSAP timeline). One new flag `SCENE_MOTION_V2_ENABLED` gates all three: Go reads it (`SceneMotionV2Enabled()`), passes `ScenesParams.MotionV2 bool` into the template, which exposes `const MOTION_V2` (same wiring as `AudioMotion` → `MOTION_UP`, template:166). When `MOTION_V2` is false, none of the new tweens are scheduled.

### 1. Mid-scene parallax drift

After a scene's content-entrance settles (at `y:0`), add one continuous tween drifting the whole `.scene-content` block `y: 0 → -12px` over the remainder of the scene, `ease:"none"` — opposite the background's ken-burns zoom, giving subtle depth so the focal content is never dead-still.

The drift **must start after the entrance tween ends** (entrance also animates `content.y`); overlapping tweens on the same property conflict. Start the drift at roughly `sc.start + entranceDuration` and run it to `sc.end`. The per-child stagger (template:263–273) animates the children's transforms, which compose with the parent drift — no conflict. Applies to every layout.

### 2. Entrance variety

Three entrance geometries, rotated per scene by index so no two consecutive scenes match (rotation guarantees this):
- **rise** — current: `y:+ → 0` + fade
- **slide** — `x:±60 → 0` + fade (direction alternates)
- **punch** — `scale:0.9 → 1` + fade

The variant is chosen in **Go** — a pure helper `entranceForScene(idx int) string` (returns `"rise"`/`"slide"`/`"punch"`) — and carried on the per-scene content object as `sc.entrance` (mirrors how `sc.speed` already flows via `Content.Speed`, `scene_adapter.go`). The template applies the chosen geometry to the content-entrance `fromTo` (template:250–252 / 258–260).

Variant picks **geometry only**; the existing per-theme `ENTRANCE_EASE`/`ENTRANCE_DUR` and per-layout `SPEED_FACTOR` still pick the ease/duration. They compose. (The hook scene, idx 0, keeps its fast punchy entrance.)

### 3. Count-up on stat scenes

On `stat` scenes the number (`sc.stat`, rendered at template:205 as `<div class="stat">N<span class="unit">…</span></div>`) animates `0 → N`: a GSAP tween on a proxy object with an `onUpdate` that rewrites the number text while preserving the `.unit` span. Seek-safe (GSAP recomputes on timeline seek).

Parsing is in JS: extract the leading numeric part of `sc.stat` (handle integers, decimals, thousands separators). If it is not parseable (non-numeric stat), **skip the count-up and render the value statically** — graceful fallback, never a blank or NaN.

## Feature flag

| Flag | Off behavior |
|------|--------------|
| `SCENE_MOTION_V2_ENABLED` | none of the three tweens scheduled — current template behavior exactly |

Follows the existing env pattern (`presets.go:93` `StylePresetsEnabled`, `audio.go:17` `AudioMotionEnabled`): `os.Getenv("SCENE_MOTION_V2_ENABLED") == "true"`. No migration (template + `scene_adapter.go` only).

## Verification

Template JS has no test runner, so eyeballing a real render is the primary gate.

1. **Go unit test** — `entranceForScene`: correct rotation, no two consecutive scenes equal, deterministic (no randomness).
2. **Render a real clip with the flag ON**, extract multiple frames per scene, and check:
   - count-up: the stat number differs across frames within a stat scene (0→N).
   - parallax: `.scene-content` position shifts across a scene's frames, smoothly.
   - entrance variety: consecutive scenes enter differently.
   - no overflow / blank / broken layout.
3. **Render with the flag OFF** — confirm output is unchanged (regression guard).
4. Full motion smoothness is confirmed by a human eyeball of the MP4.

**Honest risk:** a local render needs `npx hyperframes` + headless Chrome + a DB (TTS keys). If the dev environment cannot render, eyeballing happens on **one prod clip** with the flag on (enable flag → produce one clip → inspect → keep on or flip off). This will be called out explicitly at that step.

## Rollout

1. Implement (TDD the Go helper); `go test ./...` green.
2. Render + eyeball with the flag on; tune drift distance / count-up feel if needed.
3. `/simplify` the diff.
4. Merge → push → Railway auto-deploy with the flag **OFF** (no behavior change).
5. Set `SCENE_MOTION_V2_ENABLED=true` on prod when ready; eyeball the first real prod clip.

## Rollback

1. First line: flag `=false` (immediate).
2. Second line: revert the commit.

## Verification checklist (definition of done)

- [ ] `entranceForScene` unit test passing (rotation, no-repeat, deterministic).
- [ ] `go test ./...` green; flag defaults off in code.
- [ ] One clip rendered with the flag on, frames inspected: count-up progresses, content drifts, entrances vary, no breakage.
- [ ] One clip rendered with the flag off, confirmed identical to pre-change behavior.
