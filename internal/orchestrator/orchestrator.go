package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
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
	settingsRepo    *repository.SettingsRepo
	formatsRepo     *repository.FormatsRepo
	questionAgent   *agent.QuestionAgent
	scriptAgent     *agent.ScriptAgent
	imageAgent      *agent.ImageAgent
	sceneAgent      *agent.SceneAgent
	criticAgent     *agent.CriticAgent
	visualQAAgent   *agent.VisualQAAgent
	autoReviewAgent *agent.AutoReviewAgent
	producer        *producer.Producer
	clipsRepo       *repository.ClipsRepo
	scenesRepo      *repository.ScenesRepo
	critiquesRepo   *repository.CritiquesRepo
	visualQARepo    *repository.VisualQARepo
	autoReviewsRepo *repository.AutoReviewsRepo
	themesRepo      *repository.ThemesRepo
	agentsRepo      *repository.AgentsRepo
	analyticsRepo   *repository.AnalyticsRepo
	tracker         *progress.Tracker
}

func New(
	qa *agent.QuestionAgent,
	sa *agent.ScriptAgent,
	ia *agent.ImageAgent,
	sca *agent.SceneAgent,
	ca *agent.CriticAgent,
	vqa *agent.VisualQAAgent,
	ara *agent.AutoReviewAgent,
	prod *producer.Producer,
	clips *repository.ClipsRepo,
	scenes *repository.ScenesRepo,
	critiques *repository.CritiquesRepo,
	visualqa *repository.VisualQARepo,
	autoreviews *repository.AutoReviewsRepo,
	themes *repository.ThemesRepo,
	agents *repository.AgentsRepo,
	analytics *repository.AnalyticsRepo,
	settings *repository.SettingsRepo,
	formats *repository.FormatsRepo,
	tracker *progress.Tracker,
) *Orchestrator {
	return &Orchestrator{
		settingsRepo: settings, formatsRepo: formats, questionAgent: qa, scriptAgent: sa, imageAgent: ia,
		sceneAgent: sca, criticAgent: ca, visualQAAgent: vqa, autoReviewAgent: ara,
		producer: prod, clipsRepo: clips, scenesRepo: scenes, critiquesRepo: critiques, visualQARepo: visualqa,
		autoReviewsRepo: autoreviews,
		themesRepo:      themes, agentsRepo: agents, analyticsRepo: analytics, tracker: tracker,
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
	var topicStats string
	if v, err := o.settingsRepo.Get(ctx, "topic_stats_enabled"); err != nil || v != "false" {
		// Enabled by default; only the explicit value "false" disables it (kill switch).
		if scores, err := o.analyticsRepo.TopicPerformance(ctx, 30, 3); err != nil {
			log.Printf("topic performance unavailable, using round-robin category: %v", err)
		} else {
			category = PickCategoryWeighted(categories, scores, weekNum, rand.Intn)
			topicStats = FormatTopicStats(scores)
		}
	}

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

	questions, err := o.questionAgent.Generate(ctx, count, category, format, persona, topicStats, qaCfg)
	if errors.Is(err, agent.ErrNoFreshNews) {
		// No reliable news found — never fabricate news; produce a Q&A clip instead.
		log.Println("No fresh news available, falling back to Q&A format")
		format, err = o.formatsRepo.GetByName(ctx, "qa")
		if err != nil {
			o.tracker.FailStep("question", err)
			return fmt.Errorf("fallback to qa format: %w", err)
		}
		questions, err = o.questionAgent.Generate(ctx, count, category, format, persona, topicStats, qaCfg)
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
	anyFailed := false
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
			anyFailed = true
			continue
		}
		o.tracker.CompleteStep("complete")
	}
	if anyFailed {
		o.nudgeRetry()
	}

	log.Println("Weekly production complete")
	return nil
}

