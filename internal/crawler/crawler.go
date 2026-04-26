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
		`SELECT id, name, url, source_type FROM knowledge_sources WHERE enabled = TRUE`)
	if err != nil {
		return fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	type source struct {
		ID, Name, URL, Type string
	}
	var sources []source
	for rows.Next() {
		var s source
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.Type); err != nil {
			return fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, s)
	}

	for _, s := range sources {
		log.Printf("Crawling: %s (%s)", s.Name, s.URL)
		if err := c.crawlSource(ctx, s.ID, s.URL); err != nil {
			log.Printf("Failed to crawl %s: %v", s.Name, err)
			continue
		}
		c.pool.Exec(ctx,
			`UPDATE knowledge_sources SET last_crawled_at = NOW() WHERE id = $1`, s.ID)
		log.Printf("Crawled: %s", s.Name)
	}
	return nil
}

func (c *Crawler) crawlSource(ctx context.Context, sourceID, url string) error {
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

	text := string(body)
	text = cleanText(text)

	if len(strings.Fields(text)) < 50 {
		return fmt.Errorf("content too short from %s", url)
	}

	chunks := rag.ChunkText(text, 300, 50)
	stored := 0
	for _, chunk := range chunks {
		if len(strings.Fields(chunk)) < 20 {
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
	log.Printf("Stored %d/%d chunks from %s", stored, len(chunks), url)
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
