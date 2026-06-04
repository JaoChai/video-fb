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
	ffmpeg       *FFmpegAssembler
	defaultVoice string
	workDir      string
	tracker      *progress.Tracker
	hf           *hyperframesDeps // nil = disabled, use FFmpeg
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

func NewProducer(pool *pgxpool.Pool, kie *KieClient, openRouter *OpenRouterClient, ffmpeg *FFmpegAssembler, voice, workDir string, tracker *progress.Tracker) *Producer {
	os.MkdirAll(workDir, 0755)
	return &Producer{pool: pool, kie: kie, openRouter: openRouter, ffmpeg: ffmpeg, defaultVoice: voice, workDir: workDir, tracker: tracker}
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
		if err := p.openRouter.GenerateImage(ctx, prompt.ImagePrompt169, "16:9", img169); err != nil {
			p.tracker.FailStep("images", err)
			return nil, fmt.Errorf("generate 16:9 image: %w", err)
		}
	} else {
		log.Printf("Skipping 16:9 image for %s (file exists)", clipID)
	}

	if !fileExists(img916) {
		if err := p.openRouter.GenerateImage(ctx, prompt.ImagePrompt916, "9:16", img916); err != nil {
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

	// Multi-scene path: generate per-scene TTS, then render both aspect ratios
	// with the composition_scenes agent. On any error, fall through to the
	// existing single-scene assembly below.
	if p.hf != nil && p.hf.multiScene && p.hf.scenesAgentCfg != nil &&
		(!fileExists(video169) || !fileExists(video916)) {
		log.Printf("Multi-scene assembly for %s (Hyperframes)", clipID)
		voice := p.getVoice(ctx)
		msVoicePath, bounds, msErr := p.synthScenesVoice(ctx, scenes, voice, clipDir)
		if msErr != nil {
			log.Printf("Multi-scene TTS failed for %s, falling back to single-scene: %v", clipID, msErr)
		} else {
			// Replace the voicePath for uploads later if multi-scene took over.
			voicePath = msVoicePath

			// Both videos share the SAME concatenated audio (msVoicePath), so we
			// ALWAYS (re)render BOTH together — even on a resume where one already
			// exists — to guarantee the two ratios never end up with mismatched
			// audio tracks (e.g. a leftover from a prior single-scene run with
			// different audio). No per-video fileExists skip here.
			if err := p.assembleMultiScene(ctx, clipID, clipDir, scenes, bounds, voicePath, "9:16", video916); err != nil {
				log.Printf("Multi-scene 9:16 failed for %s, falling back: %v", clipID, err)
				if err2 := p.ffmpeg.AssembleSingleImageVertical(img916, voicePath, video916); err2 != nil {
					return nil, fmt.Errorf("assemble 9:16 (fallback): %w", err2)
				}
			}

			if err := p.assembleMultiScene(ctx, clipID, clipDir, scenes, bounds, voicePath, "16:9", video169); err != nil {
				log.Printf("Multi-scene 16:9 failed for %s, falling back: %v", clipID, err)
				if err2 := p.ffmpeg.AssembleSingleImage(img169, voicePath, video169); err2 != nil {
					return nil, fmt.Errorf("assemble 16:9 (fallback): %w", err2)
				}
			}

			// Both videos handled — skip the single-scene block below.
			goto assemblyDone
		}
	}

	// Single-scene path (original behavior, always reachable as fallback).
	if !fileExists(video169) {
		log.Printf("Assembling 16:9 video for %s", clipID)
		if err := p.ffmpeg.AssembleSingleImage(img169, voicePath, video169); err != nil {
			return nil, fmt.Errorf("assemble 16:9: %w", err)
		}
	} else {
		log.Printf("Skipping 16:9 assembly for %s (file exists)", clipID)
	}

	if !fileExists(video916) {
		if p.hf != nil {
			log.Printf("Assembling 9:16 video for %s via Hyperframes", clipID)
			if err := p.assembleHyperframes916(ctx, clipID, clipDir, scenes, voicePath, video916); err != nil {
				log.Printf("Hyperframes 9:16 failed for %s, falling back to FFmpeg: %v", clipID, err)
				if err := p.ffmpeg.AssembleSingleImageVertical(img916, voicePath, video916); err != nil {
					return nil, fmt.Errorf("assemble 9:16 (fallback): %w", err)
				}
			}
		} else {
			log.Printf("Assembling 9:16 video for %s", clipID)
			if err := p.ffmpeg.AssembleSingleImageVertical(img916, voicePath, video916); err != nil {
				return nil, fmt.Errorf("assemble 9:16: %w", err)
			}
		}
	} else {
		log.Printf("Skipping 9:16 assembly for %s (file exists)", clipID)
	}

assemblyDone:

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

// assembleHyperframes916 renders the vertical video with the composition agent +
// Hyperframes (CSS background, captions, animated cards) instead of a static image.
func (p *Producer) assembleHyperframes916(ctx context.Context, clipID, clipDir string, scenes []agent.GeneratedScene, voicePath, outPath string) error {
	if len(scenes) == 0 {
		return fmt.Errorf("no scenes")
	}
	scene := scenes[0]

	var category, questioner string
	if err := p.pool.QueryRow(ctx,
		`SELECT category, questioner_name FROM clips WHERE id = $1`, clipID).Scan(&category, &questioner); err != nil {
		return fmt.Errorf("load clip metadata: %w", err)
	}

	// Captions are best-effort: if transcription fails the video still renders
	// (cards + title), just without phrase captions.
	segments, err := p.hf.transcriber.Transcribe(ctx, voicePath)
	if err != nil {
		log.Printf("Hyperframes: transcribe failed for %s (captions skipped): %v", clipID, err)
		segments = nil
	}
	duration := scene.DurationSeconds
	if n := len(segments); n > 0 && segments[n-1].End > duration {
		duration = segments[n-1].End
	}
	if duration <= 0 {
		duration = 60
	}

	var cfg models.AgentConfig
	if err := p.pool.QueryRow(ctx,
		`SELECT system_prompt, prompt_template, model, temperature, skills, insights
		 FROM agent_configs WHERE agent_name = 'composition'`).
		Scan(&cfg.SystemPrompt, &cfg.PromptTemplate, &cfg.Model, &cfg.Temperature, &cfg.Skills, &cfg.Insights); err != nil {
		return fmt.Errorf("load composition agent config: %w", err)
	}

	decision, err := p.hf.compAgent.Decide(ctx, agent.CompositionTemplateData{
		Question:        scene.TextContent,
		VoiceText:       scene.VoiceText,
		Category:        category,
		QuestionerName:  questioner,
		FormatName:      "qa",
		DurationSeconds: duration,
		SegmentsContext: segmentsContext(segments),
	}, &cfg)
	if err != nil {
		return fmt.Errorf("composition decide: %w", err)
	}

	// Generate clean background art (NO text) for the GPT image to sit behind the
	// Hyperframes-drawn text/captions. If it fails, fall back to the CSS background.
	bgMode, bgImg, bgPath := "css", "", ""
	bgFile := filepath.Join(clipDir, "bg-9x16.png")
	if !fileExists(bgFile) {
		if err := p.openRouter.GenerateImage(ctx, backgroundArtPrompt(category), "9:16", bgFile); err != nil {
			log.Printf("Hyperframes: bg art gen failed for %s, using CSS bg: %v", clipID, err)
		}
	}
	if fileExists(bgFile) {
		bgMode, bgImg, bgPath = "image", "assets/bg-9x16.png", bgFile
	}

	cards := make([]CardSpec, len(decision.Cards))
	for i, c := range decision.Cards {
		cards[i] = CardSpec{
			ID: fmt.Sprintf("card%d", i+1), Type: c.Type,
			StartSec: c.StartSec, EndSec: c.EndSec,
			Kicker: c.Kicker, Body: c.Body, StepNum: c.StepNum,
		}
	}
	params := CompositionParams{
		Title:           scene.TextContent,
		HighlightWords:  decision.HighlightWords,
		Kicker:          decision.Kicker,
		BrandName:       "ADS VANCE",
		CategoryLabel:   strings.ToUpper(category),
		QuestionerName:  questioner,
		LayoutVariant:   "dynamic_karaoke",
		AccentColor:     decision.AccentColor,
		SecondaryAccent: decision.SecondaryAccent,
		AnimationSpeed:  decision.AnimationSpeed,
		BackgroundMode:  bgMode,
		BackgroundImage: bgImg,
		VoiceSrc:        "assets/voice.wav",
		DurationSeconds: duration,
		Segments:        segments,
		Cards:           cards,
	}

	projectDir := filepath.Join(clipDir, "composition-916")
	os.RemoveAll(projectDir)
	if _, err := p.hf.builder.Build(params, clipID, projectDir, voicePath, bgPath); err != nil {
		return fmt.Errorf("build composition: %w", err)
	}
	if err := p.hf.renderer.Lint(ctx, projectDir); err != nil {
		return fmt.Errorf("composition lint: %w", err)
	}
	if err := p.hf.renderer.Render(ctx, projectDir, "output.mp4"); err != nil {
		return fmt.Errorf("composition render: %w", err)
	}
	if err := os.Rename(filepath.Join(projectDir, "output.mp4"), outPath); err != nil {
		return fmt.Errorf("move rendered video: %w", err)
	}

	// Record the design used, for the analyzer's future style-vs-performance learning.
	style := fmt.Sprintf("%s|%s|%s", params.LayoutVariant, decision.AccentColor, decision.AnimationSpeed)
	if _, err := p.pool.Exec(ctx, `UPDATE clips SET composition_style = $2 WHERE id = $1`, clipID, style); err != nil {
		log.Printf("Hyperframes: failed to record composition_style for %s: %v", clipID, err)
	}
	return nil
}

// aspectTag returns the file-safe ratio tag for a given aspect-ratio string.
func aspectTag(aspect string) string {
	if aspect == "16:9" {
		return "16x9"
	}
	return "9x16"
}

// assembleMultiScene renders a multi-scene Hyperframes video for one aspect ratio.
// It is modelled on assembleHyperframes916 but uses DecideScenes + BuildScenes
// (the multi-scene pipeline) so each script scene gets its own layout variant.
// On any error the caller should fall back to the single-scene path.
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

	// Transcription is best-effort: captions are nice but not required.
	segments, err := p.hf.transcriber.Transcribe(ctx, voicePath)
	if err != nil {
		log.Printf("assembleMultiScene: transcribe failed for %s (captions skipped): %v", clipID, err)
		segments = nil
	}

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
			if d.BgArtPrompt == "" {
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
					if genErr = p.openRouter.GenerateImage(egCtx, d.BgArtPrompt, aspect, bgFile); genErr == nil {
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

// backgroundArtPrompt builds an image prompt for a clean, text-free background.
// The Hyperframes layer draws all text on top, so the art must have none —
// just brand-colored abstract tech visuals with open negative space.
func backgroundArtPrompt(category string) string {
	motif := map[string]string{
		"pixel":    "glowing data tracking nodes and conversion flow lines",
		"payment":  "abstract billing dashboard glow and currency flow",
		"account":  "secure account shield and network grid",
		"campaign": "ascending performance charts and audience network",
	}[category]
	if motif == "" {
		motif = "abstract digital marketing dashboard glow"
	}
	return "Clean abstract background art for a 9:16 vertical video. " +
		"Dark navy gradient (#0a1428 to #16284a) with subtle orange (#ff6b2b) accents. " +
		"Motif: " + motif + ". Modern flat tech, cinematic depth, soft glow. " +
		"IMPORTANT: absolutely NO text, NO letters, NO numbers, NO words, NO UI labels, NO logos. " +
		"Keep the upper-center area calm and uncluttered (negative space) for text overlay added later."
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
