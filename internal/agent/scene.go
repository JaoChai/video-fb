package agent

import (
	"context"
	"fmt"

	"github.com/jaochai/video-fb/internal/models"
)

// SceneTemplateData fills the seeded `scene` prompt template. Field names match
// the design §4.6 registry exactly (renderTemplate substitutes {{.FieldName}}).
type SceneTemplateData struct {
	Script            string
	TargetSceneCount  int
	TargetDurationSec int
	ThemeDescription  string
}

// SceneAgent is the Director: it breaks a finished script into 6-10 constrained
// scenes for the 9:16 hyperframes template. Runs on Claude (cfg.Model is
// claude-sonnet-4-6, routed by KieLLMClient prefix). Output fields map onto the
// new scenes columns added in Plan 2b-1.
type SceneAgent struct {
	llm *KieLLMClient
}

func NewSceneAgent(llm *KieLLMClient) *SceneAgent {
	return &SceneAgent{llm: llm}
}

// Generate turns a script into an ordered scene array. cfg is the `scene`
// AgentConfig (fetched by the caller via GetByName). targetSceneCount and
// targetDurationSec steer length; theme supplies brand styling for image_prompt.
func (a *SceneAgent) Generate(ctx context.Context, script string, targetSceneCount, targetDurationSec int, theme *models.BrandTheme, cfg *models.AgentConfig) ([]GeneratedScene, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, SceneTemplateData{
		Script:            script,
		TargetSceneCount:  targetSceneCount,
		TargetDurationSec: targetDurationSec,
		ThemeDescription:  buildSceneThemeDescription(theme),
	})
	if err != nil {
		return nil, fmt.Errorf("render scene template: %w", err)
	}

	var scenes []GeneratedScene
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &scenes); err != nil {
		return nil, fmt.Errorf("generate scenes: %w", err)
	}
	return scenes, nil
}

// buildSceneThemeDescription renders a short Thai brand summary for the Director,
// so its draft image_prompt and tone stay on-brand. Pure — testable.
func buildSceneThemeDescription(theme *models.BrandTheme) string {
	return fmt.Sprintf("แบรนด์ Ads Vance: navy %s + ส้ม %s, มาสคอตเสือดาว. สไตล์ภาพ: %s",
		theme.PrimaryColor, theme.AccentColor, safeStr(theme.ImageStyle))
}
