package agent

import "testing"

// The content_brain_v2 script prompt emits voice_script/answer_script. A reply
// with neither is empty narration — it must be rejected at the script stage so
// nothing empty flows into scene breakdown. Regression for the empty-script
// production failure.
func TestValidateGeneratedScript_BothBlank(t *testing.T) {
	err := validateGeneratedScript(&GeneratedScript{VoiceScript: "  ", AnswerScript: ""})
	if err == nil {
		t.Fatal("expected error when voice_script and answer_script blank, got nil")
	}
}

// A non-blank voice_script passes.
func TestValidateGeneratedScript_HasVoice(t *testing.T) {
	if err := validateGeneratedScript(&GeneratedScript{VoiceScript: "สวัสดีครับ"}); err != nil {
		t.Fatalf("expected nil for valid script, got %v", err)
	}
}

// answer_script alone (voice_script blank) also passes — scriptNarration falls
// back to it.
func TestValidateGeneratedScript_AnswerOnly(t *testing.T) {
	if err := validateGeneratedScript(&GeneratedScript{AnswerScript: "บทเต็ม"}); err != nil {
		t.Fatalf("expected nil when only answer_script present, got %v", err)
	}
}

func TestScriptTemplateData_NewFieldsRender(t *testing.T) {
	td := ScriptTemplateData{
		Question:             "Q",
		ArchetypeInstruction: "ARCH",
		RoleInstruction:      "ROLE",
	}
	out, err := renderTemplate("h {{.ArchetypeInstruction}} i {{.RoleInstruction}}", td)
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	want := "h ARCH i ROLE"
	if out != want {
		t.Errorf("render mismatch:\n got: %s\nwant: %s", out, want)
	}
}

// DebateLens renders into the script prompt; empty lens leaves no residue —
// the flag-off path must produce a byte-identical prompt to before the field
// existed (aside from the appended placeholder resolving to empty).
func TestScriptTemplateData_DebateLensRender(t *testing.T) {
	out, err := renderTemplate("base {{.DebateLens}}", ScriptTemplateData{DebateLens: "LENS"})
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	if out != "base LENS" {
		t.Errorf("got %q want %q", out, "base LENS")
	}
	out, err = renderTemplate("base {{.DebateLens}}", ScriptTemplateData{})
	if err != nil {
		t.Fatalf("renderTemplate empty: %v", err)
	}
	if out != "base " {
		t.Errorf("empty lens: got %q want %q", out, "base ")
	}
}
