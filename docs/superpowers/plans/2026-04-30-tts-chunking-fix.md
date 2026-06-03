# TTS Chunking Fix — แก้เสียงตัดกลางคัน

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** แก้ปัญหา Gemini Flash TTS Preview ตัดเสียงไม่จบสำหรับ voice script ยาว โดยแบ่ง text เป็นท่อนก่อนส่ง TTS แล้วต่อ PCM เข้าด้วยกัน + เพิ่ม validation ความยาวเสียง

**Architecture:** แยก `GenerateVoice` ออกเป็น 3 ขั้น: (1) `splitVoiceText` แบ่ง text ที่จุด `...` สะสมจน ~400 chars/ท่อน (2) `generatePCM` ส่ง API ทีละท่อน return raw PCM bytes (3) ต่อ PCM ทั้งหมด → validate ความยาว → wrap WAV → เขียนไฟล์

**Tech Stack:** Go, OpenRouter TTS API (`google/gemini-3.1-flash-tts-preview`)

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/producer/openrouter.go` | แยก `generateVoiceOnce` → `generatePCM` (return bytes), refactor `GenerateVoice` ให้ chunk + concat, เพิ่ม `splitVoiceText` |
| Create | `internal/producer/openrouter_test.go` | Unit tests สำหรับ `splitVoiceText` |

---

### Task 1: เพิ่มฟังก์ชัน `splitVoiceText` + tests

**Files:**
- Modify: `internal/producer/openrouter.go` (เพิ่มฟังก์ชันใหม่ท้ายไฟล์)
- Create: `internal/producer/openrouter_test.go`

- [ ] **Step 1: สร้าง test file พร้อม test cases สำหรับ `splitVoiceText`**

```go
// internal/producer/openrouter_test.go
package producer

import (
	"testing"
)

