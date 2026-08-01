package repository

import "testing"

func TestValidRenderStage(t *testing.T) {
	for _, s := range []string{StageLint, StageInspect, StageRender} {
		if !ValidRenderStage(s) {
			t.Errorf("ValidRenderStage(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "LINT", "check", "render "} {
		if ValidRenderStage(s) {
			t.Errorf("ValidRenderStage(%q) = true, want false", s)
		}
	}
}
