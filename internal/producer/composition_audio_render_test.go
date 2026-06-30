package producer

import (
	"strings"
	"testing"
)

func baseAudioParams() ScenesParams {
	return ScenesParams{
		AspectRatio:     "9:16",
		BrandName:       "Ads Vance",
		DurationSeconds: 10,
		VoiceSrc:        "assets/voice.wav",
		Scenes: []SceneSpec{{
			SceneNumber: 1, StartSec: 0, EndSec: 10, BackgroundMode: "css",
			Content: SceneContent{SceneNumber: 1, Start: 0, End: 10, Layout: "hero", Title: "Hi"},
		}},
	}
}

func TestRenderIncludesAmbientAndCues(t *testing.T) {
	p := baseAudioParams()
	p.AmbientSrc = "assets/ambient.mp3"
	p.TransitionCues = []TransitionCue{{Src: "assets/sfx/whoosh1.mp3", AtSec: 3.2}}
	html, err := RenderCompositionScenes(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	if !strings.Contains(s, `src="assets/ambient.mp3"`) {
		t.Error("ambient audio tag missing")
	}
	if !strings.Contains(s, `data-track-index="3"`) {
		t.Error("ambient track index missing")
	}
	if !strings.Contains(s, "assets/sfx/whoosh1.mp3") {
		t.Error("transition cue not in cues JSON")
	}
	if !strings.Contains(s, `a.id = "sfx"`) {
		t.Error("SFX audio elements must get a unique id (else silent — Task 0 finding)")
	}
}

func TestRenderOmitsAudioWhenAbsent(t *testing.T) {
	html, err := RenderCompositionScenes(baseAudioParams())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(html), "assets/ambient.mp3") {
		t.Error("ambient tag present when AmbientSrc empty")
	}
}
