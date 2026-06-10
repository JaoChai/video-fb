package agent

import (
	"strings"
	"unicode"
)

// decorative is a small denylist of bullet/star glyphs the LLM loved emitting in
// the old broken output; content now supplies its own bullet styling.
const decorative = "•‣◦▪▸●★☆◆"

var sceneLayouts = map[string]bool{"hook": true, "hero": true, "stat": true, "step": true, "tip": true, "cta": true}

// ClampLayout maps an LLM layout value to a supported one; unknown -> "hero".
func ClampLayout(v string) string {
	if sceneLayouts[v] {
		return v
	}
	return "hero"
}

// StripEmoji removes emoji / pictographic runes the bundled Sarabun font cannot
// render (they become tofu boxes). Thai, Latin, digits, and ASCII punctuation stay.
func StripEmoji(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if strings.ContainsRune(decorative, r) {
			continue
		}
		if r >= 0x1F000 || ((unicode.Is(unicode.So, r) || unicode.Is(unicode.Sk, r)) && r >= 0x2100) {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
