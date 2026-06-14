# Visual QA Agent Implementation Plan (Phase 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After a clip renders, look at the actual frames the viewer will see and **block auto-publish** when a scene is visually broken. Detect-and-block only — no auto-fix, no re-render, ever.

**Architecture:** A new `VisualQAAgent` (same pattern as `SceneAgent` / `CriticAgent`) runs in `orchestrator.produceClipWithID` *after* `producer.ProduceHyperframes916` returns and *before* the `status` decision. The orchestrator extracts one frame per scene from the local MP4 (`ExtractFrameAt`), sends each frame to a vision-capable Claude model, and asks "is anything visually broken?". A pure `summarizeVerdicts` function decides clip-level pass/fail (fail if ANY scene fails). Pass → `status='ready'` (publishable). Fail → `status='needs_review'` (the publisher's `WHERE c.status = 'ready'` gate skips it). Every run is appended to a new `visual_qa` table. **Fail-safe is inverted vs Phase 1:** infra/decode/LLM errors are treated as OK (do NOT block on infra), so a flaky vision call never strands a good clip in review — only a *confident visual defect* blocks.

**Tech Stack:** Go, pgx/Postgres (Neon), ffmpeg frame extraction, the `KieLLMClient` Claude path extended with a vision body, DB-driven `agent_configs`.

> **Migration numbers are provisional.** This plan uses `035`/`036`. Phase 3 (learning loop) reserves `037`. If either phase is implemented while the repo's latest migration has moved past `034`, renumber to the next unused number — migrations are tracked by filename, gaps are harmless.

**The single unknown this plan de-risks first:** it is currently UNVERIFIED whether the kie.ai `/claude/v1/messages` proxy forwards Anthropic image content blocks. **Task 1 is a smoke test that gates everything else.** If Task 1 fails, STOP — the detect-via-vision approach is blocked and must be escalated, not worked around.

---

## File Structure

- **Create** `cmd/visionsmoke/main.go` — throwaway vision smoke-test probe (Task 1; deleted in Task 8).
- **Modify** `internal/agent/kiellm.go` — add the vision request/message structs, `buildClaudeVisionBody`, and `GenerateVisionJSON` (reuses `parseClaudeText`).
- **Create** `internal/agent/kiellm_vision_test.go` — pure test for `buildClaudeVisionBody` marshalling shape.
- **Modify** `internal/producer/ffmpeg.go` — add `ExtractFrameAt`.
- **Create** `internal/agent/visualqa.go` — `VisualQAAgent`, input/output types, `summarizeVerdicts`, `Review`.
- **Create** `internal/agent/visualqa_test.go` — pure tests for `summarizeVerdicts` + schema unmarshal.
- **Create** `internal/repository/visualqa.go` — `VisualQARepo.Create` (append-only insert).
- **Create** `migrations/035_visual_qa.sql` — the log table.
- **Create** `migrations/036_visual_qa_agent_config.sql` — seed the `visual_qa` agent_configs row.
- **Modify** `internal/producer/producer.go` — add `LocalVideo916Path` to `ProduceResult` and populate it.
- **Modify** `internal/orchestrator/orchestrator.go` — add `visualQAAgent` + `visualQARepo` fields, extend `New(...)`, insert the QA block between the producer return and the status decision.
- **Modify** `cmd/server/main.go` — construct `NewVisualQAAgent` + `NewVisualQARepo`, pass to `orchestrator.New`.

**The ONLY publish-gate change:** when QA fails, `status='needs_review'` instead of `'ready'`. Nothing else in the publisher changes — its existing `WHERE c.status = 'ready'` already excludes any other status.

---

## Task 1: Vision smoke-test spike — de-risk the unknown FIRST (gates the rest)

**Files:**
- Create: `cmd/visionsmoke/main.go`

The kie.ai Claude proxy is known to forward plain-text Messages bodies (every current agent uses it). It is **UNKNOWN** whether it forwards Anthropic *image* content blocks. Verify with one real call before building anything on it. This probe embeds a 1×1 red PNG (no file I/O, no fixtures needed), POSTs a vision body to `kieClaudeAPI` with `claude-sonnet-4-6`, and asserts HTTP 200 + non-empty text.

- [ ] **Step 1: Write the probe**

Create `cmd/visionsmoke/main.go`. The `apiKey` comes from the `KIE_API_KEY` env var (the probe is standalone and does not touch the DB).

```go
// Command visionsmoke is a throwaway probe that verifies the kie.ai
// /claude/v1/messages proxy forwards Anthropic image content blocks.
// Run: KIE_API_KEY=sk-... go run ./cmd/visionsmoke
// It is deleted in Task 8 once the result is recorded. This GATES Phase 2:
// if it does not print "VISION OK", the detect-via-vision approach is blocked.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// 1x1 red PNG, base64 (no file needed).
const onePxRedPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func main() {
	apiKey := os.Getenv("KIE_API_KEY")
	if apiKey == "" {
		fmt.Println("SKIP: set KIE_API_KEY to run the vision smoke test")
		os.Exit(2)
	}

	// Anthropic Messages API vision shape: content is an ARRAY of blocks —
	// one text block plus one base64 image block.
	body := map[string]any{
		"model":      "claude-sonnet-4-6",
		"max_tokens": 64,
		"stream":     false,
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "Reply with the single word RED if this image is a solid red square."},
					map[string]any{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": "image/png",
							"data":       onePxRedPNGBase64,
						},
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", "https://api.kie.ai/claude/v1/messages", bytes.NewReader(raw))
	if err != nil {
		fmt.Println("FAIL: build request:", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("FAIL: send request:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("FAIL: HTTP %d: %s\n", resp.StatusCode, truncate(string(respBody), 600))
		os.Exit(1)
	}

	// Parse the same shape parseClaudeText expects: content[].text.
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		fmt.Printf("FAIL: unparseable 200 body: %v\nraw: %s\n", err, truncate(string(respBody), 600))
		os.Exit(1)
	}
	if parsed.Error != nil {
		fmt.Println("FAIL: claude error:", parsed.Error.Message)
		os.Exit(1)
	}
	var text string
	for _, c := range parsed.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	if text == "" {
		fmt.Printf("FAIL: 200 but no text content\nraw: %s\n", truncate(string(respBody), 600))
		os.Exit(1)
	}
	fmt.Printf("VISION OK: model replied %q\n", truncate(text, 120))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
```

- [ ] **Step 2: Confirm it compiles**

Run: `go build ./cmd/visionsmoke/`
Expected: build OK (exit 0).

- [ ] **Step 3: Run the probe against the real proxy**

Run (substitute the real key — the same value stored in `settings.kie_api_key`):
```bash
KIE_API_KEY="<kie_api_key>" go run ./cmd/visionsmoke
```
Expected: a line beginning `VISION OK:` (e.g. `VISION OK: model replied "RED"`).

**GATE — this decides whether Phase 2 proceeds:**
- `VISION OK` → the kie.ai proxy forwards image blocks. Continue to Task 2.
- `FAIL: HTTP 4xx/5xx` or `claude error` mentioning images/content → **STOP.** The proxy does not accept vision content. Do NOT attempt Tasks 2-8. Report the exact HTTP status + body and escalate: the detect-via-vision design is blocked and needs a different vision transport (a decision the user must make).
- `SKIP` → the key env var was not set; set it and re-run. Do not proceed on a SKIP.

- [ ] **Step 4: Commit (record the spike; it is removed in Task 8)**

```bash
git add cmd/visionsmoke/main.go
git commit -m "spike(vision): kie.ai claude proxy vision smoke test (Phase 2 gate)"
```

---

## Task 2: `GenerateVisionJSON` on KieLLMClient

**Files:**
- Modify: `internal/agent/kiellm.go`
- Test: `internal/agent/kiellm_vision_test.go`

The existing `kieClaudeMessage.Content` is a plain `string`, so it cannot carry image blocks. Add a parallel vision request/message pair whose `Content` is `[]any`, plus a `buildClaudeVisionBody`, plus a `GenerateVisionJSON` that POSTs to `kieClaudeAPI` and reuses `parseClaudeText` + `extractJSON` (already in this package).

- [ ] **Step 1: Write the failing test for the body shape**

Create `internal/agent/kiellm_vision_test.go`:

```go
package agent

import (
	"encoding/json"
	"testing"
)

func TestBuildClaudeVisionBody_Shape(t *testing.T) {
	imgs := [][]byte{{0x89, 0x50, 0x4e, 0x47}} // fake PNG bytes
	raw, err := buildClaudeVisionBody("claude-sonnet-4-6", "SYS", "USER", 0.2, imgs, kieLLMMaxTokens)
	if err != nil {
		t.Fatalf("buildClaudeVisionBody error: %v", err)
	}

	var parsed struct {
		Model     string `json:"model"`
		System    string `json:"system"`
		MaxTokens int    `json:"max_tokens"`
		Stream    bool   `json:"stream"`
		Messages  []struct {
			Role    string `json:"role"`
			Content []struct {
				Type   string `json:"type"`
				Text   string `json:"text"`
				Source *struct {
					Type      string `json:"type"`
					MediaType string `json:"media_type"`
					Data      string `json:"data"`
				} `json:"source"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("body not valid JSON: %v\nraw: %s", err, raw)
	}

	if parsed.Model != "claude-sonnet-4-6" || parsed.System != "SYS" || parsed.Stream != false {
		t.Fatalf("scalar fields wrong: %+v", parsed)
	}
	if len(parsed.Messages) != 1 || parsed.Messages[0].Role != "user" {
		t.Fatalf("want 1 user message, got %+v", parsed.Messages)
	}
	content := parsed.Messages[0].Content
	if len(content) != 2 {
		t.Fatalf("want 2 content blocks (text + image), got %d", len(content))
	}
	if content[0].Type != "text" || content[0].Text != "USER" {
		t.Errorf("block 0 should be the text prompt, got %+v", content[0])
	}
	if content[1].Type != "image" || content[1].Source == nil {
		t.Fatalf("block 1 should be an image with a source, got %+v", content[1])
	}
	if content[1].Source.Type != "base64" || content[1].Source.MediaType != "image/png" {
		t.Errorf("image source shape wrong: %+v", content[1].Source)
	}
	if content[1].Source.Data == "" {
		t.Errorf("image data not base64-encoded")
	}
}

