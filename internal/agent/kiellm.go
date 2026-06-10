package agent

import (
	"encoding/json"
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
