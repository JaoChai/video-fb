package producer

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
