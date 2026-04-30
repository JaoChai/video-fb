package models

import (
	"encoding/json"
	"time"
)

type KnowledgeSource struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Content    string `json:"content"`
	Enabled    bool   `json:"enabled"`
	ChunkCount int    `json:"chunk_count"`
}

type KnowledgeSourceSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	ContentPreview string `json:"content_preview"`
	Enabled        bool   `json:"enabled"`
	ChunkCount     int    `json:"chunk_count"`
}

type KnowledgeChunk struct {
	ID        string          `json:"id"`
	SourceID  string          `json:"source_id"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata"`
	URL       *string         `json:"url"`
	CrawledAt time.Time       `json:"crawled_at"`
}
