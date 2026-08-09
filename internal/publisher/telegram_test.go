package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestSendTelegram_PostsExpectedPayload(t *testing.T) {
	var body map[string]any
	var reqID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		reqID = r.Header.Get("x-request-id")
		_, _ = w.Write([]byte(`{"post":{"_id":"TG1"}}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	p.sendTelegram(context.Background(), "clip-1", "tg_acc",
		"ตั้งงบยังไงไม่ให้บาน | Ads Vance", "https://www.youtube.com/watch?v=abc")

	if body["content"] != "ตั้งงบยังไงไม่ให้บาน\n\nhttps://www.youtube.com/watch?v=abc" {
		t.Fatalf("unexpected content: %v", body["content"])
	}
	if body["publishNow"] != true {
		t.Fatalf("expected publishNow=true, got %v", body["publishNow"])
	}
	platforms, ok := body["platforms"].([]any)
	if !ok || len(platforms) != 1 {
		t.Fatalf("expected 1 platform, got %v", body["platforms"])
	}
	first := platforms[0].(map[string]any)
	if first["platform"] != "telegram" || first["accountId"] != "tg_acc" {
		t.Fatalf("unexpected platform target: %v", first)
	}
	// Telegram ไม่มีชื่อโพสต์ ส่ง title ไปก็ไม่มีที่ลง — ต้องไม่ติดไปกับคำขอ
	if _, has := body["title"]; has {
		t.Fatalf("expected no title field, got %v", body["title"])
	}
	if reqID != postRequestID("clip-1", "telegram") {
		t.Fatalf("expected deterministic x-request-id, got %q", reqID)
	}
}

// 409 ต้องถูกจับเป็น "เคยส่งแล้ว" ไม่ใช่ error ทั่วไป — ยืนยันผ่าน log เพราะ pool == nil
// ในเทสต์ตัดขั้นเขียน DB ออก (แพตเทิร์นเดียวกับเทสต์อื่นในไฟล์นี้) จึงไม่มีทางสังเกต
// ผลลัพธ์จากภายนอกได้นอกจากข้อความ log ที่แต่ละ branch เขียนไว้คนละข้อความ
func TestSendTelegram_DuplicateSavesExistingPostID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"Duplicate","details":{"existingPostId":"dup-1"}}`))
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	p.sendTelegram(context.Background(), "clip-1", "tg_acc", "หัวข้อ", "https://youtu.be/x")

	got := logBuf.String()
	if !strings.Contains(got, "ซ้ำกับโพสต์ dup-1") {
		t.Fatalf("expected duplicate-handling log line, got: %s", got)
	}
	if strings.Contains(got, "เข้าช่องไม่สำเร็จ") {
		t.Fatalf("409 ถูกปฏิบัติเหมือน error ทั่วไป ไม่ใช่ duplicate: %s", got)
	}
}

func TestSendTelegram_ErrorDoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	// ต้องกลับมาเงียบๆ ไม่ panic ไม่ค้าง — คลิปขึ้น YouTube ไปแล้ว ห้ามให้ Telegram ทำพัง
	p.sendTelegram(context.Background(), "clip-1", "tg_acc", "หัวข้อ", "https://youtu.be/x")
}
