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
		`SELECT ks.id, ks.name, ks.category, ks.content, ks.enabled,
		        COALESCE((SELECT COUNT(*) FROM knowledge_chunks kc WHERE kc.source_id = ks.id), 0) AS chunk_count
		 FROM knowledge_sources ks ORDER BY ks.category, ks.name`)
	if err != nil {
		return nil, fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	var sources []models.KnowledgeSource
	for rows.Next() {
		var s models.KnowledgeSource
		if err := rows.Scan(&s.ID, &s.Name, &s.Category, &s.Content, &s.Enabled, &s.ChunkCount); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func (r *KnowledgeRepo) CreateSource(ctx context.Context, name, category, content string) (*models.KnowledgeSource, error) {
	var s models.KnowledgeSource
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge_sources (name, category, content, url, source_type, crawl_frequency)
		 VALUES ($1, $2, $3, '', $2, 'manual')
		 RETURNING id, name, category, content, enabled`,
		name, category, content,
	).Scan(&s.ID, &s.Name, &s.Category, &s.Content, &s.Enabled)
	if err != nil {
		return nil, fmt.Errorf("create source: %w", err)
	}
	return &s, nil
}

func (r *KnowledgeRepo) UpdateSource(ctx context.Context, id, name, category, content string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE knowledge_sources SET name=$2, category=$3, content=$4 WHERE id=$1`,
		id, name, category, content)
	if err != nil {
		return fmt.Errorf("update source %s: %w", id, err)
	}
	return nil
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

func (r *KnowledgeRepo) DeleteChunksBySource(ctx context.Context, sourceID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM knowledge_chunks WHERE source_id = $1`, sourceID)
	if err != nil {
		return fmt.Errorf("delete chunks for %s: %w", sourceID, err)
	}
	return nil
}
