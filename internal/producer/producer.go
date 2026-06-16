package producer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	openRouter   *OpenRouterClient
	ffmpeg       *FFmpegAssembler
	defaultVoice string
	workDir      string
	tracker      *progress.Tracker
	hf           *hyperframesDeps // nil until EnableHyperframes; only the multi-scene path uses it
}

func NewProducer(pool *pgxpool.Pool, kie *KieClient, openRouter *OpenRouterClient, ffmpeg *FFmpegAssembler, voice, workDir string, tracker *progress.Tracker) *Producer {
	os.MkdirAll(workDir, 0755)
	return &Producer{pool: pool, kie: kie, openRouter: openRouter, ffmpeg: ffmpeg, defaultVoice: voice, workDir: workDir, tracker: tracker}
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

// AssembleHyperframes916 turns SceneAgent scenes into a 9:16 multi-scene MP4 via
// the hyperframes engine and returns the LOCAL output.mp4 path. Steps: per-scene
// TTS (measured) → per-scene gpt-image-2 backgrounds (kie.ai; missing/failed →
// css) → GeneratedScene→SceneSpec → fill the multi-scene template → render.
// Upload / thumbnail / clip-status are the caller's job (orchestrator).
// Requires EnableHyperframes to have been called.
func (p *Producer) AssembleHyperframes916(ctx context.Context, clipID string, scenes []agent.GeneratedScene) (string, error) {
	if p.hf == nil {
		return "", fmt.Errorf("hyperframes not enabled (call EnableHyperframes)")
	}
	if len(scenes) == 0 {
		return "", fmt.Errorf("no scenes")
	}

	clipDir := filepath.Join(p.workDir, clipID)
	if err := os.MkdirAll(clipDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir clipDir: %w", err)
	}
	voice := p.getVoice(ctx)

	// 1) per-scene TTS → combined voice.wav + measured [start,end) bounds.
	voicePath, bounds, err := p.synthScenesVoice(ctx, scenes, voice, clipDir)
	if err != nil {
		return "", fmt.Errorf("synth scenes voice: %w", err)
	}

	// 2) per-scene gpt-image-2 backgrounds (kie.ai). A missing/failed image is
	//    non-fatal — BuildScenes downgrades that scene to a css background.
	//    Circuit breaker: once one image fails (kie degraded), skip gen for ALL
	//    remaining scenes (css) instead of grinding through retries one by one —
	//    turns a "kie is down" run from ~hours into ~minutes.
	bgPaths := map[int]string{}
	imageDegraded := false
	for _, s := range scenes {
		if strings.TrimSpace(s.ImagePrompt) == "" {
			continue
		}
		bgFile := filepath.Join(clipDir, fmt.Sprintf("bg-scene%d.png", s.SceneNumber))
		if !fileExists(bgFile) && !imageDegraded {
			prompt := buildScenePrompt(s.ImagePrompt, "9:16")
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

	// 3) map scenes → SceneSpec, build captions, assemble ScenesParams.
	specs := buildSceneSpecs(scenes, bounds)
	if len(specs) == 0 {
		return "", fmt.Errorf("buildSceneSpecs returned empty (scenes=%d bounds=%d)", len(scenes), len(bounds))
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
	}

	// 4) build the project dir and render the MP4.
	projectDir := filepath.Join(clipDir, "composition-916")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir projectDir: %w", err)
	}
	if _, err := p.hf.builder.BuildScenes(params, clipID, projectDir, voicePath, bgPaths); err != nil {
		return "", fmt.Errorf("build scenes: %w", err)
	}
	if err := p.hf.renderer.Inspect(ctx, projectDir); err != nil {
		log.Printf("hyperframes inspect flagged layout issues for clip %s (rendering anyway): %v", clipID, err)
	}
	if err := p.hf.renderer.Render(ctx, projectDir, "output.mp4"); err != nil {
		return "", fmt.Errorf("render: %w", err)
	}
	return filepath.Join(projectDir, "output.mp4"), nil
}

// ProduceHyperframes916 assembles a 9:16 multi-scene MP4 from scenes (per-scene
// TTS + gpt-image-2 + render via AssembleHyperframes916), extracts a thumbnail
// from the first frame, uploads both to kie.ai, and returns their URLs. It is the
// multi-scene counterpart to the static Produce. Requires EnableHyperframes and a
// non-nil tracker (the production path always provides one).
func (p *Producer) ProduceHyperframes916(ctx context.Context, clipID string, scenes []agent.GeneratedScene) (*ProduceResult, error) {
	p.tracker.StartStep("assembly")
	mp4Path, err := p.AssembleHyperframes916(ctx, clipID, scenes)
	if err != nil {
		p.tracker.FailStep("assembly", err)
		return nil, fmt.Errorf("assemble hyperframes: %w", err)
	}
	p.tracker.CompleteStep("assembly")

	p.tracker.StartStep("upload")
	thumbPath := filepath.Join(filepath.Dir(mp4Path), "thumbnail.png")
	if err := p.ffmpeg.ExtractThumbnail(mp4Path, thumbPath); err != nil {
		p.tracker.FailStep("upload", err)
		return nil, fmt.Errorf("extract thumbnail: %w", err)
	}

	uploadDir := "adsvance/" + clipID
	video916URL, err := p.kie.UploadFile(ctx, mp4Path, uploadDir)
	if err != nil {
		p.tracker.FailStep("upload", err)
		return nil, fmt.Errorf("upload video: %w", err)
	}
	thumbnailURL, err := p.kie.UploadFile(ctx, thumbPath, uploadDir)
	if err != nil {
		p.tracker.FailStep("upload", err)
		return nil, fmt.Errorf("upload thumbnail: %w", err)
	}
	p.tracker.CompleteStep("upload")

	return &ProduceResult{Video916URL: video916URL, ThumbnailURL: thumbnailURL, LocalVideo916Path: mp4Path}, nil
}
