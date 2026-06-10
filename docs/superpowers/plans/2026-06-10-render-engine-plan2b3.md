# Hyperframes Render Engine (Plan 2b-3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the proven multi-scene Hyperframes render engine onto `master` as a **standalone, tested, unwired** Go layer in `internal/producer/` — given a `ScenesParams` (hand-built `SceneSpec[]`) it fills the multi-scene HTML template and shells out to `npx hyperframes render` to produce a 9:16 MP4. Nothing on the existing production path changes, so master stays green.

**Architecture:** This is a **faithful port** of the already-written, already-tested render engine on the rolled-back branch `redesign/hyperframes-video-engine` (the work behind the [[project_hyperframes_rollback]] tag `backup/pre-rollback-hyperframes`). The engine has **zero dependency on the `internal/agent` package** — it takes `ScenesParams` value structs and emits an MP4. We port only the **multi-scene** path (`RenderCompositionScenes` / `BuildScenes`), dropping the obsolete single-scene "dynamic_karaoke" path. The `GeneratedScene → ScenesParams` mapping, per-scene TTS timing, and per-scene image generation are deliberately deferred to **Plan 2b-4 (producer rewrite)**, which is what consumes this engine. The GSAP runtime is bundled as a local asset (the fix for the [[project_video_redesign]] scene-freeze bug: CDN unreachable in sandboxed render).

**Tech Stack:** Go 1.25 (module `github.com/jaochai/video-fb`), `html/template` (multi-scene template fill), `//go:embed` (template + GSAP), `os/exec` → `npx hyperframes@0.6.70` (headless Chrome → MP4), Sarabun Thai fonts (local `@font-face`).

**Source of truth for the port:** every file/asset below is extracted verbatim (or verbatim-minus-named-symbols) from the git ref `redesign/hyperframes-video-engine`. Use `git show <ref>:<path>` to extract; do **not** retype large assets by hand.

---

## Scope

**In scope (this plan):** the pure render engine + its unit tests + a render harness gated behind `HF_RENDER=1`. Files created under `internal/producer/`:

- `brand.go` + `brand_test.go` — brand color/motion/type tokens + image-prompt helpers (single source of truth, referenced by template CSS vars).
- `mascot.go` + `mascot_test.go` — mascot pose/cue helpers (referenced by scene bumper asset paths).
- `composition_types.go` — `SceneSpec`, `SlotSpec`, `ScenesParams`, `TranscriptSegment` (multi-scene types only).
- `composition.go` — `RenderCompositionScenes` + `scenesTemplateData` + the `//go:embed` blocks.
- `composition_builder.go` — `BuildScenes` + project-assembly + helper funcs (`highlightTitle`, `sanitizeHexColor`, …).
- `hyperframes.go` — `HyperframesRenderer` (lint / inspect / render driver).
- `composition_scenes_test.go` — pure template-render unit tests (no Chrome).
- `composition_builder_test.go` — pure project-builder unit test (dummy fonts, no Chrome).
- `composition_scenes_render_test.go` — real `npx hyperframes` lint+inspect harness, **skipped unless `HF_RENDER=1`**.
- `templates/layout_multi_scene.html.tmpl` — the 9:16/16:9 multi-scene template (verbatim asset).
- `templates/gsap-3.14.2.min.js` — vendored GSAP runtime (verbatim asset).
- `assets/fonts/Sarabun-{Regular,SemiBold,Bold,ExtraBold}.ttf` — committed Thai fonts.

**Explicitly OUT of scope (deferred):**
- **Plan 2b-4 (producer):** `captions.go` (`captionSegmentsFromScenes`), `multiscene.go` (`synthScenesVoice`, `computeBounds`, `buildSceneSpecs`), the `GeneratedScene → ScenesParams` adapter, per-scene TTS+ffprobe timing, per-scene gpt-image-2, and rewiring `producer.go` `Produce`. These need `p.openRouter` + the agent layer.
- **Plan 2b-5 (orchestrator go-live):** topic-driven flow, `image`→`imageprompt` rename, `question` removal, frontend `TEMPLATE_VARS`/step labels, `main.go` wiring.
- **Plan 2b-6:** Dockerfile (Node 22 + Chrome + FFmpeg + pinned hyperframes) + Railway deploy.
- **The single-scene "dynamic_karaoke" path** (`RenderComposition`, `CompositionParams`, `CardSpec`, `Build`, `layout_dynamic_karaoke.html.tmpl`) — obsolete; **do not port it.**

