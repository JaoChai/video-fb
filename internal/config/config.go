package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL      string
	Port             string
	APIKey           string
	ClaudeAPIKey     string
	KieAPIKey        string
	ElevenLabsVoice  string
	FFmpegPath       string
	ZernioAPIKey     string
	SchedulerEnabled bool
}

func Load() *Config {
	godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ffmpeg := os.Getenv("FFMPEG_PATH")
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}

	voice := os.Getenv("ELEVENLABS_VOICE")

	return &Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		Port:             port,
		APIKey:           os.Getenv("API_KEY"),
		ClaudeAPIKey:     os.Getenv("CLAUDE_API_KEY"),
		KieAPIKey:        os.Getenv("KIE_API_KEY"),
		ElevenLabsVoice:  voice,
		FFmpegPath:       ffmpeg,
		ZernioAPIKey:     os.Getenv("ZERNIO_API_KEY"),
		SchedulerEnabled: envBool("SCHEDULER_ENABLED", true),
	}
}

// envBool reads a boolean env var. An unset or unparseable value falls back
// to def — a typo must never silently become false, since the scheduler
// default (true) drives the production/publish/retry cron on this instance.
func envBool(key string, def bool) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return def
	}
	return v
}
