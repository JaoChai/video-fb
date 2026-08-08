package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestZernioClient(baseURL, apiKey string) *ZernioClient {
	return &ZernioClient{
		fallbackKey: apiKey,
		pool:        nil,
		client:      &http.Client{Timeout: 5 * time.Second},
		baseURL:     baseURL,
	}
}

func TestGetAnalytics_UsesQueryParamPostID(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"postId":"P123",
			"analytics":{"impressions":100,"reach":80,"likes":10,"comments":2,"shares":1,"saves":0,"clicks":5,"views":50,"engagementRate":0.18,"lastUpdated":"2026-05-12T00:00:00Z"},
			"platformAnalytics":[{"platform":"youtube","platformPostId":"yt_abc","accountId":"acc1","analytics":{"impressions":100,"reach":80,"likes":10,"comments":2,"shares":1,"saves":0,"clicks":5,"views":50,"engagementRate":0.18,"lastUpdated":"2026-05-12T00:00:00Z"},"syncStatus":"synced"}]
		}`))
	}))
	defer srv.Close()

	z := newTestZernioClient(srv.URL, "test_key")

	resp, err := z.GetAnalytics(context.Background(), "P123", "youtube")
	if err != nil {
		t.Fatalf("GetAnalytics err: %v", err)
	}
	if !strings.Contains(capturedURL, "postId=P123") || !strings.Contains(capturedURL, "platform=youtube") {
		t.Fatalf("expected postId+platform as query params, got %s", capturedURL)
	}
	if resp.PostID != "P123" {
		t.Fatalf("expected PostID=P123, got %q", resp.PostID)
	}
	if len(resp.PlatformAnalytics) != 1 || resp.PlatformAnalytics[0].PlatformPostID != "yt_abc" {
		t.Fatalf("expected platformAnalytics[0].PlatformPostID=yt_abc, got %+v", resp.PlatformAnalytics)
	}
	if resp.PlatformAnalytics[0].Analytics.Views != 50 {
		t.Fatalf("expected views=50, got %d", resp.PlatformAnalytics[0].Analytics.Views)
	}
}

func TestGetYouTubeDailyViews_AggregatesWatchTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"videoId": "abc",
			"totalViews": 100,
			"dailyViews": [
				{"date":"2026-05-10","views":60,"estimatedMinutesWatched":30.0,"averageViewDuration":30.0},
				{"date":"2026-05-11","views":40,"estimatedMinutesWatched":20.0,"averageViewDuration":30.0}
			],
			"scopeStatus":{"hasAnalyticsScope":true}
		}`))
	}))
	defer srv.Close()

	z := newTestZernioClient(srv.URL, "k")
	resp, err := z.GetYouTubeDailyViews(context.Background(), "abc", "acc1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalViews != 100 || len(resp.DailyViews) != 2 {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

func TestGetYouTubeDailyViews_ScopeMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(412)
		_, _ = w.Write([]byte(`{"success":false,"error":"scope missing","code":"youtube_analytics_scope_missing"}`))
	}))
	defer srv.Close()

	z := newTestZernioClient(srv.URL, "k")
	_, err := z.GetYouTubeDailyViews(context.Background(), "abc", "acc1")
	if !errors.Is(err, ErrYouTubeScopeMissing) {
		t.Fatalf("expected ErrYouTubeScopeMissing, got %v", err)
	}
}

