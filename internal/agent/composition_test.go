package agent

import "testing"

func TestScenesDecision_Normalize(t *testing.T) {
	d := &ScenesDecision{Scenes: []SceneDesign{
		{SceneNumber: 1, LayoutVariant: "hook_big", AnimationSpeed: "fast",
			Slots: []Slot{{Role: "headline", Text: "hi"}, {Role: "weird", Text: "x"}, {Role: "body", Text: "  "}}},
		{SceneNumber: 2, LayoutVariant: "bogus", AnimationSpeed: "turbo",
			Slots: []Slot{{Role: "step", Text: "do it"}}},
	}}
	d.Normalize()

	if d.Scenes[0].Slots[1].Role != defaultSlotRole {
		t.Errorf("invalid slot role not defaulted: %q", d.Scenes[0].Slots[1].Role)
	}
	if len(d.Scenes[0].Slots) != 2 {
		t.Errorf("empty-text slot not dropped: got %d slots", len(d.Scenes[0].Slots))
	}
	if d.Scenes[1].LayoutVariant != defaultLayoutVariant {
		t.Errorf("invalid layout_variant not defaulted: %q", d.Scenes[1].LayoutVariant)
	}
	if d.Scenes[1].AnimationSpeed != "normal" {
		t.Errorf("invalid animation_speed not defaulted: %q", d.Scenes[1].AnimationSpeed)
	}
	if !validLayoutVariants[d.Scenes[0].LayoutVariant] {
		t.Errorf("valid layout_variant wrongly changed: %q", d.Scenes[0].LayoutVariant)
	}
}
