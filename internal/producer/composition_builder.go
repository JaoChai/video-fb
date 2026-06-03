package producer

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Static project files Hyperframes expects alongside index.html. Kept as
// constants so a built project does not depend on the PoC directory.
const projectPackageJSON = `{
  "name": "clip",
  "private": true,
  "type": "module",
  "scripts": {
    "check": "npx --yes hyperframes@0.6.70 lint",
    "render": "npx --yes hyperframes@0.6.70 render"
  }
}
`

const projectHyperframesJSON = `{
  "$schema": "https://hyperframes.heygen.com/schema/hyperframes.json",
  "paths": { "blocks": "compositions", "components": "compositions/components", "assets": "assets" }
}
`

// CompositionBuilder assembles a renderable Hyperframes project directory from
// CompositionParams + a chosen Go template.
type CompositionBuilder struct {
	templatesDir string // dir holding layout_*.html.tmpl
	fontsDir     string // source Sarabun .ttf files to copy into each project
}

func NewCompositionBuilder(templatesDir, fontsDir string) *CompositionBuilder {
	return &CompositionBuilder{templatesDir: templatesDir, fontsDir: fontsDir}
}

// outroLeadSeconds is how long before the clip end the outro fades in.
const outroLeadSeconds = 4.0

// templateData is the flat view passed to the html/template. JSON/HTML fields are
// pre-rendered so the template stays declarative.
type templateData struct {
	AccentColor     string
	SecondaryAccent string
	BrandName       string
	CategoryLabel   string
	QuestionerName  string
	Kicker          string
	TitleHTML       template.HTML
	OutroBrandHTML  template.HTML
	BackgroundMode  string
	BackgroundImage string
	VoiceSrc        string
	AnimationSpeed  string
	DurationSeconds float64
	OutroStart      float64
	OutroDuration   float64
	CardsJSON       template.JS
	SegmentsJSON    template.JS
}

// Build writes a complete project into projectDir and returns the dir path.
// voicePath/bgPath are absolute source files copied into the project's assets.
func (b *CompositionBuilder) Build(params CompositionParams, projectDir, voicePath, bgPath string) (string, error) {
	assetsDir := filepath.Join(projectDir, "assets")
	fontsDst := filepath.Join(assetsDir, "fonts")
	if err := os.MkdirAll(fontsDst, 0o755); err != nil {
		return "", fmt.Errorf("mkdir project: %w", err)
	}

	if err := copyFile(voicePath, filepath.Join(assetsDir, "voice.wav")); err != nil {
		return "", fmt.Errorf("copy voice: %w", err)
	}
	if params.BackgroundMode == "image" && bgPath != "" {
		if err := copyFile(bgPath, filepath.Join(assetsDir, filepath.Base(params.BackgroundImage))); err != nil {
			return "", fmt.Errorf("copy background: %w", err)
		}
	}
	if err := copyDir(b.fontsDir, fontsDst); err != nil {
		return "", fmt.Errorf("copy fonts: %w", err)
	}

	cardsJSON, err := json.Marshal(params.Cards)
	if err != nil {
		return "", fmt.Errorf("marshal cards: %w", err)
	}
	segsJSON, err := json.Marshal(params.Segments)
	if err != nil {
		return "", fmt.Errorf("marshal segments: %w", err)
	}

	outroStart := params.DurationSeconds - outroLeadSeconds
	if outroStart < 0 {
		outroStart = 0
	}

	data := templateData{
		AccentColor:     sanitizeHexColor(params.AccentColor, "#ff6b2b"),
		SecondaryAccent: sanitizeHexColor(params.SecondaryAccent, "#2fd17a"),
		BrandName:       params.BrandName,
		CategoryLabel:   params.CategoryLabel,
		QuestionerName:  params.QuestionerName,
		Kicker:          params.Kicker,
		TitleHTML:       highlightTitle(params.Title, params.HighlightWords),
		OutroBrandHTML:  outroBrandHTML(params.BrandName),
		BackgroundMode:  backgroundMode(params.BackgroundMode),
		BackgroundImage: params.BackgroundImage,
		VoiceSrc:        params.VoiceSrc,
		AnimationSpeed:  animationSpeed(params.AnimationSpeed),
		DurationSeconds: params.DurationSeconds,
		OutroStart:      outroStart,
		OutroDuration:   params.DurationSeconds - outroStart,
		CardsJSON:       template.JS(cardsJSON),
		SegmentsJSON:    template.JS(segsJSON),
	}

	tmplPath := filepath.Join(b.templatesDir, "layout_"+params.LayoutVariant+".html.tmpl")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", tmplPath, err)
	}
	out, err := os.Create(filepath.Join(projectDir, "index.html"))
	if err != nil {
		return "", fmt.Errorf("create index.html: %w", err)
	}
	defer out.Close()
	if err := tmpl.Execute(out, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	clipID := filepath.Base(projectDir)
	metaJSON := fmt.Sprintf(`{"id": %q, "name": %q}`, clipID, clipID)
	for name, content := range map[string]string{
		"package.json":     projectPackageJSON,
		"hyperframes.json": projectHyperframesJSON,
		"meta.json":        metaJSON,
	} {
		if err := os.WriteFile(filepath.Join(projectDir, name), []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", name, err)
		}
	}

	return projectDir, nil
}

// highlightTitle wraps each highlight word in <span class="hl"> while escaping
// everything else. Escaping the words too keeps the match consistent.
func highlightTitle(title string, words []string) template.HTML {
	escaped := html.EscapeString(title)
	for _, w := range words {
		if w == "" {
			continue
		}
		ew := html.EscapeString(w)
		escaped = strings.ReplaceAll(escaped, ew, `<span class="hl">`+ew+`</span>`)
	}
	return template.HTML(escaped)
}

// outroBrandHTML renders the brand with its last word in an accent span
// (e.g. "ADS VANCE" → ADS <span>VANCE</span>). Falls back to the whole brand.
func outroBrandHTML(brand string) template.HTML {
	brand = strings.TrimSpace(brand)
	if brand == "" {
		return template.HTML(`ADS <span>VANCE</span>`)
	}
	parts := strings.Fields(brand)
	if len(parts) < 2 {
		return template.HTML(html.EscapeString(brand)) //nolint:gosec // escaped
	}
	head := html.EscapeString(strings.Join(parts[:len(parts)-1], " "))
	tail := html.EscapeString(parts[len(parts)-1])
	return template.HTML(head + ` <span>` + tail + `</span>`) //nolint:gosec // escaped
}

// sanitizeHexColor accepts #rgb / #rrggbb / #rrggbbaa and returns fallback
// otherwise, so an LLM-chosen color can never break the CSS context.
func sanitizeHexColor(c, fallback string) string {
	c = strings.TrimSpace(c)
	if c == "" || c[0] != '#' {
		return fallback
	}
	hex := c[1:]
	if len(hex) != 3 && len(hex) != 6 && len(hex) != 8 {
		return fallback
	}
	for _, r := range hex {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return fallback
		}
	}
	return c
}

func backgroundMode(m string) string {
	if m == "image" {
		return "image"
	}
	return "css"
}

func animationSpeed(s string) string {
	switch s {
	case "fast", "slow":
		return s
	default:
		return "normal"
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func copyDir(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(srcDir, e.Name()), filepath.Join(dstDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
