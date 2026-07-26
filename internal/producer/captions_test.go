package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// runesNoSpace strips all whitespace and returns the remaining runes, so we can
// assert caption content is byte-for-byte the ground truth regardless of how it
// was packed into lines. zwsp ก็ต้องถอดด้วย เพราะเป็นอักขระจัดบรรทัดที่
// GuardLoanWords แทรกเข้ามา ไม่ใช่เนื้อความ — และ strings.Fields ไม่ตัดให้
// (U+200B ไม่นับเป็น whitespace)
func runesNoSpace(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, zwsp, "")), "")
}

func TestCaptionSegmentsFromScenes_MatchesGroundTruth(t *testing.T) {
	// ซีนที่สองมีคำทับศัพท์ (แอดมิน) เพื่อให้ GuardLoanWords ได้แทรก zwsp จริง —
	// ยืนยันว่า zwsp ไม่นับเป็นเนื้อความที่ drift ไปจากบทพากย์
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "บัญชีโฆษณาโดนแบนถาวรเพราะอะไรกันแน่"},
		{SceneNumber: 2, VoiceText: "วันนี้เรามีสามขั้นตอนกู้สิทธิ์แอดมินคืนมาฝาก"},
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
		{SceneNumber: 1, VoiceText: ""},          // empty text → skip
		{SceneNumber: 2, VoiceText: "มีข้อความ"}, // zero-width bound → skip
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
		// วลีที่แพ็กจากหลาย token ต้องไม่เกิน captionMaxRunes — แต่ token เดี่ยวที่
		// ยาวเกินอยู่แล้ว (ข้อความไทยไม่มีช่องว่าง) ตั้งใจปล่อยไว้ทั้งคำ เพราะการ
		// หั่นให้พอดีจำนวนตัวอักษรคือการตัดกลางคำ ซึ่งคือบั๊กที่เรากำลังแก้
		if n := len([]rune(s.Text)); n > captionMaxRunes && len(strings.Fields(s.Text)) > 1 {
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

// หกประโยคนี้คือ voice_text จริงของซีนที่ backtest (2026-07-26) จับได้ว่าผู้ชม
// เห็นคำขาดกลางคำ: แอดมินอ|ยู่ · บัญชีที่เค|ยมีปัญหา · ของทั้|งบัญชี ·
// เป็นปัญห|าแบบนี้ · ขาดไม่ไ|ด้ · ตัวตนผู้จ่|ายเงิน
func TestSplitCaptionPhrases_KeepsTokensWhole(t *testing.T) {
	voices := []string{
		"จุดที่สาม domain กับ Page ที่ยังผูกกับ BM เดิม ถ้ายังชี้กลับไปหาเขา เจ้าของเก่าเคลมคืนได้ทั้งที่คุณคุมแอดมินอยู่",
		"ถ้าเป็นบัญชีซื้อมา อย่าเอาบัตรใบเดียวกันไปผูกซ้ำกับบัญชีที่เคยมีปัญหา เพราะมันลากทั้งพอร์ตเสียหายได้",
		"เพราะระบบมองอัตราเปลี่ยนพฤติกรรมสะสมของทั้งบัญชี ไม่ใช่ขนาดงบต่อแคมเปญ",
		"พอร์ตคุณเคยพังจากจุดที่ไม่คิดว่าจะเป็นปัญหาแบบนี้ไหม ทักไลน์ทีมงานมาคุยกันได้ครับ",
		"ปี 2026 CAPI ไม่ใช่ทางเลือกแล้ว มันคือมาตรฐานขั้นต่ำที่ถือหลายบัญชีขาดไม่ได้",
		"ข้อสอง สำคัญมากสำหรับสาย agency คือ Payer KYC ถ้าใครจ่ายเงินแทนคนอื่น แพลตฟอร์มต้องเก็บข้อมูลและยืนยันตัวตนผู้จ่ายเงินด้วย",
	}
	for _, v := range voices {
		whole := map[string]bool{}
		for _, tok := range strings.Fields(v) {
			whole[tok] = true
		}
		for _, ph := range splitCaptionPhrases(v) {
			for _, tok := range strings.Fields(ph) {
				if !whole[tok] {
					t.Errorf("token %q ถูกหั่นมาจากคำเต็ม\nประโยค: %q", tok, v)
				}
			}
		}
	}
}

// วลีเดี่ยวที่ยาวเกิน captionMaxRunes ต้องอยู่ครบเป็นวลีเดียว ไม่ถูกซอย
func TestSplitCaptionPhrases_LongThaiRunStaysWhole(t *testing.T) {
	long := "แพลตฟอร์มต้องเก็บข้อมูลและยืนยันตัวตนผู้จ่ายเงินให้ครบทุกขั้นตอนก่อนอนุมัติ"
	got := splitCaptionPhrases(long)
	if len(got) != 1 {
		t.Fatalf("คาดว่าได้ 1 วลี ได้ %d วลี: %q", len(got), got)
	}
	if got[0] != long {
		t.Errorf("วลีเพี้ยน\n got: %q\nwant: %q", got[0], long)
	}
}

// แคปชั่นต้องถูก guard แล้ว — นี่คือข้อความที่ไปโผล่บนจอจริง
func TestCaptionSegmentsFromScenes_GuardsLoanWords(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "ทักไลน์ไอดีแอดส์แวนซ์ได้เลยครับ"},
	}
	bounds := []sceneBound{{Start: 0, End: 5}}

	segs := captionSegmentsFromScenes(scenes, bounds)
	if len(segs) == 0 {
		t.Fatal("expected segments, got none")
	}
	var joined string
	for _, s := range segs {
		joined += s.Text
	}
	if !strings.Contains(joined, zwsp+"ไอดีแอดส์แวนซ์"+zwsp) {
		t.Errorf("แคปชั่นไม่ได้ประกบคำทับศัพท์ด้วย ZWSP: %q", joined)
	}
}

// emphasis ต้องยังจับคู่ได้หลัง guard — ZWSP อยู่นอกคำ ไม่ใช่ในคำ
func TestCaptionSegmentsFromScenes_EmphasisSurvivesGuard(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "ต้องถอดสิทธิ์แอดมินออกก่อนเสมอ", EmphasisWords: []string{"แอดมิน"}},
	}
	bounds := []sceneBound{{Start: 0, End: 5}}

	segs := captionSegmentsFromScenes(scenes, bounds)
	found := false
	for _, s := range segs {
		for _, e := range s.Emphasis {
			if e == "แอดมิน" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("คำเน้นหายไปหลัง guard: %+v", segs)
	}
}

// ZWSP ต้องไม่หลุดไปเส้นทาง TTS — ข้อความที่ส่งเข้า splitVoiceText ต้องดิบเสมอ
func TestSplitVoiceText_NeverSeesZWSP(t *testing.T) {
	raw := "ทักไลน์ไอดีแอดส์แวนซ์ได้เลยครับ ทีมงานตอบเร็วมาก"
	for _, chunk := range splitVoiceText(raw, 40) {
		if strings.Contains(chunk, zwsp) {
			t.Errorf("ZWSP รั่วเข้า TTS: %q", chunk)
		}
	}
}
