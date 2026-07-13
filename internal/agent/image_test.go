package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// Locks the prompt↔struct contract for the cover-prompt output (migration 054
// template): the LLM must emit {"image_prompt": "..."}.
func TestCoverPromptOutputParsesSchema(t *testing.T) {
	raw := `{"image_prompt": "a rejected-status warning dialog floating above a dark desk, main subject in upper half"}`
	var out coverPromptOut
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("cover prompt JSON did not unmarshal: %v", err)
	}
	if out.ImagePrompt == "" {
		t.Error("ImagePrompt is empty")
	}
}

// Guards CoverImageTemplateData field names against the migration-054 template
// vars ({{.QuestionText}}, {{.Category}}, {{.HookText}}).
func TestCoverImageTemplateRendersVars(t *testing.T) {
	tmpl := "q={{.QuestionText}} cat={{.Category}} hook={{.HookText}}"
	out, err := renderTemplate(tmpl, CoverImageTemplateData{
		QuestionText: "Q", Category: "account", HookText: "H",
	})
	if err != nil {
		t.Fatalf("renderTemplate err: %v", err)
	}
	for _, want := range []string{"q=Q", "cat=account", "hook=H"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q: %s", want, out)
		}
	}
}
