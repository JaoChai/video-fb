package producer

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/agent"
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
	r2           *R2Client
	openRouter   *OpenRouterClient
	ffmpeg       *FFmpegAssembler
	defaultVoice string
	workDir      string
	tracker      *progress.Tracker
	hf           *hyperframesDeps // nil until EnableHyperframes; only the multi-scene path uses it
}

func NewProducer(pool *pgxpool.Pool, kie *KieClient, r2 *R2Client, openRouter *OpenRouterClient, ffmpeg *FFmpegAssembler, voice, workDir string, tracker *progress.Tracker) *Producer {
	os.MkdirAll(workDir, 0755)
	return &Producer{pool: pool, kie: kie, r2: r2, openRouter: openRouter, ffmpeg: ffmpeg, defaultVoice: voice, workDir: workDir, tracker: tracker}
}

// KieCredits returns the kie.ai account credit balance — a cheap pre-flight the
// orchestrator runs before a production run.
func (p *Producer) KieCredits(ctx context.Context) (int, error) {
	return p.kie.GetCredits(ctx)
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
	Video169URL       string
	Video916URL       string
	ThumbnailURL      string
	LocalVideo916Path string
	// SceneDurations is each scene's real rendered duration (from voice bounds),
	// in scene order — persisted as the accurate scenes.duration_seconds. Empty
	// for the static Produce path (only the hyperframes path measures bounds).
	SceneDurations []float64
	// InspectFlagged is true when the hyperframes layout inspector reported an
	// overflow/clip issue for this clip. Surfaced so the orchestrator can route
	// the clip to needs_review instead of publishing a visibly-broken layout.
	InspectFlagged bool
	// AudioFlagged is true when the rendered voice track is silent/too short
	// (QA audio check). The orchestrator routes such clips to needs_review when
	// QA_AUDIO_CHECK_ENABLED is on.
	AudioFlagged bool
	// RenderFlagged is true when the render emitted browser errors (a silently
	// frozen render). The orchestrator retries once, then routes to needs_review.
	RenderFlagged bool
}

// FFmpeg exposes the assembler so callers (Visual QA) can extract frames from a
// rendered MP4 without re-rendering.
func (p *Producer) FFmpeg() *FFmpegAssembler { return p.ffmpeg }

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
	if !fileExists(video169) {
		log.Printf("Assembling 16:9 video for %s", clipID)
		if err := p.ffmpeg.AssembleSingleImage(img169, voicePath, video169); err != nil {
			return nil, fmt.Errorf("assemble 16:9: %w", err)
		}
	} else {
		log.Printf("Skipping 16:9 assembly for %s (file exists)", clipID)
	}

	video916 := filepath.Join(clipDir, "video-9x16.mp4")
	if !fileExists(video916) {
		log.Printf("Assembling 9:16 video for %s", clipID)
		if err := p.ffmpeg.AssembleSingleImageVertical(img916, voicePath, video916); err != nil {
			return nil, fmt.Errorf("assemble 9:16: %w", err)
		}
	} else {
		log.Printf("Skipping 9:16 assembly for %s (file exists)", clipID)
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

func fileExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.Size() > 0
}

// buildTransitionCues places one SFX per scene boundary (scene 2..N), firing
// 0.2s before the incoming scene's start so the whoosh leads the visual cut.
// names[i] is used for boundary i; extra names/specs are ignored. Returns nil
// when there are <2 scenes or no names.
func buildTransitionCues(specs []SceneSpec, names []string) []TransitionCue {
	if len(specs) < 2 || len(names) == 0 {
		return nil
	}
	var cues []TransitionCue
	for i := 1; i < len(specs); i++ {
		if i-1 >= len(names) {
			break
		}
		at := specs[i].StartSec - 0.2
		if at < 0 {
			at = 0
		}
		cues = append(cues, TransitionCue{Name: names[i-1], AtSec: at})
	}
	return cues
}

func (p *Producer) lastAmbient(ctx context.Context) string {
	var v string
	_ = p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'last_ambient'`).Scan(&v)
	return v
}

func (p *Producer) saveLastAmbient(ctx context.Context, name string) {
	if _, err := p.pool.Exec(ctx,
		`INSERT INTO settings (key, value) VALUES ('last_ambient', $1)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, name); err != nil {
		log.Printf("saveLastAmbient: %v", err)
	}
}

// hyperframesDeps bundles the multi-scene render engine; set via EnableHyperframes.
type hyperframesDeps struct {
	builder  *CompositionBuilder
	renderer *HyperframesRenderer
}

