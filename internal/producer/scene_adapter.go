package producer

import (
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

// buildSceneContent maps a GeneratedScene + its measured audio bound into the
// structured SceneContent the Style-B template renders. Phase-1 stub: emits a
// minimal "hero" with the on-screen text as the title. A later phase replaces
// this with per-layout structuring (stat/step/rows/chips/cta).
func buildSceneContent(s agent.GeneratedScene, b sceneBound) SceneContent {
	return SceneContent{
		SceneNumber: s.SceneNumber, Start: b.Start, End: b.End,
		Layout: "hero", CaptionStyle: normalizeCaptionStyle(s.CaptionStyle),
		Title: highlightTitleStr(strings.TrimSpace(s.OnScreenText), s.EmphasisWords),
	}
}
