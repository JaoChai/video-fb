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
		specs[i].Content.Entrance = entranceForScene(i)
	}
	// StepTotal is derived after the loop because a scene cannot know how many
	// siblings share its layout. Go owns this value so the progress rail never
	// depends on parsing the model's Thai "ขั้นที่ n / N" string.
	stepTotal := 0
	for i := range specs {
		if specs[i].Content.Layout == "uistep" {
			stepTotal++
		}
	}
	for i := range specs {
		if specs[i].Content.Layout == "uistep" {
			specs[i].Content.StepTotal = stepTotal
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
	case "hook", "casefile":
		return "fast"
	case "hero", "stat", "evidence", "verdict":
		return "slow"
	default:
		return "normal"
	}
}

var entranceVariants = []string{"punch", "rise", "slide"}

// entranceForScene picks a rotating entrance geometry (punch/rise/slide) so
// consecutive scenes never enter identically. Index-based, so a render is
// deterministic. Scene 0 (the hook) gets "punch" for a snappy open. idx is a
// non-negative loop index, so a plain modulo is enough.
func entranceForScene(idx int) string {
	return entranceVariants[idx%3]
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
		Stamp                                                                string
		Rows                                                                 []struct {
			T   string `json:"t"`
			Bad bool   `json:"bad"`
		} `json:"rows"`
		Chips []struct {
			N string `json:"n"`
			T string `json:"t"`
		} `json:"chips"`
		Panels []struct {
			Time  string `json:"time"`
			T     string `json:"t"`
			Quote string `json:"quote"`
			Dark  bool   `json:"dark"`
		} `json:"panels"`
		Callout string `json:"callout"`
		Panel   *struct {
			Chrome     string `json:"chrome"`
			Breadcrumb string `json:"breadcrumb"`
			Items      []struct {
				Label string `json:"label"`
				State string `json:"state"`
			} `json:"items"`
			Field *struct {
				Label string `json:"label"`
				Value string `json:"value"`
			} `json:"field"`
		} `json:"panel"`
	}
	if len(s.Content) > 0 {
		if err := json.Unmarshal(s.Content, &raw); err != nil {
			log.Printf("scene %d: malformed content JSON (%v) — degrading to hero", s.SceneNumber, err)
		}
	}
	clean := agent.StripEmoji
	c.Kicker = clean(raw.Kicker)
	c.Sub = agent.TruncateRunes(clean(raw.Sub), 50)
	c.Title = clean(raw.Title) // may legitimately contain <span class="acc">…</span>
	if c.Layout == "uistep" {
		c.Title = agent.TruncateRunes(c.Title, 34)
	}
	c.Stat, c.Unit = clean(raw.Stat), clean(raw.Unit)
	c.StatLabel = agent.TruncateRunes(clean(raw.StatLabel), 28)
	c.Num, c.Of = clean(raw.Num), clean(raw.Of)
	c.Pill = agent.TruncateRunes(clean(raw.Pill), 16)
	c.CTA = agent.TruncateRunes(clean(raw.CTA), 14)
	c.Brand = clean(raw.Brand)
	for _, r := range raw.Rows {
		if t := agent.TruncateRunes(clean(r.T), 36); t != "" {
			c.Rows = append(c.Rows, ContentRow{Text: t, Bad: r.Bad})
		}
	}
	for _, ch := range raw.Chips {
		c.Chips = append(c.Chips, ContentChip{N: clean(ch.N), T: clean(ch.T)})
	}
	c.Stamp = agent.TruncateRunes(clean(raw.Stamp), 18)
	// tutorial uistep: the simulated screen. Item labels are NOT truncated here —
	// they must stay byte-comparable against the catalog's ui_vocab, which the
	// render gate already validated. Only Thai copy gets clamped.
	c.Callout = agent.TruncateRunes(clean(raw.Callout), 60)
	if raw.Panel != nil {
		p := &ContentUIPanel{
			Chrome:     clean(raw.Panel.Chrome),
			Breadcrumb: clean(raw.Panel.Breadcrumb),
		}
		for _, it := range raw.Panel.Items {
			if len(p.Items) >= 5 { // เกิน 5 แถวล้นกรอบ 9:16
				break
			}
			if lb := strings.TrimSpace(clean(it.Label)); lb != "" {
				p.Items = append(p.Items, ContentUIItem{Label: lb, State: agent.ClampUIState(it.State)})
			}
		}
		// A field with no value is worse than no field: it renders an empty
		// highlighted box that usually just repeats the target row's label and
		// teaches the viewer nothing. Only keep the field when there is an actual
		// value to type or read.
		if raw.Panel.Field != nil {
			if val := strings.TrimSpace(clean(raw.Panel.Field.Value)); val != "" {
				p.Field = &ContentUIField{Label: clean(raw.Panel.Field.Label), Value: val}
			}
		}
		c.Panel = p
	}
	// Casefile poster headline: the scene's on_screen_text (hook line) rendered
	// big above the folder, with emphasis words amber-highlighted. Go-derived —
	// no prompt/schema change needed.
	if c.Layout == "casefile" {
		c.Hook = highlightTitleStr(clean(strings.TrimSpace(s.OnScreenText)), s.EmphasisWords)
	}
	for _, pn := range raw.Panels {
		if len(c.Panels) >= 3 { // template จัดวางได้สูงสุด 3 ช่องโดยไม่ล้นเฟรม
			break
		}
		if t := agent.TruncateRunes(clean(pn.T), 36); t != "" {
			c.Panels = append(c.Panels, ContentPanel{
				Time:  agent.TruncateRunes(clean(pn.Time), 12),
				T:     t,
				Quote: agent.TruncateRunes(clean(pn.Quote), 44),
				Dark:  pn.Dark,
			})
		}
	}
	// Fallback: if the model gave no structured content, render a hero title from
	// the legacy on_screen_text + emphasis_words so the scene is never blank.
	empty := c.Title == "" && len(c.Rows) == 0 && c.Stat == "" && c.CTA == "" &&
		len(c.Chips) == 0 && c.Pill == "" && c.Sub == "" && c.StatLabel == "" &&
		c.Stamp == "" && len(c.Panels) == 0 && c.Panel == nil && c.Callout == ""
	if empty {
		log.Printf("scene %d: no structured content (layout %q) — hero fallback from on_screen_text", s.SceneNumber, s.Layout)
		c.Layout = "hero"
		c.Title = highlightTitleStr(clean(strings.TrimSpace(s.OnScreenText)), s.EmphasisWords)
	}
	// Derive after the hero fallback so speed follows the final layout.
	c.Speed = speedForLayout(c.Layout)
	guardSceneContent(&c)
	return c
}

