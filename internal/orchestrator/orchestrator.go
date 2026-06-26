package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

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
// Brand mentions are normalized using the brandAliases map (loaded from DB settings).
func sanitizeVoiceText(s string, brandAliases map[string]string) string {
	keys := make([]string, 0, len(brandAliases))
	for k := range brandAliases {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, k := range keys {
		s = strings.ReplaceAll(s, k, brandAliases[k])
	}
	s = urlRegex.ReplaceAllString(s, "")
	s = atHandleRgx.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// scriptNarration joins all scene voice_texts into the full narration the
// SceneAgent breaks down. The legacy ScriptAgent emits a single scene whose
// voice_text is the whole narration; joining is defensive against multi-scene.
func scriptNarration(script *agent.GeneratedScript) string {
	parts := make([]string, 0, len(script.Scenes))
	for _, s := range script.Scenes {
		if t := strings.TrimSpace(s.VoiceText); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

type Orchestrator struct {
	settingsRepo  *repository.SettingsRepo
	formatsRepo   *repository.FormatsRepo
	questionAgent *agent.QuestionAgent
	scriptAgent   *agent.ScriptAgent
	imageAgent    *agent.ImageAgent
	sceneAgent    *agent.SceneAgent
	criticAgent   *agent.CriticAgent
	visualQAAgent *agent.VisualQAAgent
	producer      *producer.Producer
	clipsRepo     *repository.ClipsRepo
	scenesRepo    *repository.ScenesRepo
	critiquesRepo *repository.CritiquesRepo
	visualQARepo  *repository.VisualQARepo
	themesRepo    *repository.ThemesRepo
	agentsRepo    *repository.AgentsRepo
	tracker       *progress.Tracker
}

func New(
	qa *agent.QuestionAgent,
	sa *agent.ScriptAgent,
	ia *agent.ImageAgent,
	sca *agent.SceneAgent,
	ca *agent.CriticAgent,
	vqa *agent.VisualQAAgent,
	prod *producer.Producer,
	clips *repository.ClipsRepo,
	scenes *repository.ScenesRepo,
	critiques *repository.CritiquesRepo,
	visualqa *repository.VisualQARepo,
	themes *repository.ThemesRepo,
	agents *repository.AgentsRepo,
	settings *repository.SettingsRepo,
	formats *repository.FormatsRepo,
	tracker *progress.Tracker,
) *Orchestrator {
	return &Orchestrator{
		settingsRepo: settings, formatsRepo: formats, questionAgent: qa, scriptAgent: sa, imageAgent: ia,
		sceneAgent: sca, criticAgent: ca, visualQAAgent: vqa,
		producer: prod, clipsRepo: clips, scenesRepo: scenes, critiquesRepo: critiques, visualQARepo: visualqa,
		themesRepo: themes, agentsRepo: agents, tracker: tracker,
	}
}

// ErrProductionRunning is returned when a production/render is already in
// progress. The render pipeline shares one Chrome/CPU budget, so every entry
// point refuses to start a second concurrent run rather than oversubscribe it.
var ErrProductionRunning = errors.New("production already in progress")

func (o *Orchestrator) ProduceWeekly(ctx context.Context, count int) error {
	// Pre-flight: don't kick off an expensive run with no kie credits. Fail-open
	// on a check error (don't block production on a flaky meta-check) — only abort
	// when we positively know the balance is empty.
	if credits, err := o.producer.KieCredits(ctx); err != nil {
		log.Printf("kie credit pre-check skipped (non-fatal): %v", err)
	} else if credits <= 0 {
		return fmt.Errorf("kie เครดิตหมด (เหลือ %d) — เติมเครดิตที่ kie.ai ก่อนผลิต", credits)
	} else {
		log.Printf("kie credits OK: %d", credits)
	}

	weekNum := int(time.Now().Unix() / (7 * 24 * 3600))

	categories, err := o.settingsRepo.GetCategories(ctx)
	if err != nil {
		return fmt.Errorf("read categories: %w", err)
	}
	if len(categories) == 0 {
		return fmt.Errorf("no categories configured")
	}
	category := categories[weekNum%len(categories)]

	brandAliases, err := o.settingsRepo.GetBrandAliases(ctx)
	if err != nil {
		return fmt.Errorf("read brand aliases: %w", err)
	}

	format, err := o.formatsRepo.PickNext(ctx)
	if err != nil {
		return fmt.Errorf("pick content format: %w", err)
	}

	persona, err := o.settingsRepo.Get(ctx, "audience_persona")
	if err != nil {
		log.Printf("Warning: audience_persona not set, using empty: %v", err)
		persona = ""
	}

	log.Printf("Producing %d clips — category: %s, format: %s", count, category, format.DisplayName)

	if !o.tracker.StartProduction(1) {
		return ErrProductionRunning
	}
	defer o.tracker.FinishProduction()

	// Register cancellation here — tied to the gate, so only the run that won
	// StartProduction can be stopped, and a refused concurrent caller can't clobber
	// the active run's cancel func (which a handler-level SetCancelFunc could).
	ctx, cancel := context.WithCancel(ctx)
	o.tracker.SetCancelFunc(cancel)
	defer cancel()

	o.tracker.StartClip(1, "Generating questions...")
	o.tracker.StartStep("question")

	qaCfg, err := o.agentsRepo.GetByName(ctx, "question")
	if err != nil {
		o.tracker.FailStep("question", err)
		return fmt.Errorf("get question agent config: %w", err)
	}

	questions, err := o.questionAgent.Generate(ctx, count, category, format, persona, qaCfg)
	if errors.Is(err, agent.ErrNoFreshNews) {
		// No reliable news found — never fabricate news; produce a Q&A clip instead.
		log.Println("No fresh news available, falling back to Q&A format")
		format, err = o.formatsRepo.GetByName(ctx, "qa")
		if err != nil {
			o.tracker.FailStep("question", err)
			return fmt.Errorf("fallback to qa format: %w", err)
		}
		questions, err = o.questionAgent.Generate(ctx, count, category, format, persona, qaCfg)
	}
	if err != nil {
		o.tracker.FailStep("question", err)
		return fmt.Errorf("generate questions: %w", err)
	}
	o.tracker.CompleteStep("question")
	if len(questions) > count {
		questions = questions[:count]
	}
	log.Printf("Generated %d questions (requested %d)", len(questions), count)

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

	o.tracker.SetTotalClips(len(questions))
	for i, q := range questions {
		if ctx.Err() != nil {
			log.Printf("Production cancelled, stopping at clip %d/%d", i+1, len(questions))
			o.tracker.AddErrorLog(fmt.Sprintf("Stopped at clip %d/%d", i+1, len(questions)))
			break
		}
		log.Printf("[%d/%d] Processing: %s", i+1, len(questions), q.Question)
		o.tracker.StartClip(i+1, q.Question)
		if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format, persona); err != nil {
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

func (o *Orchestrator) produceClip(ctx context.Context, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string, format *models.ContentFormat, persona string) error {
	today := time.Now().Format("2006-01-02")
	clip, err := o.clipsRepo.Create(ctx, models.CreateClipRequest{
		Title:          q.Question,
		Question:       q.Question,
		QuestionerName: q.QuestionerName,
		Category:       q.Category,
		PublishDate:    &today,
		ContentFormat:  format.FormatName,
	})
	if err != nil {
		return fmt.Errorf("create clip: %w", err)
	}

	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status})

	return o.produceClipWithID(ctx, clip.ID, q, theme, scriptCfg, imageCfg, brandAliases, format, persona)
}

// Target shape for the multi-scene explainer (design: 60–90 s, 6–10 scenes).
const (
	targetSceneCount  = 8
	targetDurationSec = 75
)

// brandTailRe matches a trailing brand mention in any form the LLM might add:
// " | Ads Vance", "| Ads Vance", "{Ads Vance}", "(Ads Vance)", "[Ads Vance]", " Ads Vance".
// Matching the bare name (no delimiter) at the end is deliberate — the canonical
// suffix is re-appended afterward, so a trailing "Ads Vance" is always treated as brand.
var brandTailRe = regexp.MustCompile(`(?i)\s*[|({\[]?\s*ads\s*vance\s*[)}\]]?\s*$`)

func validateScript(script *agent.GeneratedScript) {
	const suffix = " | Ads Vance"
	// YouTube's hard title limit is 100 runes; 90 leaves margin for the suffix
	// while letting most full Thai titles through untouched (truncation cuts Thai
	// mid-word since there are no spaces, so we avoid it whenever possible).
	const maxLen = 90
	// Trailing chars left dangling after brand removal / mid-title truncation:
	// whitespace, pipe, hyphen, and unmatched opening brackets (their content was cut).
	const trimCutset = " |-({["

	// Strip any brand variant the LLM appended — repeat to catch doubled brands.
	title := script.YoutubeTitle
	for {
		stripped := strings.TrimRight(brandTailRe.ReplaceAllString(title, ""), trimCutset)
		if stripped == title {
			break
		}
		title = stripped
	}
	title = strings.TrimSpace(title)

	maxContent := maxLen - len([]rune(suffix))
	if titleRunes := []rune(title); len(titleRunes) > maxContent {
		// Reserve one rune for an ellipsis so the cut reads as intentional, never
		// a mid-word fragment (Thai has no spaces, so a word-boundary cut isn't
		// reliable — the ellipsis is the safe, language-agnostic signal).
		cut := strings.TrimRight(strings.TrimSpace(string(titleRunes[:maxContent-1])), trimCutset)
		title = cut + "…"
		log.Printf("Warning: youtube_title truncated to fit %d chars", maxLen)
	}

	script.YoutubeTitle = title + suffix
}

func (o *Orchestrator) produceClipWithID(ctx context.Context, clipID string, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string, format *models.ContentFormat, persona string) error {
	o.tracker.StartStep("script")
	script, err := o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, format, persona, scriptCfg)
	if err != nil {
		o.tracker.FailStep("script", err)
		return o.failClip(ctx, clipID, fmt.Errorf("script: %w", err))
	}
	validateScript(script)
	o.tracker.CompleteStep("script")

	// ── Break the narration into 6-10 animated scenes (SceneAgent, Claude) ──
	o.tracker.StartStep("scene")
	sceneCfg, err := o.agentsRepo.GetByName(ctx, "scene")
	if err != nil {
		o.tracker.FailStep("scene", err)
		return o.failClip(ctx, clipID, fmt.Errorf("get scene config: %w", err))
	}
	narration := scriptNarration(script)
	scenes, err := o.sceneAgent.Generate(ctx, narration, targetSceneCount, targetDurationSec, theme, sceneCfg)
	if err != nil {
		o.tracker.FailStep("scene", err)
		return o.failClip(ctx, clipID, fmt.Errorf("scene breakdown: %w", err))
	}
	o.tracker.CompleteStep("scene")

	// ── Content Critic: review + revise content before render. Optional gate;
	//    on disable/error/anomaly it returns the original content unchanged. ──
	if criticCfg, cErr := o.agentsRepo.GetByName(ctx, "critic"); cErr == nil && criticCfg.Enabled {
		o.tracker.StartStep("critic")
		res := o.criticAgent.Review(ctx, agent.CriticInput{
			Question:  q.Question,
			Narration: narration,
			Scenes:    scenes,
			Metadata: agent.CriticMetadata{
				YoutubeTitle:       script.YoutubeTitle,
				YoutubeDescription: script.YoutubeDescription,
				YoutubeTags:        script.YoutubeTags,
			},
		}, criticCfg)
		scenes = res.Scenes
		script.YoutubeTitle = res.Metadata.YoutubeTitle
		script.YoutubeDescription = res.Metadata.YoutubeDescription
		script.YoutubeTags = res.Metadata.YoutubeTags
		// Re-enforce title length + brand suffix on the critic-revised metadata.
		validateScript(script)
		if scoreJSON, mErr := json.Marshal(res.Score); mErr == nil {
			changesJSON, _ := json.Marshal(res.Changes)
			if pErr := o.critiquesRepo.Create(ctx, clipID, scoreJSON, changesJSON, res.Applied); pErr != nil {
				log.Printf("critic: persist critique failed (non-fatal): %v", pErr)
			}
		} else {
			log.Printf("critic: marshal score failed, critique not persisted (non-fatal): %v", mErr)
		}
		o.tracker.CompleteStep("critic")
	}

	// Sanitize each scene's narration for TTS (brand aliases, strip URLs/@handles).
	// Runs after the critic so any rewritten voice_text is cleaned too.
	for i := range scenes {
		scenes[i].VoiceText = sanitizeVoiceText(scenes[i].VoiceText, brandAliases)
	}

	// Persist scenes with the 2b layout/caption fields.
	for _, scene := range scenes {
		emphasis, mErr := json.Marshal(scene.EmphasisWords)
		if mErr != nil || len(emphasis) == 0 {
			emphasis = []byte("[]")
		}
		overlays := scene.TextOverlays
		if overlays == nil {
			overlays = []byte("[]")
		}
		o.scenesRepo.Create(ctx, models.CreateSceneRequest{
			ClipID:          clipID,
			SceneNumber:     scene.SceneNumber,
			SceneType:       scene.SceneType,
			TextContent:     scene.TextContent,
			ImagePrompt:     scene.ImagePrompt,
			VoiceText:       scene.VoiceText,
			DurationSeconds: scene.DurationSeconds,
			TextOverlays:    overlays,
			LayoutVariant:   scene.LayoutVariant,
			OnScreenText:    scene.OnScreenText,
			EmphasisWords:   emphasis,
			Beat:            scene.Beat,
			CaptionStyle:    scene.CaptionStyle,
		})
	}

	// Metadata from the validated script.
	o.clipsRepo.UpsertMetadata(ctx, models.ClipMetadata{
		ClipID:       clipID,
		YoutubeTitle: &script.YoutubeTitle,
		YoutubeDesc:  &script.YoutubeDescription,
		YoutubeTags:  script.YoutubeTags,
	})

	// ── Assemble the multi-scene 9:16 video + thumbnail + upload ──
	result, err := o.producer.ProduceHyperframes916(ctx, clipID, scenes)
	if err != nil {
		return o.failClip(ctx, clipID, fmt.Errorf("produce hyperframes: %w", err))
	}

	// ── Visual QA: look at the rendered frames; block auto-publish on a
	//    confident visual defect. Optional gate; disabled/absent ⇒ 'ready'.
	//    Fail-safe is fail-OPEN: infra errors never block (see VisualQAAgent). ──
	status := "ready"
	if qaCfg, qErr := o.agentsRepo.GetByName(ctx, "visual_qa"); qErr == nil && qaCfg.Enabled && result.LocalVideo916Path != "" {
		o.tracker.StartStep("visual_qa")
		frames := o.extractQAFrames(clipID, result.LocalVideo916Path, scenes)
		qaRes := o.visualQAAgent.Review(ctx, agent.VisualQAInput{
			Question: q.Question,
			Frames:   frames,
		}, qaCfg)
		if wErr := o.visualQARepo.Create(ctx, clipID, qaRes.Passed, agent.MarshalVerdicts(qaRes.Verdicts)); wErr != nil {
			log.Printf("visualqa: persist result failed (non-fatal): %v", wErr)
		}
		if !qaRes.Passed {
			status = "needs_review"
			log.Printf("visualqa: clip %s FAILED — status=needs_review (publish blocked); verdicts=%s",
				clipID, string(agent.MarshalVerdicts(qaRes.Verdicts)))
		}
		o.tracker.CompleteStep("visual_qa")
	}

	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{
		Status:       &status,
		Video916URL:  &result.Video916URL,
		ThumbnailURL: &result.ThumbnailURL,
		VoiceScript:  &narration,
		AnswerScript: &narration,
	})
	o.clipsRepo.ClearFailReason(ctx, clipID)
	if status == "ready" {
		log.Printf("Clip ready (hyperframes): %s", clipID)
	}
	return nil
}

func (o *Orchestrator) RetryAllFailed(ctx context.Context, maxRetries int) error {
	failed, err := o.clipsRepo.ListFailed(ctx, maxRetries)
	if err != nil {
		return fmt.Errorf("list failed: %w", err)
	}
	if len(failed) == 0 {
		return nil
	}

	if !o.tracker.StartProduction(len(failed)) {
		return ErrProductionRunning
	}
	defer o.tracker.FinishProduction()

	ctx, cancel := context.WithCancel(ctx)
	o.tracker.SetCancelFunc(cancel)
	defer cancel()

	for i, clip := range failed {
		if ctx.Err() != nil {
			o.tracker.AddErrorLog(fmt.Sprintf("Retry stopped at clip %d/%d", i+1, len(failed)))
			break
		}
		c := clip
		o.tracker.StartClip(i+1, c.Title)
		log.Printf("Retrying clip %s (%s)", c.ID, c.Title)
		if err := o.RetryClip(ctx, &c); err != nil {
			log.Printf("Retry failed for %s: %v", c.ID, err)
			o.tracker.AddErrorLog(fmt.Sprintf("Retry %s failed: %v", c.ID, err))
		} else {
			log.Printf("Retry succeeded for %s", c.ID)
			o.tracker.CompleteStep("complete")
		}
	}
	return nil
}

func (o *Orchestrator) RetryClip(ctx context.Context, clip *models.Clip) error {
	log.Printf("Retrying failed clip %s: %s", clip.ID, clip.Title)

	brandAliases, err := o.settingsRepo.GetBrandAliases(ctx)
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("read brand aliases: %w", err))
	}

	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status})

	q := agent.GeneratedQuestion{
		Question:       clip.Question,
		QuestionerName: clip.QuestionerName,
		Category:       clip.Category,
	}

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
	format, err := o.formatsRepo.GetByName(ctx, "qa")
	if err != nil {
		format = &models.ContentFormat{FormatName: "qa", DisplayName: "Q&A"}
	}
	persona, _ := o.settingsRepo.Get(ctx, "audience_persona")

	return o.produceClipWithID(ctx, clip.ID, q, theme, scriptCfg, imageCfg, brandAliases, format, persona)
}

