package agent

import (
	"context"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

// Locks the PIPELINE_FAST_ENABLED Review contract: verdicts come back one per
// frame IN FRAME ORDER even though frames are judged concurrently. Frames carry
// no PNG bytes, so every reviewFrame returns fail-open before touching the LLM —
// the agent's nil llm doubles as a trap that panics if a network call slips in.
func TestReview_Parallel_KeepsFrameOrder(t *testing.T) {
	a := NewVisualQAAgent(nil)
	in := VisualQAInput{Question: "q", Fast: true}
	for i := 1; i <= 8; i++ {
		in.Frames = append(in.Frames, QAFrame{SceneNumber: i})
	}
	cfg := &models.AgentConfig{}

	res := a.Review(context.Background(), in, cfg)

	if len(res.Verdicts) != 8 {
		t.Fatalf("want 8 verdicts, got %d", len(res.Verdicts))
	}
	for i, v := range res.Verdicts {
		if v.SceneNumber != i+1 {
			t.Fatalf("verdict %d: want scene %d, got %d", i, i+1, v.SceneNumber)
		}
		if !v.OK {
			t.Fatalf("scene %d: frames without bytes must fail-open (OK)", v.SceneNumber)
		}
	}
	if !res.Passed {
		t.Fatal("all fail-open verdicts must pass the clip")
	}
}
