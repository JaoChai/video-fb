package producer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeBounds(t *testing.T) {
	t.Run("cumulative windows", func(t *testing.T) {
		got := computeBounds([]float64{8, 11, 5})
		want := []sceneBound{{0, 8}, {8, 19}, {19, 24}}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("bound %d = %v, want %v", i, got[i], want[i])
			}
		}
	})
	t.Run("nil and empty", func(t *testing.T) {
		if got := computeBounds(nil); got != nil {
			t.Errorf("computeBounds(nil) = %v, want nil", got)
		}
		if got := computeBounds([]float64{}); got != nil {
			t.Errorf("computeBounds([]) = %v, want nil", got)
		}
	})
	t.Run("single", func(t *testing.T) {
		got := computeBounds([]float64{12.5})
		if len(got) != 1 || got[0] != (sceneBound{0, 12.5}) {
			t.Errorf("got %v, want [{0 12.5}]", got)
		}
	})
}

// TestWavDurationSeconds round-trips a known PCM payload through wrapPCMAsWAV
// (24 kHz, 16-bit, mono) and asserts wavDurationSeconds recovers its length.
func TestWavDurationSeconds(t *testing.T) {
	// 24000 samples * 2 bytes = 48000 bytes = exactly 1.0 s of audio.
	pcm := make([]byte, 24000*2)
	wav := wrapPCMAsWAV(pcm, 24000, 1, 16)
	dir := t.TempDir()
	path := filepath.Join(dir, "one-second.wav")
	if err := os.WriteFile(path, wav, 0o644); err != nil {
		t.Fatalf("write wav: %v", err)
	}
	dur, err := wavDurationSeconds(path)
	if err != nil {
		t.Fatalf("wavDurationSeconds: %v", err)
	}
	if dur < 0.99 || dur > 1.01 {
		t.Errorf("duration = %v s, want ~1.0 s", dur)
	}

	// readWAVPCM must return exactly the PCM payload (header stripped).
	got, err := readWAVPCM(path)
	if err != nil {
		t.Fatalf("readWAVPCM: %v", err)
	}
	if len(got) != len(pcm) {
		t.Errorf("readWAVPCM len = %d, want %d", len(got), len(pcm))
	}
}
