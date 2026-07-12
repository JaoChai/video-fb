package agent

import (
	"strings"
	"testing"
)

func v(scene int, ok bool, issues ...string) SceneVerdict {
	return SceneVerdict{SceneNumber: scene, OK: ok, Issues: issues}
}

func TestConfirmMerge_PassedFirstIsUntouched(t *testing.T) {
	first := VisualQAResult{Verdicts: []SceneVerdict{v(1, true), v(2, true)}, Passed: true}
	got := ConfirmMerge(first, VisualQAResult{})
	if !got.Passed || len(got.Verdicts) != 2 {
		t.Fatalf("passed result must be returned unchanged, got %+v", got)
	}
}

// A defect visible at BOTH sample points is real → scene stays failed, issues merged.
func TestConfirmMerge_BothFlag_StaysFailed(t *testing.T) {
	first := VisualQAResult{Verdicts: []SceneVerdict{v(1, false, "ล้นกรอบ")}, Passed: false}
	confirm := VisualQAResult{Verdicts: []SceneVerdict{v(1, false, "ยังล้นกรอบ")}, Passed: false}
	got := ConfirmMerge(first, confirm)
	if got.Passed {
		t.Fatal("scene flagged by both passes must keep the clip failed")
	}
	if len(got.Verdicts[0].Issues) != 2 {
		t.Errorf("issues must merge both passes, got %v", got.Verdicts[0].Issues)
	}
}

// Flagged only at the first sample (mid-animation / karaoke phrase) → cleared.
func TestConfirmMerge_ConfirmOK_Clears(t *testing.T) {
	first := VisualQAResult{Verdicts: []SceneVerdict{v(1, false, "ข้อความถูกตัด")}, Passed: false}
	confirm := VisualQAResult{Verdicts: []SceneVerdict{v(1, true)}, Passed: true}
	got := ConfirmMerge(first, confirm)
	if !got.Passed {
		t.Fatal("scene cleared by the confirm pass must pass the clip")
	}
	if !got.Verdicts[0].OK {
		t.Error("verdict must flip to OK")
	}
	if len(got.Verdicts[0].Issues) == 0 || !strings.Contains(got.Verdicts[0].Issues[0], "recheck") {
		t.Errorf("cleared verdict must keep an audit note, got %v", got.Verdicts[0].Issues)
	}
}

// No confirm frame for the scene (extraction failed) → fail-open: cleared.
func TestConfirmMerge_MissingConfirmFrame_Clears(t *testing.T) {
	first := VisualQAResult{Verdicts: []SceneVerdict{v(3, false, "จอว่าง")}, Passed: false}
	got := ConfirmMerge(first, VisualQAResult{Passed: true})
	if !got.Passed {
		t.Fatal("missing confirm frame must fail open (scene cleared)")
	}
}

func TestConfirmMerge_Mixed(t *testing.T) {
	first := VisualQAResult{
		Verdicts: []SceneVerdict{v(1, true), v(2, false, "ล้น"), v(3, false, "ตัด")},
		Passed:   false,
	}
	confirm := VisualQAResult{Verdicts: []SceneVerdict{v(2, false, "ล้นจริง"), v(3, true)}, Passed: false}
	got := ConfirmMerge(first, confirm)
	if got.Passed {
		t.Fatal("scene 2 confirmed failed → clip must fail")
	}
	if got.Verdicts[1].OK || !got.Verdicts[2].OK || !got.Verdicts[0].OK {
		t.Errorf("want scene2 failed, scenes 1&3 ok; got %+v", got.Verdicts)
	}
}
