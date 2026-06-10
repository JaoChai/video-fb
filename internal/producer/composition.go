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
	BrandCSS        template.CSS // brand color/motion/type vars (single source of truth)
	BrandName       string
	CategoryLabel   string
	QuestionerName  string
	Kicker          string
	VoiceSrc        string
	DurationSeconds float64
	IntroMascot     string
	OutroMascot     string
	CTAText         string
	OutroStartSec   float64
	OutroDurSec     float64
	Scenes          []SceneSpec
	SegmentsJSON    template.JS
	ScenesJSON      template.JS
}

//go:embed templates/*.html.tmpl
var templateFS embed.FS

// gsapMinJS is the GSAP runtime, vendored and written into each project's assets/
// dir by the builder so the template can load it via a relative <script src> with
// NO CDN fetch at render time. The Railway container's render couldn't reach
// cdn.jsdelivr.net, so GSAP failed to load and every scene froze on the first
// frame (audio was unaffected). It is bundled as an asset rather than inlined
// into index.html so its internal Math.random() does not trip the Hyperframes
// "non_deterministic_code" lint gate (which would fall the render back to a
// static image).
//
//go:embed templates/gsap-3.14.2.min.js
var gsapMinJS string

// RenderCompositionScenes executes the multi-scene layout template for p and
// returns the composition HTML (it does not assemble a project dir or copy
// assets — use CompositionBuilder.BuildScenes for that).
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

	// Sanitize each scene's LLM-chosen accent color before it reaches the
	// template's inline CSS (copy first — don't mutate the caller's slice).
	sanitizedScenes := make([]SceneSpec, len(p.Scenes))
	copy(sanitizedScenes, p.Scenes)
	for i := range sanitizedScenes {
		sanitizedScenes[i].AccentColor = sanitizeHexColor(sanitizedScenes[i].AccentColor, Brand.Orange)
	}

	// Structured, render-ready content for the template's in-page DOM builder.
	// The image background path is resolved here (the scene knows it only after
	// BuildScenes has copied the asset), overriding whatever Content carried.
	contents := make([]SceneContent, len(p.Scenes))
	for i, sc := range p.Scenes {
		contents[i] = sc.Content
		if sc.BackgroundMode == "image" {
			contents[i].BackgroundImage = sc.BackgroundImage
		}
	}
	scenesJSON, err := json.Marshal(contents)
	if err != nil {
		return nil, fmt.Errorf("marshal scene contents: %w", err)
	}

	outroStart := p.DurationSeconds - 1.6
	if outroStart < 0 {
		outroStart = 0
	}

	data := scenesTemplateData{
		Width:           width,
		Height:          height,
		BrandCSS:        template.CSS(Brand.CSSVars()),
		BrandName:       p.BrandName,
		CategoryLabel:   p.CategoryLabel,
		QuestionerName:  p.QuestionerName,
		Kicker:          p.Kicker,
		VoiceSrc:        p.VoiceSrc,
		DurationSeconds: p.DurationSeconds,
		IntroMascot:     p.IntroMascot,
		OutroMascot:     p.OutroMascot,
		CTAText:         p.CTAText,
		OutroStartSec:   outroStart,
		OutroDurSec:     p.DurationSeconds - outroStart,
		Scenes:          sanitizedScenes,
		SegmentsJSON:    template.JS(segsJSON),
		ScenesJSON:      template.JS(scenesJSON),
	}

	const name = "layout_multi_scene.html.tmpl"
	funcs := template.FuncMap{
		"durSec": func(start, end float64) float64 {
			d := end - start
			if d < 0.1 {
				d = 0.1
			}
			return d
		},
		"addInt": func(a, b int) int { return a + b },
	}
	tmpl, err := template.New(name).Funcs(funcs).ParseFS(templateFS, "templates/"+name)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}
