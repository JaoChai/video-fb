package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
	"github.com/jaochai/video-fb/internal/progress"
	"github.com/jaochai/video-fb/internal/repository"
)

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

	o.tracker.StartStep("script")
	script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, scriptCfg.Model, buildPrompt(scriptCfg), scriptCfg.Temperature)
	if err != nil {
		o.tracker.FailStep("script", err)
		return o.failClip(ctx, clip.ID, fmt.Errorf("script: %w", err))
	}
	o.tracker.CompleteStep("script")

	for _, scene := range script.Scenes {
		overlays := scene.TextOverlays
		if overlays == nil {
			overlays = []byte("[]")
		}
		o.scenesRepo.Create(ctx, models.CreateSceneRequest{
			ClipID:          clip.ID,
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
		return o.failClip(ctx, clip.ID, fmt.Errorf("image prompts: %w", err))
	}
	o.tracker.CompleteStep("image_prompts")

	var fullVoice string
	for _, s := range script.Scenes {
		fullVoice += s.VoiceText + " "
	}

	result, err := o.producer.Produce(ctx, clip.ID, script.Scenes, imagePrompts, fullVoice)
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("produce: %w", err))
	}

	readyStatus := "ready"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{
		Status:       &readyStatus,
		Video169URL:  &result.Video169URL,
		Video916URL:  &result.Video916URL,
		ThumbnailURL: &result.ThumbnailURL,
		AnswerScript: &fullVoice,
		VoiceScript:  &fullVoice,
	})

	o.pool.Exec(ctx,
		`INSERT INTO clip_metadata (clip_id, youtube_title, youtube_description, youtube_tags)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (clip_id) DO UPDATE SET youtube_title=$2, youtube_description=$3, youtube_tags=$4`,
		clip.ID, script.YoutubeTitle, script.YoutubeDescription, script.YoutubeTags)

	log.Printf("Clip ready: %s — %s", clip.ID, q.Question)
	return nil
}

func (o *Orchestrator) failClip(ctx context.Context, clipID string, err error) error {
	status := "failed"
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{Status: &status})
	return err
}