**Why this stays green:** every file is **new**. No existing `internal/producer/*.go` file is edited, no `main.go` change, no orchestrator change, no migration. The current static-image `Produce` path is untouched and keeps running. `go build ./...` and `go test ./...` pass because the new code is additive and self-contained.

---

## File Structure

```
internal/producer/
  brand.go                         NEW  — Brand/Motion/Type tokens, CSSVars(), image-prompt helpers
  brand_test.go                    NEW
  mascot.go                        NEW  — mascot pose/cue helpers
  mascot_test.go                   NEW
  composition_types.go             NEW  — SceneSpec, SlotSpec, ScenesParams, TranscriptSegment
  composition.go                   NEW  — RenderCompositionScenes, scenesTemplateData, embeds
  composition_builder.go           NEW  — BuildScenes + helpers (highlightTitle, sanitizeHexColor, …)
  hyperframes.go                   NEW  — HyperframesRenderer (lint/inspect/render)
  composition_scenes_test.go       NEW  — pure render unit tests
  composition_builder_test.go      NEW  — pure builder unit test
  composition_scenes_render_test.go NEW — HF_RENDER=1 lint+inspect harness
  templates/
    layout_multi_scene.html.tmpl   NEW  — verbatim asset (~40 KB)
    gsap-3.14.2.min.js             NEW  — verbatim asset (~71 KB)
  assets/fonts/
    Sarabun-Regular.ttf            NEW  — committed font
    Sarabun-SemiBold.ttf           NEW
    Sarabun-Bold.ttf               NEW
    Sarabun-ExtraBold.ttf          NEW
```

**Dependency check (why no `agent` import):** `brand.go`, `mascot.go`, `composition_types.go`, `composition.go`, `composition_builder.go`, `hyperframes.go` import only the stdlib (`bytes`, `embed`, `encoding/json`, `fmt`, `html`, `html/template`, `io`, `log`, `os`, `os/exec`, `path/filepath`, `strings`, `time`). On the source branch, only `captions.go` and `multiscene.go` import `internal/agent` — and those are out of scope (2b-4). Confirm with a grep step in Task 7.

---

## Conventions for this plan

- `REF` = `redesign/hyperframes-video-engine` (the source git ref). It is an existing local branch (verify: `git branch --list redesign/hyperframes-video-engine`).
- "Extract verbatim" = `git show REF:<path> > <dest>` then make only the named deletions.
- After each task, the listed `go test` must pass. The render-harness test (Task 6) is skipped without `HF_RENDER=1`, so it never blocks `go test ./...`.
- Run every `go build`/`go test` with the sandbox disabled if you hit a Go build-cache permission error (known sandbox quirk in this repo).

---

## Task 1: Vendor the static assets (template, GSAP, fonts)

**Files:**
- Create: `internal/producer/templates/layout_multi_scene.html.tmpl`
- Create: `internal/producer/templates/gsap-3.14.2.min.js`
- Create: `internal/producer/assets/fonts/Sarabun-{Regular,SemiBold,Bold,ExtraBold}.ttf`

These are binary/large verbatim artifacts — extract, never retype.

- [ ] **Step 1: Create dirs and extract the template + GSAP from the source branch**

```bash
cd /Users/jaochai/Code/video-fb
mkdir -p internal/producer/templates internal/producer/assets/fonts
git show redesign/hyperframes-video-engine:internal/producer/templates/layout_multi_scene.html.tmpl > internal/producer/templates/layout_multi_scene.html.tmpl
git show redesign/hyperframes-video-engine:internal/producer/templates/gsap-3.14.2.min.js > internal/producer/templates/gsap-3.14.2.min.js
```

- [ ] **Step 2: Extract the four Sarabun fonts (committed on the branch under `hyperframes-poc/assets/fonts/`)**

