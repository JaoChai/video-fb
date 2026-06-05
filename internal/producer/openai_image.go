package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const openAIImageAPI = "https://api.openai.com/v1/images/generations"

// gptImageSize maps a video aspect ratio to a gpt-image-2 size string. Sizes
// must have width/height divisible by 16 and aspect within 1:3..3:1.
func gptImageSize(aspect string) string {
	switch aspect {
	case "9:16":
		return "864x1536"
	case "16:9":
		return "1536x864"
	default:
		return "1024x1024"
	}
}

type OpenAIImageClient struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewOpenAIImageClient(pool *pgxpool.Pool) *OpenAIImageClient {
	return &OpenAIImageClient{pool: pool, client: &http.Client{Timeout: 5 * time.Minute}}
}

func (o *OpenAIImageClient) getAPIKey(ctx context.Context) (string, error) {
	var key string
	if err := o.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'openai_api_key'`).Scan(&key); err != nil {
		return "", fmt.Errorf("get openai_api_key from settings: %w", err)
	}
	if key == "" {
		return "", fmt.Errorf("openai_api_key is empty — set it in Settings")
	}
	return key, nil
}

type oaiImageReq struct {
	Model        string `json:"model"`
	Prompt       string `json:"prompt"`
	Size         string `json:"size"`
	Quality      string `json:"quality"`
	OutputFormat string `json:"output_format"`
}

type oaiImageResp struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GenerateImage matches OpenRouterClient.GenerateImage's signature so producer
// call sites swap with no other change.
func (o *OpenAIImageClient) GenerateImage(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	return retryableCall(ctx, "openai-image", func() error {
		return o.generateImageOnce(ctx, prompt, aspectRatio, outputPath)
	})
}

func (o *OpenAIImageClient) generateImageOnce(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	apiKey, err := o.getAPIKey(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(oaiImageReq{
		Model:        "gpt-image-2",
		Prompt:       prompt,
		Size:         gptImageSize(aspectRatio),
		Quality:      "high",
		OutputFormat: "png",
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", openAIImageAPI, bytes.NewReader(body))
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
	var result oaiImageResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if result.Error != nil {
		return fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Data) == 0 || result.Data[0].B64JSON == "" {
		return fmt.Errorf("no image data in response")
	}
	// gpt-image-2 returns raw base64 (no data: prefix). saveBase64Image splits on
	// the first comma and decodes the tail — prepend a dummy prefix to reuse it.
	return saveBase64Image("png;base64,"+result.Data[0].B64JSON, outputPath)
}
