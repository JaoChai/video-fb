package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

// latestAnalyticsCTE takes the newest analytics row per (clip, platform,
// post_type), excluding posts whose publish failed — a failed post never went
// live, so its default 0-view row would otherwise pollute totals and inflate
// the "0 views" count as if it were live-but-unwatched content.
const latestAnalyticsCTE = `WITH latest AS (
	SELECT DISTINCT ON (clip_id, platform, post_type)
		clip_id, platform, post_type, views, likes, comments, shares,
		watch_time_seconds, retention_rate,
		engagement_rate, avg_view_percentage, subscribers_gained
	FROM clip_analytics
	WHERE NOT EXISTS (
		SELECT 1 FROM clip_publish_status ps
		WHERE ps.clip_id = clip_analytics.clip_id
		  AND ps.platform = clip_analytics.platform
		  AND ps.post_type = clip_analytics.post_type
		  AND ps.status = 'failed')
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
		 FROM clip_analytics ca WHERE clip_id = $1
		   AND NOT EXISTS (
			SELECT 1 FROM clip_publish_status ps
			WHERE ps.clip_id = ca.clip_id AND ps.platform = ca.platform
			  AND ps.post_type = ca.post_type AND ps.status = 'failed')
		 ORDER BY fetched_at DESC`, clipID)
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
		       COALESCE(SUM(l.watch_time_seconds),0),
		       COALESCE(AVG(NULLIF(l.avg_view_percentage, 0)), AVG(NULLIF(l.retention_rate, 0)), 0),
		       COALESCE(AVG(NULLIF(l.engagement_rate, 0)), 0),
		       COALESCE(SUM(l.subscribers_gained), 0)
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
			&p.Shares, &p.WatchTimeSeconds, &p.AvgRetention,
			&p.EngagementRate, &p.SubscribersGained); err != nil {
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

// PresetRetention returns the mean latest retention_rate per style_preset over the
// last windowDays, across all platforms, for published clips that carry a preset
// and have analytics. Presets without qualifying analytics simply do not appear.
func (r *AnalyticsRepo) PresetRetention(ctx context.Context, windowDays int) ([]models.PresetScore, error) {
	rows, err := r.pool.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (clip_id, platform, post_type)
				clip_id, platform, post_type, retention_rate, fetched_at
			FROM clip_analytics
			WHERE fetched_at >= NOW() - make_interval(days => $1)
			  AND NOT EXISTS (
				SELECT 1 FROM clip_publish_status ps
				WHERE ps.clip_id = clip_analytics.clip_id
				  AND ps.platform = clip_analytics.platform
				  AND ps.post_type = clip_analytics.post_type
				  AND ps.status = 'failed')
			ORDER BY clip_id, platform, post_type, fetched_at DESC
		)
		SELECT c.style_preset,
		       COALESCE(AVG(NULLIF(l.retention_rate, 0)), 0) AS avg_ret,
		       COUNT(DISTINCT l.clip_id) AS n
		FROM clips c
		JOIN latest l ON l.clip_id = c.id
		WHERE c.style_preset <> ''
		GROUP BY c.style_preset`, windowDays)
	if err != nil {
		return nil, fmt.Errorf("preset retention: %w", err)
	}
	defer rows.Close()

	out := []models.PresetScore{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		var s models.PresetScore
		if err := rows.Scan(&s.Preset, &s.AvgRetention, &s.N); err != nil {
			return nil, fmt.Errorf("scan preset score: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate preset scores: %w", err)
	}
	return out, nil
}

func (r *AnalyticsRepo) Create(ctx context.Context, a models.ClipAnalytics) error {
	if a.PostType == "" {
		a.PostType = "regular"
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_analytics (clip_id, platform, post_type, views, likes, comments, shares,
		    watch_time_seconds, retention_rate, engagement_rate, avg_view_percentage,
		    subscribers_gained, subscribers_lost)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		a.ClipID, a.Platform, a.PostType, a.Views, a.Likes, a.Comments, a.Shares,
		a.WatchTimeSeconds, a.RetentionRate, a.EngagementRate, a.AvgViewPercentage,
		a.SubscribersGained, a.SubscribersLost)
	if err != nil {
		return fmt.Errorf("create analytics: %w", err)
	}
	return nil
}

// UpsertDaily records one day of YouTube analytics for a post. Re-fetches
// overwrite the same (clip, platform, post_type, date) row because Zernio's
// recent days keep moving for ~48h.
func (r *AnalyticsRepo) UpsertDaily(ctx context.Context, d models.ClipAnalyticsDaily) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_analytics_daily
		    (clip_id, platform, post_type, date, views, estimated_minutes_watched,
		     average_view_duration, avg_view_percentage, subscribers_gained,
		     subscribers_lost, likes, comments, shares)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 ON CONFLICT (clip_id, platform, post_type, date) DO UPDATE SET
		    views = EXCLUDED.views,
		    estimated_minutes_watched = EXCLUDED.estimated_minutes_watched,
		    average_view_duration = EXCLUDED.average_view_duration,
		    avg_view_percentage = EXCLUDED.avg_view_percentage,
		    subscribers_gained = EXCLUDED.subscribers_gained,
		    subscribers_lost = EXCLUDED.subscribers_lost,
		    likes = EXCLUDED.likes,
		    comments = EXCLUDED.comments,
		    shares = EXCLUDED.shares`,
		d.ClipID, d.Platform, d.PostType, d.Date, d.Views, d.EstimatedMinutesWatched,
		d.AverageViewDuration, d.AvgViewPercentage, d.SubscribersGained,
		d.SubscribersLost, d.Likes, d.Comments, d.Shares)
	if err != nil {
		return fmt.Errorf("upsert daily analytics: %w", err)
	}
	return nil
}