func (o *Orchestrator) resumeFromImagePrompts(ctx context.Context, clipID string, scenes []models.Scene, questionerName string, brandAliases map[string]string) error {
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
	imagePrompts, err := o.imageAgent.GeneratePrompts(ctx, genScenes, theme, questionerName, imageCfg)
	if err != nil {
		o.tracker.FailStep("image_prompts", err)
		return o.failClip(ctx, clipID, fmt.Errorf("image prompts: %w", err))
	}
	o.saveImagePrompts(ctx, clipID, imagePrompts)
	o.tracker.CompleteStep("image_prompts")

	voiceScript := buildVoiceScript(scenes, brandAliases)
	return o.runProduction(ctx, clipID, genScenes, imagePrompts, voiceScript)
}

func (o *Orchestrator) resumeFromProduction(ctx context.Context, clipID string, scenes []models.Scene, questionerName string, brandAliases map[string]string) error {
	imagePrompts, err := parseImagePrompts(scenes)
	if err != nil {
		log.Printf("Retry %s: failed to parse image prompts, regenerating: %v", clipID, err)
		return o.resumeFromImagePrompts(ctx, clipID, scenes, questionerName, brandAliases)
	}

	genScenes := scenesToGenerated(scenes)
	voiceScript := buildVoiceScript(scenes, brandAliases)
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
	o.clipsRepo.ClearFailReason(ctx, clipID)

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

func buildVoiceScript(scenes []models.Scene, brandAliases map[string]string) string {
	var b strings.Builder
	for _, s := range scenes {
		b.WriteString(s.VoiceText)
		b.WriteString(" ")
	}
	return sanitizeVoiceText(b.String(), brandAliases)
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

// sceneMidTimestamps returns the midpoint timestamp (seconds) of each scene's
// window, reconstructed from cumulative DurationSeconds. Pure — the exactness
// only matters enough to land inside the scene.
func sceneMidTimestamps(scenes []agent.GeneratedScene) []float64 {
	mids := make([]float64, len(scenes))
	cursor := 0.0
	for i, s := range scenes {
		d := s.DurationSeconds
		if d < 0 {
			d = 0
		}
		mids[i] = cursor + d/2
		cursor += d
	}
	return mids
}

// extractQAFrames extracts one PNG frame per scene from the local MP4 and pairs
// it with the scene's text. A per-scene extraction failure is logged and that
// frame is dropped (Visual QA fails open on missing frames).
func (o *Orchestrator) extractQAFrames(clipID, mp4Path string, scenes []agent.GeneratedScene) []agent.QAFrame {
	mids := sceneMidTimestamps(scenes)
	frames := make([]agent.QAFrame, 0, len(scenes))
	for i, s := range scenes {
		outPath := filepath.Join(filepath.Dir(mp4Path), fmt.Sprintf("qa-scene%d.png", s.SceneNumber))
		if err := o.producer.FFmpeg().ExtractFrameAt(mp4Path, outPath, mids[i]); err != nil {
			log.Printf("visualqa: clip %s scene %d frame extract failed (skip): %v", clipID, s.SceneNumber, err)
			continue
		}
		png, err := os.ReadFile(outPath)
		os.Remove(outPath) // bytes are in memory now; don't leave QA frame PNGs on disk
		if err != nil {
			log.Printf("visualqa: clip %s scene %d frame read failed (skip): %v", clipID, s.SceneNumber, err)
			continue
		}
		frames = append(frames, agent.QAFrame{
			SceneNumber:  s.SceneNumber,
			PNG:          png,
			OnScreenText: s.OnScreenText,
			VoiceText:    s.VoiceText,
		})
	}
	return frames
}
