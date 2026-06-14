package agent

import (
	"encoding/json"
	"testing"
)

func TestAcceptProposal_Accepts(t *testing.T) {
	in := LearnInput{CurrentSkills: "- เดิม"}
	out := &LearnOutput{NewSkills: "- เดิม\n- เพิ่ม hook ตัวเลขช็อก", Rationale: "hook ต่ำ", Confident: true}
	if !AcceptProposal(in, out) {
		t.Fatal("AcceptProposal = false, want true for a confident, non-empty, changed proposal")
	}
}

func TestAcceptProposal_RejectsNotConfident(t *testing.T) {
	in := LearnInput{CurrentSkills: "- เดิม"}
	out := &LearnOutput{NewSkills: "- ใหม่", Confident: false}
	if AcceptProposal(in, out) {
		t.Fatal("AcceptProposal = true, want false when not confident")
	}
}

func TestAcceptProposal_RejectsEmpty(t *testing.T) {
	in := LearnInput{CurrentSkills: "- เดิม"}
	for _, blank := range []string{"", "   ", "\n\t  "} {
		out := &LearnOutput{NewSkills: blank, Confident: true}
		if AcceptProposal(in, out) {
			t.Fatalf("AcceptProposal = true, want false for blank new_skills %q", blank)
		}
	}
}

func TestAcceptProposal_RejectsIdentical(t *testing.T) {
	in := LearnInput{CurrentSkills: "  - เดิม  "}
	out := &LearnOutput{NewSkills: "- เดิม", Confident: true}
	if AcceptProposal(in, out) {
		t.Fatal("AcceptProposal = true, want false when new == current (after trim)")
	}
}

func TestAcceptProposal_RejectsNil(t *testing.T) {
	if AcceptProposal(LearnInput{CurrentSkills: "x"}, nil) {
		t.Fatal("AcceptProposal = true, want false for nil output")
	}
}

// Locks the prompt<->struct contract: the JSON the learner is told to emit must
// unmarshal cleanly into LearnOutput.
func TestLearnOutputParsesSchema(t *testing.T) {
	raw := `{ "new_skills": "- ปรับ hook", "rationale": "hook ต่ำสุด", "confident": true }`
	var out LearnOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("LearnOutput did not unmarshal: %v", err)
	}
	if !out.Confident || out.NewSkills != "- ปรับ hook" || out.Rationale != "hook ต่ำสุด" {
		t.Errorf("unexpected parse: %+v", out)
	}
}
