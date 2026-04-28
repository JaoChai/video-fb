# OpenRouter Image + TTS Migration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Kie AI image generation and voice (ElevenLabs) with OpenRouter APIs — using the OpenRouter key already in Settings.

**Architecture:** Create a new `openrouter.go` client in the producer package that handles image generation (chat completions + modalities) and TTS (streaming SSE + base64 audio). Producer.go switches from calling kie.GenerateImage/GenerateVoice to calling the new OpenRouter client. Kie AI is retained for file upload only.

**Tech Stack:** Go, OpenRouter API (chat completions), SSE streaming, base64 decode

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/producer/openrouter.go` | **Create** | OpenRouter client: image generation + TTS |
| `internal/producer/producer.go` | **Modify** | Switch GenerateImage/GenerateVoice calls to OpenRouter |
| `internal/producer/kieai.go` | **Keep** | Still used for file upload (UploadFile) |

---

### Task 1: Create OpenRouter client struct

**Files:**
- Create: `internal/producer/openrouter.go`

- [ ] **Step 1: Create the client struct and constructor**

```go
package producer

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const openRouterAPI = "https://openrouter.ai/api/v1/chat/completions"

type OpenRouterClient struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewOpenRouterClient(pool *pgxpool.Pool) *OpenRouterClient {
	return &OpenRouterClient{
		pool:   pool,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (o *OpenRouterClient) getAPIKey(ctx context.Context) (string, error) {
	var key string
	err := o.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'openrouter_api_key'`).Scan(&key)
	if err != nil {
		return "", fmt.Errorf("get openrouter_api_key from settings: %w", err)
	}
	if key == "" {
		return "", fmt.Errorf("openrouter_api_key is empty — set it in Settings page")
	}
	return key, nil
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`

---

### Task 2: Implement image generation via OpenRouter

**Files:**
- Modify: `internal/producer/openrouter.go`

- [ ] **Step 1: Add request/response types and GenerateImage method**

```go
import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type openRouterRequest struct {
	Model      string            `json:"model"`
	Messages   []orMessage       `json:"messages"`
	Modalities []string          `json:"modalities"`
	ImageConfig *orImageConfig   `json:"image_config,omitempty"`
}

type orMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type orImageConfig struct {
	AspectRatio string `json:"aspect_ratio,omitempty"`
	ImageSize   string `json:"image_size,omitempty"`
}

type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			Images  []struct {
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"images"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (o *OpenRouterClient) GenerateImage(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	apiKey, err := o.getAPIKey(ctx)
	if err != nil {
		return err
	}

	reqBody := openRouterRequest{
		Model: "openai/gpt-5.4-image-2",
		Messages: []orMessage{
			{Role: "user", Content: prompt},
		},
		Modalities:  []string{"image", "text"},
		ImageConfig: &orImageConfig{AspectRatio: aspectRatio, ImageSize: "2K"},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openRouterAPI, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}

	var result openRouterResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if result.Error != nil {
		return fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 || len(result.Choices[0].Message.Images) == 0 {
		return fmt.Errorf("no images in response")
	}

	dataURL := result.Choices[0].Message.Images[0].ImageURL.URL
	return saveBase64Image(dataURL, outputPath)
}

func saveBase64Image(dataURL, outputPath string) error {
	// Format: "data:image/png;base64,iVBORw0KGgo..."
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data URL format")
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}

	os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err := os.WriteFile(outputPath, decoded, 0644); err != nil {
		return fmt.Errorf("write image: %w", err)
	}

	log.Printf("Saved image (%d bytes) to %s", len(decoded), outputPath)
	return nil
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`

---

### Task 3: Implement TTS via OpenRouter (streaming SSE)

**Files:**
- Modify: `internal/producer/openrouter.go`

- [ ] **Step 1: Add GenerateVoice method with SSE streaming**

```go
type openRouterTTSRequest struct {
	Model      string      `json:"model"`
	Messages   []orMessage `json:"messages"`
	Modalities []string    `json:"modalities"`
	Audio      *orAudio    `json:"audio"`
	Stream     bool        `json:"stream"`
}

type orAudio struct {
	Voice  string `json:"voice"`
	Format string `json:"format"`
}

func (o *OpenRouterClient) GenerateVoice(ctx context.Context, text, voice, outputPath string) error {
	apiKey, err := o.getAPIKey(ctx)
	if err != nil {
		return err
	}

	// Map ElevenLabs voice names to OpenRouter voices
	orVoice := mapVoice(voice)

	reqBody := openRouterTTSRequest{
		Model: "google/gemini-3.1-flash-tts-preview",
		Messages: []orMessage{
			{Role: "user", Content: text},
		},
		Modalities: []string{"text", "audio"},
		Audio:      &orAudio{Voice: orVoice, Format: "mp3"},
		Stream:     true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal TTS request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openRouterAPI, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create TTS request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TTS HTTP %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}

	return parseSSEAudio(resp.Body, outputPath)
}

func parseSSEAudio(reader io.Reader, outputPath string) error {
	var audioChunks []string
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Audio struct {
						Data string `json:"data"`
					} `json:"audio"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Audio.Data != "" {
			audioChunks = append(audioChunks, chunk.Choices[0].Delta.Audio.Data)
		}
	}

	if len(audioChunks) == 0 {
		return fmt.Errorf("no audio data received from TTS")
	}

	fullBase64 := strings.Join(audioChunks, "")
	audioBytes, err := base64.StdEncoding.DecodeString(fullBase64)
	if err != nil {
		return fmt.Errorf("decode TTS audio: %w", err)
	}

	os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err := os.WriteFile(outputPath, audioBytes, 0644); err != nil {
		return fmt.Errorf("write audio file: %w", err)
	}

	log.Printf("Saved TTS audio (%d bytes) to %s", len(audioBytes), outputPath)
	return nil
}

func mapVoice(elevenLabsVoice string) string {
	// Map common ElevenLabs voices to OpenRouter/Gemini voices
	mapping := map[string]string{
		"adam":    "onyx",
		"daniel":  "echo",
		"rachel":  "nova",
		"sarah":   "shimmer",
		"charlie": "fable",
		"laura":   "alloy",
	}
	if v, ok := mapping[strings.ToLower(elevenLabsVoice)]; ok {
		return v
	}
	return "alloy"
}
```

- [ ] **Step 2: Add `bufio` to imports**

Make sure the import section includes `bufio`.

- [ ] **Step 3: Build check**

Run: `go build ./...`

---

### Task 4: Add retry wrapper for OpenRouter calls

**Files:**
- Modify: `internal/producer/openrouter.go`

- [ ] **Step 1: Add retryable wrapper that reuses kieai.go patterns**

```go
func (o *OpenRouterClient) retryableCall(ctx context.Context, operation string, fn func() error) error {
	const maxRetries = 5
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 30 * time.Second
			log.Printf("[retry] %s attempt %d/%d after %v (error: %v)", operation, attempt, maxRetries, backoff, lastErr)
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("%s cancelled: %w", operation, ctx.Err())
			case <-timer.C:
			}
		}
		lastErr = fn()
		if lastErr == nil {
			if attempt > 0 {
				log.Printf("[retry] %s succeeded on attempt %d", operation, attempt)
			}
			return nil
		}
		if !isRetryable(lastErr) {
			return lastErr
		}
	}
	return fmt.Errorf("%s failed after %d retries: %w", operation, maxRetries, lastErr)
}
```

- [ ] **Step 2: Wrap GenerateImage and GenerateVoice with retry**

Change `GenerateImage` to wrap the core logic:

```go
func (o *OpenRouterClient) GenerateImage(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	return o.retryableCall(ctx, "openrouter-image", func() error {
		return o.generateImageOnce(ctx, prompt, aspectRatio, outputPath)
	})
}
```

Rename the current implementation to `generateImageOnce`. Same pattern for `GenerateVoice` → `generateVoiceOnce`.

- [ ] **Step 3: Build check**

Run: `go build ./...`

---

### Task 5: Wire OpenRouter into Producer

**Files:**
- Modify: `internal/producer/producer.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add OpenRouterClient to Producer struct**

