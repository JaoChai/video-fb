package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
	"github.com/jaochai/video-fb/internal/progress"
	"github.com/jaochai/video-fb/internal/repository"
)

var (
	urlRegex    = regexp.MustCompile(`(?i)https?://\S+|t\.me/\S+|line\.me/\S+`)
	atHandleRgx = regexp.MustCompile(`@[A-Za-z][A-Za-z0-9_.\-]*`)
)

// sanitizeVoiceText strips URLs and @handles from voice script before TTS.
// Gemini TTS chokes on URLs and brand handles, often truncating audio mid-sentence.
// Brand mentions are normalized to a Thai phonetic spelling so the model says it correctly.
func sanitizeVoiceText(s string) string {
	replacer := strings.NewReplacer(
		"@adsvance", "แอดส์แวนซ์",
		"@AdsVance", "แอดส์แวนซ์",
		"@Adsvance", "แอดส์แวนซ์",
		"AdsVance", "แอดส์แวนซ์",
		"Adsvance", "แอดส์แวนซ์",
		"adsvance", "แอดส์แวนซ์",
		"Ads Vance", "แอดส์แวนซ์",
	)
	s = replacer.Replace(s)
	s = urlRegex.ReplaceAllString(s, "")
	s = atHandleRgx.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

var categories = []string{"account", "payment", "campaign", "pixel"}

type Orchestrator struct {
	pool          *pgxpool.Pool
	questionAgent *agent.QuestionAgent
	scriptAgent   *agent.ScriptAgent
	imageAgent    *agent.ImageAgent
	producer      *producer.Producer
	clipsRepo     *repository.ClipsRepo
	scenesRepo    *repository.ScenesRepo
	themesRepo    *repository.ThemesRepo
	agentsRepo    *repository.AgentsRepo
	tracker       *progress.Tracker
}

func New(
	pool *pgxpool.Pool,
	qa *agent.QuestionAgent,
	sa *agent.ScriptAgent,
	ia *agent.ImageAgent,
	prod *producer.Producer,
	clips *repository.ClipsRepo,
	scenes *repository.ScenesRepo,
	themes *repository.ThemesRepo,
	agents *repository.AgentsRepo,
	tracker *progress.Tracker,
) *Orchestrator {
	return &Orchestrator{
		pool: pool, questionAgent: qa, scriptAgent: sa, imageAgent: ia,
		producer: prod, clipsRepo: clips, scenesRepo: scenes,
		themesRepo: themes, agentsRepo: agents, tracker: tracker,
	}
}

func buildPrompt(cfg *models.AgentConfig) string {
	if cfg.Skills == "" {
		return cfg.SystemPrompt
	}
	return cfg.SystemPrompt + "\n\n## Skills & Guidelines\n" + cfg.Skills
}

func (o *Orchestrator) ProduceWeekly(ctx context.Context, count int) error {
	weekNum := int(time.Now().Unix() / (7 * 24 * 3600))
	category := categories[weekNum%len(categories)]
	log.Printf("Producing %d clips for category: %s", count, category)

	defer o.tracker.FinishProduction()

	o.tracker.StartProduction(1)
	o.tracker.StartClip(1, "Generating questions...")
	o.tracker.StartStep("question")

	qaCfg, err := o.agentsRepo.GetByName(ctx, "question")
	if err != nil {
		o.tracker.FailStep("question", err)
		return fmt.Errorf("get question agent config: %w", err)
	}

	questions, err := o.questionAgent.Generate(ctx, count, category, qaCfg.Model, buildPrompt(qaCfg), qaCfg.Temperature)
	if err != nil {
		o.tracker.FailStep("question", err)
		return fmt.Errorf("generate questions: %w", err)
	}
	o.tracker.CompleteStep("question")
	log.Printf("Generated %d questions", len(questions))

	theme, err := o.themesRepo.GetActive(ctx)
	if err != nil {
		return fmt.Errorf("get active theme: %w", err)
	}

	scriptCfg, err := o.agentsRepo.GetByName(ctx, "script")
	if err != nil {
		return fmt.Errorf("get script agent config: %w", err)
	}
	imageCfg, err := o.agentsRepo.GetByName(ctx, "image")
	if err != nil {
		return fmt.Errorf("get image agent config: %w", err)
	}

	o.tracker.StartProduction(len(questions))
	for i, q := range questions {
		if ctx.Err() != nil {
			log.Printf("Production cancelled, stopping at clip %d/%d", i+1, len(questions))
			o.tracker.AddErrorLog(fmt.Sprintf("Stopped at clip %d/%d", i+1, len(questions)))
			break
		}
		log.Printf("[%d/%d] Processing: %s", i+1, len(questions), q.Question)
		o.tracker.StartClip(i+1, q.Question)
		if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg); err != nil {
			errMsg := fmt.Sprintf("Clip %d failed: %v", i+1, err)
			log.Print(errMsg)
			o.tracker.AddErrorLog(errMsg)
			continue
		}
		o.tracker.CompleteStep("complete")
	}

	log.Println("Weekly production complete")
	return nil
}

