# Video Style Presets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make each produced clip automatically pick one of a curated set of complete visual style presets (palette + AI art style + overlay font), so consecutive clips look different while staying recognizably "Ads Vance".

**Architecture:** Presets are defined in Go as a list of `StylePreset` values (each a full `BrandColors` palette + an image-style anchor + a font). The orchestrator picks one per clip (random, avoiding the previous clip's preset) behind a feature flag, persists the chosen key on the clip, and threads the preset through `ProduceHyperframes916` so both the AI image prompt and the HTML overlay CSS read from the preset instead of the hardcoded package-global `Brand`. Off ⇒ everything falls back to the `signature` preset (today's exact look).

**Tech Stack:** Go, pgx/pgxpool, html/template, embedded fonts (.ttf), Chrome-render hyperframes pipeline.

## Global Constraints

- Language of all on-screen/voice copy is Thai; any bundled font MUST render Thai vowels and tone marks correctly (verify by render before shipping). — from spec "Fonts".
- Brand-constant across ALL presets: leopard mascot, brand name `ADS VANCE` (`producer.BrandName`), CTA copy (`producer.BrandCTA`). Presets never change these. — from spec "Brand-constant anchors".
- Production must never break on preset failure: empty/unknown preset → `signature` fallback. — from spec "Error handling".
- Feature flag `STYLE_PRESETS_ENABLED` (env, `"true"` to enable). Off ⇒ pixel-identical to today. — from spec "Feature flag".
- Hyperframes render is CPU-sensitive (past `protocolTimeout`/contention incidents); do not add new render-time work beyond swapping CSS/prompt values. — project history.
- The `signature` preset's `BrandColors`, `ImageAnchor`, and font MUST equal today's hardcoded `producer.Brand` / `Brand.ImageStyleAnchor()` / Sarabun, so flag-off is a no-op visually.

---

## File Structure

- **Create** `internal/producer/presets.go` — `StylePreset` type, the curated `Presets` list, `PickPreset`, `StylePresetsEnabled`, `StylePreset.BrandCSS()`, `StylePreset.AsTheme()`.
- **Create** `internal/producer/presets_test.go` — unit tests for the above.
- **Modify** `internal/producer/brand.go` — `buildScenePrompt` gains a preset param; `CSSVars` becomes font-parameterized (or a preset-aware composer is added).
- **Modify** `internal/producer/composition.go` — `RenderCompositionScenes` uses `params.Palette` instead of global `Brand` for `BrandCSS` and the accent fallback.
- **Modify** `internal/producer/composition_types.go` — `ScenesParams` gains `Palette BrandColors` + `BrandCSS string`.
- **Modify** `internal/producer/producer.go` — `AssembleHyperframes916` and `ProduceHyperframes916` gain a `preset StylePreset` param; pass it into `buildScenePrompt` and `ScenesParams`.
- **Modify** `internal/orchestrator/orchestrator.go` — select a preset per clip, persist it, thread it into the producer and into the Scene/Image agents.
- **Modify** `internal/repository/clips.go` — `LastStylePreset`; persist `style_preset` on create/update.
- **Modify** `internal/models/clip.go` — add `StylePreset` to clip request/struct.
- **Create** `migrations/040_clip_style_preset.sql` — `ALTER TABLE clips ADD COLUMN style_preset`.
- **Modify** (Task 5 only) `internal/producer/templates/layout_multi_scene.html.tmpl` + `internal/producer/assets/fonts/` — alt Thai fonts.

---

## Task 1: StylePreset unit (type, presets, selection, CSS)

**Files:**
- Create: `internal/producer/presets.go`
- Test: `internal/producer/presets_test.go`

**Interfaces:**
- Consumes: `BrandColors`, `MotionTokens` (`Motion`), `TypeTokens` (`Type`), `Brand`, `Brand.ImageStyleAnchor()` — all in `internal/producer/brand.go`. `models.BrandTheme` in `internal/models/theme.go`.
- Produces:
  - `type StylePreset struct { Key, DisplayName string; Palette BrandColors; ImageAnchor string; Font TypeTokens }`
  - `var Presets []StylePreset` (first element Key `"signature"`)
  - `func PickPreset(lastKey string) StylePreset`
  - `func PresetByKey(key string) StylePreset` (unknown → signature)
  - `func StylePresetsEnabled() bool`
  - `func (p StylePreset) BrandCSS() string`
  - `func (p StylePreset) AsTheme(base *models.BrandTheme) *models.BrandTheme`

