package agent

import (
	"encoding/json"
	"testing"
)

// twoScenesInput builds a minimal valid CriticInput with two scenes.
func twoScenesInput() CriticInput {
	return CriticInput{
		Question:  "บัญชีโฆษณาโดนแบนทำไง",
		Narration: "...",
		Scenes: []GeneratedScene{
			{SceneNumber: 1, SceneType: "hook", Layout: "hook", LayoutVariant: "hook_big",
				DurationSeconds: 4.5, VoiceText: "ของเดิม 1", ImagePrompt: "img1"},
			{SceneNumber: 2, SceneType: "cta", Layout: "cta", LayoutVariant: "phrase_block",
				DurationSeconds: 6, VoiceText: "ของเดิม 2", ImagePrompt: "img2"},
		},
		Metadata: CriticMetadata{YoutubeTitle: "เดิม", YoutubeDescription: "เดิม", YoutubeTags: []string{"a"}},
	}
}

func goodScore() CriticScore { return CriticScore{Hook: 8, Clarity: 7, BrandFit: 9, Overall: 8} }

func TestReconcile_HappyPath_AppliesContentEdits(t *testing.T) {
	in := twoScenesInput()
	out := CriticOutput{
		Scenes: []GeneratedScene{
			{SceneNumber: 1, VoiceText: "ปรับแล้ว 1", ImagePrompt: "img1-better",
				DurationSeconds: 999, SceneType: "WRONG", Layout: "WRONG"},
			{SceneNumber: 2, VoiceText: "ปรับแล้ว 2", ImagePrompt: "img2-better"},
		},
		Metadata: CriticMetadata{YoutubeTitle: "ใหม่", YoutubeTags: []string{"x", "y"}},
		Score:    goodScore(),
		Changes:  []CriticChange{{Field: "scene[0].voice_text", Reason: "คมขึ้น"}},
	}
	got := reconcileCritique(in, out)

	if !got.Applied {
		t.Fatal("Applied = false, want true")
	}
	if got.Scenes[0].VoiceText != "ปรับแล้ว 1" {
		t.Errorf("VoiceText not applied: %q", got.Scenes[0].VoiceText)
	}
	if got.Scenes[0].DurationSeconds != 4.5 {
		t.Errorf("DurationSeconds = %v, want 4.5 (original)", got.Scenes[0].DurationSeconds)
	}
	if got.Scenes[0].SceneType != "hook" || got.Scenes[0].Layout != "hook" {
		t.Errorf("layout/type drifted: %q / %q", got.Scenes[0].SceneType, got.Scenes[0].Layout)
	}
	if got.Metadata.YoutubeTitle != "ใหม่" {
		t.Errorf("title not applied: %q", got.Metadata.YoutubeTitle)
	}
	if got.Metadata.YoutubeDescription != "เดิม" {
		t.Errorf("description should keep original, got %q", got.Metadata.YoutubeDescription)
	}
}

func TestReconcile_CountMismatch_FailsSafe(t *testing.T) {
	in := twoScenesInput()
	out := CriticOutput{
		Scenes: []GeneratedScene{{SceneNumber: 1, VoiceText: "x"}},
		Score:  goodScore(),
	}
	got := reconcileCritique(in, out)
	if got.Applied {
		t.Fatal("Applied = true, want false on count mismatch")
	}
	if got.Scenes[0].VoiceText != "ของเดิม 1" {
		t.Errorf("did not return original scenes")
	}
}

func TestReconcile_EmptyVoice_FailsSafe(t *testing.T) {
	in := twoScenesInput()
	out := CriticOutput{
		Scenes: []GeneratedScene{
			{SceneNumber: 1, VoiceText: "   "},
			{SceneNumber: 2, VoiceText: "ok"},
		},
		Score: goodScore(),
	}
	if reconcileCritique(in, out).Applied {
		t.Fatal("Applied = true, want false on empty voice_text")
	}
}

