// internal/producer/presets_test.go
package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

// allPresets is every visual identity the system can render: one per format.
// The rotating theme pool is gone — a preset that is not in this list cannot be
// reached by any code path.
func allPresets() []StylePreset { return []StylePreset{CaseFilePreset, TutorialPreset} }

func TestPresets_UniqueKeysValidHexAndAnchors(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range allPresets() {
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

// Brand invariant: both formats keep navy+orange. They differ by media, font,
// and motion — never by palette.
func TestPresets_ShareBrandPaletteAndHaveHeadingFontAndMotion(t *testing.T) {
	for _, p := range allPresets() {
		if p.Palette != Brand {
			t.Errorf("preset %q palette drifts from Brand (violates brand invariant)", p.Key)
		}
		if p.HeadingFont.HeadingFamily == "" {
			t.Errorf("preset %q missing HeadingFont.HeadingFamily", p.Key)
		}
		if p.Motion.EntranceEase == "" || p.Motion.BGZoomTo < 1.0 {
			t.Errorf("preset %q has invalid Motion %+v", p.Key, p.Motion)
		}
	}
}

func TestPresetByKey_ResolvesBothFormats(t *testing.T) {
	if PresetByKey("tutorial").Key != "tutorial" {
		t.Error("tutorial key must resolve to the tutorial preset")
	}
	if PresetByKey("case-file").Key != "case-file" {
		t.Error("case-file key must resolve to the case-file preset")
	}
}

// A clip persisted before the two-format world stores a theme key that no longer
// exists. Retry/resume must still render it, as the channel default.
func TestPresetByKey_UnknownAndEmptyFallBackToCaseFile(t *testing.T) {
	for _, key := range []string{"", "does-not-exist", "editorial-bold", "neon-techno"} {
		if got := PresetByKey(key); got.Key != "case-file" {
			t.Errorf("PresetByKey(%q) = %q, want case-file", key, got.Key)
		}
	}
}

func TestBrandCSS_ContainsPaletteAndFont(t *testing.T) {
	p := PresetByKey("case-file")
	css := p.BrandCSS()
	for _, want := range []string{"--navy-deep", "--orange", "--orange-bright", "--ink", "--muted", "--red", p.Palette.Navy, p.Font.Family} {
		if !strings.Contains(css, want) {
			t.Errorf("BrandCSS missing %q", want)
		}
	}
}

func TestAsTheme_OverridesColorsFromPreset(t *testing.T) {
	base := &models.BrandTheme{PrimaryColor: "x", AccentColor: "y", Name: "Base"}
	p := TutorialPreset
	got := p.AsTheme(base)
	if got.PrimaryColor != p.Palette.Navy || got.AccentColor != p.Palette.Orange {
		t.Error("AsTheme must override primary/accent from the preset palette")
	}
	if base.PrimaryColor != "x" {
		t.Error("AsTheme must not mutate the base theme")
	}
}
