package models

import (
	"encoding/json"
	"time"
)

type Clip struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Question       string    `json:"question"`
	QuestionerName string    `json:"questioner_name"`
	AnswerScript   string    `json:"answer_script"`
	VoiceScript    string    `json:"voice_script"`
	Category       string    `json:"category"`
	Status         string    `json:"status"`
	Video169URL    *string   `json:"video_16_9_url"`
	Video916URL    *string   `json:"video_9_16_url"`
	ThumbnailURL   *string   `json:"thumbnail_url"`
	PublishDate    *string   `json:"publish_date"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	FailReason     *string   `json:"fail_reason,omitempty"`
	RetryCount     int       `json:"retry_count"`
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
	ID               string    `json:"id"`
	ClipID           string    `json:"clip_id"`
	Platform         string    `json:"platform"`
	Views            int       `json:"views"`
	Likes            int       `json:"likes"`
	Comments         int       `json:"comments"`
	Shares           int       `json:"shares"`
	WatchTimeSeconds float64   `json:"watch_time_seconds"`
	RetentionRate    float64   `json:"retention_rate"`
	FetchedAt        time.Time `json:"fetched_at"`
}
