package publisher

import "testing"

func TestNormalizePublishStatus(t *testing.T) {
	cases := map[string]string{
		"published":  "published",
		"failed":     "failed",
		"scheduled":  "scheduled",
		"pending":    "scheduled",
		"processing": "scheduled",
		"":           "unknown",
		"weird":      "unknown",
	}
	for in, want := range cases {
		if got := normalizePublishStatus(in); got != want {
			t.Errorf("normalizePublishStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolvePostStatus(t *testing.T) {
	var failed AnalyticsResponse
	loadFixture(t, "analytics_tiktok_failed.json", &failed)
	status, errMsg := resolvePostStatus(&failed, "tiktok")
	if status != "failed" {
		t.Errorf("status = %q, want failed", status)
	}
	if errMsg == "" {
		t.Error("errMsg empty, want TikTok error text")
	}

	var ok AnalyticsResponse
	loadFixture(t, "analytics_tiktok_published.json", &ok)
	status, errMsg = resolvePostStatus(&ok, "tiktok")
	if status != "published" || errMsg != "" {
		t.Errorf("status/err = %q/%q, want published/empty", status, errMsg)
	}

	// Platform entry missing entirely → fall back to top-level status/message.
	empty := &AnalyticsResponse{Status: "scheduled", Message: "sync pending"}
	status, errMsg = resolvePostStatus(empty, "tiktok")
	if status != "scheduled" {
		t.Errorf("fallback status = %q, want scheduled", status)
	}
	if errMsg != "sync pending" {
		t.Errorf("fallback errMsg = %q, want %q", errMsg, "sync pending")
	}
}
