package agent

import "testing"

func TestNormalizeAutoReview(t *testing.T) {
	cases := []struct {
		name      string
		raw       AutoReviewDecision
		threshold float64
		want      string
	}{
		{"approve high confidence", AutoReviewDecision{Decision: "approve", Confidence: 0.9}, 0.8, "approve"},
		{"approve below threshold -> hold", AutoReviewDecision{Decision: "approve", Confidence: 0.6}, 0.8, "hold"},
		{"retry passes through", AutoReviewDecision{Decision: "retry", Confidence: 0.9}, 0.8, "retry"},
		{"hold passes through", AutoReviewDecision{Decision: "hold", Confidence: 0.9}, 0.8, "hold"},
		{"unknown decision -> hold", AutoReviewDecision{Decision: "garbage", Confidence: 0.99}, 0.8, "hold"},
		{"empty decision -> hold", AutoReviewDecision{Decision: "", Confidence: 0.99}, 0.8, "hold"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeAutoReview(c.raw, c.threshold)
			if got.Decision != c.want {
				t.Fatalf("normalizeAutoReview(%+v) = %q, want %q", c.raw, got.Decision, c.want)
			}
		})
	}
}

func TestNormalizeAutoReviewError(t *testing.T) {
	got := autoReviewError("vision failed")
	if got.Decision != "hold" {
		t.Fatalf("autoReviewError decision = %q, want hold (fail-closed)", got.Decision)
	}
	if len(got.Reasons) == 0 {
		t.Fatalf("autoReviewError should carry a reason")
	}
}
