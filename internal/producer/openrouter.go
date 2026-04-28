package producer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
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
	return retryableCall(ctx, "openrouter-tts", func() error {
		return o.generateVoiceOnce(ctx, text, voice, outputPath)
	})
}

func (o *OpenRouterClient) generateVoiceOnce(ctx context.Context, text, voice, outputPath string) error {
	apiKey, err := o.getAPIKey(ctx)
	if err != nil {
		return err
	}

	reqBody := orRequest{
		Model:      "google/gemini-3.1-flash-tts-preview",
		Messages:   []orMessage{{Role: "user", Content: text}},
		Modalities: []string{"audio"},
		Audio:      &orAudio{Voice: mapVoice(voice), Format: "mp3"},
		Stream:     true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal TTS: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openRouterAPI, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create TTS request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TTS %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}

	return parseSSEAudio(resp.Body, outputPath)
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

func parseSSEAudio(reader io.Reader, outputPath string) error {
	var audioChunks []string
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Audio struct {
						Data string `json:"data"`
					} `json:"audio"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Audio.Data != "" {
			audioChunks = append(audioChunks, chunk.Choices[0].Delta.Audio.Data)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read SSE stream: %w", err)
	}
	if len(audioChunks) == 0 {
		return fmt.Errorf("no audio data received from TTS")
	}

	for i, chunk := range audioChunks {
		audioChunks[i] = strings.TrimRight(chunk, "=")
	}
	fullBase64 := strings.Join(audioChunks, "")
	audioBytes, err := base64.RawStdEncoding.DecodeString(fullBase64)
	if err != nil {
		return fmt.Errorf("decode TTS audio: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, audioBytes, 0644); err != nil {
		return fmt.Errorf("write audio: %w", err)
	}

	log.Printf("Saved TTS audio (%d bytes) to %s", len(audioBytes), outputPath)
	return nil
}

func mapVoice(voice string) string {
	if ValidVoices[strings.ToLower(voice)] {
		return voice
	}
	return "alloy"
}
