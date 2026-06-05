package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/progress"
	"golang.org/x/sync/errgroup"
)

var ValidVoices = map[string]bool{
	"zephyr": true, "puck": true, "charon": true, "kore": true, "fenrir": true,
	"leda": true, "orus": true, "aoede": true, "callirrhoe": true, "autonoe": true,
	"enceladus": true, "iapetus": true, "umbriel": true, "algieba": true, "despina": true,
	"erinome": true, "algenib": true, "rasalgethi": true, "laomedeia": true, "achernar": true,
	"alnilam": true, "schedar": true, "gacrux": true, "pulcherrima": true, "achird": true,
	"zubenelgenubi": true, "vindemiatrix": true, "sadachbia": true, "sadaltager": true, "sulafat": true,
}

type Producer struct {
	pool         *pgxpool.Pool
	kie          *KieClient
	openRouter   *OpenRouterClient
	openAIImage  *OpenAIImageClient
	defaultVoice string
	workDir      string
	tracker      *progress.Tracker
	hf           *hyperframesDeps // must be set (multi-scene) — there is no fallback render path
}

// hyperframesDeps bundles everything the Hyperframes render paths need.
type hyperframesDeps struct {
	compAgent       *agent.CompositionAgent
	builder         *CompositionBuilder
	renderer        *HyperframesRenderer
	transcriber     Transcriber
	scenesAgentCfg  *models.AgentConfig // nil = multi-scene path disabled
	multiScene      bool                // true when HYPERFRAMES_MULTI_SCENE=true
}

func NewProducer(pool *pgxpool.Pool, kie *KieClient, openRouter *OpenRouterClient, openAIImage *OpenAIImageClient, voice, workDir string, tracker *progress.Tracker) *Producer {
	os.MkdirAll(workDir, 0755)
	return &Producer{pool: pool, kie: kie, openRouter: openRouter, openAIImage: openAIImage, defaultVoice: voice, workDir: workDir, tracker: tracker}
}

// EnableHyperframes turns on the Hyperframes render path. scenesAgentCfg and
// multiScene together gate the multi-scene path; if either is nil/false the
// existing single-scene path is used instead.
func (p *Producer) EnableHyperframes(
	compAgent *agent.CompositionAgent,
	builder *CompositionBuilder,
	renderer *HyperframesRenderer,
	tr Transcriber,
	scenesAgentCfg *models.AgentConfig,
	multiScene bool,
) {
	p.hf = &hyperframesDeps{
		compAgent:      compAgent,
		builder:        builder,
		renderer:       renderer,
		transcriber:    tr,
		scenesAgentCfg: scenesAgentCfg,
		multiScene:     multiScene,
	}
}

