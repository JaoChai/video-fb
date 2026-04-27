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
)

type Producer struct {
	kie     *KieClient
	ffmpeg  *FFmpegAssembler
	voice   string
	workDir string
	tracker *progress.Tracker
}

func NewProducer(kie *KieClient, ffmpeg *FFmpegAssembler, voice, workDir string, tracker *progress.Tracker) *Producer {
	os.MkdirAll(workDir, 0755)
	return &Producer{kie: kie, ffmpeg: ffmpeg, voice: voice, workDir: workDir, tracker: tracker}
}

type ProduceResult struct {
	Video169Path  string
	Video916Path  string
	ThumbnailPath string
}

func (p *Producer) Produce(ctx context.Context, clipID string, scenes []agent.GeneratedScene, imagePrompts []agent.SceneImagePrompts, voiceScript string) (*ProduceResult, error) {
	clipDir := filepath.Join(p.workDir, clipID)
	os.MkdirAll(clipDir, 0755)

	p.tracker.StartStep("voice")
	log.Printf("Generating voice for %s", clipID)
	voicePath := filepath.Join(clipDir, "voice.mp3")
	if err := p.kie.GenerateVoice(ctx, voiceScript, p.voice, voicePath); err != nil {
		p.tracker.FailStep("voice", err)
		return nil, fmt.Errorf("generate voice: %w", err)
	}
	p.tracker.CompleteStep("voice")

	p.tracker.StartStep("images")
	var scenes169 []AssemblyScene
	var scenes916 []AssemblyScene

	for i, prompt := range imagePrompts {
		log.Printf("Generating images for scene %d of %s", prompt.SceneNumber, clipID)

		img169 := filepath.Join(clipDir, fmt.Sprintf("scene-%d-16x9.png", prompt.SceneNumber))
		if err := p.kie.GenerateImage(ctx, prompt.ImagePrompt169, "16:9", img169); err != nil {
			return nil, fmt.Errorf("generate 16:9 image scene %d: %w", prompt.SceneNumber, err)
		}

		img916 := filepath.Join(clipDir, fmt.Sprintf("scene-%d-9x16.png", prompt.SceneNumber))
		if err := p.kie.GenerateImage(ctx, prompt.ImagePrompt916, "9:16", img916); err != nil {
			return nil, fmt.Errorf("generate 9:16 image scene %d: %w", prompt.SceneNumber, err)
		}

		dur := 10.0
		if i < len(scenes) {
			dur = scenes[i].DurationSeconds
		}

		scenes169 = append(scenes169, AssemblyScene{ImagePath: img169, DurationSeconds: dur})
		scenes916 = append(scenes916, AssemblyScene{ImagePath: img916, DurationSeconds: dur})
	}

	p.tracker.CompleteStep("images")

	p.tracker.StartStep("assembly")
	video169 := filepath.Join(clipDir, "video-16x9.mp4")
	log.Printf("Assembling 16:9 video for %s", clipID)
	if err := p.ffmpeg.Assemble(scenes169, voicePath, video169); err != nil {
		return nil, fmt.Errorf("assemble 16:9: %w", err)
	}

	video916 := filepath.Join(clipDir, "video-9x16.mp4")
	log.Printf("Assembling 9:16 video for %s", clipID)
	if err := p.ffmpeg.Assemble(scenes916, voicePath, video916); err != nil {
		return nil, fmt.Errorf("assemble 9:16: %w", err)
	}

	thumbPath := filepath.Join(clipDir, "thumbnail.png")
	if len(imagePrompts) > 0 {
		thumbPrompt := strings.Replace(imagePrompts[0].ImagePrompt169,
			"chat bubble", "YouTube thumbnail, large bold text, eye-catching", 1)
		if err := p.kie.GenerateImage(ctx, thumbPrompt, "16:9", thumbPath); err != nil {
			log.Printf("Thumbnail generation failed: %v (using scene 1 image)", err)
			thumbPath = scenes169[0].ImagePath
		}
	}

	p.tracker.CompleteStep("assembly")

	return &ProduceResult{
		Video169Path:  video169,
		Video916Path:  video916,
		ThumbnailPath: thumbPath,
	}, nil
}