- [ ] **Step 1: Write the failing test**

```go
// internal/producer/presets_test.go
package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func TestPresets_SignatureIsFirstAndMatchesBrand(t *testing.T) {
	if len(Presets) == 0 {
		t.Fatal("Presets must not be empty")
	}
	sig := Presets[0]
	if sig.Key != "signature" {
		t.Fatalf("Presets[0].Key = %q, want signature", sig.Key)
	}
	if sig.Palette != Brand {
		t.Error("signature palette must equal Brand (flag-off must be a no-op)")
	}
	if sig.ImageAnchor != Brand.ImageStyleAnchor() {
		t.Error("signature ImageAnchor must equal Brand.ImageStyleAnchor()")
	}
	if sig.Font != Type {
		t.Error("signature Font must equal Type (Sarabun)")
	}
}

func TestPresets_AllUniqueKeysAndValidHex(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Presets {
		if seen[p.Key] {
			t.Errorf("duplicate preset key %q", p.Key)
		}
		seen[p.Key] = true
		for _, c := range []string{p.Palette.NavyDeep, p.Palette.Navy, p.Palette.Orange, p.Palette.OrangeBright, p.Palette.Ink, p.Palette.Muted} {
			if !strings.HasPrefix(c, "#") || (len(c) != 7 && len(c) != 4) {
				t.Errorf("preset %q has invalid hex %q", p.Key, c)
			}
		}
		if strings.TrimSpace(p.ImageAnchor) == "" {
			t.Errorf("preset %q has empty ImageAnchor", p.Key)
		}
	}
}

func TestPickPreset_AvoidsLastWhenPossible(t *testing.T) {
	if len(Presets) < 2 {
		t.Skip("need >=2 presets to test avoid-last")
	}
	last := Presets[0].Key
	for i := 0; i < 50; i++ {
		got := PickPreset(last)
		if got.Key == last {
			t.Fatalf("PickPreset(%q) returned the avoided key", last)
		}
	}
}

func TestPickPreset_EmptyLastReturnsValid(t *testing.T) {
	got := PickPreset("")
	if PresetByKey(got.Key).Key != got.Key {
		t.Errorf("PickPreset returned unknown key %q", got.Key)
	}
}

func TestPresetByKey_UnknownFallsBackToSignature(t *testing.T) {
	if PresetByKey("does-not-exist").Key != "signature" {
		t.Error("unknown key must fall back to signature")
	}
}

func TestBrandCSS_ContainsPaletteAndFont(t *testing.T) {
	p := PresetByKey("signature")
	css := p.BrandCSS()
	for _, want := range []string{"--navy-deep", "--orange", "--orange-bright", "--ink", "--muted", "--red", p.Palette.Navy, p.Font.Family} {
		if !strings.Contains(css, want) {
			t.Errorf("BrandCSS missing %q", want)
		}
	}
}

func TestAsTheme_OverridesColorsFromPreset(t *testing.T) {
	base := &models.BrandTheme{PrimaryColor: "x", AccentColor: "y", Name: "Base"}
	p := Presets[len(Presets)-1]
	got := p.AsTheme(base)
	if got.PrimaryColor != p.Palette.Navy || got.AccentColor != p.Palette.Orange {
		t.Error("AsTheme must override primary/accent from the preset palette")
	}
	if base.PrimaryColor != "x" {
		t.Error("AsTheme must not mutate the base theme")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run 'Preset|PickPreset|BrandCSS|AsTheme' -v`
Expected: FAIL (undefined: StylePreset, Presets, PickPreset, …).

- [ ] **Step 3: Implement `presets.go`**

