package publisher

import "testing"

func TestCleanTikTokHook(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"strips brand tag", "3 สัญญาณบัญชีกำลังพัง | Ads Vance", "3 สัญญาณบัญชีกำลังพัง"},
		{"keeps non-brand pipe", "ทางเลือก A | B ต่างกันยังไง", "ทางเลือก A | B ต่างกันยังไง"},
		{"no tag untouched", "หยุดก่อน อย่ากดอุทธรณ์ซ้ำ", "หยุดก่อน อย่ากดอุทธรณ์ซ้ำ"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cleanTikTokHook(c.in); got != c.want {
				t.Errorf("cleanTikTokHook(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestCleanTikTokHookCaps(t *testing.T) {
	long := make([]rune, 200)
	for i := range long {
		long[i] = 'ก'
	}
	got := []rune(cleanTikTokHook(string(long)))
	if len(got) > 120 {
		t.Errorf("caption not capped: got %d runes, want <= 120", len(got))
	}
}

func TestBuildTikTokCaption(t *testing.T) {
	// Hashtags appended; brand tag stripped; no off-platform description leaks in.
	got := buildTikTokCaption("เคสบัญชีโดนแบน | Ads Vance", "#ยิงแอด #เฟสแบน")
	want := "เคสบัญชีโดนแบน\n\n#ยิงแอด #เฟสแบน"
	if got != want {
		t.Errorf("buildTikTokCaption = %q, want %q", got, want)
	}

	// No hashtags configured → just the hook, no trailing separator.
	if got := buildTikTokCaption("เคสบัญชีโดนแบน", "  "); got != "เคสบัญชีโดนแบน" {
		t.Errorf("buildTikTokCaption with blank hashtags = %q, want %q", got, "เคสบัญชีโดนแบน")
	}
}
