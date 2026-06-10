package agent

import (
	"encoding/json"
	"testing"
)

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
