package orchestrator

import (
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/producer"
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

func TestRetryPresetForCurrentMode(t *testing.T) {
	t.Setenv("CASE_FORMAT_ENABLED", "true")
	if got := retryPresetForCurrentMode("editorial-bold"); got.Key != producer.CaseFilePreset.Key {
		t.Errorf("flag on: stored classic must rebuild as case-file, got %q", got.Key)
	}
	t.Setenv("CASE_FORMAT_ENABLED", "")
	if got := retryPresetForCurrentMode("case-file"); got.Key == producer.CaseFilePreset.Key {
		t.Errorf("flag off: stored case-file must fall back to classic, got %q", got.Key)
	}
	if got := retryPresetForCurrentMode("neon-techno"); got.Key != "neon-techno" {
		t.Errorf("flag off: stored classic preset must be kept, got %q", got.Key)
	}
	if got := retryPresetForCurrentMode(""); got.Key != "editorial-bold" {
		t.Errorf("flag off: empty stored key must fall back to editorial-bold, got %q", got.Key)
	}
}