In `producer.go`, add the field:

```go
type Producer struct {
	kie          *KieClient
	openRouter   *OpenRouterClient
	ffmpeg       *FFmpegAssembler
	defaultVoice string
	workDir      string
	tracker      *progress.Tracker
}

func NewProducer(kie *KieClient, openRouter *OpenRouterClient, ffmpeg *FFmpegAssembler, voice, workDir string, tracker *progress.Tracker) *Producer {
	os.MkdirAll(workDir, 0755)
	return &Producer{kie: kie, openRouter: openRouter, ffmpeg: ffmpeg, defaultVoice: voice, workDir: workDir, tracker: tracker}
}
```

- [ ] **Step 2: Replace Kie AI calls with OpenRouter in Produce()**

In the voice step, change:

```go
// Before:
if err := p.kie.GenerateVoice(ctx, voiceScript, voice, voicePath); err != nil {
// After:
if err := p.openRouter.GenerateVoice(ctx, voiceScript, voice, voicePath); err != nil {
```

In the image steps, change:

```go
// Before:
if err := p.kie.GenerateImage(ctx, prompt.ImagePrompt169, "16:9", img169); err != nil {
// After:
if err := p.openRouter.GenerateImage(ctx, prompt.ImagePrompt169, "16:9", img169); err != nil {
```

