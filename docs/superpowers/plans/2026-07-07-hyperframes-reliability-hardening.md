# Hyperframes Reliability Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close six backend reliability gaps in the hyperframes video pipeline so broken/silent clips are caught (retried or human-reviewed) instead of shipped, and infra hiccups never fail a whole clip.

**Architecture:** Each fix is a small, local change. Where the change is non-trivial we extract a pure helper (`renderGateDecision`, `uploadWithFallback`, `voiceTooShort`, `silenceRatio`, `qaFrameTargets`) and unit-test the helper, then wire it in. Three behavior-changing gates are guarded by env flags (default off). Two new signals (`RenderFlagged`, `AudioFlagged`) travel from the producer to the orchestrator via a new internal `assembleOutput` struct and existing `ProduceResult`.

**Tech Stack:** Go, standard `testing` package. No new dependencies. No DB migrations.

## Global Constraints

- Feature flags use the existing env pattern verbatim: `os.Getenv("NAME") == "true"` (see `internal/producer/presets.go:93`, `internal/producer/audio.go:17`). Flag names: `RENDER_ERROR_GATE_ENABLED`, `TTS_LENGTH_GATE_ENABLED`, `QA_AUDIO_CHECK_ENABLED`. All default off (unset → false).
- No schema changes. Reuse the existing `needs_review` status and `retry_count` column.
- All PCM is 24000 Hz, 16-bit, mono (`sampleRate=24000`, `bytesPerSample=2`).
- Every commit message ends with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
- Run the full suite with `go test ./...` before each commit; individual tests with `go test ./internal/producer/ -run TestName -v`.

---

### Task 1: Fix 6 — Stop deploys burning a clip's retry budget

**Files:**
- Modify: `internal/repository/clips.go:124-136` (`ResetStaleProducing`)

**Interfaces:**
- Consumes: nothing.
- Produces: no signature change — `ResetStaleProducing(ctx) (int64, error)` unchanged.

There is no test DB harness in this repo (repository tests are pure-function only, e.g. `formats_test.go`), and this is a one-line SQL change, so it is verified by `go build` + SQL review rather than a unit test.

- [ ] **Step 1: Remove the retry_count increment**

In `internal/repository/clips.go`, edit `ResetStaleProducing` to drop the `retry_count = retry_count + 1,` line. The result:

```go
func (r *ClipsRepo) ResetStaleProducing(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE clips
		 SET status = 'failed',
		     fail_reason = 'การผลิตถูกขัดจังหวะ (เซิร์ฟเวอร์รีสตาร์ท) — กด Retry เพื่อลองใหม่',
		     updated_at = NOW()
		 WHERE status = 'producing'`)
	if err != nil {
		return 0, fmt.Errorf("reset stale producing clips: %w", err)
	}
	return tag.RowsAffected(), nil
}
```

Also update the doc comment's last sentence to note the retry budget is intentionally preserved: append to the existing comment above the function:

```go
// A restart is infrastructure, not a content failure, so retry_count is left
// unchanged — an interrupted render must not consume the clip's retry budget.
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./...`
Expected: builds with no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/repository/clips.go
git commit -m "fix(clips): don't burn retry budget when recovering interrupted renders

ResetStaleProducing incremented retry_count on every restart-orphaned clip;
two deploys during one clip's render exhausted maxClipRetries and stuck a
good clip in 'failed'. A restart is infra, not a content failure.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Fix 5 — Skip QA frame extraction for zero-duration scenes

**Files:**
- Create: `internal/orchestrator/qa_frames_test.go`
- Modify: `internal/orchestrator/orchestrator.go:764-789` (`extractQAFrames`)

**Interfaces:**
- Consumes: nothing.
- Produces: `func qaFrameTargets(durs []float64) []bool` — element `i` is true when scene `i` should be sampled (duration > 0). `extractQAFrames` uses it to skip zero-width scenes.

A zero-duration scene (empty `VoiceText` → silent placeholder) gets a timestamp exactly on the transition boundary from `sceneAwareTimestamps`, producing a blank frame that QA false-flags. Skip those scenes entirely.

- [ ] **Step 1: Write the failing test**

Create `internal/orchestrator/qa_frames_test.go`:

```go
package orchestrator

import "testing"

