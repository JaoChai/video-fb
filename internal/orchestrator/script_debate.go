package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/jaochai/video-fb/internal/agent"
)

// debateLens is one writing angle in the script newsroom debate, loaded from
// the script_debate_lenses setting.
type debateLens struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Instruction string `json:"instruction"`
}

// parseDebateLenses parses the setting JSON. Returns nil (caller falls back to
// single-pass) on bad JSON or fewer than 2 usable lenses — a one-writer
// "debate" is just a slower single pass.
func parseDebateLenses(raw string) []debateLens {
	var lenses []debateLens
	if err := json.Unmarshal([]byte(raw), &lenses); err != nil {
		return nil
	}
	usable := make([]debateLens, 0, len(lenses))
	for _, l := range lenses {
		if strings.TrimSpace(l.Key) != "" && strings.TrimSpace(l.Instruction) != "" {
			usable = append(usable, l)
		}
	}
	if len(usable) < 2 {
		return nil
	}
	return usable
}

type scriptGenFn func(lensInstruction string) (*agent.GeneratedScript, error)
type scriptJudgeFn func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error)

// runScriptDebate runs one writer per lens in parallel, then the judge.
// Fail-open ladder: judge error → first candidate; one candidate → skip judge;
// zero candidates → error (caller runs the plain single-pass generate).
func runScriptDebate(lenses []debateLens, gen scriptGenFn, judge scriptJudgeFn) (*agent.GeneratedScript, []agent.JudgeCandidate, *agent.JudgeVerdict, string, error) {
	type slot struct {
		script *agent.GeneratedScript
		err    error
	}
	slots := make([]slot, len(lenses))
	var wg sync.WaitGroup
	for i, l := range lenses {
		wg.Add(1)
		go func(i int, l debateLens) {
			defer wg.Done()
			s, err := gen("## มุมมองการเขียนรอบนี้ (" + l.Name + ")\n" + l.Instruction)
			slots[i] = slot{script: s, err: err}
		}(i, l)
	}
	wg.Wait()

	var cands []agent.JudgeCandidate
	var scripts []*agent.GeneratedScript
	for i, s := range slots {
		if s.err != nil || s.script == nil {
			continue
		}
		cands = append(cands, agent.NewJudgeCandidate(lenses[i].Key, s.script))
		scripts = append(scripts, s.script)
	}

	switch len(scripts) {
	case 0:
		return nil, nil, nil, "", fmt.Errorf("all %d debate writers failed", len(lenses))
	case 1:
		return scripts[0], cands, nil, "single_candidate", nil
	}

	verdict, err := judge(cands)
	if err != nil {
		return scripts[0], cands, nil, "judge_failed", nil
	}
	return &verdict.Final, cands, verdict, "judge", nil
}
