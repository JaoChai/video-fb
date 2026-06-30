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

func TestPickWeighted_ExploitPicksHighestEligible(t *testing.T) {
	// rng(100) -> 50 (>=30, no explore); no second rng call expected on exploit.
	rng := scriptedRng(t, 50)
	scores := []models.PresetScore{
		{Preset: "teal-coral", AvgRetention: 0.40, N: 5},
		{Preset: "purple-gold", AvgRetention: 0.60, N: 5}, // highest eligible
		{Preset: "charcoal-electric", AvgRetention: 0.90, N: 1}, // N<min, ignored for exploit
	}
	got := PickPresetWeighted("signature", scores, 0.30, 3, rng)
	if got.Key != "purple-gold" {
		t.Fatalf("exploit picked %q, want purple-gold", got.Key)
	}
}

func TestPickWeighted_ExploreRollUsesUniform(t *testing.T) {
	// rng(100) -> 10 (<30 explore); rng(len(candidates)) -> 0 -> first candidate.
	rng := scriptedRng(t, 10, 0)
	scores := []models.PresetScore{{Preset: "purple-gold", AvgRetention: 0.9, N: 9}}
	got := PickPresetWeighted("signature", scores, 0.30, 3, rng)
	candidates := candidateKeysExcluding("signature")
	if got.Key != candidates[0] {
		t.Fatalf("explore picked %q, want first candidate %q", got.Key, candidates[0])
	}
}

func TestPickWeighted_NoEligibleFallsBackToUniform(t *testing.T) {
	// All N<minClips → no exploit target → uniform even on a no-explore roll.
	// rng(100) -> 99 (no explore), then rng(len) -> 0.
	rng := scriptedRng(t, 99, 0)
	scores := []models.PresetScore{{Preset: "purple-gold", AvgRetention: 0.9, N: 1}}
	got := PickPresetWeighted("signature", scores, 0.30, 3, rng)
	if got.Key == "signature" {
		t.Fatal("must not return lastKey")
	}
	if PresetByKey(got.Key).Key != got.Key {
		t.Fatalf("returned unknown preset %q", got.Key)
	}
}

func TestPickWeighted_EmptyScoresUniform(t *testing.T) {
	rng := scriptedRng(t, 99, 1)
	got := PickPresetWeighted("signature", nil, 0.30, 3, rng)
	if got.Key == "signature" {
		t.Fatal("must not return lastKey")
	}
}

func TestPickWeighted_NeverReturnsLastKey(t *testing.T) {
	scores := []models.PresetScore{{Preset: "signature", AvgRetention: 0.99, N: 99}}
	// signature scores highest but is lastKey → excluded; no other candidate has scores,
	// so bestIdx == -1 → uniform path: rng(100) coin then rng(len) for the pick.
	got := PickPresetWeighted("signature", scores, 0.0, 1, scriptedRng(t, 50, 0))
	if got.Key == "signature" {
		t.Fatal("returned the avoided lastKey")
	}
}

// scriptedRng returns a deterministic rng(n) that yields the given values in order,
// clamped into [0,n). Fails the test if called more times than scripted.
func scriptedRng(t *testing.T, vals ...int) func(int) int {
	t.Helper()
	i := 0
	return func(n int) int {
		if i >= len(vals) {
			t.Fatalf("rng called more than the %d scripted times", len(vals))
		}
		v := vals[i]
		i++
		if n <= 0 {
			return 0
		}
		return v % n
	}
}

// candidateKeysExcluding mirrors PickPresetWeighted's candidate ordering: all
// Presets in declared order except lastKey.
func candidateKeysExcluding(lastKey string) []string {
	var ks []string
	for _, p := range Presets {
		if p.Key != lastKey {
			ks = append(ks, p.Key)
		}
	}
	return ks
}
