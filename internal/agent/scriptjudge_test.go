package agent

import "testing"

func judgeCands() []JudgeCandidate {
	return []JudgeCandidate{
		{Lens: "hook_maximalist", VoiceScript: "ฉบับ A"},
		{Lens: "skeptic_editor", VoiceScript: "ฉบับ B"},
	}
}

// Verdict whose final script is empty is unusable — the caller must fall back
// to a raw candidate, so validation rejects it here.
func TestValidateJudgeVerdict_EmptyFinal(t *testing.T) {
	v := &JudgeVerdict{WinnerLens: "hook_maximalist"}
	if err := validateJudgeVerdict(v, judgeCands()); err == nil {
		t.Fatal("expected error for empty final script, got nil")
	}
}

// winner_lens must name one of the actual candidates; a hallucinated lens key
// means the judge ignored the input.
func TestValidateJudgeVerdict_UnknownWinner(t *testing.T) {
	v := &JudgeVerdict{
		WinnerLens: "made_up",
		Final:      GeneratedScript{VoiceScript: "โอเค"},
	}
	if err := validateJudgeVerdict(v, judgeCands()); err == nil {
		t.Fatal("expected error for unknown winner_lens, got nil")
	}
}

func TestValidateJudgeVerdict_Valid(t *testing.T) {
	v := &JudgeVerdict{
		WinnerLens: "skeptic_editor",
		Final:      GeneratedScript{VoiceScript: "โอเค"},
	}
	if err := validateJudgeVerdict(v, judgeCands()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// NewJudgeCandidate carries only the fields the judge needs — scenes and
// timing stay out of the prompt.
func TestNewJudgeCandidate(t *testing.T) {
	s := &GeneratedScript{
		AnswerScript: "เต็ม", VoiceScript: "พากย์",
		YoutubeTitle: "T", YoutubeDescription: "D", YoutubeTags: []string{"a"},
	}
	c := NewJudgeCandidate("target_viewer", s)
	if c.Lens != "target_viewer" || c.AnswerScript != "เต็ม" || c.VoiceScript != "พากย์" ||
		c.YoutubeTitle != "T" || c.YoutubeDescription != "D" || len(c.YoutubeTags) != 1 {
		t.Errorf("candidate fields not copied: %+v", c)
	}
}
