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
}

func NewProducer(pool *pgxpool.Pool, kie *KieClient, openRouter *OpenRouterClient, ffmpeg *FFmpegAssembler, voice, workDir string, tracker *progress.Tracker) *Producer {
	os.MkdirAll(workDir, 0755)
	return &Producer{pool: pool, kie: kie, openRouter: openRouter, ffmpeg: ffmpeg, defaultVoice: voice, workDir: workDir, tracker: tracker}
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