```go
// internal/producer/presets.go
package producer

import (
	"hash/fnv"
	"os"

	"github.com/jaochai/video-fb/internal/models"
)

// StylePreset is one complete, internally-cohesive visual identity: a full color
// palette, a matching AI art-style anchor, and an overlay font. Presets vary the
// look per clip; the leopard mascot, brand name, and CTA stay constant across all
// of them (see producer.BrandName / BrandCTA) so the channel still reads as one brand.
type StylePreset struct {
	Key         string      // stable id persisted on the clip
	DisplayName string      // human label for logs/admin
	Palette     BrandColors // overlay + image colors
	ImageAnchor string      // art-style paragraph; its colors MUST match Palette
	Font        TypeTokens  // overlay font (Thai-capable)
}

// Presets is the curated set. Presets[0] is "signature" — it equals today's
// hardcoded Brand look and is the universal fallback, so a disabled flag or any
// selection failure reproduces the current output exactly.
var Presets = []StylePreset{
	{
		Key:         "signature",
		DisplayName: "Signature Royal Blue",
		Palette:     Brand,
		ImageAnchor: Brand.ImageStyleAnchor(),
		Font:        Type,
	},
	{
		Key:         "teal-coral",
		DisplayName: "Teal & Coral",
		Palette: BrandColors{
			NavyDeep: "#04302E", Navy: "#0A5247", NavyHi: "#138B7A",
			Orange: "#FF6B5C", OrangeSoft: "#F2856E", OrangeBright: "#FF9A7A",
			Ink: "#F2FBF8", Muted: "#A9D9CE",
			Warn: "#ff5a52", Win: "#2fd17a", Info: "#3b82f6",
		},
		ImageAnchor: "Soft 3D clay-render illustration style with gentle studio lighting. " +
			"Strict two-tone palette: deep teal #0A5247 as the dominant background and structural color, " +
			"warm coral #FF6B5C as the single accent for highlights and focal points. " +
			"No other saturated hues. Rounded clean shapes, soft shadows, no photorealism, no text. " +
			"Atmosphere: friendly, modern, premium digital-marketing brand identity.",
		Font: Type,
	},
	{
		Key:         "purple-gold",
		DisplayName: "Royal Purple & Gold",
		Palette: BrandColors{
			NavyDeep: "#1A0E3D", Navy: "#2E1A66", NavyHi: "#4A2E9E",
			Orange: "#F0C030", OrangeSoft: "#E8B84A", OrangeBright: "#FFD66B",
			Ink: "#F7F3FF", Muted: "#C9B8FF",
			Warn: "#ff5a52", Win: "#2fd17a", Info: "#3b82f6",
		},
		ImageAnchor: "Premium gradient-mesh illustration with glassy translucent surfaces and soft glow. " +
			"Strict two-tone palette: deep royal purple #2E1A66 as the dominant background and structural color, " +
			"luxurious gold #F0C030 as the single accent for highlights and focal points. " +
			"No other saturated hues. Smooth vector rendering, subtle bloom, no photorealism, no text. " +
			"Atmosphere: luxurious, confident, premium digital-marketing brand identity.",
		Font: Type,
	},
	{
		Key:         "charcoal-electric",
		DisplayName: "Charcoal & Electric Blue",
		Palette: BrandColors{
			NavyDeep: "#10141B", Navy: "#1B2330", NavyHi: "#2A3647",
			Orange: "#2E8BFF", OrangeSoft: "#4A9BFF", OrangeBright: "#6FB4FF",
			Ink: "#F2F6FF", Muted: "#9FB2CC",
			Warn: "#ff5a52", Win: "#2fd17a", Info: "#3b82f6",
		},
		ImageAnchor: "Sleek techno HUD illustration with crisp neon line-art and thin glowing strokes. " +
			"Strict two-tone palette: near-black charcoal #1B2330 as the dominant background and structural color, " +
			"electric blue #2E8BFF as the single accent for highlights and focal points. " +
			"No other saturated hues. Clean vector rendering, subtle scanline glow, no photorealism, no text. " +
			"Atmosphere: high-tech, sharp, premium digital-marketing brand identity.",
		Font: Type,
	},
	{
		Key:         "sunset-magenta",
		DisplayName: "Sunset Magenta",
		Palette: BrandColors{
			NavyDeep: "#2B0E2E", Navy: "#5A1A4D", NavyHi: "#8E2A66",
			Orange: "#FF8A3D", OrangeSoft: "#FF7A5C", OrangeBright: "#FFB454",
			Ink: "#FFF3F0", Muted: "#F0C2D6",
			Warn: "#ff5a52", Win: "#2fd17a", Info: "#3b82f6",
		},
		ImageAnchor: "Warm grainy risograph poster illustration with bold flat shapes and a soft paper texture. " +
			"Strict two-tone palette: deep magenta-plum #5A1A4D as the dominant background and structural color, " +
			"warm sunset orange #FF8A3D as the single accent for highlights and focal points. " +
			"No other saturated hues. Slight grain, no photorealism, no text. " +
			"Atmosphere: bold, energetic, premium digital-marketing brand identity.",
		Font: Type,
	},
}

// StylePresetsEnabled reports whether per-clip preset selection is on. Off ⇒
// callers use the signature preset, reproducing today's exact look.
func StylePresetsEnabled() bool { return os.Getenv("STYLE_PRESETS_ENABLED") == "true" }

// PresetByKey returns the preset with key, or the signature preset (Presets[0])
// when key is unknown/empty. Never panics.
func PresetByKey(key string) StylePreset {
	for _, p := range Presets {
		if p.Key == key {
			return p
		}
	}
	return Presets[0]
}

// PickPreset chooses a preset for the next clip, avoiding lastKey when more than
// one preset exists so two clips in a row never share a look. Selection is
// deterministic per call input (no Math.random — that is banned in this codebase
// and would also break render-journal resumes): it folds lastKey through FNV and
// indexes the remaining presets, which is enough variety for ~3 clips/day.
func PickPreset(lastKey string) StylePreset {
	if len(Presets) == 1 {
		return Presets[0]
	}
	// Build the candidate list excluding lastKey.
	candidates := make([]StylePreset, 0, len(Presets))
	for _, p := range Presets {
		if p.Key != lastKey {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return Presets[0]
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(lastKey + "|salt-v1"))
	return candidates[int(h.Sum32())%len(candidates)]
}

// BrandCSS renders the :root CSS custom-property block for this preset's palette
// + font + the shared Motion tokens. Var names exactly match those the layout
// template consumes (the template aliases --amber* → --orange*).
func (p StylePreset) BrandCSS() string {
	return p.Palette.cssVars(p.Font)
}

// AsTheme returns a copy of base with the color + image-style fields overridden
// from this preset, so the Scene/Image text agents describe the SAME colors that
// will actually be rendered. base is not mutated.
func (p StylePreset) AsTheme(base *models.BrandTheme) *models.BrandTheme {
	out := *base
	out.PrimaryColor = p.Palette.Navy
	out.SecondaryColor = p.Palette.NavyHi
	out.AccentColor = p.Palette.Orange
	out.FontName = p.Font.Family
	anchor := p.ImageAnchor
	out.ImageStyle = &anchor
	return &out
}
```

