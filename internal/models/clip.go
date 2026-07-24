package models

import (
	"encoding/json"
	"time"
)

type Clip struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	Question         string    `json:"question"`
	QuestionerName   string    `json:"questioner_name"`
	AnswerScript     string    `json:"answer_script"`
	VoiceScript      string    `json:"voice_script"`
	Category         string    `json:"category"`
	Status           string    `json:"status"`
	Video169URL      *string   `json:"video_16_9_url"`
	Video916URL      *string   `json:"video_9_16_url"`
	ThumbnailURL     *string   `json:"thumbnail_url"`
	PublishDate      *string   `json:"publish_date"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	FailReason       *string   `json:"fail_reason,omitempty"`
	RetryCount       int       `json:"retry_count"`
	ReviewRetryCount int       `json:"review_retry_count"`
	AutoReviewHeld   bool      `json:"auto_review_held"`
	StylePreset      string    `json:"style_preset"`
	ContentFormat    string    `json:"content_format"`
	ProductionStage  string    `json:"production_stage"`
	CaseNumber       *int      `json:"case_number,omitempty"` // case-file format running number (nil = classic clip)
}

type Scene struct {
	ID              string          `json:"id"`
	ClipID          string          `json:"clip_id"`
	SceneNumber     int             `json:"scene_number"`
	SceneType       string          `json:"scene_type"`
	TextContent     string          `json:"text_content"`
	ImagePrompt     string          `json:"image_prompt"`
	Image169URL     *string         `json:"image_16_9_url"`
	Image916URL     *string         `json:"image_9_16_url"`
	VoiceText       string          `json:"voice_text"`
	DurationSeconds float64         `json:"duration_seconds"`
	TextOverlays    json.RawMessage `json:"text_overlays"`
	LayoutVariant   string          `json:"layout_variant"`
	OnScreenText    string          `json:"on_screen_text"`
	EmphasisWords   json.RawMessage `json:"emphasis_words"`
	Beat            string          `json:"beat"`
	CaptionStyle    string          `json:"caption_style"`
	Layout          string          `json:"layout"`
	Content         json.RawMessage `json:"content"`
}

// VisualQA is one persisted Visual QA run. Issues is the raw per-scene verdict
// array ([{scene_number, ok, issues}]) the frontend renders to explain why a
// clip landed in status='needs_review'.
type VisualQA struct {
	ID        string          `json:"id"`
	ClipID    string          `json:"clip_id"`
	Passed    bool            `json:"passed"`
	Issues    json.RawMessage `json:"issues"`
	CreatedAt time.Time       `json:"created_at"`
}

// AutoReview is one append-only auto-review decision row.
type AutoReview struct {
	ID         string          `json:"id"`
	ClipID     string          `json:"clip_id"`
	Decision   string          `json:"decision"`
	Confidence float64         `json:"confidence"`
	DefectType string          `json:"defect_type"`
	Reasons    json.RawMessage `json:"reasons"`
	CreatedAt  time.Time       `json:"created_at"`
}

// VisualQAStats is the aggregate Visual QA tally shown on the Content page:
// how many QA runs happened and how many blocked a clip (passed=false). Survives
// clip deletion (clip_id FK is ON DELETE SET NULL).
type VisualQAStats struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Blocked int `json:"blocked"`
}

type ClipMetadata struct {
	ClipID         string   `json:"clip_id"`
	YoutubeTitle   *string  `json:"youtube_title"`
	YoutubeDesc    *string  `json:"youtube_description"`
	YoutubeTags    []string `json:"youtube_tags"`
	ZernioPostID   *string  `json:"zernio_post_id"`
	YoutubeVideoID *string  `json:"youtube_video_id"`
	TiktokPostID   *string  `json:"tiktok_post_id"`
	IGPostID       *string  `json:"ig_post_id"`
	FBPostID       *string  `json:"fb_post_id"`
}

type ClipAnalytics struct {
	ID                string    `json:"id"`
	ClipID            string    `json:"clip_id"`
	Platform          string    `json:"platform"`
	PostType          string    `json:"post_type"`
	Views             int       `json:"views"`
	Likes             int       `json:"likes"`
	Comments          int       `json:"comments"`
	Shares            int       `json:"shares"`
	WatchTimeSeconds  float64   `json:"watch_time_seconds"`
	RetentionRate     float64   `json:"retention_rate"`
	EngagementRate    float64   `json:"engagement_rate"`     // percent: 0.83 = 0.83%
	AvgViewPercentage float64   `json:"avg_view_percentage"` // fraction: 0.83 = 83%, may exceed 1 (Shorts loops)
	SubscribersGained int       `json:"subscribers_gained"`
	SubscribersLost   int       `json:"subscribers_lost"`
	FetchedAt         time.Time `json:"fetched_at"`
}

type AnalyticsSummary struct {
	TotalViews     int     `json:"total_views"`
	TotalLikes     int     `json:"total_likes"`
	TotalComments  int     `json:"total_comments"`
	TotalShares    int     `json:"total_shares"`
	AvgRetention   float64 `json:"avg_retention_rate"`
	TotalWatchTime float64 `json:"total_watch_time_seconds"`
	ClipCount      int     `json:"clip_count"`
}

type ClipPerformance struct {
	ClipID           string   `json:"clip_id"`
	Title            string   `json:"title"`
	Category         string   `json:"category"`
	Views            int      `json:"views"`
	Likes            int      `json:"likes"`
	Comments         int      `json:"comments"`
	Shares           int      `json:"shares"`
	RetentionRate    float64  `json:"retention_rate"`
	WatchTimeSeconds float64  `json:"watch_time_seconds"`
	Sparkline        []int    `json:"sparkline"`        // daily view deltas, oldest→newest
	FailedPlatforms  []string `json:"failed_platforms"` // platforms whose publish failed
}

// PresetScore is one style preset's measured retention over a recent window,
// used to bias preset selection toward better-performing looks.
type PresetScore struct {
	Preset       string  `json:"preset"`
	AvgRetention float64 `json:"avg_retention"` // mean of latest-per-clip retention_rate, 0..1
	N            int     `json:"n"`             // number of distinct clips counted
}

type SegmentedTotals struct {
	PostType         string  `json:"post_type"`
	Views            int     `json:"views"`
	Likes            int     `json:"likes"`
	Comments         int     `json:"comments"`
	Shares           int     `json:"shares"`
	WatchTimeSeconds float64 `json:"watch_time_seconds"`
	AvgRetention     float64 `json:"avg_retention_rate"`
}

type PlatformTotals struct {
	Platform          string  `json:"platform"`
	Views             int     `json:"views"`
	Likes             int     `json:"likes"`
	Comments          int     `json:"comments"`
	Shares            int     `json:"shares"`
	WatchTimeSeconds  float64 `json:"watch_time_seconds"`
	AvgRetention      float64 `json:"avg_retention_rate"`
	EngagementRate    float64 `json:"engagement_rate"`
	SubscribersGained int     `json:"subscribers_gained"`
}

type TrendPoint struct {
	Day       time.Time `json:"day"`
	Views     int       `json:"views"`
	Likes     int       `json:"likes"`
	Comments  int       `json:"comments"`
	Shares    int       `json:"shares"`
	WatchTime float64   `json:"watch_time_seconds"`
	Retention float64   `json:"avg_retention_rate"`
}

type DeltaSummary struct {
	Views          float64 `json:"views_pct"`
	Likes          float64 `json:"likes_pct"`
	Comments       float64 `json:"comments_pct"`
	Shares         float64 `json:"shares_pct"`
	WatchTime      float64 `json:"watch_time_pct"`
	RetentionPoint float64 `json:"retention_pp"`
}

// ClipAnalyticsDaily is one day of YouTube analytics for one post,
// upserted from Zernio's daily-views endpoint on every fetch.
type ClipAnalyticsDaily struct {
	ClipID                  string  `json:"clip_id"`
	Platform                string  `json:"platform"`
	PostType                string  `json:"post_type"`
	Date                    string  `json:"date"` // YYYY-MM-DD as sent by Zernio
	Views                   int     `json:"views"`
	EstimatedMinutesWatched float64 `json:"estimated_minutes_watched"`
	AverageViewDuration     float64 `json:"average_view_duration"`
	AvgViewPercentage       float64 `json:"avg_view_percentage"` // fraction
	SubscribersGained       int     `json:"subscribers_gained"`
	SubscribersLost         int     `json:"subscribers_lost"`
	Likes                   int     `json:"likes"`
	Comments                int     `json:"comments"`
	Shares                  int     `json:"shares"`
}

// ClipPublishStatus is the last-seen publish outcome of one Zernio post.
type ClipPublishStatus struct {
	ClipID       string  `json:"clip_id"`
	Platform     string  `json:"platform"`
	PostType     string  `json:"post_type"`
	ZernioPostID string  `json:"zernio_post_id"`
	Status       string  `json:"status"`
	ErrorMessage *string `json:"error_message"`
}

// PublishFailure is one failed publish surfaced on the Analytics page.
type PublishFailure struct {
	ClipID       string    `json:"clip_id"`
	Title        string    `json:"title"`
	Platform     string    `json:"platform"`
	PostType     string    `json:"post_type"`
	ErrorMessage string    `json:"error_message"`
	CheckedAt    time.Time `json:"checked_at"`
}

// CategoryScore is one topic category's measured performance: mean within-platform
// views percentile (0..1) across its clips over a recent window.
type CategoryScore struct {
	Category      string  `json:"category"`
	AvgPercentile float64 `json:"avg_percentile"`
	AvgViews      float64 `json:"avg_views"`
	N             int     `json:"n"`
}
