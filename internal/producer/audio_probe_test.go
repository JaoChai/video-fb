package producer

import (
	"encoding/binary"
	"testing"
)

// loudPCM builds n 16-bit mono samples all at a given amplitude.
func loudPCM(n int, amp int16) []byte {
	b := make([]byte, n*2)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(amp))
	}
	return b
}

func TestSilenceRatioAllSilent(t *testing.T) {
	if got := silenceRatio(make([]byte, 2000), 500); got != 1.0 {
		t.Errorf("all-zero PCM silenceRatio = %v, want 1.0", got)
	}
}

func TestSilenceRatioAllLoud(t *testing.T) {
	if got := silenceRatio(loudPCM(1000, 8000), 500); got != 0.0 {
		t.Errorf("loud PCM silenceRatio = %v, want 0.0", got)
	}
}

func TestSilenceRatioEmpty(t *testing.T) {
	if got := silenceRatio(nil, 500); got != 1.0 {
		t.Errorf("empty PCM silenceRatio = %v, want 1.0", got)
	}
}

func TestVoiceSilent(t *testing.T) {
	// Long, loud track → not silent.
	if voiceSilent(loudPCM(240000, 8000), 10.0) {
		t.Error("loud 10s track should not be flagged silent")
	}
	// Full-length but all-silence → flagged.
	if !voiceSilent(make([]byte, 480000), 10.0) {
		t.Error("all-silent 10s track should be flagged")
	}
	// Too short → flagged regardless of content.
	if !voiceSilent(loudPCM(12000, 8000), 0.5) {
		t.Error("0.5s track should be flagged (too short)")
	}
}
