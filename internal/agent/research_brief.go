package agent

// ResearchBrief is the structured output of the topic-driven ResearchAgent:
// a sourced, angle-bearing summary the ScriptAgent turns into narration.
// Wired in Plan 2b-2; defined here so dependent plans share a stable type.
type ResearchBrief struct {
	Topic          string     `json:"topic"`
	CoreMessage    string     `json:"core_message"`
	NarrativeAngle string     `json:"narrative_angle"`
	KeyPoints      []KeyPoint `json:"key_points"`
	Stats          []Stat     `json:"stats"`
}

// KeyPoint is one sourced fact. UseAs is the narrative role (e.g. "hook",
// "problem", "proof") so the script agent knows where it belongs.
type KeyPoint struct {
	Claim      string `json:"claim"`
	SourceURL  string `json:"source_url"`
	Confidence string `json:"confidence"`
	UseAs      string `json:"use_as"`
}

type Stat struct {
	Value     string `json:"value"`
	Context   string `json:"context"`
	SourceURL string `json:"source_url"`
}
