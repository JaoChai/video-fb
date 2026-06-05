package producer

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jaochai/video-fb/internal/agent"
)

// buildSceneSpecs maps the composition agent's per-scene design + the script
// scenes + audio boundaries into render-ready []SceneSpec. Slot text gets emphasis
// applied via highlightTitle. bgMode is "image" for scenes that have a background
// image planned, else "css".
//
// Designs and bounds are matched by index (0-based). If the slices differ in
// length, only min(len(designs), len(bounds)) entries are produced; extras are
// silently dropped so a short LLM response never panics.
func buildSceneSpecs(designs []agent.SceneDesign, bounds []sceneBound, bgMode func(sceneNumber int) string) []SceneSpec {
	n := len(designs)
	if nb := len(bounds); nb < n {
		n = nb
	}
	if n == 0 {
		return nil
	}

	specs := make([]SceneSpec, n)
	for i := 0; i < n; i++ {
		d := designs[i]
		b := bounds[i]

		// Number step slots sequentially within this scene.
		stepCounter := 0
		slots := make([]SlotSpec, 0, len(d.Slots))
		for _, s := range d.Slots {
			ss := SlotSpec{
				Role: s.Role,
				HTML: highlightTitle(s.Text, s.Emphasis),
			}
			if s.Role == "step" {
				stepCounter++
				ss.StepNum = stepCounter
			}
			slots = append(slots, ss)
		}

		mascotPose := ""
		if cuePose := MascotCueToPose(d.MascotCue); cuePose != "" {
			mascotPose = "assets/mascot/" + cuePose + ".png"
		}

		specs[i] = SceneSpec{
			SceneNumber:    d.SceneNumber,
			LayoutVariant:  d.LayoutVariant,
			AccentColor:    d.AccentColor,
			AnimationSpeed: d.AnimationSpeed,
			StartSec:       b.Start,
			EndSec:         b.End,
			BackgroundMode: bgMode(d.SceneNumber),
			Slots:          slots,
			CaptionStyle:   d.CaptionStyle,
			MascotPose:     mascotPose,
		}
	}
	return specs
}

// sceneBound is one scene's [start, end) window on the combined audio timeline.
type sceneBound struct{ Start, End float64 }

// computeBounds turns per-scene durations into cumulative [start, end) windows.
// Example: [8, 11, 5] → [{0,8},{8,19},{19,24}]
func computeBounds(durations []float64) []sceneBound {
	if len(durations) == 0 {
		return nil
	}
	bounds := make([]sceneBound, len(durations))
	var cursor float64
	for i, d := range durations {
		bounds[i] = sceneBound{Start: cursor, End: cursor + d}
		cursor += d
	}
	return bounds
}

// wavDurationSeconds reads the PCM data-chunk size from a WAV file header and
// converts it to seconds using the same 24 kHz / 16-bit / mono parameters
// that GenerateVoice writes.
func wavDurationSeconds(path string) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open wav %s: %w", path, err)
	}
	defer f.Close()

	// Minimal WAV header: 44 bytes.
	// Bytes 40-43: PCM data chunk size (little-endian uint32).
	var header [44]byte
	if _, err := f.Read(header[:]); err != nil {
		return 0, fmt.Errorf("read wav header %s: %w", path, err)
	}
	// Sanity check RIFF / WAVE markers.
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return 0, fmt.Errorf("not a valid WAV file: %s", path)
	}

	dataSize := binary.LittleEndian.Uint32(header[40:44])
	const sampleRate = 24000
	const bytesPerSample = 2 // 16-bit mono
	duration := float64(dataSize) / float64(sampleRate*bytesPerSample)
	return duration, nil
}

// readWAVPCM reads just the PCM payload (everything after the 44-byte header)
// from a WAV file written by GenerateVoice.
func readWAVPCM(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wav %s: %w", path, err)
	}
	if len(data) < 44 {
		return nil, fmt.Errorf("wav file too small: %s", path)
	}
	return data[44:], nil
}

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
