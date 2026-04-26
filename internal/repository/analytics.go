package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type AnalyticsRepo struct {
	pool *pgxpool.Pool
}

func NewAnalyticsRepo(pool *pgxpool.Pool) *AnalyticsRepo {
	return &AnalyticsRepo{pool: pool}
}

func (r *AnalyticsRepo) ListByClip(ctx context.Context, clipID string) ([]models.ClipAnalytics, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, clip_id, platform, views, likes, comments, shares,
		        watch_time_seconds, retention_rate, fetched_at
		 FROM clip_analytics WHERE clip_id = $1 ORDER BY fetched_at DESC`, clipID)
	if err != nil {
		return nil, fmt.Errorf("query analytics: %w", err)
	}
	defer rows.Close()

	var results []models.ClipAnalytics
	for rows.Next() {
		var a models.ClipAnalytics
		if err := rows.Scan(&a.ID, &a.ClipID, &a.Platform, &a.Views, &a.Likes,
			&a.Comments, &a.Shares, &a.WatchTimeSeconds, &a.RetentionRate, &a.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan analytics: %w", err)
		}
		results = append(results, a)
	}
	return results, nil
}

func (r *AnalyticsRepo) Create(ctx context.Context, a models.ClipAnalytics) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_analytics (clip_id, platform, views, likes, comments, shares, watch_time_seconds, retention_rate)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ClipID, a.Platform, a.Views, a.Likes, a.Comments, a.Shares, a.WatchTimeSeconds, a.RetentionRate)
	if err != nil {
		return fmt.Errorf("create analytics: %w", err)
	}
	return nil
}
