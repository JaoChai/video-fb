package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL     string
	Port            string
	APIKey          string
	ClaudeAPIKey    string
	KieAPIKey       string
	ElevenLabsVoice string
	FFmpegPath      string
	ZernioAPIKey    string
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
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		Port:            port,
		APIKey:          os.Getenv("API_KEY"),
		ClaudeAPIKey:    os.Getenv("CLAUDE_API_KEY"),
		KieAPIKey:       os.Getenv("KIE_API_KEY"),
		ElevenLabsVoice: voice,
		FFmpegPath:      ffmpeg,
		ZernioAPIKey:    os.Getenv("ZERNIO_API_KEY"),
	}
}
