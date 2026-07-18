package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jaochai/video-fb/internal/models"
)

// JudgeCandidate is the slim view of one debate script the judge scores.
// Scenes/timing are deliberately excluded — the judge rules on content only.
type JudgeCandidate struct {
	Lens               string   `json:"lens"`
	AnswerScript       string   `json:"answer_script"`
	VoiceScript        string   `json:"voice_script"`
	YoutubeTitle       string   `json:"youtube_title"`
	YoutubeDescription string   `json:"youtube_description"`
	YoutubeTags        []string `json:"youtube_tags"`
}

func NewJudgeCandidate(lens string, s *GeneratedScript) JudgeCandidate {
	return JudgeCandidate{
		Lens:               lens,
		AnswerScript:       s.AnswerScript,
		VoiceScript:        s.VoiceScript,
		YoutubeTitle:       s.YoutubeTitle,
		YoutubeDescription: s.YoutubeDescription,
		YoutubeTags:        s.YoutubeTags,
	}
}

// JudgeScore is the judge's 1-10 rating of one candidate.
type JudgeScore struct {
	Lens        string `json:"lens"`
	Hook        int    `json:"hook"`
	Accuracy    int    `json:"accuracy"`
	AudienceFit int    `json:"audience_fit"`
}

// JudgeVerdict is the raw JSON the judge LLM returns. Final uses the same
// shape as a normal script-agent output so downstream stages see no difference.
type JudgeVerdict struct {
	Scores     []JudgeScore    `json:"scores"`
	WinnerLens string          `json:"winner_lens"`
	Rationale  string          `json:"rationale"`
	Final      GeneratedScript `json:"final"`
}

type JudgeInput struct {
	Question        string
	AudiencePersona string
	Candidates      []JudgeCandidate
}

type judgeTemplateData struct {
	Question        string
	AudiencePersona string
	CandidatesJSON  string
}

type ScriptJudgeAgent struct {
	llm *KieLLMClient
}

func NewScriptJudgeAgent(llm *KieLLMClient) *ScriptJudgeAgent {
	return &ScriptJudgeAgent{llm: llm}
}

// Judge scores the candidates and returns the merged final script. Any error
// (LLM, parse, invalid verdict) is returned as-is; the caller is responsible
// for the fail-open fallback to a raw candidate.
func (a *ScriptJudgeAgent) Judge(ctx context.Context, in JudgeInput, cfg *models.AgentConfig) (*JudgeVerdict, error) {
	candJSON, err := json.Marshal(in.Candidates)
	if err != nil {
		return nil, fmt.Errorf("marshal candidates: %w", err)
	}
	userPrompt, err := renderTemplate(cfg.PromptTemplate, judgeTemplateData{
		Question:        in.Question,
		AudiencePersona: in.AudiencePersona,
		CandidatesJSON:  string(candJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("render judge template: %w", err)
	}
	var v JudgeVerdict
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &v); err != nil {
		return nil, fmt.Errorf("judge llm: %w", err)
	}
	if err := validateJudgeVerdict(&v, in.Candidates); err != nil {
		return nil, fmt.Errorf("judge verdict invalid: %w", err)
	}
	return &v, nil
}

// validateJudgeVerdict rejects verdicts the pipeline cannot safely use: an
// empty final script (same rule as validateGeneratedScript) or a winner_lens
// that names no actual candidate.
func validateJudgeVerdict(v *JudgeVerdict, cands []JudgeCandidate) error {
	if err := validateGeneratedScript(&v.Final); err != nil {
		return err
	}
	for _, c := range cands {
		if c.Lens == v.WinnerLens {
			return nil
		}
	}
	return fmt.Errorf("winner_lens %q not among candidates", v.WinnerLens)
}
