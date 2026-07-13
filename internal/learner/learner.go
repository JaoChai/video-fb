package learner

import (
	"context"
	"fmt"
	"log"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

// Guardrail constants. The loop only acts on a STRONG signal: enough critiques in
// the window AND either a regression against the agent's own baseline or one
// issue recurring across the window. Tuned conservatively because changes
// auto-apply.
const (
	// windowDays is how far back LowScorePatterns aggregates for the CURRENT window.
	windowDays = 30
	// baselineDays is the trailing window the current scores are compared against.
	baselineDays = 90
	// minCritiques is the minimum critique rows (in a window) before we act.
	minCritiques = 8
	// regressionMargin: the weakest dimension must sit this far below its own
	// baseline average to count as a real regression. Replaces the old absolute
	// lowScoreThreshold=6.0 which real scores (~7.8-8.9) could never reach.
	regressionMargin = 0.5
	// issueFrequencyThreshold: alternatively fire when the single most common
	// critique issue appears in at least this fraction of window critiques.
	issueFrequencyThreshold = 0.4
	// topIssuesN caps how many recurring issues feed the pattern summary.
	topIssuesN = 8
)

// allowedAgents is the FIXED allowlist of agents the learner may ever touch.
// Auto-apply is restricted to these names; nothing else can be modified.
var allowedAgents = []string{"scene", "script"}

// SkillRevisionsWriter is the append-only audit sink. Implemented in Task 6 by a
// tiny repo; declared here so the service depends on a narrow interface.
type SkillRevisionsWriter interface {
	Record(ctx context.Context, agentName, oldSkills, newSkills, rationale string, critiqueWindow int) error
}

// agentsRepoIface is the subset of *repository.AgentsRepo the Learner needs.
// *repository.AgentsRepo already satisfies this exactly.
type agentsRepoIface interface {
	GetByName(ctx context.Context, name string) (*models.AgentConfig, error)
	UpdateSkillsByName(ctx context.Context, agentName, newSkills string) error
}

// critiquesRepoIface is the subset of *repository.CritiquesRepo the Learner needs.
type critiquesRepoIface interface {
	LowScorePatterns(ctx context.Context, sinceDays, topN int) (repository.ScorePatterns, error)
}

// learnerAgentIface is the subset of *agent.LearnerAgent the Learner needs.
type learnerAgentIface interface {
	Propose(ctx context.Context, in agent.LearnInput, cfg *models.AgentConfig) (*agent.LearnOutput, error)
}

// strongSignal is the pure gate, now RELATIVE to the agent's own history:
// fire on (a) regression — the weakest current dimension sits regressionMargin
// below the same dimension's baseline average (baseline must itself have enough
// rows), or (b) frequency — one issue recurs in >= issueFrequencyThreshold of
// window critiques. Returns (fire, weakest-dim, weakest-val, gate-name) so the
// caller logs exactly why it acted or skipped.
func strongSignal(p, base repository.ScorePatterns) (bool, string, float64, string) {
	name, val := p.LowestDimension()
	if p.N < minCritiques {
		return false, name, val, "insufficient"
	}
	if base.N >= minCritiques && val < base.Dim(name)-regressionMargin {
		return true, name, val, "regression"
	}
	if len(p.TopIssues) > 0 && float64(p.TopIssues[0].Count) >= issueFrequencyThreshold*float64(p.N) {
		return true, name, val, "frequency"
	}
	return false, name, val, "no_gate"
}

// Learner runs the guardrailed auto-apply loop.
type Learner struct {
	agents    agentsRepoIface
	critiques critiquesRepoIface
	llmAgent  learnerAgentIface
	audit     SkillRevisionsWriter
}

func New(
	agents agentsRepoIface,
	critiques critiquesRepoIface,
	llmAgent learnerAgentIface,
	audit SkillRevisionsWriter,
) *Learner {
	return &Learner{agents: agents, critiques: critiques, llmAgent: llmAgent, audit: audit}
}

// RunOnce executes one pass: for each allowlisted agent, aggregate recent
// critiques, apply the strong-signal gate, ask the learner to propose, validate,
// and — only on accept — write an audit row THEN update the agent's skills. Never
// fatal: a failure on one agent is logged and the loop continues.
func (l *Learner) RunOnce(ctx context.Context) error {
	learnerCfg, err := l.agents.GetByName(ctx, "learner")
	if err != nil {
		return fmt.Errorf("learner agent config: %w", err)
	}
	if !learnerCfg.Enabled {
		log.Printf("learner: disabled (agent_configs['learner'].enabled = false); skipping run")
		return nil
	}

	// Fetch the full patterns once; each agent then operates on a filtered copy.
	patterns, err := l.critiques.LowScorePatterns(ctx, windowDays, topIssuesN)
	if err != nil {
		return fmt.Errorf("learner: aggregate failed: %w", err)
	}

	baseline, err := l.critiques.LowScorePatterns(ctx, baselineDays, topIssuesN)
	if err != nil {
		return fmt.Errorf("learner: baseline aggregate failed: %w", err)
	}

	for _, name := range allowedAgents {
		// Filter TopIssues to only those owned by this agent.
		var ownedIssues []repository.FieldIssue
		for _, fi := range patterns.TopIssues {
			if agentForField(fi.Field) == name {
				ownedIssues = append(ownedIssues, fi)
			}
		}
		if len(ownedIssues) == 0 {
			log.Printf("learner: %s has no attributable issues, skipping", name)
			continue
		}

		// Build a per-agent ScorePatterns with the filtered issue list.
		agentPatterns := patterns
		agentPatterns.TopIssues = ownedIssues

		ok, lowDim, lowVal, gate := strongSignal(agentPatterns, baseline)
		if !ok {
			log.Printf("learner: [%s] skip — weak signal (%s; n=%d weakest=%s avg=%.2f baseline=%.2f)",
				name, gate, agentPatterns.N, lowDim, lowVal, baseline.Dim(lowDim))
			continue
		}

		target, err := l.agents.GetByName(ctx, name)
		if err != nil {
			log.Printf("learner: [%s] config not found (skip): %v", name, err)
			continue
		}

		in := agent.LearnInput{
			AgentName:     name,
			CurrentSkills: target.Skills,
			Patterns:      formatPatterns(agentPatterns),
			WindowDays:    windowDays,
		}
		out, err := l.llmAgent.Propose(ctx, in, learnerCfg)
		if err != nil {
			log.Printf("learner: [%s] propose failed (skip): %v", name, err)
			continue
		}

		if !agent.AcceptProposal(in, out) {
			log.Printf("learner: [%s] skip — proposal rejected by guardrail (confident=%v, empty=%v)",
				name, out != nil && out.Confident, out == nil || out.NewSkills == "")
			continue
		}

		// Audit FIRST (append-only, revertable), then apply. If the audit write
		// fails we do NOT apply — the change must always be recorded.
		if err := l.audit.Record(ctx, name, target.Skills, out.NewSkills, out.Rationale, agentPatterns.N); err != nil {
			log.Printf("learner: [%s] audit write failed — NOT applying: %v", name, err)
			continue
		}
		if err := l.agents.UpdateSkillsByName(ctx, name, out.NewSkills); err != nil {
			log.Printf("learner: [%s] apply failed AFTER audit (revert from skill_revisions if needed): %v", name, err)
			continue
		}
		log.Printf("learner: [%s] APPLIED new skills (gate=%s weakest=%s avg=%.2f n=%d) — rationale: %s",
			name, gate, lowDim, lowVal, agentPatterns.N, out.Rationale)
	}
	return nil
}

// formatPatterns renders aggregated patterns into the Thai summary the learner
// prompt_template consumes. Pure.
func formatPatterns(p repository.ScorePatterns) string {
	lowDim, lowVal := p.LowestDimension()
	s := fmt.Sprintf(
		"จำนวน critique: %d\nคะแนนเฉลี่ย — hook: %.2f, clarity: %.2f, brand_fit: %.2f, overall: %.2f\nมิติที่อ่อนสุด: %s (%.2f)\n\nปัญหาที่ critic แก้บ่อยสุด:\n",
		p.N, p.AvgHook, p.AvgClarity, p.AvgBrandFit, p.AvgOverall, lowDim, lowVal,
	)
	if len(p.TopIssues) == 0 {
		s += "- (ไม่มีรายการ)\n"
		return s
	}
	for _, fi := range p.TopIssues {
		s += fmt.Sprintf("- %s — %s (x%d)\n", fi.Field, fi.Reason, fi.Count)
	}
	return s
}
