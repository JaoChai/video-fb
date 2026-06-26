package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func providerForModel(model string) (string, error) {
	switch {
	case strings.HasPrefix(model, "claude-"):
		return "claude", nil
	case strings.HasPrefix(model, "gemini-"):
		return "gemini", nil
	case strings.HasPrefix(model, "gpt-"):
		return "gpt5", nil
	default:
		return "", fmt.Errorf("unknown model provider for %q (expected claude-*, gemini-* or gpt-*)", model)
	}
}

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

// parseGeminiText concatenates all candidate part text from a Gemini response.
// kie.ai's streamGenerateContent returns Server-Sent Events ("data: {...}" lines
// terminated by "data: [DONE]"), confirmed by live call. A JSON array of chunks
// and a single response object are also tolerated as fallbacks.
func parseGeminiText(body []byte) (string, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", fmt.Errorf("empty gemini response")
	}
	var chunks []kieGeminiResponse
	switch trimmed[0] {
	case '[':
		if err := json.Unmarshal(trimmed, &chunks); err != nil {
			return "", fmt.Errorf("parse gemini stream array: %w", err)
		}
	case '{':
		var single kieGeminiResponse
		if err := json.Unmarshal(trimmed, &single); err != nil {
			return "", fmt.Errorf("parse gemini response: %w", err)
		}
		chunks = []kieGeminiResponse{single}
	default:
		chunks = parseGeminiSSE(trimmed)
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

// parseGeminiSSE extracts response chunks from Server-Sent Events: each line is
// "data: <json>", terminated by "data: [DONE]". Non-data and non-JSON lines are
// skipped so keep-alives and the terminator don't break parsing.
func parseGeminiSSE(body []byte) []kieGeminiResponse {
	var chunks []kieGeminiResponse
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var ch kieGeminiResponse
		if err := json.Unmarshal([]byte(payload), &ch); err != nil {
			continue
		}
		chunks = append(chunks, ch)
	}
	return chunks
}

// gpt-5-4 uses the OpenAI Responses API shape (NOT chat completions): the prompt
// goes in an `input` array of messages whose content is typed blocks, and the
// reply text lives in output[](type=message).content[](type=output_text).text.
type kieGPT5Block struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type kieGPT5Message struct {
	Role    string         `json:"role"`
	Content []kieGPT5Block `json:"content"`
}

type kieGPT5Request struct {
	Model  string           `json:"model"`
	Stream bool             `json:"stream"`
	Input  []kieGPT5Message `json:"input"`
}

// buildGPT5Body builds a non-streaming Responses request. gpt-5-4 takes no
// temperature (it's a reasoning model), so temp is intentionally dropped.
func buildGPT5Body(model, system, user string) ([]byte, error) {
	input := make([]kieGPT5Message, 0, 2)
	if system != "" {
		input = append(input, kieGPT5Message{Role: "system", Content: []kieGPT5Block{{Type: "input_text", Text: system}}})
	}
	input = append(input, kieGPT5Message{Role: "user", Content: []kieGPT5Block{{Type: "input_text", Text: user}}})
	return json.Marshal(kieGPT5Request{Model: model, Stream: false, Input: input})
}

type kieGPT5Response struct {
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func parseGPT5Text(body []byte) (string, error) {
	var r kieGPT5Response
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("parse gpt-5 response: %w", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("gpt-5 error: %s", r.Error.Message)
	}
	var sb strings.Builder
	for _, o := range r.Output {
		if o.Type != "message" {
			continue
		}
		for _, c := range o.Content {
			if c.Type == "output_text" {
				sb.WriteString(c.Text)
			}
		}
	}
	out := sb.String()
	if out == "" {
		return "", fmt.Errorf("no text in gpt-5 response")
	}
	return out, nil
}

const (
	kieClaudeAPI    = "https://api.kie.ai/claude/v1/messages"
	kieGeminiAPIFmt = "https://api.kie.ai/gemini/v1/models/%s:streamGenerateContent"
	kieGPT5API      = "https://api.kie.ai/codex/v1/responses"
	// kieGPT5FallbackModel is the model the flaky kie.ai Claude proxy falls back
	// to on a retryable failure (network error / HTTP 5xx).
	kieGPT5FallbackModel = "gpt-5-4"
	kieLLMMaxTokens      = 8000
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

	text, retryable, err := c.doKieRequest(ctx, apiKey, provider, model, system, user, temp, search)
	if err == nil {
		return text, nil
	}

	// The kie.ai Claude proxy is flaky (intermittent HTTP 500 "Network error").
	// On a retryable failure, fall back once to gpt-5-4 so a transient proxy
	// outage doesn't fail the whole clip. Gemini/gpt-5 callers don't fall back.
	if provider == "claude" && retryable {
		log.Printf("KieLLM: claude %q failed (%v) — falling back to %s", model, err, kieGPT5FallbackModel)
		fbText, _, fbErr := c.doKieRequest(ctx, apiKey, "gpt5", kieGPT5FallbackModel, system, user, temp, false)
		if fbErr == nil {
			return fbText, nil
		}
		return "", fmt.Errorf("claude failed (%v); %s fallback also failed: %w", err, kieGPT5FallbackModel, fbErr)
	}
	return "", err
}

// doKieRequest performs one kie.ai call for the given provider and returns the
// parsed text. The retryable bool is true when the failure is worth a fallback:
// a transport error or an HTTP 5xx (vs. a 4xx / parse error, which won't improve
// on retry).
func (c *KieLLMClient) doKieRequest(ctx context.Context, apiKey, provider, model, system, user string, temp float64, search bool) (text string, retryable bool, err error) {
	var url string
	var reqBody []byte
	switch provider {
	case "claude":
		url = kieClaudeAPI
		reqBody, err = buildClaudeBody(model, system, user, temp, kieLLMMaxTokens)
	case "gemini":
		url = fmt.Sprintf(kieGeminiAPIFmt, model)
		reqBody, err = buildGeminiBody(system, user, temp, search)
	case "gpt5":
		url = kieGPT5API
		reqBody, err = buildGPT5Body(model, system, user)
	}
	if err != nil {
		return "", false, fmt.Errorf("build %s request: %w", provider, err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", false, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", true, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", true, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode >= 500, fmt.Errorf("kie %s HTTP %d: %s", provider, resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}

	switch provider {
	case "claude":
		text, err = parseClaudeText(respBody)
	case "gemini":
		text, err = parseGeminiText(respBody)
	case "gpt5":
		text, err = parseGPT5Text(respBody)
	}
	return text, false, err
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
