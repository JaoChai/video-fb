package agent

import "testing"

// renderTemplate ต้องแทน {{.CategoryAngle}} {{.ArchetypeInstruction}} {{.RoleInstruction}} {{.TopicStats}}
func TestQuestionTemplateData_NewFieldsRender(t *testing.T) {
	td := QuestionTemplateData{
		Count: 3, Category: "multi-account",
		CategoryAngle:        "ANGLEX",
		ArchetypeInstruction: "ARCHX",
		RoleInstruction:      "ROLEX",
		TopicStats:           "STATSX",
	}
	out, err := renderTemplate("a {{.CategoryAngle}} b {{.ArchetypeInstruction}} c {{.RoleInstruction}} d {{.TopicStats}}", td)
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	want := "a ANGLEX b ARCHX c ROLEX d STATSX"
	if out != want {
		t.Errorf("render mismatch:\n got: %s\nwant: %s", out, want)
	}
}
