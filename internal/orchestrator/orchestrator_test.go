package orchestrator

import (
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestScriptNarration(t *testing.T) {
	t.Run("single scene", func(t *testing.T) {
		s := &agent.GeneratedScript{Scenes: []agent.GeneratedScene{{VoiceText: "สวัสดีครับ วันนี้มาเล่าเรื่องแอด"}}}
		if got := scriptNarration(s); got != "สวัสดีครับ วันนี้มาเล่าเรื่องแอด" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("joins multiple, trims, skips empty", func(t *testing.T) {
		s := &agent.GeneratedScript{Scenes: []agent.GeneratedScene{
			{VoiceText: "  ตอนแรก  "}, {VoiceText: ""}, {VoiceText: "ตอนสอง"},
		}}
		if got := scriptNarration(s); got != "ตอนแรก ตอนสอง" {
			t.Errorf("got %q, want %q", got, "ตอนแรก ตอนสอง")
		}
	})
	t.Run("no scenes", func(t *testing.T) {
		if got := scriptNarration(&agent.GeneratedScript{}); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}
