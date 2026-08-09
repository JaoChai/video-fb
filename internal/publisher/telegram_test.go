package publisher

import (
	"strings"
	"testing"
)

func TestTelegramMessage(t *testing.T) {
	cases := []struct {
		name  string
		title string
		url   string
		want  string
	}{
		{
			name:  "ตัดแบรนด์แท็กท้ายชื่อ",
			title: "แยกผลรายตำแหน่งใน Ads Manager แล้วตัดตัวเผางบ | Ads Vance",
			url:   "https://www.youtube.com/watch?v=iJq2W3-vRbs",
			want:  "แยกผลรายตำแหน่งใน Ads Manager แล้วตัดตัวเผางบ\n\nhttps://www.youtube.com/watch?v=iJq2W3-vRbs",
		},
		{
			name:  "ไม่มีแบรนด์แท็กก็ส่งตามเดิม",
			title: "ตั้งงบยังไงไม่ให้บาน",
			url:   "https://www.youtube.com/watch?v=abc",
			want:  "ตั้งงบยังไงไม่ให้บาน\n\nhttps://www.youtube.com/watch?v=abc",
		},
		{
			name:  "escape อักขระที่ Telegram ตีเป็น HTML",
			title: "A&B <คู่แข่ง> ใครแพงกว่า",
			url:   "https://www.youtube.com/watch?v=abc",
			want:  "A&amp;B &lt;คู่แข่ง&gt; ใครแพงกว่า\n\nhttps://www.youtube.com/watch?v=abc",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := telegramMessage(c.title, c.url); got != c.want {
				t.Errorf("telegramMessage() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestTelegramPlatforms(t *testing.T) {
	got := telegramPlatforms("tg_acc")
	if len(got) != 1 {
		t.Fatalf("expected 1 target, got %d", len(got))
	}
	if got[0].Platform != platformTelegram || got[0].AccountID != "tg_acc" {
		t.Fatalf("unexpected target %+v", got[0])
	}
	// ไม่แนบ platformSpecificData: Zernio ใช้ parseMode HTML เป็นค่า default อยู่แล้ว
	// และฟิลด์นี้ผูก type กับ YouTubeOptions — แนบไปจะทำให้ต้องแก้เส้นทาง YouTube โดยไม่จำเป็น
	if got[0].PlatformSpecificData != nil {
		t.Fatalf("expected nil PlatformSpecificData, got %+v", got[0].PlatformSpecificData)
	}
}

func TestYoutubePostURL(t *testing.T) {
	ps := &PostStatus{Platforms: []PostPlatform{
		{Platform: "telegram", PlatformPostURL: "https://t.me/x/1"},
		{Platform: "youtube", PlatformPostURL: "https://www.youtube.com/watch?v=abc"},
	}}
	if got := youtubePostURL(ps); got != "https://www.youtube.com/watch?v=abc" {
		t.Fatalf("expected youtube url, got %q", got)
	}
	if got := youtubePostURL(&PostStatus{}); got != "" {
		t.Fatalf("expected empty string when no platforms, got %q", got)
	}
	// ยังไม่มีลิงก์ = ยังไม่พร้อมส่ง ต้องคืนค่าว่าง ไม่ใช่เดา URL เอง
	noURL := &PostStatus{Platforms: []PostPlatform{{Platform: "youtube", PlatformPostID: "abc"}}}
	if got := youtubePostURL(noURL); got != "" {
		t.Fatalf("expected empty string when url missing, got %q", got)
	}
}

func TestStripBrandTagStillUsedByTikTok(t *testing.T) {
	if got := stripBrandTag("ตั้งงบยังไง | Ads Vance"); !strings.HasSuffix(got, "ตั้งงบยังไง") {
		t.Fatalf("stripBrandTag ไม่ได้ตัดแบรนด์แท็ก: %q", got)
	}
}
