package agent

import "testing"

func TestFilterBySimilarity(t *testing.T) {
	questions := []GeneratedQuestion{
		{Question: "Pixel ไม่นับยอดขาย"},
		{Question: "บัญชีโดนแบนกู้คืนยังไง"},
		{Question: "CBO งบกระจุก ad set เดียว"},
	}
	similarities := map[string]SimilarityMatch{
		"Pixel ไม่นับยอดขาย":          {Similarity: 0.92, MatchedTitle: "Pixel ติดตั้งแล้วไม่ทำงาน"},
		"บัญชีโดนแบนกู้คืนยังไง":      {Similarity: 0.60, MatchedTitle: "เปิดบัญชีใหม่"},
		"CBO งบกระจุก ad set เดียว": {Similarity: 0.86, MatchedTitle: "CBO ใช้เงินแค่ ad set เดียว"},
	}

	passed, rejected := filterBySimilarity(questions, similarities, 0.85)

	if len(passed) != 1 {
		t.Fatalf("expected 1 passed, got %d", len(passed))
	}
	if passed[0].Question != "บัญชีโดนแบนกู้คืนยังไง" {
		t.Errorf("wrong question passed: %s", passed[0].Question)
	}
	if len(rejected) != 2 {
		t.Fatalf("expected 2 rejected, got %d", len(rejected))
	}
}

func TestFilterBySimilarityNoMatches(t *testing.T) {
	questions := []GeneratedQuestion{{Question: "คำถามใหม่"}}
	passed, rejected := filterBySimilarity(questions, map[string]SimilarityMatch{}, 0.85)
	if len(passed) != 1 || len(rejected) != 0 {
		t.Errorf("expected all pass when no similarity data, got passed=%d rejected=%d", len(passed), len(rejected))
	}
}
