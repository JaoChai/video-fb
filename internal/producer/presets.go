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

	HeadingFont TypeTokens    // display font for headlines; zero ⇒ use Font
	Motion      MotionProfile // per-theme entrance/ken-burns feel
}

// Presets is the curated set of design themes. Presets[0] is "editorial-bold" —
// it equals today's hardcoded Brand look and is the universal fallback, so a
// disabled flag or any selection failure reproduces the current output exactly.
// All themes share Palette: Brand — navy+orange is the brand invariant; themes
// differ only by ImageAnchor (art media), HeadingFont, and Motion.
var Presets = []StylePreset{
	{
		Key:         "editorial-bold",
		DisplayName: "Editorial Bold",
		Palette:     Brand,
		ImageAnchor: "Flat modern editorial illustration, premium and clean, with soft cinematic lighting. " +
			"Strict two-tone palette: vivid royal blue #0047AF as the dominant background/structural color, " +
			"warm amber gold #F0A030 as the single accent for highlights and focal points. No other saturated hues. " +
			"Crisp vector-quality shapes, confident composition, subtle top-center glow, gentle edge vignette, minimal grain. " +
			"No photorealism, no 3D render, no text. Atmosphere: confident, authoritative, premium digital-marketing brand.",
		Font:        Type,
		HeadingFont: TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
		Motion:      MotionProfile{EntranceDur: 0.60, EntranceEase: "power3.out", BGZoomTo: 1.10},
	},
	{
		Key:         "cinematic-photo",
		DisplayName: "Cinematic Photo",
		Palette:     Brand,
		ImageAnchor: "Cinematic editorial PHOTOGRAPHY, shot on 85mm f/1.4, natural window light, shallow depth of field, " +
			"warm filmic color grade with a subtle deep-navy #0047AF duotone wash in the shadows and warm amber #F0A030 highlights. " +
			"Real-world settings (modern office, hands on a laptop showing an ads dashboard, banknotes, people), photorealistic, " +
			"premium and trustworthy. NO illustration, NO 3D render, NO cartoon, NO flat vector, no text. " +
			"Atmosphere: credible, premium, real digital-marketing business.",
		Font:        TypeTokens{Family: "IBM Plex Sans Thai", HeadingFamily: "Kanit"},
		HeadingFont: TypeTokens{Family: "IBM Plex Sans Thai", HeadingFamily: "Kanit"},
		Motion:      MotionProfile{EntranceDur: 0.70, EntranceEase: "power2.out", BGZoomTo: 1.12},
	},
	{
		Key:         "neon-techno",
		DisplayName: "Neon Techno HUD",
		Palette:     Brand,
		ImageAnchor: "Sleek techno HUD illustration on a dark deep-navy #062F78 background, crisp neon line-art and thin glowing strokes, " +
			"glassmorphism panels, data/graph/ring motifs. Electric-blue glow accents with warm amber #F0A030 as the single focal accent. " +
			"High-tech, sharp, clean vector rendering, subtle scanline glow. No photorealism, no text. " +
			"Atmosphere: high-energy, data-driven, premium digital-marketing brand.",
		Font:        TypeTokens{Family: "Prompt", HeadingFamily: "Prompt"},
		HeadingFont: TypeTokens{Family: "Prompt", HeadingFamily: "Prompt"},
		Motion:      MotionProfile{EntranceDur: 0.34, EntranceEase: "power4.out", BGZoomTo: 1.05},
	},
	{
		Key:         "soft-3d-clay",
		DisplayName: "Soft 3D Clay",
		Palette:     Brand,
		ImageAnchor: "Soft 3D clay-render illustration, rounded matte shapes with gentle soft studio shadows, tactile and friendly. " +
			"Palette anchored to brand royal blue #0047AF with warm amber #F0A030 as the single accent; warm approachable mood. " +
			"Claymorphism, smooth surfaces, no harsh edges. No photorealism, no text. " +
			"Atmosphere: friendly, approachable, premium digital-marketing brand.",
		Font:        TypeTokens{Family: "Prompt", HeadingFamily: "Kanit"},
		HeadingFont: TypeTokens{Family: "Prompt", HeadingFamily: "Kanit"},
		Motion:      MotionProfile{EntranceDur: 0.48, EntranceEase: "back.out(1.6)", BGZoomTo: 1.08},
	},
}

// Performance-weighted selection tuning (spec defaults; callers pass them in).
const (
	DefaultEpsilon    = 0.30
	DefaultMinClips   = 3
	DefaultWindowDays = 30
)

// StylePresetsEnabled reports whether per-clip preset selection is on. Off ⇒
// callers use the editorial-bold preset, reproducing today's exact look.
func StylePresetsEnabled() bool { return os.Getenv("STYLE_PRESETS_ENABLED") == "true" }

// StylePresetsPerformanceEnabled reports whether per-clip selection should be biased
// by measured retention (epsilon-greedy). Requires STYLE_PRESETS_ENABLED as well;
// off ⇒ uniform avoid-last selection.
func StylePresetsPerformanceEnabled() bool {
	return os.Getenv("STYLE_PRESETS_PERFORMANCE_ENABLED") == "true"
}

// PickPresetWeighted chooses a preset with epsilon-greedy over retention scores,
// always excluding lastKey (the avoid-last rule). rng(n) must return an int in [0,n);
// pass rand.Intn in production. rng is consumed in a fixed order: first rng(100) for
// the explore/exploit coin, then — only when exploring or when no eligible exploit
// target exists — rng(len(candidates)) for the uniform pick.
//
//   - explore (prob epsilon) OR no candidate with N >= minClips: uniform among candidates.
//   - exploit: the candidate with the highest AvgRetention among those with N >= minClips.
//
// Candidates with N < minClips are "unknown": never chosen by exploit, only reachable
// via an explore roll — so new/under-sampled presets never starve.
func PickPresetWeighted(lastKey string, scores []models.PresetScore, epsilon float64, minClips int, rng func(int) int) StylePreset {
	candidates := make([]StylePreset, 0, len(Presets))
	for _, p := range Presets {
		if p.Key != lastKey {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return Presets[0]
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	scoreByKey := make(map[string]models.PresetScore, len(scores))
	for _, s := range scores {
		scoreByKey[s.Preset] = s
	}

	bestIdx := -1
	for i := range candidates {
		s, ok := scoreByKey[candidates[i].Key]
		if !ok || s.N < minClips {
			continue
		}
		if bestIdx == -1 || s.AvgRetention > scoreByKey[candidates[bestIdx].Key].AvgRetention {
			bestIdx = i
		}
	}

	explore := rng(100) < int(epsilon*100)
	if bestIdx == -1 || explore {
		return candidates[rng(len(candidates))]
	}
	return candidates[bestIdx]
}

// PresetByKey returns the preset with key, or the editorial-bold preset (Presets[0])
// when key is unknown/empty. Resolves the out-of-pool case-file preset too, so a
// retried case clip keeps its visual identity. Never panics.
func PresetByKey(key string) StylePreset {
	if key == CaseFilePreset.Key {
		return CaseFilePreset
	}
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
	font := p.Font
	if p.HeadingFont.HeadingFamily != "" {
		font.HeadingFamily = p.HeadingFont.HeadingFamily
	}
	return p.Palette.cssVars(font)
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