func (o *Orchestrator) produceClip(ctx context.Context, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string, format *models.ContentFormat, persona string) error {
	preset := producer.PresetByKey("editorial-bold")
	if producer.StylePresetsEnabled() {
		last, _ := o.clipsRepo.LastStylePreset(ctx)
		preset = producer.PickPreset(last)
		if producer.StylePresetsPerformanceEnabled() {
			scores, err := o.analyticsRepo.PresetRetention(ctx, producer.DefaultWindowDays)
			if err != nil {
				log.Printf("preset perf: scores unavailable (%v); uniform pick %s", err, preset.Key)
			} else {
				preset = producer.PickPresetWeighted(last, scores,
					producer.DefaultEpsilon, producer.DefaultMinClips, rand.Intn)
				log.Printf("preset perf: picked %s (window=%dd, %d preset scores)",
					preset.Key, producer.DefaultWindowDays, len(scores))
			}
		}
	}

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
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status, StylePreset: &preset.Key})

	return o.produceClipWithID(ctx, clip.ID, q, theme, preset, scriptCfg, imageCfg, brandAliases, format, persona)
}

// Target shape for the multi-scene explainer (design: 60–90 s, 6–10 scenes).
const (
	targetSceneCount  = 8
	targetDurationSec = 75
)

// production_stage checkpoint values (see migration 042). A clip at
// stageContentReady or later has scenes+metadata persisted, so a retry can skip
// the LLM stages and resume at rendering.
const (
	stageContentReady = "content_ready"
	stageRendered     = "rendered"
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

func (o *Orchestrator) produceClipWithID(ctx context.Context, clipID string, q agent.GeneratedQuestion, theme *models.BrandTheme, preset producer.StylePreset, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string, format *models.ContentFormat, persona string) error {
	// Derive a per-clip theme so text agents describe the same colors that get
	// rendered. When the flag is off clipTheme == theme — no behavior change.
	clipTheme := theme
	if producer.StylePresetsEnabled() {
		clipTheme = preset.AsTheme(theme)
	}

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
	scenes, err := o.sceneAgent.Generate(ctx, narration, targetSceneCount, targetDurationSec, clipTheme, sceneCfg)
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
			Layout:          scene.Layout,
			Content:         scene.Content,
		})
	}

	// Metadata from the validated script.
	o.clipsRepo.UpsertMetadata(ctx, models.ClipMetadata{
		ClipID:       clipID,
		YoutubeTitle: &script.YoutubeTitle,
		YoutubeDesc:  &script.YoutubeDescription,
		YoutubeTags:  script.YoutubeTags,
	})

	// Content (script/scenes/critic/metadata) is now durably persisted — checkpoint
	// so a later failure resumes at rendering instead of regenerating content.
	contentStage := stageContentReady
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{ProductionStage: &contentStage})

	return o.renderAndFinalize(ctx, clipID, q, scenes, preset, narration)
}

