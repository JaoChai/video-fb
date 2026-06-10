# Style-B Hyperframes Production Port — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every live-generated 9:16 clip render in the proven "Style B" look — no emoji tofu, Thai-safe spacing, structured glass-card/step/stat layouts, one clean animation per element, GPT-IMAGE images placed under a scrim — gated by `hyperframes inspect`.

**Architecture:** The new multi-scene template is a Go-parameterized copy of the proven golden file `render-test-out/styleB-916/index.html`. All per-scene DOM is built in-page from a **structured** `ScenesJSON` (injected by Go) — the Go side only serializes a typed `SceneContent` to JSON. The SceneAgent LLM is upgraded (new migration) to emit that structured, emoji-free content. The image prompt is tightened so GPT-IMAGE returns clean illustrations with no baked-in text. A pre-render `Inspect` gate catches overflow.

**Tech Stack:** Go (`html/template`), Hyperframes CLI 0.6.70 (HeyGen), GSAP, kie.ai gpt-image-2, Postgres-seeded agent prompts.

**Golden reference (already proven, renders clean):**
- `render-test-out/styleB-916/index.html` — the target template body (CSS + JS scene builder + animations)
- `render-test-out/styleB-916/output.mp4` — the proven render

---

## Key design decision (locked)

The SceneAgent currently emits one `on_screen_text` line per scene (often emoji-laden), which the adapter dumps into a single `headline` slot — this is the root of the "มั่ว" output. We switch to **structured per-scene content**: the LLM emits a `layout` enum + typed `content` object (rows / stat+chips / step / cta), Go serializes it to `ScenesJSON`, and the template renders the Style-B DOM. This is the only way to reliably reproduce the cards/stats/steps look.

Phases are independently shippable. **Phase 1 alone** (template + Go plumbing, fed by a deterministic sample) reproduces the demo with zero LLM risk and is fully offline-testable. Phases 2–3 wire the live LLM + images + safety gate.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `internal/producer/composition_types.go` | render structs | **Modify** — add `SceneContent`, `ContentRow`, `ContentChip`; add `Content` to `SceneSpec` |
| `internal/producer/templates/layout_multi_scene.html.tmpl` | the composition HTML/CSS/JS | **Replace** with Style-B body, Go-parameterized |
| `internal/producer/composition.go` | build `scenesTemplateData` incl. `ScenesJSON` | **Modify** — serialize structured content |
| `internal/producer/scene_adapter.go` | `GeneratedScene[]` → `SceneSpec[]` | **Modify** — build `Content`, drop single-headline path |
| `internal/producer/composition_scenes_render_test.go` | template render test | **Modify** — assert structured markup, no emoji |
| `internal/agent/script.go` | `GeneratedScene` LLM contract | **Modify** — add `Layout`, `Content` |
| `internal/agent/scene_content.go` | sanitize/clamp LLM content | **Create** |
| `internal/agent/scene_content_test.go` | sanitizer tests | **Create** |
| `migrations/0XX_scene_prompt_styleb.sql` | update seeded `scene` agent prompt | **Create** |
| `internal/producer/brand.go` | `buildScenePrompt` image instructions | **Modify** (line ~260) |
| `internal/producer/producer.go` | render orchestration | **Modify** (~line 297-300) — add Inspect gate |

---

## PHASE 1 — Template + Go plumbing (deterministic, offline-testable)

### Task 1: Structured content types

**Files:**
- Modify: `internal/producer/composition_types.go`

- [ ] **Step 1: Add the structured types** (append after `SlotSpec`):

```go
// SceneContent is the structured, render-ready content for one scene. It is
// serialized into ScenesJSON and consumed by the template's in-page DOM builder.
// Exactly one layout's fields are populated per scene; the rest stay zero.
type SceneContent struct {
	SceneNumber int     `json:"scene"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Layout      string  `json:"type"` // hook|hero|stat|step|tip|cta
	CaptionStyle string `json:"caption_style"` // word_pop|phrase_block
	BackgroundImage string `json:"bg"` // relative assets path, "" = gradient only

	Kicker string        `json:"kicker,omitempty"`
	Title  string        `json:"title,omitempty"` // may contain <span class="acc"> from emphasis
	Sub    string        `json:"sub,omitempty"`
	Rows   []ContentRow  `json:"rows,omitempty"`
	Stat   string        `json:"stat,omitempty"`
	Unit   string        `json:"unit,omitempty"`
	StatLabel string     `json:"statLabel,omitempty"`
	Chips  []ContentChip `json:"chips,omitempty"`
	Num    string        `json:"num,omitempty"`
	Of     string        `json:"of,omitempty"`
	Pill   string        `json:"pill,omitempty"`
	CTA    string        `json:"cta,omitempty"`
	Brand  string        `json:"brand,omitempty"`
}

