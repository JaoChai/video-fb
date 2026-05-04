package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

const latestAnalyticsCTE = `WITH latest AS (
	SELECT DISTINCT ON (clip_id, platform)
		clip_id, views, likes, comments, shares,
		watch_time_seconds, retention_rate
	FROM clip_analytics
	ORDER BY clip_id, platform, fetched_at DESC
)`

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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate analytics: %w", err)
	}
	return results, nil
}

func (r *AnalyticsRepo) Summary(ctx context.Context) (models.AnalyticsSummary, error) {
	var s models.AnalyticsSummary
	err := r.pool.QueryRow(ctx, latestAnalyticsCTE+`
		SELECT COALESCE(SUM(l.views),0), COALESCE(SUM(l.likes),0),
			   COALESCE(SUM(l.comments),0), COALESCE(SUM(l.shares),0),
			   COALESCE(AVG(l.retention_rate),0), COALESCE(SUM(l.watch_time_seconds),0),
			   (SELECT COUNT(*) FROM clips WHERE status = 'published')
		FROM latest l`).Scan(
		&s.TotalViews, &s.TotalLikes, &s.TotalComments, &s.TotalShares,
		&s.AvgRetention, &s.TotalWatchTime, &s.ClipCount)
	if err != nil {
		return s, fmt.Errorf("query analytics summary: %w", err)
	}
	return s, nil
}

func (r *AnalyticsRepo) TopClips(ctx context.Context, limit int) ([]models.ClipPerformance, error) {
	rows, err := r.pool.Query(ctx, latestAnalyticsCTE+`
		SELECT c.id, c.title, c.category,
			   COALESCE(SUM(l.views),0) AS total_views,
			   COALESCE(SUM(l.likes),0), COALESCE(SUM(l.comments),0),
			   COALESCE(SUM(l.shares),0), COALESCE(AVG(l.retention_rate),0),
			   COALESCE(SUM(l.watch_time_seconds),0)
		FROM clips c
		LEFT JOIN latest l ON l.clip_id = c.id
		WHERE c.status = 'published'
		GROUP BY c.id, c.title, c.category
		ORDER BY total_views DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query top clips: %w", err)
	}
	defer rows.Close()

	var results []models.ClipPerformance
	for rows.Next() {
		var cp models.ClipPerformance
		if err := rows.Scan(&cp.ClipID, &cp.Title, &cp.Category,
			&cp.Views, &cp.Likes, &cp.Comments, &cp.Shares,
			&cp.RetentionRate, &cp.WatchTimeSeconds); err != nil {
			return nil, fmt.Errorf("scan top clip: %w", err)
		}
		results = append(results, cp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate top clips: %w", err)
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
