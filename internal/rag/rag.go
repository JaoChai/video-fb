package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Engine struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewEngine(pool *pgxpool.Pool) *Engine {
	return &Engine{
		pool:   pool,
		client: &http.Client{},
	}
}

func (e *Engine) getAPIKey(ctx context.Context) (string, error) {
	var key string
	err := e.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'openrouter_api_key'`).Scan(&key)
	if err != nil {
		return "", fmt.Errorf("get openrouter_api_key from settings: %w", err)
	}
	if key == "" {
		return "", fmt.Errorf("openrouter_api_key is empty")
	}
	return key, nil
}

type embeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (e *Engine) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	apiKey, err := e.getAPIKey(ctx)
	if err != nil {
		return nil, err
	}

	reqBody := embeddingRequest{
		Input: []string{text},
		Model: "openai/text-embedding-3-small",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send embedding request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	var result embeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("embedding error: %s", result.Error.Message)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Data[0].Embedding, nil
}

func (e *Engine) StoreChunk(ctx context.Context, sourceID, content, url string, embedding []float64) error {
	embStr := formatVector(embedding)
	_, err := e.pool.Exec(ctx,
		`INSERT INTO knowledge_chunks (source_id, content, url, embedding)
		 VALUES ($1, $2, $3, $4::vector)`,
		sourceID, content, url, embStr)
	if err != nil {
		return fmt.Errorf("store chunk: %w", err)
	}
	return nil
}

type SearchResult struct {
	Content    string  `json:"content"`
	URL        string  `json:"url"`
	Similarity float64 `json:"similarity"`
}

func (e *Engine) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	embedding, err := e.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	embStr := formatVector(embedding)
	rows, err := e.pool.Query(ctx,
		`SELECT content, COALESCE(url, ''), 1 - (embedding <=> $1::vector) AS similarity
		 FROM knowledge_chunks
		 ORDER BY embedding <=> $1::vector
		 LIMIT $2`,
		embStr, topK)
	if err != nil {
		return nil, fmt.Errorf("search chunks: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Content, &r.URL, &r.Similarity); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		results = append(results, r)
	}
	return results, nil
}

func formatVector(v []float64) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%f", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func ChunkText(text string, maxChunkSize int, overlap int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []string
	start := 0
	for start < len(words) {
		end := start + maxChunkSize
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[start:end], " ")
		chunks = append(chunks, chunk)
		start = end - overlap
		if start >= end || start < 1 {
			break
		}
	}
	return chunks
}
