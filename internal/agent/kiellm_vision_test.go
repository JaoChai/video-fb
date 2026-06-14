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
