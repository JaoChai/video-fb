package producer

import (
	"strings"
	"testing"
)

// TestBrandColors verifies the canonical brand color values are correct.
func TestBrandColors(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"NavyDeep", Brand.NavyDeep, "#0a1428"},
		{"Navy", Brand.Navy, "#0f1d35"},
		{"NavyHi", Brand.NavyHi, "#16284a"},
		{"Orange", Brand.Orange, "#ff6b2b"},
		{"OrangeSoft", Brand.OrangeSoft, "#ff8a52"},
		{"OrangeBright", Brand.OrangeBright, "#ff9457"},
		{"Ink", Brand.Ink, "#f4f7fb"},
		{"Muted", Brand.Muted, "#aebdd4"},
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
// and contains the canonical navy and orange hex codes.
func TestImageStyleAnchor(t *testing.T) {
	first := Brand.ImageStyleAnchor()
	second := Brand.ImageStyleAnchor()

	if first == "" {
		t.Fatal("ImageStyleAnchor() returned empty string")
	}
	if first != second {
		t.Errorf("ImageStyleAnchor() is not deterministic:\n  call 1: %q\n  call 2: %q", first, second)
	}
	if !strings.Contains(first, "#0a1428") {
		t.Errorf("ImageStyleAnchor() missing navy hex #0a1428: %q", first)
	}
	if !strings.Contains(first, "#ff6b2b") {
		t.Errorf("ImageStyleAnchor() missing orange hex #ff6b2b: %q", first)
	}
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
