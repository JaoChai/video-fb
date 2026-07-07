package producer

import (
	"encoding/binary"
	"os"
)

// QAAudioCheckEnabled turns on the voice-presence QA gate. Off → no audio check.
func QAAudioCheckEnabled() bool { return os.Getenv("QA_AUDIO_CHECK_ENABLED") == "true" }

// silenceRatio returns the fraction of 16-bit mono samples whose absolute
// amplitude is below threshold (near-silence). Empty PCM returns 1.0.
func silenceRatio(pcm []byte, threshold int32) float64 {
	n := len(pcm) / 2
	if n == 0 {
		return 1.0
	}
	silent := 0
	for i := 0; i+1 < len(pcm); i += 2 {
		s := int32(int16(binary.LittleEndian.Uint16(pcm[i : i+2])))
		if s < 0 {
			s = -s
		}
		if s < threshold {
			silent++
		}
	}
	return float64(silent) / float64(n)
}

// voiceSilent flags a rendered voice track that is effectively empty — too short
// overall, or almost entirely below the near-silence threshold.
func voiceSilent(pcm []byte, durationSec float64) bool {
	return durationSec < 1.0 || silenceRatio(pcm, 500) > 0.98
}

// probeVoiceSilent reads voice.wav and reports whether it is silent/too short.
// A read/probe error returns false (fail-open — the audio gate never invents a
// problem out of an unreadable file).
func probeVoiceSilent(voicePath string) bool {
	dur, err := wavDurationSeconds(voicePath)
	if err != nil {
		return false
	}
	pcm, err := readWAVPCM(voicePath)
	if err != nil {
		return false
	}
	return voiceSilent(pcm, dur)
}