func (p *Producer) getVoice(ctx context.Context) string {
	var dbVoice string
	if err := p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'elevenlabs_voice'`).Scan(&dbVoice); err == nil && dbVoice != "" {
		if ValidVoices[strings.ToLower(dbVoice)] {
			return dbVoice
		}
		log.Printf("WARNING: invalid voice '%s' in DB, falling back to default", dbVoice)
	}
	if p.defaultVoice != "" && ValidVoices[strings.ToLower(p.defaultVoice)] {
		return p.defaultVoice
	}
	return "Achird"
}

type ProduceResult struct {
	Video169URL  string
	Video916URL  string
	ThumbnailURL string
}

// retryRender runs fn up to attempts times, returning nil on first success or
// the last error after exhausting attempts. Respects ctx cancellation.
func retryRender(ctx context.Context, attempts int, fn func() error) error {
	var last error
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if last = fn(); last == nil {
			return nil
		}
		log.Printf("render attempt %d/%d failed: %v", i+1, attempts, last)
	}
	return last
}

func (p *Producer) Produce(ctx context.Context, clipID string, scenes []agent.GeneratedScene, imagePrompts []agent.SceneImagePrompts, voiceScript string) (*ProduceResult, error) {
	clipDir := filepath.Join(p.workDir, clipID)
	os.MkdirAll(clipDir, 0755)

	if len(imagePrompts) == 0 {
		return nil, fmt.Errorf("no image prompts provided")
	}
	prompt := imagePrompts[0]

	voicePath := filepath.Join(clipDir, "voice.wav")
	p.tracker.StartStep("voice")
	if !fileExists(voicePath) {
		voice := p.getVoice(ctx)
		log.Printf("Generating voice for %s (voice: %s)", clipID, voice)
		if err := p.openRouter.GenerateVoice(ctx, voiceScript, voice, voicePath); err != nil {
			p.tracker.FailStep("voice", err)
			return nil, fmt.Errorf("generate voice: %w", err)
		}
	} else {
		log.Printf("Skipping voice for %s (file exists)", clipID)
	}
	p.tracker.CompleteStep("voice")

	p.tracker.StartStep("images")
	img169 := filepath.Join(clipDir, "question-16x9.png")
	img916 := filepath.Join(clipDir, "question-9x16.png")
	if !fileExists(img169) || !fileExists(img916) {
		os.Remove(img169)
		os.Remove(img916)
		os.Remove(filepath.Join(clipDir, "video-16x9.mp4"))
		os.Remove(filepath.Join(clipDir, "video-9x16.mp4"))
	}
	if !fileExists(img169) {
		if err := p.openAIImage.GenerateImage(ctx, prompt.ImagePrompt169, "16:9", img169); err != nil {
			p.tracker.FailStep("images", err)
			return nil, fmt.Errorf("generate 16:9 image: %w", err)
		}
	} else {
		log.Printf("Skipping 16:9 image for %s (file exists)", clipID)
	}

	if !fileExists(img916) {
		if err := p.openAIImage.GenerateImage(ctx, prompt.ImagePrompt916, "9:16", img916); err != nil {
			p.tracker.FailStep("images", err)
			return nil, fmt.Errorf("generate 9:16 image: %w", err)
		}
	} else {
		log.Printf("Skipping 9:16 image for %s (file exists)", clipID)
	}
	p.tracker.CompleteStep("images")

	p.tracker.StartStep("assembly")
	video169 := filepath.Join(clipDir, "video-16x9.mp4")
	video916 := filepath.Join(clipDir, "video-9x16.mp4")

	// Hyperframes multi-scene is the ONLY render path. If it isn't enabled there is
	// no fallback — fail loudly rather than silently producing a lesser video.
	if p.hf == nil || !p.hf.multiScene || p.hf.scenesAgentCfg == nil {
		err := fmt.Errorf("hyperframes multi-scene pipeline not enabled — cannot produce video")
		p.tracker.FailStep("assembly", err)
		return nil, err
	}

	log.Printf("Multi-scene assembly for %s (Hyperframes)", clipID)
	voice := p.getVoice(ctx)
	msVoicePath, bounds, msErr := p.synthScenesVoice(ctx, scenes, voice, clipDir)
	if msErr != nil {
		p.tracker.FailStep("assembly", msErr)
		return nil, fmt.Errorf("multi-scene TTS: %w", msErr)
	}
	voicePath = msVoicePath

	// Both ratios share the SAME concatenated audio, so always render both together.
	// Each render gets a retry; on exhaustion we fail the clip (no fallback) and the
	// tracker surfaces the error to the UI.
	if err := retryRender(ctx, 2, func() error {
		return p.assembleMultiScene(ctx, clipID, clipDir, scenes, bounds, voicePath, "9:16", video916)
	}); err != nil {
		p.tracker.FailStep("assembly", fmt.Errorf("9:16 render failed after retries: %w", err))
		return nil, fmt.Errorf("render 9:16 after retries: %w", err)
	}
	if err := retryRender(ctx, 2, func() error {
		return p.assembleMultiScene(ctx, clipID, clipDir, scenes, bounds, voicePath, "16:9", video169)
	}); err != nil {
		p.tracker.FailStep("assembly", fmt.Errorf("16:9 render failed after retries: %w", err))
		return nil, fmt.Errorf("render 16:9 after retries: %w", err)
	}

	thumbPath := img169
	p.tracker.CompleteStep("assembly")

	p.tracker.StartStep("upload")
	log.Printf("Uploading files to Kie AI for %s", clipID)
	uploadDir := "adsvance/" + clipID

	var video169URL, video916URL, thumbnailURL string
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		url, err := p.kie.UploadFile(gCtx, video169, uploadDir)
		if err != nil {
			return fmt.Errorf("upload 16:9 video: %w", err)
		}
		video169URL = url
		return nil
	})
	g.Go(func() error {
		url, err := p.kie.UploadFile(gCtx, video916, uploadDir)
		if err != nil {
			return fmt.Errorf("upload 9:16 video: %w", err)
		}
		video916URL = url
		return nil
	})
	g.Go(func() error {
		url, err := p.kie.UploadFile(gCtx, thumbPath, uploadDir)
		if err != nil {
			return fmt.Errorf("upload thumbnail: %w", err)
		}
		thumbnailURL = url
		return nil
	})
	g.Go(func() error {
		_, err := p.kie.UploadFile(gCtx, voicePath, uploadDir)
		if err != nil {
			return fmt.Errorf("upload voice: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		_, err := p.kie.UploadFile(gCtx, img916, uploadDir)
		if err != nil {
			return fmt.Errorf("upload 9:16 image: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		p.tracker.FailStep("upload", err)
		return nil, err
	}

	p.tracker.CompleteStep("upload")
	log.Printf("Files uploaded for %s — 16:9: %s, 9:16: %s, thumb: %s",
		clipID, video169URL, video916URL, thumbnailURL)

	return &ProduceResult{
		Video169URL:  video169URL,
		Video916URL:  video916URL,
		ThumbnailURL: thumbnailURL,
	}, nil
}

// aspectTag returns the file-safe ratio tag for a given aspect-ratio string.
func aspectTag(aspect string) string {
	if aspect == "16:9" {
		return "16x9"
	}
	return "9x16"
}

// assembleMultiScene renders a multi-scene Hyperframes video for one aspect ratio.
// It uses DecideScenes + BuildScenes (the multi-scene pipeline) so each script
// scene gets its own layout variant. This is the only render path; on error the
// caller retries and ultimately fails the clip (no fallback).
func (p *Producer) assembleMultiScene(ctx context.Context, clipID, clipDir string, scenes []agent.GeneratedScene, bounds []sceneBound, voicePath, aspect, outPath string) error {
	if len(scenes) == 0 {
		return fmt.Errorf("no scenes")
	}
	if len(bounds) == 0 {
		return fmt.Errorf("no scene bounds")
	}

	var category, questioner string
	if err := p.pool.QueryRow(ctx,
		`SELECT category, questioner_name FROM clips WHERE id = $1`, clipID).
		Scan(&category, &questioner); err != nil {
		return fmt.Errorf("load clip metadata: %w", err)
	}

	// Captions come straight from the GROUND-TRUTH scene VoiceText (the exact text
	// we sent to TTS), timed to each scene's audio window — NOT from re-transcribing
	// the audio with ASR, which mangled Thai vowels (สระ) and words. No Whisper round-trip.
	segments := captionSegmentsFromScenes(scenes, bounds)

	// Build the ScenesJSON that feeds the agent prompt. Include the fields the
	// composition_scenes agent needs: scene number, type, headline, voice text,
	// background hint.
	type sceneEntry struct {
		Number    int    `json:"number"`
		Type      string `json:"type"`
		Headline  string `json:"headline"`
		VoiceText string `json:"voice_text"`
		BgHint    string `json:"bg_hint"`
	}
	entries := make([]sceneEntry, len(scenes))
	for i, s := range scenes {
		entries[i] = sceneEntry{
			Number:    s.SceneNumber,
			Type:      s.SceneType,
			Headline:  s.TextContent,
			VoiceText: s.VoiceText,
			BgHint:    s.BgHint,
		}
	}
	scenesJSON, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal scenes for agent: %w", err)
	}

	// Total duration: last bound's End (or sum of scene durations as fallback).
	totalDur := bounds[len(bounds)-1].End
	if totalDur <= 0 {
		for _, s := range scenes {
			totalDur += s.DurationSeconds
		}
	}
	if totalDur <= 0 {
		totalDur = 60
	}

	decision, err := p.hf.compAgent.DecideScenes(ctx, agent.ScenesTemplateData{
		ScenesJSON:      string(scenesJSON),
		Category:        category,
		QuestionerName:  questioner,
		DurationSeconds: totalDur,
	}, p.hf.scenesAgentCfg)
	if err != nil {
		return fmt.Errorf("composition_scenes decide: %w", err)
	}

	// Generate per-scene background art in parallel. Failures are non-fatal:
	// the bgMode func returns "css" for scenes with no image.
	ratioTag := aspectTag(aspect)
	bgPaths := make(map[int]string)
	if len(decision.Scenes) > 0 {
		eg, egCtx := errgroup.WithContext(ctx)
		bgResults := make([]struct {
			sceneNum int
			path     string
		}, len(decision.Scenes))

		for i, d := range decision.Scenes {
			i, d := i, d
			if d.BgArtPrompt == "" || d.BgMode != "hero" {
				continue
			}
			bgFile := filepath.Join(clipDir, fmt.Sprintf("bg-scene%d-%s.png", d.SceneNumber, ratioTag))
			bgResults[i].sceneNum = d.SceneNumber
			eg.Go(func() error {
				if fileExists(bgFile) {
					bgResults[i].path = bgFile
					return nil
				}
				// Retry transient image-API failures (e.g. "no images in response"):
				// a missing bg forces the scene onto the CSS background, which renders
				// much slower and previously pushed the 16:9 render past its timeout.
				var genErr error
				for attempt := 1; attempt <= 3; attempt++ {
					if genErr = p.openAIImage.GenerateImage(egCtx, buildScenePrompt(d.BgArtPrompt, aspect), aspect, bgFile); genErr == nil {
						break
					}
					log.Printf("assembleMultiScene: bg gen attempt %d/3 failed for scene %d (%s): %v", attempt, d.SceneNumber, clipID, genErr)
				}
				if genErr != nil {
					return nil // non-fatal — scene downgrades to CSS background
				}
				if fileExists(bgFile) {
					bgResults[i].path = bgFile
				}
				return nil
			})
		}
		if err := eg.Wait(); err != nil {
			return fmt.Errorf("bg image errgroup: %w", err)
		}
		for _, r := range bgResults {
			if r.path != "" {
				bgPaths[r.sceneNum] = r.path
			}
		}
	}

	bgModeFunc := func(sceneNumber int) string {
		if _, ok := bgPaths[sceneNumber]; ok {
			return "image"
		}
		return "css"
	}

	specs := buildSceneSpecs(decision.Scenes, bounds, bgModeFunc)
	if len(specs) == 0 {
		return fmt.Errorf("buildSceneSpecs returned empty (designs=%d bounds=%d)", len(decision.Scenes), len(bounds))
	}

	params := ScenesParams{
		AspectRatio:     aspect,
		BrandName:       "ADS VANCE",
		CategoryLabel:   strings.ToUpper(category),
		QuestionerName:  questioner,
		Kicker:          decision.Kicker,
		DurationSeconds: totalDur,
		Scenes:          specs,
		Segments:        segments,
		IntroMascot:     "assets/mascot/rocket.png",
		OutroMascot:     "assets/mascot/wave.png",
		CTAText:         "กดติดตาม ADS VANCE ไม่พลาดเรื่องแอด",
	}

	projectDir := filepath.Join(clipDir, "scenes-"+ratioTag)
	os.RemoveAll(projectDir)
	if _, err := p.hf.builder.BuildScenes(params, clipID, projectDir, voicePath, bgPaths); err != nil {
		return fmt.Errorf("build scenes: %w", err)
	}
	if err := p.hf.renderer.Lint(ctx, projectDir); err != nil {
		return fmt.Errorf("multi-scene lint: %w", err)
	}
	// Inspect is a gate: if the layout has clipped/overlapping content, fail now
	// so the caller can fall back to a simpler render rather than producing junk.
	if err := p.hf.renderer.Inspect(ctx, projectDir); err != nil {
		return fmt.Errorf("multi-scene inspect: %w", err)
	}
	if err := p.hf.renderer.Render(ctx, projectDir, "output.mp4"); err != nil {
		return fmt.Errorf("multi-scene render: %w", err)
	}
	if err := os.Rename(filepath.Join(projectDir, "output.mp4"), outPath); err != nil {
		return fmt.Errorf("move multi-scene video: %w", err)
	}
	return nil
}

func segmentsContext(segs []TranscriptSegment) string {
	var b strings.Builder
	for _, s := range segs {
		fmt.Fprintf(&b, "[%.1f-%.1f] %s\n", s.Start, s.End, s.Text)
	}
	return b.String()
}

func fileExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.Size() > 0
}