func TestBuildClaudeVisionBody_MultipleImages(t *testing.T) {
	raw, err := buildClaudeVisionBody("claude-sonnet-4-6", "", "U", 0, [][]byte{{1}, {2}, {3}}, 100)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var parsed struct {
		Messages []struct {
			Content []struct {
				Type string `json:"type"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	// 1 text block + 3 image blocks.
	if got := len(parsed.Messages[0].Content); got != 4 {
		t.Fatalf("want 4 content blocks, got %d", got)
	}
}
```

- [ ] **Step 2: Add the vision structs + builder + GenerateVisionJSON**

In `internal/agent/kiellm.go`, after the existing `buildClaudeBody` function (right after its closing `}` near line 51), insert:

```go
// kieClaudeVisionMessage mirrors kieClaudeMessage but Content is an array of
// blocks (text + image) instead of a plain string, as the Anthropic Messages
// API requires for vision.
type kieClaudeVisionMessage struct {
	Role    string `json:"role"`
	Content []any  `json:"content"`
}

// kieClaudeVisionRequest mirrors kieClaudeRequest with vision messages.
type kieClaudeVisionRequest struct {
	Model       string                   `json:"model"`
	System      string                   `json:"system,omitempty"`
	MaxTokens   int                      `json:"max_tokens"`
	Stream      bool                     `json:"stream"`
	Temperature float64                  `json:"temperature,omitempty"`
	Messages    []kieClaudeVisionMessage `json:"messages"`
}

// buildClaudeVisionBody builds a Claude Messages body whose single user turn
// carries the text prompt followed by one base64 image block per frame. All
// frames are treated as image/png (ExtractFrameAt writes PNG).
func buildClaudeVisionBody(model, system, user string, temp float64, images [][]byte, maxTokens int) ([]byte, error) {
	content := make([]any, 0, len(images)+1)
	content = append(content, map[string]any{"type": "text", "text": user})
	for _, img := range images {
		content = append(content, map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": "image/png",
				"data":       base64.StdEncoding.EncodeToString(img),
			},
		})
	}
	return json.Marshal(kieClaudeVisionRequest{
		Model:       model,
		System:      system,
		MaxTokens:   maxTokens,
		Stream:      false,
		Temperature: temp,
		Messages:    []kieClaudeVisionMessage{{Role: "user", Content: content}},
	})
}
```

Then add `GenerateVisionJSON` at the end of the file (after `GenerateJSON`). It mirrors the Claude branch of `generate` but uses the vision body, and reuses `parseClaudeText` + `extractJSON` + the retry shape of `GenerateJSON`:

```go
// GenerateVisionJSON sends a vision request (one text prompt + N PNG frames) to
// the Claude proxy and unmarshals the (fence-stripped) JSON reply into target.
// Vision is Claude-only; a non claude-* model is rejected. Retries with lower
// temperature on JSON parse failure, like GenerateJSON.
func (c *KieLLMClient) GenerateVisionJSON(ctx context.Context, model, system, user string, temp float64, images [][]byte, target any) error {
	if provider, err := providerForModel(model); err != nil {
		return err
	} else if provider != "claude" {
		return fmt.Errorf("vision requires a claude-* model, got %q", model)
	}
	apiKey, err := c.getAPIKey(ctx)
	if err != nil {
		return err
	}

	const maxRetries = 2
	var lastErr error
	for attempt := range maxRetries + 1 {
		t := temp
		if attempt > 0 {
			t = max(0, temp-0.2*float64(attempt))
			log.Printf("KieLLM vision JSON retry %d/%d (temperature: %.2f)", attempt, maxRetries, t)
		}

		reqBody, err := buildClaudeVisionBody(model, system, user, t, images, kieLLMMaxTokens)
		if err != nil {
			return fmt.Errorf("build claude vision request: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, "POST", kieClaudeAPI, bytes.NewReader(reqBody))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := c.client.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("kie claude vision HTTP %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
		}

		text, err := parseClaudeText(respBody)
		if err != nil {
			return err
		}
		cleaned := extractJSON(text)
		if err := json.Unmarshal([]byte(cleaned), target); err != nil {
			lastErr = fmt.Errorf("parse vision JSON from KieLLM: %w\nraw: %s", err, text[:min(len(text), 300)])
			continue
		}
		return nil
	}
	return lastErr
}
```

All identifiers used (`base64`, `bytes`, `io`, `log`, `http`, `fmt`, `json`, `providerForModel`, `getAPIKey`, `parseClaudeText`, `extractJSON`, `kieClaudeAPI`, `kieLLMMaxTokens`, `c.client`) already exist in `kiellm.go` — note `encoding/base64` is NOT yet imported; add it.

- [ ] **Step 3: Add the `encoding/base64` import**

In the import block at the top of `internal/agent/kiellm.go`, add `"encoding/base64"` (keep imports grouped/sorted; it goes just before `"encoding/json"`).

- [ ] **Step 4: Build + run the body tests**

Run: `go build ./internal/agent/ && go test ./internal/agent/ -run 'TestBuildClaudeVisionBody' -v`
Expected: build OK, both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/kiellm.go internal/agent/kiellm_vision_test.go
git commit -m "feat(agent): GenerateVisionJSON + claude vision body on KieLLMClient"
```

---

## Task 3: `ExtractFrameAt` ffmpeg method

**Files:**
- Modify: `internal/producer/ffmpeg.go`

Generalize the existing `ExtractThumbnail` (which hard-codes `-ss 1.5`) into a seek-to-arbitrary-timestamp variant. Same flags otherwise: `-frames:v 1 -update 1 -y`.

- [ ] **Step 1: Add the method**

In `internal/producer/ffmpeg.go`, after `ExtractThumbnail` (after its closing `}` at the end of the file), add:

```go
// ExtractFrameAt writes a single PNG frame from videoPath at tsSeconds into the
// timeline. It is the generalized form of ExtractThumbnail (which is fixed at
// 1.5s). Used by Visual QA to grab one representative frame per scene. A
// negative tsSeconds is clamped to 0.
func (f *FFmpegAssembler) ExtractFrameAt(videoPath, outPath string, tsSeconds float64) error {
	if tsSeconds < 0 {
		tsSeconds = 0
	}
	os.MkdirAll(filepath.Dir(outPath), 0755)
	args := []string{"-ss", fmt.Sprintf("%.3f", tsSeconds), "-i", videoPath, "-frames:v", "1", "-update", "1", "-y", outPath}
	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extract frame at %.3fs failed: %w", tsSeconds, err)
	}
	return nil
}
```

All imports (`os`, `os/exec`, `path/filepath`, `fmt`) are already present in `ffmpeg.go`.

- [ ] **Step 2: Verify it builds**

Run: `go build ./internal/producer/`
Expected: build OK.

(No unit test: `ExtractFrameAt` shells out to ffmpeg and needs a real MP4. It is exercised end-to-end at render time. **Manual check at implementation time:** against any local rendered clip dir, e.g.
`ffmpeg -ss 3.0 -i /tmp/adsvance-output/<clipID>/composition-916/output.mp4 -frames:v 1 -update 1 -y /tmp/frame.png` and confirm `/tmp/frame.png` is a real content frame.)

- [ ] **Step 3: Commit**

```bash
git add internal/producer/ffmpeg.go
git commit -m "feat(producer): ExtractFrameAt for per-scene frame grabs"
```

---

## Task 4: VisualQAAgent types + `summarizeVerdicts` (pure, TDD) + `Review`

**Files:**
- Create: `internal/agent/visualqa.go`
- Test: `internal/agent/visualqa_test.go`

`summarizeVerdicts` is the pure decision: the clip passes iff **every** scene verdict is `OK=true`. The fail-safe lives in `Review`: any per-scene vision/decode error is recorded as an `OK=true` verdict (with the error noted in `Issues` for the log) so infra hiccups never block publish — only a confident visual defect (`OK=false`) does.

- [ ] **Step 1: Write the types + the pure summarizer + Review**

Create `internal/agent/visualqa.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jaochai/video-fb/internal/models"
)

// QAFrame is one extracted scene frame plus the metadata the model needs to
// judge it (the on-screen text it SHOULD show, and the voice line for context).
type QAFrame struct {
	SceneNumber  int
	PNG          []byte
	OnScreenText string
	VoiceText    string
}

// VisualQAInput is the per-clip QA request: the question (topic context) plus
// one frame per scene.
type VisualQAInput struct {
	Question string
	Frames   []QAFrame
}

// SceneVerdict is the model's judgement for one scene. OK=false means a
// confident visual defect that should block auto-publish. Issues are the
// human-readable reasons (also written to the visual_qa log).
type SceneVerdict struct {
	SceneNumber int      `json:"scene_number"`
	OK          bool     `json:"ok"`
	Issues      []string `json:"issues"`
}

// visionVerdict is the raw single-frame JSON the model returns (one scene).
type visionVerdict struct {
	OK     bool     `json:"ok"`
	Issues []string `json:"issues"`
}

// VisualQAResult is what Review hands back to the orchestrator: per-scene
// verdicts plus the clip-level Passed decision.
type VisualQAResult struct {
	Verdicts []SceneVerdict
	Passed   bool
}

// summarizeVerdicts is the pure clip-level decision: the clip passes iff every
// scene verdict is OK. An empty verdict slice passes (nothing to block on —
// fail-open, consistent with the infra-error policy).
func summarizeVerdicts(verdicts []SceneVerdict) bool {
	for _, v := range verdicts {
		if !v.OK {
			return false
		}
	}
	return true
}

// VisualQATemplateData fills the seeded `visual_qa` prompt_template for one
// scene/frame.
type VisualQATemplateData struct {
	Question     string
	SceneNumber  int
	OnScreenText string
	VoiceText    string
}

// VisualQAAgent looks at one rendered frame per scene and flags visual defects.
// Runs on a vision-capable Claude model (cfg.Model = claude-sonnet-4-6).
type VisualQAAgent struct {
	llm *KieLLMClient
}

func NewVisualQAAgent(llm *KieLLMClient) *VisualQAAgent {
	return &VisualQAAgent{llm: llm}
}

// Review judges every frame and returns per-scene verdicts + the clip decision.
// It NEVER blocks on infrastructure: a template/vision/decode error for a scene
// is logged and recorded as an OK verdict (with the error in Issues), so only a
// confident visual defect (model says ok=false) can fail the clip. cfg is the
// `visual_qa` AgentConfig fetched by the caller via GetByName.
func (a *VisualQAAgent) Review(ctx context.Context, in VisualQAInput, cfg *models.AgentConfig) VisualQAResult {
	verdicts := make([]SceneVerdict, 0, len(in.Frames))
	for _, f := range in.Frames {
		verdicts = append(verdicts, a.reviewFrame(ctx, in.Question, f, cfg))
	}
	return VisualQAResult{Verdicts: verdicts, Passed: summarizeVerdicts(verdicts)}
}

// reviewFrame judges a single frame. On ANY error it returns an OK verdict
// (fail-open) annotated with the error, never blocking publish on infra.
func (a *VisualQAAgent) reviewFrame(ctx context.Context, question string, f QAFrame, cfg *models.AgentConfig) SceneVerdict {
	ok := func(note string) SceneVerdict {
		var issues []string
		if note != "" {
			issues = []string{note}
		}
		return SceneVerdict{SceneNumber: f.SceneNumber, OK: true, Issues: issues}
	}

	if len(f.PNG) == 0 {
		log.Printf("visualqa: scene %d has no frame bytes — treating as OK (fail-open)", f.SceneNumber)
		return ok("no frame extracted (skipped)")
	}

	userPrompt, err := renderTemplate(cfg.PromptTemplate, VisualQATemplateData{
		Question:     question,
		SceneNumber:  f.SceneNumber,
		OnScreenText: f.OnScreenText,
		VoiceText:    f.VoiceText,
	})
	if err != nil {
		log.Printf("visualqa: scene %d template error (fail-open): %v", f.SceneNumber, err)
		return ok(fmt.Sprintf("template error: %v", err))
	}

	var out visionVerdict
	if err := a.llm.GenerateVisionJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, [][]byte{f.PNG}, &out); err != nil {
		log.Printf("visualqa: scene %d vision error (fail-open): %v", f.SceneNumber, err)
		return ok(fmt.Sprintf("vision error: %v", err))
	}
	return SceneVerdict{SceneNumber: f.SceneNumber, OK: out.OK, Issues: out.Issues}
}

