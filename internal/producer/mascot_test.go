package producer

import (
	"strings"
	"testing"
)

func TestMascotPoses(t *testing.T) {
	want := []string{"rocket", "point_left", "point_right", "thumbs_up", "think", "wave"}
	got := MascotPoseNames()
	if len(got) != len(want) {
		t.Fatalf("MascotPoseNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pose[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMascotEditPrompt(t *testing.T) {
	p := MascotEditPrompt("thumbs_up")
	if p == "" {
		t.Fatal("empty prompt")
	}
	for _, must := range []string{"cheetah", "astronaut", "transparent", "thumbs"} {
		if !strings.Contains(strings.ToLower(p), must) {
			t.Errorf("prompt missing %q: %q", must, p)
		}
	}
}

func TestMascotCueToPose(t *testing.T) {
	cases := map[string]string{"point": "point_right", "thumbs": "thumbs_up", "think": "think", "none": "", "": ""}
	for cue, want := range cases {
		if got := MascotCueToPose(cue); got != want {
			t.Errorf("MascotCueToPose(%q) = %q, want %q", cue, got, want)
		}
	}
}
