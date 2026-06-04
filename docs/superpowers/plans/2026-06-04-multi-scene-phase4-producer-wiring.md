# Multi-Scene Phase 4 — Producer Wiring Implementation Plan

> **For agentic workers:** subagent-driven-development. The end-to-end gate is a REAL render (lint+inspect+render for both ratios) — costs API ($ for per-scene GPT-image ×2 + TTS). Steps use `- [ ]`.

**Goal:** ต่อทุกชิ้นของ Phase 1-3 เข้า producer จริง — multi-scene end-to-end: script ฉาก → TTS ต่อฉาก concat → DecideScenes → AI bg ต่อฉาก×2 ratio → BuildScenes → lint+inspect+render ทั้ง 9:16 + 16:9 → upload. 16:9 เลิก FFmpeg static มาเป็น Hyperframes. persist bg_hint.

**Architecture:** เพิ่ม `assembleMultiScene` ใน producer (แทน assembleHyperframes916 เมื่อ multi-scene เปิด), builder `BuildScenes` (ใช้ RenderCompositionScenes), per-scene TTS helper (concat → boundaries). EnableHyperframes โหลด config `composition_scenes`. คง path เดิม (Decide/assembleHyperframes916/FFmpeg) เป็น fallback. flag คุม: ใช้ env เดิม `HYPERFRAMES_ENABLED` + เพิ่ม `HYPERFRAMES_MULTI_SCENE` (default off) เพื่อสลับทีละขั้นบน production

**Tech:** Go, OpenRouter (GenerateVoice text,voice,out / GenerateImage prompt,aspect,out), Hyperframes CLI, pgx

**Reference:** internal/producer/producer.go (Produce, assembleHyperframes916, getVoice), composition_builder.go (Build, project constants, copyFile/copyDir), composition.go (RenderCompositionScenes, ScenesParams/SceneSpec/SlotSpec), agent/composition.go (DecideScenes, ScenesDecision/SceneDesign/Slot), openrouter.go (GenerateVoice:148, GenerateImage:88), hyperframes.go (Lint/Inspect/Render), orchestrator.go (per-scene voice loop, scene save), composition_builder.go highlightTitle (emphasis→HTML)

---

### Task 1: scenes.bg_hint column + repo + persist (closes Phase-1 TODO)

**Files:** Create `migrations/024_scene_bg_hint.sql`; Modify `internal/repository/scenes.go`, `internal/models` scene structs, `internal/orchestrator/orchestrator.go` (scene save + scenesToGenerated)

- [ ] Step 1: migration `ALTER TABLE scenes ADD COLUMN IF NOT EXISTS bg_hint TEXT NOT NULL DEFAULT '';`
- [ ] Step 2: add `BgHint` to models.Scene + CreateSceneRequest; scenesRepo.Create INSERT includes bg_hint; ListByClip SELECT includes it
- [ ] Step 3: orchestrator scene-save passes `scene.BgHint`; `scenesToGenerated` restores `BgHint` (remove the 2 TODO(phase-4) comments)
- [ ] Step 4: `go build ./... && go test ./internal/repository/ ./internal/orchestrator/` green
- [ ] Step 5: commit `feat(scenes): persist bg_hint`

### Task 2: per-scene TTS → concat + boundaries

