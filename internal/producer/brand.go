package producer

import "strings"

// BrandColors is the single source of truth for the ADS VANCE visual palette,
// scoped to image-prompt generation. Hex values are taken directly from the
// layout templates (layout_dynamic_karaoke.html.tmpl and
// layout_multi_scene.html.tmpl) and must stay in sync with them.
type BrandColors struct {
	// Background / structural navy scale
	NavyDeep string // #0a1428 — page background, deepest layer
	Navy     string // #0f1d35 — card/panel fill
	NavyHi   string // #16284a — highlighted/elevated navy surface

	// Primary accent — orange
	Orange       string // #ff6b2b — primary CTA, badge, accent text
	OrangeSoft   string // #ff8a52 — gradient midpoint, step number bg
	OrangeBright string // #ff9457 — progress bar, highlight text

	// Text tones
	Ink   string // #f4f7fb — primary body text on dark
	Muted string // #aebdd4 — secondary / caption text

	// Semantic status colors
	Warn string // #ff5a52 — cause / danger (red)
	Win  string // #2fd17a — positive / win (green)
	Info string // #3b82f6 — informational (blue)
}

// Brand is the canonical, exported ADS VANCE brand token set.
// All image-prompt generation should reference this value.
var Brand = BrandColors{
	NavyDeep: "#0a1428",
	Navy:     "#0f1d35",
	NavyHi:   "#16284a",

	Orange:       "#ff6b2b",
	OrangeSoft:   "#ff8a52",
	OrangeBright: "#ff9457",

	Ink:   "#f4f7fb",
	Muted: "#aebdd4",

	Warn: "#ff5a52",
	Win:  "#2fd17a",
	Info: "#3b82f6",
}

// SafeZoneSpec describes the screen regions for a given aspect ratio: which
// band text overlays occupy, and which area the image should keep clear.
// These are human-readable descriptions embedded directly into image-generation
// prompts so the model understands where to avoid placing focal content.
type SafeZoneSpec struct {
	Aspect        string // "9:16" or "16:9"
	TextBand      string // where on-screen text will sit
	NegativeSpace string // the region the image should keep clear for text
}

// SafeZone returns the safe-zone spec for a given aspect ratio.
// Supported values: "9:16" (portrait/Reels) and "16:9" (landscape/YouTube).
// The returned TextBand and NegativeSpace strings are designed to be embedded
// verbatim into image-generation prompts.
func (b BrandColors) SafeZone(aspect string) SafeZoneSpec {
	switch aspect {
	case "9:16":
		return SafeZoneSpec{
			Aspect:   "9:16",
			TextBand: "lower third of the frame — karaoke captions and brand badges sit in the bottom 35% of the portrait frame",
			NegativeSpace: "keep the upper two-thirds of the image free of text, logos, or busy detail; " +
				"the lower third will be overlaid with semi-transparent caption and card elements",
		}
	case "16:9":
		return SafeZoneSpec{
			Aspect:   "16:9",
			TextBand: "lower band of the frame — captions and title overlay sit in the bottom 20% of the landscape frame",
			NegativeSpace: "keep the upper four-fifths of the image free of text or logos; " +
				"the bottom 20% will be overlaid with semi-transparent caption elements",
		}
	default:
		// Unknown aspect: return a permissive fallback so callers do not panic.
		return SafeZoneSpec{
			Aspect:        aspect,
			TextBand:      "lower portion of the frame",
			NegativeSpace: "keep the lower third of the image free of important detail for text overlay",
		}
	}
}

// ImageStyleAnchor returns a locked, deterministic style-anchor paragraph for
// AI image-generation prompts. Prepend this to every scene prompt so all clips
// share a single cohesive visual identity.
//
// The string is constant — same output on every call — and intentionally
// includes the literal navy (#0a1428) and orange (#ff6b2b) hex codes so the
// image model honours the exact palette.
func (b BrandColors) ImageStyleAnchor() string {
	return "Flat modern editorial illustration style with soft cinematic lighting. " +
		"Strict two-tone palette: deep navy #0a1428 as the dominant background and structural color, " +
		"vibrant orange #ff6b2b as the single accent for highlights, glows, and focal points. " +
		"No other saturated hues. Clean vector-quality rendering, minimal grain, no photorealism. " +
		"Subtle radial glow from the top-center, gentle vignette at the edges. " +
		"Atmosphere: confident, modern, premium digital-marketing brand identity."
}

// genericSceneSubject is the fallback subject used when buildScenePrompt
// receives an empty or whitespace-only concept.
const genericSceneSubject = "abstract modern digital-marketing concept art"

// buildScenePrompt composes a complete AI image-generation prompt from three
// locked blocks:
//
//  1. Style anchor — Brand.ImageStyleAnchor(), shared across all scenes so every
//     clip has a cohesive visual identity.
//  2. Subject — the caller-supplied concept (e.g. "a Facebook Ads Manager
//     dashboard showing a rising conversion graph"). Falls back to
//     genericSceneSubject when concept is empty or whitespace.
//  3. Composition — instructs the image model to preserve the safe zone for text
//     overlay and produce absolutely no text, letters, or logos in the image.
//
// The function is deterministic: same (concept, aspect) always yields the same
// string. It is placed in brand.go because it is purely brand-prompt composition,
// building on ImageStyleAnchor and SafeZone which live here.
func buildScenePrompt(concept, aspect string) string {
	subject := strings.TrimSpace(concept)
	if subject == "" {
		subject = genericSceneSubject
	}

	sz := Brand.SafeZone(aspect)

	return Brand.ImageStyleAnchor() + " " +
		"Subject: " + subject + ". " +
		"Composition: " + sz.NegativeSpace + ". " +
		"Keep the image uncluttered with generous negative space. " +
		"ABSOLUTELY NO text, letters, numbers, words, UI labels, or logos anywhere in the image."
}
