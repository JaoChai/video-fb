package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const openRouterAPI = "https://openrouter.ai/api/v1/chat/completions"

type LLMClient struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewLLMClient(pool *pgxpool.Pool) *LLMClient {
	return &LLMClient{pool: pool, client: &http.Client{}}
}

type chatRequest struct {
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *LLMClient) getAPIKey(ctx context.Context) (string, error) {
	var key string
	err := c.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'openrouter_api_key'`).Scan(&key)
	if err != nil {
		return "", fmt.Errorf("get openrouter_api_key from settings: %w", err)
	}
	if key == "" {
		return "", fmt.Errorf("openrouter_api_key is empty")
	}
	return key, nil
}

func (c *LLMClient) Generate(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	apiKey, err := c.getAPIKey(ctx)
	if err != nil {
		return "", err
	}

	reqBody := chatRequest{
		Model:       model,
		MaxTokens:   8000,
		Temperature: temperature,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openRouterAPI, bytes.NewReader(body))
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

	var result chatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("LLM error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty response from LLM (model: %s)", model)
	}

	return result.Choices[0].Message.Content, nil
}

func (c *LLMClient) GenerateJSON(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64, target any) error {
	text, err := c.Generate(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return err
	}

	cleaned := extractJSON(text)

	if err := json.Unmarshal([]byte(cleaned), target); err != nil {
		return fmt.Errorf("parse JSON from LLM: %w\nraw: %s", err, text[:min(len(text), 300)])
	}
	return nil
}

func extractJSON(text string) string {
	// Strip markdown code fences (```json ... ```)
	if idx := strings.Index(text, "```json"); idx >= 0 {
		text = text[idx+7:]
		if end := strings.Index(text, "```"); end >= 0 {
			return strings.TrimSpace(text[:end])
		}
	}
	if idx := strings.Index(text, "```"); idx >= 0 {
		inner := text[idx+3:]
		if end := strings.Index(inner, "```"); end >= 0 {
			candidate := strings.TrimSpace(inner[:end])
			if len(candidate) > 0 && (candidate[0] == '{' || candidate[0] == '[') {
				return candidate
			}
		}
	}

	// Find first [ or { and match to last ] or }
	arrStart := strings.IndexByte(text, '[')
	objStart := strings.IndexByte(text, '{')

	start := -1
	isArray := false
	if arrStart >= 0 && (objStart < 0 || arrStart < objStart) {
		start = arrStart
		isArray = true
	} else if objStart >= 0 {
		start = objStart
	}

	if start < 0 {
		return text
	}

	text = text[start:]
	var closing byte = '}'
	if isArray {
		closing = ']'
	}

	if last := strings.LastIndexByte(text, closing); last >= 0 {
		return text[:last+1]
	}
	return text
}
