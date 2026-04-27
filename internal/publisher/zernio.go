package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const zernioAPI = "https://zernio.com/api/v1"

type ZernioClient struct {
	apiKey string
	client *http.Client
}

func NewZernioClient(apiKey string) *ZernioClient {
	return &ZernioClient{
		apiKey: apiKey,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

type PlatformTarget struct {
	Platform  string `json:"platform"`
	AccountID string `json:"accountId"`
}

type MediaItem struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type PostRequest struct {
	Title      string           `json:"title,omitempty"`
	Content    string           `json:"content"`
	Platforms  []PlatformTarget `json:"platforms"`
	MediaItems []MediaItem      `json:"mediaItems,omitempty"`
	IsDraft    bool             `json:"isDraft,omitempty"`
	PublishNow bool             `json:"publishNow,omitempty"`
}

type PostResponse struct {
	Post struct {
		ID string `json:"_id"`
	} `json:"post"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type AnalyticsResponse struct {
	Views            int     `json:"views"`
	Likes            int     `json:"likes"`
	Comments         int     `json:"comments"`
	Shares           int     `json:"shares"`
	WatchTimeSeconds float64 `json:"watchTimeSeconds"`
	RetentionRate    float64 `json:"retentionRate"`
}

func (z *ZernioClient) Post(ctx context.Context, req PostRequest) (*PostResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal post: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", zernioAPI+"/posts", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+z.apiKey)

	resp, err := z.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result PostResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &result, nil
}

func (z *ZernioClient) GetAnalytics(ctx context.Context, postID, platform string) (*AnalyticsResponse, error) {
	url := fmt.Sprintf("%s/analytics/%s?platform=%s", zernioAPI, postID, platform)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+z.apiKey)

	resp, err := z.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get analytics: %w", err)
	}
	defer resp.Body.Close()

	var result AnalyticsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse analytics: %w", err)
	}
	return &result, nil
}
