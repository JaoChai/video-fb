package producer

import "html/template"

// TranscriptSegment is one phrase-level caption with its audio time window.
// Matches the Whisper verbose_json segment shape (start/end in seconds).
type TranscriptSegment struct {
	Text     string   `json:"text"`
	Start    float64  `json:"start"`
	End      float64  `json:"end"`
	Emphasis []string `json:"emph,omitempty"` // words the template should highlight; empty ⇒ longest-word fallback
}

// SceneSpec is one fully-resolved scene the multi-scene template renders.
type SceneSpec struct {
	SceneNumber     int
	LayoutVariant   string  // hook_big|hook_punch|list_steps|stat_reveal|compare_two|quote_cta
	AccentColor     string  // sanitized hex
	AnimationSpeed  string  // fast|normal|slow
	StartSec        float64 // scene window on the continuous timeline
	EndSec          float64
	BackgroundMode  string // "css" | "image"
	BackgroundImage string // relative assets path when image
	MascotPose      string // relative assets path to a per-scene mascot PNG ("" = none)
	CaptionStyle    string // "word_pop" | "phrase_block"
	Slots           []SlotSpec
	Content         SceneContent
}

// SlotSpec is one semantic content slot rendered in scene flow layout.
type SlotSpec struct {
	Role    string        // headline|body|badge|step|stat|callout
	HTML    template.HTML // pre-escaped (emphasis applied)
	StepNum int
}

// SceneContent is the structured, render-ready content for one scene. It is
// serialized into ScenesJSON and consumed by the template's in-page DOM builder.
// Exactly one layout's fields are populated per scene; the rest stay zero.
type SceneContent struct {
	SceneNumber     int     `json:"scene"`
	Start           float64 `json:"start"`
	End             float64 `json:"end"`
	Layout          string  `json:"type"`          // hook|hero|stat|step|tip|cta
	CaptionStyle    string  `json:"caption_style"` // word_pop|phrase_block
	BackgroundImage string  `json:"bg"`            // relative assets path, "" = gradient only

	Kicker    string        `json:"kicker,omitempty"`
	Title     string        `json:"title,omitempty"` // may contain <span class="acc"> from emphasis
	Sub       string        `json:"sub,omitempty"`
	Rows      []ContentRow  `json:"rows,omitempty"`
	Stat      string        `json:"stat,omitempty"`
	Unit      string        `json:"unit,omitempty"`
	StatLabel string        `json:"statLabel,omitempty"`
	Chips     []ContentChip `json:"chips,omitempty"`
	Num       string        `json:"num,omitempty"`
	Of        string        `json:"of,omitempty"`
	Pill      string        `json:"pill,omitempty"`
	CTA       string        `json:"cta,omitempty"`
	Brand     string        `json:"brand,omitempty"`
}

// ContentRow is one bullet row. Bad=true tints it red (problem/❌ replacement).
type ContentRow struct {
	Text string `json:"t"`
	Bad  bool   `json:"bad,omitempty"`
}

// ContentChip is one small stat chip beneath a stat card.
type ContentChip struct {
	N string `json:"n"`
	T string `json:"t"`
}

// TransitionCue is one scene-transition sound effect placement. Name is the
// embedded SFX base name (input); Src is the project-relative asset path the
// builder fills in; AtSec is the timeline start in seconds.
type TransitionCue struct {
	Name  string  `json:"-"`
	Src   string  `json:"src"`
	AtSec float64 `json:"at"`
}

// ScenesParams is the full input for the multi-scene template.
type ScenesParams struct {
	AspectRatio     string // "9:16" | "16:9"
	BrandName       string
	CategoryLabel   string
	QuestionerName  string
	Kicker          string
	VoiceSrc        string
	DurationSeconds float64
	IntroMascot     string // relative assets path for the intro bumper mascot ("" = none)
	OutroMascot     string // relative assets path for the outro bumper mascot ("" = none)
	CTAText         string // call-to-action line shown under the outro mascot ("" = none)
	Scenes          []SceneSpec
	Segments        []TranscriptSegment

	// Palette + BrandCSS drive per-clip style presets. When zero/empty,
	// RenderCompositionScenes falls back to the package-global Brand (today's look).
	Palette  BrandColors
	BrandCSS string

	// ThemeKey + Motion identify the active design theme for the template
	// (data-theme attribute + per-theme texture/motion). Zero Motion ⇒
	// MotionDefault (today's Editorial Bold feel).
	ThemeKey string
	Motion   MotionProfile

	// Audio + motion upgrade (gated by AUDIO_MOTION_ENABLED). All zero ⇒ today's
	// voice-only, current-motion output.
	AmbientLocalPath string          // absolute path to a prepared ambient.mp3 (input; builder copies it)
	AmbientSrc       string          // project-relative path; set by BuildScenes
	TransitionCues   []TransitionCue // scene-transition SFX placements
	AudioMotion      bool            // enable upgraded GSAP transitions
}