func TestQAFrameTargetsSkipsZeroDuration(t *testing.T) {
	got := qaFrameTargets([]float64{3.0, 0.0, 5.0})
	want := []bool{true, false, true}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("scene %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestQAFrameTargetsNegativeIsSkipped(t *testing.T) {
	got := qaFrameTargets([]float64{-1.0, 2.0})
	if got[0] || !got[1] {
		t.Errorf("got %v, want [false true]", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator/ -run TestQAFrameTargets -v`
Expected: FAIL — `undefined: qaFrameTargets`.

- [ ] **Step 3: Add the helper**

In `internal/orchestrator/orchestrator.go`, add above `extractQAFrames`:

```go
// qaFrameTargets marks which scenes should have a QA frame sampled. Zero- (or
// negative-) duration scenes are skipped: they come from an empty VoiceText
// placeholder and their sampled timestamp lands exactly on a transition
// boundary, producing a blank frame that QA false-flags.
func qaFrameTargets(durs []float64) []bool {
	targets := make([]bool, len(durs))
	for i, d := range durs {
		targets[i] = d > 0
	}
	return targets
}
```

- [ ] **Step 4: Wire it into extractQAFrames**

In `extractQAFrames`, after `mids := o.qaFrameTimestamps(mp4Path, durs, qaSceneFrac)`, add the target computation and skip. The loop becomes:

```go
	mids := o.qaFrameTimestamps(mp4Path, durs, qaSceneFrac)
	targets := qaFrameTargets(durs)
	frames := make([]agent.QAFrame, 0, len(scenes))
	for i, s := range scenes {
		if i >= len(mids) {
			break // sampler returned no usable timestamps (missing durations + probe fail) — fail-open
		}
		if !targets[i] {
			continue // zero-duration scene: sampling it would hit a transition boundary
		}
		outPath := filepath.Join(filepath.Dir(mp4Path), fmt.Sprintf("qa-scene%d.png", s.SceneNumber))
```

(Leave the rest of the loop body unchanged.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/orchestrator/ -run TestQAFrameTargets -v`
Expected: PASS (both tests).

- [ ] **Step 6: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/orchestrator/qa_frames_test.go
git commit -m "fix(qa): skip QA frame extraction for zero-duration scenes

An empty-VoiceText scene renders a silent zero-width placeholder; its sampled
timestamp lands on a transition boundary → blank frame → QA false positive.
Skip those scenes so scene-aware sampling isn't re-polluted.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Fix 2 — R2 upload runtime fallback to kie

**Files:**
- Create: `internal/producer/upload_fallback_test.go`
- Modify: `internal/producer/producer.go:411-420` (`uploadPersistent`)

**Interfaces:**
- Consumes: nothing.
- Produces: `func uploadWithFallback(primary, fallback func() (string, error)) (string, error)` — runs `primary`; on error logs and runs `fallback`. `uploadPersistent` uses it so an R2 outage degrades to kie instead of failing the clip.

- [ ] **Step 1: Write the failing test**

Create `internal/producer/upload_fallback_test.go`:

```go
package producer

import (
	"errors"
	"testing"
)

func TestUploadWithFallbackUsesPrimaryOnSuccess(t *testing.T) {
	fallbackCalled := false
	url, err := uploadWithFallback(
		func() (string, error) { return "primary-url", nil },
		func() (string, error) { fallbackCalled = true; return "fallback-url", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "primary-url" {
		t.Errorf("url = %q, want primary-url", url)
	}
	if fallbackCalled {
		t.Error("fallback should not run when primary succeeds")
	}
}

func TestUploadWithFallbackFallsBackOnPrimaryError(t *testing.T) {
	url, err := uploadWithFallback(
		func() (string, error) { return "", errors.New("r2 down") },
		func() (string, error) { return "fallback-url", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "fallback-url" {
		t.Errorf("url = %q, want fallback-url", url)
	}
}

func TestUploadWithFallbackReturnsFallbackError(t *testing.T) {
	_, err := uploadWithFallback(
		func() (string, error) { return "", errors.New("r2 down") },
		func() (string, error) { return "", errors.New("kie down") },
	)
	if err == nil {
		t.Fatal("expected error when both uploads fail")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestUploadWithFallback -v`
Expected: FAIL — `undefined: uploadWithFallback`.

- [ ] **Step 3: Add the helper and wire uploadPersistent**

In `internal/producer/producer.go`, add the helper and update `uploadPersistent`:

```go
// uploadWithFallback runs primary; on error it logs and runs fallback. Used so a
// transient R2 outage degrades to kie's temporary URL instead of failing the clip.
func uploadWithFallback(primary, fallback func() (string, error)) (string, error) {
	url, err := primary()
	if err == nil {
		return url, nil
	}
	log.Printf("uploadPersistent: primary (R2) upload failed, falling back to kie: %v", err)
	return fallback()
}

// uploadPersistent stores a rendered file at a durable URL. It prefers R2 (URLs
// never expire); if R2 is disabled/unconfigured OR the R2 upload errors at
// runtime it falls back to kie.ai's temporary upload so the pipeline keeps
// working. r2Key is the full object key; kieDir is the legacy kie uploadPath.
func (p *Producer) uploadPersistent(ctx context.Context, localPath, r2Key, kieDir string) (string, error) {
	if p.r2 != nil && p.r2.Enabled(ctx) {
		return uploadWithFallback(
			func() (string, error) { return p.r2.Upload(ctx, localPath, r2Key, "") },
			func() (string, error) { return p.kie.UploadFile(ctx, localPath, kieDir) },
		)
	}
	return p.kie.UploadFile(ctx, localPath, kieDir)
}
```

(Confirm `log` is already imported in `producer.go` — it is, used at line 328.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/producer/ -run TestUploadWithFallback -v`
Expected: PASS (all three tests).

- [ ] **Step 5: Commit**

```bash
git add internal/producer/producer.go internal/producer/upload_fallback_test.go
git commit -m "fix(upload): fall back to kie when R2 upload errors at runtime

uploadPersistent chose R2 xor kie up front, so an R2 outage (not just R2 being
disabled) failed the whole clip's upload. Now a runtime R2 error degrades to
kie's temporary URL and logs an alert.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Fix 3 — TTS-too-short gate (retry once, then fail)

**Files:**
- Create: `internal/producer/openrouter_tts_gate_test.go`
- Modify: `internal/producer/openrouter.go:148-205` (`GenerateVoice`), add helpers + flag

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func pcmDurationSeconds(pcm []byte) float64`
  - `func voiceTooShort(durationSec float64, textRunes int) bool`
  - `func generateVoicePCMWithGate(textRunes int, gateEnabled bool, gen func() ([]byte, error)) ([]byte, error)` — calls `gen`; if gate on and result too short, retries once and errors if still short.
  - `func TTSLengthGateEnabled() bool`

- [ ] **Step 1: Write the failing test**

Create `internal/producer/openrouter_tts_gate_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run 'TestVoiceTooShort|TestGate' -v`
Expected: FAIL — `undefined: voiceTooShort` / `generateVoicePCMWithGate` / `pcmDurationSeconds`.

- [ ] **Step 3: Add helpers and the flag**

In `internal/producer/openrouter.go`, add (near the top-level funcs, e.g. above `GenerateVoice`):

```go
// TTSLengthGateEnabled turns on retry-then-fail when TTS output is implausibly
// short for its input (likely truncation). Off → legacy warning-only behavior.
func TTSLengthGateEnabled() bool { return os.Getenv("TTS_LENGTH_GATE_ENABLED") == "true" }

// pcmDurationSeconds returns the play length of raw 24kHz 16-bit mono PCM.
func pcmDurationSeconds(pcm []byte) float64 {
	const sampleRate = 24000
	const bytesPerSample = 2
	return float64(len(pcm)) / float64(sampleRate*bytesPerSample)
}

// voiceTooShort flags audio that is implausibly short for its input text.
func voiceTooShort(durationSec float64, textRunes int) bool {
	return durationSec < 5.0 && textRunes > 100
}

// generateVoicePCMWithGate calls gen to synthesize PCM. If the length gate is on
// and the result is too short for the input, it retries once; if it is still too
// short it returns an error so the caller fails the clip (→ retry tick) instead
// of shipping missing narration.
func generateVoicePCMWithGate(textRunes int, gateEnabled bool, gen func() ([]byte, error)) ([]byte, error) {
	pcm, err := gen()
	if err != nil {
		return nil, err
	}
	if !gateEnabled || !voiceTooShort(pcmDurationSeconds(pcm), textRunes) {
		return pcm, nil
	}
	log.Printf("WARNING: TTS audio unusually short (%.1fs for %d runes) — retrying once",
		pcmDurationSeconds(pcm), textRunes)
	pcm, err = gen()
	if err != nil {
		return nil, err
	}
	if voiceTooShort(pcmDurationSeconds(pcm), textRunes) {
		return nil, fmt.Errorf("TTS audio too short after retry (%.1fs for %d runes) — likely truncation",
			pcmDurationSeconds(pcm), textRunes)
	}
	return pcm, nil
}
```

(Confirm `os` is already imported in `openrouter.go` — it is, used at line 195.)

- [ ] **Step 4: Refactor GenerateVoice to use the gate**

Replace `GenerateVoice` (lines 148-205) with a version whose chunk-synthesis is wrapped in a `gen` closure and run through the gate. The final WAV write stays the same:

```go
func (o *OpenRouterClient) GenerateVoice(ctx context.Context, text, voice, outputPath string) error {
	chunks := splitVoiceText(text, ttsMaxChunkRunes)
	if len(chunks) == 0 {
		return fmt.Errorf("no text to generate voice for")
	}
	if len(chunks) > 1 {
		log.Printf("Splitting voice text into %d chunks for TTS (%d chars total)", len(chunks), len([]rune(text)))
	}

	// gen synthesizes the full PCM for the text once (all chunks concatenated with
	// trims/gaps). It is a closure so the length gate can invoke it twice.
	gen := func() ([]byte, error) {
		var allPCM []byte
		for i, chunk := range chunks {
			if len(chunks) > 1 {
				log.Printf("Generating TTS chunk %d/%d (%d chars)", i+1, len(chunks), len([]rune(chunk)))
			}
			var pcm []byte
			err := retryableCall(ctx, "openrouter-tts", func() error {
				var genErr error
				pcm, genErr = o.generatePCM(ctx, chunk, voice)
				return genErr
			})
			if err != nil {
				return nil, fmt.Errorf("TTS chunk %d/%d: %w", i+1, len(chunks), err)
			}
			if len(chunks) > 1 {
				trimLeading := i > 0
				trimTrailing := i < len(chunks)-1
				pcm = trimPCMSilence(pcm, trimLeading, trimTrailing)
				if i > 0 {
					allPCM = append(allPCM, make([]byte, gapBytes)...)
				}
			}
			allPCM = append(allPCM, pcm...)
		}
		return allPCM, nil
	}

	allPCM, err := generateVoicePCMWithGate(len([]rune(text)), TTSLengthGateEnabled(), gen)
	if err != nil {
		return err
	}

	const sampleRate = 24000
	wavData := wrapPCMAsWAV(allPCM, sampleRate, 1, 16)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, wavData, 0644); err != nil {
		return fmt.Errorf("write audio: %w", err)
	}
	log.Printf("Saved TTS audio (%d bytes PCM → %d bytes WAV, %.1fs, %d chunks) to %s",
		len(allPCM), len(wavData), pcmDurationSeconds(allPCM), len(chunks), outputPath)
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/producer/ -run 'TestVoiceTooShort|TestGate' -v`
Expected: PASS (all four tests).

- [ ] **Step 6: Run the producer suite to confirm no regression**

Run: `go test ./internal/producer/ -v`
Expected: PASS (existing TTS/audio tests still green).

- [ ] **Step 7: Commit**

```bash
git add internal/producer/openrouter.go internal/producer/openrouter_tts_gate_test.go
git commit -m "feat(tts): retry-then-fail on truncated TTS output (flag-gated)

TTS output that was implausibly short for its input only logged a warning and
shipped a clip with missing narration. With TTS_LENGTH_GATE_ENABLED it now
retries once and fails the clip (→ retry tick) if still short. Flag off keeps
legacy warning-only behavior.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Fix 4 — QA audio check + introduce assembleOutput struct

**Files:**
- Create: `internal/producer/audio_probe_test.go`
- Create: `internal/producer/audio_probe.go`
- Modify: `internal/producer/producer.go` — `ProduceResult` struct (add `AudioFlagged`), `AssembleHyperframes916` return type → `*assembleOutput`, `ProduceHyperframes916` caller
- Modify: `internal/producer/producer_hyperframes_test.go:55` (smoke test call site)
- Modify: `internal/orchestrator/orchestrator.go` — act on `AudioFlagged` in `renderAndFinalize`

**Interfaces:**
- Consumes: `readWAVPCM(path) ([]byte, error)` (`scene_timing.go:68`), `wavDurationSeconds(path) (float64, error)` (`scene_timing.go:41`).
- Produces:
  - `func silenceRatio(pcm []byte, threshold int32) float64`
  - `func voiceSilent(pcm []byte, durationSec float64) bool`
  - `func QAAudioCheckEnabled() bool`
  - `type assembleOutput struct { mp4Path string; sceneDurations []float64; inspectFlagged bool; audioFlagged bool; renderFlagged bool }` (defined here; `renderFlagged` is populated in Task 6)
  - `ProduceResult.AudioFlagged bool`

- [ ] **Step 1: Write the failing test**

Create `internal/producer/audio_probe_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run 'TestSilenceRatio|TestVoiceSilent' -v`
Expected: FAIL — `undefined: silenceRatio` / `voiceSilent`.

- [ ] **Step 3: Create the audio-probe helpers**

Create `internal/producer/audio_probe.go`:

```go
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
```

- [ ] **Step 4: Run helper tests to verify they pass**

Run: `go test ./internal/producer/ -run 'TestSilenceRatio|TestVoiceSilent' -v`
Expected: PASS (all four tests).

- [ ] **Step 5: Add AudioFlagged to ProduceResult**

In `internal/producer/producer.go`, add a field to the `ProduceResult` struct (after `InspectFlagged bool` at line 76):

```go
	// AudioFlagged is true when the rendered voice track is silent/too short
	// (QA audio check). The orchestrator routes such clips to needs_review when
	// QA_AUDIO_CHECK_ENABLED is on.
	AudioFlagged bool
```

- [ ] **Step 6: Convert AssembleHyperframes916 to return assembleOutput**

In `internal/producer/producer.go`, define the struct above `AssembleHyperframes916` and change its signature + all `return` statements. Add:

```go
// assembleOutput carries the render products and post-render health signals from
// AssembleHyperframes916 up to ProduceHyperframes916.
type assembleOutput struct {
	mp4Path        string
	sceneDurations []float64
	inspectFlagged bool
	audioFlagged   bool
	renderFlagged  bool // populated by the render browser-error gate
}
```

Change the signature from:

```go
func (p *Producer) AssembleHyperframes916(ctx context.Context, clipID string, scenes []agent.GeneratedScene, preset StylePreset) (string, []float64, bool, error) {
```

to:

```go
func (p *Producer) AssembleHyperframes916(ctx context.Context, clipID string, scenes []agent.GeneratedScene, preset StylePreset) (*assembleOutput, error) {
```

Update every early-return in the function to return `nil, fmt.Errorf(...)` instead of `"", nil, false, fmt.Errorf(...)`. There are seven such returns (mkdir clipDir, synth voice, buildSceneSpecs empty, mkdir projectDir, build scenes, render — see lines 303, 310, 341, 395, 398, 406). Each becomes e.g.:

```go
		return nil, fmt.Errorf("mkdir clipDir: %w", err)
```

Then replace the final success return (line 408) with an `assembleOutput`. Just before it, compute the audio flag from the already-synthesized `voicePath`:

```go
	audioFlagged := probeVoiceSilent(voicePath)
	return &assembleOutput{
		mp4Path:        filepath.Join(projectDir, "output.mp4"),
		sceneDurations: boundsToDurations(bounds),
		inspectFlagged: inspectFlagged,
		audioFlagged:   audioFlagged,
	}, nil
```

- [ ] **Step 7: Update the ProduceHyperframes916 caller**

In `ProduceHyperframes916` (line 429), change:

```go
	mp4Path, sceneDurations, inspectFlagged, err := p.AssembleHyperframes916(ctx, clipID, scenes, preset)
	if err != nil {
		p.tracker.FailStep("assembly", err)
		return nil, fmt.Errorf("assemble hyperframes: %w", err)
	}
```

to:

```go
	out, err := p.AssembleHyperframes916(ctx, clipID, scenes, preset)
	if err != nil {
		p.tracker.FailStep("assembly", err)
		return nil, fmt.Errorf("assemble hyperframes: %w", err)
	}
	mp4Path := out.mp4Path
```

Then in the final `ProduceResult` literal (line 456), reference `out` for the flags:

```go
	return &ProduceResult{
		Video916URL:       video916URL,
		ThumbnailURL:      thumbnailURL,
		LocalVideo916Path: mp4Path,
		SceneDurations:    out.sceneDurations,
		InspectFlagged:    out.inspectFlagged,
		AudioFlagged:      out.audioFlagged,
		RenderFlagged:     out.renderFlagged,
	}, nil
```

(`RenderFlagged` is declared in Task 6; if implementing Task 5 alone, omit that line and re-add it in Task 6. When doing 5→6 in order, it is cleanest to add the `ProduceResult.RenderFlagged` field now in Step 5 as well — see Task 6 Step 3.)

- [ ] **Step 8: Fix the smoke-test call site**

In `internal/producer/producer_hyperframes_test.go:55`, change:

```go
	out, _, _, err := p.AssembleHyperframes916(context.Background(), "smoke-clip", scenes, PresetByKey("editorial-bold"))
```

to:

```go
	out, err := p.AssembleHyperframes916(context.Background(), "smoke-clip", scenes, PresetByKey("editorial-bold"))
```

Then update any later use of `out` in that test to `out.mp4Path` (it was previously the mp4 path string). Read lines 55-75 of that file first and adjust references accordingly (e.g. `if out == ""` → `if out.mp4Path == ""`).

- [ ] **Step 9: Wire AudioFlagged into the orchestrator**

In `internal/orchestrator/orchestrator.go`, in `renderAndFinalize`, after the inspect-flag block (after line 505), add:

```go
	// A silent/too-short voice track can't be seen by the still-frame vision QA —
	// route to needs_review when the audio gate is on.
	if result.AudioFlagged && producer.QAAudioCheckEnabled() && status == "ready" {
		status = "needs_review"
		log.Printf("clip %s: voice track silent/too short — status=needs_review (publish blocked)", clipID)
	}
```

(Confirm `producer` is already imported in `orchestrator.go` — it is, used for `producer.StylePreset`.)

- [ ] **Step 10: Run the full producer + orchestrator suites**

Run: `go test ./internal/producer/ ./internal/orchestrator/`
Expected: PASS. Then `go build ./...` — no errors.

- [ ] **Step 11: Commit**

```bash
git add internal/producer/audio_probe.go internal/producer/audio_probe_test.go internal/producer/producer.go internal/producer/producer_hyperframes_test.go internal/orchestrator/orchestrator.go
git commit -m "feat(qa): flag silent/too-short voice tracks to needs_review (flag-gated)

Still-frame visual QA is blind to audio. AssembleHyperframes916 now probes the
rendered voice.wav (duration + near-silence ratio) and surfaces AudioFlagged via
a new assembleOutput struct; with QA_AUDIO_CHECK_ENABLED the orchestrator routes
silent clips to needs_review. Flag off = no change.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Fix 1 — Broken/frozen render gate (retry, then human review)

**Files:**
- Create: `internal/producer/render_gate_test.go`
- Modify: `internal/producer/hyperframes.go` — `run` returns issues; `Lint`/`Inspect`/`Render` adapt; add `RenderErrorGateEnabled` + `RenderGateDecision`
- Modify: `internal/producer/producer.go` — capture render issues in `AssembleHyperframes916`; add `ProduceResult.RenderFlagged`
- Modify: `internal/orchestrator/orchestrator.go` — act on `RenderFlagged` in `renderAndFinalize`

**Interfaces:**
- Consumes: `assembleOutput` (Task 5), `scanBrowserIssues([]byte) []string` (`hyperframes.go:51`), `ClipsRepo.GetByID(ctx, id) (*models.Clip, error)` (`clips.go:58`), `models.Clip.RetryCount int` (`clip.go:24`).
- Produces:
  - `func RenderErrorGateEnabled() bool`
  - `type RenderGateAction int` with `RenderGateNone`, `RenderGateRetry`, `RenderGateReview`
  - `func RenderGateDecision(flagged, gateEnabled bool, retryCount int) RenderGateAction`
  - `ProduceResult.RenderFlagged bool`
  - `HyperframesRenderer.Render(ctx, dir, outputPath) ([]string, error)` (signature changes: now returns browser issues)

- [ ] **Step 1: Write the failing test**

Create `internal/producer/render_gate_test.go`:

```go
package producer

import "testing"

func TestRenderGateDisabled(t *testing.T) {
	if RenderGateDecision(true, false, 0) != RenderGateNone {
		t.Error("gate off must be RenderGateNone even when flagged")
	}
}

func TestRenderGateNotFlagged(t *testing.T) {
	if RenderGateDecision(false, true, 5) != RenderGateNone {
		t.Error("not flagged must be RenderGateNone")
	}
}

func TestRenderGateFirstOffenseRetries(t *testing.T) {
	if RenderGateDecision(true, true, 0) != RenderGateRetry {
		t.Error("first offense (retryCount 0) must retry")
	}
}

func TestRenderGatePersistentGoesToReview(t *testing.T) {
	if RenderGateDecision(true, true, 1) != RenderGateReview {
		t.Error("still broken after a retry must go to review")
	}
	if RenderGateDecision(true, true, 3) != RenderGateReview {
		t.Error("retryCount>=1 must go to review")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/producer/ -run TestRenderGate -v`
Expected: FAIL — `undefined: RenderGateDecision` / `RenderGateNone` / etc.

- [ ] **Step 3: Add the flag, action type, and decision helper**

In `internal/producer/hyperframes.go`, add `"os"` to the imports, then add:

```go
// RenderErrorGateEnabled turns on treating a browser-error-flagged render as a
// failure (retry) then routing to needs_review. Off → legacy log-only behavior.
func RenderErrorGateEnabled() bool { return os.Getenv("RENDER_ERROR_GATE_ENABLED") == "true" }

// RenderGateAction is what to do about a render that tripped browser-error detection.
type RenderGateAction int

const (
	RenderGateNone   RenderGateAction = iota // do nothing (not flagged, or gate off)
	RenderGateRetry                          // fail the clip so the retry tick re-renders
	RenderGateReview                         // still broken after a retry → human review
)

// RenderGateDecision decides how to handle a render flagged with browser errors.
// First offense (retryCount 0) retries; a render still broken after at least one
// retry goes to human review. A frozen render exits 0 and looks fine to the
// still-frame vision QA, so this is the only place it can be caught.
func RenderGateDecision(flagged, gateEnabled bool, retryCount int) RenderGateAction {
	if !flagged || !gateEnabled {
		return RenderGateNone
	}
	if retryCount == 0 {
		return RenderGateRetry
	}
	return RenderGateReview
}
```

- [ ] **Step 4: Run the decision test to verify it passes**

Run: `go test ./internal/producer/ -run TestRenderGate -v`
Expected: PASS (all four tests).

- [ ] **Step 5: Make run/Render return browser issues**

In `internal/producer/hyperframes.go`, change `run` to return the issues and adapt the three callers:

```go
func (h *HyperframesRenderer) run(ctx context.Context, dir string, args ...string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	cmd := hyperframesCmd(ctx, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	issues := scanBrowserIssues(out)
	if len(issues) > 0 {
		log.Printf("hyperframes %v browser issues:\n%s", args, strings.Join(issues, "\n"))
	}
	if err != nil {
		return issues, fmt.Errorf("hyperframes %v failed: %w\n%s", args, err, lastBytes(out, 600))
	}
	return issues, nil
}
```

```go
func (h *HyperframesRenderer) Lint(ctx context.Context, dir string) error {
	_, err := h.run(ctx, dir, "lint")
	return err
}
```

```go
func (h *HyperframesRenderer) Inspect(ctx context.Context, dir string) error {
	_, err := h.run(ctx, dir, "inspect")
	return err
}
```

```go
// Render produces an MP4 at outputPath from the composition in dir. It returns
// any browser-error lines the render emitted (a render can exit 0 while a JS
// exception froze every animation into a static video). Quality is standard/24fps
// so the memory-heavy multi-scene render fits the ~8GB container without OOM.
func (h *HyperframesRenderer) Render(ctx context.Context, dir, outputPath string) ([]string, error) {
	return h.run(ctx, dir, "render", "--output", outputPath, "--quality", "standard", "--fps", "24", "-w", renderWorkers)
}
```

- [ ] **Step 6: Capture render issues in AssembleHyperframes916**

In `internal/producer/producer.go`, replace the render call (line 405-407):

```go
	if err := p.hf.renderer.Render(ctx, projectDir, "output.mp4"); err != nil {
		return "", nil, false, fmt.Errorf("render: %w", err)
	}
```

with (note the returns already became `nil, ...` in Task 5):

```go
	renderIssues, err := p.hf.renderer.Render(ctx, projectDir, "output.mp4")
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
```

Then set `renderFlagged` in the success `assembleOutput` literal (from Task 5 Step 6):

```go
	audioFlagged := probeVoiceSilent(voicePath)
	return &assembleOutput{
		mp4Path:        filepath.Join(projectDir, "output.mp4"),
		sceneDurations: boundsToDurations(bounds),
		inspectFlagged: inspectFlagged,
		audioFlagged:   audioFlagged,
		renderFlagged:  len(renderIssues) > 0,
	}, nil
```

Add the `RenderFlagged` field to `ProduceResult` (after `AudioFlagged` from Task 5):

```go
	// RenderFlagged is true when the render emitted browser errors (a silently
	// frozen render). The orchestrator retries once, then routes to needs_review.
	RenderFlagged bool
```

(The `ProduceResult` literal in `ProduceHyperframes916` already sets `RenderFlagged: out.renderFlagged` from Task 5 Step 7.)

- [ ] **Step 7: Wire RenderFlagged into the orchestrator**

In `internal/orchestrator/orchestrator.go` `renderAndFinalize`, after the "reflect durations onto scenes" loop (after line 478, before the visual-QA block), add:

```go
	// A render that emitted browser errors is silently frozen (exits 0, looks fine
	// to still-frame QA). Retry once via the failed-clip tick; if it's still broken
	// after a retry, route to human review instead of publishing.
	if result.RenderFlagged {
		retryCount := 0
		if clip, gErr := o.clipsRepo.GetByID(ctx, clipID); gErr == nil && clip != nil {
			retryCount = clip.RetryCount
		}
		switch producer.RenderGateDecision(true, producer.RenderErrorGateEnabled(), retryCount) {
		case producer.RenderGateRetry:
			return o.failClip(ctx, clipID, fmt.Errorf("render emitted browser errors — retrying"))
		case producer.RenderGateReview:
			status = "needs_review"
			log.Printf("clip %s: render browser errors persisted after retry — status=needs_review (publish blocked)", clipID)
		}
	}
```

(`status` is declared just below at line 481 today — move the `status := "ready"` declaration to ABOVE this block so the `RenderGateReview` case can assign it. Concretely: relocate `status := "ready"` to immediately before this render-gate block, and change the later line 481 to not re-declare it.)

- [ ] **Step 8: Run the full suites + build**

Run: `go test ./internal/producer/ ./internal/orchestrator/`
Expected: PASS. Then `go build ./...` — no errors, and `go vet ./internal/producer/ ./internal/orchestrator/` clean.

- [ ] **Step 9: Commit**

```bash
git add internal/producer/hyperframes.go internal/producer/render_gate_test.go internal/producer/producer.go internal/orchestrator/orchestrator.go
git commit -m "feat(render): catch silently-frozen renders — retry then review (flag-gated)

A render can exit 0 while a JS exception froze every animation; scanBrowserIssues
detected it but only logged, so the broken clip published (still-frame QA can't
see a frozen clip). Render now returns its browser-error lines; with
RENDER_ERROR_GATE_ENABLED a flagged render is retried once, then routed to
needs_review. Flag off = legacy log-only behavior.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Full verification + simplify

**Files:** none (verification only).

- [ ] **Step 1: Run the whole test suite**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 2: Build and vet**

Run: `go build ./... && go vet ./...`
Expected: no errors.

- [ ] **Step 3: Confirm all three flags default off**

Grep the flag helpers and confirm none is enabled by default:

Run: `grep -rn 'RENDER_ERROR_GATE_ENABLED\|TTS_LENGTH_GATE_ENABLED\|QA_AUDIO_CHECK_ENABLED' internal/`
Expected: each appears only in its `os.Getenv(...) == "true"` helper (plus tests) — no default-true wiring.

- [ ] **Step 4: Simplify the diff**

Invoke `/simplify` on the branch diff (user preference: simplify before the final commit). Apply any reductions, re-run `go test ./...`, and commit any changes with message `refactor: simplify reliability-hardening diff`.

- [ ] **Step 5: Smoke test (manual, before deploy)**

Produce one real clip end-to-end on this branch (or a live-test) with all three flags **on**, and confirm a healthy clip is NOT falsely blocked: it renders, uploads (R2), passes the render gate (no browser issues), passes the audio gate (voice present), and reaches `ready`. This is the false-positive guard from the spec's verification checklist.

---

## Self-Review

**Spec coverage:**
- Spec Fix 1 (broken-render gate) → Task 6. ✓
- Spec Fix 2 (R2 fallback) → Task 3. ✓
- Spec Fix 3 (TTS truncation gate) → Task 4. ✓
- Spec Fix 4 (QA audio check) → Task 5. ✓
- Spec Fix 5 (empty-scene QA false positive) → Task 2. ✓
- Spec Fix 6 (retry budget) → Task 1. ✓
- Three flags default-off → Task 7 Step 3 verifies. ✓
- No migration → confirmed (no `migrations/` file created). ✓
- Rollout smoke test (healthy clip not falsely blocked) → Task 7 Step 5. ✓

**Placeholder scan:** No TBD/TODO. Every code step shows complete code. The one intentional forward-reference (Task 5 Step 7 notes `RenderFlagged` is finalized in Task 6) is explicit, not a placeholder.

**Type consistency:**
- `assembleOutput` fields (`mp4Path`, `sceneDurations`, `inspectFlagged`, `audioFlagged`, `renderFlagged`) are used identically in Tasks 5 and 6. ✓
- `Render` returns `([]string, error)` in Task 6 and its sole non-test caller (`AssembleHyperframes916`) is updated in the same task. ✓
- `RenderGateAction` constants (`RenderGateNone/Retry/Review`) match between Task 6 Step 3 (definition), the test (Step 1), and the orchestrator switch (Step 7). ✓
- Flag helper names (`RenderErrorGateEnabled`, `TTSLengthGateEnabled`, `QAAudioCheckEnabled`) match the spec's flag table and their `os.Getenv` strings. ✓
- `ClipsRepo.GetByID` and `models.Clip.RetryCount` used in Task 6 exist (verified: `clips.go:58`, `clip.go:24`). ✓
