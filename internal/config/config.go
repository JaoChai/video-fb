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
	ZernioAPIKey    string

	// Hyperframes video pipeline (off by default; the only render path — no FFmpeg fallback).
	HyperframesEnabled    bool
	HyperframesFontsDir   string
	HyperframesMultiScene bool // generate multi-scene 9:16+16:9 via composition_scenes agent
}

func Load() *Config {
	godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	voice := os.Getenv("ELEVENLABS_VOICE")

	fontsDir := os.Getenv("HYPERFRAMES_FONTS_DIR")
	if fontsDir == "" {
		fontsDir = "/app/assets/fonts"
	}

	return &Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		Port:            port,
		APIKey:          os.Getenv("API_KEY"),
		ClaudeAPIKey:    os.Getenv("CLAUDE_API_KEY"),
		KieAPIKey:       os.Getenv("KIE_API_KEY"),
		ElevenLabsVoice: voice,
		ZernioAPIKey:    os.Getenv("ZERNIO_API_KEY"),

		HyperframesEnabled:    os.Getenv("HYPERFRAMES_ENABLED") == "true",
		HyperframesFontsDir:   fontsDir,
		HyperframesMultiScene: os.Getenv("HYPERFRAMES_MULTI_SCENE") == "true",
	}
}
