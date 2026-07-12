package agent

import "testing"

// A script with no scenes must be rejected — otherwise scriptNarration yields an
// empty string that flows into scene breakdown and the Director LLM replies with
// prose instead of JSON ("invalid character 'I'"). Regression for the empty-
// script production failure.
func TestValidateGeneratedScript_EmptyScenes(t *testing.T) {
	err := validateGeneratedScript(&GeneratedScript{Scenes: nil})
	if err == nil {
		t.Fatal("expected error for empty scenes, got nil")
	}
}

// Scenes present but all voice_text blank is equally useless — narration would
// still be empty.
func TestValidateGeneratedScript_AllVoiceBlank(t *testing.T) {
	err := validateGeneratedScript(&GeneratedScript{Scenes: []GeneratedScene{
		{SceneNumber: 1, VoiceText: "  "},
		{SceneNumber: 2, VoiceText: ""},
	}})
	if err == nil {
		t.Fatal("expected error when all voice_text blank, got nil")
	}
}

// A valid script (at least one non-blank voice_text) passes.
func TestValidateGeneratedScript_Valid(t *testing.T) {
	err := validateGeneratedScript(&GeneratedScript{Scenes: []GeneratedScene{
		{SceneNumber: 1, VoiceText: "สวัสดีครับ"},
	}})
	if err != nil {
		t.Fatalf("expected nil for valid script, got %v", err)
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
