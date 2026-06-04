package agent

import "testing"

func TestNormalize_ClampsAndRenumbers(t *testing.T) {
	s := &GeneratedScript{Scenes: []GeneratedScene{
		{SceneNumber: 5, SceneType: "hook", VoiceText: "a"},
		{SceneNumber: 9, SceneType: "weird", VoiceText: "b"},
		{SceneNumber: 1, SceneType: "step", VoiceText: "c"},
		{SceneNumber: 2, SceneType: "win", VoiceText: "d"},
		{SceneNumber: 3, SceneType: "cta", VoiceText: "e"},
		{SceneNumber: 4, SceneType: "problem", VoiceText: "f"},
		{SceneNumber: 6, SceneType: "step", VoiceText: "g"},
	}}
	s.Normalize()

	if len(s.Scenes) != maxScenes {
		t.Fatalf("want %d scenes, got %d", maxScenes, len(s.Scenes))
	}
	if s.Scenes[0].VoiceText != "a" {
		t.Errorf("wrong scene kept at index 0: got VoiceText %q", s.Scenes[0].VoiceText)
	}
	for i, sc := range s.Scenes {
		if sc.SceneNumber != i+1 {
			t.Errorf("scene[%d] number = %d, want %d", i, sc.SceneNumber, i+1)
		}
		if !validSceneTypes[sc.SceneType] {
			t.Errorf("scene[%d] type %q not normalized", i, sc.SceneType)
		}
	}
}

func TestNormalize_DefaultsInvalidType(t *testing.T) {
	s := &GeneratedScript{Scenes: []GeneratedScene{
		{SceneNumber: 1, SceneType: "", VoiceText: "x"},
		{SceneNumber: 2, SceneType: "STEP", VoiceText: "y"},
	}}
	s.Normalize()
	if s.Scenes[0].SceneType != SceneStep || s.Scenes[1].SceneType != SceneStep {
		t.Errorf("invalid scene types not defaulted to %q: %+v", SceneStep, s.Scenes)
	}
}
