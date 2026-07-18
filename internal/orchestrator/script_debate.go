package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
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
	// Index-assigned slots keep candidate order stable (same pattern as
	// VisualQAAgent.Review); each writer is fail-open so Wait is just a barrier.
	scripts := make([]*agent.GeneratedScript, len(lenses))
	var g errgroup.Group
	g.SetLimit(4)
	for i, l := range lenses {
		i, l := i, l
		g.Go(func() error {
			s, err := gen(l.Instruction)
			if err == nil {
				scripts[i] = s
			}
			return nil
		})
	}
	g.Wait()

	var cands []agent.JudgeCandidate
	var first *agent.GeneratedScript
	for i, s := range scripts {
		if s == nil {
			continue
		}
		if first == nil {
			first = s
		}
		cands = append(cands, agent.NewJudgeCandidate(lenses[i].Key, s))
	}

	switch len(cands) {
	case 0:
		return nil, nil, nil, "", fmt.Errorf("all %d debate writers failed", len(lenses))
	case 1:
		return first, cands, nil, "single_candidate", nil
	}

	verdict, err := judge(cands)
	if err != nil {
		return first, cands, nil, "judge_failed", nil
	}
	return &verdict.Final, cands, verdict, "judge", nil
}

// generateScript is the script-stage entry point. Flag off (or any config
// gap) → the plain single-pass path, byte-for-byte the old behavior. Flag on →
// newsroom debate with the fail-open ladder in runScriptDebate; if even that
// errors, fall back to single-pass. The debate can never fail a clip.
func (o *Orchestrator) generateScript(ctx context.Context, clipID string, q agent.GeneratedQuestion, format *models.ContentFormat, persona, archetypeInstr, roleInstr string, scriptCfg *models.AgentConfig) (*agent.GeneratedScript, error) {
	single := func() (*agent.GeneratedScript, error) {
		return o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, format, persona, archetypeInstr, roleInstr, "", scriptCfg)
	}

	if raw, _ := o.settingsRepo.Get(ctx, "script_debate_enabled"); raw != "true" {
		return single()
	}

	lensRaw, _ := o.settingsRepo.Get(ctx, "script_debate_lenses")
	lenses := parseDebateLenses(lensRaw)
	judgeCfg, jerr := o.agentsRepo.GetByName(ctx, "script_judge")
	if lenses == nil || jerr != nil || judgeCfg == nil || !judgeCfg.Enabled {
		log.Printf("script debate: config unavailable (lenses=%d, judgeErr=%v) — single-pass fallback", len(lenses), jerr)
		return single()
	}

	// Research/KB context depends only on the question — build once and share
	// across all lens writers instead of paying the web-search + vector lookup
	// three times for identical output.
	ragContext, rerr := o.scriptAgent.BuildRAGContext(ctx, q.Question, format)
	if rerr != nil {
		log.Printf("script debate: rag context failed (%v) — single-pass fallback", rerr)
		return single()
	}
	gen := func(lensInstruction string) (*agent.GeneratedScript, error) {
		return o.scriptAgent.GenerateWithContext(ctx, q.Question, q.QuestionerName, q.Category, format, persona, archetypeInstr, roleInstr, ragContext, lensInstruction, scriptCfg)
	}
	judge := func(cands []agent.JudgeCandidate) (*agent.JudgeVerdict, error) {
		return o.scriptJudgeAgent.Judge(ctx, agent.JudgeInput{
			Question:        q.Question,
			AudiencePersona: persona,
			Candidates:      cands,
		}, judgeCfg)
	}

	final, cands, verdict, source, err := runScriptDebate(lenses, gen, judge)
	if err != nil {
		log.Printf("script debate: %v — single-pass fallback", err)
		return single()
	}
	log.Printf("script debate: source=%s candidates=%d clip=%s", source, len(cands), clipID)
	o.recordScriptDebate(ctx, clipID, cands, verdict, source)
	return final, nil
}

// recordScriptDebate persists the audit row; failures only log — audit must
// never block production.
func (o *Orchestrator) recordScriptDebate(ctx context.Context, clipID string, cands []agent.JudgeCandidate, verdict *agent.JudgeVerdict, source string) {
	if o.scriptDebatesRepo == nil {
		return
	}
	candJSON, err := json.Marshal(cands)
	if err != nil {
		log.Printf("script debate: marshal candidates for audit failed (non-fatal): %v", err)
		return
	}
	var verdictJSON []byte
	if verdict != nil {
		verdictJSON, err = json.Marshal(verdict)
		if err != nil {
			log.Printf("script debate: marshal verdict for audit failed (non-fatal): %v", err)
			verdictJSON = nil
		}
	}
	if err := o.scriptDebatesRepo.Insert(ctx, clipID, candJSON, verdictJSON, source); err != nil {
		log.Printf("script debate: audit insert failed (non-fatal): %v", err)
	}
}
