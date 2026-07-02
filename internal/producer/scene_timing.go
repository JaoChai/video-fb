package producer

import (
	"encoding/binary"
	"fmt"
	"os"
)

// sceneBound is one scene's [start, end) window on the combined audio timeline.
type sceneBound struct{ Start, End float64 }

// boundsToDurations returns each scene's real rendered duration (End-Start),
// derived from the measured voice bounds. Used to persist accurate per-scene
// durations (scenes.duration_seconds), which the scene agent never emits.
func boundsToDurations(bounds []sceneBound) []float64 {
	durs := make([]float64, len(bounds))
	for i, b := range bounds {
		durs[i] = b.End - b.Start
	}
	return durs
}

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
