# Multi-Scene Phase 3 — Template + Layout Variants + Inspect Gate

> **For agentic workers:** subagent-driven-development. Visual layer — verification is **render + `hyperframes inspect`**, not only unit tests. Steps use `- [ ]`.

**Goal:** เปลี่ยน template เป็น multi-scene: หลายฉาก (พื้นหลัง AI cross-fade) บน timeline เดียว, แต่ละฉากใช้ flex-column layout variant + slots, caption band + progress ต่อเนื่อง, ทำงานทั้ง 9:16 + 16:9, มี `inspect` เป็น gate กัน overlap

**Architecture:** Go `RenderCompositionScenes(params)` ใหม่ (รับ scenes[]+slots) → execute template `layout_multi_scene.html.tmpl` ที่ iterate ฉาก: ต่อฉากมี `.scene` (absolute full-frame, decorative bg) + `.scene-content` (flex column + padding + gap — **ห้าม absolute** ตาม Hyperframes rule) เติม slots ตาม role, GSAP cross-fade bg + fade scene-content ตามช่วงเวลาฉาก, caption band (flow-safe, reserved zone) + progress bar ต่อเนื่อง. ปรับ padding/font ตาม aspect ratio (param `AspectRatio` "9:16"|"16:9"). `HyperframesRenderer` เพิ่ม `Inspect()` (เรียก `hyperframes inspect`) เป็น gate หลัง Lint. **เก็บ template `layout_dynamic_karaoke` เดิมไว้** (producer ยังใช้จน Phase 4)

**Tech:** Go html/template + embed, GSAP timeline, Hyperframes 0.6.70 (`fitTextFontSize`, `inspect`), CSS flexbox

**Reference (ต้องอ่านก่อนทำ):**
- `internal/producer/templates/layout_dynamic_karaoke.html.tmpl` (template เดิม — โครง GSAP/caption/progress/Ken Burns เอามาต่อยอด)
- `internal/producer/composition.go` (RenderComposition เดิม), `composition_builder.go` (templateData), `hyperframes.go` (Lint/Render)
- Hyperframes installed docs (verified): `~/.npm/_npx/*/node_modules/hyperframes/dist/skills/hyperframes/SKILL.md` (Layout Before Animation: `.scene-content` flex, ห้าม absolute content), `references/captions.md`, `commands/layout-audit.browser.js` (inspect: canvas_overflow/container_overflow/clipped_text)
- `internal/agent/composition.go` types: `SceneDesign{SceneNumber,LayoutVariant,Slots,AccentColor,BgArtPrompt,AnimationSpeed}`, `Slot{Role,Text,Emphasis}`, layout variants hook_big/list_steps/stat_reveal/quote_cta

**Scope:** template + Go render + inspect gate. **ไม่แตะ producer wiring** (Phase 4). dev branch.

---

### Task 1: Scene render data model (Go) + RenderCompositionScenes skeleton + test

**Files:** Modify `internal/producer/composition.go`, `composition_types.go`; Test `internal/producer/composition_scenes_test.go`

- [ ] **Step 1** — เพิ่ม types ใน composition_types.go:
```go
// SceneSpec is one fully-resolved scene the multi-scene template renders.
type SceneSpec struct {
	SceneNumber   int
	LayoutVariant string  // hook_big|list_steps|stat_reveal|quote_cta
	AccentColor   string  // sanitized hex
	AnimationSpeed string // fast|normal|slow
	StartSec      float64 // scene window on the continuous timeline
	EndSec        float64
	BackgroundMode  string // "css" | "image"
	BackgroundImage string // relative assets path when image
	Slots         []SlotSpec
}

// SlotSpec is one semantic content slot rendered in scene flow layout.
type SlotSpec struct {
	Role     string // headline|body|badge|step
	HTML     template.HTML // pre-escaped (emphasis applied)
	StepNum  int
}

// ScenesParams is the full input for the multi-scene template.
type ScenesParams struct {
	AspectRatio    string // "9:16" | "16:9"
	BrandName      string
	CategoryLabel  string
	QuestionerName string
	Kicker         string
	VoiceSrc       string
	DurationSeconds float64
	Scenes         []SceneSpec
	Segments       []TranscriptSegment // continuous karaoke captions
}
```
- [ ] **Step 2** — failing test `composition_scenes_test.go`: build a `ScenesParams` with 3 scenes (varied layout variants + slots) for "9:16", call `RenderCompositionScenes`, assert: no `{{`/`}}` left, contains each scene's headline text, contains `data-width="1080"`/`data-height="1920"`, SEGMENTS JSON present, no nested `<span class="hl"><span`. Add a second case for "16:9" asserting `data-width="1920"`/`data-height="1080"`. (Run → FAIL: undefined RenderCompositionScenes)
- [ ] **Step 3** — implement `RenderCompositionScenes(p ScenesParams) ([]byte, error)` in composition.go: validate Duration>0 & len(Scenes)>0; marshal Segments + a scenes JSON for the GSAP driver; pick width/height from AspectRatio; execute new template `layout_multi_scene.html.tmpl`. (Template built in Task 2 — for this step create a MINIMAL template that emits the data-* attrs + scene headlines + segments JSON so the test passes; Task 2 fleshes out visuals.)
- [ ] **Step 4** — run test → PASS, `go build ./...`
- [ ] **Step 5** — commit `feat(producer): multi-scene render data model + RenderCompositionScenes`

