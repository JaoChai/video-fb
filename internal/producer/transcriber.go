package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Transcriber turns an audio file into phrase-level caption segments.
// Implementations can be swapped (OpenAI Whisper API, local whisper, DGX).
type Transcriber interface {
	Transcribe(ctx context.Context, audioPath string) ([]TranscriptSegment, error)
}

const openAITranscriptionsAPI = "https://api.openai.com/v1/audio/transcriptions"

// OpenAITranscriber calls OpenAI's Whisper endpoint directly (NOT via OpenRouter —
// OpenRouter has no audio/transcriptions endpoint). The key is read from the
// settings table under 'openai_api_key'.
type OpenAITranscriber struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewOpenAITranscriber(pool *pgxpool.Pool) *OpenAITranscriber {
	return &OpenAITranscriber{pool: pool, client: &http.Client{Timeout: 3 * time.Minute}}
}

func (t *OpenAITranscriber) apiKey(ctx context.Context) (string, error) {
	var key string
	err := t.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'openai_api_key'`).Scan(&key)
	if err != nil {
		return "", fmt.Errorf("get openai_api_key from settings: %w", err)
	}
	if key == "" {
		return "", fmt.Errorf("openai_api_key is empty — set it in Settings (OpenRouter has no transcription endpoint)")
	}
	return key, nil
}

// verboseResponse is the subset of Whisper verbose_json we consume.
type verboseResponse struct {
	Segments []struct {
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
	} `json:"segments"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (t *OpenAITranscriber) Transcribe(ctx context.Context, audioPath string) ([]TranscriptSegment, error) {
	key, err := t.apiKey(ctx)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("open audio %s: %w", audioPath, err)
	}
	defer f.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copy audio into form: %w", err)
	}
	// segment-level verbose_json (phrase captions; Thai has no word boundaries so
	// segment granularity reads better than word-level for our captions).
	for k, v := range map[string]string{
		"model":           "whisper-1",
		"response_format": "verbose_json",
		"language":        "th",
	} {
		if err := w.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("write field %s: %w", k, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openAITranscriptionsAPI, &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send transcription request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read transcription response: %w", err)
	}

	var result verboseResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse transcription: %w (body=%s)", err, string(respBody[:min(len(respBody), 300)]))
	}
	if result.Error != nil {
		return nil, fmt.Errorf("transcription API error: %s", result.Error.Message)
	}

	segments := make([]TranscriptSegment, 0, len(result.Segments))
	for _, s := range result.Segments {
		segments = append(segments, TranscriptSegment{
			Text:  strings.TrimSpace(s.Text),
			Start: math.Round(s.Start*100) / 100,
			End:   math.Round(s.End*100) / 100,
		})
	}
	return segments, nil
}
