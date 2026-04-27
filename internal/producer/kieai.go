package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const kieAPI = "https://api.kie.ai/api/v1"

type KieClient struct {
	pool         *pgxpool.Pool
	client       *http.Client
	uploadClient *http.Client
}

func NewKieClient(pool *pgxpool.Pool) *KieClient {
	return &KieClient{
		pool:         pool,
		client:       &http.Client{Timeout: 30 * time.Second},
		uploadClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (k *KieClient) getAPIKey(ctx context.Context) (string, error) {
	var key string
	err := k.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'kie_api_key'`).Scan(&key)
	if err != nil {
		return "", fmt.Errorf("get kie_api_key from settings: %w", err)
	}
	if key == "" {
		return "", fmt.Errorf("kie_api_key is empty — set it in Settings page")
	}
	return key, nil
}

type kieTaskRequest struct {
	Model string         `json:"model"`
	Input map[string]any `json:"input"`
}

type kieTaskResponse struct {
	Code int `json:"code"`
	Data struct {
		TaskID string `json:"taskId"`
	} `json:"data"`
}

type kieStatusResponse struct {
	Code int `json:"code"`
	Data struct {
		State      string `json:"state"`
		ResultJSON string `json:"resultJson"`
		FailCode   string `json:"failCode"`
		FailMsg    string `json:"failMsg"`
	} `json:"data"`
}

func (k *KieClient) GenerateImage(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	return k.retryableGenerate(ctx, "generate-image", func() error {
		taskID, err := k.createTask(ctx, "gpt-image-2-text-to-image", map[string]any{
			"prompt":       prompt,
			"aspect_ratio": aspectRatio,
			"resolution":   "2K",
		})
		if err != nil {
			return fmt.Errorf("create image task: %w", err)
		}

		result, err := k.pollTask(ctx, taskID, 180*time.Second)
		if err != nil {
			return fmt.Errorf("poll image task: %w", err)
		}

		imageURL := extractFirstURL(result)
		if imageURL == "" {
			return fmt.Errorf("no image URL in result: %v", result)
		}

		return k.downloadFile(ctx, imageURL, outputPath)
	})
}

func (k *KieClient) GenerateVoice(ctx context.Context, text, voice, outputPath string) error {
	return k.retryableGenerate(ctx, "generate-voice", func() error {
		taskID, err := k.createTask(ctx, "elevenlabs/text-to-dialogue-v3", map[string]any{
			"dialogue":      []map[string]string{{"text": text, "voice": voice}},
			"language_code": "th",
			"stability":     0.5,
		})
		if err != nil {
			return fmt.Errorf("create voice task: %w", err)
		}

		result, err := k.pollTask(ctx, taskID, 300*time.Second)
		if err != nil {
			return fmt.Errorf("poll voice task: %w", err)
		}

		audioURL := extractFirstURL(result)
		if audioURL == "" {
			return fmt.Errorf("no audio URL in result: %v", result)
		}

		return k.downloadFile(ctx, audioURL, outputPath)
	})
}

func (k *KieClient) createTask(ctx context.Context, model string, input map[string]any) (string, error) {
	apiKey, err := k.getAPIKey(ctx)
	if err != nil {
		return "", err
	}

	reqBody := kieTaskRequest{Model: model, Input: input}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", kieAPI+"/jobs/createTask", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := k.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read createTask response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("createTask HTTP %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
	}
	var result kieTaskResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("decode createTask response: %w (body: %s)", err, string(respBody[:min(len(respBody), 200)]))
	}
	if result.Data.TaskID == "" {
		return "", fmt.Errorf("no taskId returned (code: %d, body: %s)", result.Code, string(respBody[:min(len(respBody), 200)]))
	}
	return result.Data.TaskID, nil
}

func (k *KieClient) pollTask(ctx context.Context, taskID string, timeout time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	apiKey, err := k.getAPIKey(ctx)
	if err != nil {
		return nil, err
	}

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/jobs/recordInfo?taskId=%s", kieAPI, taskID), nil)
		if err != nil {
			return nil, fmt.Errorf("create poll request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := k.client.Do(req)
		if err != nil {
			log.Printf("Poll task %s HTTP error: %v", taskID, err)
			time.Sleep(3 * time.Second)
			continue
		}

		var result kieStatusResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			log.Printf("Poll task %s decode error: %v", taskID, err)
			time.Sleep(3 * time.Second)
			continue
		}
		resp.Body.Close()

		switch result.Data.State {
		case "success":
			var output map[string]any
			if err := json.Unmarshal([]byte(result.Data.ResultJSON), &output); err != nil {
				return nil, fmt.Errorf("parse resultJson: %w (raw: %s)", err, result.Data.ResultJSON[:min(len(result.Data.ResultJSON), 200)])
			}
			return output, nil
		case "fail":
			return nil, fmt.Errorf("task failed: %s — %s", result.Data.FailCode, result.Data.FailMsg)
		}
		time.Sleep(3 * time.Second)
	}
	return nil, fmt.Errorf("task %s timed out after %v", taskID, timeout)
}

func extractFirstURL(result map[string]any) string {
	if urls, ok := result["resultUrls"].([]any); ok && len(urls) > 0 {
		if u, ok := urls[0].(string); ok {
			return u
		}
	}
	for _, key := range []string{"audio_url", "image_url", "url"} {
		if u, ok := result[key].(string); ok && u != "" {
			return u
		}
	}
	return ""
}

var retryablePatterns = []string{
	"500", "internal error",
	"429", "rate limit",
	"timeout", "timed out",
	"connection refused", "connection reset",
	"eof", "broken pipe",
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, pattern := range retryablePatterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}

const maxRetries = 3

func (k *KieClient) retryableGenerate(ctx context.Context, operation string, generate func() error) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 10 * time.Second
			log.Printf("[retry] %s attempt %d/%d after %v (error: %v)", operation, attempt, maxRetries, backoff, lastErr)
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("%s cancelled during retry: %w", operation, ctx.Err())
			case <-timer.C:
			}
		}

		lastErr = generate()
		if lastErr == nil {
			if attempt > 0 {
				log.Printf("[retry] %s succeeded on attempt %d", operation, attempt)
			}
			return nil
		}

		if !isRetryable(lastErr) {
			return lastErr
		}
	}
	return fmt.Errorf("%s failed after %d retries: %w", operation, maxRetries, lastErr)
}

func (k *KieClient) downloadFile(ctx context.Context, url, outputPath string) error {
	dir := filepath.Dir(outputPath)
	os.MkdirAll(dir, 0755)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := k.client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

const kieFileUploadAPI = "https://kieai.redpandaai.co/api"

type kieUploadResponse struct {
	Success bool `json:"success"`
	Code    int  `json:"code"`
	Data    struct {
		FileID      string `json:"fileId"`
		FileName    string `json:"fileName"`
		FileURL     string `json:"fileUrl"`
		DownloadURL string `json:"downloadUrl"`
	} `json:"data"`
	Msg string `json:"msg"`
}

func (k *KieClient) UploadFile(ctx context.Context, localPath, uploadPath string) (string, error) {
	apiKey, err := k.getAPIKey(ctx)
	if err != nil {
		return "", err
	}

	fileData, err := os.ReadFile(localPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	fileName := filepath.Base(localPath)

	var fileURL string
	err = k.retryableGenerate(ctx, "upload-file", func() error {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)

		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			return fmt.Errorf("create form file: %w", err)
		}
		if _, err := io.Copy(part, bytes.NewReader(fileData)); err != nil {
			return fmt.Errorf("copy file data: %w", err)
		}

		if uploadPath != "" {
			writer.WriteField("uploadPath", uploadPath)
		}
		writer.Close()

		req, err := http.NewRequestWithContext(ctx, "POST", kieFileUploadAPI+"/file-stream-upload", &body)
		if err != nil {
			return fmt.Errorf("create upload request: %w", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := k.uploadClient.Do(req)
		if err != nil {
			return fmt.Errorf("upload file: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read upload response: %w", err)
		}
		var result kieUploadResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return fmt.Errorf("parse upload response: %w (body: %s)", err, string(respBody[:min(len(respBody), 200)]))
		}
		if !result.Success {
			return fmt.Errorf("upload failed: %s (code: %d)", result.Msg, result.Code)
		}
		fileURL = result.Data.FileURL
		return nil
	})
	return fileURL, err
}

type kieDownloadURLResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data string `json:"data"`
}

func (k *KieClient) GetDownloadURL(ctx context.Context, fileURL string) (string, error) {
	apiKey, err := k.getAPIKey(ctx)
	if err != nil {
		return "", err
	}

	reqBody, _ := json.Marshal(map[string]string{"url": fileURL})
	req, err := http.NewRequestWithContext(ctx, "POST", kieAPI+"/common/download-url", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := k.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("get download url: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result kieDownloadURLResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse download url response: %w (body: %s)", err, string(respBody[:min(len(respBody), 200)]))
	}
	if result.Code != 200 {
		return "", fmt.Errorf("download url failed: %s (code: %d)", result.Msg, result.Code)
	}
	return result.Data, nil
}
