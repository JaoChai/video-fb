package producer

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// These cover the SceneContent edge cases that the omitempty serialization +
// hero fallback can silently mangle: content-light layouts must NOT be
// overwritten to hero, and malformed/empty content MUST degrade to hero.
func TestBuildSceneContent_EdgeCases(t *testing.T) {
	b := sceneBound{Start: 0, End: 5}

	t.Run("step with title but no rows keeps step", func(t *testing.T) {
		s := agent.GeneratedScene{
			SceneNumber:  1,
			Layout:       "step",
			Content:      json.RawMessage(`{"title":"ติดตั้ง pixel","num":"1","of":"3"}`),
			OnScreenText: "สำรอง",
			CaptionStyle: "phrase_block",
		}
		c := buildSceneContent(s, b)
		if c.Layout != "step" {
			t.Fatalf("Layout = %q, want step (must not fall back to hero)", c.Layout)
		}
		if len(c.Rows) != 0 {
			t.Errorf("Rows = %v, want empty", c.Rows)
		}
		if c.Title == "" || !strings.Contains(c.Title, "pixel") {
			t.Errorf("Title = %q, want preserved step title", c.Title)
		}
	})

	t.Run("tip with only pill keeps tip", func(t *testing.T) {
		s := agent.GeneratedScene{
			SceneNumber:  2,
			Layout:       "tip",
			Content:      json.RawMessage(`{"pill":"เคล็ดลับ"}`),
			OnScreenText: "สำรอง",
			CaptionStyle: "word_pop",
		}
		c := buildSceneContent(s, b)
		if c.Layout != "tip" {
			t.Fatalf("Layout = %q, want tip (must not fall back to hero)", c.Layout)
		}
		if c.Pill != "เคล็ดลับ" {
			t.Errorf("Pill = %q, want preserved", c.Pill)
		}
	})

	t.Run("malformed content degrades to hero from on_screen_text", func(t *testing.T) {
		s := agent.GeneratedScene{
			SceneNumber:  3,
			Layout:       "stat",
			Content:      json.RawMessage("{bad"),
			OnScreenText: "สำรอง",
			CaptionStyle: "phrase_block",
		}
		c := buildSceneContent(s, b)
		if c.Layout != "hero" {
			t.Fatalf("Layout = %q, want hero", c.Layout)
		}
		if c.Title == "" {
			t.Errorf("Title is empty, want hero title from on_screen_text")
		}
	})

	t.Run("hook with rows preserves and cleans rows", func(t *testing.T) {
		s := agent.GeneratedScene{
			SceneNumber:  4,
			Layout:       "hook",
			Content:      json.RawMessage(`{"rows":[{"t":"• แบน ✅","bad":false},{"t":"กู้คืน","bad":true}]}`),
			OnScreenText: "สำรอง",
			CaptionStyle: "phrase_block",
		}
		c := buildSceneContent(s, b)
		if c.Layout != "hook" {
			t.Fatalf("Layout = %q, want hook", c.Layout)
		}
		if len(c.Rows) != 2 {
			t.Fatalf("Rows len = %d, want 2", len(c.Rows))
		}
		if strings.Contains(c.Rows[0].Text, "•") {
			t.Errorf("Rows[0].Text = %q, want decorative bullet stripped", c.Rows[0].Text)
		}
		if !c.Rows[1].Bad {
			t.Errorf("Rows[1].Bad = false, want true preserved")
		}
	})
}
