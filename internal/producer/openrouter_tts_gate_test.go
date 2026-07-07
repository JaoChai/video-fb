package producer

import "testing"

// 24000 Hz * 2 bytes/sample: 1 second of PCM = 48000 bytes.
func pcmOfSeconds(sec float64) []byte {
	return make([]byte, int(sec*48000))
}

func TestVoiceTooShort(t *testing.T) {
	if !voiceTooShort(3.0, 200) {
		t.Error("3s for 200 runes should be too short")
	}
	if voiceTooShort(3.0, 50) {
		t.Error("3s for 50 runes (short text) should be OK")
	}
	if voiceTooShort(9.0, 200) {
		t.Error("9s for 200 runes should be OK")
	}
}

func TestGateRetriesOnceThenSucceeds(t *testing.T) {
	calls := 0
	pcm, err := generateVoicePCMWithGate(200, true, func() ([]byte, error) {
		calls++
		if calls == 1 {
			return pcmOfSeconds(2.0), nil // too short first time
		}
		return pcmOfSeconds(10.0), nil // good on retry
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (one retry)", calls)
	}
	if pcmDurationSeconds(pcm) < 5.0 {
		t.Error("expected the retried (long) pcm")
	}
}

func TestGateErrorsWhenStillShortAfterRetry(t *testing.T) {
	_, err := generateVoicePCMWithGate(200, true, func() ([]byte, error) {
		return pcmOfSeconds(2.0), nil
	})
	if err == nil {
		t.Fatal("expected error when audio is still too short after retry")
	}
}

func TestGateOffDoesNotRetry(t *testing.T) {
	calls := 0
	_, err := generateVoicePCMWithGate(200, false, func() ([]byte, error) {
		calls++
		return pcmOfSeconds(2.0), nil
	})
	if err != nil {
		t.Fatalf("gate off should not error: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry when gate off)", calls)
	}
}
