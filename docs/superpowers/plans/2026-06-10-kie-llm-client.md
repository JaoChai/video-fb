# kie.ai LLM Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a kie.ai LLM client (`internal/agent/kiellm.go`) that talks to Claude Sonnet 4.6 and Gemini 3.5 Flash through kie.ai, routing by model-name prefix, with a JSON-mode wrapper — the foundation every agent will use.

**Architecture:** One client, two request shapers. kie.ai is not uniform: Claude uses the Anthropic Messages format at `/claude/v1/messages` (synchronous, `stream:false`); Gemini uses the Google `streamGenerateContent` format at `/gemini/v1/models/<model>:streamGenerateContent` (streaming). A single `Generate` entry point dispatches by prefix (`claude-*` vs `gemini-*`). All wire-format logic (body builders, response parsers, prefix routing) is split into **pure functions** so it is fully unit-testable with no network or DB.

**Tech Stack:** Go 1.25, `pgxpool` (settings lookup, mirrors existing `LLMClient`), stdlib `net/http`/`encoding/json`. Reuses the existing `extractJSON` helper in `internal/agent/llm.go` (same package).

**Scope:** This plan is additive — it adds a new file and does not modify or remove any existing code. The existing OpenRouter `LLMClient` keeps working; agents migrate to `KieLLMClient` in a later plan. Auth key is the existing `kie_api_key` settings row (already used by `producer.KieClient`).

---

## File Structure

- **Create:** `internal/agent/kiellm.go` — the client + pure helpers (provider routing, Claude/Gemini body builders + parsers, `Generate`/`GenerateWithSearch`/`GenerateJSON`).
- **Create:** `internal/agent/kiellm_test.go` — table tests for all pure functions.

Single file is correct here: this mirrors the existing `internal/agent/llm.go` (one file = one LLM client) and all the logic is one cohesive responsibility (~180 lines, comparable to `llm.go`).

---

## Task 1: Provider routing by model prefix

**Files:**
- Create: `internal/agent/kiellm.go`
- Test: `internal/agent/kiellm_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/agent/kiellm_test.go`:

```go
package agent

import "testing"

func TestProviderForModel(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		want    string
		wantErr bool
	}{
		{"claude sonnet", "claude-sonnet-4-6", "claude", false},
		{"gemini flash", "gemini-3-5-flash", "gemini", false},
		{"unknown", "openai/gpt-4.1", "", true},
		{"empty", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := providerForModel(tt.model)
			if (err != nil) != tt.wantErr {
				t.Fatalf("providerForModel(%q) err = %v, wantErr %v", tt.model, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("providerForModel(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestProviderForModel`
Expected: FAIL — compile error `undefined: providerForModel`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/agent/kiellm.go`:

```go
package agent

import (
	"fmt"
	"strings"
)

