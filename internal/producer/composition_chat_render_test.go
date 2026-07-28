package producer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chatParams() ScenesParams {
	mk := func(n int, layout string, c SceneContent) SceneSpec {
		c.SceneNumber, c.Layout = n, layout
		c.Start, c.End = float64(n-1)*5, float64(n)*5
		return SceneSpec{SceneNumber: n, StartSec: c.Start, EndSec: c.End,
			LayoutVariant: "hook_big", CaptionStyle: "phrase_block", Content: c}
	}
	return ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 25, Format: "chat", ThemeKey: "chat",
		Scenes: []SceneSpec{
			mk(1, "hook", SceneContent{Rows: []ContentRow{{Text: "บัญชีโดนแบนตอนตีสอง"}},
				BackgroundImage: "assets/bg-scene1.png"}),
			mk(2, "chat_in", SceneContent{
				Asker: "คุณเก่ง", Stamp: "21:47 น.",
				Msgs: []ContentMessage{
					{From: "them", Text: "พี่ครับ บัญชีโดนแบน"},
					{From: "them", Text: "เพิ่งผูกบัตรใหม่เมื่อวาน", Alert: true},
				}}),
			mk(3, "chat_out", SceneContent{
				Msgs: []ContentMessage{
					{From: "me", Text: "อย่าเพิ่งยื่นอุทธรณ์ครับ"},
					{From: "them", Text: "แล้วทำไงดี"},
				},
				Verdict: "รอ 72 ชั่วโมงแล้วยื่นครั้งเดียว"}),
			mk(4, "recap", SceneContent{Title: "จำ 2 ข้อนี้",
				Chips: []ContentChip{{N: "72", T: "ชั่วโมงที่ต้องรอ"}, {N: "1", T: "ครั้งเท่านั้น"}}}),
		},
		Segments: []TranscriptSegment{{Text: "บัญชีโดนแบน", Start: 0, End: 2}},
	}
}

func TestRenderChatFormat(t *testing.T) {
	out, err := RenderCompositionScenes(chatParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{
		`data-format="chat"`,
		"const FORMAT_CHAT = true",
		`data-layout="chat_in"`,
		`data-layout="chat_out"`,
		"คุณเก่ง",
		"21:47 น.",
		"พี่ครับ บัญชีโดนแบน",
		"รอ 72 ชั่วโมงแล้วยื่นครั้งเดียว",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered chat HTML missing %q", want)
		}
	}
}

// ฟองข้อความของเรากับของลูกค้าต้องแยกกันได้ในผลลัพธ์ ไม่งั้นคลิปอ่านไม่รู้ว่าใครพูด
func TestChatBubblesCarrySide(t *testing.T) {
	out, _ := RenderCompositionScenes(chatParams())
	html := string(out)
	if !strings.Contains(html, "ch-bub") {
		t.Fatal("chat bubbles must render with the ch-bub class")
	}
	for _, want := range []string{`"from":"them"`, `"from":"me"`} {
		if !strings.Contains(html, want) {
			t.Errorf("SCENES JSON missing %q — the renderer cannot tell the sides apart", want)
		}
	}
}

// โหมดอื่นต้องไม่ติดธง FORMAT_CHAT
func TestChatFlagOffForOtherModes(t *testing.T) {
	p := chatParams()
	p.Format, p.ThemeKey = "case", "case-file"
	out, _ := RenderCompositionScenes(p)
	if !strings.Contains(string(out), "const FORMAT_CHAT = false") {
		t.Error("FORMAT_CHAT must be false when the clip is not a chat clip")
	}
}

// TestDumpChatHTML เขียนผลเรนเดอร์ลงไฟล์ให้เปิดดูด้วยตา ปกติข้าม — ตั้ง
// HF_KEEP_DIR=<dir> เพื่อสั่งให้ทิ้งไฟล์ (ตั้งใจไม่ assert อะไร มันคือด่านสายตา)
func TestDumpChatHTML(t *testing.T) {
	dir := os.Getenv("HF_KEEP_DIR")
	if dir == "" {
		t.Skip("set HF_KEEP_DIR=<dir> to dump the chat HTML for eyeballing")
	}
	out, err := RenderCompositionScenes(chatParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chat.html"), out, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Logf("wrote %s/chat.html", dir)
}