- [ ] **Step 4: Add the font-parameterized palette CSS composer to `brand.go`**

In `internal/producer/brand.go`, refactor `CSSVars` to delegate to a private,
font-parameterized `cssVars`, keeping the old public method behavior intact (it
uses the package `Type`):

Replace the `func (b BrandColors) CSSVars() string {` body's opening so it reads:

```go
// CSSVars returns the :root block using the package default font (Type).
// Kept for callers/tests that don't vary the font.
func (b BrandColors) CSSVars() string { return b.cssVars(Type) }

// cssVars renders the :root block for this palette with an explicit font.
func (b BrandColors) cssVars(t TypeTokens) string {
	return fmt.Sprintf(`:root {
```

…and at the bottom of the format-args list change the four `Type.Weight*` /
`Type.Family` references to the parameter `t`:

```go
		t.Family,
		t.WeightRegular, t.WeightSemiBold, t.WeightBold, t.WeightExtraBold,
	)
}
```

(Leave the `--font-family: "%s", sans-serif;` literal in the format string as-is.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/producer/ -run 'Preset|PickPreset|BrandCSS|AsTheme|CSSVars' -v`
Expected: PASS. Then `go build ./...` → no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/producer/presets.go internal/producer/presets_test.go internal/producer/brand.go
git commit -m "feat(producer): style preset type, curated presets, selection + per-font CSS"
```

---

## Task 2: Thread the preset through the render seam

**Files:**
- Modify: `internal/producer/composition_types.go` (ScenesParams)
- Modify: `internal/producer/composition.go:50-114` (RenderCompositionScenes)
- Modify: `internal/producer/brand.go` (buildScenePrompt signature)
- Modify: `internal/producer/producer.go:243,276,298-306,330-332` (Assemble/Produce + image loop + ScenesParams)
- Modify: `internal/orchestrator/orchestrator.go:392` (call site — pass signature for now)
- Test: `internal/producer/composition_scenes_test.go` (add a case)

**Interfaces:**
- Consumes: `StylePreset`, `StylePreset.BrandCSS()`, `PresetByKey` (Task 1).
- Produces:
  - `ScenesParams.Palette BrandColors`, `ScenesParams.BrandCSS string`
  - `func buildScenePrompt(concept, aspect string, preset StylePreset) string`
  - `func (p *Producer) AssembleHyperframes916(ctx, clipID string, scenes []agent.GeneratedScene, preset StylePreset) (string, error)`
  - `func (p *Producer) ProduceHyperframes916(ctx, clipID string, scenes []agent.GeneratedScene, preset StylePreset) (*ProduceResult, error)`

- [ ] **Step 1: Write the failing test** (overlay palette comes from the preset)

```go
// add to internal/producer/composition_scenes_test.go
func TestRenderCompositionScenes_UsesPresetPalette(t *testing.T) {
	preset := PresetByKey("teal-coral")
	params := ScenesParams{
		AspectRatio:     "9:16",
		BrandName:       BrandName,
		BrandCSS:        preset.BrandCSS(),
		Palette:         preset.Palette,
		VoiceSrc:        "assets/voice.wav",
		DurationSeconds: 6,
		Scenes:          []SceneSpec{{Content: SceneContent{}}},
	}
	html, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(html), preset.Palette.Navy) {
		t.Errorf("rendered HTML missing preset navy %q", preset.Palette.Navy)
	}
	if strings.Contains(string(html), Brand.Navy) && preset.Palette.Navy != Brand.Navy {
		t.Errorf("rendered HTML leaked hardcoded Brand navy")
	}
}
```

(If `SceneSpec{Content: SceneContent{}}` is insufficient for a render, copy the
minimal valid `SceneSpec` shape from the existing passing test in this file.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestRenderCompositionScenes_UsesPresetPalette -v`
Expected: FAIL (unknown field `BrandCSS`/`Palette` in ScenesParams).