func providerForModel(model string) (string, error) {
	switch {
	case strings.HasPrefix(model, "claude-"):
		return "claude", nil
	case strings.HasPrefix(model, "gemini-"):
		return "gemini", nil
	default:
		return "", fmt.Errorf("unknown model provider for %q (expected claude-* or gemini-*)", model)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/ -run TestProviderForModel`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/kiellm.go internal/agent/kiellm_test.go
git commit -m "feat(agent): kie.ai LLM provider routing by model prefix"
```

---

## Task 2: Claude request body + response parser

**Files:**
- Modify: `internal/agent/kiellm.go`
- Test: `internal/agent/kiellm_test.go`

- [ ] **Step 1: Write the failing tests**

First, replace the import block at the top of `internal/agent/kiellm_test.go` so it reads:

```go
package agent

import (
	"encoding/json"
	"strings"
	"testing"
)
```

Then append these tests to the file:

```go
func TestBuildClaudeBody(t *testing.T) {
	body, err := buildClaudeBody("claude-sonnet-4-6", "SYS", "USER", 0.5, 8000)
	if err != nil {
		t.Fatalf("buildClaudeBody err: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if parsed["model"] != "claude-sonnet-4-6" {
		t.Errorf("model = %v", parsed["model"])
	}
	if parsed["system"] != "SYS" {
		t.Errorf("system = %v, want SYS", parsed["system"])
	}
	if parsed["stream"] != false {
		t.Errorf("stream = %v, want false", parsed["stream"])
	}
	msgs, ok := parsed["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("messages = %v", parsed["messages"])
	}
	m0 := msgs[0].(map[string]any)
	if m0["role"] != "user" || m0["content"] != "USER" {
		t.Errorf("message[0] = %v", m0)
	}
}

func TestParseClaudeText(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "text blocks concatenated, tool_use ignored",
			body: `{"content":[{"type":"text","text":"hello "},{"type":"tool_use","name":"x"},{"type":"text","text":"world"}]}`,
			want: "hello world",
		},
		{
			name:    "error field",
			body:    `{"error":{"type":"invalid_request_error","message":"bad"}}`,
			wantErr: true,
		},
		{
			name:    "no text",
			body:    `{"content":[{"type":"tool_use","name":"x"}]}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseClaudeText([]byte(tt.body))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run 'TestBuildClaudeBody|TestParseClaudeText'`
Expected: FAIL — compile error `undefined: buildClaudeBody`, `undefined: parseClaudeText`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/agent/kiellm.go` (and add `"encoding/json"` to its import block so it reads `import ( "encoding/json"; "fmt"; "strings" )`):

```go
type kieClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type kieClaudeRequest struct {
	Model       string             `json:"model"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Stream      bool               `json:"stream"`
	Temperature float64            `json:"temperature,omitempty"`
	Messages    []kieClaudeMessage `json:"messages"`
}

func buildClaudeBody(model, system, user string, temp float64, maxTokens int) ([]byte, error) {
	return json.Marshal(kieClaudeRequest{
		Model:       model,
		System:      system,
		MaxTokens:   maxTokens,
		Stream:      false,
		Temperature: temp,
		Messages:    []kieClaudeMessage{{Role: "user", Content: user}},
	})
}

type kieClaudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseClaudeText(body []byte) (string, error) {
	var r kieClaudeResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("parse claude response: %w", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("claude error: %s", r.Error.Message)
	}
	var sb strings.Builder
	for _, c := range r.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	out := sb.String()
	if out == "" {
		return "", fmt.Errorf("no text in claude response")
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/ -run 'TestBuildClaudeBody|TestParseClaudeText'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/kiellm.go internal/agent/kiellm_test.go
git commit -m "feat(agent): kie.ai Claude request builder + response parser"
```

---

## Task 3: Gemini request body + response parser

**Files:**
- Modify: `internal/agent/kiellm.go`
- Test: `internal/agent/kiellm_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/agent/kiellm_test.go`:

```go
func TestBuildGeminiBody(t *testing.T) {
	body, err := buildGeminiBody("SYS", "USER", 0.4, true)
	if err != nil {
		t.Fatalf("buildGeminiBody err: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	contents := parsed["contents"].([]any)
	c0 := contents[0].(map[string]any)
	if c0["role"] != "user" {
		t.Errorf("role = %v", c0["role"])
	}
	parts := c0["parts"].([]any)
	text := parts[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "SYS") || !strings.Contains(text, "USER") {
		t.Errorf("system not prepended to user text: %q", text)
	}
	if _, ok := parsed["tools"]; !ok {
		t.Error("googleSearch tool missing when search=true")
	}
}

func TestBuildGeminiBodyNoSearch(t *testing.T) {
	body, _ := buildGeminiBody("", "hi", 0, false)
	var parsed map[string]any
	json.Unmarshal(body, &parsed)
	if _, ok := parsed["tools"]; ok {
		t.Error("tools should be absent when search=false")
	}
}

func TestParseGeminiText(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "stream array of chunks",
			body: `[{"candidates":[{"content":{"parts":[{"text":"foo "}]}}]},{"candidates":[{"content":{"parts":[{"text":"bar"}]}}]}]`,
			want: "foo bar",
		},
		{
			name: "single object",
			body: `{"candidates":[{"content":{"parts":[{"text":"solo"}]}}]}`,
			want: "solo",
		},
		{
			name:    "error chunk",
			body:    `[{"error":{"message":"quota"}}]`,
			wantErr: true,
		},
		{
			name:    "empty",
			body:    `[]`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGeminiText([]byte(tt.body))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run 'TestBuildGeminiBody|TestParseGeminiText'`
Expected: FAIL — compile error `undefined: buildGeminiBody`, `undefined: parseGeminiText`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/agent/kiellm.go` (add `"bytes"` to the import block so it reads `import ( "bytes"; "encoding/json"; "fmt"; "strings" )`):

```go
type kieGeminiPart struct {
	Text string `json:"text"`
}

type kieGeminiContent struct {
	Role  string          `json:"role"`
	Parts []kieGeminiPart `json:"parts"`
}

type kieGeminiTool struct {
	GoogleSearch map[string]any `json:"googleSearch"`
}

type kieGenConfig struct {
	Temperature float64 `json:"temperature,omitempty"`
}

type kieGeminiRequest struct {
	Contents         []kieGeminiContent `json:"contents"`
	Tools            []kieGeminiTool    `json:"tools,omitempty"`
	GenerationConfig *kieGenConfig      `json:"generationConfig,omitempty"`
}

func buildGeminiBody(system, user string, temp float64, search bool) ([]byte, error) {
	text := user
	if system != "" {
		// Gemini has no system role — prepend system instruction to the user turn.
		text = system + "\n\n" + user
	}
	req := kieGeminiRequest{
		Contents: []kieGeminiContent{{Role: "user", Parts: []kieGeminiPart{{Text: text}}}},
	}
	if search {
		req.Tools = []kieGeminiTool{{GoogleSearch: map[string]any{}}}
	}
	if temp > 0 {
		req.GenerationConfig = &kieGenConfig{Temperature: temp}
	}
	return json.Marshal(req)
}

type kieGeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// parseGeminiText handles both a JSON array of streamed chunks and a single
// response object, concatenating all candidate part text.
func parseGeminiText(body []byte) (string, error) {
	trimmed := bytes.TrimSpace(body)
	var chunks []kieGeminiResponse
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &chunks); err != nil {
			return "", fmt.Errorf("parse gemini stream array: %w", err)
		}
	} else {
		var single kieGeminiResponse
		if err := json.Unmarshal(trimmed, &single); err != nil {
			return "", fmt.Errorf("parse gemini response: %w", err)
		}
		chunks = []kieGeminiResponse{single}
	}
	var sb strings.Builder
	for _, ch := range chunks {
		if ch.Error != nil {
			return "", fmt.Errorf("gemini error: %s", ch.Error.Message)
		}
		for _, cand := range ch.Candidates {
			for _, p := range cand.Content.Parts {
				sb.WriteString(p.Text)
			}
		}
	}
	out := sb.String()
	if out == "" {
		return "", fmt.Errorf("no text in gemini response")
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/ -run 'TestBuildGeminiBody|TestParseGeminiText'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/kiellm.go internal/agent/kiellm_test.go
git commit -m "feat(agent): kie.ai Gemini request builder + tolerant stream parser"
```

---

## Task 4: Client glue — Generate / GenerateWithSearch / GenerateJSON

**Files:**
- Modify: `internal/agent/kiellm.go`

This task wires the HTTP transport and the JSON wrapper. The HTTP `Do` path is integration-only (needs a live key), so it is not unit-tested here; correctness is guarded by `go build` + `go vet` and the pure-function tests from Tasks 1–3. `GenerateJSON` reuses the existing `extractJSON` and `min`/`max` builtins (Go 1.25) already used by `llm.go` in this package.

- [ ] **Step 1: Add transport + wrappers**

Append to `internal/agent/kiellm.go`, and extend the import block to:

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)
```

Then append:

```go
const (
	kieClaudeAPI   = "https://api.kie.ai/claude/v1/messages"
	kieGeminiAPIFmt = "https://api.kie.ai/gemini/v1/models/%s:streamGenerateContent"
	kieLLMMaxTokens = 8000
)

type KieLLMClient struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewKieLLMClient(pool *pgxpool.Pool) *KieLLMClient {
	return &KieLLMClient{pool: pool, client: &http.Client{Timeout: 5 * time.Minute}}
}

func (c *KieLLMClient) getAPIKey(ctx context.Context) (string, error) {
	var key string
	if err := c.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'kie_api_key'`).Scan(&key); err != nil {
		return "", fmt.Errorf("get kie_api_key from settings: %w", err)
	}
	if key == "" {
		return "", fmt.Errorf("kie_api_key is empty — set it in Settings page")
	}
	return key, nil
}

// Generate routes to Claude or Gemini by model prefix and returns the text.
func (c *KieLLMClient) Generate(ctx context.Context, model, system, user string, temp float64) (string, error) {
	return c.generate(ctx, model, system, user, temp, false)
}

// GenerateWithSearch is Generate with Gemini googleSearch grounding enabled.
// Only valid for gemini-* models (the research agent); Claude ignores the flag.
func (c *KieLLMClient) GenerateWithSearch(ctx context.Context, model, system, user string, temp float64) (string, error) {
	return c.generate(ctx, model, system, user, temp, true)
}

func (c *KieLLMClient) generate(ctx context.Context, model, system, user string, temp float64, search bool) (string, error) {
	provider, err := providerForModel(model)
	if err != nil {
		return "", err
	}
	apiKey, err := c.getAPIKey(ctx)
	if err != nil {
		return "", err
	}

	var url string
	var reqBody []byte
	switch provider {
	case "claude":
		url = kieClaudeAPI
		reqBody, err = buildClaudeBody(model, system, user, temp, kieLLMMaxTokens)
	case "gemini":
		url = fmt.Sprintf(kieGeminiAPIFmt, model)
		reqBody, err = buildGeminiBody(system, user, temp, search)
	}
	if err != nil {
		return "", fmt.Errorf("build %s request: %w", provider, err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kie %s HTTP %d: %s", provider, resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}

	if provider == "claude" {
		return parseClaudeText(respBody)
	}
	return parseGeminiText(respBody)
}

// GenerateJSON calls Generate and unmarshals the (fence-stripped) result into
// target, retrying with progressively lower temperature on parse failure.
func (c *KieLLMClient) GenerateJSON(ctx context.Context, model, system, user string, temp float64, target any) error {
	const maxRetries = 2
	var lastErr error
	for attempt := range maxRetries + 1 {
		t := temp
		if attempt > 0 {
			t = max(0, temp-0.2*float64(attempt))
			log.Printf("KieLLM JSON retry %d/%d (temperature: %.2f)", attempt, maxRetries, t)
		}
		text, err := c.Generate(ctx, model, system, user, t)
		if err != nil {
			return err
		}
		cleaned := extractJSON(text)
		if err := json.Unmarshal([]byte(cleaned), target); err != nil {
			lastErr = fmt.Errorf("parse JSON from KieLLM: %w\nraw: %s", err, text[:min(len(text), 300)])
			continue
		}
		return nil
	}
	return lastErr
}
```

- [ ] **Step 2: Build and vet**

Run: `go build ./... && go vet ./internal/agent/`
Expected: no output (success). If `extractJSON` is reported undefined, confirm it exists in `internal/agent/llm.go` (same package) — it does; do not redefine it.

- [ ] **Step 3: Run the full package test suite**

Run: `go test ./internal/agent/`
Expected: PASS (all existing agent tests + the new `TestProviderForModel`, `TestBuildClaudeBody`, `TestParseClaudeText`, `TestBuildGeminiBody`, `TestParseGeminiText`).

- [ ] **Step 4: Commit**

```bash
git add internal/agent/kiellm.go
git commit -m "feat(agent): kie.ai LLM client transport + GenerateJSON wrapper"
```

---

## Self-Review Notes

- **Spec coverage:** Implements spec §4.1 (kie.ai LLM client: two shapers, prefix routing, JSON wrapper, `kie_api_key`, control flow off response fields not top-level `code`). Other spec sections (agents, render, producer, orchestrator, frontend, deploy) are covered by Plans 2 and 3.
- **Honesty flags (from research — verify against a live call before Plan 2 agents depend on exact shapes):**
  1. Gemini `streamGenerateContent` wire format (JSON array vs SSE) is not nailed down in kie.ai docs. `parseGeminiText` tolerates a JSON array or a single object; if kie.ai returns SSE (`data:` lines), add an SSE-splitting branch then. Validate with one real `gemini-3-5-flash` call.
  2. Gemini `generationConfig.temperature` support is unconfirmed in docs — sent as standard Gemini field; harmless if ignored.
  3. Claude `system` as a top-level field is the Anthropic Messages convention; confirm kie.ai passes it through (fallback: prepend system to the user message, same as Gemini).
- **No placeholders, types consistent:** `buildClaudeBody`/`parseClaudeText`/`buildGeminiBody`/`parseGeminiText`/`providerForModel`/`KieLLMClient.Generate`/`GenerateWithSearch`/`GenerateJSON` names are used identically across tasks.
- **Integration check (manual, optional before Plan 2):** with `kie_api_key` set in `settings`, a tiny `main` or test calling `Generate(ctx, "claude-sonnet-4-6", "", "say hi", 0)` and `GenerateWithSearch(ctx, "gemini-3-5-flash", "", "latest Meta ads news", 0.3)` should each return non-empty text. This is the moment to fix any wire-format surprise from the flags above.