```bash
for f in Regular SemiBold Bold ExtraBold; do
  git show redesign/hyperframes-video-engine:hyperframes-poc/assets/fonts/Sarabun-$f.ttf > internal/producer/assets/fonts/Sarabun-$f.ttf
done
```

- [ ] **Step 3: Verify the assets landed intact**

```bash
ls -l internal/producer/templates/ internal/producer/assets/fonts/
# Expect (approx): layout_multi_scene.html.tmpl ≈ 40 KB, gsap-3.14.2.min.js ≈ 71 KB,
# each Sarabun-*.ttf a non-trivial binary (tens–hundreds of KB).
grep -c 'assets/gsap.min.js'  internal/producer/templates/layout_multi_scene.html.tmpl   # expect 1
grep -c 'const SCENES'        internal/producer/templates/layout_multi_scene.html.tmpl   # expect ≥1
grep -c 'const SEGMENTS'      internal/producer/templates/layout_multi_scene.html.tmpl   # expect ≥1
grep -c '{{ .BrandCSS }}'     internal/producer/templates/layout_multi_scene.html.tmpl   # expect 1
file internal/producer/assets/fonts/Sarabun-Regular.ttf                                  # expect "TrueType"/"OpenType" font data
```
Expected: template contains the local `assets/gsap.min.js` script ref, the `const SCENES`/`const SEGMENTS` JS injection points, and the `{{ .BrandCSS }}` slot; fonts are real TTF binaries.

- [ ] **Step 4: Commit the assets**

```bash
git add internal/producer/templates/ internal/producer/assets/fonts/
git commit -m "feat(producer): vendor multi-scene hyperframes template + local GSAP + Sarabun fonts"
```

---

## Task 2: Brand tokens (`brand.go`)

The single source of truth for the ADS VANCE palette/motion/type. Its `CSSVars()` output is injected into the template's `{{ .BrandCSS }}` slot, and `Brand.Orange` is the accent-color fallback in `RenderCompositionScenes`. Self-contained (imports only `fmt`, `strings`); port verbatim.

**Files:**
- Create: `internal/producer/brand.go`
- Test: `internal/producer/brand_test.go`

- [ ] **Step 1: Extract `brand.go` and `brand_test.go` verbatim**

```bash
git show redesign/hyperframes-video-engine:internal/producer/brand.go      > internal/producer/brand.go
git show redesign/hyperframes-video-engine:internal/producer/brand_test.go > internal/producer/brand_test.go
```

