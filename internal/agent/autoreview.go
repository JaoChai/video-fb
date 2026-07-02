package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)

// AutoReviewDecision is the raw JSON the model returns for the whole clip.
type AutoReviewDecision struct {
	Decision   string   `json:"decision"`    // approve | retry | hold
	Confidence float64  `json:"confidence"`  // 0-1
	Reasons    []string `json:"reasons"`     // Thai, human-readable
	DefectType string   `json:"defect_type"` // none | stochastic | deterministic
}

// AutoReviewResult is the normalized, fail-closed outcome the orchestrator acts on.
type AutoReviewResult struct {
	Decision   string // approve | retry | hold  (never anything else)
	Confidence float64
	Reasons    []string
	DefectType string
}

// AutoReviewInput is one clip's judging request: topic context, one frame per
// scene, and the Visual QA issues that flagged it.
type AutoReviewInput struct {
	Question string
	Frames   []QAFrame
	QAIssues []string
}

// AutoReviewAgent is the second-opinion judge. Vision-capable Claude model.
type AutoReviewAgent struct {
	llm *KieLLMClient
}

func NewAutoReviewAgent(llm *KieLLMClient) *AutoReviewAgent {
	return &AutoReviewAgent{llm: llm}
}

// normalizeAutoReview enforces the fail-closed policy: approve only when the
// model said "approve" AND confidence >= threshold; every other case (unknown
// decision, low-confidence approve) becomes "hold". "retry" and "hold" pass
// through. Pure function — unit tested.
func normalizeAutoReview(raw AutoReviewDecision, threshold float64) AutoReviewResult {
	res := AutoReviewResult{Confidence: raw.Confidence, Reasons: raw.Reasons, DefectType: raw.DefectType}
	switch raw.Decision {
	case "approve":
		if raw.Confidence >= threshold {
			res.Decision = "approve"
		} else {
			res.Decision = "hold"
		}
	case "retry":
		res.Decision = "retry"
	default: // "hold", "", or anything unexpected
		res.Decision = "hold"
	}
	return res
}

// autoReviewError builds a fail-closed hold result carrying an error note.
func autoReviewError(note string) AutoReviewResult {
	return AutoReviewResult{Decision: "hold", Reasons: []string{note}}
}

// Judge makes ONE holistic multi-image vision call for the whole clip and
// returns the normalized, fail-closed result. Any error → hold.
func (a *AutoReviewAgent) Judge(ctx context.Context, in AutoReviewInput, cfg *models.AgentConfig, threshold float64) AutoReviewResult {
	if len(in.Frames) == 0 {
		return autoReviewError("no frames to review (fail-closed hold)")
	}
	pngs := make([][]byte, 0, len(in.Frames))
	var b strings.Builder
	fmt.Fprintf(&b, "หัวข้อคลิป: %s\n\nสิ่งที่ Visual QA จับได้ (ตำหนิที่ต้องพิจารณา):\n", in.Question)
	for _, iss := range in.QAIssues {
		fmt.Fprintf(&b, "- %s\n", iss)
	}
	b.WriteString("\nเฟรมจริงแต่ละซีน (เรียงตามลำดับภาพที่แนบ):\n")
	for _, f := range in.Frames {
		if len(f.PNG) == 0 {
			continue
		}
		pngs = append(pngs, f.PNG)
		fmt.Fprintf(&b, "- ซีน %d: ข้อความที่ควรขึ้นจอ = %q\n", f.SceneNumber, f.OnScreenText)
	}
	if len(pngs) == 0 {
		return autoReviewError("all frames empty (fail-closed hold)")
	}
	b.WriteString("\nตอบเป็น JSON เท่านั้น: {\"decision\":\"approve|retry|hold\",\"confidence\":0-1,\"reasons\":[\"...\"],\"defect_type\":\"none|stochastic|deterministic\"}")

	var raw AutoReviewDecision
	if err := a.llm.GenerateVisionJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), b.String(), cfg.Temperature, pngs, &raw); err != nil {
		log.Printf("autoreview: vision error (fail-closed hold): %v", err)
		return autoReviewError(fmt.Sprintf("vision error: %v", err))
	}
	return normalizeAutoReview(raw, threshold)
}