// guardSceneContent ประกบคำทับศัพท์ในทุกช่องข้อความที่ผู้ชมเห็น เรียกเป็นขั้นสุดท้าย
// ของ buildSceneContent เพื่อให้ทำงานหลัง highlightTitleStr และ TruncateRunes แล้ว
// (guard ก่อน highlight = zwsp ไปขวางการจับคู่คำเน้น, guard ก่อน truncate =
// zwsp ถูกนับเป็นตัวอักษรจนข้อความโดนตัดสั้นกว่าที่ควร)
//
// ยกเว้น Panel.Items[].Label และ Panel.Field.Value โดยตั้งใจ — สองช่องนี้ต้อง
// เทียบกับ ui_vocab ของ catalog ได้ทุกไบต์ (agent.UIVocabViolations) และเป็นชื่อ
// เมนูภาษาอังกฤษของ Meta อยู่แล้ว จึงไม่มีคำทับศัพท์ไทยให้ต้องประกบ.
func guardSceneContent(c *SceneContent) {
	c.Kicker = GuardLoanWords(c.Kicker)
	c.Title = GuardLoanWords(c.Title)
	c.Sub = GuardLoanWords(c.Sub)
	c.StatLabel = GuardLoanWords(c.StatLabel)
	c.Pill = GuardLoanWords(c.Pill)
	c.CTA = GuardLoanWords(c.CTA)
	c.Brand = GuardLoanWords(c.Brand)
	c.Stamp = GuardLoanWords(c.Stamp)
	c.Callout = GuardLoanWords(c.Callout)
	c.Hook = GuardLoanWords(c.Hook)
	for i := range c.Rows {
		c.Rows[i].Text = GuardLoanWords(c.Rows[i].Text)
	}
	for i := range c.Chips {
		c.Chips[i].N = GuardLoanWords(c.Chips[i].N)
		c.Chips[i].T = GuardLoanWords(c.Chips[i].T)
	}
	for i := range c.Panels {
		c.Panels[i].T = GuardLoanWords(c.Panels[i].T)
		c.Panels[i].Quote = GuardLoanWords(c.Panels[i].Quote)
	}
	if c.Panel != nil {
		c.Panel.Breadcrumb = GuardLoanWords(c.Panel.Breadcrumb)
		if c.Panel.Field != nil {
			c.Panel.Field.Label = GuardLoanWords(c.Panel.Field.Label)
		}
	}
}