- [ ] **Step 3: Add fields to `ScenesParams`** (`composition_types.go`, inside the struct)

```go
	// Palette + BrandCSS drive per-clip style presets. When zero/empty,
	// RenderCompositionScenes falls back to the package-global Brand (today's look).
	Palette  BrandColors
	BrandCSS string
```

- [ ] **Step 4: Use them in `RenderCompositionScenes`** (`composition.go`)

Replace line 73's accent fallback and line 99's BrandCSS with preset-aware values.
Near the top of the function (after the `len(p.Scenes)==0` guard) add:

```go
	pal := p.Palette
	if pal == (BrandColors{}) {
		pal = Brand
	}
	brandCSS := p.BrandCSS
	if brandCSS == "" {
		brandCSS = Brand.CSSVars()
	}
```

Then change line ~73:

```go
		sanitizedScenes[i].AccentColor = sanitizeHexColor(sanitizedScenes[i].AccentColor, pal.Orange)
```

and line ~99:

```go
		BrandCSS:        template.CSS(brandCSS),
```

- [ ] **Step 5: Change `buildScenePrompt` to take a preset** (`brand.go`)

```go
func buildScenePrompt(concept, aspect string, preset StylePreset) string {
	subject := strings.TrimSpace(concept)
	if subject == "" {
		subject = genericSceneSubject
	}
	sz := preset.Palette.SafeZone(aspect)
	return preset.ImageAnchor + " " +
		"Subject: " + subject + ". " +
		"Composition: " + sz.NegativeSpace + ". " +
		"Keep the image uncluttered with generous negative space. " +
		"ABSOLUTELY NO text, letters, numbers, words, UI labels, or logos anywhere in the image." +
		" Place the main subject in the UPPER 55% of the frame; keep the LOWER 45% as simple, uncluttered background (a text card is overlaid there)."
}
```

Update the existing `buildScenePrompt` unit test (in `brand_test.go`) to pass
`Presets[0]` and assert the prompt contains `Presets[0].ImageAnchor`.

- [ ] **Step 6: Thread the preset through the Producer** (`producer.go`)

- `AssembleHyperframes916(... scenes []agent.GeneratedScene, preset StylePreset)`; at line ~276 call `buildScenePrompt(s.ImagePrompt, "9:16", preset)`; in the `ScenesParams{...}` literal (line ~298) add `Palette: preset.Palette, BrandCSS: preset.BrandCSS(),`.
- `ProduceHyperframes916(... scenes []agent.GeneratedScene, preset StylePreset)`; pass `preset` into the `AssembleHyperframes916` call at line ~332.

- [ ] **Step 7: Fix the orchestrator call site** (`orchestrator.go:392`) — pass signature for now

