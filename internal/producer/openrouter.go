package producer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const openRouterAPI = "https://openrouter.ai/api/v1/chat/completions"
const openRouterTTSAPI = "https://openrouter.ai/api/v1/audio/speech"

type OpenRouterClient struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewOpenRouterClient(pool *pgxpool.Pool) *OpenRouterClient {
	return &OpenRouterClient{
		pool:   pool,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (o *OpenRouterClient) getAPIKey(ctx context.Context) (string, error) {
	var key string
	err := o.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'openrouter_api_key'`).Scan(&key)
	if err != nil {
		return "", fmt.Errorf("get openrouter_api_key from settings: %w", err)
	}
	if key == "" {
		return "", fmt.Errorf("openrouter_api_key is empty — set it in Settings page")
	}
	return key, nil
}

type orRequest struct {
	Model       string         `json:"model"`
	Messages    []orMessage    `json:"messages"`
	Modalities  []string       `json:"modalities"`
	ImageConfig *orImageConfig `json:"image_config,omitempty"`
	Audio       *orAudio       `json:"audio,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
}

type orMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type orImageConfig struct {
	AspectRatio string `json:"aspect_ratio,omitempty"`
	ImageSize   string `json:"image_size,omitempty"`
}

type orAudio struct {
	Voice  string `json:"voice"`
	Format string `json:"format"`
}

type orResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			Images  []struct {
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"images"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (o *OpenRouterClient) GenerateImage(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	return retryableCall(ctx, "openrouter-image", func() error {
		return o.generateImageOnce(ctx, prompt, aspectRatio, outputPath)
	})
}

func (o *OpenRouterClient) generateImageOnce(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	apiKey, err := o.getAPIKey(ctx)
	if err != nil {
		return err
	}

	reqBody := orRequest{
		Model:       "openai/gpt-5.4-image-2",
		Messages:    []orMessage{{Role: "user", Content: prompt}},
		Modalities:  []string{"image", "text"},
		ImageConfig: &orImageConfig{AspectRatio: aspectRatio, ImageSize: "2K"},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openRouterAPI, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}

	var result orResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if result.Error != nil {
		return fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 || len(result.Choices[0].Message.Images) == 0 {
		return fmt.Errorf("no images in response")
	}

	return saveBase64Image(result.Choices[0].Message.Images[0].ImageURL.URL, outputPath)
}

func (o *OpenRouterClient) GenerateVoice(ctx context.Context, text, voice, outputPath string) error {
	chunks := splitVoiceText(text, ttsMaxChunkRunes)
	if len(chunks) == 0 {
		return fmt.Errorf("no text to generate voice for")
	}

	if len(chunks) > 1 {
		log.Printf("Splitting voice text into %d chunks for TTS (%d chars total)", len(chunks), len([]rune(text)))
	}

	var allPCM []byte
	for i, chunk := range chunks {
		if len(chunks) > 1 {
			log.Printf("Generating TTS chunk %d/%d (%d chars)", i+1, len(chunks), len([]rune(chunk)))
		}

		var pcm []byte
		err := retryableCall(ctx, "openrouter-tts", func() error {
			var genErr error
			pcm, genErr = o.generatePCM(ctx, chunk, voice)
			return genErr
		})
		if err != nil {
			return fmt.Errorf("TTS chunk %d/%d: %w", i+1, len(chunks), err)
		}

		if len(chunks) > 1 {
			trimLeading := i > 0
			trimTrailing := i < len(chunks)-1
			pcm = trimPCMSilence(pcm, trimLeading, trimTrailing)
			if i > 0 {
				allPCM = append(allPCM, make([]byte, gapBytes)...)
			}
		}
		allPCM = append(allPCM, pcm...)
	}

	// Validate audio duration
	const sampleRate = 24000
	const bytesPerSample = 2 // 16-bit mono
	durationSec := float64(len(allPCM)) / float64(sampleRate*bytesPerSample)
	if durationSec < 5.0 && len([]rune(text)) > 100 {
		log.Printf("WARNING: TTS audio unusually short (%.1fs for %d chars) — possible truncation", durationSec, len([]rune(text)))
	}

	wavData := wrapPCMAsWAV(allPCM, sampleRate, 1, 16)

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, wavData, 0644); err != nil {
		return fmt.Errorf("write audio: %w", err)
	}

	log.Printf("Saved TTS audio (%d bytes PCM → %d bytes WAV, %.1fs, %d chunks) to %s",
		len(allPCM), len(wavData), durationSec, len(chunks), outputPath)
	return nil
}

func (o *OpenRouterClient) generatePCM(ctx context.Context, text, voice string) ([]byte, error) {
	apiKey, err := o.getAPIKey(ctx)
	if err != nil {
		return nil, err
	}

	reqBody := struct {
		Model          string `json:"model"`
		Input          string `json:"input"`
		Voice          string `json:"voice"`
		ResponseFormat string `json:"response_format"`
	}{
		Model:          "google/gemini-3.1-flash-tts-preview",
		Input:          text,
		Voice:          mapVoice(voice),
		ResponseFormat: "pcm",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal TTS: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openRouterTTSAPI, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create TTS request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}

	pcmData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read TTS audio: %w", err)
	}
	if len(pcmData) == 0 {
		return nil, fmt.Errorf("no audio data received from TTS")
	}

	return pcmData, nil
}

func saveBase64Image(dataURL, outputPath string) error {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data URL format")
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, decoded, 0644); err != nil {
		return fmt.Errorf("write image: %w", err)
	}

	log.Printf("Saved image (%d bytes) to %s", len(decoded), outputPath)
	return nil
}


func wrapPCMAsWAV(pcmData []byte, sampleRate, numChannels, bitsPerSample int) []byte {
	dataSize := len(pcmData)
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8

	buf := make([]byte, 44+dataSize)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+dataSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], uint16(numChannels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(bitsPerSample))
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))
	copy(buf[44:], pcmData)

	return buf
}

