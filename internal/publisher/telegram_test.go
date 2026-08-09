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

// 409 ต้องถูกจับเป็น "เคยส่งแล้ว" ผ่าน adoptDuplicate ซึ่งเช็คสถานะโพสต์เดิมก่อนเสมอ (ไม่ใช่
// เชื่อ ExistingPostID ตรงๆ) — mock ต้องตอบทั้ง POST (409) และ GET /posts/dup-1 (published)
// ยืนยันผ่าน log เพราะ pool == nil ในเทสต์ตัดขั้นเขียน DB ออก จึงไม่มีทางสังเกตผลลัพธ์
// จากภายนอกได้นอกจากข้อความ log ที่แต่ละ branch เขียนไว้คนละข้อความ
func TestSendTelegram_DuplicateSavesExistingPostID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"post":{"_id":"dup-1","status":"published"}}`))
			return
		}
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

// ถ้าโพสต์เดิมยังไม่ published จริง (scheduled/publishing/failed) ต้องไม่ adopt — ปฏิบัติ
// เหมือน error ทั่วไป (mark failed ให้รอบเก็บตกลองใหม่) ไม่ใช่บันทึกเป็นสำเร็จทั้งที่ของจริง
// ยังไม่ขึ้น (บั๊กเดียวกับที่เคยเกิดกับคิว YouTube 08-08)
func TestSendTelegram_DuplicateNotYetPublishedIsNotAdopted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"post":{"_id":"dup-1","status":"scheduled"}}`))
			return
		}
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
	if strings.Contains(got, "ซ้ำกับโพสต์ dup-1 ที่เคยส่งแล้ว บันทึกเป็นส่งแล้ว") {
		t.Fatalf("โพสต์ยังไม่ published ต้องไม่ adopt แต่ log บอกว่า adopt แล้ว: %s", got)
	}
	if !strings.Contains(got, "เข้าช่องไม่สำเร็จ") {
		t.Fatalf("expected generic-failure log line, got: %s", got)
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

func TestPostTelegramForClip_DisabledWhenNoAccount(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"post":{"_id":"X"}}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	p.postTelegramForClip(context.Background(), "clip-1", "", "หัวข้อ", "MAIN", "SHORTS")

	if calls != 0 {
		t.Fatalf("account id ว่าง = ปิดสนิท ต้องไม่ยิง HTTP เลย แต่ยิงไป %d ครั้ง", calls)
	}
}

func TestPostTelegramForClip_PrefersMainPostAndPostsOnce(t *testing.T) {
	var gotPaths []string
	var content string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"post":{"_id":"MAIN","status":"published","platforms":[
				{"platform":"youtube","status":"published","platformPostId":"vid1","platformPostUrl":"https://www.youtube.com/watch?v=vid1"}]}}`))
			return
		}
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		content, _ = body["content"].(string)
		_, _ = w.Write([]byte(`{"post":{"_id":"TG1"}}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	p.postTelegramForClip(context.Background(), "clip-1", "tg_acc", "หัวข้อ", "MAIN", "SHORTS")

	if len(gotPaths) != 2 {
		t.Fatalf("คาด GET /posts/MAIN แล้ว POST /posts อย่างละครั้ง ได้ %v", gotPaths)
	}
	if gotPaths[0] != "GET /posts/MAIN" {
		t.Fatalf("ต้องใช้โพสต์ 16:9 ก่อนเสมอ ได้ %q", gotPaths[0])
	}
	if !strings.Contains(content, "https://www.youtube.com/watch?v=vid1") {
		t.Fatalf("ข้อความไม่มีลิงก์ที่ได้จาก GetPost: %q", content)
	}
}

func TestPostTelegramForClip_FallsBackToShorts(t *testing.T) {
	var firstPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if firstPath == "" {
			firstPath = r.URL.Path
		}
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"post":{"_id":"SHORTS","status":"published","platforms":[
				{"platform":"youtube","platformPostUrl":"https://www.youtube.com/watch?v=vid2"}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"post":{"_id":"TG1"}}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	p.postTelegramForClip(context.Background(), "clip-1", "tg_acc", "หัวข้อ", "", "SHORTS")

	if firstPath != "/posts/SHORTS" {
		t.Fatalf("ไม่มีโพสต์ 16:9 ต้องใช้ Shorts แทน ได้ %q", firstPath)
	}
}

func TestPostTelegramForClip_NoURLSkipsSending(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// ยังไม่มี platformPostUrl (Zernio ยังไม่อัปเดต) = ยังไม่พร้อมส่ง
			_, _ = w.Write([]byte(`{"post":{"_id":"MAIN","status":"publishing","platforms":[{"platform":"youtube"}]}}`))
			return
		}
		posts++
		_, _ = w.Write([]byte(`{"post":{"_id":"TG1"}}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	p.postTelegramForClip(context.Background(), "clip-1", "tg_acc", "หัวข้อ", "MAIN", "")

	if posts != 0 {
		t.Fatalf("ไม่มีลิงก์ต้องไม่ส่ง ปล่อยให้รอบเก็บตกจัดการ แต่ส่งไป %d ครั้ง", posts)
	}
}