```go
	result, err := o.producer.ProduceHyperframes916(ctx, clipID, scenes, producer.PresetByKey("signature"))
```

(Confirm `producer` is already imported in orchestrator.go — it is, via `internal/producer`.)

- [ ] **Step 8: Run tests + build**

Run: `go test ./internal/producer/... && go build ./...`
Expected: PASS / no errors. Existing render-gated tests (`HF_RENDER`) stay skipped.

- [ ] **Step 9: Commit**

```bash
git add internal/producer/ internal/orchestrator/orchestrator.go
git commit -m "feat(producer): thread style preset into image prompt + overlay CSS"
```

---

## Task 3: Persist the chosen preset on the clip

**Files:**
- Create: `migrations/040_clip_style_preset.sql`
- Modify: `internal/models/clip.go` (Clip, CreateClipRequest, UpdateClipRequest)
- Modify: `internal/repository/clips.go` (Create, Update, new `LastStylePreset`)
- Test: none new (covered by build + a repo smoke if a DB harness exists; otherwise compile-only)

**Interfaces:**
- Produces: `func (r *ClipsRepo) LastStylePreset(ctx context.Context) (string, error)` (returns `""` when no clips/null).

- [ ] **Step 1: Write the migration**

```sql
-- 040_clip_style_preset.sql
-- Records which style preset (see internal/producer/presets.go) produced a clip:
-- used for trace/debug and to let the scheduler avoid repeating the previous
-- clip's look. NULL/'' = legacy clips produced before presets existed.
ALTER TABLE clips ADD COLUMN IF NOT EXISTS style_preset TEXT NOT NULL DEFAULT '';
```

- [ ] **Step 2: Verify the migration on a Neon branch** (do NOT touch prod)

Run via the Neon tool on a throwaway branch: `ALTER TABLE clips ADD COLUMN IF NOT EXISTS style_preset TEXT NOT NULL DEFAULT '';` then `SELECT style_preset FROM clips LIMIT 1;` Expected: column exists, empty string. Delete the branch.

- [ ] **Step 3: Add the model field** (`internal/models/clip.go`)

Add `StylePreset string \`json:"style_preset"\`` to the `Clip` struct, and
`StylePreset *string \`json:"style_preset"\`` to `UpdateClipRequest`. (Inspect the
file first; match the existing pointer-vs-value convention used by sibling fields.)

- [ ] **Step 4: Persist on update + add LastStylePreset** (`internal/repository/clips.go`)

In `Update`, include `style_preset` in the dynamic SET when `req.StylePreset != nil`
(follow the exact pattern the method already uses for other optional fields — read
lines 76-103 first). Add:

```go
// LastStylePreset returns the style_preset of the most recently created clip,
// or "" if there are none. Used to avoid repeating a look on the next clip.
func (r *ClipsRepo) LastStylePreset(ctx context.Context) (string, error) {
	var key string
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(style_preset, '') FROM clips ORDER BY created_at DESC LIMIT 1`).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("last style preset: %w", err)
	}
	return key, nil
}
```

(Ensure `errors` and `github.com/jackc/pgx/v5` are imported — check the file header.)

- [ ] **Step 5: Build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add migrations/040_clip_style_preset.sql internal/models/clip.go internal/repository/clips.go
git commit -m "feat(clips): persist style_preset + LastStylePreset lookup"
```

---

## Task 4: Orchestrator — select, persist, and apply the preset per clip

**Files:**
- Modify: `internal/orchestrator/orchestrator.go` (`produceClip`, `produceClipWithID`)
- Test: `internal/orchestrator/orchestrator_test.go` if present; otherwise build-only + a focused unit on the selection helper

**Interfaces:**
- Consumes: `producer.StylePresetsEnabled`, `producer.PickPreset`, `producer.PresetByKey`, `producer.StylePreset`, `StylePreset.AsTheme`, `clipsRepo.LastStylePreset` (Tasks 1, 3).

- [ ] **Step 1: Select the preset in `produceClip`** — before creating the clip

Read `produceClip` (orchestrator.go:228-246). Add at the top:

```go
	preset := producer.PresetByKey("signature")
	if producer.StylePresetsEnabled() {
		last, _ := o.clipsRepo.LastStylePreset(ctx) // best-effort; "" → no avoid
		preset = producer.PickPreset(last)
	}
```

