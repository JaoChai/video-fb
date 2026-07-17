package agent

import (
	"context"
	"errors"
	"testing"
)

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

func TestCooldownFilterRetry_RetriesPastCooldown(t *testing.T) {
	cd := map[string]bool{"low_trust_score": true}
	inCD := func(_ context.Context, pp string) (bool, error) { return cd[pp], nil }
	regenCalls := 0
	regen := func(_ context.Context, avoid []string, n int) ([]GeneratedQuestion, error) {
		regenCalls++
		return []GeneratedQuestion{{Question: "q2", PainPoint: "agency_trust_score"}}, nil
	}
	initial := []GeneratedQuestion{{Question: "q1", PainPoint: "low_trust_score"}}
	got := cooldownFilterRetry(context.Background(), initial, 1, 2, inCD, regen)
	if len(got) != 1 || got[0].PainPoint != "agency_trust_score" {
		t.Fatalf("want 1 q w/ agency_trust_score, got %+v", got)
	}
	if regenCalls != 1 {
		t.Fatalf("want 1 regen call, got %d", regenCalls)
	}
}

func TestCooldownFilterRetry_FailOpenWhenAllInCooldown(t *testing.T) {
	inCD := func(_ context.Context, _ string) (bool, error) { return true, nil }
	regen := func(_ context.Context, _ []string, n int) ([]GeneratedQuestion, error) {
		return []GeneratedQuestion{{Question: "qX", PainPoint: "low_trust_score"}}, nil
	}
	initial := []GeneratedQuestion{{Question: "q1", PainPoint: "low_trust_score"}}
	got := cooldownFilterRetry(context.Background(), initial, 1, 2, inCD, regen)
	if len(got) != 1 {
		t.Fatalf("fail-open must return 1 question, got %d", len(got))
	}
}

func TestCooldownFilterRetry_NoRetryWhenClean(t *testing.T) {
	inCD := func(_ context.Context, _ string) (bool, error) { return false, nil }
	regenCalls := 0
	regen := func(_ context.Context, _ []string, n int) ([]GeneratedQuestion, error) {
		regenCalls++
		return nil, nil
	}
	initial := []GeneratedQuestion{{Question: "q1", PainPoint: "ad_fatigue"}}
	got := cooldownFilterRetry(context.Background(), initial, 1, 2, inCD, regen)
	if len(got) != 1 || regenCalls != 0 {
		t.Fatalf("want 1 q and 0 regen, got %d q, %d regen", len(got), regenCalls)
	}
}

func TestCooldownFilterRetry_CooldownErrorFailsOpen(t *testing.T) {
	inCD := func(_ context.Context, _ string) (bool, error) { return false, errors.New("db down") }
	regen := func(_ context.Context, _ []string, n int) ([]GeneratedQuestion, error) {
		t.Fatal("regen must not run when question kept via fail-open")
		return nil, nil
	}
	initial := []GeneratedQuestion{{Question: "q1", PainPoint: "low_trust_score"}}
	got := cooldownFilterRetry(context.Background(), initial, 1, 2, inCD, regen)
	if len(got) != 1 {
		t.Fatalf("cooldown error must fail-open keep question, got %d", len(got))
	}
}
