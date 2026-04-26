package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const kieAPI = "https://api.kie.ai/api/v1"

type KieClient struct {
	apiKey string
	client *http.Client
}

func NewKieClient(apiKey string) *KieClient {
	return &KieClient{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
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
		Status string         `json:"status"`
		Output map[string]any `json:"output"`
	} `json:"data"`
}

func (k *KieClient) GenerateImage(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	taskID, err := k.createTask(ctx, "gpt-image-2-text-to-image", map[string]any{
		"prompt":       prompt,
		"aspect_ratio": aspectRatio,
		"resolution":   "2K",
	})
	if err != nil {
		return fmt.Errorf("create image task: %w", err)
	}

	result, err := k.pollTask(ctx, taskID, 120*time.Second)
	if err != nil {
		return fmt.Errorf("poll image task: %w", err)
	}

	imageURL, ok := result["image_url"].(string)
	if !ok {
		return fmt.Errorf("no image_url in result")
	}

	return k.downloadFile(ctx, imageURL, outputPath)
}

func (k *KieClient) GenerateVoice(ctx context.Context, text, voice, outputPath string) error {
	taskID, err := k.createTask(ctx, "elevenlabs/text-to-dialogue-v3", map[string]any{
		"dialogue":      []map[string]string{{"text": text, "voice": voice}},
		"language_code": "th",
		"stability":     0.5,
	})
	if err != nil {
		return fmt.Errorf("create voice task: %w", err)
	}

	result, err := k.pollTask(ctx, taskID, 120*time.Second)
	if err != nil {
		return fmt.Errorf("poll voice task: %w", err)
	}

	audioURL, ok := result["audio_url"].(string)
	if !ok {
		return fmt.Errorf("no audio_url in result")
	}

	return k.downloadFile(ctx, audioURL, outputPath)
}

func (k *KieClient) createTask(ctx context.Context, model string, input map[string]any) (string, error) {
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
	req.Header.Set("Authorization", "Bearer "+k.apiKey)

	resp, err := k.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result kieTaskResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Data.TaskID == "" {
		return "", fmt.Errorf("no taskId returned")
	}
	return result.Data.TaskID, nil
}

func (k *KieClient) pollTask(ctx context.Context, taskID string, timeout time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/jobs/getTaskDetail?taskId=%s", kieAPI, taskID), nil)
		req.Header.Set("Authorization", "Bearer "+k.apiKey)

		resp, err := k.client.Do(req)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		var result kieStatusResponse
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		switch result.Data.Status {
		case "completed", "success":
			return result.Data.Output, nil
		case "failed", "error":
			return nil, fmt.Errorf("task failed: %v", result.Data.Output)
		}
		time.Sleep(3 * time.Second)
	}
	return nil, fmt.Errorf("task %s timed out after %v", taskID, timeout)
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
