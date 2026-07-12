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

// threshold ที่ส่งเข้า filterBySimilarity ควบคุม cutoff (ไม่ใช่ const ตายตัว)
func TestFilterBySimilarity_CustomThreshold(t *testing.T) {
	questions := []GeneratedQuestion{{Question: "q1"}, {Question: "q2"}}
	sims := map[string]SimilarityMatch{
		"q1": {Similarity: 0.75, MatchedTitle: "old"},
		"q2": {Similarity: 0.60, MatchedTitle: "old2"},
	}
	// threshold 0.72 → q1 (0.75>=0.72) reject, q2 (0.60<0.72) pass
	passed, rejected := filterBySimilarity(questions, sims, 0.72)
	if len(passed) != 1 || passed[0].Question != "q2" {
		t.Errorf("expected q2 to pass at threshold 0.72, got passed=%+v", passed)
	}
	if len(rejected) != 1 || rejected[0].Question.Question != "q1" {
		t.Errorf("expected q1 rejected at 0.72, got rejected=%+v", rejected)
	}
}

// SetThreshold เปลี่ยนค่าที่ Deduper ใช้
func TestDeduper_SetThreshold(t *testing.T) {
	d := &Deduper{}
	d.SetThreshold(0.72)
	if d.threshold != 0.72 {
		t.Errorf("expected threshold 0.72, got %v", d.threshold)
	}
	d.SetThreshold(0) // ค่าไร้สาระ → ไม่เปลี่ยน
	if d.threshold != 0.72 {
		t.Errorf("zero threshold should not overwrite, got %v", d.threshold)
	}
}

// lexicalCheckIsSystemicFailure ต้องแยก "ไม่มีอะไรให้เทียบ" (past ว่าง) ออกจาก
// "เทียบไม่ได้เลยสักคู่" (pg_trgm ล่มทั้งระบบ) — เคสหลังต้อง propagate error
// ไม่ใช่คืน map ว่างเงียบๆ (มิฉะนั้นจะเหมือน accept-all แบบไม่มี log)
func TestLexicalCheckIsSystemicFailure(t *testing.T) {
	cases := []struct {
		name                        string
		pastLen, attempted, errored int
		want                        bool
	}{
		{"no past titles yet (brand new system)", 0, 0, 0, false},
		{"all pairs succeeded", 5, 5, 0, false},
		{"some pairs errored, some succeeded (transient)", 5, 5, 3, false},
		{"every attempted pair errored (systemic pg_trgm failure)", 5, 5, 5, true},
		{"past titles exist but nothing attempted (no questions)", 5, 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := lexicalCheckIsSystemicFailure(c.pastLen, c.attempted, c.errored)
			if got != c.want {
				t.Errorf("lexicalCheckIsSystemicFailure(%d, %d, %d) = %v, want %v",
					c.pastLen, c.attempted, c.errored, got, c.want)
			}
		})
	}
}
