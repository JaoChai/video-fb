package agent

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)

// CriticMetadata mirrors the YouTube metadata fields the critic may revise.
type CriticMetadata struct {
	YoutubeTitle       string   `json:"youtube_title"`
	YoutubeDescription string   `json:"youtube_description"`
	YoutubeTags        []string `json:"youtube_tags"`
}

// CriticInput is everything the critic reviews for one clip.
type CriticInput struct {
	Question  string
	Narration string
	Scenes    []GeneratedScene
	Metadata  CriticMetadata
}

// CriticScore is the per-dimension quality score (each 1-10).
type CriticScore struct {
	Hook     int `json:"hook"`
	Clarity  int `json:"clarity"`
	BrandFit int `json:"brand_fit"`
	Overall  int `json:"overall"`
}

// CriticChange is one human-readable edit the critic made.
type CriticChange struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// CriticOutput is the raw JSON the LLM returns.
type CriticOutput struct {
	Scenes   []GeneratedScene `json:"scenes"`
	Metadata CriticMetadata   `json:"metadata"`
	Score    CriticScore      `json:"score"`
	Changes  []CriticChange   `json:"changes"`
}

// CriticResult is what Review hands back to the orchestrator: the content to
// actually use (the original bundle when the fail-safe triggered) plus the
// score/changes to log. Applied is false when the original was kept.
type CriticResult struct {
	Scenes   []GeneratedScene
	Metadata CriticMetadata
	Score    CriticScore
	Changes  []CriticChange
	Applied  bool
}

// scoreInRange reports whether every score dimension is within 1-10.
func scoreInRange(s CriticScore) bool {
	for _, v := range []int{s.Hook, s.Clarity, s.BrandFit, s.Overall} {
		if v < 1 || v > 10 {
			return false
		}
	}
	return true
}

// reconcileCritique merges the critic's output onto the original content and
// enforces the edit boundary. It returns the original bundle (Applied=false) on
// ANY anomaly, so the pipeline can never render something worse than the input.
// Only content fields are taken from the critic; all structural/timing/layout
// fields are copied from the original scene.
func reconcileCritique(in CriticInput, out CriticOutput) CriticResult {
	fail := CriticResult{
		Scenes:   in.Scenes,
		Metadata: in.Metadata,
		Score:    out.Score,
		Changes:  out.Changes,
		Applied:  false,
	}

	if len(in.Scenes) == 0 || len(out.Scenes) != len(in.Scenes) || !scoreInRange(out.Score) {
		return fail
	}

	critByNum := make(map[int]GeneratedScene, len(out.Scenes))
	for _, cs := range out.Scenes {
		if _, dup := critByNum[cs.SceneNumber]; dup {
			return fail // duplicate scene_number
		}
		critByNum[cs.SceneNumber] = cs
	}

	merged := make([]GeneratedScene, len(in.Scenes))
	for i, orig := range in.Scenes {
		cs, ok := critByNum[orig.SceneNumber]
		if !ok {
			return fail // critic dropped or renumbered a scene
		}
		if strings.TrimSpace(cs.VoiceText) == "" {
			return fail // never ship an empty narration
		}
		m := orig // copy keeps every structural/timing/layout field
		m.VoiceText = cs.VoiceText
		m.OnScreenText = cs.OnScreenText
		m.TextContent = cs.TextContent
		m.ImagePrompt = cs.ImagePrompt
		m.EmphasisWords = cs.EmphasisWords
		merged[i] = m
	}

	meta := in.Metadata
	if s := strings.TrimSpace(out.Metadata.YoutubeTitle); s != "" {
		meta.YoutubeTitle = s
	}
	if s := strings.TrimSpace(out.Metadata.YoutubeDescription); s != "" {
		meta.YoutubeDescription = s
	}
	if len(out.Metadata.YoutubeTags) > 0 {
		meta.YoutubeTags = out.Metadata.YoutubeTags
	}

	return CriticResult{
		Scenes:   merged,
		Metadata: meta,
		Score:    out.Score,
		Changes:  out.Changes,
		Applied:  true,
	}
}

// CriticTemplateData fills the seeded `critic` prompt_template.
type CriticTemplateData struct {
	Question  string
	Narration string
	InputJSON string
}

// CriticAgent reviews a clip's generated content and revises it in place. Runs
// on Claude (cfg.Model), same KieLLMClient path as the other agents.
type CriticAgent struct {
	llm *KieLLMClient
}

func NewCriticAgent(llm *KieLLMClient) *CriticAgent {
	return &CriticAgent{llm: llm}
}

// Review never returns an error: on any failure it returns the ORIGINAL content
// (Applied=false) so the caller can render unchanged. cfg is the `critic`
// AgentConfig fetched by the caller via GetByName.
func (a *CriticAgent) Review(ctx context.Context, in CriticInput, cfg *models.AgentConfig) CriticResult {
	orig := CriticResult{Scenes: in.Scenes, Metadata: in.Metadata, Applied: false}

	inputJSON, err := json.Marshal(struct {
		Scenes   []GeneratedScene `json:"scenes"`
		Metadata CriticMetadata   `json:"metadata"`
	}{in.Scenes, in.Metadata})
	if err != nil {
		return orig
	}

	userPrompt, err := renderTemplate(cfg.PromptTemplate, CriticTemplateData{
		Question:  in.Question,
		Narration: in.Narration,
		InputJSON: string(inputJSON),
	})
	if err != nil {
		return orig
	}

	var out CriticOutput
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &out); err != nil {
		return orig
	}
	return reconcileCritique(in, out)
}