// ContentRow is one bullet row. Bad=true tints it red (problem/❌ replacement).
type ContentRow struct {
	Text string `json:"t"`
	Bad  bool   `json:"bad,omitempty"`
}

// ContentChip is one small stat chip beneath a stat card.
type ContentChip struct {
	N string `json:"n"`
	T string `json:"t"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/producer/`
Expected: no output, exit 0.

---

### Task 2: Replace the template with the Style-B body

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl`
- Reference (golden): `render-test-out/styleB-916/index.html`

- [ ] **Step 1: Replace the file** with the golden body wrapped in the Go template define and parameterized. Copy `render-test-out/styleB-916/index.html` verbatim, then apply EXACTLY these substitutions:

1. Wrap: first line `{{define "layout_multi_scene.html.tmpl"}}<!doctype html>` … last line `…</html>{{end}}`.
2. Dimensions: replace literal `1080`→`{{.Width}}`, `1920`→`{{.Height}}`, `66.28`→`{{.DurationSeconds}}` in the `#root` data-attrs, `html,body`, `#root` width/height, and `const TOTAL = {{.DurationSeconds}};`.
3. Brand colors: replace the hard-coded `:root{ --navy-deep:#062F78; … }` block with `:root{ {{.BrandCSS}} }` (BrandCSS already emits the same `--navy-*/--amber-*` vars via `Brand.CSSVars()`).
4. Scene wrappers + content: DELETE the 9 static `<div id="scene-N">…</div>` blocks and the static `const SCENES = [ … ]` literal. Replace the SCENES literal with:
   ```js
   const SCENES = {{.ScenesJSON}};
   ```
   and build the scene wrappers in JS from SCENES (loop creating `.scene.clip` with `data-start/data-duration/data-track-index` = `10+i`, the `.scene-bg` from `sc.bg`, `.scrim`, and `.scene-content`). Use this loop in place of the static wrappers (insert right after `const SCENES`):
   ```js
   const root = document.getElementById("root");
   SCENES.forEach((sc, i) => {
     const w = document.createElement("div");
     w.id = "scene-" + sc.scene; w.className = "scene clip";
     w.setAttribute("data-start", sc.start);
     w.setAttribute("data-duration", Math.max(0.1, sc.end - sc.start).toFixed(3));
     w.setAttribute("data-track-index", 10 + i);
     w.setAttribute("data-layout-allow-overflow", "");
     w.innerHTML =
       (sc.bg ? '<div class="scene-bg" style="background-image:url(\\'' + sc.bg + '\\')"></div>' : '<div class="scene-bg"></div>') +
       '<div class="scrim"></div><div class="scene-content" data-i="' + i + '"></div>';
     root.insertBefore(w, document.getElementById("progress"));
   });
   ```
   > NOTE: dynamically-created top-level `.clip` wrappers must exist before Hyperframes initializes its timeline. They are created synchronously at the top of the inline `<script>`, which runs at page load before render seek — verified working with the caption-phrase pattern in the original template. **Task 5 validates this assumption with a real `inspect` run; if wrappers are not scheduled, fall back to server-side `{{range .Scenes}}` static wrappers (keep the JS `.scene-content` fill).**
5. Segments: replace the static `const SEGMENTS = [ … ]` with `const SEGMENTS = {{.SegmentsJSON}};`.
6. Voice + audio: replace `src="assets/voice.wav"` with `src="{{.VoiceSrc}}"`.
7. Keep the badge text dynamic: replace `<div class="badge-brand">ADS VANCE</div><div class="badge-cat">การเงิน</div>` with `<div class="badge-brand">{{.BrandName}}</div><div class="badge-cat">{{.CategoryLabel}}</div>`. (These wrappers stay static; only their text is templated.)

- [ ] **Step 2: Verify the template parses**

Run: `go test ./internal/producer/ -run TestRenderCompositionScenes -count=1`
Expected: may fail assertions (Task 4 updates them) but MUST NOT fail with a template parse error. If you see `template: ... unexpected`, fix the `{{ }}` wrapping before continuing.

---

### Task 3: Serialize structured content into ScenesJSON

**Files:**
- Modify: `internal/producer/composition.go` (the `ScenesJSON` build, ~line 109-123)
- Modify: `internal/producer/scene_adapter.go`

- [ ] **Step 1: Build `[]SceneContent` in the adapter.** Replace `buildSceneSpecs`'s single-headline slot construction so each `SceneSpec` carries a `Content`. Add a `Content SceneContent` field to `SceneSpec` (Task 1 covers types; add the field):

```go
// in composition_types.go SceneSpec, add:
	Content SceneContent
```

Then in `scene_adapter.go`, after computing `bgMode`, build content from the (Phase-2) structured fields with a safe fallback to a hero title when structured content is absent:

```go
content := buildSceneContent(s, b) // see Task 9; Phase-1 stub below
specs[i] = SceneSpec{
	SceneNumber:    s.SceneNumber,
	LayoutVariant:  normalizeLayout(s.LayoutVariant),
	AccentColor:    Brand.Orange,
	AnimationSpeed: "normal",
	StartSec:       b.Start,
	EndSec:         b.End,
	BackgroundMode: bgMode,
	CaptionStyle:   normalizeCaptionStyle(s.CaptionStyle),
	Content:        content,
}
```

Phase-1 stub for `buildSceneContent` (replaced in Task 9):
```go
func buildSceneContent(s agent.GeneratedScene, b sceneBound) SceneContent {
	return SceneContent{
		SceneNumber: s.SceneNumber, Start: b.Start, End: b.End,
		Layout: "hero", CaptionStyle: normalizeCaptionStyle(s.CaptionStyle),
		Title: highlightTitleStr(strings.TrimSpace(s.OnScreenText), s.EmphasisWords),
	}
}
```
(`highlightTitleStr` returns a `string` with `<span class="acc">` wraps — extract the non-`template.HTML` core of existing `highlightTitle`.)

- [ ] **Step 2: Serialize in composition.go.** Where `ScenesJSON` is currently built, marshal `[]SceneContent` (with `BackgroundImage` set from each scene's resolved asset path) instead of the old shape:

```go
contents := make([]SceneContent, len(p.Scenes))
for i, sc := range p.Scenes {
	contents[i] = sc.Content
	if sc.BackgroundMode == "image" {
		contents[i].BackgroundImage = sc.BackgroundImage
	}
}
scenesJSON, _ := json.Marshal(contents)
```
and assign `ScenesJSON: template.JS(scenesJSON)`.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: exit 0.

---

### Task 4: Template render test — structured markup, no emoji

**Files:**
- Modify: `internal/producer/composition_scenes_render_test.go`

- [ ] **Step 1: Write the failing test** asserting the new contract on a sample with all layout types:

```go
func TestRenderCompositionScenes_StyleB(t *testing.T) {
	params := ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", CategoryLabel: "การเงิน",
		VoiceSrc: "assets/voice.wav", DurationSeconds: 10,
		Scenes: []SceneSpec{
			{SceneNumber: 1, StartSec: 0, EndSec: 5, BackgroundMode: "image", BackgroundImage: "assets/bg-scene1.png",
				Content: SceneContent{SceneNumber: 1, Start: 0, End: 5, Layout: "stat", Stat: "2026", StatLabel: "ปีบังคับใช้",
					Chips: []ContentChip{{N: "90%", T: "ยังไม่รองรับ"}}}},
			{SceneNumber: 2, StartSec: 5, EndSec: 10, BackgroundMode: "css",
				Content: SceneContent{SceneNumber: 2, Start: 5, End: 10, Layout: "step", Num: "1", Of: "ขั้นที่ 1",
					Title: "โทรธนาคาร", Rows: []ContentRow{{Text: "ขอเปิดต่างประเทศ"}}}},
		},
		Segments: []TranscriptSegment{{Text: "ทดสอบ", Start: 0, End: 10}},
	}
	html, err := RenderCompositionScenes(params)
	if err != nil { t.Fatalf("render: %v", err) }
	for _, want := range []string{`const SCENES =`, `"type":"stat"`, `"stat":"2026"`, `"type":"step"`, `letter-spacing:0`, `--amber`} {
		if !strings.Contains(html, want) { t.Errorf("missing %q", want) }
	}
	// No emoji should ever reach the composition.
	for _, emo := range []string{"❌", "✅", "📞", "💳", "🛡️", "👇", "⏰"} {
		if strings.Contains(html, emo) { t.Errorf("emoji leaked: %q", emo) }
	}
}
```

- [ ] **Step 2: Run it — expect FAIL** (assertions on new markup)

Run: `go test ./internal/producer/ -run TestRenderCompositionScenes_StyleB -v`
Expected: FAIL until Tasks 2–3 land. With Tasks 2–3 done: PASS.

- [ ] **Step 3: Make it pass** — fix any field-name / JSON-tag mismatch surfaced by the test.

- [ ] **Step 4: Run full producer tests**

Run: `go test ./internal/producer/ -count=1`
Expected: PASS (update any old assertions that referenced the removed single-headline slot markup).

- [ ] **Step 5: Commit**

```bash
git add internal/producer/ docs/superpowers/plans/
git commit -m "feat(producer): Style-B structured multi-scene template + content model"
```

---

### Task 5: Golden render gate (real Hyperframes)

**Files:** none (validation only)

- [ ] **Step 1: Regenerate the golden composition through the Go path** using the deterministic sample, OR re-confirm the existing golden renders clean:

Run:
```bash
cd render-test-out/styleB-916 && npx --yes hyperframes@0.6.70 inspect
```
Expected: `0 layout issues across N sample(s)`.

- [ ] **Step 2: If Task 2's JS-built wrappers fail inspect** (scenes missing/blank), switch the template to server-side static `{{range .Scenes}}` `.scene.clip` wrappers (keep JS filling `.scene-content`), then re-run inspect. Document which path was used in the commit message.

---

## PHASE 2 — LLM agent emits structured content

### Task 6: Extend the LLM contract

**Files:**
- Modify: `internal/agent/script.go` (`GeneratedScene`, ~line 33-48)

- [ ] **Step 1: Add fields** to `GeneratedScene`:

```go
	// Style-B structured content (Plan 2b-6). Emitted by the upgraded SceneAgent.
	Layout  string          `json:"layout"`  // hook|hero|stat|step|tip|cta
	Content json.RawMessage `json:"content"` // typed per layout; parsed in scene_content.go
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/agent/`
Expected: exit 0.

---

### Task 7: Migration — upgrade the seeded `scene` prompt

**Files:**
- Create: `migrations/0XX_scene_prompt_styleb.sql` (use the next free number; check `ls migrations/`)
- Reference: existing seed in `migrations/011_agent_prompt_templates.sql`

- [ ] **Step 1: Write the migration** updating the `scene` agent's `prompt_template` (and system prompt) so it returns, per scene, a `layout` enum + a `content` object with the EXACT keys the template consumes, and **forbids emoji**. Include one worked example per layout. Skeleton:

```sql
-- 0XX_scene_prompt_styleb.sql
UPDATE agent_configs SET prompt_template = $TPL$
... ภาษาไทย instructions ...
ทุกฉากต้องส่ง JSON: {"scene_number":N,"voice_text":"...","layout":"hook|hero|stat|step|tip|cta",
"caption_style":"word_pop|phrase_block","image_prompt":"...","content":{...}}

ห้ามใส่ emoji/อีโมจิ/สัญลักษณ์ ❌✅📞 ใน content หรือ on-screen ใดๆ เด็ดขาด — ใช้ rows/bad แทน.

content ตาม layout:
- hook:  {"kicker":"...","rows":[{"t":"ปัญหา","bad":true}, ...]}        // bad=true = แถวแดง
- hero:  {"title":"...","sub":"..."}                                      // เน้นคำด้วย <span class="acc">คำ</span>
- stat:  {"kicker":"...","stat":"2026","unit":"","statLabel":"...","chips":[{"n":"90%","t":"..."}]}
- step:  {"num":"1","of":"ขั้นตอนที่ 1 / 4","title":"...","rows":[{"t":"..."}]}
- tip:   {"pill":"ป้องกันระยะยาว","rows":[{"t":"..."}]}
- cta:   {"title":"...","cta":"ทักหาเราเลย","brand":"ADS VANCE","sub":"..."}
$TPL$
WHERE name = 'scene';
```

- [ ] **Step 2: Apply locally and verify it loads** (use the project's migrate path — check `Makefile` for the target):

Run: `make migrate` (or the documented migrate command)
Expected: migration applied, no error. Confirm: the `scene` row's `prompt_template` now contains `"layout"`.

---

### Task 8: Content sanitizer (defense in depth)

**Files:**
- Create: `internal/agent/scene_content.go`
- Create: `internal/agent/scene_content_test.go`

- [ ] **Step 1: Write failing tests:**

```go
func TestStripEmoji(t *testing.T) {
	if got := StripEmoji("ขั้น 1 📞 โทร ✅"); got != "ขั้น 1  โทร " {
		t.Fatalf("got %q", got)
	}
}
func TestClampLayout(t *testing.T) {
	if ClampLayout("banana") != "hero" { t.Fatal("unknown→hero") }
	if ClampLayout("stat") != "stat" { t.Fatal("known kept") }
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./internal/agent/ -run 'TestStripEmoji|TestClampLayout' -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement:**

```go
package agent

import "unicode"

var sceneLayouts = map[string]bool{"hook": true, "hero": true, "stat": true, "step": true, "tip": true, "cta": true}

func ClampLayout(v string) string {
	if sceneLayouts[v] { return v }
	return "hero"
}

// StripEmoji removes emoji / pictographic runes that the bundled Sarabun font
// cannot render (they become tofu boxes). Thai, Latin, digits, punctuation stay.
func StripEmoji(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r > 0x2190 && (unicode.Is(unicode.So, r) || unicode.Is(unicode.Sk, r) || r >= 0x1F000) {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
```

- [ ] **Step 4: Run — expect PASS**

Run: `go test ./internal/agent/ -run 'TestStripEmoji|TestClampLayout' -v`
Expected: PASS.

---

### Task 9: Adapter maps `Content` → `SceneContent`

**Files:**
- Modify: `internal/producer/scene_adapter.go` (replace the Task-3 stub `buildSceneContent`)

- [ ] **Step 1: Parse the LLM `Content` JSON into `SceneContent`**, applying `ClampLayout` + `StripEmoji` to every text field, with the hero-title fallback when `Content` is empty/unparseable:

```go
func buildSceneContent(s agent.GeneratedScene, b sceneBound) SceneContent {
	c := SceneContent{
		SceneNumber: s.SceneNumber, Start: b.Start, End: b.End,
		Layout: agent.ClampLayout(s.Layout), CaptionStyle: normalizeCaptionStyle(s.CaptionStyle),
	}
	var raw struct {
		Kicker, Title, Sub, Stat, Unit, StatLabel, Num, Of, Pill, CTA, Brand string
		Rows  []struct{ T string `json:"t"`; Bad bool `json:"bad"` } `json:"rows"`
		Chips []struct{ N, T string } `json:"chips"`
	}
	if len(s.Content) > 0 {
		_ = json.Unmarshal(s.Content, &raw)
	}
	clean := agent.StripEmoji
	c.Kicker, c.Title, c.Sub = clean(raw.Kicker), highlightKeepSpans(clean(raw.Title)), clean(raw.Sub)
	c.Stat, c.Unit, c.StatLabel = clean(raw.Stat), clean(raw.Unit), clean(raw.StatLabel)
	c.Num, c.Of, c.Pill = clean(raw.Num), clean(raw.Of), clean(raw.Pill)
	c.CTA, c.Brand = clean(raw.CTA), clean(raw.Brand)
	for _, r := range raw.Rows { c.Rows = append(c.Rows, ContentRow{Text: clean(r.T), Bad: r.Bad}) }
	for _, ch := range raw.Chips { c.Chips = append(c.Chips, ContentChip{N: clean(ch.N), T: clean(ch.T)}) }
	if c.Layout == "hero" && c.Title == "" {
		c.Title = highlightTitleStr(clean(strings.TrimSpace(s.OnScreenText)), s.EmphasisWords)
	}
	return c
}
```
(`highlightKeepSpans` is a no-op passthrough that preserves any `<span class="acc">` the LLM emitted; if you prefer Go-side emphasis, route through `highlightTitleStr` with `EmphasisWords` instead.)

- [ ] **Step 2: Run producer tests**

Run: `go test ./internal/producer/ -count=1`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/agent/ internal/producer/ migrations/
git commit -m "feat(agent): structured Style-B scene content + emoji sanitizer + prompt migration"
```

---

## PHASE 3 — Clean images + inspect gate

### Task 10: Image prompt returns clean illustrations

**Files:**
- Modify: `internal/producer/brand.go` (`buildScenePrompt`, line ~260)

- [ ] **Step 1: Tighten the instruction** so GPT-IMAGE returns a clean illustration with NO baked-in text, numbers, emoji, or UI — the template supplies all text. Add to the prompt string:

```
" IMPORTANT: illustration only — no text, no numbers, no logos, no emoji, no UI chrome. " +
" Subject centered in the UPPER 55% of the frame; leave the lower 45% as clean empty navy " +
" background (a content card is overlaid there). Flat vector style, brand navy + amber. "
```

- [ ] **Step 2: Verify build + existing brand test**

Run: `go test ./internal/producer/ -run TestBuildScenePrompt -v` (or `go build ./internal/producer/` if no such test)
Expected: PASS / exit 0. Update the brand test's expected substring if it asserts the prompt text.

---

### Task 11: Inspect gate before render

**Files:**
- Modify: `internal/producer/producer.go` (~line 297-300)

- [ ] **Step 1: Call `Inspect` after build, before `Render`; on overflow, log + still render (non-fatal) so a clip is never silently dropped, but the issue is visible:**

```go
if _, err := p.hf.builder.BuildScenes(params, clipID, projectDir, voicePath, bgPaths); err != nil {
	return "", fmt.Errorf("build scenes: %w", err)
}
if err := p.hf.renderer.Inspect(ctx, projectDir); err != nil {
	log.Printf("hyperframes inspect flagged layout issues for clip %s (rendering anyway): %v", clipID, err)
}
if err := p.hf.renderer.Render(ctx, projectDir, "output.mp4"); err != nil {
	return "", fmt.Errorf("render: %w", err)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

---

### Task 12: End-to-end smoke

**Files:**
- Modify: `internal/producer/producer_hyperframes_test.go` (`TestAssembleHyperframes916_Smoke`, line ~31)

- [ ] **Step 1: Ensure the smoke test feeds structured `Layout`/`Content`** (or the hero fallback) and asserts the produced `index.html` contains `const SCENES =` and no emoji. Run:

Run: `go test ./internal/producer/ -run TestAssembleHyperframes916_Smoke -v`
Expected: PASS.

- [ ] **Step 2: Commit**

```bash
git add internal/producer/
git commit -m "feat(producer): clean image prompt + inspect gate before render"
```

---

## PHASE 4 — Ship (deploy 2b-6)

### Task 13: Simplify, verify, deploy

- [ ] **Step 1: Simplify the diff** (per user preference): run `/simplify` over the branch diff and apply.
- [ ] **Step 2: Full build + test**

Run: `go build ./... && go test ./... -count=1`
Expected: all PASS.

- [ ] **Step 3: One real end-to-end render** (locally or via the Docker image) of a sample topic; eyeball 3-4 frames for Style-B correctness + no tofu.
- [ ] **Step 4: Commit + deploy** per `pre-deploy-checklist` skill (Railway). Confirm the live `/orchestrator/produce` produces a Style-B clip.

---

## Self-Review

- **Spec coverage:** emoji tofu → Task 8 + Task 7 (forbid) + Task 10 (images); Thai spacing → Task 2 (`letter-spacing:0`); structured B layouts → Tasks 1-3,6-9; clean animation → Task 2 (golden body); image placement → Task 2 (scrim) + Task 10 (safe zone); inspect gate → Task 11. ✅
- **Type consistency:** `SceneContent` JSON tags (`type`,`stat`,`statLabel`,`rows`,`chips`,`bg`) match the golden template's `sc.*` reads and the test assertions in Task 4. `buildSceneContent` signature identical in Task 3 stub and Task 9 final. ✅
- **Open risk (flagged in-plan):** Task 2's JS-built scene wrappers vs Hyperframes clip scheduler — Task 5 validates with real `inspect`; documented fallback to server-side `{{range}}`.