Same for the 9:16 image call.

- [ ] **Step 3: Update main.go to create OpenRouterClient**

In `cmd/server/main.go`, find where Producer is created and add:

```go
orClient := producer.NewOpenRouterClient(pool)
prod := producer.NewProducer(kieClient, orClient, ffmpegAssembler, cfg.Voice, workDir, tracker)
```

- [ ] **Step 4: Build check**

Run: `go build ./...`

---

### Task 6: Update voice settings and ValidVoices

**Files:**
- Modify: `internal/producer/producer.go`

- [ ] **Step 1: Update ValidVoices to include OpenRouter voices**

```go
var ValidVoices = map[string]bool{
	// OpenRouter / Gemini TTS voices
	"alloy": true, "echo": true, "fable": true,
	"onyx": true, "nova": true, "shimmer": true,
	// Legacy ElevenLabs voices (mapped via mapVoice)
	"rachel": true, "aria": true, "roger": true, "sarah": true, "laura": true,
	"charlie": true, "george": true, "callum": true, "river": true, "liam": true,
	"charlotte": true, "alice": true, "matilda": true, "will": true, "jessica": true,
	"eric": true, "chris": true, "brian": true, "daniel": true, "lily": true,
	"bill": true, "adam": true,
}
```

- [ ] **Step 2: Update default voice in getVoice()**

Change the fallback from `"Daniel"` to `"alloy"`:

```go
func (p *Producer) getVoice(ctx context.Context) string {
	// ... existing DB + defaultVoice logic ...
	return "alloy"
}
```

- [ ] **Step 3: Build check**

Run: `go build ./...`

---

### Task 7: Verify, commit, and push

- [ ] **Step 1: Full build**

Run: `go build ./...`

- [ ] **Step 2: Review diff**

Run: `git diff --stat`

Expected files changed:
- `internal/producer/openrouter.go` (new)
- `internal/producer/producer.go` (modified)
- `cmd/server/main.go` (modified)

- [ ] **Step 3: Commit and push**

```bash
git add internal/producer/openrouter.go internal/producer/producer.go cmd/server/main.go
git commit -m "feat: migrate image + voice generation from Kie AI to OpenRouter

Image generation now uses OpenRouter (openai/gpt-5.4-image-2) with
base64 response — no more task polling. Voice uses Gemini 3.1 Flash
TTS via SSE streaming. Both use the existing OpenRouter API key.

Kie AI is retained for file upload only.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
git push origin master
```

---

### Task 8: Update Settings page voice options (optional follow-up)

**Files:**
- Modify: frontend Settings page

- [ ] **Step 1: Update voice dropdown to show OpenRouter voice names**

Add the 6 OpenRouter voices (alloy, echo, fable, onyx, nova, shimmer) as primary options, keep ElevenLabs voices as "legacy" that get auto-mapped.

- [ ] **Step 2: Build frontend**

Run: `cd frontend && npm run build`