Set `style_preset` when creating the clip: add `StylePreset: preset.Key` to the
`models.CreateClipRequest` (add that field to the request struct + `Create` INSERT
in clips.go, mirroring Task 3's pattern) OR persist immediately after create via
`o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{StylePreset: &preset.Key})`.
Prefer the Update path to avoid widening `CreateClipRequest`.

Then pass `preset` into `produceClipWithID(ctx, clip.ID, q, theme, preset, scriptCfg, imageCfg, ...)`.

- [ ] **Step 2: Apply the preset inside `produceClipWithID`**

Change the signature to accept `preset producer.StylePreset`. Where the per-clip
theme is used for the text agents, derive it from the preset so the AI describes the
rendered colors:

```go
	clipTheme := theme
	if producer.StylePresetsEnabled() {
		clipTheme = preset.AsTheme(theme)
	}
```

Use `clipTheme` in the `o.sceneAgent.Generate(... clipTheme ...)` call (line ~312)
and in any ImageAgent call in this path. Replace the `ProduceHyperframes916(ctx,
clipID, scenes, producer.PresetByKey("signature"))` from Task 2 with `... scenes,
preset)`.

- [ ] **Step 3: Build + run orchestrator/producer tests**

Run: `go build ./... && go test ./internal/orchestrator/... ./internal/producer/...`
Expected: PASS / no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/repository/clips.go internal/models/clip.go
git commit -m "feat(orchestrator): pick + apply a style preset per clip behind flag"
```

---

## Task 5 (gated, ship last): Alternate Thai overlay fonts

> This task is the highest-risk part (Thai glyph rendering + template edits). It is
> independent of Tasks 1-4: until it lands, every preset's `Font` is `Type`
> (Sarabun) and the system already varies color + art style. Ship Tasks 1-4 first;
> do this only after a render of Tasks 1-4 looks right.

**Files:**
- Add: `internal/producer/assets/fonts/Kanit-*.ttf`, `Prompt-*.ttf` (Regular/SemiBold/Bold/ExtraBold each) — Google Fonts, SIL OFL, Thai-complete.
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (lines 8-21)
- Modify: `internal/producer/presets.go` (assign fonts to some presets)

- [ ] **Step 1: Add @font-face for the new families + use the var in body**

In the template `<style>` head, after the Sarabun `@font-face` block, add the same
four-weight block for `"Kanit"` and `"Prompt"` pointing at `assets/fonts/Kanit-*.ttf`
/ `Prompt-*.ttf`. Change the body rule (line ~21) from the literal
`font-family:"Sarabun",sans-serif` to `font-family:var(--font-family),sans-serif`.

- [ ] **Step 2: Assign fonts to presets** (`presets.go`)

Set `Font:` on 1-2 presets to `TypeTokens{Family: "Kanit", WeightRegular:400, WeightSemiBold:600, WeightBold:700, WeightExtraBold:800}` (and similarly `"Prompt"`). Leave `signature` on `Type` (Sarabun).

- [ ] **Step 3: Render-verify each font** (the gate)

For each preset with a non-Sarabun font, render one clip locally and visually confirm
Thai vowels/tone marks render correctly and nothing clips. Use the existing
render-gated harness:

Run: `HF_RENDER=1 go test ./internal/producer/ -run TestAssembleHyperframes916_Smoke -v`
(plus a manual eyeball of the output mp4/frames). Expected: PASS + legible Thai.
If a font misrenders Thai, revert that preset's `Font` to `Type` and drop the font.

- [ ] **Step 4: Commit**

```bash
git add internal/producer/assets/fonts/ internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/presets.go
git commit -m "feat(producer): alternate Thai overlay fonts per preset (verified)"
```

---

## Known limitation (not a task)

A few amber-specific glow shadows in the template are hardcoded `rgba(255,180,84,…)`
(e.g. `.stat`, bullet dots). They won't recolor with the palette, so non-amber presets
keep a faint warm glow on those elements. Cosmetic only; defer unless it reads wrong in
the render review.

---

## Rollout (after all tasks)

1. Deploy with `STYLE_PRESETS_ENABLED` unset → no visible change; migration 040 applies.
2. Render-test each preset (Task 5 gate covers fonts; eyeball colors/art for the rest).
3. Set `STYLE_PRESETS_ENABLED=true` on Railway → watch the first day's 3 clips.
4. Rollback = unset the flag (no redeploy of code needed if set via Railway var).