// MarshalVerdicts is a small helper for the orchestrator to JSON-encode verdicts
// for the visual_qa.issues column without importing encoding/json there twice.
func MarshalVerdicts(verdicts []SceneVerdict) []byte {
	b, err := json.Marshal(verdicts)
	if err != nil {
		return []byte("[]")
	}
	return b
}
```

- [ ] **Step 2: Write the pure tests**

Create `internal/agent/visualqa_test.go`. `summarizeVerdicts` and `MarshalVerdicts` are pure — no LLM mock needed. The schema test locks the prompt↔struct contract for the single-frame reply.

```go
package agent

import (
	"encoding/json"
	"testing"
)

func TestSummarize_AllOK_Passes(t *testing.T) {
	v := []SceneVerdict{
		{SceneNumber: 1, OK: true},
		{SceneNumber: 2, OK: true},
	}
	if !summarizeVerdicts(v) {
		t.Fatal("want pass when every scene OK")
	}
}

func TestSummarize_AnyFail_Fails(t *testing.T) {
	v := []SceneVerdict{
		{SceneNumber: 1, OK: true},
		{SceneNumber: 2, OK: false, Issues: []string{"caption overflow"}},
		{SceneNumber: 3, OK: true},
	}
	if summarizeVerdicts(v) {
		t.Fatal("want fail when any scene not OK")
	}
}