---

### Task 2: Full multi-scene template + 4 layout variants (VISUAL — render-verified)

**Files:** Create `internal/producer/templates/layout_multi_scene.html.tmpl`; maybe partials per variant

- [ ] **Step 1** — Build the template evolving from `layout_dynamic_karaoke.html.tmpl`. Requirements (verify each against Hyperframes SKILL.md):
  - `#root` fixed `{{.Width}}×{{.Height}}`, `overflow:hidden`
  - Per scene: a `.scene` layer (absolute, full-frame) holding the background (image w/ Ken Burns OR css gradient per `BackgroundMode`) — decorative, absolute OK
  - Per scene: a `.scene-content` = `width:100%;height:100%;display:flex;flex-direction:column;justify-content:center;gap;padding;box-sizing:border-box` — **NEVER absolute** (this is the anti-overlap guarantee). Slots render in flow by role; layout_variant changes flex alignment/sizing (hook_big = big centered headline; list_steps = stacked steps; stat_reveal = large stat + label; quote_cta = centered quote + CTA)
  - Headline/body text use `fitTextFontSize` (with the string-length fallback already in the old template) — maxWidth ≈ 900 (9:16) / 1600 (16:9)
  - Caption band: reserved zone (9:16 ~600-700px from bottom / 16:9 ~80-120px), full-width centered (NOT translateX), continuous karaoke across scenes (reuse old caption GSAP)
  - Progress bar continuous 0→TOTAL; optional brand badge intro; outro CTA scene handled as the last scene (quote_cta)
  - GSAP: one `window.__timelines["main"]`; per scene cross-fade (`tl.to` opacity on `.scene` at scene boundaries with overlap for fade); scene-content fade in/out within its window; `tl.set(...opacity:0)` hard-kill at scene end
  - Padding/font scale by aspect ratio (template receives `.Width`/`.Height` or `.AspectRatio`)
- [ ] **Step 2** — `go test ./internal/producer/ -run Composition` (Task 1 test + existing) PASS; `go build ./...`
- [ ] **Step 3 — RENDER VERIFY (the real gate):** extend `cmd/hfslice` (or add a flag) to build a multi-scene project from a sample ScenesParams and run `hyperframes lint` + `hyperframes inspect` + `hyperframes render`. Run it locally for BOTH 9:16 and 16:9. Confirm: inspect reports **no** canvas_overflow/container_overflow/clipped_text; render produces a valid MP4; extract 1 frame per scene and eyeball (Thai font OK, no overlap, bg cross-fade). Paste inspect output + frame paths.
- [ ] **Step 4** — commit `feat(producer): multi-scene template + 4 layout variants`

---

### Task 3: Inspect gate in HyperframesRenderer

**Files:** Modify `internal/producer/hyperframes.go`

- [ ] **Step 1** — add `func (h *HyperframesRenderer) Inspect(ctx, dir) error` mirroring `Lint` but `h.run(ctx, dir, "inspect")` (or `"layout"` if that's the CLI verb — confirm via `npx hyperframes --help`). Doc the intent (collision/overflow audit gate).
- [ ] **Step 2** — `go build ./...` clean
- [ ] **Step 3** — commit `feat(producer): hyperframes inspect gate for collision audit`

---

## Self-Review
- Spec §3 (anti-overlap: flex `.scene-content`, fitText, reserved caption zone, inspect gate) → Task 2 + Task 3. §5 (template 1-for-2-ratio, cross-fade, per-scene bg) → Task 1+2. Layout library 4 variants → Task 2.
- Visual correctness NOT claimable from unit tests alone → Task 2 Step 3 render+inspect is the gate (explicit).
- Additive: old `layout_dynamic_karaoke` + `RenderComposition` kept; producer wiring deferred to Phase 4.
- Note: `template.HTML`/`template.JS` for slot HTML + JSON (XSS-safe: emphasis applied via the existing escaped highlightTitle-style helper).
