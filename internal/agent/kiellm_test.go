package agent

import (
	"encoding/json"
	"strings"
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
		{"gpt5 fallback", "gpt-5-4", "gpt5", false},
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
		{
			// Real kie.ai wire format (confirmed by live call): SSE data lines + [DONE].
			name: "SSE data lines",
			body: "data: {\"candidates\": [{\"content\": {\"role\": \"model\", \"parts\": [{\"text\": \"po\"}]}}]}\n\ndata: {\"candidates\": [{\"content\": {\"parts\": [{\"text\": \"ng\"}]}}]}\n\ndata: [DONE]\n",
			want: "pong",
		},
		{
			name:    "SSE with usage-only and DONE (no candidate text)",
			body:    "data: {\"usageMetadata\":{\"totalTokenCount\":5}}\n\ndata: [DONE]\n",
			wantErr: true,
		},
		{
			name:    "SSE error line",
			body:    "data: {\"error\":{\"message\":\"quota\"}}\n\ndata: [DONE]\n",
			wantErr: true,
		},
		{
			name:    "empty body",
			body:    "",
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

func TestBuildGPT5Body(t *testing.T) {
	body, err := buildGPT5Body("gpt-5-4", "SYS", "USER")
	if err != nil {
		t.Fatalf("buildGPT5Body err: %v", err)
	}
	var parsed kieGPT5Request
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Model != "gpt-5-4" || parsed.Stream {
		t.Errorf("model=%q stream=%v, want gpt-5-4 / false", parsed.Model, parsed.Stream)
	}
	if len(parsed.Input) != 2 {
		t.Fatalf("input len = %d, want 2 (system + user)", len(parsed.Input))
	}
	if parsed.Input[0].Role != "system" || parsed.Input[0].Content[0].Type != "input_text" || parsed.Input[0].Content[0].Text != "SYS" {
		t.Errorf("system msg = %+v", parsed.Input[0])
	}
	if parsed.Input[1].Role != "user" || parsed.Input[1].Content[0].Text != "USER" {
		t.Errorf("user msg = %+v", parsed.Input[1])
	}
}

func TestBuildGPT5BodyNoSystem(t *testing.T) {
	body, _ := buildGPT5Body("gpt-5-4", "", "USER")
	var parsed kieGPT5Request
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Input) != 1 || parsed.Input[0].Role != "user" {
		t.Errorf("expected only a user message, got %+v", parsed.Input)
	}
}

func TestParseGPT5Text(t *testing.T) {
	body := []byte(`{"output":[{"type":"reasoning","summary":[]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello world"}],"status":"completed"}],"status":"completed"}`)
	got, err := parseGPT5Text(body)
	if err != nil {
		t.Fatalf("parseGPT5Text err: %v", err)
	}
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestParseGPT5TextError(t *testing.T) {
	if _, err := parseGPT5Text([]byte(`{"error":{"message":"boom"}}`)); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected error containing 'boom', got %v", err)
	}
	if _, err := parseGPT5Text([]byte(`{"output":[]}`)); err == nil {
		t.Errorf("expected error on empty output")
	}
}

func TestBuildGPT5VisionBody(t *testing.T) {
	imgs := [][]byte{[]byte("PNGDATA1"), []byte("PNGDATA2")}
	body, err := buildGPT5VisionBody("gpt-5-4", "SYS", "look", imgs)
	if err != nil {
		t.Fatalf("buildGPT5VisionBody err: %v", err)
	}
	var parsed kieGPT5Request
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Input) != 2 {
		t.Fatalf("input len = %d, want 2 (system + user)", len(parsed.Input))
	}
	user := parsed.Input[1]
	if user.Role != "user" {
		t.Fatalf("want user role, got %q", user.Role)
	}
	// 1 text block + 2 image blocks
	if len(user.Content) != 3 {
		t.Fatalf("user content blocks = %d, want 3", len(user.Content))
	}
	if user.Content[0].Type != "input_text" || user.Content[0].Text != "look" {
		t.Errorf("block0 = %+v", user.Content[0])
	}
	for i := 1; i <= 2; i++ {
		if user.Content[i].Type != "input_image" || !strings.HasPrefix(user.Content[i].ImageURL, "data:image/png;base64,") {
			t.Errorf("image block %d = %+v", i, user.Content[i])
		}
	}
}
