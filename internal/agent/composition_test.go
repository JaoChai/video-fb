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

// TestScenesDecision_Normalize_NewVariants verifies that the new layout variants
// and slot roles added in Phase 3-4 are accepted by Normalize() without being
// defaulted to fallback values.
func TestScenesDecision_Normalize_NewVariants(t *testing.T) {
	d := &ScenesDecision{Scenes: []SceneDesign{
		{
			SceneNumber:   1,
			LayoutVariant: LayoutHookPunch,
			AnimationSpeed: "fast",
			Slots: []Slot{
				{Role: "headline", Text: "โดนแบนภายในคืนเดียว"},
				{Role: "stat", Text: "97%"},
				{Role: "callout", Text: "มีทางออก"},
			},
		},
		{
			SceneNumber:   2,
			LayoutVariant: LayoutCompareTwo,
			AnimationSpeed: "normal",
			Slots: []Slot{
				{Role: "headline", Text: "ก่อน vs หลัง"},
				{Role: "stat", Text: "3x ROAS"},
				{Role: "callout", Text: "เพิ่มขึ้น 300%"},
			},
		},
	}}
	d.Normalize()

	// hook_punch must not be defaulted away.
	if d.Scenes[0].LayoutVariant != LayoutHookPunch {
		t.Errorf("hook_punch wrongly defaulted: got %q", d.Scenes[0].LayoutVariant)
	}
	// compare_two must not be defaulted away.
	if d.Scenes[1].LayoutVariant != LayoutCompareTwo {
		t.Errorf("compare_two wrongly defaulted: got %q", d.Scenes[1].LayoutVariant)
	}
	// stat slot role must be preserved.
	if d.Scenes[0].Slots[1].Role != "stat" {
		t.Errorf("stat slot role wrongly changed: got %q", d.Scenes[0].Slots[1].Role)
	}
	// callout slot role must be preserved.
	if d.Scenes[0].Slots[2].Role != "callout" {
		t.Errorf("callout slot role wrongly changed: got %q", d.Scenes[0].Slots[2].Role)
	}
	// Both scenes should retain all 3 slots (no empty text).
	if len(d.Scenes[0].Slots) != 3 {
		t.Errorf("scene 1 slots: want 3, got %d", len(d.Scenes[0].Slots))
	}
	if len(d.Scenes[1].Slots) != 3 {
		t.Errorf("scene 2 slots: want 3, got %d", len(d.Scenes[1].Slots))
	}
}

// TestScenesDecision_Normalize_StillDefaultsUnknown confirms that genuinely
// unknown variants and roles continue to be defaulted (regression guard).
func TestScenesDecision_Normalize_StillDefaultsUnknown(t *testing.T) {
	d := &ScenesDecision{Scenes: []SceneDesign{
		{
			SceneNumber:   1,
			LayoutVariant: "totally_made_up",
			AnimationSpeed: "warp_speed",
			Slots: []Slot{
				{Role: "floaty_text", Text: "unknown role"},
			},
		},
	}}
	d.Normalize()

	if d.Scenes[0].LayoutVariant != defaultLayoutVariant {
		t.Errorf("unknown layout_variant not defaulted: got %q", d.Scenes[0].LayoutVariant)
	}
	if d.Scenes[0].AnimationSpeed != "normal" {
		t.Errorf("unknown animation_speed not defaulted: got %q", d.Scenes[0].AnimationSpeed)
	}
	if d.Scenes[0].Slots[0].Role != defaultSlotRole {
		t.Errorf("unknown slot role not defaulted: got %q", d.Scenes[0].Slots[0].Role)
	}
}
