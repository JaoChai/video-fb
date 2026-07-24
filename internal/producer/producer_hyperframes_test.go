package producer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	p := NewProducer(pool, kie, NewR2Client(pool), or, ffmpeg, "", t.TempDir(), nil)
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
		{SceneNumber: 1, VoiceText: "บัญชีโฆษณาโดนแบนถาวรเพราะอะไร",
			Layout: "hook", CaptionStyle: "word_pop", ImagePrompt: "",
			Content: json.RawMessage(`{"kicker":"รู้ก่อน","rows":[{"t":"บัญชีโดนแบน","bad":true},{"t":"เพิ่มบัตรไม่ได้","bad":true}]}`)},
		{SceneNumber: 2, VoiceText: "อย่ารอให้สาย ทักแอดส์แวนซ์ได้เลย",
			Layout: "cta", CaptionStyle: "phrase_block", ImagePrompt: "",
			Content: json.RawMessage(`{"title":"เจอปัญหานี้อยู่?","cta":"ทักหาเราเลย","brand":"ADS VANCE"}`)},
	}

	out, err := p.AssembleHyperframes916(context.Background(), "smoke-clip", scenes, PresetByKey("editorial-bold"), CaseInfo{})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	fi, err := os.Stat(out.mp4Path)
	if err != nil || fi.Size() < 10_000 {
		t.Fatalf("expected a non-trivial MP4 at %s (size=%d, err=%v)", out.mp4Path, fi.Size(), err)
	}
	t.Logf("rendered %s (%d bytes)", out.mp4Path, fi.Size())

	htmlBytes, herr := os.ReadFile(filepath.Join(filepath.Dir(out.mp4Path), "index.html"))
	if herr != nil {
		t.Fatalf("read index.html: %v", herr)
	}
	html := string(htmlBytes)
	if !strings.Contains(html, "const SCENES =") {
		t.Error("composition missing structured SCENES")
	}
	for _, emo := range []string{"❌", "✅", "📞", "•"} {
		if strings.Contains(html, emo) {
			t.Errorf("emoji/bullet leaked into composition: %q", emo)
		}
	}
}
