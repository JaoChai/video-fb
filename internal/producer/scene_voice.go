package producer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jaochai/video-fb/internal/agent"
)

// synthScenesVoice TTSes each scene's VoiceText, concatenates them into one
// voice.wav at clipDir/voice.wav, and returns its path plus per-scene [start,end)
// boundaries on the combined timeline.
func (p *Producer) synthScenesVoice(ctx context.Context, scenes []agent.GeneratedScene, voice, clipDir string) (string, []sceneBound, error) {
	if len(scenes) == 0 {
		return "", nil, fmt.Errorf("no scenes")
	}

	// TTS each scene into a temporary per-scene WAV.
	tmpPaths := make([]string, len(scenes))
	for i, scene := range scenes {
		tmpPath := filepath.Join(clipDir, fmt.Sprintf("voice-scene%d.wav", i+1))
		tmpPaths[i] = tmpPath

		if scene.VoiceText == "" {
			// Write a zero-duration placeholder (minimal valid WAV with no PCM).
			wavData := wrapPCMAsWAV(nil, 24000, 1, 16)
			if err := os.WriteFile(tmpPath, wavData, 0644); err != nil {
				return "", nil, fmt.Errorf("write silent wav for scene %d: %w", i+1, err)
			}
			log.Printf("synthScenesVoice: scene %d has empty VoiceText — writing silent placeholder", i+1)
			continue
		}

		log.Printf("synthScenesVoice: TTS scene %d/%d (%d chars)", i+1, len(scenes), len([]rune(scene.VoiceText)))
		if err := p.openRouter.GenerateVoice(ctx, scene.VoiceText, voice, tmpPath); err != nil {
			return "", nil, fmt.Errorf("TTS scene %d: %w", i+1, err)
		}
	}

	// Measure per-scene durations and concatenate all PCM.
	durations := make([]float64, len(scenes))
	var allPCM []byte
	for i, tmpPath := range tmpPaths {
		dur, err := wavDurationSeconds(tmpPath)
		if err != nil {
			return "", nil, fmt.Errorf("measure duration scene %d: %w", i+1, err)
		}
		durations[i] = dur

		pcm, err := readWAVPCM(tmpPath)
		if err != nil {
			return "", nil, fmt.Errorf("read PCM scene %d: %w", i+1, err)
		}
		allPCM = append(allPCM, pcm...)
	}

	// Write the combined WAV.
	outPath := filepath.Join(clipDir, "voice.wav")
	const sampleRate = 24000
	wavData := wrapPCMAsWAV(allPCM, sampleRate, 1, 16)
	if err := os.MkdirAll(clipDir, 0755); err != nil {
		return "", nil, fmt.Errorf("create clipDir: %w", err)
	}
	if err := os.WriteFile(outPath, wavData, 0644); err != nil {
		return "", nil, fmt.Errorf("write combined voice.wav: %w", err)
	}

	bounds := computeBounds(durations)

	total := 0.0
	for _, d := range durations {
		total += d
	}
	log.Printf("synthScenesVoice: %d scenes, total %.1fs → %s", len(scenes), total, outPath)
	for i, b := range bounds {
		log.Printf("  scene %d: [%.2fs, %.2fs) (%.2fs)", i+1, b.Start, b.End, durations[i])
	}

	// Clean up per-scene temp WAVs.
	for _, tmpPath := range tmpPaths {
		os.Remove(tmpPath)
	}

	return outPath, bounds, nil
}
