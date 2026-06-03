package producer

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
)

//go:embed templates/*.html.tmpl
var templateFS embed.FS

// RenderComposition executes the layout template for p and returns the
// composition HTML only (it does not assemble a project dir or copy assets —
// use CompositionBuilder.Build for that). It is handy for tests and previews.
//
// The template is selected by p.LayoutVariant (default "dynamic_karaoke").
func RenderComposition(p CompositionParams) ([]byte, error) {
	variant := p.LayoutVariant
	if variant == "" {
		variant = "dynamic_karaoke"
	}
	name := "layout_" + variant + ".html.tmpl"

	if p.DurationSeconds <= 0 {
		return nil, fmt.Errorf("DurationSeconds must be > 0, got %v", p.DurationSeconds)
	}

	cardsJSON, err := json.Marshal(p.Cards)
	if err != nil {
		return nil, fmt.Errorf("marshal cards: %w", err)
	}
	segsJSON, err := json.Marshal(p.Segments)
	if err != nil {
		return nil, fmt.Errorf("marshal segments: %w", err)
	}

	outroStart := p.DurationSeconds - outroLeadSeconds
	if outroStart < 0 {
		outroStart = 0
	}

	data := templateData{
		AccentColor:     sanitizeHexColor(p.AccentColor, "#ff6b2b"),
		SecondaryAccent: sanitizeHexColor(p.SecondaryAccent, "#2fd17a"),
		BrandName:       p.BrandName,
		CategoryLabel:   p.CategoryLabel,
		QuestionerName:  p.QuestionerName,
		Kicker:          p.Kicker,
		TitleHTML:       highlightTitle(p.Title, p.HighlightWords),
		OutroBrandHTML:  outroBrandHTML(p.BrandName),
		BackgroundMode:  backgroundMode(p.BackgroundMode),
		BackgroundImage: p.BackgroundImage,
		VoiceSrc:        p.VoiceSrc,
		AnimationSpeed:  animationSpeed(p.AnimationSpeed),
		DurationSeconds: p.DurationSeconds,
		OutroStart:      outroStart,
		OutroDuration:   p.DurationSeconds - outroStart,
		CardsJSON:       template.JS(cardsJSON),
		SegmentsJSON:    template.JS(segsJSON),
	}

	tmpl, err := template.New(name).ParseFS(templateFS, "templates/"+name)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}
