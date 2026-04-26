package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const claudeAPI = "https://api.anthropic.com/v1/messages"

type ClaudeClient struct {
	apiKey string
	model  string
	client *http.Client
}

func NewClaudeClient(apiKey, model string) *ClaudeClient {
	return &ClaudeClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

type claudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	System      string          `json:"system,omitempty"`
	Messages    []claudeMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *ClaudeClient) Generate(ctx context.Context, systemPrompt, userPrompt string, temperature float64) (string, error) {
	reqBody := claudeRequest{
		Model:       c.model,
		MaxTokens:   8000,
		System:      systemPrompt,
		Temperature: temperature,
		Messages: []claudeMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", claudeAPI, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result claudeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("claude error: %s", result.Error.Message)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}

	return result.Content[0].Text, nil
}

func (c *ClaudeClient) GenerateJSON(ctx context.Context, systemPrompt, userPrompt string, temperature float64, target any) error {
	text, err := c.Generate(ctx, systemPrompt, userPrompt, temperature)
	if err != nil {
		return err
	}

	cleaned := text
	if idx := bytes.IndexByte([]byte(cleaned), '['); idx > 0 {
		cleaned = cleaned[idx:]
	} else if idx := bytes.IndexByte([]byte(cleaned), '{'); idx > 0 {
		cleaned = cleaned[idx:]
	}
	if last := bytes.LastIndexByte([]byte(cleaned), ']'); last >= 0 {
		cleaned = cleaned[:last+1]
	} else if last := bytes.LastIndexByte([]byte(cleaned), '}'); last >= 0 {
		cleaned = cleaned[:last+1]
	}

	if err := json.Unmarshal([]byte(cleaned), target); err != nil {
		return fmt.Errorf("parse JSON from Claude: %w\nraw: %s", err, text[:min(len(text), 200)])
	}
	return nil
}
