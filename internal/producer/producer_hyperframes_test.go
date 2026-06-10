package producer

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/agent"
)

// newSmokeProducer builds a real Producer against the DB with the hyperframes
// engine enabled. Used only by the gated smoke below.
func newSmokeProducer(t *testing.T, dbURL string) (*Producer, func()) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	kie := NewKieClient(pool, DefaultKieConfig())
	or := NewOpenRouterClient(pool)
	ffmpeg := NewFFmpegAssembler("ffmpeg", "") // unused by the hyperframes path
	p := NewProducer(pool, kie, or, ffmpeg, "", t.TempDir(), nil)
	p.EnableHyperframes("assets/fonts")
	return p, func() { pool.Close() }
}

// TestAssembleHyperframes916_Smoke is a MANUAL end-to-end check. It needs
// HF_RENDER=1 plus a real DATABASE_URL (for the kie/openrouter API keys in the
// settings table) and Node+Chrome on PATH. It produces a real MP4. CI skips it.
func TestAssembleHyperframes916_Smoke(t *testing.T) {
	if os.Getenv("HF_RENDER") != "1" {
		t.Skip("set HF_RENDER=1 (and DATABASE_URL + Node/Chrome) to run the end-to-end smoke")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL required for the end-to-end smoke (kie/openrouter keys live in settings)")
	}

	p, cleanup := newSmokeProducer(t, dbURL)
	defer cleanup()

	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, LayoutVariant: "hook_big", VoiceText: "บัญชีโฆษณาโดนแบนถาวรเพราะอะไร",
			OnScreenText: "บัญชีโดนแบน", EmphasisWords: []string{"แบน"}, CaptionStyle: "word_pop", ImagePrompt: ""},
		{SceneNumber: 2, LayoutVariant: "quote_cta", VoiceText: "อย่ารอให้สาย ทักแอดส์แวนซ์ได้เลย",
			OnScreenText: "ทักแอดส์แวนซ์", CaptionStyle: "phrase_block", ImagePrompt: ""},
	}

	out, err := p.AssembleHyperframes916(context.Background(), "smoke-clip", scenes)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	fi, err := os.Stat(out)
	if err != nil || fi.Size() < 10_000 {
		t.Fatalf("expected a non-trivial MP4 at %s (size=%d, err=%v)", out, fi.Size(), err)
	}
	t.Logf("rendered %s (%d bytes)", out, fi.Size())
}