func TestSummarize_Empty_Passes(t *testing.T) {
	if !summarizeVerdicts(nil) {
		t.Fatal("empty verdicts should fail-open (pass)")
	}
}

// Locks the single-frame reply contract: the JSON the model is told to emit must
// unmarshal cleanly into visionVerdict.
func TestVisionVerdictParsesSchema(t *testing.T) {
	raw := `{ "ok": false, "issues": ["ตัวหนังสือล้นกรอบ", "สีไม่ตรงแบรนด์"] }`
	var out visionVerdict
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("visionVerdict did not unmarshal: %v", err)
	}
	if out.OK || len(out.Issues) != 2 {
		t.Errorf("unexpected parse: %+v", out)
	}
}

func TestMarshalVerdicts_RoundTrips(t *testing.T) {
	in := []SceneVerdict{{SceneNumber: 1, OK: false, Issues: []string{"x"}}}
	b := MarshalVerdicts(in)
	var back []SceneVerdict
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("MarshalVerdicts produced invalid JSON: %v", err)
	}
	if len(back) != 1 || back[0].OK || back[0].SceneNumber != 1 {
		t.Errorf("round-trip mismatch: %+v", back)
	}
}
```

- [ ] **Step 3: Build + run the tests**

Run: `go build ./internal/agent/ && go test ./internal/agent/ -run 'TestSummarize|TestVisionVerdict|TestMarshalVerdicts' -v`
Expected: build OK, all 5 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/agent/visualqa.go internal/agent/visualqa_test.go
git commit -m "feat(agent): VisualQAAgent + summarizeVerdicts (fail-open on infra)"
```

