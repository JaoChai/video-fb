package publisher

import (
	"encoding/json"
	"os"
	"testing"
)

func loadFixture(t *testing.T, name string, v any) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
}

func TestParseTikTokPublished(t *testing.T) {
	var resp AnalyticsResponse
	loadFixture(t, "analytics_tiktok_published.json", &resp)

	if resp.Status != "published" {
		t.Errorf("status = %q, want published", resp.Status)
	}
	pa := resp.PlatformAnalytics[0]
	if pa.Status != "published" {
		t.Errorf("platform status = %q, want published", pa.Status)
	}
	if pa.ErrorMessage != "" {
		t.Errorf("errorMessage = %q, want empty", pa.ErrorMessage)
	}
	if pa.Analytics.Views != 120 || pa.Analytics.EngagementRate != 0.83 {
		t.Errorf("views/engagement = %d/%v, want 120/0.83", pa.Analytics.Views, pa.Analytics.EngagementRate)
	}
}

func TestParseTikTokFailed(t *testing.T) {
	var resp AnalyticsResponse
	loadFixture(t, "analytics_tiktok_failed.json", &resp)

	if resp.Status != "failed" {
		t.Errorf("status = %q, want failed", resp.Status)
	}
	pa := resp.PlatformAnalytics[0]
	if pa.Status != "failed" {
		t.Errorf("platform status = %q, want failed", pa.Status)
	}
	if pa.ErrorMessage == "" {
		t.Error("errorMessage empty, want TikTok download error")
	}
	// analytics is JSON null for failed posts — must not panic, must zero-value.
	if pa.Analytics.Views != 0 {
		t.Errorf("views = %d, want 0", pa.Analytics.Views)
	}
	if resp.Analytics.LastUpdated != "" {
		t.Errorf("flat lastUpdated = %q, want empty (failed post has no analytics)", resp.Analytics.LastUpdated)
	}
	if resp.Message == "" {
		t.Error("message empty, want failed-post explanation")
	}
}

func TestParseYouTubeDailyViews(t *testing.T) {
	var resp YouTubeDailyViewsResponse
	loadFixture(t, "youtube_daily_views.json", &resp)

	if len(resp.DailyViews) != 3 {
		t.Fatalf("daily entries = %d, want 3", len(resp.DailyViews))
	}
	d := resp.DailyViews[1]
	if d.AverageViewPercentage != 483.11 {
		t.Errorf("averageViewPercentage = %v, want 483.11", d.AverageViewPercentage)
	}
	if resp.DailyViews[2].SubscribersGained != 1 {
		t.Errorf("subscribersGained = %d, want 1", resp.DailyViews[2].SubscribersGained)
	}
}