func (o *Orchestrator) produceClip(ctx context.Context, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig) error {
	today := time.Now().Format("2006-01-02")
	clip, err := o.clipsRepo.Create(ctx, models.CreateClipRequest{
		Title:          q.Question,
		Question:       q.Question,
		QuestionerName: q.QuestionerName,
		Category:       q.Category,
		PublishDate:    &today,
	})
	if err != nil {
		return fmt.Errorf("create clip: %w", err)
	}

	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status})

	return o.produceClipWithID(ctx, clip.ID, q, theme, scriptCfg, imageCfg)
}

func (o *Orchestrator) produceClipWithID(ctx context.Context, clipID string, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig) error {
	o.tracker.StartStep("script")
	script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, scriptCfg.Model, buildPrompt(scriptCfg), scriptCfg.Temperature)
	if err != nil {
		o.tracker.FailStep("script", err)
		return o.failClip(ctx, clipID, fmt.Errorf("script: %w", err))
	}
	o.tracker.CompleteStep("script")

	for _, scene := range script.Scenes {
		overlays := scene.TextOverlays
		if overlays == nil {
			overlays = []byte("[]")
		}
		o.scenesRepo.Create(ctx, models.CreateSceneRequest{
			ClipID:          clipID,
			SceneNumber:     scene.SceneNumber,
			SceneType:       scene.SceneType,
			TextContent:     scene.TextContent,
			VoiceText:       scene.VoiceText,
			DurationSeconds: scene.DurationSeconds,
			TextOverlays:    overlays,
		})
	}

	o.tracker.StartStep("image_prompts")
	imagePrompts, err := o.imageAgent.GeneratePrompts(ctx, script.Scenes, theme, q.QuestionerName, imageCfg.Model, buildPrompt(imageCfg), imageCfg.Temperature)
	if err != nil {
		o.tracker.FailStep("image_prompts", err)
		return o.failClip(ctx, clipID, fmt.Errorf("image prompts: %w", err))
	}
	o.saveImagePrompts(ctx, clipID, imagePrompts)
	o.tracker.CompleteStep("image_prompts")

	var fullVoice string
	for _, s := range script.Scenes {
		fullVoice += s.VoiceText + " "
	}
	fullVoice = sanitizeVoiceText(fullVoice)

	o.pool.Exec(ctx,
		`INSERT INTO clip_metadata (clip_id, youtube_title, youtube_description, youtube_tags)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (clip_id) DO UPDATE SET youtube_title=$2, youtube_description=$3, youtube_tags=$4`,
		clipID, script.YoutubeTitle, script.YoutubeDescription, script.YoutubeTags)

	return o.runProduction(ctx, clipID, script.Scenes, imagePrompts, fullVoice)
}

func (o *Orchestrator) RetryClip(ctx context.Context, clip *models.Clip) error {
	log.Printf("Retrying failed clip %s: %s", clip.ID, clip.Title)

	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status})

	scenes, err := o.scenesRepo.ListByClip(ctx, clip.ID)
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("list scenes: %w", err))
	}

	q := agent.GeneratedQuestion{
		Question:       clip.Question,
		QuestionerName: clip.QuestionerName,
		Category:       clip.Category,
	}

	if len(scenes) == 0 {
		log.Printf("Retry %s: no scenes found, running full pipeline", clip.ID)
		theme, err := o.themesRepo.GetActive(ctx)
		if err != nil {
			return o.failClip(ctx, clip.ID, fmt.Errorf("get theme: %w", err))
		}
		scriptCfg, err := o.agentsRepo.GetByName(ctx, "script")
		if err != nil {
			return o.failClip(ctx, clip.ID, fmt.Errorf("get script config: %w", err))
		}
		imageCfg, err := o.agentsRepo.GetByName(ctx, "image")
		if err != nil {
			return o.failClip(ctx, clip.ID, fmt.Errorf("get image config: %w", err))
		}
		return o.produceClipWithID(ctx, clip.ID, q, theme, scriptCfg, imageCfg)
	}

	if scenes[0].ImagePrompt == "" {
		log.Printf("Retry %s: scenes exist, resuming from image prompts (saving Claude script credits)", clip.ID)
		return o.resumeFromImagePrompts(ctx, clip.ID, scenes, q.QuestionerName)
	}

	log.Printf("Retry %s: scenes + image prompts exist, resuming from production (saving all Claude credits)", clip.ID)
	return o.resumeFromProduction(ctx, clip.ID, scenes, q.QuestionerName)
}

