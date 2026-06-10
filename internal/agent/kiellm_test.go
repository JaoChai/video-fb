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
