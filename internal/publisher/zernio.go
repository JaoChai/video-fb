package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const zernioAPI = "https://zernio.com/api/v1"

const VisibilityPrivate = "private"

type ZernioClient struct {
	fallbackKey string
	pool        *pgxpool.Pool
	client      *http.Client
	baseURL     string
}

func NewZernioClient(fallbackKey string, pool *pgxpool.Pool) *ZernioClient {
	return &ZernioClient{
		fallbackKey: fallbackKey,
		pool:        pool,
		client:      &http.Client{Timeout: 60 * time.Second},
		baseURL:     zernioAPI,
	}
}

func (z *ZernioClient) getAPIKey(ctx context.Context) string {
	if z.pool != nil {
		var dbKey string
		if err := z.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'zernio_api_key'`).Scan(&dbKey); err == nil && dbKey != "" {
			return dbKey
		}
	}
	return z.fallbackKey
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
	Visibility string           `json:"visibility,omitempty"`
}

type PostResponse struct {
	Post struct {
		ID string `json:"_id"`
	} `json:"post"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type PostMetrics struct {
	Impressions    int     `json:"impressions"`
	Reach          int     `json:"reach"`
	Likes          int     `json:"likes"`
	Comments       int     `json:"comments"`
	Shares         int     `json:"shares"`
	Saves          int     `json:"saves"`
	Clicks         int     `json:"clicks"`
	Views          int     `json:"views"`
	EngagementRate float64 `json:"engagementRate"`
	LastUpdated    string  `json:"lastUpdated"`
}

type PlatformAnalyticsEntry struct {
	Platform       string      `json:"platform"`
	PlatformPostID string      `json:"platformPostId"`
	AccountID      string      `json:"accountId"`
	Analytics      PostMetrics `json:"analytics"`
	SyncStatus     string      `json:"syncStatus"`
}

type AnalyticsResponse struct {
	PostID            string                   `json:"postId"`
	Status            string                   `json:"status"`
	Analytics         PostMetrics              `json:"analytics"`
	PlatformAnalytics []PlatformAnalyticsEntry `json:"platformAnalytics"`
	SyncStatus        string                   `json:"syncStatus"`
	Message           string                   `json:"message"`
}

type DailyViewEntry struct {
	Date                    string  `json:"date"`
	Views                   int     `json:"views"`
	EstimatedMinutesWatched float64 `json:"estimatedMinutesWatched"`
	AverageViewDuration     float64 `json:"averageViewDuration"`
	SubscribersGained       int     `json:"subscribersGained"`
	SubscribersLost         int     `json:"subscribersLost"`
	Likes                   int     `json:"likes"`
	Comments                int     `json:"comments"`
	Shares                  int     `json:"shares"`
}

type YouTubeDailyViewsResponse struct {
	Success     bool             `json:"success"`
	VideoID     string           `json:"videoId"`
	TotalViews  int              `json:"totalViews"`
	DailyViews  []DailyViewEntry `json:"dailyViews"`
	ScopeStatus struct {
		HasAnalyticsScope bool `json:"hasAnalyticsScope"`
	} `json:"scopeStatus"`
	LastSyncedAt string `json:"lastSyncedAt"`
}

// ErrYouTubeScopeMissing indicates the user must re-authorize YouTube to expose daily-views.
var ErrYouTubeScopeMissing = errors.New("youtube analytics scope missing")

func (z *ZernioClient) GetYouTubeDailyViews(ctx context.Context, videoID, accountID string) (*YouTubeDailyViewsResponse, error) {
	q := neturl.Values{}
	q.Set("videoId", videoID)
	q.Set("accountId", accountID)
	endpoint := fmt.Sprintf("%s/analytics/youtube/daily-views?%s", z.baseURL, q.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+z.getAPIKey(ctx))

	resp, err := z.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get daily-views: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read daily-views response: %w", err)
	}

	if resp.StatusCode == 412 {
		return nil, ErrYouTubeScopeMissing
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daily-views API %d for video %s: %s", resp.StatusCode, videoID, string(respBody[:min(len(respBody), 300)]))
	}

	var result YouTubeDailyViewsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse daily-views: %w", err)
	}
	return &result, nil
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
	apiKey := z.getAPIKey(ctx)
	if apiKey == "" {
		return nil, fmt.Errorf("zernio API key not configured")
	}
	log.Printf("[zernio] posting to %d platform(s), media: %d items", len(req.Platforms), len(req.MediaItems))

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := z.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("zernio %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}

	var result PostResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("zernio error: %s", result.Error)
	}
	if result.Post.ID == "" {
		return nil, fmt.Errorf("zernio returned empty post ID (response: %s)", string(respBody[:min(len(respBody), 300)]))
	}
	return &result, nil
}

func (z *ZernioClient) GetAnalytics(ctx context.Context, postID, platform string) (*AnalyticsResponse, error) {
	q := neturl.Values{}
	q.Set("postId", postID)
	q.Set("platform", platform)
	endpoint := fmt.Sprintf("%s/analytics?%s", z.baseURL, q.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+z.getAPIKey(ctx))

	resp, err := z.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get analytics: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read analytics response: %w", err)
	}

	// 200 = ready, 202 = sync pending (still parseable), 424 = all platforms failed
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != 424 {
		return nil, fmt.Errorf("analytics API returned %d for post %s/%s: %s", resp.StatusCode, postID, platform, string(respBody[:min(len(respBody), 300)]))
	}

	var result AnalyticsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse analytics: %w (body=%s)", err, string(respBody[:min(len(respBody), 300)]))
	}
	return &result, nil
}

