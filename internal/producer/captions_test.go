package producer

import (
	"strings"
	"testing"
	"unicode"

	"github.com/jaochai/video-fb/internal/agent"
)

// runesNoSpace strips all whitespace and returns the remaining runes, so we can
// assert caption content is byte-for-byte the ground truth regardless of how it
// was packed into lines.
func runesNoSpace(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func TestCaptionSegmentsFromScenes_MatchesGroundTruth(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "บัญชีโฆษณาโดนแบนถาวรเพราะอะไรกันแน่"},
		{SceneNumber: 2, VoiceText: "วันนี้เรามีสามขั้นตอนกู้คืนมาฝาก"},
	}
	bounds := []sceneBound{{Start: 0, End: 6}, {Start: 6, End: 13}}

	segs := captionSegmentsFromScenes(scenes, bounds)
	if len(segs) == 0 {
		t.Fatal("expected segments, got none")
	}

	// Every rune of the original VoiceText must appear in the captions, in order,
	// with nothing invented — this is the whole point of the fix.
	wantText := runesNoSpace(scenes[0].VoiceText) + runesNoSpace(scenes[1].VoiceText)
	var got strings.Builder
	for _, s := range segs {
		got.WriteString(runesNoSpace(s.Text))
	}
	if got.String() != wantText {
		t.Errorf("caption text drifted from ground truth\n got: %q\nwant: %q", got.String(), wantText)
	}
}

func TestCaptionSegmentsFromScenes_TimingWithinBoundsAndMonotonic(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "ประโยคแรก สั้นๆ"},
		{SceneNumber: 2, VoiceText: "ประโยคที่สอง ยาวกว่าเดิมนิดหน่อย เพื่อทดสอบการแบ่งเวลา"},
	}
	bounds := []sceneBound{{Start: 0, End: 5}, {Start: 5, End: 14}}

	segs := captionSegmentsFromScenes(scenes, bounds)

	var prevEnd float64
	for i, s := range segs {
		if s.Start < 0 || s.End <= s.Start {
			t.Errorf("seg %d has bad window [%v,%v]", i, s.Start, s.End)
		}
		if s.Start+1e-9 < prevEnd {
			t.Errorf("seg %d start %v overlaps previous end %v", i, s.Start, prevEnd)
		}
		prevEnd = s.End
	}
	// The last caption must end exactly at the final scene boundary (no drift).
	if last := segs[len(segs)-1]; last.End != 14 {
		t.Errorf("last segment end = %v, want 14 (scene boundary)", last.End)
	}
}

func TestCaptionSegmentsFromScenes_SkipsEmptyAndZeroWidth(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: ""},                       // empty text → skip
		{SceneNumber: 2, VoiceText: "มีข้อความ"},               // zero-width bound → skip
		{SceneNumber: 3, VoiceText: "ปกติ"},
	}
	bounds := []sceneBound{{Start: 0, End: 0}, {Start: 0, End: 0}, {Start: 0, End: 4}}

	segs := captionSegmentsFromScenes(scenes, bounds)
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment (only scene 3), got %d", len(segs))
	}
	if got := runesNoSpace(segs[0].Text); got != "ปกติ" {
		t.Errorf("got %q, want %q", got, "ปกติ")
	}
}

func TestCaptionSegmentsFromScenes_LongTextSplitsIntoPhrases(t *testing.T) {
	long := "นี่คือประโยคที่ยาวมากๆ มีหลายวรรค เพื่อให้ระบบต้องแบ่งออกเป็นหลายแคปชั่น ไม่ให้ตัวหนังสือล้นจอ และต้องอ่านได้สบายตา"
	scenes := []agent.GeneratedScene{{SceneNumber: 1, VoiceText: long}}
	bounds := []sceneBound{{Start: 0, End: 12}}

	segs := captionSegmentsFromScenes(scenes, bounds)
	if len(segs) < 2 {
		t.Fatalf("expected long text to split into multiple phrases, got %d", len(segs))
	}
	for i, s := range segs {
		if n := len([]rune(s.Text)); n > captionMaxRunes {
			t.Errorf("seg %d has %d runes, exceeds captionMaxRunes=%d: %q", i, n, captionMaxRunes, s.Text)
		}
	}
}

func TestCaptionSegments_CarryEmphasisFromScene(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "บัญชีโฆษณาโดนแบนถาวร", EmphasisWords: []string{"โดนแบน"}},
	}
	bounds := []sceneBound{{Start: 0, End: 5}}
	segs := captionSegmentsFromScenes(scenes, bounds)
	if len(segs) == 0 {
		t.Fatal("expected segments")
	}
	// At least one produced segment must carry the scene's emphasis words so the
	// template can highlight the RIGHT word (not merely the longest).
	found := false
	for _, s := range segs {
		for _, e := range s.Emphasis {
			if e == "โดนแบน" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no segment carried emphasis %q; got %+v", "โดนแบน", segs)
	}
}

func TestSafeCut_doesNotSplitCombiningMark(t *testing.T) {
	// "ที่" = ท + ◌ี(U+0E35 Mn) ; a naive cut at an index landing on the mark
	// would orphan it. Build a >max run ending so index `max` is a combining mark.
	base := []rune("กกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกก") // 42 base consonants
	tr := append(base, 'ี')                                        // index 42 = combining mark
	cut := safeCut(tr, 42)
	if unicodeIsMn(tr[cut]) {
		t.Errorf("safeCut returned index %d which is a combining mark", cut)
	}
	if cut < 1 {
		t.Errorf("safeCut backed off too far: %d", cut)
	}
}

func unicodeIsMn(r rune) bool { return unicode.Is(unicode.Mn, r) }
