package producer

import (
	"strings"
	"testing"
)

// TestBuildScenePrompt covers the three-block contract for buildScenePrompt.
func TestBuildScenePrompt(t *testing.T) {
	preset := Presets[0] // signature preset — identical to the former hardcoded Brand look
	anchor := preset.ImageAnchor
	sz916 := preset.Palette.SafeZone("9:16")
	sz169 := preset.Palette.SafeZone("16:9")

	t.Run("contains style anchor", func(t *testing.T) {
		out := buildScenePrompt("a rising conversion graph dashboard", "9:16", preset)
		if !strings.Contains(out, anchor) {
			t.Errorf("output missing style anchor\ngot: %q", out)
		}
	})

	t.Run("contains preset ImageAnchor", func(t *testing.T) {
		out := buildScenePrompt("a rising conversion graph dashboard", "9:16", preset)
		if !strings.Contains(out, Presets[0].ImageAnchor) {
			t.Errorf("output missing Presets[0].ImageAnchor\ngot: %q", out)
		}
	})

	t.Run("contains concept subject", func(t *testing.T) {
		concept := "a Facebook Ads Manager dashboard showing a rising conversion graph"
		out := buildScenePrompt(concept, "9:16", preset)
		if !strings.Contains(out, concept) {
			t.Errorf("output missing concept %q\ngot: %q", concept, out)
		}
	})

	t.Run("contains no-text instruction", func(t *testing.T) {
		out := buildScenePrompt("a vibrant cityscape at dusk", "16:9", preset)
		lower := strings.ToLower(out)
		if !strings.Contains(lower, "no text") {
			t.Errorf("output missing no-text instruction\ngot: %q", out)
		}
	})

	t.Run("contains negative space for 9:16", func(t *testing.T) {
		out := buildScenePrompt("concept art", "9:16", preset)
		if !strings.Contains(out, sz916.NegativeSpace) {
			t.Errorf("output missing 9:16 NegativeSpace\ngot: %q", out)
		}
	})

	t.Run("contains negative space for 16:9", func(t *testing.T) {
		out := buildScenePrompt("concept art", "16:9", preset)
		if !strings.Contains(out, sz169.NegativeSpace) {
			t.Errorf("output missing 16:9 NegativeSpace\ngot: %q", out)
		}
	})

	t.Run("aspect negative space differs by ratio", func(t *testing.T) {
		out916 := buildScenePrompt("concept art", "9:16", preset)
		out169 := buildScenePrompt("concept art", "16:9", preset)
		if out916 == out169 {
			t.Error("9:16 and 16:9 outputs are identical; negative-space block should differ")
		}
	})

	t.Run("deterministic same concept and aspect", func(t *testing.T) {
		concept := "entrepreneur checking analytics on laptop"
		a := buildScenePrompt(concept, "9:16", preset)
		b := buildScenePrompt(concept, "9:16", preset)
		if a != b {
			t.Errorf("buildScenePrompt is not deterministic\ncall1: %q\ncall2: %q", a, b)
		}
	})

	t.Run("empty concept falls back to generic subject", func(t *testing.T) {
		out := buildScenePrompt("", "9:16", preset)
		if out == "" {
			t.Fatal("output is empty for empty concept")
		}
		if !strings.Contains(out, anchor) {
			t.Errorf("fallback output missing style anchor\ngot: %q", out)
		}
		// Must not contain a bare "Subject: ." or "Subject: " at end — i.e. the
		// subject field must be filled with a real fallback value.
		if strings.Contains(out, "Subject: .") || strings.Contains(out, "Subject:  .") {
			t.Errorf("fallback subject appears empty in output\ngot: %q", out)
		}
		// Pin the actual fallback contract: the generic subject must be present.
		if !strings.Contains(out, "abstract modern digital-marketing concept art") {
			t.Errorf("fallback output missing generic subject\ngot: %q", out)
		}
	})

	t.Run("whitespace-only concept falls back to generic subject", func(t *testing.T) {
		out := buildScenePrompt("   ", "9:16", preset)
		if out == "" {
			t.Fatal("output is empty for whitespace concept")
		}
		if strings.Contains(out, "Subject: .") || strings.Contains(out, "Subject:  .") {
			t.Errorf("whitespace concept produced empty subject\ngot: %q", out)
		}
		// Pin the actual fallback contract: the generic subject must be present.
		if !strings.Contains(out, "abstract modern digital-marketing concept art") {
			t.Errorf("whitespace concept output missing generic subject\ngot: %q", out)
		}
	})
}

