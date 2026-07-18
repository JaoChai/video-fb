package orchestrator

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func testLenses() []debateLens {
	return []debateLens{
		{Key: "hook_maximalist", Name: "Hook", Instruction: "แรงสุด"},
		{Key: "skeptic_editor", Name: "Skeptic", Instruction: "แม่นสุด"},
		{Key: "target_viewer", Name: "Viewer", Instruction: "ตรง pain สุด"},
	}
}

// Happy path: all writers succeed, judge succeeds → final comes from the
// judge's merged script, all candidates recorded.
func TestRunScriptDebate_JudgeWins(t *testing.T) {
	gen := func(lens string) (*agent.GeneratedScript, error) {
		return &agent.GeneratedScript{VoiceScript: "ฉบับ:" + lens}, nil
	}
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) {
		if len(cands) != 3 {
			t.Fatalf("judge got %d candidates, want 3", len(cands))
		}
		return &agent.JudgeVerdict{
			WinnerLens: "skeptic_editor",
			Final:      agent.GeneratedScript{VoiceScript: "ฉบับรวมร่าง"},
		}, nil
	}
	final, cands, verdict, source, err := runScriptDebate(testLenses(), gen, judge)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if source != "judge" || verdict == nil || final.VoiceScript != "ฉบับรวมร่าง" || len(cands) != 3 {
		t.Errorf("got source=%s verdict=%v final=%q cands=%d", source, verdict, final.VoiceScript, len(cands))
	}
}

// The lens block each writer receives must contain that lens's instruction —
// this is the "blind, independent angles" property the whole feature rests on.
func TestRunScriptDebate_LensInstructionReachesWriters(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	gen := func(lens string) (*agent.GeneratedScript, error) {
		mu.Lock()
		seen = append(seen, lens)
		mu.Unlock()
		return &agent.GeneratedScript{VoiceScript: "x"}, nil
	}
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) {
		return &agent.JudgeVerdict{WinnerLens: "hook_maximalist", Final: agent.GeneratedScript{VoiceScript: "y"}}, nil
	}
	if _, _, _, _, err := runScriptDebate(testLenses(), gen, judge); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	joined := strings.Join(seen, "|")
	for _, want := range []string{"แรงสุด", "แม่นสุด", "ตรง pain สุด"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no writer received instruction %q (got %q)", want, joined)
		}
	}
}

// Judge failure is fail-open: fall back to the first successful candidate,
// verdict nil, no error surfaced.
func TestRunScriptDebate_JudgeFails_FirstCandidate(t *testing.T) {
	gen := func(lens string) (*agent.GeneratedScript, error) {
		return &agent.GeneratedScript{VoiceScript: "ฉบับ:" + lens}, nil
	}
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) {
		return nil, errors.New("judge exploded")
	}
	final, cands, verdict, source, err := runScriptDebate(testLenses(), gen, judge)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if source != "judge_failed" || verdict != nil || len(cands) != 3 {
		t.Errorf("got source=%s verdict=%v cands=%d", source, verdict, len(cands))
	}
	if !strings.Contains(final.VoiceScript, "แรงสุด") {
		t.Errorf("expected first (hook) candidate, got %q", final.VoiceScript)
	}
}

// Exactly one writer succeeds → judge must be skipped entirely.
func TestRunScriptDebate_SingleCandidateSkipsJudge(t *testing.T) {
	gen := func(lens string) (*agent.GeneratedScript, error) {
		if !strings.Contains(lens, "แม่นสุด") {
			return nil, errors.New("writer down")
		}
		return &agent.GeneratedScript{VoiceScript: "ฉบับเดียวที่รอด"}, nil
	}
	judgeCalled := false
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) {
		judgeCalled = true
		return nil, nil
	}
	final, cands, verdict, source, err := runScriptDebate(testLenses(), gen, judge)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if judgeCalled {
		t.Error("judge must not be called with a single candidate")
	}
	if source != "single_candidate" || verdict != nil || len(cands) != 1 || final.VoiceScript != "ฉบับเดียวที่รอด" {
		t.Errorf("got source=%s verdict=%v cands=%d final=%q", source, verdict, len(cands), final.VoiceScript)
	}
}

// All writers fail → error out so the caller can run the plain single-pass.
func TestRunScriptDebate_AllWritersFail(t *testing.T) {
	gen := func(lens string) (*agent.GeneratedScript, error) { return nil, errors.New("down") }
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) { return nil, nil }
	if _, _, _, _, err := runScriptDebate(testLenses(), gen, judge); err == nil {
		t.Fatal("expected error when all writers fail, got nil")
	}
}

func TestParseDebateLenses(t *testing.T) {
	good := `[{"key":"a","name":"A","instruction":"ก"},{"key":"b","name":"B","instruction":"ข"}]`
	if got := parseDebateLenses(good); len(got) != 2 {
		t.Errorf("valid JSON: got %d lenses, want 2", len(got))
	}
	if got := parseDebateLenses("not json"); got != nil {
		t.Errorf("invalid JSON must return nil, got %v", got)
	}
	// blank key/instruction rows are dropped; fewer than 2 usable → nil
	oneUsable := `[{"key":"a","name":"A","instruction":"ก"},{"key":"","name":"B","instruction":"ข"}]`
	if got := parseDebateLenses(oneUsable); got != nil {
		t.Errorf("<2 usable lenses must return nil, got %v", got)
	}
}