// EnableHyperframes wires the multi-scene render engine into the producer.
// fontsDir is an ABSOLUTE path to the Sarabun .ttf files (the repo copy lives at
// internal/producer/assets/fonts); a caller outside this package — e.g. the
// orchestrator wiring in main.go — must resolve it absolutely, since the render
// runs from a per-clip working dir, not the package dir.
// Additive: the static-image Produce path does not use p.hf.
func (p *Producer) EnableHyperframes(fontsDir string) {
	p.hf = &hyperframesDeps{
		builder:  NewCompositionBuilder(fontsDir),
		renderer: NewHyperframesRenderer(),
	}
}

// assembleOutput carries the render products and post-render health signals from
// AssembleHyperframes916 up to ProduceHyperframes916. It is nil on any error
// return. sceneDurations is each scene's real rendered duration (from measured
// voice bounds), in scene order — persisted by the caller as the accurate
// scenes.duration_seconds (the scene agent never emits durations).
type assembleOutput struct {
	mp4Path        string
	sceneDurations []float64
	inspectFlagged bool
	audioFlagged   bool
	renderFlagged  bool // populated by the render browser-error gate
}

// generateSceneImagesParallel is the PIPELINE_FAST_ENABLED variant of the
// per-scene background gen: up to 4 concurrent kie calls instead of one at a
// time (~5 min → ~1 min for 10 scenes). Two-stage fallback chain per scene:
// gpt-image-2 → nano-banana-2-lite → css, each stage with its own circuit
// breaker — the first failure of a stage stops LATER scenes from attempting
// that stage (they start at the next one), but images already generated are
// still used. Populates bgPaths (sceneNumber → file); errors are non-fatal by
// design (BuildScenes renders a css background for any scene without a file).
func (p *Producer) generateSceneImagesParallel(ctx context.Context, scenes []agent.GeneratedScene, preset StylePreset, clipID, clipDir string, bgPaths map[int]string) {
	var mu sync.Mutex
	var primaryDown, fallbackDown atomic.Bool
	var g errgroup.Group
	g.SetLimit(4)
	for _, s := range scenes {
		if strings.TrimSpace(s.ImagePrompt) == "" {
			continue
		}
		s := s
		bgFile := filepath.Join(clipDir, fmt.Sprintf("bg-scene%d.png", s.SceneNumber))
		g.Go(func() error {
			if !fileExists(bgFile) {
				prompt := buildScenePrompt(s.ImagePrompt, "9:16", preset, clipID)
				generated := false
				if !primaryDown.Load() {
					if genErr := p.kie.GenerateImage(ctx, prompt, "9:16", bgFile); genErr != nil {
						log.Printf("AssembleHyperframes916: scene %d primary image gen failed — unstarted scenes go straight to fallback model: %v", s.SceneNumber, genErr)
						primaryDown.Store(true)
					} else {
						generated = true
					}
				}
				if !generated && !fallbackDown.Load() {
					if genErr := p.kie.GenerateImageFallback(ctx, prompt, "9:16", bgFile); genErr != nil {
						log.Printf("AssembleHyperframes916: scene %d fallback image gen failed — unstarted scenes use css: %v", s.SceneNumber, genErr)
						fallbackDown.Store(true)
					}
				}
			}
			if fileExists(bgFile) {
				mu.Lock()
				bgPaths[s.SceneNumber] = bgFile
				mu.Unlock()
			}
			return nil
		})
	}
	g.Wait() // goroutines never return errors — Wait is just the barrier
}

