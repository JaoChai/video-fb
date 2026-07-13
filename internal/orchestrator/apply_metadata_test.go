package orchestrator

import (
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// applyGeneratedMetadata overwrites script metadata from the metadata agent's
// output; a blank title means "do not apply" (keep script's own metadata).
func TestApplyGeneratedMetadata(t *testing.T) {
	base := func() *agent.GeneratedScript {
		return &agent.GeneratedScript{
			YoutubeTitle:       "ชื่อจาก script",
			YoutubeDescription: "desc จาก script",
			YoutubeTags:        []string{"a", "b"},
		}
	}

	t.Run("full metadata replaces all three fields", func(t *testing.T) {
		s := base()
		ok := applyGeneratedMetadata(s, &agent.GeneratedMetadata{
			YoutubeTitle:       "ชื่อจาก metadata",
			YoutubeDescription: "desc จาก metadata",
			YoutubeTags:        []string{"x"},
		})
		if !ok || s.YoutubeTitle != "ชื่อจาก metadata" || s.YoutubeDescription != "desc จาก metadata" || len(s.YoutubeTags) != 1 {
			t.Errorf("applied=%v script=%+v", ok, s)
		}
	})

	t.Run("blank title → not applied, script untouched", func(t *testing.T) {
		s := base()
		ok := applyGeneratedMetadata(s, &agent.GeneratedMetadata{YoutubeTitle: "  "})
		if ok || s.YoutubeTitle != "ชื่อจาก script" {
			t.Errorf("applied=%v title=%q", ok, s.YoutubeTitle)
		}
	})

	t.Run("blank desc/tags keep script values", func(t *testing.T) {
		s := base()
		ok := applyGeneratedMetadata(s, &agent.GeneratedMetadata{YoutubeTitle: "ชื่อใหม่"})
		if !ok || s.YoutubeDescription != "desc จาก script" || len(s.YoutubeTags) != 2 {
			t.Errorf("applied=%v script=%+v", ok, s)
		}
	})

	t.Run("nil metadata → not applied", func(t *testing.T) {
		s := base()
		if applyGeneratedMetadata(s, nil) {
			t.Error("applied=true for nil metadata")
		}
	})
}