---

## Task 5: Migration — `visual_qa` log table + seed the `visual_qa` agent config

**Files:**
- Create: `migrations/035_visual_qa.sql`
- Create: `migrations/036_visual_qa_agent_config.sql`

`migrations/034` is the latest existing file (Phase 1's critic config), so Phase 2 starts at `035`. `clips.id` is `UUID` (see `migrations/001_initial_schema.sql`), so `clip_id` is a UUID FK.

- [ ] **Step 1: Write the table migration**

```sql
-- 035_visual_qa.sql
-- Append-only log of Visual QA runs. One row per clip per QA pass. `passed`
-- FALSE is what drove the clip to status='needs_review' (publish blocked).
-- `issues` is the JSON array of per-scene verdicts (scene_number/ok/issues).
CREATE TABLE IF NOT EXISTS visual_qa (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id    UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    passed     BOOLEAN NOT NULL,
    issues     JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_visual_qa_clip_id ON visual_qa (clip_id);
```

- [ ] **Step 2: Write the seed migration**

Column list mirrors the working precedent in `migrations/030_topic_pipeline_schema.sql`
(`agent_name, system_prompt, prompt_template, model, temperature, enabled, skills`);
other columns (`insights`, `config`) take their table defaults. `model` is
`claude-sonnet-4-6` (vision-capable, routed by `KieLLMClient` claude prefix).
The guard is `WHERE NOT EXISTS` so re-running never clobbers prompt edits made in
the Settings UI.

```sql
-- 036_visual_qa_agent_config.sql
-- Seed the Visual QA agent: looks at ONE rendered frame per scene and decides
-- whether anything is visually broken. Detect + block only (no fix, no
-- re-render). Kill switch:
-- UPDATE agent_configs SET enabled = FALSE WHERE agent_name = 'visual_qa';
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
SELECT
  'visual_qa',
  'คุณคือ Visual QA ของ Ads Vance — ตรวจ "เฟรมจริง" ที่เรนเดอร์ออกมาจากวิดีโอสั้น 9:16 ภาษาไทย ว่ามีอะไรพังทางสายตาไหม. คุณเห็นภาพ 1 เฟรมต่อ 1 ซีน. ตัดสินแบบเข้มงวดแต่ยุติธรรม: ตั้ง ok=false เฉพาะเมื่อมั่นใจว่ามีปัญหาจริงที่คนดูจะเห็นชัด ไม่ใช่เดา. ตอบเป็น JSON object เท่านั้น.

สิ่งที่ถือว่า "พัง" (ok=false):
- caption/ตัวหนังสือ ล้นกรอบ หรือ ทับขอบจอ จนอ่านไม่ครบ.
- สีหลุดแบรนด์อย่างชัดเจน (แบรนด์คือ navy + ส้ม; เสือดาวเป็นมาสคอต). พื้นหลังสีจัดผิดธีมจนดูไม่ใช่แบรนด์.
- มีตัวหนังสือ "อบเข้าไปในรูปพื้นหลัง AI" (baked-in text) — ตัวอักษรมั่ว/ภาษาต่างดาว/สะกดเพี้ยนที่ไม่ใช่ caption ของระบบ.
- ภาพ AI เพี้ยน/น่าเกลียดชัดเจน (มือ/หน้า/วัตถุบิดเบี้ยว, artifact หนัก).

สิ่งที่ "ไม่ถือว่าพัง" (ok=true):
- รสนิยมส่วนตัว, ภาพธรรมดาแต่ไม่ผิด, ครอปแน่นแต่ยังอ่านออก.
- ถ้าไม่แน่ใจ ให้ ok=true (อย่าบล็อกคลิปดีเพราะเดา).',
  'ตรวจเฟรมของซีนที่ {{.SceneNumber}} จากคลิปเรื่อง: {{.Question}}

ข้อความบนจอที่ "ควร" จะเห็น (on_screen_text): {{.OnScreenText}}
บทพากย์ของซีนนี้ (context): {{.VoiceText}}

ดูภาพที่แนบมา แล้วตอบเป็น JSON object เท่านั้น (ห้ามมีข้อความอื่นนอก JSON):
{
  "ok": true,
  "issues": []
}

ถ้าพบปัญหาให้ ok=false และใส่เหตุผลสั้นๆ ภาษาไทยใน issues เช่น ["ตัวหนังสือล้นกรอบ","สีพื้นหลังไม่ตรงแบรนด์"]. ถ้าไม่มีปัญหาให้ ok=true และ issues เป็น array ว่าง.',
  'claude-sonnet-4-6',
  0.2,
  TRUE,
  '- บล็อกเฉพาะเมื่อมั่นใจ: caption ล้นกรอบ / สีหลุดแบรนด์ / baked-in text มั่ว / ภาพ AI เพี้ยนหนัก.
- ไม่แน่ใจ = ok=true เสมอ (fail-open).
- แบรนด์: navy + ส้ม, มาสคอตเสือดาว.'
WHERE NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'visual_qa');
```

- [ ] **Step 3: Commit**

```bash
git add migrations/035_visual_qa.sql migrations/036_visual_qa_agent_config.sql
git commit -m "feat(db): visual_qa log table + seed visual_qa agent config"
```

(The migrations apply later via `database.RunMigrations` on startup / `make migrate`; no code depends on the table existing at build/test time.)

---

## Task 6: VisualQARepo

**Files:**
- Create: `internal/repository/visualqa.go`

Takes a pre-marshalled `issues` JSON blob so the repository stays decoupled from the `agent` package (the orchestrator marshals the verdicts via `agent.MarshalVerdicts`).

- [ ] **Step 1: Write the repo**

```go
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type VisualQARepo struct {
	pool *pgxpool.Pool
}

func NewVisualQARepo(pool *pgxpool.Pool) *VisualQARepo {
	return &VisualQARepo{pool: pool}
}

// Create appends one Visual QA row. issues is the JSON-encoded per-scene verdict
// array.
func (r *VisualQARepo) Create(ctx context.Context, clipID string, passed bool, issues []byte) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO visual_qa (clip_id, passed, issues) VALUES ($1, $2, $3)`,
		clipID, passed, issues)
	return err
}
```

(Confirm the `pgxpool` import path matches the other repos — open `internal/repository/clips.go` and copy its import line verbatim if it differs. Phase 1's `internal/repository/critiques.go` uses exactly this path.)

- [ ] **Step 2: Verify it builds**

Run: `go build ./internal/repository/`
Expected: build OK.

- [ ] **Step 3: Commit**

```bash
git add internal/repository/visualqa.go
git commit -m "feat(repo): VisualQARepo append-only Create"
```

---

## Task 7: Expose the local MP4 path from the producer

**Files:**
- Modify: `internal/producer/producer.go`

`ProduceHyperframes916` currently throws away the local `mp4Path` (it only returns the uploaded URLs). Visual QA reads frames from that local file, so add a `LocalVideo916Path` field to `ProduceResult` and populate it.

> **Verify at implementation time:** confirm the local MP4 still exists when the orchestrator runs QA — i.e. nothing deletes `clipDir` / `projectDir` between `ProduceHyperframes916` returning and the orchestrator's status decision. As read on 2026-06-14, `ProduceHyperframes916` does NOT clean up the working dir, and `produceClipWithID` runs QA synchronously right after it returns, so the file is present. If a future cleanup step is added, QA must extract frames *before* it runs.

- [ ] **Step 1: Add the result field**

In `internal/producer/producer.go`, the `ProduceResult` struct currently is:

```go
type ProduceResult struct {
	Video169URL  string
	Video916URL  string
	ThumbnailURL string
```

Add a field to it:

```go
type ProduceResult struct {
	Video169URL       string
	Video916URL       string
	ThumbnailURL      string
	LocalVideo916Path string
```

(Leave the rest of the struct unchanged.)

- [ ] **Step 2: Populate it in `ProduceHyperframes916`**

In `ProduceHyperframes916`, the final return is currently:

```go
	return &ProduceResult{Video916URL: video916URL, ThumbnailURL: thumbnailURL}, nil
```

`mp4Path` is in scope (returned by `AssembleHyperframes916` at the top of the function). Change the return to include it:

```go
	return &ProduceResult{Video916URL: video916URL, ThumbnailURL: thumbnailURL, LocalVideo916Path: mp4Path}, nil
```

- [ ] **Step 3: Verify it builds**

Run: `go build ./internal/producer/`
Expected: build OK.

- [ ] **Step 4: Commit**

```bash
git add internal/producer/producer.go
git commit -m "feat(producer): expose LocalVideo916Path for Visual QA frame reads"
```

---

## Task 8: Wire Visual QA into the orchestrator

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`

QA runs between `ProduceHyperframes916` returning and the status decision. Per-scene frame timestamps are derived from the cumulative `DurationSeconds` of the scenes the orchestrator already holds — the midpoint of each scene's window — so no producer internals (`sceneBound`) need to be exported. Pass → `'ready'`; fail → `'needs_review'`.

> **Verify at implementation time:** the per-scene window is reconstructed from `scenes[i].DurationSeconds` here. The producer's own bounds come from *measured TTS length* (`synthScenesVoice`), which can differ slightly from the requested `DurationSeconds`. The midpoint only needs to land somewhere inside the scene, so small drift is harmless; but if a scene's `DurationSeconds` is 0 (the synth writes a silent placeholder in that case), its midpoint collapses onto the previous boundary — acceptable for "grab a representative frame". Do not treat these timestamps as exact.

- [ ] **Step 1: Add the two struct fields**

In the `Orchestrator` struct, after the `criticAgent   *agent.CriticAgent` line add:

```go
	visualQAAgent *agent.VisualQAAgent
```

and after the `critiquesRepo *repository.CritiquesRepo` line add:

```go
	visualQARepo  *repository.VisualQARepo
```

- [ ] **Step 2: Extend the `New(...)` constructor**

Add `vqa *agent.VisualQAAgent` after `ca` and `visualqa *repository.VisualQARepo` after `critiques`, and wire them in the returned struct:

```go
func New(
	qa *agent.QuestionAgent,
	sa *agent.ScriptAgent,
	ia *agent.ImageAgent,
	sca *agent.SceneAgent,
	ca *agent.CriticAgent,
	vqa *agent.VisualQAAgent,
	prod *producer.Producer,
	clips *repository.ClipsRepo,
	scenes *repository.ScenesRepo,
	critiques *repository.CritiquesRepo,
	visualqa *repository.VisualQARepo,
	themes *repository.ThemesRepo,
	agents *repository.AgentsRepo,
	settings *repository.SettingsRepo,
	formats *repository.FormatsRepo,
	tracker *progress.Tracker,
) *Orchestrator {
	return &Orchestrator{
		settingsRepo: settings, formatsRepo: formats, questionAgent: qa, scriptAgent: sa, imageAgent: ia,
		sceneAgent: sca, criticAgent: ca, visualQAAgent: vqa,
		producer: prod, clipsRepo: clips, scenesRepo: scenes, critiquesRepo: critiques, visualQARepo: visualqa,
		themesRepo: themes, agentsRepo: agents, tracker: tracker,
	}
}
```

- [ ] **Step 3: Add the QA helper (pure timestamp math + frame extraction)**

Append this helper to `internal/orchestrator/orchestrator.go` (anywhere at file scope; it groups naturally near `produceClipWithID`). It builds the per-scene frames; on any extraction error for a scene it simply omits that frame (the agent treats a missing frame as fail-open OK):

```go
// sceneMidTimestamps returns the midpoint timestamp (seconds) of each scene's
// window, reconstructed from cumulative DurationSeconds. Pure — the exactness
// only matters enough to land inside the scene.
func sceneMidTimestamps(scenes []agent.GeneratedScene) []float64 {
	mids := make([]float64, len(scenes))
	cursor := 0.0
	for i, s := range scenes {
		d := s.DurationSeconds
		if d < 0 {
			d = 0
		}
		mids[i] = cursor + d/2
		cursor += d
	}
	return mids
}

// extractQAFrames extracts one PNG frame per scene from the local MP4 and pairs
// it with the scene's text. A per-scene extraction failure is logged and that
// frame is dropped (Visual QA fails open on missing frames).
func (o *Orchestrator) extractQAFrames(clipID, mp4Path string, scenes []agent.GeneratedScene) []agent.QAFrame {
	mids := sceneMidTimestamps(scenes)
	frames := make([]agent.QAFrame, 0, len(scenes))
	for i, s := range scenes {
		outPath := filepath.Join(filepath.Dir(mp4Path), fmt.Sprintf("qa-scene%d.png", s.SceneNumber))
		if err := o.producer.FFmpeg().ExtractFrameAt(mp4Path, outPath, mids[i]); err != nil {
			log.Printf("visualqa: clip %s scene %d frame extract failed (skip): %v", clipID, s.SceneNumber, err)
			continue
		}
		png, err := os.ReadFile(outPath)
		if err != nil {
			log.Printf("visualqa: clip %s scene %d frame read failed (skip): %v", clipID, s.SceneNumber, err)
			continue
		}
		frames = append(frames, agent.QAFrame{
			SceneNumber:  s.SceneNumber,
			PNG:          png,
			OnScreenText: s.OnScreenText,
			VoiceText:    s.VoiceText,
		})
	}
	return frames
}
```

This helper calls `o.producer.FFmpeg()`. The `Producer` does not currently expose its `*FFmpegAssembler`. Add a one-line accessor to `internal/producer/producer.go` (near the other `Producer` methods):

```go
// FFmpeg exposes the assembler so callers (Visual QA) can extract frames from a
// rendered MP4 without re-rendering.
func (p *Producer) FFmpeg() *FFmpegAssembler { return p.ffmpeg }
```

> **Verify at implementation time:** confirm the `Producer` struct field is named `ffmpeg` (it is, per `NewProducer(... ffmpeg *FFmpegAssembler ...)` storing `ffmpeg: ffmpeg`). If the field name differs, match it.

- [ ] **Step 4: Insert the QA block before the status decision**

In `produceClipWithID`, the existing tail is:

```go
	// ── Assemble the multi-scene 9:16 video + thumbnail + upload ──
	result, err := o.producer.ProduceHyperframes916(ctx, clipID, scenes)
	if err != nil {
		return o.failClip(ctx, clipID, fmt.Errorf("produce hyperframes: %w", err))
	}

	readyStatus := "ready"
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{
		Status:       &readyStatus,
		Video916URL:  &result.Video916URL,
		ThumbnailURL: &result.ThumbnailURL,
		VoiceScript:  &narration,
		AnswerScript: &narration,
	})
	log.Printf("Clip ready (hyperframes): %s", clipID)
	return nil
}
```

Replace it with (QA decides the status; default stays `'ready'` if QA is disabled/absent):

```go
	// ── Assemble the multi-scene 9:16 video + thumbnail + upload ──
	result, err := o.producer.ProduceHyperframes916(ctx, clipID, scenes)
	if err != nil {
		return o.failClip(ctx, clipID, fmt.Errorf("produce hyperframes: %w", err))
	}

	// ── Visual QA: look at the rendered frames; block auto-publish on a
	//    confident visual defect. Optional gate; disabled/absent ⇒ 'ready'.
	//    Fail-safe is fail-OPEN: infra errors never block (see VisualQAAgent). ──
	status := "ready"
	if qaCfg, qErr := o.agentsRepo.GetByName(ctx, "visual_qa"); qErr == nil && qaCfg.Enabled && result.LocalVideo916Path != "" {
		o.tracker.StartStep("visual_qa")
		frames := o.extractQAFrames(clipID, result.LocalVideo916Path, scenes)
		qaRes := o.visualQAAgent.Review(ctx, agent.VisualQAInput{
			Question: q.Question,
			Frames:   frames,
		}, qaCfg)
		if wErr := o.visualQARepo.Create(ctx, clipID, qaRes.Passed, agent.MarshalVerdicts(qaRes.Verdicts)); wErr != nil {
			log.Printf("visualqa: persist result failed (non-fatal): %v", wErr)
		}
		if !qaRes.Passed {
			status = "needs_review"
			log.Printf("visualqa: clip %s FAILED — status=needs_review (publish blocked); verdicts=%s",
				clipID, string(agent.MarshalVerdicts(qaRes.Verdicts)))
		}
		o.tracker.CompleteStep("visual_qa")
	}

	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{
		Status:       &status,
		Video916URL:  &result.Video916URL,
		ThumbnailURL: &result.ThumbnailURL,
		VoiceScript:  &narration,
		AnswerScript: &narration,
	})
	if status == "ready" {
		log.Printf("Clip ready (hyperframes): %s", clipID)
	}
	return nil
}
```

This uses `q.Question` — confirm `q` (the `agent.GeneratedQuestion` param of `produceClipWithID`) has a `Question` field; the Phase 1 critic block already reads `q.Question`, so it does.

- [ ] **Step 5: Add the new imports**

The QA helper uses `os` (`os.ReadFile`) and `filepath` (`filepath.Join`/`Dir`). The current `orchestrator.go` import block (read 2026-06-14) does NOT import `os` or `path/filepath`. Add both:

```go
	"os"
	"path/filepath"
```

(`fmt`, `log`, `models`, `agent`, `producer`, `repository` are already imported. **Verify at implementation time** whether `os`/`path/filepath` are already present — if a later edit added them, do not duplicate.)

- [ ] **Step 6: Verify it builds**

Run: `go build ./internal/orchestrator/ ./internal/producer/`
Expected: PASS — these packages compile on their own. (`go build ./...` still fails until Task 9 fixes `cmd/server/main.go`'s call to the old `New(...)` signature.)

- [ ] **Step 7: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/producer/producer.go
git commit -m "feat(orchestrator): Visual QA gate — needs_review on visual defect"
```

---

## Task 9: Wire construction in main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Construct the new agent + repo and pass them in**

After the line `criticAgent := agent.NewCriticAgent(llm)` add:

```go
	visualQAAgent := agent.NewVisualQAAgent(llm)
```

After the line `critiquesRepo := repository.NewCritiquesRepo(pool)` add:

```go
	visualQARepo := repository.NewVisualQARepo(pool)
```

Change the `orchestrator.New(...)` call to insert `visualQAAgent` after `criticAgent` and `visualQARepo` after `critiquesRepo`:

```go
	orch := orchestrator.New(questionAgent, scriptAgent, imageAgent, sceneAgent, criticAgent, visualQAAgent, prod,
		clipsRepo, scenesRepo, critiquesRepo, visualQARepo, themesRepo, agentsRepo, settingsRepo, formatsRepo, tracker)
```

- [ ] **Step 2: Verify the whole project builds**

Run: `go build ./...`
Expected: build OK (exit 0).

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(main): construct and wire VisualQAAgent + VisualQARepo"
```

---

## Task 10: Remove the spike + full verification

**Files:**
- Delete: `cmd/visionsmoke/main.go`

- [ ] **Step 1: Delete the throwaway probe**

Its job (proving the proxy accepts vision) is done and recorded by Task 1's commit history. Remove it so it isn't shipped.

```bash
git rm cmd/visionsmoke/main.go
git commit -m "chore(vision): drop visionsmoke probe after Phase 2 gate passed"
```

- [ ] **Step 2: Build, vet, and run the full agent test suite**

Run:
```bash
go build ./... && go vet ./internal/agent/ ./internal/orchestrator/ ./internal/producer/ ./internal/repository/ && go test ./internal/agent/ -v
```
Expected: build OK, vet clean, all agent tests PASS — including the new vision-body tests (Task 2) and the `summarizeVerdicts`/schema tests (Task 4), alongside Phase 1's critic tests.

- [ ] **Step 3: Sanity-check the migrations apply (requires DB)**

Only if a dev/staging DB is configured:
```bash
make migrate
```
Expected: `035_visual_qa.sql` and `036_visual_qa_agent_config.sql` apply with no error; re-running is a no-op (`IF NOT EXISTS` + `WHERE NOT EXISTS` guards).

- [ ] **Step 4: Final status check**

```bash
git status
```
Expected: clean working tree; all changes committed across Tasks 1-9 (and the spike removed in Task 10).

---

## Notes / deliberate Phase 2 boundaries

- **No re-render, ever.** QA runs once, after the single render, on the frames that already exist. A failing clip is parked at `status='needs_review'` for a human; nothing is regenerated or re-rendered automatically.
- **No auto-fix.** The agent only emits a verdict. It does not touch scenes, prompts, images, or metadata.
- **`needs_review` is the ONLY publish-gate change.** The publisher's existing `WHERE c.status = 'ready'` already excludes every non-ready status, so blocking is achieved by setting the status alone — no publisher code changes.
- **Fail-OPEN on infrastructure** (inverted vs Phase 1's critic, which fails *closed* to the original content). A vision/decode/template/extraction error for a scene becomes an `OK` verdict so a flaky vision call or a missing frame never strands a *good* clip in review. Only a confident model verdict of `ok=false` blocks publish. This is deliberate: the cost of a false block (a good clip never publishes until someone notices) is judged higher than the cost of a false pass (one bad clip slips through, same as today's behavior with zero QA).
- **`visual_qa` is written on every QA run**, pass or fail, so the pass-rate and the recurring issue strings are queryable (a future phase could feed these back to tune the upstream image/scene prompts — out of scope here).
- **Frame timestamps are approximate.** They are reconstructed from requested `DurationSeconds`, not the producer's measured TTS bounds. The midpoint only needs to land inside the scene; exactness is not required and not attempted.
- **QA is gated by `agent_configs['visual_qa'].enabled`.** Setting it `FALSE` reverts behavior to exactly today's (`status='ready'` unconditionally) with zero code change — a clean kill switch.
```
