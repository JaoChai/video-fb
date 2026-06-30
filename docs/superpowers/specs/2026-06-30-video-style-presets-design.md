# Video Style Presets — Per-Clip Visual Variety

**Date:** 2026-06-30
**Status:** Approved design, pre-implementation
**Scope:** Phase 1 only (color palette + AI art style + overlay font). Phase 2 (layout
variants) is outlined but deferred.

## Problem

Every clip looks identical. The system renders the same royal-blue + amber theme, the
same flat-editorial AI art style, and the same Sarabun overlay font on every clip. With
production now at 3 clips/day, the monotony is more visible and the channel feels
templated.

### Root cause (verified in code)

The `brand_themes` DB table exists and feeds the active theme into the **text** agents
(SceneAgent, ImageAgent) — but the **rendered look is hardcoded** and overrides it:

| Visible layer | Controlled by | Status |
|---|---|---|
| AI image style (palette/tone) | `buildScenePrompt` → `Brand.ImageStyleAnchor()` | hardcoded royal-blue+amber, `producer.go:276` |
| Overlay colors/font/motion (caption, card, badge) | `Brand.CSSVars()` | hardcoded, `composition.go:99` |
| Scene layout / structure | single `layout_multi_scene.html.tmpl` | one template only |

Both render seams reference the package-global `Brand` (a `BrandColors` value in
`internal/producer/brand.go`). `ProduceHyperframes916` does not receive a theme at all.

## Goal

Each clip automatically picks one of a **curated set of complete style presets**, so
consecutive clips look visibly different while staying recognizably "Ads Vance". Quality
is controlled by curating cohesive presets (not by randomly mixing dimensions).

Decisions locked during brainstorming:
- **Auto-vary per clip** (not manual re-theme).
- **Curated presets**, not independent per-dimension randomization, not AI-generated palettes.
- **Phased**: Phase 1 = palette + art style + font. Phase 2 = layout variants (deferred).
- **Presets defined in Go** (approach A), not in DB (B) and not AI-generated (C).
- **Feature-flagged** for safe rollout.

## Architecture (Phase 1)

### New unit: `internal/producer/presets.go`

```go
type StylePreset struct {
    Key         string        // stable id, stored on the clip (e.g. "signature", "teal-coral")
    DisplayName string        // human label for logs/admin
    Palette     BrandColors   // full palette → drives CSSVars() (overlay) + image anchor colors
    ImageAnchor string        // art-style paragraph whose colors match Palette
    Font        TypeTokens    // overlay font family + weights (Thai-capable)
}

var Presets []StylePreset      // 4–6 curated presets
```

- The current royal-blue+amber look becomes the preset `signature` — it is also the
  **fallback** whenever selection can't run, so "flag off" or "no data" reproduces today's
  look exactly.
- Additional presets (~4) each carry a cohesive palette **and** a matching `ImageAnchor`
  art-style paragraph (e.g. teal+coral / deep-purple+gold / charcoal+electric-blue /
  warm-sunset). Each preset is internally color-matched so overlay and AI image agree.

### Brand-constant anchors (the recognizability thread)

These stay identical across **all** presets so the channel still reads as one brand:
- Leopard mascot (intro/outro).
- Brand name `ADS VANCE` (`BrandName`).
- CTA copy (`BrandCTA`).

So clips vary in color, art style, and font — never in mascot or brand identity.

### Selection: `PickPreset(lastKey string) StylePreset`

- Random choice that **avoids the immediately-previous clip's preset**, so two clips in a
  row never share a look. With 3 clips/day, "avoid last" is enough perceived variety
  without a heavier scheduler.
- Deterministic, table-driven, unit-testable. If `Presets` is empty or only `signature`
  exists, it returns `signature` (never panics).

### Threading the preset into the render

1. New column `style_preset TEXT` on `clips` — records the chosen preset key for
   traceability and for the avoid-repeat lookup.
2. Orchestrator (`produceClip` loop): pick the preset (reading the last clip's
   `style_preset` to avoid a repeat), persist it on the clip, and pass it down.
3. `ProduceHyperframes916(ctx, clipID, scenes, preset)` gains a `preset` param:
   - `buildScenePrompt(s.ImagePrompt, "9:16", preset)` uses `preset.ImageAnchor` + palette
     instead of the global `Brand.ImageStyleAnchor()`.
   - composition `BrandCSS` uses `preset.Palette.CSSVars()` instead of `Brand.CSSVars()`.
4. **Color coherence:** feed the chosen preset's colors into SceneAgent/ImageAgent too
   (replacing the static DB-theme colors for that clip), so the colors the AI is told to
   draw match the overlay palette actually rendered. Today they can mismatch.

### Feature flag

- `STYLE_PRESETS_ENABLED` (env). Off ⇒ always `signature` ⇒ pixel-identical to today.
- On ⇒ per-clip `PickPreset`. Lets us render-test on prod for one cycle before committing,
  consistent with the project's cautious visual-rollback history.

## Data flow

```
produceClip loop
  └─ lastKey ← clipsRepo.LastStylePreset()        (avoid-repeat)
  └─ preset  ← PickPreset(lastKey)  [flag on]  | signature  [flag off]
  └─ clipsRepo: persist style_preset = preset.Key
  └─ SceneAgent/ImageAgent ← preset colors        (coherent AI prompt)
  └─ ProduceHyperframes916(..., preset)
        ├─ buildScenePrompt(..., preset)   → preset.ImageAnchor + palette
        └─ composition BrandCSS            → preset.Palette.CSSVars()
```

## Error handling

- Selection failure / empty preset list → `signature` fallback. Production never breaks.
- Missing font file → existing CSS fallback (`"<family>", sans-serif` in `CSSVars`).
- Image-gen still governed by the existing circuit breaker + credit gate — unchanged.

## Fonts (the flagged risk)

Varying the overlay font requires bundling additional **Thai-capable** fonts (e.g. Kanit,
Prompt) under `internal/producer/templates/fonts/`, with matching `@font-face` blocks. The
template already switches family via the `--font-family` CSS var, so no template logic
changes — only assets + per-preset `Font`.

**Verify-before-use:** each candidate font must render Thai vowels/tone marks correctly in
a real render before it ships in a preset. Start with **2–3 verified fonts**; presets
without a verified alternative font reuse Sarabun. Font variety must never regress Thai
legibility.

## Testing

- Unit: `PickPreset` avoids `lastKey` and returns a valid preset; empty/singleton lists
  fall back to `signature`.
- Unit: every preset's `Palette.CSSVars()` emits all CSS var names the template references
  (guards the template-sync invariant for all presets, not just `signature`).
- Unit: `buildScenePrompt` includes the preset's `ImageAnchor` and palette colors.
- Render smoke test: render one clip per preset locally and eyeball it (colors coherent,
  Thai text legible, no layout breakage) before enabling the flag on prod.

## Rollout

1. Ship behind `STYLE_PRESETS_ENABLED=false` (no visible change).
2. Render-test each preset locally; fix any color/font issues.
3. Enable the flag; watch the first day's 3 clips; rollback = flip the flag off.

## Phase 2 (deferred): Layout variants

Add 2–3 alternate `layout_*.html.tmpl` templates with structurally different scene
compositions (card placement, motion language), and extend selection to pick a layout per
clip. Heavier and higher render-risk (each template needs its own render testing, à la the
past `protocolTimeout`/CPU-contention incidents), so it ships only after Phase 1 is proven
on prod. Out of scope for this spec.