// captureBody รัน Post กับ httptest server แล้วคืน JSON ดิบที่ยิงออกไป
func captureBody(t *testing.T, req PostRequest) string {
	t.Helper()
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"post":{"_id":"P1"}}`))
	}))
	defer srv.Close()

	z := newTestZernioClient(srv.URL, "test_key")
	if _, err := z.Post(context.Background(), req); err != nil {
		t.Fatalf("Post err: %v", err)
	}
	return body
}

func TestPost_SendsFirstCommentInPlatformSpecificData(t *testing.T) {
	body := captureBody(t, PostRequest{
		Title:   "หัวข้อคลิป",
		Content: "หัวข้อคลิป\n\nคำอธิบาย",
		Platforms: []PlatformTarget{{
			Platform:  "youtube",
			AccountID: "acc1",
			PlatformSpecificData: &YouTubeOptions{
				Title:        "หัวข้อคลิป",
				Visibility:   VisibilityPublic,
				FirstComment: "ติดต่อทีมงานได้ที่ LINE id : @adsvance",
			},
		}},
		Visibility: VisibilityPublic,
		PublishNow: true,
	})

	for _, want := range []string{
		`"platformSpecificData"`,
		`"firstComment"`,
		`LINE id : @adsvance`,
		`"visibility":"public"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %s, got %s", want, body)
		}
	}

	// ตรวจซ้ำด้วย unmarshal เจาะจง nesting: substring match ข้างบนจะผ่านแม้ title/
	// visibility/firstComment ถูก serialize ผิดระดับ (เช่นหลุดไปอยู่ platforms[0] ตรงๆ)
	// เพราะ "visibility":"public" แมตช์ฟิลด์บนสุดได้เหมือนกัน ต้องยืนยันว่าค่าทั้งสามอยู่
	// ใน platformSpecificData จริง ไม่ใช่แค่มีอยู่ที่ไหนสักแห่งใน body
	var parsed struct {
		Platforms []struct {
			PlatformSpecificData *struct {
				Title        string `json:"title"`
				Visibility   string `json:"visibility"`
				FirstComment string `json:"firstComment"`
			} `json:"platformSpecificData"`
		} `json:"platforms"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("unmarshal body: %v (body=%s)", err, body)
	}
	if len(parsed.Platforms) != 1 || parsed.Platforms[0].PlatformSpecificData == nil {
		t.Fatalf("expected 1 platform with platformSpecificData, got %+v", parsed.Platforms)
	}
	psd := parsed.Platforms[0].PlatformSpecificData
	if psd.Title != "หัวข้อคลิป" {
		t.Fatalf("expected nested title %q, got %q", "หัวข้อคลิป", psd.Title)
	}
	if psd.Visibility != VisibilityPublic {
		t.Fatalf("expected nested visibility %q, got %q", VisibilityPublic, psd.Visibility)
	}
	if psd.FirstComment != "ติดต่อทีมงานได้ที่ LINE id : @adsvance" {
		t.Fatalf("expected nested firstComment %q, got %q", "ติดต่อทีมงานได้ที่ LINE id : @adsvance", psd.FirstComment)
	}
}

func TestPost_OmitsPlatformSpecificDataWhenUnset(t *testing.T) {
	body := captureBody(t, PostRequest{
		Title:      "หัวข้อคลิป",
		Content:    "หัวข้อคลิป",
		Platforms:  []PlatformTarget{{Platform: "youtube", AccountID: "acc1"}},
		Visibility: VisibilityPublic,
		PublishNow: true,
	})

	if strings.Contains(body, "platformSpecificData") {
		t.Fatalf("expected no platformSpecificData key when unset, got %s", body)
	}
}

func TestNewZernioClient_PostClientTimeout(t *testing.T) {
	z := NewZernioClient("k", nil)
	if z.postClient == nil {
		t.Fatal("postClient must be initialized")
	}
	if z.postClient.Timeout != 5*time.Minute {
		t.Fatalf("postClient timeout = %v, want 5m", z.postClient.Timeout)
	}
}

// struct literal ที่ไม่ตั้ง postClient (แพทเทิร์นเทสต์เดิมทั้งไฟล์) ต้องยังใช้ Post ได้
func TestPost_NilPostClientFallsBackToClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"post":{"_id":"p1"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	resp, err := z.Post(context.Background(), PostRequest{Content: "x"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if resp.Post.ID != "p1" {
		t.Fatalf("got %q", resp.Post.ID)
	}
}

func TestPost_SendsXRequestIDHeader(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("x-request-id")
		w.Write([]byte(`{"post":{"_id":"p1"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	if _, err := z.Post(context.Background(), PostRequest{Content: "x", RequestID: "rid-1"}); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if got != "rid-1" {
		t.Fatalf("x-request-id = %q, want rid-1", got)
	}
}

func TestPost_AdoptsExistingPostOnReplay(t *testing.T) {
	// Zernio ตอบ 200 + existingPost เมื่อ x-request-id ซ้ำภายใน ~5 นาที (docs/guides/idempotency)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"existingPost":{"_id":"orig-1"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	resp, err := z.Post(context.Background(), PostRequest{Content: "x", RequestID: "rid-1"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if resp.Post.ID != "orig-1" {
		t.Fatalf("Post.ID = %q, want orig-1", resp.Post.ID)
	}
}

func TestPostRequestID_DeterministicAndDistinct(t *testing.T) {
	a := postRequestID("clip-1", "916")
	b := postRequestID("clip-1", "916")
	c := postRequestID("clip-1", "169")
	if a != b {
		t.Fatalf("not deterministic: %q vs %q", a, b)
	}
	if a == c {
		t.Fatal("formats must produce distinct ids")
	}
	if len(a) != 36 {
		t.Fatalf("want uuid-shaped id, got %q", a)
	}
}

func TestPost_409ReturnsDuplicatePostError(t *testing.T) {
	// body จริงที่วัดได้ 08-08: existingPostId อยู่ใน details (docs แสดง top-level ด้วย)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"Duplicate","details":{"accountId":"a","platform":"youtube","existingPostId":"dup-1"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	_, err := z.Post(context.Background(), PostRequest{Content: "x"})
	var dup *DuplicatePostError
	if !errors.As(err, &dup) {
		t.Fatalf("want DuplicatePostError, got %v", err)
	}
	if dup.ExistingPostID != "dup-1" {
		t.Fatalf("ExistingPostID = %q", dup.ExistingPostID)
	}
}

func TestPost_409TopLevelExistingPostID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"Duplicate","existingPostId":"dup-2"}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	_, err := z.Post(context.Background(), PostRequest{Content: "x"})
	var dup *DuplicatePostError
	if !errors.As(err, &dup) || dup.ExistingPostID != "dup-2" {
		t.Fatalf("got %v", err)
	}
}

func TestGetPost_ReturnsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/posts/p9" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`{"post":{"_id":"p9","status":"published"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	ps, err := z.GetPost(context.Background(), "p9")
	if err != nil {
		t.Fatalf("GetPost: %v", err)
	}
	if ps.ID != "p9" || ps.Status != "published" {
		t.Fatalf("got %+v", ps)
	}
}

func TestAdoptDuplicate_PublishedAdopts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"post":{"_id":"dup-1","status":"published"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	p := &Publisher{zernio: z}
	id, ok := p.adoptDuplicate(context.Background(), &DuplicatePostError{ExistingPostID: "dup-1"})
	if !ok || id != "dup-1" {
		t.Fatalf("got id=%q ok=%v", id, ok)
	}
}

func TestAdoptDuplicate_NotPublishedDoesNotAdopt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"post":{"_id":"dup-1","status":"scheduled"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	p := &Publisher{zernio: z}
	if _, ok := p.adoptDuplicate(context.Background(), &DuplicatePostError{ExistingPostID: "dup-1"}); ok {
		t.Fatal("must not adopt a non-published post")
	}
}

func TestAdoptDuplicate_NonDuplicateError(t *testing.T) {
	p := &Publisher{}
	if _, ok := p.adoptDuplicate(context.Background(), errors.New("boom")); ok {
		t.Fatal("plain errors must not adopt")
	}
}