// AssembleHyperframes916 turns SceneAgent scenes into a 9:16 multi-scene MP4 via
// the hyperframes engine and returns an assembleOutput (local output.mp4 path,
// per-scene durations, and the inspect/audio/render health flags). Steps:
// per-scene TTS (measured) → per-scene gpt-image-2 backgrounds (kie.ai;
// missing/failed → css) → GeneratedScene→SceneSpec → fill the multi-scene
// template → render. Upload / thumbnail / clip-status are the caller's job
// (orchestrator). Requires EnableHyperframes to have been called.
func (p *Producer) AssembleHyperframes916(ctx context.Context, clipID string, scenes []agent.GeneratedScene, preset StylePreset) (*assembleOutput, error) {
	if p.hf == nil {
		return nil, fmt.Errorf("hyperframes not enabled (call EnableHyperframes)")
	}
	if len(scenes) == 0 {
		return nil, fmt.Errorf("no scenes")
	}

	clipDir := filepath.Join(p.workDir, clipID)
	if err := os.MkdirAll(clipDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir clipDir: %w", err)
	}
	voice := p.getVoice(ctx)

	// 1) per-scene TTS → combined voice.wav + measured [start,end) bounds.
	voicePath, bounds, err := p.synthScenesVoice(ctx, scenes, voice, clipDir)
	if err != nil {
		return nil, fmt.Errorf("synth scenes voice: %w", err)
	}

	// 2) per-scene gpt-image-2 backgrounds (kie.ai). A missing/failed image is
	//    non-fatal — BuildScenes downgrades that scene to a css background.
	//    Circuit breaker: once one image fails (kie degraded), skip gen for ALL
	//    remaining scenes (css) instead of grinding through retries one by one —
	//    turns a "kie is down" run from ~hours into ~minutes.
	bgPaths := map[int]string{}
	if PipelineFastEnabled() {
		p.generateSceneImagesParallel(ctx, scenes, preset, clipID, clipDir, bgPaths)
	} else {
		imageDegraded := false
		for _, s := range scenes {
			if strings.TrimSpace(s.ImagePrompt) == "" {
				continue
			}
			bgFile := filepath.Join(clipDir, fmt.Sprintf("bg-scene%d.png", s.SceneNumber))
			if !fileExists(bgFile) && !imageDegraded {
				prompt := buildScenePrompt(s.ImagePrompt, "9:16", preset, clipID)
				if genErr := p.kie.GenerateImage(ctx, prompt, "9:16", bgFile); genErr != nil {
					log.Printf("AssembleHyperframes916: scene %d image gen failed — tripping circuit breaker, remaining scenes use css: %v", s.SceneNumber, genErr)
					imageDegraded = true
					continue
				}
			}
			if fileExists(bgFile) {
				bgPaths[s.SceneNumber] = bgFile
			}
		}
	}

	// 3) map scenes → SceneSpec, build captions, assemble ScenesParams.
	specs := buildSceneSpecs(scenes, bounds)
	if len(specs) == 0 {
		return nil, fmt.Errorf("buildSceneSpecs returned empty (scenes=%d bounds=%d)", len(scenes), len(bounds))
	}
	segments := captionSegmentsFromScenes(scenes, bounds)
	total := 0.0
	if len(bounds) > 0 {
		total = bounds[len(bounds)-1].End
	}
	params := ScenesParams{
		AspectRatio:     "9:16",
		BrandName:       BrandName,
		CTAText:         BrandCTA,
		VoiceSrc:        "assets/voice.wav",
		DurationSeconds: total,
		Scenes:          specs,
		Segments:        segments,
		Palette:         preset.Palette,
		BrandCSS:        preset.BrandCSS(),
		ThemeKey:        preset.Key,
		Motion:          preset.Motion,
	}

	params.MotionV2 = SceneMotionV2Enabled()
	params.Cover = CoverSceneEnabled()

	if AudioMotionEnabled() {
		params.AudioMotion = true

		// Ambient bed: avoid-last pick, extracted from embed, looped to clip length.
		lastAmb := p.lastAmbient(ctx)
		ambName := PickAmbient(lastAmb, rand.Intn)
		if ambName != "" && total > 0 {
			rawAmb := filepath.Join(clipDir, "ambient-src.mp3")
			if data, rerr := audioAssetsFS.ReadFile(ambientDir + "/" + ambName); rerr == nil {
				if werr := os.WriteFile(rawAmb, data, 0o644); werr == nil {
					preparedAmb := filepath.Join(clipDir, "ambient.mp3")
					if berr := p.ffmpeg.BuildAmbientBed(rawAmb, preparedAmb, total); berr == nil {
						params.AmbientLocalPath = preparedAmb
						p.saveLastAmbient(ctx, ambName)
					} else {
						log.Printf("AssembleHyperframes916: ambient bed prep failed (continuing without): %v", berr)
					}
				} else {
					log.Printf("AssembleHyperframes916: ambient write failed (continuing without): %v", werr)
				}
			} else {
				log.Printf("AssembleHyperframes916: ambient embed read failed (continuing without): %v", rerr)
			}
		}

		// Transition SFX: one per scene boundary.
		sfxNames := PickTransitionSFX(len(specs)-1, rand.Intn)
		params.TransitionCues = buildTransitionCues(specs, sfxNames)
	}

	// 4) build the project dir and render the MP4.
	projectDir := filepath.Join(clipDir, "composition-916")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir projectDir: %w", err)
	}
	if _, err := p.hf.builder.BuildScenes(params, clipID, projectDir, voicePath, bgPaths); err != nil {
		return nil, fmt.Errorf("build scenes: %w", err)
	}
	inspectFlagged := false
	if err := p.hf.renderer.Inspect(ctx, projectDir); err != nil {
		inspectFlagged = true
		log.Printf("hyperframes inspect flagged layout issues for clip %s: %v", clipID, err)
	}
	renderIssues, err := p.hf.renderer.Render(ctx, projectDir, "output.mp4")
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	audioFlagged := probeVoiceSilent(voicePath)
	return &assembleOutput{
		mp4Path:        filepath.Join(projectDir, "output.mp4"),
		sceneDurations: boundsToDurations(bounds),
		inspectFlagged: inspectFlagged,
		audioFlagged:   audioFlagged,
		renderFlagged:  len(renderIssues) > 0,
	}, nil
}

