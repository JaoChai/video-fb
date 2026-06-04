package producer

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
)

// scenesTemplateData is the flat view passed to the multi-scene html/template.
type scenesTemplateData struct {
	Width           int
	Height          int
	BrandName       string
	CategoryLabel   string
	QuestionerName  string
	Kicker          string
	VoiceSrc        string
	DurationSeconds float64
	Scenes          []SceneSpec
	SegmentsJSON    template.JS
	ScenesJSON      template.JS
}

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

// RenderCompositionScenes executes the multi-scene layout template for p and
// returns the composition HTML. It is the parallel to RenderComposition for the
// multi-scene pipeline.
func RenderCompositionScenes(p ScenesParams) ([]byte, error) {
	if p.DurationSeconds <= 0 {
		return nil, fmt.Errorf("DurationSeconds must be > 0, got %v", p.DurationSeconds)
	}
	if len(p.Scenes) == 0 {
		return nil, fmt.Errorf("Scenes must not be empty")
	}

	width, height := 1080, 1920
	if p.AspectRatio == "16:9" {
		width, height = 1920, 1080
	}

	segsJSON, err := json.Marshal(p.Segments)
	if err != nil {
		return nil, fmt.Errorf("marshal segments: %w", err)
	}

	// lightweight timing slice for the GSAP driver
	type sceneTiming struct {
		SceneNumber int     `json:"scene"`
		StartSec    float64 `json:"start"`
		EndSec      float64 `json:"end"`
	}
	timings := make([]sceneTiming, len(p.Scenes))
	for i, s := range p.Scenes {
		timings[i] = sceneTiming{SceneNumber: s.SceneNumber, StartSec: s.StartSec, EndSec: s.EndSec}
	}
	scenesJSON, err := json.Marshal(timings)
	if err != nil {
		return nil, fmt.Errorf("marshal scene timings: %w", err)
	}

	data := scenesTemplateData{
		Width:           width,
		Height:          height,
		BrandName:       p.BrandName,
		CategoryLabel:   p.CategoryLabel,
		QuestionerName:  p.QuestionerName,
		Kicker:          p.Kicker,
		VoiceSrc:        p.VoiceSrc,
		DurationSeconds: p.DurationSeconds,
		Scenes:          p.Scenes,
		SegmentsJSON:    template.JS(segsJSON),
		ScenesJSON:      template.JS(scenesJSON),
	}

	const name = "layout_multi_scene.html.tmpl"
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
