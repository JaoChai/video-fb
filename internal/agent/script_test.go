package agent

import "testing"

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