// TestBrandColors verifies the canonical brand color values are correct.
func TestBrandColors(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"NavyDeep", Brand.NavyDeep, "#062F78"},
		{"Navy", Brand.Navy, "#0047AF"},
		{"NavyHi", Brand.NavyHi, "#1A5FD0"},
		{"Orange", Brand.Orange, "#F0A030"},
		{"OrangeSoft", Brand.OrangeSoft, "#E8A030"},
		{"OrangeBright", Brand.OrangeBright, "#FFB454"},
		{"Ink", Brand.Ink, "#F6F9FF"},
		{"Muted", Brand.Muted, "#BCD2FF"},
		{"Warn", Brand.Warn, "#ff5a52"},
		{"Win", Brand.Win, "#2fd17a"},
		{"Info", Brand.Info, "#3b82f6"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("Brand.%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

// TestImageStyleAnchor verifies the style anchor is deterministic, non-empty,
// and contains the canonical royal-blue and amber hex codes.
func TestImageStyleAnchor(t *testing.T) {
	first := Brand.ImageStyleAnchor()
	second := Brand.ImageStyleAnchor()

	if first == "" {
		t.Fatal("ImageStyleAnchor() returned empty string")
	}
	if first != second {
		t.Errorf("ImageStyleAnchor() is not deterministic:\n  call 1: %q\n  call 2: %q", first, second)
	}
	if !strings.Contains(first, "#0047AF") {
		t.Errorf("ImageStyleAnchor() missing royal-blue hex #0047AF: %q", first)
	}
	if !strings.Contains(first, "#F0A030") {
		t.Errorf("ImageStyleAnchor() missing amber hex #F0A030: %q", first)
	}
}

// TestMotionTokens verifies the canonical Motion token values are correct.
func TestMotionTokens(t *testing.T) {
	t.Run("easing curves are non-empty cubic-bezier strings", func(t *testing.T) {
		for name, val := range map[string]string{
			"EaseOut":    Motion.EaseOut,
			"EaseInOut":  Motion.EaseInOut,
			"EaseIn":     Motion.EaseIn,
			"EaseSpring": Motion.EaseSpring,
		} {
			if val == "" {
				t.Errorf("Motion.%s is empty", name)
			}
			if !strings.HasPrefix(val, "cubic-bezier(") {
				t.Errorf("Motion.%s = %q; want cubic-bezier(...) format", name, val)
			}
		}
	})

	t.Run("canonical easing values", func(t *testing.T) {
		if Motion.EaseOut != "cubic-bezier(0.22,1,0.36,1)" {
			t.Errorf("Motion.EaseOut = %q", Motion.EaseOut)
		}
		if Motion.EaseInOut != "cubic-bezier(0.45,0,0.55,1)" {
			t.Errorf("Motion.EaseInOut = %q", Motion.EaseInOut)
		}
		if Motion.EaseIn != "cubic-bezier(0.55,0,1,0.45)" {
			t.Errorf("Motion.EaseIn = %q", Motion.EaseIn)
		}
		if Motion.EaseSpring != "cubic-bezier(0.34,1.56,0.64,1)" {
			t.Errorf("Motion.EaseSpring = %q", Motion.EaseSpring)
		}
	})

	t.Run("canonical duration values", func(t *testing.T) {
		if Motion.DurFast != 0.32 {
			t.Errorf("Motion.DurFast = %v, want 0.32", Motion.DurFast)
		}
		if Motion.DurNormal != 0.60 {
			t.Errorf("Motion.DurNormal = %v, want 0.60", Motion.DurNormal)
		}
		if Motion.DurSlow != 0.90 {
			t.Errorf("Motion.DurSlow = %v, want 0.90", Motion.DurSlow)
		}
	})

	t.Run("durations increase fast < normal < slow", func(t *testing.T) {
		if !(Motion.DurFast < Motion.DurNormal && Motion.DurNormal < Motion.DurSlow) {
			t.Errorf("expected DurFast < DurNormal < DurSlow, got %v %v %v",
				Motion.DurFast, Motion.DurNormal, Motion.DurSlow)
		}
	})
}

// TestTypeTokens verifies the canonical Type token values are correct.
func TestTypeTokens(t *testing.T) {
	if Type.Family != "Sarabun" {
		t.Errorf("Type.Family = %q, want %q", Type.Family, "Sarabun")
	}
	weights := map[string]int{
		"Regular":   Type.WeightRegular,
		"SemiBold":  Type.WeightSemiBold,
		"Bold":      Type.WeightBold,
		"ExtraBold": Type.WeightExtraBold,
	}
	wantWeights := map[string]int{
		"Regular":   400,
		"SemiBold":  600,
		"Bold":      700,
		"ExtraBold": 800,
	}
	for name, got := range weights {
		if got != wantWeights[name] {
			t.Errorf("Type.Weight%s = %d, want %d", name, got, wantWeights[name])
		}
	}
}

// TestCSSVars verifies the CSSVars() output is deterministic, contains all
// required color var names with correct hex values, and includes motion and
// type vars.
func TestCSSVars(t *testing.T) {
	css := Brand.CSSVars()

	t.Run("deterministic", func(t *testing.T) {
		second := Brand.CSSVars()
		if css != second {
			t.Error("CSSVars() is not deterministic")
		}
	})

	t.Run("non-empty", func(t *testing.T) {
		if css == "" {
			t.Fatal("CSSVars() returned empty string")
		}
	})

	// Color vars — names and values must exactly match the templates.
	colorVars := []struct{ varName, hex string }{
		{"--navy-deep", "#062F78"},
		{"--navy", "#0047AF"},
		{"--navy-hi", "#1A5FD0"},
		{"--orange", "#F0A030"},
		{"--orange-soft", "#E8A030"},
		{"--orange-bright", "#FFB454"},
		{"--ink", "#F6F9FF"},
		{"--muted", "#BCD2FF"},
		{"--green", "#2fd17a"},
		{"--red", "#ff5a52"},
	}
	for _, cv := range colorVars {
		t.Run("color "+cv.varName, func(t *testing.T) {
			want := cv.varName + ": " + cv.hex
			if !strings.Contains(css, want) {
				t.Errorf("CSSVars() missing %q\noutput:\n%s", want, css)
			}
		})
	}

	// Motion vars must be present.
	motionVars := []string{
		"--ease-out:",
		"--ease-in-out:",
		"--ease-in:",
		"--ease-spring:",
		"--dur-fast:",
		"--dur-normal:",
		"--dur-slow:",
	}
	for _, mv := range motionVars {
		t.Run("motion "+mv, func(t *testing.T) {
			if !strings.Contains(css, mv) {
				t.Errorf("CSSVars() missing motion var %q", mv)
			}
		})
	}

	// Type vars must be present.
	typeVars := []string{
		"--font-family:",
		"--font-weight-regular:",
		"--font-weight-semibold:",
		"--font-weight-bold:",
		"--font-weight-extrabold:",
	}
	for _, tv := range typeVars {
		t.Run("type "+tv, func(t *testing.T) {
			if !strings.Contains(css, tv) {
				t.Errorf("CSSVars() missing type var %q", tv)
			}
		})
	}

	// Sarabun font family must appear in the output.
	t.Run("font family Sarabun", func(t *testing.T) {
		if !strings.Contains(css, "Sarabun") {
			t.Errorf("CSSVars() missing Sarabun font family")
		}
	})

	// Duration values must use the 's' unit suffix.
	t.Run("durations have s unit", func(t *testing.T) {
		if !strings.Contains(css, "--dur-fast: 0.32s") {
			t.Errorf("CSSVars() --dur-fast missing or wrong value")
		}
		if !strings.Contains(css, "--dur-normal: 0.6s") {
			t.Errorf("CSSVars() --dur-normal missing or wrong value")
		}
		if !strings.Contains(css, "--dur-slow: 0.9s") {
			t.Errorf("CSSVars() --dur-slow missing or wrong value")
		}
	})
}

// TestSafeZone verifies SafeZone returns non-empty, sensible output for both
// supported aspect ratios and a permissive non-empty fallback for unknown ratios.
func TestSafeZone(t *testing.T) {
	t.Run("portrait 9:16", func(t *testing.T) {
		sz := Brand.SafeZone("9:16")
		if sz.TextBand == "" {
			t.Error("SafeZone(9:16).TextBand is empty")
		}
		if sz.NegativeSpace == "" {
			t.Error("SafeZone(9:16).NegativeSpace is empty")
		}
		if sz.Aspect != "9:16" {
			t.Errorf("SafeZone(9:16).Aspect = %q, want %q", sz.Aspect, "9:16")
		}
	})

	t.Run("landscape 16:9", func(t *testing.T) {
		sz := Brand.SafeZone("16:9")
		if sz.TextBand == "" {
			t.Error("SafeZone(16:9).TextBand is empty")
		}
		if sz.NegativeSpace == "" {
			t.Error("SafeZone(16:9).NegativeSpace is empty")
		}
		if sz.Aspect != "16:9" {
			t.Errorf("SafeZone(16:9).Aspect = %q, want %q", sz.Aspect, "16:9")
		}
	})

	t.Run("portrait and landscape descriptions differ", func(t *testing.T) {
		p := Brand.SafeZone("9:16")
		l := Brand.SafeZone("16:9")
		if p.TextBand == l.TextBand && p.NegativeSpace == l.NegativeSpace {
			t.Error("SafeZone returned identical descriptions for 9:16 and 16:9")
		}
	})

	t.Run("unknown aspect falls back permissively", func(t *testing.T) {
		sz := Brand.SafeZone("1:1")
		if sz.Aspect != "1:1" {
			t.Errorf("SafeZone(1:1).Aspect = %q, want %q (echoed back)", sz.Aspect, "1:1")
		}
		if sz.TextBand == "" {
			t.Error("SafeZone(1:1).TextBand is empty; fallback must be non-empty")
		}
		if sz.NegativeSpace == "" {
			t.Error("SafeZone(1:1).NegativeSpace is empty; fallback must be non-empty")
		}
	})
}

func TestCSSVars_EmitsHeadingFontAndMotionProfile(t *testing.T) {
	// A theme with a distinct heading font + snappy motion must surface those as
	// CSS custom properties the template can consume.
	tt := TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit",
		WeightRegular: 400, WeightSemiBold: 600, WeightBold: 700, WeightExtraBold: 800}
	css := Brand.cssVars(tt)
	for _, want := range []string{`--font-heading: "Kanit"`, "--font-family:"} {
		if !strings.Contains(css, want) {
			t.Errorf("cssVars missing %q\n%s", want, css)
		}
	}
}

func TestMotionProfile_Default(t *testing.T) {
	if MotionDefault.EntranceDur <= 0 || MotionDefault.EntranceEase == "" || MotionDefault.BGZoomTo < 1.0 {
		t.Errorf("MotionDefault has invalid zero values: %+v", MotionDefault)
	}
}
