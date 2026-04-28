package preflight

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/producer"
)

type CheckResult struct {
	OK     bool
	Errors []string
}

var requiredKeys = []string{"elevenlabs_voice", "kie_api_key", "zernio_api_key", "openrouter_api_key"}

func Run(ctx context.Context, pool *pgxpool.Pool) CheckResult {
	settings := make(map[string]string)
	rows, err := pool.Query(ctx,
		`SELECT key, value FROM settings WHERE key = ANY($1)`, requiredKeys)
	if err != nil {
		return CheckResult{Errors: []string{fmt.Sprintf("query settings: %v", err)}}
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		settings[k] = v
	}

	var errors []string

	if voice := settings["elevenlabs_voice"]; voice != "" && !producer.ValidVoices[strings.ToLower(voice)] {
		errors = append(errors, fmt.Sprintf("invalid elevenlabs_voice '%s' — update in Settings page", voice))
	}
	if settings["kie_api_key"] == "" {
		errors = append(errors, "Kie AI API key not set — configure in Settings page")
	}
	if settings["zernio_api_key"] == "" {
		errors = append(errors, "Zernio API key not set — configure in Settings page")
	}
	if settings["openrouter_api_key"] == "" {
		errors = append(errors, "OpenRouter API key not set — configure in Settings page")
	}

	return CheckResult{OK: len(errors) == 0, Errors: errors}
}
