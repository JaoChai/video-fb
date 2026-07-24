package agent

import "testing"

func TestStripEmoji(t *testing.T) {
	cases := map[string]string{
		"ขั้น 1 📞 โทร ✅": "ขั้น 1  โทร ",
		"2026 ปี":        "2026 ปี",
		"Pay Now 💳":      "Pay Now ",
		"no emoji here":  "no emoji here",
		"ขั้น 1 • โทร":   "ขั้น 1  โทร",
	}
	for in, want := range cases {
		if got := StripEmoji(in); got != want {
			t.Errorf("StripEmoji(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClampLayout(t *testing.T) {
	for _, v := range []string{"hook", "hero", "stat", "step", "tip", "cta",
		"casefile", "comic", "evidence", "board", "verdict"} {
		if ClampLayout(v) != v {
			t.Errorf("ClampLayout(%q) changed a valid layout", v)
		}
	}
	if ClampLayout("banana") != "hero" {
		t.Errorf("unknown layout must clamp to hero")
	}
	if ClampLayout("") != "hero" {
		t.Errorf("empty layout must clamp to hero")
	}
}

func TestTruncateRunes_underLimitUnchanged(t *testing.T) {
	if got := TruncateRunes("สั้น", 14); got != "สั้น" {
		t.Errorf("want unchanged, got %q", got)
	}
}

func TestTruncateRunes_cutsToLimit(t *testing.T) {
	in := "กกกกกกกกกกกกกกกกกกกก" // 20 runes
	got := TruncateRunes(in, 14)
	if r := []rune(got); len(r) > 14 {
		t.Errorf("want <=14 runes, got %d (%q)", len(r), got)
	}
}
