package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type KnowledgeRepo struct {
	pool *pgxpool.Pool
}

func NewKnowledgeRepo(pool *pgxpool.Pool) *KnowledgeRepo {
	return &KnowledgeRepo{pool: pool}
}

func (r *KnowledgeRepo) ListSources(ctx context.Context) ([]models.KnowledgeSource, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, url, source_type, crawl_frequency, last_crawled_at, enabled
		 FROM knowledge_sources ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	var sources []models.KnowledgeSource
	for rows.Next() {
		var s models.KnowledgeSource
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.SourceType,
			&s.CrawlFrequency, &s.LastCrawledAt, &s.Enabled); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func (r *KnowledgeRepo) CreateSource(ctx context.Context, name, url, sourceType, crawlFreq string) (*models.KnowledgeSource, error) {
	var s models.KnowledgeSource
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge_sources (name, url, source_type, crawl_frequency)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, url, source_type, crawl_frequency, last_crawled_at, enabled`,
		name, url, sourceType, crawlFreq,
	).Scan(&s.ID, &s.Name, &s.URL, &s.SourceType, &s.CrawlFrequency, &s.LastCrawledAt, &s.Enabled)
	if err != nil {
		return nil, fmt.Errorf("create source: %w", err)
	}
	return &s, nil
}

func (r *KnowledgeRepo) ToggleSource(ctx context.Context, id string, enabled bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE knowledge_sources SET enabled = $2 WHERE id = $1`, id, enabled)
	if err != nil {
		return fmt.Errorf("toggle source %s: %w", id, err)
	}
	return nil
}

func (r *KnowledgeRepo) DeleteSource(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM knowledge_sources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete source %s: %w", id, err)
	}
	return nil
}
