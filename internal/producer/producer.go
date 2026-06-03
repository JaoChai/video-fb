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

// hyperframesDeps bundles everything the Hyperframes 9:16 render path needs.
type hyperframesDeps struct {
	compAgent   *agent.CompositionAgent
	builder     *CompositionBuilder
	renderer    *HyperframesRenderer
	transcriber Transcriber
}

func NewProducer(pool *pgxpool.Pool, kie *KieClient, openRouter *OpenRouterClient, ffmpeg *FFmpegAssembler, voice, workDir string, tracker *progress.Tracker) *Producer {
	os.MkdirAll(workDir, 0755)
	return &Producer{pool: pool, kie: kie, openRouter: openRouter, ffmpeg: ffmpeg, defaultVoice: voice, workDir: workDir, tracker: tracker}
}

// EnableHyperframes turns on the Hyperframes 9:16 render path. When set, the
// vertical video is generated with animation/captions; if it fails at runtime,
// Produce falls back to the FFmpeg static-image render so a clip is never lost.
func (p *Producer) EnableHyperframes(compAgent *agent.CompositionAgent, builder *CompositionBuilder, renderer *HyperframesRenderer, tr Transcriber) {
	p.hf = &hyperframesDeps{compAgent: compAgent, builder: builder, renderer: renderer, transcriber: tr}
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
		BackgroundMode:  "css",
		VoiceSrc:        "assets/voice.wav",
		DurationSeconds: duration,
		Segments:        segments,
		Cards:           cards,
	}

	projectDir := filepath.Join(clipDir, "composition-916")
	os.RemoveAll(projectDir)
	if _, err := p.hf.builder.Build(params, projectDir, voicePath, ""); err != nil {
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
