package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// extractJSON must return a string that json.Unmarshal accepts even when the
// LLM emits raw control characters (newlines, tabs) inside string values — a
// common failure mode of the kie.ai Claude proxy. Regression for the
// "invalid character '\n' in string literal" production failures.
func TestExtractJSON_RawNewlineInString(t *testing.T) {
	// A code-fenced object whose "description" value contains a literal newline.
	raw := "```json\n{\n  \"youtube_title\": \"ETDA พ.ย. 2026\",\n  \"youtube_description\": \"บรรทัดแรก\nบรรทัดสอง\"\n}\n```"

	cleaned := extractJSON(raw)

	var out struct {
		YoutubeTitle       string `json:"youtube_title"`
		YoutubeDescription string `json:"youtube_description"`
	}
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		t.Fatalf("Unmarshal failed on cleaned JSON: %v\ncleaned: %q", err, cleaned)
	}
	if out.YoutubeTitle != "ETDA พ.ย. 2026" {
		t.Errorf("title = %q, want %q", out.YoutubeTitle, "ETDA พ.ย. 2026")
	}
	if !strings.Contains(out.YoutubeDescription, "บรรทัดแรก") || !strings.Contains(out.YoutubeDescription, "บรรทัดสอง") {
		t.Errorf("description lost content: %q", out.YoutubeDescription)
	}
}

// A raw tab inside a string value must also survive extraction.
func TestExtractJSON_RawTabInString(t *testing.T) {
	raw := "{\"a\": \"x\ty\"}"

	cleaned := extractJSON(raw)

	var out map[string]string
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		t.Fatalf("Unmarshal failed: %v\ncleaned: %q", err, cleaned)
	}
	if out["a"] != "x\ty" {
		t.Errorf("value = %q, want %q", out["a"], "x\ty")
	}
}

// A raw control byte immediately following a backslash (a spurious escape the
// proxy sometimes emits) must still be repaired, not passed through.
func TestExtractJSON_RawControlAfterBackslash(t *testing.T) {
	raw := "{\"a\": \"line1\\\nline2\"}" // ...line1 <backslash> <raw LF> line2...

	cleaned := extractJSON(raw)

	var out map[string]string
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		t.Fatalf("Unmarshal failed: %v\ncleaned: %q", err, cleaned)
	}
	if !strings.Contains(out["a"], "line1") || !strings.Contains(out["a"], "line2") {
		t.Errorf("value lost content: %q", out["a"])
	}
}

// A legitimate escaped-backslash pair followed by an ASCII letter must survive
// unchanged (guardrail so the backslash lookahead doesn't over-repair).
func TestExtractJSON_PreservesEscapedBackslash(t *testing.T) {
	raw := `{"a": "x\\ny"}` // decodes to: x \ n y

	cleaned := extractJSON(raw)

	var out map[string]string
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		t.Fatalf("Unmarshal failed: %v\ncleaned: %q", err, cleaned)
	}
	if out["a"] != `x\ny` {
		t.Errorf("value = %q, want %q", out["a"], `x\ny`)
	}
}

// Already-valid JSON must pass through unchanged (no double-escaping of an
// escaped \n sequence that the LLM emitted correctly).
func TestExtractJSON_PreservesEscapedNewline(t *testing.T) {
	raw := `{"a": "line1\nline2"}`

	cleaned := extractJSON(raw)

	var out map[string]string
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		t.Fatalf("Unmarshal failed: %v\ncleaned: %q", err, cleaned)
	}
	if out["a"] != "line1\nline2" {
		t.Errorf("value = %q, want %q", out["a"], "line1\nline2")
	}
}