func mapVoice(voice string) string {
	if ValidVoices[strings.ToLower(voice)] {
		return voice
	}
	return "Kore"
}

const ttsMaxChunkRunes = 400
const silenceThreshold = 500
const gapDurationMs = 150
const gapBytes = 24000 * 2 * gapDurationMs / 1000 // 24kHz × 2 bytes × 150ms = 7200 bytes

func trimPCMSilence(pcm []byte, trimLeading, trimTrailing bool) []byte {
	if len(pcm) < 2 {
		return pcm
	}

	start := 0
	end := len(pcm)

	if trimLeading {
		for start < end-1 {
			sample := int16(pcm[start]) | int16(pcm[start+1])<<8
			if sample > silenceThreshold || sample < -silenceThreshold {
				break
			}
			start += 2
		}
	}

	if trimTrailing {
		for end > start+1 {
			sample := int16(pcm[end-2]) | int16(pcm[end-1])<<8
			if sample > silenceThreshold || sample < -silenceThreshold {
				break
			}
			end -= 2
		}
	}

	return pcm[start:end]
}

func splitVoiceText(text string, maxChunkRunes int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len([]rune(text)) <= maxChunkRunes {
		return []string{text}
	}

	const sep = "..."
	parts := strings.Split(text, sep)

	var chunks []string
	var segments []string
	currentLen := 0

	for _, part := range parts {
		partRunes := len([]rune(part))
		sepLen := 0
		if len(segments) > 0 {
			sepLen = len([]rune(sep))
		}

		if currentLen+sepLen+partRunes > maxChunkRunes && len(segments) > 0 {
			chunk := strings.TrimSpace(strings.Join(segments, sep))
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
			segments = []string{part}
			currentLen = partRunes
		} else {
			segments = append(segments, part)
			currentLen += sepLen + partRunes
		}
	}

	if len(segments) > 0 {
		chunk := strings.TrimSpace(strings.Join(segments, sep))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}
