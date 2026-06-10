package producer

import (
	"fmt"
	"strings"
)

// BrandName is the channel brand name shown in badges/bumpers.
const BrandName = "ADS VANCE"

// BrandCTA is the outro call-to-action copy.
const BrandCTA = "กดติดตาม ADS VANCE ไม่พลาดเรื่องแอด"

// BrandColors is the single source of truth for the ADS VANCE visual palette,
// scoped to image-prompt generation. Hex values are taken directly from the
// layout_multi_scene.html.tmpl template and must stay in sync with it.
type BrandColors struct {
	// Background / structural navy scale
	NavyDeep string // #062F78 — page background, deepest layer
	Navy     string // #0047AF — card/panel fill
	NavyHi   string // #1A5FD0 — highlighted/elevated navy surface

	// Primary accent — amber
	Orange       string // #F0A030 — primary CTA, badge, accent text
	OrangeSoft   string // #E8A030 — gradient midpoint, step number bg
	OrangeBright string // #FFB454 — progress bar, highlight text

	// Text tones
	Ink   string // #F6F9FF — primary body text on dark
	Muted string // #BCD2FF — secondary / caption text

	// Semantic status colors
	Warn string // #ff5a52 — cause / danger (red)
	Win  string // #2fd17a — positive / win (green)
	Info string // #3b82f6 — informational (blue)
}

// Brand is the canonical, exported ADS VANCE brand token set.
// All image-prompt generation should reference this value.
var Brand = BrandColors{
	NavyDeep: "#062F78",
	Navy:     "#0047AF",
	NavyHi:   "#1A5FD0",

	Orange:       "#F0A030",
	OrangeSoft:   "#E8A030",
	OrangeBright: "#FFB454",

	Ink:   "#F6F9FF",
	Muted: "#BCD2FF",

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
			NegativeSpace: "compose the main subject toward the top of the frame or out to the side margins; " +
				"keep the vertical center and the lower 35% calm and uncluttered, because a large centered " +
				"headline and bottom captions are overlaid there",
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
		"Strict two-tone palette: vivid royal blue #0047AF as the dominant background and structural color, " +
		"warm amber gold #F0A030 as the single accent for highlights, glows, and focal points. " +
		"No other saturated hues. Clean vector-quality rendering, minimal grain, no photorealism. " +
		"Subtle radial glow from the top-center, gentle vignette at the edges. " +
		"Atmosphere: confident, modern, premium digital-marketing brand identity."
}

// ── Motion tokens ────────────────────────────────────────────────────────────

// MotionTokens holds the brand animation language: named easing curves (as CSS
// cubic-bezier() strings) and standard durations in seconds. All values match
// the eases and timings already used in the layout templates.
type MotionTokens struct {
	// Easing curves — use in CSS transition/animation and JS (GSAP) ease strings.
	EaseOut    string // cubic-bezier(0.22,1,0.36,1) — smooth deceleration (power3.out feel)
	EaseInOut  string // cubic-bezier(0.45,0,0.55,1) — symmetric ease in/out
	EaseIn     string // cubic-bezier(0.55,0,1,0.45) — acceleration into a cut
	EaseSpring string // cubic-bezier(0.34,1.56,0.64,1) — slight overshoot (back.out(1.3) feel)

	// Standard durations in seconds — use for CSS var values and JS constants.
	DurFast   float64 // 0.32 s — micro-interactions, caption pops
	DurNormal float64 // 0.60 s — default element entrance/exit
	DurSlow   float64 // 0.90 s — hero reveals, large block entrances
}

// Motion is the canonical ADS VANCE motion token set.
// Values are derived from the eases and durations used in the layout templates.
var Motion = MotionTokens{
	EaseOut:    "cubic-bezier(0.22,1,0.36,1)",
	EaseInOut:  "cubic-bezier(0.45,0,0.55,1)",
	EaseIn:     "cubic-bezier(0.55,0,1,0.45)",
	EaseSpring: "cubic-bezier(0.34,1.56,0.64,1)",

	DurFast:   0.32,
	DurNormal: 0.60,
	DurSlow:   0.90,
}

// ── Type tokens ──────────────────────────────────────────────────────────────

// TypeTokens holds the brand typography system: font family and the four
// weights loaded by the layout templates. No size scale is included here —
// sizes are layout-specific and live in the templates.
type TypeTokens struct {
	// Family is the primary font family string for CSS font-family declarations.
	Family string // "Sarabun"

	// Weight constants match the four @font-face declarations in both templates.
	WeightRegular   int // 400
	WeightSemiBold  int // 600
	WeightBold      int // 700
	WeightExtraBold int // 800
}

// Type is the canonical ADS VANCE type token set.
// Font is Sarabun (loaded locally via @font-face in both layout templates).
var Type = TypeTokens{
	Family:          "Sarabun",
	WeightRegular:   400,
	WeightSemiBold:  600,
	WeightBold:      700,
	WeightExtraBold: 800,
}

// ── CSSVars ───────────────────────────────────────────────────────────────────

// CSSVars returns a CSS :root block of custom properties for all brand color,
// motion, and type tokens.
//
// Color var names EXACTLY match the names referenced in
// layout_multi_scene.html.tmpl (which injects this block via {{ .BrandCSS }}),
// so the template's colors stay in sync with this single source of truth.
//
// Motion and type vars are additive (prefixed --ease-*, --dur-*, --font-*).
//
// The output is deterministic; it contains no newline at the end of the block.
func (b BrandColors) CSSVars() string {
	return fmt.Sprintf(`:root {
  /* ── Brand colors (navy scale) ── */
  --navy-deep: %s;
  --navy: %s;
  --navy-hi: %s;

  /* ── Brand colors (orange family) ── */
  --orange: %s;
  --orange-soft: %s;
  --orange-bright: %s;

  /* ── Brand colors (text) ── */
  --ink: %s;
  --muted: %s;

  /* ── Brand colors (semantic) ── */
  --green: %s;
  --red: %s;

  /* ── Motion easings ── */
  --ease-out: %s;
  --ease-in-out: %s;
  --ease-in: %s;
  --ease-spring: %s;

  /* ── Motion durations (seconds) ── */
  --dur-fast: %ss;
  --dur-normal: %ss;
  --dur-slow: %ss;

  /* ── Typography ── */
  --font-family: "%s", sans-serif;
  --font-weight-regular: %d;
  --font-weight-semibold: %d;
  --font-weight-bold: %d;
  --font-weight-extrabold: %d;
}`,
		b.NavyDeep, b.Navy, b.NavyHi,
		b.Orange, b.OrangeSoft, b.OrangeBright,
		b.Ink, b.Muted,
		b.Win, b.Warn,
		Motion.EaseOut, Motion.EaseInOut, Motion.EaseIn, Motion.EaseSpring,
		formatDur(Motion.DurFast), formatDur(Motion.DurNormal), formatDur(Motion.DurSlow),
		Type.Family,
		Type.WeightRegular, Type.WeightSemiBold, Type.WeightBold, Type.WeightExtraBold,
	)
}

// formatDur formats a duration float as a compact decimal string without
// trailing zeros, suitable for use in CSS values (e.g. 0.32, 0.6, 0.9).
func formatDur(d float64) string {
	s := fmt.Sprintf("%.2f", d)
	// Trim trailing zeros after decimal point, but keep at least one digit.
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
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
		"ABSOLUTELY NO text, letters, numbers, words, UI labels, or logos anywhere in the image." +
		" Place the main subject in the UPPER 55% of the frame; keep the LOWER 45% as simple, uncluttered background (a text card is overlaid there)."
}
