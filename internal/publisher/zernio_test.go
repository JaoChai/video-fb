package publisher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