func (o *Orchestrator) resumeFromImagePrompts(ctx context.Context, clipID string, scenes []models.Scene, questionerName string) error {
	theme, err := o.themesRepo.GetActive(ctx)
	if err != nil {
		return o.failClip(ctx, clipID, fmt.Errorf("get theme: %w", err))
	}
	imageCfg, err := o.agentsRepo.GetByName(ctx, "image")
	if err != nil {
		return o.failClip(ctx, clipID, fmt.Errorf("get image config: %w", err))
	}

	genScenes := scenesToGenerated(scenes)

	o.tracker.StartStep("image_prompts")
	imagePrompts, err := o.imageAgent.GeneratePrompts(ctx, genScenes, theme, questionerName, imageCfg.Model, buildPrompt(imageCfg), imageCfg.Temperature)
	if err != nil {
		o.tracker.FailStep("image_prompts", err)
		return o.failClip(ctx, clipID, fmt.Errorf("image prompts: %w", err))
	}
	o.saveImagePrompts(ctx, clipID, imagePrompts)
	o.tracker.CompleteStep("image_prompts")

	voiceScript := buildVoiceScript(scenes)
	return o.runProduction(ctx, clipID, genScenes, imagePrompts, voiceScript)
}

func (o *Orchestrator) resumeFromProduction(ctx context.Context, clipID string, scenes []models.Scene, questionerName string) error {
	imagePrompts, err := parseImagePrompts(scenes)
	if err != nil {
		log.Printf("Retry %s: failed to parse image prompts, regenerating: %v", clipID, err)
		return o.resumeFromImagePrompts(ctx, clipID, scenes, questionerName)
	}

	genScenes := scenesToGenerated(scenes)
	voiceScript := buildVoiceScript(scenes)
	return o.runProduction(ctx, clipID, genScenes, imagePrompts, voiceScript)
}

func (o *Orchestrator) runProduction(ctx context.Context, clipID string, scenes []agent.GeneratedScene, imagePrompts []agent.SceneImagePrompts, voiceScript string) error {
	result, err := o.producer.Produce(ctx, clipID, scenes, imagePrompts, voiceScript)
	if err != nil {
		return o.failClip(ctx, clipID, fmt.Errorf("produce: %w", err))
	}

	readyStatus := "ready"
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{
		Status:       &readyStatus,
		Video169URL:  &result.Video169URL,
		Video916URL:  &result.Video916URL,
		ThumbnailURL: &result.ThumbnailURL,
		AnswerScript: &voiceScript,
		VoiceScript:  &voiceScript,
	})

	log.Printf("Clip ready (resumed): %s", clipID)
	return nil
}

func (o *Orchestrator) saveImagePrompts(ctx context.Context, clipID string, prompts []agent.SceneImagePrompts) {
	for _, p := range prompts {
		data, err := json.Marshal(map[string]string{"169": p.ImagePrompt169, "916": p.ImagePrompt916})
		if err != nil {
			log.Printf("Failed to marshal image prompt for clip %s scene %d: %v", clipID, p.SceneNumber, err)
			continue
		}
		if err := o.scenesRepo.UpdateImagePrompt(ctx, clipID, p.SceneNumber, string(data)); err != nil {
			log.Printf("Failed to save image prompt for clip %s scene %d: %v", clipID, p.SceneNumber, err)
		}
	}
}

func scenesToGenerated(scenes []models.Scene) []agent.GeneratedScene {
	gen := make([]agent.GeneratedScene, len(scenes))
	for i, s := range scenes {
		gen[i] = agent.GeneratedScene{
			SceneNumber:     s.SceneNumber,
			SceneType:       s.SceneType,
			TextContent:     s.TextContent,
			VoiceText:       s.VoiceText,
			DurationSeconds: s.DurationSeconds,
			TextOverlays:    s.TextOverlays,
		}
	}
	return gen
}

func buildVoiceScript(scenes []models.Scene) string {
	var b strings.Builder
	for _, s := range scenes {
		b.WriteString(s.VoiceText)
		b.WriteString(" ")
	}
	return sanitizeVoiceText(b.String())
}

func parseImagePrompts(scenes []models.Scene) ([]agent.SceneImagePrompts, error) {
	var prompts []agent.SceneImagePrompts
	for _, s := range scenes {
		if s.ImagePrompt == "" {
			return nil, fmt.Errorf("scene %d has no image prompt", s.SceneNumber)
		}
		var parsed map[string]string
		if err := json.Unmarshal([]byte(s.ImagePrompt), &parsed); err != nil {
			return nil, fmt.Errorf("scene %d: invalid image prompt JSON: %w", s.SceneNumber, err)
		}
		if parsed["169"] == "" || parsed["916"] == "" {
			return nil, fmt.Errorf("scene %d: missing image prompt keys", s.SceneNumber)
		}
		prompts = append(prompts, agent.SceneImagePrompts{
			SceneNumber:    s.SceneNumber,
			ImagePrompt169: parsed["169"],
			ImagePrompt916: parsed["916"],
		})
	}
	if len(prompts) == 0 {
		return nil, fmt.Errorf("no image prompts found")
	}
	return prompts, nil
}

func (o *Orchestrator) failClip(ctx context.Context, clipID string, err error) error {
	status := "failed"
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{Status: &status})
	o.clipsRepo.IncrementRetry(ctx, clipID, err.Error())
	return err
}
