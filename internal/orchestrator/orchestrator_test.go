package orchestrator

import (
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestScriptNarration(t *testing.T) {
	// The content_brain_v2 script prompt emits voice_script (the voiceover text
	// the SceneAgent breaks into scenes), not a scenes[] array.
	t.Run("uses voice_script", func(t *testing.T) {
		s := &agent.GeneratedScript{VoiceScript: "  สวัสดีครับ วันนี้มาเล่าเรื่องแอด  "}
		if got := scriptNarration(s); got != "สวัสดีครับ วันนี้มาเล่าเรื่องแอด" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("falls back to answer_script when voice_script blank", func(t *testing.T) {
		s := &agent.GeneratedScript{VoiceScript: "  ", AnswerScript: "บทเต็ม"}
		if got := scriptNarration(s); got != "บทเต็ม" {
			t.Errorf("got %q, want %q", got, "บทเต็ม")
		}
	})
	t.Run("empty when both blank", func(t *testing.T) {
		if got := scriptNarration(&agent.GeneratedScript{}); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}
