package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jaochai/video-fb/internal/models"
)

// QAFrame is one extracted scene frame plus the metadata the model needs to
// judge it (the on-screen text it SHOULD show, and the voice line for context).
type QAFrame struct {
	SceneNumber  int
	PNG          []byte
	OnScreenText string
	VoiceText    string
}

// VisualQAInput is the per-clip QA request: the question (topic context) plus
// one frame per scene.
type VisualQAInput struct {
	Question string
	Frames   []QAFrame
}

// SceneVerdict is the model's judgement for one scene. OK=false means a
// confident visual defect that should block auto-publish. Issues are the
// human-readable reasons (also written to the visual_qa log).
type SceneVerdict struct {
	SceneNumber int      `json:"scene_number"`
	OK          bool     `json:"ok"`
	Issues      []string `json:"issues"`
}

// visionVerdict is the raw single-frame JSON the model returns (one scene).
type visionVerdict struct {
	OK     bool     `json:"ok"`
	Issues []string `json:"issues"`
}

// VisualQAResult is what Review hands back to the orchestrator: per-scene
// verdicts plus the clip-level Passed decision.
type VisualQAResult struct {
	Verdicts []SceneVerdict
	Passed   bool
}

// summarizeVerdicts is the pure clip-level decision: the clip passes iff every
// scene verdict is OK. An empty verdict slice passes (nothing to block on —
// fail-open, consistent with the infra-error policy).
func summarizeVerdicts(verdicts []SceneVerdict) bool {
	for _, v := range verdicts {
		if !v.OK {
			return false
		}
	}
	return true
}

// VisualQATemplateData fills the seeded `visual_qa` prompt_template for one
// scene/frame.
type VisualQATemplateData struct {
	Question     string
	SceneNumber  int
	OnScreenText string
	VoiceText    string
}

// VisualQAAgent looks at one rendered frame per scene and flags visual defects.
// Runs on a vision-capable Claude model (cfg.Model = claude-sonnet-4-6).
type VisualQAAgent struct {
	llm *KieLLMClient
}

func NewVisualQAAgent(llm *KieLLMClient) *VisualQAAgent {
	return &VisualQAAgent{llm: llm}
}

// Review judges every frame and returns per-scene verdicts + the clip decision.
// It NEVER blocks on infrastructure: a template/vision/decode error for a scene
// is logged and recorded as an OK verdict (with the error in Issues), so only a
// confident visual defect (model says ok=false) can fail the clip. cfg is the
// `visual_qa` AgentConfig fetched by the caller via GetByName.
func (a *VisualQAAgent) Review(ctx context.Context, in VisualQAInput, cfg *models.AgentConfig) VisualQAResult {
	verdicts := make([]SceneVerdict, 0, len(in.Frames))
	for _, f := range in.Frames {
		verdicts = append(verdicts, a.reviewFrame(ctx, in.Question, f, cfg))
	}
	return VisualQAResult{Verdicts: verdicts, Passed: summarizeVerdicts(verdicts)}
}

// reviewFrame judges a single frame. On ANY error it returns an OK verdict
// (fail-open) annotated with the error, never blocking publish on infra.
func (a *VisualQAAgent) reviewFrame(ctx context.Context, question string, f QAFrame, cfg *models.AgentConfig) SceneVerdict {
	ok := func(note string) SceneVerdict {
		var issues []string
		if note != "" {
			issues = []string{note}
		}
		return SceneVerdict{SceneNumber: f.SceneNumber, OK: true, Issues: issues}
	}

	if len(f.PNG) == 0 {
		log.Printf("visualqa: scene %d has no frame bytes — treating as OK (fail-open)", f.SceneNumber)
		return ok("no frame extracted (skipped)")
	}

	userPrompt, err := renderTemplate(cfg.PromptTemplate, VisualQATemplateData{
		Question:     question,
		SceneNumber:  f.SceneNumber,
		OnScreenText: f.OnScreenText,
		VoiceText:    f.VoiceText,
	})
	if err != nil {
		log.Printf("visualqa: scene %d template error (fail-open): %v", f.SceneNumber, err)
		return ok(fmt.Sprintf("template error: %v", err))
	}

	var out visionVerdict
	if err := a.llm.GenerateVisionJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, [][]byte{f.PNG}, &out); err != nil {
		log.Printf("visualqa: scene %d vision error (fail-open): %v", f.SceneNumber, err)
		return ok(fmt.Sprintf("vision error: %v", err))
	}
	return SceneVerdict{SceneNumber: f.SceneNumber, OK: out.OK, Issues: out.Issues}
}

// MarshalVerdicts is a small helper for the orchestrator to JSON-encode verdicts
// for the visual_qa.issues column without importing encoding/json there twice.
func MarshalVerdicts(verdicts []SceneVerdict) []byte {
	b, err := json.Marshal(verdicts)
	if err != nil {
		return []byte("[]")
	}
	return b
}
