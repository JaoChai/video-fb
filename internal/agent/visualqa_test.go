package agent

import (
	"encoding/json"
	"testing"
)

func TestSummarize_AllOK_Passes(t *testing.T) {
	v := []SceneVerdict{
		{SceneNumber: 1, OK: true},
		{SceneNumber: 2, OK: true},
	}
	if !summarizeVerdicts(v) {
		t.Fatal("want pass when every scene OK")
	}
}

func TestSummarize_AnyFail_Fails(t *testing.T) {
	v := []SceneVerdict{
		{SceneNumber: 1, OK: true},
		{SceneNumber: 2, OK: false, Issues: []string{"caption overflow"}},
		{SceneNumber: 3, OK: true},
	}
	if summarizeVerdicts(v) {
		t.Fatal("want fail when any scene not OK")
	}
}

func TestSummarize_Empty_Passes(t *testing.T) {
	if !summarizeVerdicts(nil) {
		t.Fatal("empty verdicts should fail-open (pass)")
	}
}

// Locks the single-frame reply contract: the JSON the model is told to emit must
// unmarshal cleanly into visionVerdict.
func TestVisionVerdictParsesSchema(t *testing.T) {
	raw := `{ "ok": false, "issues": ["ตัวหนังสือล้นกรอบ", "สีไม่ตรงแบรนด์"] }`
	var out visionVerdict
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("visionVerdict did not unmarshal: %v", err)
	}
	if out.OK || len(out.Issues) != 2 {
		t.Errorf("unexpected parse: %+v", out)
	}
}

func TestMarshalVerdicts_RoundTrips(t *testing.T) {
	in := []SceneVerdict{{SceneNumber: 1, OK: false, Issues: []string{"x"}}}
	b := MarshalVerdicts(in)
	var back []SceneVerdict
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("MarshalVerdicts produced invalid JSON: %v", err)
	}
	if len(back) != 1 || back[0].OK || back[0].SceneNumber != 1 {
		t.Errorf("round-trip mismatch: %+v", back)
	}
}