// renderAndFinalize runs the media/render/upload + visual-QA tail shared by the
// full produce path and the resume path. It assumes scenes + metadata are already
// persisted. On render failure it fails the clip (retriable); on success it marks
// the clip ready/needs_review and records stage=rendered.
func (o *Orchestrator) renderAndFinalize(ctx context.Context, clipID string, q agent.GeneratedQuestion, scenes []agent.GeneratedScene, preset producer.StylePreset, narration string) error {
	result, err := o.producer.ProduceHyperframes916(ctx, clipID, scenes, preset)
	if err != nil {
		return o.failClip(ctx, clipID, fmt.Errorf("produce hyperframes: %w", err))
	}

	// Persist real per-scene durations (from measured voice bounds); scenes were
	// created with 0 because the scene agent never emits durations.
	if len(result.SceneDurations) > 0 {
		if derr := o.scenesRepo.UpdateDurations(ctx, clipID, result.SceneDurations); derr != nil {
			log.Printf("persist scene durations failed (non-fatal) for clip %s: %v", clipID, derr)
		}
	}

	// Reflect the measured durations onto the in-memory scenes so QA frame sampling
	// sees real per-scene lengths (the scene agent emits 0). Without this the QA path
	// falls back to scene-unaware slicing even though the fix is in place.
	for i := range scenes {
		if i < len(result.SceneDurations) {
			scenes[i].DurationSeconds = result.SceneDurations[i]
		}
	}

	status := "ready"

	// A render that emitted browser errors is silently frozen (exits 0, looks fine
	// to still-frame QA). Retry once via the failed-clip tick; if it's still broken
	// after a retry, route to human review instead of publishing.
	if result.RenderFlagged {
		retryCount := 0
		if clip, gErr := o.clipsRepo.GetByID(ctx, clipID); gErr == nil && clip != nil {
			retryCount = clip.RetryCount
		} else if gErr != nil {
			// Defaulting to 0 (retry) is safe (bounded by maxClipRetries), but log it
			// so a DB blip that mislabels a repeat failure as a first offense is visible.
			log.Printf("clip %s: render gate could not read retry_count (%v) — treating as first offense", clipID, gErr)
		}
		switch producer.RenderGateDecision(true, producer.RenderErrorGateEnabled(), retryCount) {
		case producer.RenderGateRetry:
			return o.failClip(ctx, clipID, fmt.Errorf("render emitted browser errors — retrying"))
		case producer.RenderGateReview:
			status = "needs_review"
			log.Printf("clip %s: render browser errors persisted after retry — status=needs_review (publish blocked)", clipID)
		}
	}

	// Visual QA is an optional gate; disabled/absent or any infra error => fail-OPEN (status stays "ready", never blocks publish).
	if qaCfg, qErr := o.agentsRepo.GetByName(ctx, "visual_qa"); qErr == nil && qaCfg.Enabled && result.LocalVideo916Path != "" {
		o.tracker.StartStep("visual_qa")
		frames := o.extractQAFrames(clipID, result.LocalVideo916Path, scenes)
		qaRes := o.visualQAAgent.Review(ctx, agent.VisualQAInput{
			Question: q.Question,
			Frames:   frames,
			Fast:     producer.PipelineFastEnabled(),
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

	// A hyperframes layout-inspector flag means visible overflow/clip — block publish
	// even if the vision QA gate passed or was disabled (fail-open QA can't catch it).
	if result.InspectFlagged && status == "ready" {
		status = "needs_review"
		log.Printf("clip %s: hyperframes inspect flagged layout — status=needs_review (publish blocked)", clipID)
	}

	// A silent/too-short voice track can't be seen by the still-frame vision QA —
	// route to needs_review when the audio gate is on.
	if result.AudioFlagged && producer.QAAudioCheckEnabled() && status == "ready" {
		status = "needs_review"
		log.Printf("clip %s: voice track silent/too short — status=needs_review (publish blocked)", clipID)
	}

	renderedStage := stageRendered
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{
		Status:          &status,
		Video916URL:     &result.Video916URL,
		ThumbnailURL:    &result.ThumbnailURL,
		VoiceScript:     &narration,
		AnswerScript:    &narration,
		ProductionStage: &renderedStage,
	})
	o.clipsRepo.ClearFailReason(ctx, clipID)
	if status == "ready" {
		log.Printf("Clip ready (hyperframes): %s", clipID)
	}
	return nil
}

func (o *Orchestrator) RetryAllFailed(ctx context.Context, maxRetries int, cooldownMinutes int) error {
	failed, err := o.clipsRepo.ListFailed(ctx, maxRetries, cooldownMinutes)
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

// resumeAtRender reports whether a failed clip has enough persisted state
// (scenes + metadata) to skip the LLM stages and resume at rendering.
func resumeAtRender(stage string) bool {
	return stage == stageContentReady || stage == stageRendered
}

func (o *Orchestrator) RetryClip(ctx context.Context, clip *models.Clip) error {
	if resumeAtRender(clip.ProductionStage) {
		return o.resumeHyperframesProduction(ctx, clip)
	}
	return o.retryFull(ctx, clip)
}

// resumeHyperframesProduction reuses the DB-persisted scenes (no LLM re-run) and
// resumes at the render stage. Falls back to a full rebuild if the scenes are
// missing (shouldn't happen for a content_ready clip).
func (o *Orchestrator) resumeHyperframesProduction(ctx context.Context, clip *models.Clip) error {
	scenes, err := o.scenesRepo.ListByClip(ctx, clip.ID)
	if err != nil || len(scenes) == 0 {
		log.Printf("resume %s: scenes unavailable (%v) — full rebuild", clip.ID, err)
		return o.retryFull(ctx, clip)
	}
	log.Printf("Resuming clip %s at render stage (%d scenes, stage=%s)", clip.ID, len(scenes), clip.ProductionStage)

	brandAliases, err := o.settingsRepo.GetBrandAliases(ctx)
	if err != nil {
		return o.failClip(ctx, clip.ID, fmt.Errorf("read brand aliases: %w", err))
	}
	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status})

	gen := scenesToGenerated(scenes)
	narration := buildVoiceScript(scenes, brandAliases)
	preset := producer.PresetByKey(clip.StylePreset)
	q := agent.GeneratedQuestion{
		Question:       clip.Question,
		QuestionerName: clip.QuestionerName,
		Category:       clip.Category,
	}
	return o.renderAndFinalize(ctx, clip.ID, q, gen, preset, narration)
}

// retryFull rebuilds a clip from scratch (script onward). Used when the clip has
// no resumable content checkpoint.
func (o *Orchestrator) retryFull(ctx context.Context, clip *models.Clip) error {
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

	// Retried clips keep their original visual identity. PresetByKey falls back to
	// editorial-bold (Presets[0]) if the stored key is empty (pre-flag clips have no stored preset).
	retryPreset := producer.PresetByKey(clip.StylePreset)
	return o.produceClipWithID(ctx, clip.ID, q, theme, retryPreset, scriptCfg, imageCfg, brandAliases, format, persona)
}

func scenesToGenerated(scenes []models.Scene) []agent.GeneratedScene {
	gen := make([]agent.GeneratedScene, len(scenes))
	for i, s := range scenes {
		var emphasis []string
		if len(s.EmphasisWords) > 0 {
			_ = json.Unmarshal(s.EmphasisWords, &emphasis)
		}
		gen[i] = agent.GeneratedScene{
			SceneNumber:     s.SceneNumber,
			SceneType:       s.SceneType,
			TextContent:     s.TextContent,
			VoiceText:       s.VoiceText,
			DurationSeconds: s.DurationSeconds,
			TextOverlays:    s.TextOverlays,
			LayoutVariant:   s.LayoutVariant,
			OnScreenText:    s.OnScreenText,
			EmphasisWords:   emphasis,
			Beat:            s.Beat,
			CaptionStyle:    s.CaptionStyle,
			ImagePrompt:     s.ImagePrompt,
			Layout:          s.Layout,
			Content:         s.Content,
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

func (o *Orchestrator) failClip(ctx context.Context, clipID string, err error) error {
	status := "failed"
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{Status: &status})
	o.clipsRepo.IncrementRetry(ctx, clipID, err.Error())
	return err
}

// nudgeRetry kicks the existing retry path shortly after a produce run that
// failed a clip, instead of leaving the clip to wait up to a full tick of the
// retry_failed schedule. Called ONCE at the edge of a produce run — never from
// failClip/retry paths, so a retry that fails again cannot re-arm a nudge (its
// second chance comes from the tick, whose 10-min cooldown gives a broken
// upstream time to recover). RetryAllFailed is gate-protected and idempotent —
// if another production is running it skips silently. 15s comfortably outlives
// the caller's deferred gate release; cooldown 0 is deliberate (speed first):
// worst case is ONE quick extra attempt before retry_count>=2 parks the clip.
func (o *Orchestrator) nudgeRetry() {
	go func() {
		time.Sleep(15 * time.Second)
		nctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if rerr := o.RetryAllFailed(nctx, 2, 0); rerr != nil {
			log.Printf("retry nudge failed: %v", rerr)
		}
	}()
}

// evenFrameTimestamps returns n timestamps evenly spread over a video of the
// given duration — the midpoint of each of n equal slices. Used for QA frame
// sampling so frames land on real content instead of collapsing to t=0 when
// per-scene duration estimates are missing (persisted duration_seconds is 0).
// duration<=0 or n<=0 yields nil.
func evenFrameTimestamps(duration float64, n int) []float64 {
	if duration <= 0 || n <= 0 {
		return nil
	}
	ts := make([]float64, n)
	slice := duration / float64(n)
	for i := 0; i < n; i++ {
		ts[i] = slice * (float64(i) + 0.5)
	}
	return ts
}

// qaSceneFrac positions each QA/auto-review sample this fraction into its scene —
// far enough past the entrance animation that content is visible, and far enough
// before the next scene that it never lands on a transition/crossfade frame.
const qaSceneFrac = 0.6

// autoReviewSceneFrac positions the auto-review sample at a DIFFERENT point in each
// scene than QA (qaSceneFrac), so the second-opinion judge inspects an independent
// frame and can overturn a QA false positive instead of re-confirming the same frame.
const autoReviewSceneFrac = 0.45

// sceneAwareTimestamps returns one timestamp per scene, each positioned `frac` into
// its own scene using the real per-scene durations, then rescaled so the estimated
// total maps onto the probed video duration. This keeps every sample inside its
// intended scene even when scene durations are unequal (unlike naive even slicing,
// which drifts onto transitions). Returns nil when durations sum to <= 0 so the
// caller can fall back. probedDur <= 0 means "don't rescale" (scale = 1).
func sceneAwareTimestamps(durations []float64, probedDur, frac float64) []float64 {
	var total float64
	for _, d := range durations {
		if d > 0 {
			total += d
		}
	}
	if total <= 0 {
		return nil
	}
	scale := 1.0
	if probedDur > 0 {
		scale = probedDur / total
	}
	ts := make([]float64, len(durations))
	var acc float64
	for i, d := range durations {
		if d < 0 {
			d = 0
		}
		ts[i] = (acc + d*frac) * scale
		acc += d
	}
	return ts
}

// qaFrameTimestamps returns one timestamp per scene for frame extraction, each
// positioned frac into its scene via real per-scene durations rescaled to
// the probed video length. Falls back to naive even slicing when per-scene
// durations are unavailable (all zero); if the probe ALSO fails it returns a
// short/nil slice, and callers must guard their index (fail-open on missing frames).
func (o *Orchestrator) qaFrameTimestamps(mp4Path string, durations []float64, frac float64) []float64 {
	probed, err := o.producer.FFmpeg().ProbeDurationSeconds(mp4Path)
	if err != nil || probed <= 0 {
		log.Printf("qa: probe duration unusable (err=%v, dur=%.3f); sampling from estimated scene durations", err, probed)
		probed = 0
	}
	if ts := sceneAwareTimestamps(durations, probed, frac); ts != nil {
		return ts
	}
	return evenFrameTimestamps(probed, len(durations))
}

// qaFrameTargets marks which scenes should have a QA frame sampled. Zero- (or
// negative-) duration scenes are skipped: they come from an empty VoiceText
// placeholder and their sampled timestamp lands exactly on a transition
// boundary, producing a blank frame that QA false-flags.
func qaFrameTargets(durs []float64) []bool {
	targets := make([]bool, len(durs))
	for i, d := range durs {
		targets[i] = d > 0
	}
	return targets
}

// extractQAFrames extracts one PNG frame per scene from the local MP4 and pairs
// it with the scene's text. A per-scene extraction failure is logged and that
// frame is dropped (Visual QA fails open on missing frames).
func (o *Orchestrator) extractQAFrames(clipID, mp4Path string, scenes []agent.GeneratedScene) []agent.QAFrame {
	durs := make([]float64, len(scenes))
	for i, s := range scenes {
		durs[i] = s.DurationSeconds
	}
	mids := o.qaFrameTimestamps(mp4Path, durs, qaSceneFrac)
	targets := qaFrameTargets(durs)
	frames := make([]agent.QAFrame, 0, len(scenes))
	for i, s := range scenes {
		if i >= len(mids) {
			break // sampler returned no usable timestamps (missing durations + probe fail) — fail-open
		}
		if !targets[i] {
			continue // zero-duration scene: sampling it would hit a transition boundary
		}
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

// auto_review tuning: approve threshold matches AutoReviewAgent's normalization
// default; retryCap bounds how many auto-triggered re-renders a clip gets before
// it's left for a human; batch caps work done per scheduler tick.
const (
	autoReviewApproveThreshold = 0.8
	autoReviewRetryCap         = 2
	autoReviewBatch            = 5
)

// downloadToTemp fetches url into a temp .mp4 and returns its path (caller removes it).
func downloadToTemp(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	f, err := os.CreateTemp("", "autoreview-*.mp4")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// autoReviewFrames downloads the clip's rendered 9:16 video and extracts one
// PNG per scene at its midpoint, paired with scene text. Returns nil on any
// failure (caller treats missing frames as a fail-closed hold).
func (o *Orchestrator) autoReviewFrames(ctx context.Context, videoURL string, scenes []models.Scene) []agent.QAFrame {
	mp4Path, err := downloadToTemp(ctx, videoURL)
	if err != nil {
		log.Printf("autoreview: download video failed: %v", err)
		return nil
	}
	defer os.Remove(mp4Path)

	durs := make([]float64, len(scenes))
	for i, s := range scenes {
		durs[i] = s.DurationSeconds
	}
	mids := o.qaFrameTimestamps(mp4Path, durs, autoReviewSceneFrac)
	frames := make([]agent.QAFrame, 0, len(scenes))
	for i, s := range scenes {
		if i >= len(mids) {
			break // sampler returned no usable timestamps (missing durations + probe fail) — fail-open
		}
		// Key the PNG to the already-unique mp4 temp filename so two clips'
		// scene-N frames never collide under concurrency.
		outPath := mp4Path + fmt.Sprintf(".scene%d.png", s.SceneNumber)
		if err := o.producer.FFmpeg().ExtractFrameAt(mp4Path, outPath, mids[i]); err != nil {
			log.Printf("autoreview: scene %d frame extract failed (skip): %v", s.SceneNumber, err)
			continue
		}
		png, err := os.ReadFile(outPath)
		os.Remove(outPath)
		if err != nil {
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

// AutoReviewPending runs the second-opinion judge over the needs_review queue.
// Disabled agent → no-op (behavior identical to manual review). Called by the
// scheduler's auto_review tick.
func (o *Orchestrator) AutoReviewPending(ctx context.Context) error {
	cfg, err := o.agentsRepo.GetByName(ctx, "auto_review")
	if err != nil || cfg == nil || !cfg.Enabled {
		return nil
	}
	clips, err := o.clipsRepo.ListNeedsReview(ctx, autoReviewRetryCap, autoReviewBatch)
	if err != nil {
		return fmt.Errorf("auto-review list: %w", err)
	}
	if len(clips) == 0 {
		return nil
	}

	// A retry decision below re-renders via RetryClip, which shares the single
	// Chrome/CPU render budget. Acquire the production gate for the whole batch —
	// same pattern as RetryAllFailed — so we never oversubscribe it with a
	// concurrent produce/retry tick. If busy, skip this tick (return nil, like
	// retryFailed) rather than surface a hard error.
	if !o.tracker.StartProduction(len(clips)) {
		log.Println("autoreview: production running, skipping tick")
		return nil
	}
	defer o.tracker.FinishProduction()

	ctx, cancel := context.WithCancel(ctx)
	o.tracker.SetCancelFunc(cancel)
	defer cancel()

	for i := range clips {
		if ctx.Err() != nil {
			log.Printf("autoreview: cancelled, stopping at clip %d/%d", i+1, len(clips))
			break
		}
		o.autoReviewOne(ctx, &clips[i], cfg)
	}
	return nil
}

// autoReviewOne judges one clip and applies the decision. Every path is logged;
// a per-clip failure never aborts the batch.
func (o *Orchestrator) autoReviewOne(ctx context.Context, clip *models.Clip, cfg *models.AgentConfig) {
	if clip.Video916URL == nil || *clip.Video916URL == "" {
		log.Printf("autoreview: clip %s has no video URL — holding", clip.ID)
		o.recordAndHold(ctx, clip, agent.AutoReviewResult{Decision: "hold", Reasons: []string{"no video url"}})
		return
	}
	scenes, err := o.scenesRepo.ListByClip(ctx, clip.ID)
	if err != nil || len(scenes) == 0 {
		log.Printf("autoreview: clip %s scenes unavailable — holding: %v", clip.ID, err)
		o.recordAndHold(ctx, clip, agent.AutoReviewResult{Decision: "hold", Reasons: []string{"scenes unavailable"}})
		return
	}

	var qaIssues []string
	if qa, err := o.visualQARepo.GetLatestByClipID(ctx, clip.ID); err != nil {
		log.Printf("autoreview: clip %s visual-QA lookup failed (continuing with no QA issues): %v", clip.ID, err)
	} else if qa != nil {
		qaIssues = flattenQAIssues(qa)
	}

	frames := o.autoReviewFrames(ctx, *clip.Video916URL, scenes)
	// Fail-closed: approve must only be reachable when EVERY scene frame is
	// present. A per-scene extract/read failure yields a partial slice — a
	// flagged scene could be silently missing — so hold for a human instead.
	if len(frames) < len(scenes) {
		reason := fmt.Sprintf("partial frame extraction — %d/%d scenes", len(frames), len(scenes))
		log.Printf("autoreview: clip %s %s — holding", clip.ID, reason)
		o.recordAndHold(ctx, clip, agent.AutoReviewResult{Decision: "hold", Reasons: []string{reason}})
		return
	}

	res := o.autoReviewAgent.Judge(ctx, agent.AutoReviewInput{
		Question: clip.Question,
		Frames:   frames,
		QAIssues: qaIssues,
	}, cfg, autoReviewApproveThreshold)

	reasons, _ := json.Marshal(res.Reasons)
	if err := o.autoReviewsRepo.Create(ctx, clip.ID, res.Decision, res.DefectType, res.Confidence, reasons); err != nil {
		log.Printf("autoreview: clip %s log write failed: %v", clip.ID, err)
	}

	switch res.Decision {
	case "approve":
		updated, err := o.clipsRepo.ApproveFromNeedsReview(ctx, clip.ID)
		if err != nil {
			log.Printf("autoreview: clip %s approve->ready failed: %v", clip.ID, err)
			return
		}
		if !updated {
			log.Printf("autoreview: clip %s no longer needs_review — skipping stale approve", clip.ID)
			return
		}
		log.Printf("autoreview: clip %s APPROVED (conf %.2f) — now ready", clip.ID, res.Confidence)
	case "retry":
		// Increment the retry budget only AFTER a successful re-render so a
		// skipped/failed render doesn't burn a retry. RetryClip relies on the
		// caller-held production gate (acquired in AutoReviewPending); it doesn't
		// acquire one itself, so ErrProductionRunning shouldn't occur here — but
		// be defensive and leave the clip unincremented for the next tick.
		if err := o.RetryClip(ctx, clip); err != nil {
			if errors.Is(err, ErrProductionRunning) {
				log.Printf("autoreview: clip %s retry skipped (production running) — leaving retry count for next tick", clip.ID)
				return
			}
			log.Printf("autoreview: clip %s retry render failed (retry count unchanged): %v", clip.ID, err)
			return
		}
		if err := o.clipsRepo.IncrementReviewRetry(ctx, clip.ID); err != nil {
			log.Printf("autoreview: clip %s retry counter bump failed: %v", clip.ID, err)
		}
		log.Printf("autoreview: clip %s RETRY re-render triggered (review_retry now %d)", clip.ID, clip.ReviewRetryCount+1)
	default: // hold
		o.recordHeld(ctx, clip)
	}
}

func (o *Orchestrator) recordAndHold(ctx context.Context, clip *models.Clip, res agent.AutoReviewResult) {
	reasons, _ := json.Marshal(res.Reasons)
	_ = o.autoReviewsRepo.Create(ctx, clip.ID, "hold", res.DefectType, res.Confidence, reasons)
	o.recordHeld(ctx, clip)
}

func (o *Orchestrator) recordHeld(ctx context.Context, clip *models.Clip) {
	if err := o.clipsRepo.SetAutoReviewHeld(ctx, clip.ID); err != nil {
		log.Printf("autoreview: clip %s set-held failed: %v", clip.ID, err)
	} else {
		log.Printf("autoreview: clip %s HELD for human review", clip.ID)
	}
}

// flattenQAIssues pulls the human-readable issue strings out of the stored
// visual_qa verdicts JSON (array of {scene_number, ok, issues}).
func flattenQAIssues(qa *models.VisualQA) []string {
	var verdicts []agent.SceneVerdict
	if err := json.Unmarshal(qa.Issues, &verdicts); err != nil {
		return nil
	}
	var out []string
	for _, v := range verdicts {
		if !v.OK {
			out = append(out, v.Issues...)
		}
	}
	return out
}
