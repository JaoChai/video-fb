package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

const latestAnalyticsCTE = `WITH latest AS (
	SELECT DISTINCT ON (clip_id, platform, post_type)
		clip_id, platform, post_type, views, likes, comments, shares,
		watch_time_seconds, retention_rate
	FROM clip_analytics
	ORDER BY clip_id, platform, post_type, fetched_at DESC
)`

type AnalyticsRepo struct {
	pool *pgxpool.Pool
}

func NewAnalyticsRepo(pool *pgxpool.Pool) *AnalyticsRepo {
	return &AnalyticsRepo{pool: pool}
}

func (r *AnalyticsRepo) ListByClip(ctx context.Context, clipID string) ([]models.ClipAnalytics, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, clip_id, platform, post_type, views, likes, comments, shares,
		        watch_time_seconds, retention_rate, fetched_at
		 FROM clip_analytics WHERE clip_id = $1 ORDER BY fetched_at DESC`, clipID)
	if err != nil {
		return nil, fmt.Errorf("query analytics: %w", err)
	}
	defer rows.Close()

	var results []models.ClipAnalytics
	for rows.Next() {
		var a models.ClipAnalytics
		if err := rows.Scan(&a.ID, &a.ClipID, &a.Platform, &a.PostType, &a.Views, &a.Likes,
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
			   COALESCE(AVG(NULLIF(l.retention_rate, 0)),0),
			   COALESCE(SUM(l.watch_time_seconds),0),
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
			   COALESCE(SUM(l.shares),0), COALESCE(AVG(NULLIF(l.retention_rate, 0)),0),
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

func (r *AnalyticsRepo) LastFetchedAt(ctx context.Context) (*time.Time, error) {
	var t *time.Time
	err := r.pool.QueryRow(ctx, `SELECT MAX(fetched_at) FROM clip_analytics`).Scan(&t)
	if err != nil {
		return nil, fmt.Errorf("query last fetched: %w", err)
	}
	return t, nil
}

func (r *AnalyticsRepo) SummaryByPostType(ctx context.Context) ([]models.SegmentedTotals, error) {
	rows, err := r.pool.Query(ctx, latestAnalyticsCTE+`
		SELECT l.post_type,
		       COALESCE(SUM(l.views),0),
		       COALESCE(SUM(l.likes),0),
		       COALESCE(SUM(l.comments),0),
		       COALESCE(SUM(l.shares),0),
		       COALESCE(SUM(l.watch_time_seconds),0),
		       COALESCE(AVG(NULLIF(l.retention_rate, 0)),0)
		FROM latest l
		GROUP BY l.post_type
		ORDER BY l.post_type`)
	if err != nil {
		return nil, fmt.Errorf("query summary by post_type: %w", err)
	}
	defer rows.Close()
	var out []models.SegmentedTotals
	for rows.Next() {
		var s models.SegmentedTotals
		if err := rows.Scan(&s.PostType, &s.Views, &s.Likes, &s.Comments,
			&s.Shares, &s.WatchTimeSeconds, &s.AvgRetention); err != nil {
			return nil, fmt.Errorf("scan post_type row: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate post_type summary: %w", err)
	}
	return out, nil
}

func (r *AnalyticsRepo) SummaryByPlatform(ctx context.Context) ([]models.PlatformTotals, error) {
	rows, err := r.pool.Query(ctx, latestAnalyticsCTE+`
		SELECT l.platform,
		       COALESCE(SUM(l.views),0),
		       COALESCE(SUM(l.likes),0),
		       COALESCE(SUM(l.comments),0),
		       COALESCE(SUM(l.shares),0),
		       COALESCE(SUM(l.watch_time_seconds),0)
		FROM latest l
		GROUP BY l.platform
		ORDER BY SUM(l.views) DESC`)
	if err != nil {
		return nil, fmt.Errorf("query summary by platform: %w", err)
	}
	defer rows.Close()
	var out []models.PlatformTotals
	for rows.Next() {
		var p models.PlatformTotals
		if err := rows.Scan(&p.Platform, &p.Views, &p.Likes, &p.Comments,
			&p.Shares, &p.WatchTimeSeconds); err != nil {
			return nil, fmt.Errorf("scan platform row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate platform summary: %w", err)
	}
	return out, nil
}

func (r *AnalyticsRepo) Trend(ctx context.Context, days int) ([]models.TrendPoint, error) {
	rows, err := r.pool.Query(ctx, `
		WITH daily AS (
			SELECT DISTINCT ON (clip_id, platform, post_type, DATE_TRUNC('day', fetched_at))
				clip_id, platform, post_type,
				DATE_TRUNC('day', fetched_at) AS day,
				views, likes, comments, shares, watch_time_seconds, retention_rate
			FROM clip_analytics
			WHERE fetched_at >= NOW() - ($1::int || ' days')::interval
			ORDER BY clip_id, platform, post_type, DATE_TRUNC('day', fetched_at), fetched_at DESC
		)
		SELECT day,
		       COALESCE(SUM(views),0),
		       COALESCE(SUM(likes),0),
		       COALESCE(SUM(comments),0),
		       COALESCE(SUM(shares),0),
		       COALESCE(SUM(watch_time_seconds),0),
		       COALESCE(AVG(NULLIF(retention_rate, 0)),0)
		FROM daily
		GROUP BY day
		ORDER BY day ASC`, days)
	if err != nil {
		return nil, fmt.Errorf("query trend: %w", err)
	}
	defer rows.Close()
	var out []models.TrendPoint
	for rows.Next() {
		var p models.TrendPoint
		if err := rows.Scan(&p.Day, &p.Views, &p.Likes, &p.Comments,
			&p.Shares, &p.WatchTime, &p.Retention); err != nil {
			return nil, fmt.Errorf("scan trend row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trend: %w", err)
	}
	return out, nil
}

func (r *AnalyticsRepo) PreviousPeriodTotals(ctx context.Context, days int) (models.AnalyticsSummary, error) {
	var s models.AnalyticsSummary
	err := r.pool.QueryRow(ctx, `
		WITH prev AS (
			SELECT DISTINCT ON (clip_id, platform, post_type)
				clip_id, platform, post_type,
				views, likes, comments, shares, watch_time_seconds, retention_rate
			FROM clip_analytics
			WHERE fetched_at < NOW() - ($1::int || ' days')::interval
			  AND fetched_at >= NOW() - (($1::int * 2) || ' days')::interval
			ORDER BY clip_id, platform, post_type, fetched_at DESC
		)
		SELECT COALESCE(SUM(views),0), COALESCE(SUM(likes),0),
		       COALESCE(SUM(comments),0), COALESCE(SUM(shares),0),
		       COALESCE(AVG(NULLIF(retention_rate, 0)),0),
		       COALESCE(SUM(watch_time_seconds),0),
		       0
		FROM prev`, days).Scan(
		&s.TotalViews, &s.TotalLikes, &s.TotalComments, &s.TotalShares,
		&s.AvgRetention, &s.TotalWatchTime, &s.ClipCount)
	if err != nil {
		return s, fmt.Errorf("query previous period: %w", err)
	}
	return s, nil
}

func (r *AnalyticsRepo) Create(ctx context.Context, a models.ClipAnalytics) error {
	if a.PostType == "" {
		a.PostType = "regular"
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_analytics (clip_id, platform, post_type, views, likes, comments, shares, watch_time_seconds, retention_rate)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		a.ClipID, a.Platform, a.PostType, a.Views, a.Likes, a.Comments, a.Shares, a.WatchTimeSeconds, a.RetentionRate)
	if err != nil {
		return fmt.Errorf("create analytics: %w", err)
	}
	return nil
}
