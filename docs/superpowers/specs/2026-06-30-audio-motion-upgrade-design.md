# Audio + Motion Upgrade for Hyperframes Clips — Design

**Date:** 2026-06-30
**Status:** Approved (brainstorming) — pending implementation plan

## Goal

Add a richer audio/visual layer to the live hyperframes video pipeline:

1. **Scene-transition sound effects** — a short whoosh/swish at each scene boundary.
2. **Background ambient music** — one looping bed per clip, picked random avoid-last, mixed quietly under the voice.
3. **Safer, more polished motion design** — upgraded GSAP transitions (slide + scale + fade between scenes), accent motion on stat/CTA, tighter easing — using transform/opacity only (GPU-friendly), no render-cost increase.

Today the only audio is the TTS voice (`voice.wav`); there are no sound effects and no background music.

## Non-Goals (YAGNI)

- Dynamic volume ducking of ambient under voice (use a fixed low volume instead).
- AI-generated sound (no ElevenLabs SFX API) — asset library only.
- Per-element / per-animation-event SFX (transition-only).
- Heavy motion: no blur, particle systems, parallax layers, or 3D.
- Touching the scene-generation agents (`scene.go`, `scene_content.go`).

## Key Architectural Decision

**This is a render-template + asset-library feature, NOT an agent feature.**

The scene content agents stay untouched. All changes live in the producer/template/builder layer — the same layer the Style Presets feature changed. This keeps the content pipeline (research → scenes → voice text) stable and isolates the blast radius.

Locked decisions from brainstorming:

| Topic | Decision |
|---|---|
| Sound source | In-repo asset library (royalty-free), no API, deterministic |
| SFX scope | Scene-transition only (whoosh/swish at each boundary) |
| Motion ambition | Safe upgrade — transform/opacity only, no added render load |
| Ambient selection | Random avoid-last, NOT tied to style preset, fixed low volume |
| Rollout | Feature flag `AUDIO_MOTION_ENABLED`, default off |

## Current Pipeline (verified from code)

The live path is `ProduceHyperframes916` → `AssembleHyperframes916` (`internal/producer/producer.go`):

1. `synthScenesVoice` TTSes each scene and concatenates to `voice.wav` with per-scene `[start,end)` bounds (`scene_voice.go`).
2. Per-scene image generation (gpt-image-2), with a CSS-gradient fallback.
3. `CompositionBuilder.BuildScenes` writes a Hyperframes project, copying `voice.wav` into `assets/` and setting `params.VoiceSrc = "assets/voice.wav"` (`composition_builder.go`).
4. `HyperframesRenderer.Render` shells out to the pinned Hyperframes CLI to render `output.mp4`.

The motion lives in the embedded GSAP timeline inside `internal/producer/templates/layout_multi_scene.html.tmpl`. The voice is one audio element:

```html
<audio id="vo" data-start="0" data-duration="{{.DurationSeconds}}" data-track-index="5" src="{{.VoiceSrc}}" data-volume="1"></audio>
```

The `data-track-index` + `data-volume` attributes strongly imply the engine supports **multiple audio tracks** mixed at render. This is the load-bearing assumption (see Risk below).

## Components to Add / Change

### 1. Asset library — `internal/producer/assets/audio/`

```
assets/audio/
  sfx/transition/   whoosh1.mp3, swish1.mp3, ...   (short, < 1s)
  ambient/          bed1.mp3, bed2.mp3, ...         (loopable, ~30–60s)
```

Files are royalty-free. They are made available to the renderer the same way fonts/GSAP already are (copied into the per-clip project `assets/` dir by the builder).

### 2. Selection logic (Go) — builder/producer

- **Ambient:** pick one ambient file at random, avoiding the last one used. Persist the last pick in `settings` (key e.g. `last_ambient`), mirroring how Style Presets track `last_preset`.
- **Transition SFX:** pick a transition file per scene boundary (varied across boundaries; can be deterministic round-robin or random).

### 3. Template wiring — `layout_multi_scene.html.tmpl`

Add audio tracks alongside the existing voice track, reusing the engine's multi-track support:

- **Ambient bed:** one `<audio>` spanning the full clip, `data-volume≈0.15` (quiet under voice), looped/padded to clip duration, on its own track index.
- **Transition SFX:** one `<audio>` per scene boundary, `data-start` = boundary time, moderate volume, on its own track index.

Exact track indices chosen to not collide with existing ones (voice=5, badges=2, progress=7, captions=9, scenes=10+i).

### 4. Builder — `composition_builder.go` (`BuildScenes`)

- Copy the chosen ambient file and the chosen transition SFX file(s) into the project `assets/` dir (like `voice.wav`).
- Extend `ScenesParams` with the new asset paths + per-boundary SFX timing so the template can render the audio elements.

### 5. Motion upgrade — GSAP timeline in the template

- **Scene transition:** combine the current fade with a slide + slight scale (outgoing scene scales/fades out, incoming slides up + fades in) — transform/opacity only.
- **Accent motion:** subtle emphasis on stat reveal and CTA (build on the existing `back.out` pop).
- **Easing:** tighten to a consistent, premium easing curve.
- Constraint: no properties that force expensive repaints (no filter/blur/box-shadow animation, no large particle counts).

## Risk & Verification (do FIRST)

**Load-bearing assumption:** the Hyperframes CLI mixes multiple `<audio>` tracks into the rendered MP4. Only the single voice track is exercised today, so this is unverified.

**Plan Task 0 (spike):** build one test project with voice + ambient + one transition SFX, render it through the pinned CLI, and confirm the output MP4 contains all three audio sources mixed and audible.

- **If yes:** proceed with the template-track design above.
- **If no:** fallback — mux ambient + SFX after render with `ffmpeg amix` (the producer already wraps ffmpeg via `FFmpegAssembler`). This changes components 3–4; resolve it before implementing the rest.

## Rollout & Rollback

- Feature flag `AUDIO_MOTION_ENABLED` (env var), default **off**.
- Flag off → exact current behavior (voice-only, current motion). No-op.
- Flag on → new audio tracks + upgraded motion.
- Rollback = set flag off (no redeploy of old code needed).
- Mirrors the Style Presets rollout (`STYLE_PRESETS_ENABLED`).

## Success Criteria

1. A clip rendered with the flag on has: audible ambient music (quiet, under voice), a transition SFX at each scene boundary, and visibly upgraded scene transitions.
2. With the flag off, output is byte-for-byte equivalent in behavior to today (voice-only).
3. Render time and memory stay within the current prod envelope (no new timeouts; 3 render workers / 20m timeout unchanged).
4. No changes to the scene-generation agents.

## Files Touched (anticipated)

| File | Change |
|---|---|
| `internal/producer/assets/audio/**` | New royalty-free SFX + ambient assets |
| `internal/producer/composition_types.go` | New fields on `ScenesParams` for ambient/SFX paths + timing |
| `internal/producer/composition_builder.go` | Copy chosen audio into project assets; populate params |
| `internal/producer/templates/layout_multi_scene.html.tmpl` | New `<audio>` tracks + upgraded GSAP motion |
| `internal/producer/producer.go` (or builder) | Ambient avoid-last selection; SFX selection; flag gate |
| settings | `last_ambient` tracking; `AUDIO_MOTION_ENABLED` flag |