// uploadWithFallback runs primary; on error it logs and runs fallback. Used so a
// transient R2 outage degrades to kie's temporary URL instead of failing the clip.
func uploadWithFallback(primary, fallback func() (string, error)) (string, error) {
	url, err := primary()
	if err == nil {
		return url, nil
	}
	log.Printf("uploadPersistent: primary (R2) upload failed, falling back to kie: %v", err)
	return fallback()
}

// uploadPersistent stores a rendered file at a durable URL. It prefers R2 (URLs
// never expire); if R2 is disabled/unconfigured OR the R2 upload errors at
// runtime it falls back to kie.ai's temporary upload so the pipeline keeps
// working. r2Key is the full object key; kieDir is the legacy kie uploadPath.
func (p *Producer) uploadPersistent(ctx context.Context, localPath, r2Key, kieDir string) (string, error) {
	if p.r2 != nil && p.r2.Enabled(ctx) {
		return uploadWithFallback(
			func() (string, error) { return p.r2.Upload(ctx, localPath, r2Key, "") },
			func() (string, error) { return p.kie.UploadFile(ctx, localPath, kieDir) },
		)
	}
	return p.kie.UploadFile(ctx, localPath, kieDir)
}

// ProduceHyperframes916 assembles a 9:16 multi-scene MP4 from scenes (per-scene
// TTS + gpt-image-2 + render via AssembleHyperframes916), extracts a thumbnail
// from the first frame, uploads both to kie.ai, and returns their URLs. It is the
// multi-scene counterpart to the static Produce. Requires EnableHyperframes and a
// non-nil tracker (the production path always provides one).
func (p *Producer) ProduceHyperframes916(ctx context.Context, clipID string, scenes []agent.GeneratedScene, preset StylePreset) (*ProduceResult, error) {
	p.tracker.StartStep("assembly")
	out, err := p.AssembleHyperframes916(ctx, clipID, scenes, preset)
	if err != nil {
		p.tracker.FailStep("assembly", err)
		return nil, fmt.Errorf("assemble hyperframes: %w", err)
	}
	mp4Path := out.mp4Path
	p.tracker.CompleteStep("assembly")

	p.tracker.StartStep("upload")
	thumbPath := filepath.Join(filepath.Dir(mp4Path), "thumbnail.png")
	if err := p.ffmpeg.ExtractThumbnail(mp4Path, thumbPath); err != nil {
		p.tracker.FailStep("upload", err)
		return nil, fmt.Errorf("extract thumbnail: %w", err)
	}

	uploadDir := "adsvance/" + clipID
	video916URL, err := p.uploadPersistent(ctx, mp4Path, "clips/"+clipID+"/video-916.mp4", uploadDir)
	if err != nil {
		p.tracker.FailStep("upload", err)
		return nil, fmt.Errorf("upload video: %w", err)
	}
	thumbnailURL, err := p.uploadPersistent(ctx, thumbPath, "clips/"+clipID+"/thumbnail.png", uploadDir)
	if err != nil {
		p.tracker.FailStep("upload", err)
		return nil, fmt.Errorf("upload thumbnail: %w", err)
	}
	p.tracker.CompleteStep("upload")

	return &ProduceResult{
		Video916URL:       video916URL,
		ThumbnailURL:      thumbnailURL,
		LocalVideo916Path: mp4Path,
		SceneDurations:    out.sceneDurations,
		InspectFlagged:    out.inspectFlagged,
		AudioFlagged:      out.audioFlagged,
		RenderFlagged:     out.renderFlagged,
	}, nil
}
