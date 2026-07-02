package producer

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/jaochai/video-fb/internal/agent"
)

// templateSceneLayouts is the set of layout_variant values the multi-scene
// template (layout_multi_scene.html.tmpl) actually styles as scene layouts.
var templateSceneLayouts = map[string]bool{
	"hook_big": true, "hook_punch": true, "list_steps": true,
	"stat_reveal": true, "compare_two": true, "quote_cta": true,
}

// normalizeLayout maps a SceneAgent layout_variant to a template-supported scene
// layout. The SceneAgent's seeded enum also includes caption styles
// (phrase_block, word_pop) and bumper names (static, intro, outro) that are not
// scene layouts; those — and any unknown value — fall back to hook_big (the
// template's default centered column).
func normalizeLayout(v string) string {
	if templateSceneLayouts[v] {
		return v
	}
	return "hook_big"
}

// normalizeCaptionStyle clamps to the two styles the template's caption driver
// understands; anything else becomes phrase_block.
func normalizeCaptionStyle(s string) string {
	if s == "word_pop" {
		return "word_pop"
	}
	return "phrase_block"
}

// buildSceneSpecs maps the SceneAgent's GeneratedScene[] plus the measured
// per-scene audio bounds into render-ready []SceneSpec for the multi-scene
// template. Each scene becomes a single headline slot built from on_screen_text
// + emphasis_words (the SceneAgent emits one on-screen line per scene), with the
// layout/caption normalized, the brand accent, and an image/css background mode
// keyed off whether the scene has an image_prompt.
//
// scenes and bounds are index-matched; the shorter slice wins so a short LLM
// response never panics. Returns nil when either is empty.
func buildSceneSpecs(scenes []agent.GeneratedScene, bounds []sceneBound) []SceneSpec {
	n := len(scenes)
	if nb := len(bounds); nb < n {
		n = nb
	}
	if n == 0 {
		return nil
	}

	specs := make([]SceneSpec, n)
	for i := 0; i < n; i++ {
		s := scenes[i]
		b := bounds[i]

		bgMode := "css"
		if strings.TrimSpace(s.ImagePrompt) != "" {
			bgMode = "image"
		}

		var slots []SlotSpec
		if txt := strings.TrimSpace(s.OnScreenText); txt != "" {
			slots = []SlotSpec{{Role: "headline", HTML: highlightTitle(txt, s.EmphasisWords)}}
		}

		specs[i] = SceneSpec{
			SceneNumber:    s.SceneNumber,
			LayoutVariant:  normalizeLayout(s.LayoutVariant),
			AccentColor:    Brand.Orange,
			AnimationSpeed: "normal",
			StartSec:       b.Start,
			EndSec:         b.End,
			BackgroundMode: bgMode,
			CaptionStyle:   normalizeCaptionStyle(s.CaptionStyle),
			Slots:          slots,
			Content:        buildSceneContent(s, b),
		}
	}
	return specs
}

// highlightTitleStr wraps emphasis words in <span class="acc"> (the Style-B
// accent class) while escaping the rest. Returns a plain string so it can be
// JSON-serialized into SceneContent.Title (the template injects it via innerHTML).
func highlightTitleStr(title string, words []string) string {
	return highlightWithClass(title, words, "acc")
}

// speedForLayout picks an entrance pacing per scene layout so a clip doesn't
// enter every scene at one uniform tempo: hook teasers snap in fast, headline
// and stat reveals ease in slow/premium, the rest stay normal. The template
// multiplies its per-theme ENTRANCE_DUR by the factor this maps to.
func speedForLayout(layout string) string {
	switch layout {
	case "hook":
		return "fast"
	case "hero", "stat":
		return "slow"
	default:
		return "normal"
	}
}

// buildSceneContent maps a GeneratedScene + its measured audio bound into the
// structured SceneContent the Style-B template renders. It clamps the layout,
// unmarshals the model's per-layout content object, and strips emoji from every
// text field. When the model emits no structured content it falls back to a hero
// title built from on_screen_text + emphasis_words, so a scene is never blank.
func buildSceneContent(s agent.GeneratedScene, b sceneBound) SceneContent {
	c := SceneContent{
		SceneNumber:  s.SceneNumber,
		Start:        b.Start,
		End:          b.End,
		Layout:       agent.ClampLayout(s.Layout),
		CaptionStyle: normalizeCaptionStyle(s.CaptionStyle),
	}
	var raw struct {
		Kicker, Title, Sub, Stat, Unit, StatLabel, Num, Of, Pill, CTA, Brand string
		Rows                                                                 []struct {
			T   string `json:"t"`
			Bad bool   `json:"bad"`
		} `json:"rows"`
		Chips []struct {
			N string `json:"n"`
			T string `json:"t"`
		} `json:"chips"`
	}
	if len(s.Content) > 0 {
		if err := json.Unmarshal(s.Content, &raw); err != nil {
			log.Printf("scene %d: malformed content JSON (%v) — degrading to hero", s.SceneNumber, err)
		}
	}
	clean := agent.StripEmoji
	c.Kicker, c.Sub = clean(raw.Kicker), clean(raw.Sub)
	c.Title = clean(raw.Title) // may legitimately contain <span class="acc">…</span>
	c.Stat, c.Unit, c.StatLabel = clean(raw.Stat), clean(raw.Unit), clean(raw.StatLabel)
	c.Num, c.Of, c.Pill = clean(raw.Num), clean(raw.Of), clean(raw.Pill)
	c.CTA, c.Brand = clean(raw.CTA), clean(raw.Brand)
	for _, r := range raw.Rows {
		if t := clean(r.T); t != "" {
			c.Rows = append(c.Rows, ContentRow{Text: t, Bad: r.Bad})
		}
	}
	for _, ch := range raw.Chips {
		c.Chips = append(c.Chips, ContentChip{N: clean(ch.N), T: clean(ch.T)})
	}
	// Fallback: if the model gave no structured content, render a hero title from
	// the legacy on_screen_text + emphasis_words so the scene is never blank.
	empty := c.Title == "" && len(c.Rows) == 0 && c.Stat == "" && c.CTA == "" &&
		len(c.Chips) == 0 && c.Pill == "" && c.Sub == "" && c.StatLabel == ""
	if empty {
		log.Printf("scene %d: no structured content (layout %q) — hero fallback from on_screen_text", s.SceneNumber, s.Layout)
		c.Layout = "hero"
		c.Title = highlightTitleStr(clean(strings.TrimSpace(s.OnScreenText)), s.EmphasisWords)
	}
	// Derive after the hero fallback so speed follows the final layout.
	c.Speed = speedForLayout(c.Layout)
	return c
}
