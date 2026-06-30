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
