package crawler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/rag"
)

type Crawler struct {
	pool   *pgxpool.Pool
	engine *rag.Engine
	client *http.Client
}

func NewCrawler(pool *pgxpool.Pool, engine *rag.Engine) *Crawler {
	return &Crawler{
		pool:   pool,
		engine: engine,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Crawler) CrawlAll(ctx context.Context) error {
	rows, err := c.pool.Query(ctx,
		`SELECT id, name, COALESCE(url, ''), content FROM knowledge_sources WHERE enabled = TRUE`)
	if err != nil {
		return fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	type source struct {
		ID, Name, URL, Content string
	}
	var sources []source
	for rows.Next() {
		var s source
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.Content); err != nil {
			return fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, s)
	}

	for _, s := range sources {
		var err error
		if s.URL != "" {
			// URL source: fetch fresh content, replace chunks
			log.Printf("Crawling URL source: %s (%s)", s.Name, s.URL)
			err = c.crawlURLSource(ctx, s.ID, s.URL)
		} else {
			// Text source: ensure chunks exist (embed once if missing)
			err = c.ensureTextSourceEmbedded(ctx, s.ID, s.Name, s.Content)
		}
		if err != nil {
			log.Printf("Failed to process %s: %v", s.Name, err)
			continue
		}
		c.pool.Exec(ctx,
			`UPDATE knowledge_sources SET last_crawled_at = NOW() WHERE id = $1`, s.ID)
	}
	return nil
}

// crawlURLSource fetches a URL via Jina Reader and replaces the source's chunks.
func (c *Crawler) crawlURLSource(ctx context.Context, sourceID, url string) error {
	readerURL := "https://r.jina.ai/" + url
	req, err := http.NewRequestWithContext(ctx, "GET", readerURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "adsvance-crawler/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	text := cleanText(string(body))
	if len(strings.Fields(text)) < 50 {
		return fmt.Errorf("content too short from %s", url)
	}

	// Replace old chunks so stale content doesn't accumulate.
	if _, err := c.pool.Exec(ctx, `DELETE FROM knowledge_chunks WHERE source_id = $1`, sourceID); err != nil {
		return fmt.Errorf("delete old chunks: %w", err)
	}

	return c.embedAndStore(ctx, sourceID, text, url, 300, 50, 20)
}

// ensureTextSourceEmbedded embeds a text source's content only if it has no chunks yet.
func (c *Crawler) ensureTextSourceEmbedded(ctx context.Context, sourceID, name, content string) error {
	var count int
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM knowledge_chunks WHERE source_id = $1`, sourceID).Scan(&count); err != nil {
		return fmt.Errorf("count chunks: %w", err)
	}
	if count > 0 {
		return nil // already embedded
	}
	if strings.TrimSpace(content) == "" {
		return nil // nothing to embed
	}
	log.Printf("Embedding text source: %s", name)
	return c.embedAndStore(ctx, sourceID, content, "", 200, 30, 10)
}

// embedAndStore chunks text, generates embeddings, and stores them.
func (c *Crawler) embedAndStore(ctx context.Context, sourceID, text, url string, chunkSize, overlap, minWords int) error {
	chunks := rag.ChunkText(text, chunkSize, overlap)
	stored := 0
	for _, chunk := range chunks {
		if len(strings.Fields(chunk)) < minWords {
			continue
		}
		embedding, err := c.engine.GenerateEmbedding(ctx, chunk)
		if err != nil {
			log.Printf("Embedding failed for chunk: %v", err)
			continue
		}
		if err := c.engine.StoreChunk(ctx, sourceID, chunk, url, embedding); err != nil {
			log.Printf("Store failed: %v", err)
			continue
		}
		stored++
	}
	log.Printf("Stored %d/%d chunks for source %s", stored, len(chunks), sourceID)
	return nil
}

func cleanText(text string) string {
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "![") || strings.HasPrefix(line, "<") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}
