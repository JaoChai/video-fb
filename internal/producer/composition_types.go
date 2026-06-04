package producer

import "html/template"

// CompositionParams is the full input the composition builder needs to render a
// Hyperframes video. The composition agent (LLM) chooses the design fields
// (layout, colors, cards, highlights); the transcript comes from Whisper; the
// media paths and brand come from the clip/theme.
type CompositionParams struct {
	// Content
	Title          string   // scene text_content (the on-screen question/headline)
	HighlightWords []string // words inside Title to accent-color
	Kicker         string   // eyebrow label, e.g. "PIXEL & CAPI"
	BrandName      string   // "ADS VANCE"
	CategoryLabel  string   // e.g. "PIXEL"
	QuestionerName string   // e.g. "คุณป๊อบ"

	// Design (chosen by composition agent)
	LayoutVariant   string // template file key, e.g. "dynamic_karaoke"
	AccentColor     string // hex, e.g. "#ff6b2b"
	SecondaryAccent string // hex for win/positive cards, e.g. "#2fd17a"
	AnimationSpeed  string // "fast" | "normal" | "slow"

	// Media
	BackgroundMode  string  // "css" (gradient+pattern) | "image" (GPT background art)
	BackgroundImage string  // relative path under assets/ when BackgroundMode == "image"
	VoiceSrc        string  // relative path under assets/, e.g. "assets/voice.wav"
	DurationSeconds float64 // total clip length

	// Timed data
	Cards    []CardSpec          // point cards synced to audio
	Segments []TranscriptSegment // phrase-level captions synced to audio
}

// CardSpec is one point card that animates in/out over a time window.
type CardSpec struct {
	ID       string  `json:"id"`    // unique DOM id, e.g. "card1"
	Type     string  `json:"type"`  // "cause" (red) | "step" (orange) | "win" (green)
	StartSec float64 `json:"start"` // when the card animates in
	EndSec   float64 `json:"end"`   // when it animates out
	Kicker   string  `json:"kicker"`
	Body     string  `json:"body"`
	StepNum  int     `json:"step"` // shown in the badge when Type == "step"
}

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
	Scenes          []SceneSpec
	Segments        []TranscriptSegment
}