func TestReconcile_ScoreOutOfRange_FailsSafe(t *testing.T) {
	in := twoScenesInput()
	out := CriticOutput{
		Scenes: []GeneratedScene{{SceneNumber: 1, VoiceText: "a"}, {SceneNumber: 2, VoiceText: "b"}},
		Score:  CriticScore{Hook: 11, Clarity: 5, BrandFit: 5, Overall: 5},
	}
	if reconcileCritique(in, out).Applied {
		t.Fatal("Applied = true, want false on score out of range")
	}
}

func TestReconcile_UnknownSceneNumber_FailsSafe(t *testing.T) {
	in := twoScenesInput()
	out := CriticOutput{
		Scenes: []GeneratedScene{{SceneNumber: 1, VoiceText: "a"}, {SceneNumber: 9, VoiceText: "b"}},
		Score:  goodScore(),
	}
	if reconcileCritique(in, out).Applied {
		t.Fatal("Applied = true, want false on unknown scene_number")
	}
}

func TestReconcile_EmptyInput_FailsSafe(t *testing.T) {
	in := CriticInput{Scenes: nil, Metadata: CriticMetadata{YoutubeTitle: "เดิม"}}
	out := CriticOutput{Scenes: nil, Score: goodScore()}
	got := reconcileCritique(in, out)
	if got.Applied {
		t.Fatal("Applied = true, want false on empty input scenes")
	}
}

func TestCriticOutputParsesSchema(t *testing.T) {
	raw := `{
	  "scenes": [ { "scene_number": 1, "voice_text": "hi", "image_prompt": "p" } ],
	  "metadata": { "youtube_title": "t", "youtube_description": "d", "youtube_tags": ["a","b"] },
	  "score": { "hook": 8, "clarity": 7, "brand_fit": 9, "overall": 8 },
	  "changes": [ { "field": "scene[0].voice_text", "reason": "r" } ]
	}`
	var out CriticOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("CriticOutput did not unmarshal: %v", err)
	}
	if out.Score.Hook != 8 || out.Metadata.YoutubeTitle != "t" || len(out.Changes) != 1 {
		t.Errorf("unexpected parse: %+v", out)
	}
}

// Critic-revised structured content must flow through reconcile; invalid or
// missing content keeps the original (render-time buildSceneContent clamps
// lengths, so no sanitize needed here).
func TestReconcileCritiqueMergesContent(t *testing.T) {
	origContent := json.RawMessage(`{"kicker":"เดิม","rows":[{"t":"แถวเดิม","bad":true}]}`)
	newContent := json.RawMessage(`{"kicker":"ใหม่","rows":[{"t":"แถวใหม่ คมกว่า","bad":true}]}`)

	in := CriticInput{
		Scenes: []GeneratedScene{{SceneNumber: 1, VoiceText: "v", Layout: "hook", Content: origContent}},
	}

	cases := []struct {
		name    string
		content json.RawMessage
		want    string
	}{
		{"valid content replaces original", newContent, string(newContent)},
		{"invalid JSON keeps original", json.RawMessage(`{broken`), string(origContent)},
		{"empty keeps original", nil, string(origContent)},
		{"null keeps original", json.RawMessage(`null`), string(origContent)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := CriticOutput{
				Scenes: []GeneratedScene{{SceneNumber: 1, VoiceText: "v2", Content: tc.content}},
				Score:  CriticScore{Hook: 8, Clarity: 8, BrandFit: 8, Overall: 8},
			}
			res := reconcileCritique(in, out)
			if !res.Applied {
				t.Fatalf("Applied = false, want true")
			}
			if got := string(res.Scenes[0].Content); got != tc.want {
				t.Errorf("Content = %s, want %s", got, tc.want)
			}
			if res.Scenes[0].Layout != "hook" {
				t.Errorf("Layout changed to %q — must stay immutable", res.Scenes[0].Layout)
			}
		})
	}
}
