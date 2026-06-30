// internal/producer/presets.go
package producer

import (
	"math/rand"
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

// PickPreset chooses a preset at random for the next clip, excluding lastKey when
// more than one preset exists so two clips in a row never share a look. Real
// randomness (math/rand, auto-seeded on Go 1.20+) is correct here: this is
// server-side Go, NOT the hyperframes render JS where non-determinism is banned —
// the orchestrator already uses time.Now(). Random (not hash-deterministic)
// selection ensures all presets get used over time instead of settling into a cycle.
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
	return candidates[rand.Intn(len(candidates))]
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
