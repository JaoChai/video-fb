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
	for _, v := range []string{"hook", "hero", "stat", "step", "tip", "cta"} {
		if ClampLayout(v) != v {
			t.Errorf("ClampLayout(%q) changed a valid layout", v)
		}
	}
	if ClampLayout("banana") != "hero" {
		t.Error("unknown should -> hero")
	}
	if ClampLayout("") != "hero" {
		t.Error("empty should -> hero")
	}
}
