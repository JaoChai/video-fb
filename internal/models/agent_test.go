package models

import "testing"

func TestBuildSystemPrompt(t *testing.T) {
	tests := []struct {
		name     string
		cfg      AgentConfig
		expected string
	}{
		{
			name:     "system prompt only",
			cfg:      AgentConfig{SystemPrompt: "base"},
			expected: "base",
		},
		{
			name:     "with skills",
			cfg:      AgentConfig{SystemPrompt: "base", Skills: "my skills"},
			expected: "base\n\n## Skills & Guidelines\nmy skills",
		},
		{
			name:     "with skills and insights",
			cfg:      AgentConfig{SystemPrompt: "base", Skills: "my skills", Insights: "hook insight"},
			expected: "base\n\n## Skills & Guidelines\nmy skills\n\n## Performance Insights\nhook insight",
		},
		{
			name:     "insights only (no skills)",
			cfg:      AgentConfig{SystemPrompt: "base", Insights: "hook insight"},
			expected: "base\n\n## Performance Insights\nhook insight",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.BuildSystemPrompt(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