func TestSplitVoiceText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxRunes int
		want     []string
	}{
		{
			name:     "short text stays as single chunk",
			text:     "สวัสดีครับ... ผมพูดเรื่องโฆษณา",
			maxRunes: 400,
			want:     []string{"สวัสดีครับ... ผมพูดเรื่องโฆษณา"},
		},
		{
			name:     "empty text returns empty",
			text:     "",
			maxRunes: 400,
			want:     nil,
		},
		{
			name:     "no separator keeps as single chunk",
			text:     "ข้อความยาวมากแต่ไม่มีจุดพัก",
			maxRunes: 400,
			want:     []string{"ข้อความยาวมากแต่ไม่มีจุดพัก"},
		},
		{
			name:     "splits at separator when exceeding max",
			text:     "ส่วนแรกของข้อความที่ยาว... ส่วนที่สองของข้อความ... ส่วนที่สามยาวมาก",
			maxRunes: 30,
			want: []string{
				"ส่วนแรกของข้อความที่ยาว",
				"ส่วนที่สองของข้อความ... ส่วนที่สามยาวมาก",
			},
		},
		{
			name:     "real voice script splits into 2 chunks",
			text:     "คุณต้นถามมาว่า... เช็คแล้วทำไมข้อมูลไม่ขึ้น... เรื่องนี้เจอบ่อย... สาเหตุหลักมีสองจุด... วิธีแก้มีสามขั้น... ขั้นแรก... โหลดส่วนเสริมเข้าเบราว์เซอร์... ขั้นสอง... รอประมาณยี่สิบสี่ชั่วโมง... ขั้นสาม... เข้าไปกดเทสต์อีเวนต์... ติดต่อทีมงานได้เลยครับ",
			maxRunes: 150,
			want: []string{
				"คุณต้นถามมาว่า... เช็คแล้วทำไมข้อมูลไม่ขึ้น... เรื่องนี้เจอบ่อย... สาเหตุหลักมีสองจุด... วิธีแก้มีสามขั้น... ขั้นแรก",
				"โหลดส่วนเสริมเข้าเบราว์เซอร์... ขั้นสอง... รอประมาณยี่สิบสี่ชั่วโมง... ขั้นสาม... เข้าไปกดเทสต์อีเวนต์... ติดต่อทีมงานได้เลยครับ",
			},
		},
		{
			name:     "single segment over max stays as one chunk",
			text:     "ข้อความยาวมากที่ไม่มีจุดพักเลยจนเกิน limit",
			maxRunes: 10,
			want:     []string{"ข้อความยาวมากที่ไม่มีจุดพักเลยจนเกิน limit"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitVoiceText(tc.text, tc.maxRunes)
			if len(got) != len(tc.want) {
				t.Fatalf("splitVoiceText() returned %d chunks, want %d\n  got:  %v\n  want: %v", len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("chunk[%d]\n  got:  %q\n  want: %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
```

- [ ] **Step 2: รัน test เพื่อยืนยันว่า fail (ฟังก์ชันยังไม่มี)**

Run: `cd /Users/jaochai/Code/video-fb && go test ./internal/producer/ -run TestSplitVoiceText -v`
Expected: FAIL — `splitVoiceText` undefined

- [ ] **Step 3: implement `splitVoiceText` ใน `openrouter.go`**

เพิ่มท้ายไฟล์ `internal/producer/openrouter.go`:

```go
const ttsMaxChunkRunes = 400

func splitVoiceText(text string, maxChunkRunes int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len([]rune(text)) <= maxChunkRunes {
		return []string{text}
	}

	const sep = "..."
	parts := strings.Split(text, sep)

	var chunks []string
	var segments []string
	currentLen := 0

	for _, part := range parts {
		partRunes := len([]rune(part))
		sepLen := 0
		if len(segments) > 0 {
			sepLen = len([]rune(sep))
		}

		if currentLen+sepLen+partRunes > maxChunkRunes && len(segments) > 0 {
			chunk := strings.TrimSpace(strings.Join(segments, sep))
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
			segments = []string{part}
			currentLen = partRunes
		} else {
			segments = append(segments, part)
			currentLen += sepLen + partRunes
		}
	}

	if len(segments) > 0 {
		chunk := strings.TrimSpace(strings.Join(segments, sep))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}
```

- [ ] **Step 4: รัน test เพื่อยืนยันว่า pass**

Run: `cd /Users/jaochai/Code/video-fb && go test ./internal/producer/ -run TestSplitVoiceText -v`
Expected: PASS ทุก test case

- [ ] **Step 5: Commit**

```bash
git add internal/producer/openrouter.go internal/producer/openrouter_test.go
git commit -m "feat: add splitVoiceText for TTS chunking"
```

---

### Task 2: Refactor `GenerateVoice` ให้ใช้ chunking + concat PCM

**Files:**
- Modify: `internal/producer/openrouter.go:148-214` (refactor `GenerateVoice` + `generateVoiceOnce`)

- [ ] **Step 1: เปลี่ยน `generateVoiceOnce` เป็น `generatePCM` ที่ return `[]byte`**

แก้ไฟล์ `internal/producer/openrouter.go` — เปลี่ยนชื่อฟังก์ชันและ return type:

```go
func (o *OpenRouterClient) generatePCM(ctx context.Context, text, voice string) ([]byte, error) {
	apiKey, err := o.getAPIKey(ctx)
	if err != nil {
		return nil, err
	}

	reqBody := struct {
		Model          string `json:"model"`
		Input          string `json:"input"`
		Voice          string `json:"voice"`
		ResponseFormat string `json:"response_format"`
	}{
		Model:          "google/gemini-3.1-flash-tts-preview",
		Input:          text,
		Voice:          mapVoice(voice),
		ResponseFormat: "pcm",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal TTS: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openRouterTTSAPI, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create TTS request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}

	pcmData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read TTS audio: %w", err)
	}
	if len(pcmData) == 0 {
		return nil, fmt.Errorf("no audio data received from TTS")
	}

	return pcmData, nil
}
```

- [ ] **Step 2: Refactor `GenerateVoice` ให้ chunk + concat + validate + write**

แทนที่ `GenerateVoice` เดิมด้วย:

```go
func (o *OpenRouterClient) GenerateVoice(ctx context.Context, text, voice, outputPath string) error {
	chunks := splitVoiceText(text, ttsMaxChunkRunes)
	if len(chunks) == 0 {
		return fmt.Errorf("no text to generate voice for")
	}

	if len(chunks) > 1 {
		log.Printf("Splitting voice text into %d chunks for TTS (%d chars total)", len(chunks), len([]rune(text)))
	}

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
			return fmt.Errorf("TTS chunk %d/%d: %w", i+1, len(chunks), err)
		}
		allPCM = append(allPCM, pcm...)
	}

	// Validate audio duration
	const sampleRate = 24000
	const bytesPerSample = 2 // 16-bit mono
	durationSec := float64(len(allPCM)) / float64(sampleRate*bytesPerSample)
	if durationSec < 5.0 && len([]rune(text)) > 100 {
		log.Printf("WARNING: TTS audio unusually short (%.1fs for %d chars) — possible truncation", durationSec, len([]rune(text)))
	}

	wavData := wrapPCMAsWAV(allPCM, sampleRate, 1, 16)

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, wavData, 0644); err != nil {
		return fmt.Errorf("write audio: %w", err)
	}

	log.Printf("Saved TTS audio (%d bytes PCM → %d bytes WAV, %.1fs, %d chunks) to %s",
		len(allPCM), len(wavData), durationSec, len(chunks), outputPath)
	return nil
}
```

- [ ] **Step 3: ลบ `generateVoiceOnce` ที่ไม่ใช้แล้ว**

ลบฟังก์ชัน `generateVoiceOnce` (lines 154-214 เดิม) ทั้งหมด — ถูกแทนที่โดย `generatePCM` + `GenerateVoice` ใหม่แล้ว

- [ ] **Step 4: Build check**

Run: `cd /Users/jaochai/Code/video-fb && go build ./...`
Expected: สำเร็จ ไม่มี compile error

- [ ] **Step 5: รัน tests ทั้งหมด**

Run: `cd /Users/jaochai/Code/video-fb && go test ./...`
Expected: PASS ทุก test

- [ ] **Step 6: Commit**

```bash
git add internal/producer/openrouter.go
git commit -m "fix: chunk TTS text to prevent audio truncation on long scripts"
```

---

### Task 3: Smoke test — ลบ cached voice file แล้ว retry clip ที่เคยถูกตัด

ขั้นตอนนี้ต้องรันบน production (Railway) ไม่ใช่ local

- [ ] **Step 1: Deploy ขึ้น Railway**

Run: `cd /Users/jaochai/Code/video-fb && git push origin master`
(GitHub Actions จะ auto-deploy ไป Railway)

- [ ] **Step 2: ตรวจว่า deploy สำเร็จ**

ดู GitHub Actions ว่ารัน workflow สำเร็จ

- [ ] **Step 3: Retry clip ที่มีปัญหาผ่าน API**

หลัง deploy สำเร็จ ต้องลบ cached `voice.wav` ของ clip เก่า (อยู่ใน work directory บน Railway) แล้ว retry — หรือสร้าง clip ใหม่เพื่อทดสอบ

- [ ] **Step 4: ตรวจ log ว่า chunking ทำงาน**

ดู Railway logs ว่ามี message:
- `Splitting voice text into N chunks for TTS`
- `Generating TTS chunk 1/N`
- `Saved TTS audio (... N chunks)`

- [ ] **Step 5: ฟังเสียงในวีดีโอว่าเนื้อหาครบไม่ตัด**

เปิดวีดีโอที่สร้างใหม่ ฟังว่า:
- ✅ มีครบทุกขั้นตอน (ขั้นแรก → ขั้นสอง → ขั้นสาม)
- ✅ มี CTA ปิดท้าย (ติดต่อแอดส์แวนซ์...)
- ✅ เสียงไม่ตัดกลางคัน
- ✅ transition ระหว่างท่อนฟังธรรมชาติ
