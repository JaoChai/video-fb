// internal/producer/presets.go
package producer

import (
	"github.com/jaochai/video-fb/internal/models"
)

// StylePreset is one complete, internally-cohesive visual identity: a full color
// palette, a matching AI art-style anchor, and an overlay font. Every clip is
// pinned to one of the two formats' presets (CaseFilePreset / TutorialPreset) by
// its content_format; the leopard mascot, brand name, and CTA stay constant
// across both (see producer.BrandName / BrandCTA) so the channel reads as one brand.
//
// The four rotating design themes and their random/retention-weighted pickers
// were removed once both formats pinned their own identity — nothing had chosen
// a rotating theme since the case-file format went live.
type StylePreset struct {
	Key         string      // stable id persisted on the clip
	DisplayName string      // human label for logs/admin
	Palette     BrandColors // overlay + image colors
	ImageAnchor string      // art-style paragraph; its colors MUST match Palette
	Font        TypeTokens  // overlay font (Thai-capable)

	HeadingFont TypeTokens    // display font for headlines; zero ⇒ use Font
	Motion      MotionProfile // per-format entrance/ken-burns feel
}

// DefaultWindowDays is the lookback for per-preset retention reporting.
const DefaultWindowDays = 30

// PresetByKey returns the preset with key, falling back to CaseFilePreset for an
// unknown or empty key — the case-file look is the channel default, so a clip
// whose stored key predates the two-format world still renders a valid identity.
// Never panics.
func PresetByKey(key string) StylePreset {
	switch key {
	case TutorialPreset.Key:
		return TutorialPreset
	case ChatPreset.Key:
		return ChatPreset
	}
	return CaseFilePreset
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
