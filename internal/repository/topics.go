package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type TopicsRepo struct {
	pool *pgxpool.Pool
}

func NewTopicsRepo(pool *pgxpool.Pool) *TopicsRepo {
	return &TopicsRepo{pool: pool}
}

func (r *TopicsRepo) ListRecent(ctx context.Context, days int) ([]models.TopicHistory, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, category, created_at FROM topic_history
		 WHERE created_at >= $1 ORDER BY created_at DESC`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("query topics: %w", err)
	}
	defer rows.Close()

	var topics []models.TopicHistory
	for rows.Next() {
		var t models.TopicHistory
		if err := rows.Scan(&t.ID, &t.Title, &t.Category, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		topics = append(topics, t)
	}
	return topics, nil
}

func (r *TopicsRepo) Create(ctx context.Context, title, category string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO topic_history (title, category) VALUES ($1, $2)`, title, category)
	return err
}
