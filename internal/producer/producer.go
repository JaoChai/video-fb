package producer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/progress"
	"golang.org/x/sync/errgroup"
)

var validVoices = map[string]bool{
	"rachel": true, "aria": true, "roger": true, "sarah": true, "laura": true,
	"charlie": true, "george": true, "callum": true, "river": true, "liam": true,
	"charlotte": true, "alice": true, "matilda": true, "will": true, "jessica": true,
	"eric": true, "chris": true, "brian": true, "daniel": true, "lily": true,
	"bill": true, "adam": true,
}

type Producer struct {
	kie          *KieClient
	ffmpeg       *FFmpegAssembler
	defaultVoice string
	workDir      string
	tracker      *progress.Tracker
}

func NewProducer(kie *KieClient, ffmpeg *FFmpegAssembler, voice, workDir string, tracker *progress.Tracker) *Producer {
	os.MkdirAll(workDir, 0755)
	return &Producer{kie: kie, ffmpeg: ffmpeg, defaultVoice: voice, workDir: workDir, tracker: tracker}
}

func (p *Producer) getVoice(ctx context.Context) string {
	var dbVoice string
	if err := p.kie.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'elevenlabs_voice'`).Scan(&dbVoice); err == nil && dbVoice != "" {
		if validVoices[strings.ToLower(dbVoice)] {
			return dbVoice
		}
		log.Printf("WARNING: invalid voice '%s' in DB, falling back to default", dbVoice)
	}
	if p.defaultVoice != "" && validVoices[strings.ToLower(p.defaultVoice)] {
		return p.defaultVoice
	}
	return "Daniel"
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

	p.tracker.StartStep("voice")
	voice := p.getVoice(ctx)
	log.Printf("Generating voice for %s (voice: %s)", clipID, voice)
	voicePath := filepath.Join(clipDir, "voice.mp3")
	if err := p.kie.GenerateVoice(ctx, voiceScript, voice, voicePath); err != nil {
		p.tracker.FailStep("voice", err)
		return nil, fmt.Errorf("generate voice: %w", err)
	}
	p.tracker.CompleteStep("voice")

	p.tracker.StartStep("images")
	log.Printf("Generating question image for %s", clipID)

	img169 := filepath.Join(clipDir, "question-16x9.png")
	if err := p.kie.GenerateImage(ctx, prompt.ImagePrompt169, "16:9", img169); err != nil {
		p.tracker.FailStep("images", err)
		return nil, fmt.Errorf("generate 16:9 image: %w", err)
	}

	img916 := filepath.Join(clipDir, "question-9x16.png")
	if err := p.kie.GenerateImage(ctx, prompt.ImagePrompt916, "9:16", img916); err != nil {
		p.tracker.FailStep("images", err)
		return nil, fmt.Errorf("generate 9:16 image: %w", err)
	}

	p.tracker.CompleteStep("images")

	p.tracker.StartStep("assembly")
	video169 := filepath.Join(clipDir, "video-16x9.mp4")
	log.Printf("Assembling 16:9 video for %s", clipID)
	if err := p.ffmpeg.AssembleSingleImage(img169, voicePath, video169); err != nil {
		return nil, fmt.Errorf("assemble 16:9: %w", err)
	}

	video916 := filepath.Join(clipDir, "video-9x16.mp4")
	log.Printf("Assembling 9:16 video for %s", clipID)
	if err := p.ffmpeg.AssembleSingleImageVertical(img916, voicePath, video916); err != nil {
		return nil, fmt.Errorf("assemble 9:16: %w", err)
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