**Files:** Modify `internal/producer/producer.go` (or new `multiscene.go`); Test where pure
- [ ] Step 1: helper `synthScenesVoice(ctx, scenes []agent.GeneratedScene, voice, clipDir) (voicePath string, bounds []sceneBound, err error)` where `sceneBound{Start,End float64}`. For each scene: GenerateVoice(scene.VoiceText) → temp wav; measure duration (reuse existing WAV duration logic / ffprobe used elsewhere); concat all into voice.wav (reuse the PCM/WAV concat already in openrouter.go's chunk join — extract/share it). bounds[i] = running offset.
- [ ] Step 2: unit-test the boundary math with stubbed per-scene durations (pure function `computeBounds([]float64) []sceneBound`)
- [ ] Step 3: build+test green; commit `feat(producer): per-scene TTS concat with scene boundaries`

### Task 3: BuildScenes builder

**Files:** Modify `internal/producer/composition_builder.go`
- [ ] Step 1: `func (b *CompositionBuilder) BuildScenes(params ScenesParams, clipID, projectDir, voicePath string, bgPaths map[int]string) (string, error)` — mkdir assets+fonts; copy voice→assets/voice.wav; for each scene with BackgroundMode=="image" copy bgPaths[sceneNumber]→assets/bg-sceneN.png and set SceneSpec.BackgroundImage; copyDir fonts; `RenderCompositionScenes(params)`→index.html; write package.json/hyperframes.json/meta.json (reuse existing constants)
- [ ] Step 2: unit-test: BuildScenes writes index.html + project files into a temp dir for a CSS-bg sample (no images), assert files exist + index.html non-empty + contains a scene headline
- [ ] Step 3: build+test green; commit `feat(producer): BuildScenes multi-scene project builder`

### Task 4: assembleMultiScene + per-ratio render

**Files:** Modify `internal/producer/producer.go`
- [ ] Step 1: `func (p *Producer) assembleMultiScene(ctx, clipID, clipDir string, scenes []agent.GeneratedScene, bounds []sceneBound, voicePath string, aspect string, outPath string) error`:
  - load category/questioner (as today)
  - transcribe voicePath → segments (best-effort, as today)
  - DecideScenes(composition_scenes cfg, scenes JSON) → ScenesDecision (already Normalized)
  - for each scene: gen AI bg via GenerateImage(sceneDesign.BgArtPrompt, aspect, bg-sceneN-<ratio>.png) (parallel errgroup; on fail → that scene BackgroundMode="css")
  - map → ScenesParams: per scene build []SlotSpec from sceneDesign.Slots (apply emphasis via highlightTitle-style helper → template.HTML), StartSec/EndSec from bounds, AccentColor, AnimationSpeed, LayoutVariant, BackgroundMode/Image; ScenesParams.Segments=segments, AspectRatio=aspect, brand/kicker/etc
  - BuildScenes → Lint → **Inspect (gate)** → Render → rename to outPath
  - record composition_style (per-scene summary)
- [ ] Step 2: wire into Produce: when multi-scene on, render BOTH `video-9x16.mp4` (aspect 9:16) and `video-16x9.mp4` (aspect 16:9) via assembleMultiScene; on any failure fall back to existing single-scene/FFmpeg path. EnableHyperframes loads composition_scenes config (add field). Add `HYPERFRAMES_MULTI_SCENE` config flag (config.go) gating the new path.
- [ ] Step 3: build+test green (unit-test the SlotSpec/ScenesParams mapping pure helper: sceneDesign+bounds → []SceneSpec). commit `feat(producer): multi-scene assembly for 9:16 + 16:9`

### Task 5: end-to-end render verify (REAL — costs API)

- [ ] Step 1: extend `cmd/hfslice` (or a guarded harness) to run the full assembleMultiScene against a real clip (needs DB + keys) OR run produce on a test clip on Railway behind `HYPERFRAMES_MULTI_SCENE=true`
- [ ] Step 2: confirm logs show multi-scene render (not fallback), inspect clean, both MP4s produced; download + eyeball both ratios
- [ ] Step 3: this is the cost/deploy checkpoint — coordinate with user before running on production

---

## Self-Review
- Spec §2 (per-scene TTS boundaries) → Task 2. §4 (DecideScenes wired) → Task 4. §5 (2 ratios via HF, 16:9 drops FFmpeg, per-scene bg, inspect gate, ≤60s already from script) → Task 4. bg_hint persist → Task 1.
- Pure/testable parts isolated (computeBounds, slot mapping, BuildScenes file-writing) for unit tests; LLM/render integration verified by Task 5 real render.
- Fallback preserved (single-scene/FFmpeg) — multi-scene behind new flag, staged rollout safe.
- Risk: Task 4 is the heavy integration; Task 5 costs API + is the deploy decision (checkpoint with user).
