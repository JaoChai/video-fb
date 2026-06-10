# Hyperframes + kie.ai Pipeline Redesign

**Date:** 2026-06-10
**Status:** Design — awaiting review
**Supersedes the video engine of:** the current static-image FFmpeg pipeline (`internal/producer/producer.go` `AssembleSingleImage*`)

---

## 1. Overview & Goals

Rebuild the video production system around three changes:

1. **Video engine → hyperframes.** Replace the current "1 static image + voiceover" output with multi-scene, animated 9:16 videos rendered by [hyperframes](https://github.com/heygen-com/hyperframes) (HTML/CSS/GSAP → headless Chrome → MP4). Reuse the proven approach from the shelved `hfslice` prototype: LLM emits constrained scene JSON, a Go templater fills one lint-clean multi-scene HTML template, then `npx hyperframes render` produces the MP4.
2. **All LLM agents → kie.ai.** Every agent moves off OpenRouter onto kie.ai, using **Claude Sonnet 4.6** for quality-critical creative work and **Gemini 3.5 Flash** for cheap/mechanical work. Images use **gpt-image-2** (already wired in `kieai.go`).
3. **New content flow.** Shift from question-driven Q&A to a topic-driven explainer pipeline: **research → script → scene breakdown → assemble**, optimized for clarity and viewer comprehension.

**Audio is unchanged** — keep the existing OpenRouter Gemini TTS path (`openrouter.go` `GenerateVoice`), per requirement.

### Confirmed decisions (from brainstorming)
- **Input:** rotate existing `categories` (settings); the Research agent finds a fresh, specific topic + angle within the week's category. Questioner concept dropped.
- **Output:** 9:16 vertical only (1080×1920).
- **Length:** 60–90 s, ~6–10 scenes.
- **Render arch:** Approach A — Go shells out to `npx hyperframes render` inside one Docker image bundling Node 22 + Chrome + FFmpeg (proven by `hfslice`).
- **Brand:** Ads Vance navy + orange + leopard mascot (`#0f1d35` navy, `#ff6b2b` orange). NOT the rolled-back royal-blue/amber theme.
- **Model assignment:** as analyzed below.

### Non-goals
- 16:9 output (dropped).
- True forced-alignment word timing (use per-scene measured durations + staggered word reveal, as `hfslice` did).
- Porting the hyperframes engine to Go (rejected).
- Separate render microservice (deferred; revisit if render load grows).
- Reworking publishing/analytics layers (`publisher`, `analyzer`) — out of scope.

---

## 2. Model & Agent Assignment

| Agent | Model (via kie.ai) | Rationale |
|---|---|---|
| **Research** | Gemini 3.5 Flash + `googleSearch` tool | Only option with live web grounding + citations (Claude on kie.ai has no web search). Cheap, fast, factual. |
| **Script** | Claude Sonnet 4.6 | Thai writing quality, hook craft, tone — Claude's strength. |
| **Scene/Director** | Claude Sonnet 4.6 | Structured + creative reasoning: choose layout, emphasis words, pacing per scene. |
| **ImagePrompt** | Gemini 3.5 Flash | Mechanical scene→prompt transform, brand-styled. Cheap. |
| **Metadata** | Gemini 3.5 Flash | Short generative task (title/desc/tags). |
| **Dedup** | Gemini 3.5 Flash | Comparison task. Cheap. |
| **Images** | `gpt-image-2-text-to-image` (async job) | Per requirement; client exists in `kieai.go`. |
| **Voice/TTS** | OpenRouter `gemini-3.1-flash-tts-preview` (unchanged) | "Use existing audio." |

**Principle:** Claude where creative quality is load-bearing (script + scene breakdown); Gemini Flash for volume/mechanical steps; spend is concentrated where it matters.

---

## 3. Architecture & Data Flow

```
Orchestrator (topic-driven, weekly cron)
  │  pick category (rotate by week)  ── existing settings.categories
  ▼
[1] ResearchAgent  (Gemini Flash + googleSearch)
        → ResearchBrief { topic, core_message, narrative_angle,
                          key_points[]{claim, source_url, confidence},
                          stats[] }
        → DedupAgent (Gemini Flash) guards against repeating recent topics
  ▼
[2] ScriptAgent  (Claude)
        → Script { hook, full_text, est_duration_sec, tone }
           structure: hook → problem → payoff → CTA, Thai, fits 60-90s
  ▼
[3] SceneAgent  (Claude)
        → Scene[] (6-10): { scene_number, beat, narration, on_screen_text,
                            layout_variant, image_prompt(draft), emphasis_words[],
                            caption_style, duration_sec(target), visual_type }
  ▼
[4] parallel:
     • ImagePromptAgent (Gemini)  → final gpt-image-2 prompt per image scene (brand-styled)
     • MetadataAgent   (Gemini)   → youtube_title/desc/tags
  ▼
[5] Producer / Assembler:
     a. Per-scene TTS (OpenRouter Gemini TTS) → ffprobe each → measured duration
        → scene start/end timeline (source of truth, overrides target duration)
     b. Concatenate scene audio → voice.wav (locked composition duration)
     c. gpt-image-2 per image scene (kie.ai async) → download local
     d. Build karaoke SEGMENTS from narration + measured timing (staggered word reveal)
     e. Fill hyperframes multi-scene HTML template (bundled GSAP + Sarabun font +
        brand CSS tokens) with SCENES / SEGMENTS / CARDS + scene images
     f. `npx hyperframes@<pinned> render` → output.mp4 (1080×1920)
     g. thumbnail (first scene frame or hero image)
     h. upload MP4 + thumbnail to kie.ai storage → store URLs
  ▼
Clip status → ready (Video916URL, ThumbnailURL, metadata)
```

---

## 4. Components

### 4.1 kie.ai LLM client — `internal/agent/kiellm.go` (new)

Replaces `internal/agent/llm.go` (OpenRouter) for all agents. kie.ai is **not** one uniform API; the client needs two request shapers + one shared auth.

- **Auth:** `Authorization: Bearer <kie_api_key>` (already in settings table), `Content-Type: application/json`.
- **Claude** — `POST https://api.kie.ai/claude/v1/messages`, Anthropic Messages format. Body: `{model:"claude-sonnet-4-6", messages:[...], max_tokens, stream:false}`. **Must set `stream:false`** (defaults true). Text at `content[]` blocks where `type=="text"`.
- **Gemini** — `POST https://api.kie.ai/gemini/v1/models/gemini-3-5-flash:streamGenerateContent`, Google format. Body: `{contents:[{role:"user",parts:[{text}]}], tools:[{googleSearch:{}}]?, generationConfig:{...}}`. **Streaming-only** — read and concatenate chunks; text at `candidates[].content.parts[].text`. Roles are `user`/`model` (not `assistant`).
- **Methods:** `GenerateClaude(ctx, system, user, temp) (string,error)`, `GenerateGemini(ctx, user, opts) (string,error)` (opts: enable googleSearch), plus `GenerateJSON*` wrappers reusing the existing `extractJSON` + retry-with-lower-temp logic from `llm.go`.
- Drive control flow off response fields, not the inconsistent top-level `code` (per kie.ai docs).

> System prompts: Claude takes a top-level `system`; Gemini has no system role — prepend system text to the first user `part` (or use `systemInstruction` if confirmed available; default to prepend).

### 4.2 Agents — `internal/agent/*`

Rewrite each agent to call `KieLLMClient` with its assigned model. Agent configs in DB (`agents` table, `agentsRepo.GetByName`) gain a model/provider field so the model per role stays DB-tunable (consistent with current pattern).

- **research.go** — Gemini + googleSearch. Input: category + persona + recent-topics (dedup). Output: `ResearchBrief` JSON. Keep the existing "never fabricate news / graceful no-op" guards; if no reliable info, skip the clip (no Q&A fallback — that concept is retired).
- **script.go** — Claude. Input: ResearchBrief + format + persona + target duration. Output: `Script`. Enforce conversational Thai, ≤15-word sentences, strong 3-second hook.
- **scene.go** (new, replaces template/image-prompt split) — Claude. Input: Script + target scene count/duration. Output: constrained `Scene[]` matching the hyperframes template's slots and `layout_variant` enum.
- **imageprompt.go** — Gemini. Input: scene + brand theme. Output: brand-styled gpt-image-2 prompt (dark navy gradient, orange accent, **no text in image**, upper-center negative space for overlay — per `hfslice` background prompt).
- **metadata.go** — Gemini. Reuse existing search-intent title logic + `validateScript` brand-suffix normalization (keep `orchestrator.go` `validateScript`).
- **dedup.go** — Gemini. Keep existing semantic dedup behavior.

### 4.3 Hyperframes render — `internal/producer/hyperframes.go` (new)

- Port the `hfslice` template approach: embed `layout_multi_scene.html.tmpl` (Go `html/template`) with brand CSS custom-property tokens, per-scene JS arrays (`SCENES`, `SEGMENTS`, `CARDS`), local `gsap.min.js`, local Sarabun Thai font.
- `layout_variant` enum: `hook_big | hook_punch | phrase_block | stat_reveal | quote_cta | word_pop | static | intro | outro`.
- `caption_style`: `phrase_block | word_pop`; karaoke key-word highlight in amber via `emphasis_words`.
- Seek-safety rules (mandatory, from `hfslice`): hard opacity-kill at scene end; single clip-path per transition; transform-only punch-in; auto-fit font shrink for overflow.
- Render: write `index.html` + assets into clip workdir, run `npx hyperframes@0.6.70 render` (pinned — the version `hfslice` used; re-verify latest stable before locking), capture `output.mp4`. Bundle everything local (CDN fails in sandboxed render — root cause of the prior scene-freeze bug).

### 4.4 Producer rewrite — `internal/producer/producer.go`

- **Per-scene TTS:** generate audio per scene via existing `OpenRouterClient.GenerateVoice`, `ffprobe` each for true duration, concatenate into `voice.wav`. Per-scene durations become the composition timeline (resolves the unmeasurable Thai-WPM problem). Keep brand-alias sanitize + URL/@handle stripping.
- **Per-scene images:** for scenes with `visual_type==image`, call `KieClient.GenerateImage` (`gpt-image-2-text-to-image`, 9:16, 2K). Download local.
- **Remove** `AssembleSingleImage` / `AssembleSingleImageVertical` static path and the single-`imagePrompts[0]` logic.
- Keep kie.ai upload (`UploadFile`) for final MP4 + thumbnail; store `Video916URL`, `ThumbnailURL`.
- Preserve resume-from-failure semantics (skip steps whose output files already exist).

### 4.5 Orchestrator — `internal/orchestrator/orchestrator.go`

- `ProduceWeekly`: rotate category by week (unchanged), then per clip run research → script → scene → (imageprompt ∥ metadata) → assemble.
- Drop questioner/Q&A-fallback branches. If research yields nothing reliable, skip that clip and log (never fabricate).
- Keep `validateScript` title normalization, tracker/progress steps (rename steps: research, script, scene, image_prompts, voice, images, render, upload).

---

## 5. Data Model / DB

Existing `clips` / `scenes` tables stay; extend them. New migration files (next number after `028` → `029_*`):

**`scenes`** add:
- `layout_variant TEXT` — hyperframes layout enum
- `on_screen_text TEXT` — kinetic-typography overlay (distinct from `voice_text`)
- `emphasis_words JSONB` — words to highlight
- `beat TEXT` — hook/problem/payoff/cta label
- `caption_style TEXT`

(`scene_type`, `image_prompt`, `voice_text`, `duration_seconds`, `Image916URL` reused. `Image169URL` becomes unused — leave column, stop writing.)

**`clips`** add:
- `core_message TEXT`
- `narrative_angle TEXT`
- `research_brief JSONB` (full brief for audit/regeneration)

Use the **safe-migration** skill when authoring these (additive columns, nullable — no destructive changes).

---

## 6. Deployment — Dockerfile

Single image (Approach A): Go binary + Node 22 + headless Chrome + FFmpeg + pinned `hyperframes@0.6.70`. Bundle `gsap.min.js` + Sarabun font as repo assets (no network at render). Pin the hyperframes version (pre-1.0, moves fast). Railway service config: ensure enough memory for headless Chrome (60 s × 30 fps ≈ 1800 screenshots).

---

## 7. Config / Settings

- Keep `kie_api_key` (already used by `KieClient`) — now also powers all LLM agents.
- Keep `openrouter_api_key` — now used **only** for TTS.
- Add per-agent model fields to `agents` table (DB-tunable model per role).
- Add a setting for pinned hyperframes version (optional; can be compile-time const).

---

## 8. Build Sequence (phases for the implementation plan)

1. **kie.ai LLM client** (`kiellm.go`) + tests (Claude + Gemini shapers, JSON wrappers). Foundation.
2. **DB migrations** (029) — additive scene/clip columns; agents model field.
3. **Agents** rewrite to KieLLM: research (Gemini+search) → script (Claude) → scene (Claude) → imageprompt/metadata/dedup (Gemini). TDD per agent (JSON schema validation).
4. **Hyperframes templater + render** (`hyperframes.go`) — port `hfslice` template, bundle assets, `npx render`, render a fixture clip end-to-end.
5. **Producer rewrite** — per-scene TTS + ffprobe timing + per-scene gpt-image-2 + render + upload; remove static path.
6. **Orchestrator** — topic-driven flow; remove Q&A fallback; wire tracker steps.
7. **Dockerfile** — Node+Chrome+FFmpeg; deploy & smoke-test one full video on Railway.
8. **Cleanup** — remove dead OpenRouter LLM path (`llm.go`) once all agents migrated; run `/simplify` on the diff before commit (per user preference).

---

## 9. Risks & Mitigations

- **Gemini streaming-only endpoint** — must aggregate chunks; budget client effort. Mitigation: thorough client test with a real call.
- **Render weight on Railway** — memory/time for headless Chrome. Mitigation: 9:16-only (one render), monitor; Approach B is the escape hatch.
- **Caption timing accuracy** — per-scene measured durations are accurate at scene granularity; intra-scene word reveal is approximate (staggered), matching `hfslice`. Acceptable.
- **kie.ai cost/rate limits** — global 20 req / 10 s; gpt-image-2 async per scene multiplies calls (6-10 images/clip). Mitigation: respect existing retry/backoff in `kieai.go`; consider concurrency cap.
- **Pinned hyperframes drift** — pre-1.0. Mitigation: pin version, bundle assets, lint before render.
- **No web-search on Claude** — research must stay on Gemini. Locked in assignment.

---

## 10. Success Criteria

1. One end-to-end run produces a 9:16, 60–90 s, 6–10 scene MP4 with animated captions synced to per-scene narration, brand navy/orange + mascot, no scene-freeze.
2. All LLM agent calls go through kie.ai (Claude or Gemini per table); only TTS uses OpenRouter.
3. Images in scenes are gpt-image-2 (kie.ai).
4. Research produces sourced facts; no fabricated news; topics don't repeat recent ones (dedup).
5. Resume-from-failure still works (re-run skips completed steps).
6. Deploys as a single Railway service.
