package agent

import (
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
