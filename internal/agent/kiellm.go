package agent

import (
	"bytes"
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
