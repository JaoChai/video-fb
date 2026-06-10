package models

import "encoding/json"

type CreateClipRequest struct {
	Title          string  `json:"title"`
	Question       string  `json:"question"`
	QuestionerName string  `json:"questioner_name"`
	Category       string  `json:"category"`
	PublishDate    *string `json:"publish_date"`
	ContentFormat  string  `json:"content_format"`
}

type UpdateClipRequest struct {
	Title          *string `json:"title"`
	Question       *string `json:"question"`
	QuestionerName *string `json:"questioner_name"`
	AnswerScript   *string `json:"answer_script"`
	VoiceScript    *string `json:"voice_script"`
	Category       *string `json:"category"`
	Status         *string `json:"status"`
	Video169URL    *string `json:"video_16_9_url"`
	Video916URL    *string `json:"video_9_16_url"`
	ThumbnailURL   *string `json:"thumbnail_url"`
	PublishDate    *string `json:"publish_date"`
}

type CreateSceneRequest struct {
	ClipID          string          `json:"clip_id"`
	SceneNumber     int             `json:"scene_number"`
	SceneType       string          `json:"scene_type"`
	TextContent     string          `json:"text_content"`
	ImagePrompt     string          `json:"image_prompt"`
	VoiceText       string          `json:"voice_text"`
	DurationSeconds float64         `json:"duration_seconds"`
	TextOverlays    json.RawMessage `json:"text_overlays"`
	LayoutVariant   string          `json:"layout_variant"`
	OnScreenText    string          `json:"on_screen_text"`
	EmphasisWords   json.RawMessage `json:"emphasis_words"`
	Beat            string          `json:"beat"`
	CaptionStyle    string          `json:"caption_style"`
}