// UpsertPublishStatus records the last-seen publish outcome of one Zernio post.
func (r *AnalyticsRepo) UpsertPublishStatus(ctx context.Context, s models.ClipPublishStatus) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_publish_status (clip_id, platform, post_type, zernio_post_id, status, error_message, checked_at)
		 VALUES ($1,$2,$3,$4,$5,$6,NOW())
		 ON CONFLICT (clip_id, platform, post_type) DO UPDATE SET
		    zernio_post_id = EXCLUDED.zernio_post_id,
		    status = EXCLUDED.status,
		    error_message = EXCLUDED.error_message,
		    checked_at = NOW()`,
		s.ClipID, s.Platform, s.PostType, s.ZernioPostID, s.Status, s.ErrorMessage)
	if err != nil {
		return fmt.Errorf("upsert publish status: %w", err)
	}
	return nil
}

// TopicPerformance scores each clip category by its clips' mean within-platform
// views percentile over the last windowDays, excluding failed publishes.
// Posts whose analytics we first saw under 3 days ago are excluded — they have
// not had time to accumulate views (first-snapshot age is the post-age proxy;
// drip-published backlog clips were created long before they were posted).
// Categories with fewer than minClips measurable clips are omitted.
func (r *AnalyticsRepo) TopicPerformance(ctx context.Context, windowDays, minClips int) ([]models.CategoryScore, error) {
	rows, err := r.pool.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (ca.clip_id, ca.platform)
				ca.clip_id, ca.platform, ca.views,
				MIN(ca.fetched_at) OVER (PARTITION BY ca.clip_id, ca.platform) AS first_seen
			FROM clip_analytics ca
			WHERE ca.fetched_at >= NOW() - make_interval(days => $1)
			  AND ca.platform IN ('youtube', 'tiktok')
			ORDER BY ca.clip_id, ca.platform, ca.fetched_at DESC
		), ranked AS (
			SELECT l.clip_id, l.views,
			       PERCENT_RANK() OVER (PARTITION BY l.platform ORDER BY l.views) AS pct
			FROM latest l
			WHERE l.first_seen <= NOW() - INTERVAL '3 days'
			  AND NOT EXISTS (
				SELECT 1 FROM clip_publish_status ps
				WHERE ps.clip_id = l.clip_id AND ps.platform = l.platform AND ps.status = 'failed')
		)
		SELECT c.category,
		       AVG(r.pct), AVG(r.views), COUNT(DISTINCT r.clip_id)
		FROM ranked r
		JOIN clips c ON c.id = r.clip_id
		WHERE c.status = 'published'
		GROUP BY c.category
		HAVING COUNT(DISTINCT r.clip_id) >= $2
		ORDER BY AVG(r.pct) DESC`, windowDays, minClips)
	if err != nil {
		return nil, fmt.Errorf("topic performance: %w", err)
	}
	defer rows.Close()

	out := []models.CategoryScore{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		var s models.CategoryScore
		if err := rows.Scan(&s.Category, &s.AvgPercentile, &s.AvgViews, &s.N); err != nil {
			return nil, fmt.Errorf("scan category score: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate category scores: %w", err)
	}
	return out, nil
}

// PublishFailures lists posts whose last-seen Zernio status is 'failed'.
func (r *AnalyticsRepo) PublishFailures(ctx context.Context) ([]models.PublishFailure, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ps.clip_id, c.title, ps.platform, ps.post_type,
		       COALESCE(ps.error_message, ''), ps.checked_at
		FROM clip_publish_status ps
		JOIN clips c ON c.id = ps.clip_id
		WHERE ps.status = 'failed'
		ORDER BY ps.checked_at DESC
		LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("publish failures: %w", err)
	}
	defer rows.Close()

	out := []models.PublishFailure{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		var f models.PublishFailure
		if err := rows.Scan(&f.ClipID, &f.Title, &f.Platform, &f.PostType, &f.ErrorMessage, &f.CheckedAt); err != nil {
			return nil, fmt.Errorf("scan publish failure: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate publish failures: %w", err)
	}
	return out, nil
}

// Sparklines returns per-clip daily view deltas (all platforms summed, oldest→newest)
// over the last `days` days, derived from the daily snapshot maxima.
func (r *AnalyticsRepo) Sparklines(ctx context.Context, days int) (map[string][]int, error) {
	rows, err := r.pool.Query(ctx, `
		WITH per_day AS (
			SELECT clip_id, platform, post_type,
			       DATE_TRUNC('day', fetched_at)::date AS day, MAX(views) AS views
			FROM clip_analytics
			WHERE fetched_at >= NOW() - make_interval(days => $1)
			GROUP BY clip_id, platform, post_type, day
		)
		SELECT clip_id, day, SUM(views)::int
		FROM per_day
		GROUP BY clip_id, day
		ORDER BY clip_id, day ASC`, days)
	if err != nil {
		return nil, fmt.Errorf("sparklines: %w", err)
	}
	defer rows.Close()

	cumulative := map[string][]int{}
	for rows.Next() {
		var clipID string
		var day time.Time
		var views int
		if err := rows.Scan(&clipID, &day, &views); err != nil {
			return nil, fmt.Errorf("scan sparkline row: %w", err)
		}
		cumulative[clipID] = append(cumulative[clipID], views)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sparkline rows: %w", err)
	}

	// Convert cumulative snapshots to daily deltas (clamped at 0 — platform
	// corrections can shrink counts and a negative bar is meaningless).
	out := map[string][]int{}
	for clipID, series := range cumulative {
		deltas := make([]int, 0, len(series))
		for i := 1; i < len(series); i++ {
			d := series[i] - series[i-1]
			if d < 0 {
				d = 0
			}
			deltas = append(deltas, d)
		}
		out[clipID] = deltas
	}
	return out, nil
}
