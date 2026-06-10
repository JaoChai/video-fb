package producer

import "html/template"

// TranscriptSegment is one phrase-level caption with its audio time window.
// Matches the Whisper verbose_json segment shape (start/end in seconds).
type TranscriptSegment struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
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
}

// SlotSpec is one semantic content slot rendered in scene flow layout.
type SlotSpec struct {
	Role    string        // headline|body|badge|step|stat|callout
	HTML    template.HTML // pre-escaped (emphasis applied)
	StepNum int
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
}