`brand.go` defines: `BrandName`, `BrandCTA` consts; `BrandColors` + `Brand` (navy scale, orange family, ink/muted, semantic warn/win/info); `SafeZoneSpec` + `SafeZone()`; `ImageStyleAnchor()`; `MotionTokens` + `Motion`; `TypeTokens` + `Type`; `CSSVars()` (emits the `:root{…}` block whose var names match the template); `formatDur()`; `buildScenePrompt()` + `genericSceneSubject`. (The image-prompt helpers `SafeZone`/`ImageStyleAnchor`/`buildScenePrompt` are exercised by `brand_test.go` here and consumed by Plan 2b-4's image step — keep them.)

- [ ] **Step 2: Run the brand tests to verify they pass**

```bash
go test ./internal/producer/ -run 'TestBrandColors|TestImageStyleAnchor|TestMotionTokens|TestTypeTokens|TestBuildScenePrompt' -v
```
Expected: PASS. (`TestBrandColors` asserts the exact hexes — Navy `#0047AF`, Orange `#F0A030`, Win `#2fd17a`, etc.; `TestBuildScenePrompt` asserts the 3-block deterministic image prompt.)

- [ ] **Step 3: Commit**

```bash
git add internal/producer/brand.go internal/producer/brand_test.go
git commit -m "feat(producer): add ADS VANCE brand tokens (color/motion/type) + image-prompt helpers"
```

---

## Task 3: Mascot helpers (`mascot.go`)

Small, dependency-free helpers (no imports) mapping mascot cues → baked-PNG pose paths, plus the gpt-image-2 edit prompt used by the offline mascot-gen step. Referenced by scene bumper/pose asset paths.

**Files:**
- Create: `internal/producer/mascot.go`
- Test: `internal/producer/mascot_test.go`

- [ ] **Step 1: Extract `mascot.go` and `mascot_test.go` verbatim**

```bash
git show redesign/hyperframes-video-engine:internal/producer/mascot.go      > internal/producer/mascot.go
git show redesign/hyperframes-video-engine:internal/producer/mascot_test.go > internal/producer/mascot_test.go
```

`mascot.go` defines: `MascotAssetDir` const, `MascotAssetPath()`, `mascotPoses` + `MascotPoseNames()`, `poseDirective` map, `MascotEditPrompt()`, `MascotCueToPose()`.

- [ ] **Step 2: Run the mascot tests**

```bash
go test ./internal/producer/ -run 'TestMascotPoses|TestMascotEditPrompt|TestMascotCueToPose' -v
```
Expected: PASS (3 tests).

- [ ] **Step 3: Commit**

```bash
git add internal/producer/mascot.go internal/producer/mascot_test.go
git commit -m "feat(producer): add mascot pose/cue helpers"
```

---

## Task 4: Multi-scene types + template renderer (`composition_types.go`, `composition.go`)

Port the **multi-scene types** and the **template renderer**, dropping the obsolete single-scene "karaoke" types/functions. `RenderCompositionScenes(ScenesParams) → []byte` is the pure core: it sanitizes each scene's accent color, marshals a lightweight scene-timing JSON + the caption segments JSON, injects `Brand.CSSVars()`, and executes `layout_multi_scene.html.tmpl`.

**Files:**
- Create: `internal/producer/composition_types.go`
- Create: `internal/producer/composition.go`
- Test: `internal/producer/composition_scenes_test.go`

- [ ] **Step 1: Extract `composition_types.go`, then delete the two karaoke-only types**

```bash
git show redesign/hyperframes-video-engine:internal/producer/composition_types.go > internal/producer/composition_types.go
```

Then **delete** the `CompositionParams` struct and the `CardSpec` struct (single-scene karaoke only). **Keep** `TranscriptSegment`, `SceneSpec`, `SlotSpec`, `ScenesParams`. The file must still `import "html/template"` (used by `SlotSpec.HTML template.HTML`).

After editing, the file's type set must be exactly:
- `TranscriptSegment{ Text string; Start, End float64 }` (json `text`/`start`/`end`)
- `SceneSpec{ SceneNumber int; LayoutVariant, AccentColor, AnimationSpeed string; StartSec, EndSec float64; BackgroundMode, BackgroundImage, MascotPose, CaptionStyle string; Slots []SlotSpec }`
- `SlotSpec{ Role string; HTML template.HTML; StepNum int }`
- `ScenesParams{ AspectRatio, BrandName, CategoryLabel, QuestionerName, Kicker, VoiceSrc string; DurationSeconds float64; IntroMascot, OutroMascot, CTAText string; Scenes []SceneSpec; Segments []TranscriptSegment }`

Verify the karaoke types are gone:
```bash
grep -cE 'type CompositionParams|type CardSpec' internal/producer/composition_types.go   # expect 0
```

- [ ] **Step 2: Extract `composition.go`, then delete the karaoke renderer**

```bash
git show redesign/hyperframes-video-engine:internal/producer/composition.go > internal/producer/composition.go
```

Then **delete** the entire `func RenderComposition(p CompositionParams) ([]byte, error)` (the single-scene karaoke renderer — it references the now-deleted `CompositionParams`/`CardSpec`/`templateData`/`outroLeadSeconds`). **Keep**: the `scenesTemplateData` struct, the `//go:embed templates/*.html.tmpl` (`templateFS`) and `//go:embed templates/gsap-3.14.2.min.js` (`gsapMinJS`) blocks, and `func RenderCompositionScenes(p ScenesParams) ([]byte, error)`.

`RenderCompositionScenes` behavior to preserve exactly:
- Errors if `DurationSeconds <= 0` or `len(Scenes) == 0`.
- `width,height = 1080,1920` (or `1920,1080` when `AspectRatio == "16:9"`).
- Copies `Scenes` before mutating; sanitizes each `AccentColor` via `sanitizeHexColor(_, Brand.Orange)`.
- Marshals a `[]sceneTiming{scene,start,end,speed,variant,caption_style}` → `ScenesJSON`, and `p.Segments` → `SegmentsJSON`.
- `outroStart = DurationSeconds - 1.6` (floored at 0).
- Injects `BrandCSS: template.CSS(Brand.CSSVars())`.
- Registers template funcs `durSec(start,end)` (min 0.1) and `addInt(a,b)`, then executes `layout_multi_scene.html.tmpl`.

Verify the karaoke renderer is gone and the imports still compile (`bytes`, `embed`, `encoding/json`, `fmt`, `html/template`):
```bash
grep -c 'func RenderComposition(' internal/producer/composition.go        # expect 0
grep -c 'func RenderCompositionScenes(' internal/producer/composition.go  # expect 1
```

> Note: `RenderCompositionScenes` calls `sanitizeHexColor` and `animationSpeed`, which live in `composition_builder.go` (Task 5). The package will **not compile** until Task 5 is added — that's expected; run the Task 4 test only after Task 5's file exists. (If you prefer a compile checkpoint here, do Step 3 after Task 5 Step 1.)

- [ ] **Step 3: Add the pure render unit tests and run them**

Extract the pure test file verbatim:
```bash
git show redesign/hyperframes-video-engine:internal/producer/composition_scenes_test.go > internal/producer/composition_scenes_test.go
```

It defines `sampleScenesParams(aspect)` (3 scenes: `hook_big`/`list_steps`/`quote_cta`, with intro/outro mascot + CTA) and asserts, via `RenderCompositionScenes`:
- `TestRenderCompositionScenes_9x16` — output contains `data-width="1080"`, `data-height="1920"`, all three Thai headlines, `const SEGMENTS`; no unrendered `{{`/`}}`; no nested `<span class="hl"><span`.
- `TestRenderCompositionScenes_16x9` — `data-width="1920"`, `data-height="1080"`.
- `TestRenderCompositionScenes_Bumper` — `id="introBumper"`, `assets/mascot/rocket.png`, `id="outroBumper"`, `assets/mascot/wave.png`, the CTA `กดติดตาม`, `assets/mascot/thumbs_up.png`.
- `TestRenderCompositionScenes_NoLegacyLiterals` — none of the old hardcoded navy/orange literals (`#0a1428`, `#0f1d35`, `#16284a`, `#ff6b2b`, `rgba(15,29,53…)`, …) survive — proves the template uses `{{ .BrandCSS }}` from `Brand`.
- `TestRenderCompositionScenes_RejectsEmpty` — errors on `DurationSeconds<=0` and on empty `Scenes`.
- `TestRenderCompositionScenes_CaptionStyleInJSON` — `ScenesJSON` carries `"caption_style":"word_pop"` and `"caption_style":"phrase_block"`.

Run (after Task 5's `composition_builder.go` exists):
```bash
go test ./internal/producer/ -run 'TestRenderCompositionScenes' -v
```
Expected: PASS (6 tests).

- [ ] **Step 4: Commit** (commit together with Task 5 if you sequenced the compile checkpoint there; otherwise:)

```bash
git add internal/producer/composition_types.go internal/producer/composition.go internal/producer/composition_scenes_test.go
git commit -m "feat(producer): multi-scene composition types + RenderCompositionScenes"
```

---

## Task 5: Project builder + shared helpers (`composition_builder.go`)

Port `BuildScenes` (assembles a renderable Hyperframes project dir) plus the shared helper funcs that `composition.go` and the builder depend on. Drop the karaoke `Build`/`templateData`/`outroLeadSeconds`.

**Files:**
- Create: `internal/producer/composition_builder.go`
- Test: `internal/producer/composition_builder_test.go`

- [ ] **Step 1: Extract `composition_builder.go`, then delete the three karaoke-only symbols**

```bash
git show redesign/hyperframes-video-engine:internal/producer/composition_builder.go > internal/producer/composition_builder.go
```

Then **delete**:
- `const outroLeadSeconds = 4.0` (used only by the karaoke `RenderComposition`),
- the `templateData` struct (used only by `RenderComposition`),
- `func (b *CompositionBuilder) Build(params CompositionParams, …)` (the karaoke single-scene builder — references deleted `CompositionParams`).

**Keep** everything else: `projectPackageJSON`, `projectHyperframesJSON` consts; `CompositionBuilder` + `NewCompositionBuilder(fontsDir)`; `func (b *CompositionBuilder) BuildScenes(params ScenesParams, clipID, projectDir, voicePath string, bgPaths map[int]string)`; and helpers `highlightTitle`, `containedInLonger`, `outroBrandHTML`, `sanitizeHexColor`, `backgroundMode`, `animationSpeed`, `writeGsapAsset`, `copyFile`, `copyDir`. (`highlightTitle`/`containedInLonger`/`outroBrandHTML` are unused inside 2b-3 but are the slot-emphasis helpers Plan 2b-4's `buildSceneSpecs` calls — keep them; unused package funcs are legal Go.)

`BuildScenes` behavior to preserve exactly:
- `mkdir projectDir/assets/fonts`; copy `voicePath` → `assets/voice.wav`; set `params.VoiceSrc = "assets/voice.wav"`.
- Copy `Scenes` (don't mutate caller); for each `BackgroundMode=="image"`, copy `bgPaths[SceneNumber]` → `assets/bg-scene<N>.png` (graceful downgrade to `"css"` if missing/copy fails).
- `copyDir(fontsDir, assets/fonts)`.
- Copy referenced mascot PNGs (`IntroMascot`, `OutroMascot`, per-scene `MascotPose`) from `<fontsDir>/../mascot/` into `assets/mascot/` (missing source = non-fatal warning).
- `writeGsapAsset(assetsDir)` → writes embedded `gsapMinJS` to `assets/gsap.min.js`.
- `RenderCompositionScenes(params)` → write `index.html`.
- Write `package.json`, `hyperframes.json`, `meta.json` (`{"id":clipID,"name":clipID}`).

Verify the karaoke builder is gone:
```bash
grep -cE 'func \(b \*CompositionBuilder\) Build\(|type templateData|outroLeadSeconds' internal/producer/composition_builder.go   # expect 0
grep -c 'func (b *CompositionBuilder) BuildScenes(' internal/producer/composition_builder.go                                   # expect 1
```

- [ ] **Step 2: Confirm the package now compiles**

```bash
go build ./internal/producer/
```
Expected: builds clean. (Tasks 4 + 5 together provide all symbols: `RenderCompositionScenes` ↔ `sanitizeHexColor`/`animationSpeed`/`Brand`.)

- [ ] **Step 3: Add the pure builder test and run it**

```bash
git show redesign/hyperframes-video-engine:internal/producer/composition_builder_test.go > internal/producer/composition_builder_test.go
```

`TestBuildScenes` creates a temp `fontsDir` (with a dummy `.ttf`), a temp `voice.wav`, and a 2-scene `ScenesParams`, then calls `BuildScenes(...,"clip-abc-123",...,map[int]string{})` and asserts: returned dir == projectDir; `index.html`, `package.json`, `hyperframes.json`, `meta.json`, `assets/voice.wav` all exist; `index.html` is non-empty and contains the scene-1 headline `บัญชีโดนแบน`; `meta.json` `id == "clip-abc-123"`; the caller's `params.VoiceSrc`/`params.Scenes` were **not** mutated. (Uses a dummy font, so it needs no committed fonts and runs in CI without Chrome.)

```bash
go test ./internal/producer/ -run 'TestBuildScenes' -v
```
Expected: PASS.

- [ ] **Step 4: Run the full producer package test (pure tests only — no Chrome yet)**

```bash
go test ./internal/producer/ -v
```
Expected: PASS. The existing `openrouter_test.go` plus all new pure tests pass; the `HF_RENDER` harness (Task 6) is added next and self-skips.

- [ ] **Step 5: Commit**

```bash
git add internal/producer/composition_builder.go internal/producer/composition_builder_test.go
# If you deferred Task 4's commit to here, add those files too:
git add internal/producer/composition_types.go internal/producer/composition.go internal/producer/composition_scenes_test.go
git commit -m "feat(producer): multi-scene project builder (BuildScenes) + render-engine helpers"
```

---

## Task 6: Render driver + lint/inspect harness (`hyperframes.go`)

Port the CLI driver that shells out to Hyperframes, plus the heavy end-to-end harness (gated so normal `go test` stays green without Node/Chrome).

**Files:**
- Create: `internal/producer/hyperframes.go`
- Test: `internal/producer/composition_scenes_render_test.go`

- [ ] **Step 1: Extract `hyperframes.go` verbatim**

```bash
git show redesign/hyperframes-video-engine:internal/producer/hyperframes.go > internal/producer/hyperframes.go
```

It defines: `const hyperframesVersion = "0.6.70"`; `HyperframesRenderer` + `NewHyperframesRenderer()` (10-min timeout); `run()` (runs the CLI, **scans output for silent in-page failures** — `[Browser:PAGEERROR]`, `Composition script failed`, `Failed to download CDN script`, `is not defined` — and logs them even on exit 0); `scanBrowserIssues()`; `hyperframesCmd()` (prefers a globally-installed `hyperframes`, falls back to `npx --yes hyperframes@0.6.70`); `Lint()`, `Inspect()`, `Render()` (`render --output <out> --quality standard --fps 24 -w 6` — tuned to fit Railway's ~8 GB without OOM); `lastBytes()`. Imports: `context`, `fmt`, `log`, `os/exec`, `strings`, `time`.

- [ ] **Step 2: Confirm it compiles**

```bash
go build ./internal/producer/
```
Expected: builds clean (no new deps; `os/exec` is stdlib).

- [ ] **Step 3: Extract the HF_RENDER-gated harness**

```bash
git show redesign/hyperframes-video-engine:internal/producer/composition_scenes_render_test.go > internal/producer/composition_scenes_render_test.go
```

It defines `richScenesParams(aspect)` (6 scenes exercising every layout variant — `hook_punch`, `hook_big`, `list_steps`, `stat_reveal`, `compare_two`, `quote_cta` — and every slot role incl. `stat`/`callout`, with realistic Thai) and `TestManualRenderMultiScene`, which **`t.Skip`s unless `HF_RENDER=1`**, then builds a real project (via `NewCompositionBuilder` + `BuildScenes`) and runs `hyperframes lint` + `inspect`.

**Adjust the harness's fonts source** to the committed fonts: it must construct `NewCompositionBuilder` with a `fontsDir` that exists in this repo. Point it at the committed `internal/producer/assets/fonts` (tests run with CWD = the package dir, so the relative path `assets/fonts` resolves). If the extracted file references the old PoC path or a `t.TempDir()` dummy-font dir, change that one line to:
```go
fontsDir := "assets/fonts"
```
(Real TTFs are required for a real render; the dummy-font trick is only valid for the pure builder test in Task 5.)

- [ ] **Step 4: Verify the harness self-skips under normal test**

```bash
go test ./internal/producer/ -run 'TestManualRenderMultiScene' -v
```
Expected: `--- SKIP: TestManualRenderMultiScene (set HF_RENDER=1 …)`. The default `go test` path must never require Node/Chrome.

- [ ] **Step 5: (Optional, local only) Run the real render harness if Node + Chrome are available**

```bash
cd internal/producer && HF_RENDER=1 go test ./ -run 'TestManualRenderMultiScene' -v ; cd -
```
Expected (only when Node ≥18 + Chrome present and network can reach npm): `hyperframes lint` + `inspect` pass with no `[Browser:PAGEERROR]`/CDN/`is not defined` issues logged. **Honesty note:** if Node/Chrome/network are unavailable (e.g. the sandbox), this is expected to fail or be un-runnable — that is acceptable for 2b-3; the binding end-to-end MP4 proof happens in Plan 2b-4/2b-6 on a machine/Docker image with the toolchain. Report the skip/failure honestly; do not claim a render succeeded that wasn't run.

- [ ] **Step 6: Commit**

```bash
git add internal/producer/hyperframes.go internal/producer/composition_scenes_render_test.go
git commit -m "feat(producer): hyperframes render driver (lint/inspect/render) + HF_RENDER harness"
```

---

## Task 7: Full verification (additive contract)

**Files:** none (verification only)

- [ ] **Step 1: Build, vet, and test the whole module**

```bash
go build ./... && go vet ./... && go test ./...
```
Expected: all PASS. New engine is additive package-level code; the `HF_RENDER` harness self-skips. (Sandbox go-build cache error → rerun with the sandbox disabled.)

- [ ] **Step 2: Confirm the render engine has no `internal/agent` dependency**

```bash
grep -rln 'internal/agent' internal/producer/*.go
```
Expected: **no output** — none of the new (or existing) producer files import the agent package in 2b-3. (The `GeneratedScene` adapter that *will* import `agent` is Plan 2b-4's `captions.go`/`multiscene.go`.)

- [ ] **Step 3: Confirm the additive contract held — only new files, nothing else touched**

```bash
git diff d9cdeae..HEAD --stat -- internal/ cmd/ frontend/ migrations/
```
(`d9cdeae` = the 2b-2 merge = HEAD before Task 1; adjust if a commit landed first.) Expected: only **new** files under `internal/producer/` appear (the 8 `.go` files, the 2 template assets, the 4 fonts, this plan doc). **No** edits to `producer.go`, `ffmpeg.go`, `kieai.go`, `openrouter.go`, `main.go`, any orchestrator/agent file, any migration, or any frontend file.

- [ ] **Step 4: Confirm the obsolete karaoke path was not ported**

```bash
grep -rcE 'RenderComposition\(|type CompositionParams|type CardSpec|func \(b \*CompositionBuilder\) Build\(|layout_dynamic_karaoke' internal/producer/*.go internal/producer/templates/* 2>/dev/null | grep -v ':0$' || echo "clean: no karaoke path"
```
Expected: `clean: no karaoke path`.

---

## Self-Review Notes

- **Spec coverage:** Implements design §4.3 (Hyperframes render — `hyperframes.go` + multi-scene template, bundled GSAP + Sarabun, seek-safety baked into the ported template, pinned `hyperframes@0.6.70`) and the render half of §6 (local-asset bundling so no CDN at render). The data-model/agent pieces (§4.2/§4.6), per-scene TTS + ffprobe timing + per-scene gpt-image-2 + producer rewrite (§4.4), orchestrator (§4.5), and frontend (§4.8) are explicitly deferred to Plans 2b-4/2b-5/2b-6. This increment is the standalone render engine the user approved building first.
- **Reuse over rewrite (per the user's decision):** every file is ported from the proven, already-tested `redesign/hyperframes-video-engine` branch — the rolled-back work recoverable via tag `backup/pre-rollback-hyperframes` ([[project_hyperframes_rollback]]). The scene-freeze root cause ([[project_video_redesign]] — GSAP via CDN) is already fixed there by the local `gsap-3.14.2.min.js` asset; `TestRenderCompositionScenes_NoLegacyLiterals` + the `scanBrowserIssues` driver guard against regressions.
- **YAGNI / Surgical:** only the multi-scene path is ported; the obsolete single-scene karaoke types/funcs/template are dropped (Task 4/5 deletions + Task 7 Step 4 guard). No existing file is edited — master's current static-image `Produce` keeps working untouched.
- **Green at every task:** the only inter-file compile coupling is `composition.go` ↔ `composition_builder.go` (Task 4 ↔ Task 5); the plan flags running Task 4's test after Task 5's file exists, and Task 5 Step 2 is the compile checkpoint. The heavy render is `HF_RENDER`-gated so `go test ./...` never needs Node/Chrome.
- **No placeholders:** every Go file is a named verbatim extraction with an explicit, greppable list of symbols to keep/delete; every test's assertions are enumerated; every step ends in a runnable command with expected output.
- **Honesty flag:** the pure tests prove the template *fills* correctly (HTML contains the right markers, no unrendered delimiters, brand CSS injected, scene JSON shaped right) — they do **not** prove a real MP4 renders. The real lint/inspect/render is `HF_RENDER`-gated and only runs where Node+Chrome exist; Task 6 Step 5 says to report a skip/failure honestly rather than claim an unproven render. The binding end-to-end MP4 success criterion (design §10.1) is met later (2b-4 wiring → 2b-6 Docker/Railway).

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-06-10-render-engine-plan2b3.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
