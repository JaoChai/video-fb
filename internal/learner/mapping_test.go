package learner

import "testing"

func TestAgentForField(t *testing.T) {
	cases := []struct {
		field string
		want  string
	}{
		// scene content
		{"scene[0].voice_text", "scene"},
		{"scene[2].image_prompt", "scene"},
		{"scene[1].on_screen_text", "scene"},
		{"scene[3].text_content", "scene"},
		{"scene[0].emphasis_words", "scene"},
		{"voice_text", "scene"},
		{"on_screen_text", "scene"},
		{"image_prompt", "scene"},
		{"text_content", "scene"},
		{"emphasis", "scene"},
		{"scene", "scene"},
		{"SCENE[0].VOICE_TEXT", "scene"}, // case-insensitive

		// metadata -> script
		{"metadata.youtube_title", "script"},
		{"metadata.youtube_description", "script"},
		{"metadata.youtube_tags", "script"},
		{"youtube_title", "script"},
		{"youtube_description", "script"},
		{"youtube_tags", "script"},
		{"title", "script"},
		{"desc", "script"},
		{"tags", "script"},

		// unknown / unowned
		{"", ""},
		{"   ", ""},
		{"duration_seconds", ""},
		{"layout_variant", ""},
		{"random.field", ""},
	}
	for _, c := range cases {
		if got := agentForField(c.field); got != c.want {
			t.Errorf("agentForField(%q) = %q, want %q", c.field, got, c.want)
		}
	}
}
