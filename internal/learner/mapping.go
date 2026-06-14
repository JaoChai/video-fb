package learner

import "strings"

// agentForField maps a clip_critiques changes[].field path to the upstream
// agent whose skills own that field. Returns "" for fields no allowlisted agent
// owns (those are ignored by the strong-signal gate). Pure.
//
// Scene content (voice_text / on_screen_text / image_prompt / text_content /
// emphasis...) belongs to the `scene` agent. Metadata (youtube_title /
// description / tags, or any metadata.* path) belongs to the `script` agent.
func agentForField(field string) string {
	f := strings.ToLower(strings.TrimSpace(field))
	if f == "" {
		return ""
	}

	// Metadata-owned fields (script agent).
	if strings.HasPrefix(f, "metadata.") ||
		strings.Contains(f, "title") ||
		strings.Contains(f, "desc") ||
		strings.Contains(f, "tags") {
		return "script"
	}

	// Scene-content fields (scene agent). Match either the "scene[" prefix or any
	// of the known scene content sub-fields.
	if strings.HasPrefix(f, "scene[") || strings.HasPrefix(f, "scene.") || f == "scene" {
		return "scene"
	}
	for _, k := range []string{"voice_text", "on_screen_text", "image_prompt", "text_content", "emphasis"} {
		if strings.Contains(f, k) {
			return "scene"
		}
	}

	return ""
}
