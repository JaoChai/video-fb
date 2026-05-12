package publisher

import (
	"context"
	"errors"
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
