package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type ClipsRepo struct {
	pool *pgxpool.Pool
}

func NewClipsRepo(pool *pgxpool.Pool) *ClipsRepo {
	return &ClipsRepo{pool: pool}
}

const clipColumns = `id, title, question, questioner_name, answer_script, voice_script,
	category, status, video_16_9_url, video_9_16_url, thumbnail_url,
	publish_date::text, created_at, updated_at, fail_reason, retry_count, style_preset, content_format,
	production_stage, review_retry_count, auto_review_held`

func scanClip(scanner interface{ Scan(dest ...any) error }) (models.Clip, error) {
	var c models.Clip
	err := scanner.Scan(
		&c.ID, &c.Title, &c.Question, &c.QuestionerName,
		&c.AnswerScript, &c.VoiceScript, &c.Category, &c.Status,
		&c.Video169URL, &c.Video916URL, &c.ThumbnailURL,
		&c.PublishDate, &c.CreatedAt, &c.UpdatedAt,
		&c.FailReason, &c.RetryCount, &c.StylePreset, &c.ContentFormat,
		&c.ProductionStage, &c.ReviewRetryCount, &c.AutoReviewHeld,
	)
	return c, err
}

func (r *ClipsRepo) List(ctx context.Context) ([]models.Clip, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+clipColumns+` FROM clips ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("query clips: %w", err)
	}
	defer rows.Close()

	var clips []models.Clip
	for rows.Next() {
		c, err := scanClip(rows)
		if err != nil {
			return nil, fmt.Errorf("scan clip: %w", err)
		}
		clips = append(clips, c)
	}
	return clips, nil
}

func (r *ClipsRepo) GetByID(ctx context.Context, id string) (*models.Clip, error) {
	c, err := scanClip(r.pool.QueryRow(ctx,
		`SELECT `+clipColumns+` FROM clips WHERE id = $1`, id))
	if err != nil {
		return nil, fmt.Errorf("get clip %s: %w", id, err)
	}
	return &c, nil
}

func (r *ClipsRepo) Create(ctx context.Context, req models.CreateClipRequest) (*models.Clip, error) {
	c, err := scanClip(r.pool.QueryRow(ctx,
		`INSERT INTO clips (title, question, questioner_name, category, publish_date, content_format)
		 VALUES ($1, $2, $3, $4, $5::date, COALESCE(NULLIF($6, ''), 'qa'))
		 RETURNING `+clipColumns,
		req.Title, req.Question, req.QuestionerName, req.Category, req.PublishDate, req.ContentFormat,
	))
	if err != nil {
		return nil, fmt.Errorf("create clip: %w", err)
	}
	return &c, nil
}

func (r *ClipsRepo) Update(ctx context.Context, id string, req models.UpdateClipRequest) (*models.Clip, error) {
	c, err := scanClip(r.pool.QueryRow(ctx,
		`UPDATE clips SET
			title = COALESCE($2, title),
			question = COALESCE($3, question),
			questioner_name = COALESCE($4, questioner_name),
			answer_script = COALESCE($5, answer_script),
			voice_script = COALESCE($6, voice_script),
			category = COALESCE($7, category),
			status = COALESCE($8, status),
			video_16_9_url = COALESCE($9, video_16_9_url),
			video_9_16_url = COALESCE($10, video_9_16_url),
			thumbnail_url = COALESCE($11, thumbnail_url),
			publish_date = COALESCE($12::date, publish_date),
			style_preset = COALESCE($13, style_preset),
			production_stage = COALESCE($14, production_stage),
			updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+clipColumns,
		id, req.Title, req.Question, req.QuestionerName,
		req.AnswerScript, req.VoiceScript, req.Category, req.Status,
		req.Video169URL, req.Video916URL, req.ThumbnailURL, req.PublishDate,
		req.StylePreset,
		req.ProductionStage,
	))
	if err != nil {
		return nil, fmt.Errorf("update clip %s: %w", id, err)
	}
	return &c, nil
}

func (r *ClipsRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM clips WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete clip %s: %w", id, err)
	}
	return nil
}

// ResetStaleProducing marks every clip stuck in 'producing' as failed. Production
// runs as a detached in-process goroutine (see OrchestratorHandler.TriggerWeekly),
// so a restart/crash/deploy mid-production orphans its clip in 'producing' forever
// — the goroutine dies before it can set 'ready'/'failed'. Called once at startup,
// where any 'producing' clip is necessarily stale. Returns the number reset.
func (r *ClipsRepo) ResetStaleProducing(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE clips
		 SET status = 'failed',
		     fail_reason = 'การผลิตถูกขัดจังหวะ (เซิร์ฟเวอร์รีสตาร์ท) — กด Retry เพื่อลองใหม่',
		     retry_count = retry_count + 1,
		     updated_at = NOW()
		 WHERE status = 'producing'`)
	if err != nil {
		return 0, fmt.Errorf("reset stale producing clips: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *ClipsRepo) ListFailed(ctx context.Context, maxRetries int, cooldownMinutes int) ([]models.Clip, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+clipColumns+` FROM clips
		 WHERE status = 'failed' AND retry_count < $1
		   AND updated_at < NOW() - make_interval(mins => $2)
		 ORDER BY created_at ASC LIMIT 5`, maxRetries, cooldownMinutes)
	if err != nil {
		return nil, fmt.Errorf("query failed clips: %w", err)
	}
	defer rows.Close()

	var clips []models.Clip
	for rows.Next() {
		c, err := scanClip(rows)
		if err != nil {
			return nil, fmt.Errorf("scan failed clip: %w", err)
		}
		clips = append(clips, c)
	}
	return clips, nil
}

func (r *ClipsRepo) IncrementRetry(ctx context.Context, id, reason string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE clips SET retry_count = retry_count + 1, fail_reason = $2, updated_at = NOW()
		 WHERE id = $1`, id, reason)
	if err != nil {
		return fmt.Errorf("increment retry for clip %s: %w", id, err)
	}
	return nil
}

// ListNeedsReview returns needs_review clips eligible for auto-review: not held
// and under the review-retry cap, oldest first, capped at `limit`.
func (r *ClipsRepo) ListNeedsReview(ctx context.Context, retryCap, limit int) ([]models.Clip, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+clipColumns+` FROM clips
		 WHERE status = 'needs_review' AND auto_review_held = FALSE AND review_retry_count < $1
		 ORDER BY created_at ASC LIMIT $2`, retryCap, limit)
	if err != nil {
		return nil, fmt.Errorf("query needs_review clips: %w", err)
	}
	defer rows.Close()
	var clips []models.Clip
	for rows.Next() {
		c, err := scanClip(rows)
		if err != nil {
			return nil, fmt.Errorf("scan needs_review clip: %w", err)
		}
		clips = append(clips, c)
	}
	return clips, nil
}

// SetAutoReviewHeld marks a clip as held so the auto-review tick stops re-judging it.
func (r *ClipsRepo) SetAutoReviewHeld(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE clips SET auto_review_held = TRUE, updated_at = NOW() WHERE id = $1`, id)
	return err
}

// ClearAutoReviewHeld lifts an auto-review hold AND promotes the clip to 'ready'
// so it actually becomes publishable. A held clip is either 'needs_review'
// (fresh auto-review hold — recordHeld only flips the flag, never the status) or a
// stale 'ready'+held row; in both cases the human "override & publish" intent is
// to make it publishable, so clearing the flag alone isn't enough — the publisher
// gates on status='ready'. Guarded to those two statuses so an unrelated state
// (producing/published/failed) is never clobbered.
func (r *ClipsRepo) ClearAutoReviewHeld(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE clips SET auto_review_held = FALSE, status = 'ready', updated_at = NOW()
		 WHERE id = $1 AND status IN ('needs_review', 'ready')`, id)
	return err
}

// IncrementReviewRetry bumps the review-retry counter (separate from retry_count).
func (r *ClipsRepo) IncrementReviewRetry(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE clips SET review_retry_count = review_retry_count + 1, updated_at = NOW() WHERE id = $1`, id)
	return err
}

// ClearFailReason wipes a stale fail_reason once a clip has successfully
// produced a video (status ready or needs_review), so a recovered clip doesn't
// keep showing the error that failed it.
func (r *ClipsRepo) ClearFailReason(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE clips SET fail_reason = NULL WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("clear fail_reason for clip %s: %w", id, err)
	}
	return nil
}

func (r *ClipsRepo) DeleteOldFailed(ctx context.Context, maxRetries int) (int, error) {
	result, err := r.pool.Exec(ctx,
		`DELETE FROM clips WHERE status = 'failed' AND retry_count >= $1
		 AND updated_at < NOW() - INTERVAL '24 hours'`, maxRetries)
	if err != nil {
		return 0, fmt.Errorf("delete old failed clips: %w", err)
	}
	return int(result.RowsAffected()), nil
}

func (r *ClipsRepo) CountConsecutiveFailed(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM (
			SELECT status FROM clips ORDER BY created_at DESC LIMIT 5
		) recent WHERE status = 'failed'`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count consecutive failed: %w", err)
	}
	return count, nil
}

// LastStylePreset returns the style_preset of the most recently created clip,
// or "" if there are none. Used to avoid repeating a look on the next clip.
func (r *ClipsRepo) LastStylePreset(ctx context.Context) (string, error) {
	var key string
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(style_preset, '') FROM clips ORDER BY created_at DESC LIMIT 1`).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("last style preset: %w", err)
	}
	return key, nil
}

func (r *ClipsRepo) UpsertMetadata(ctx context.Context, m models.ClipMetadata) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_metadata (clip_id, youtube_title, youtube_description, youtube_tags)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (clip_id) DO UPDATE SET youtube_title=$2, youtube_description=$3, youtube_tags=$4`,
		m.ClipID, m.YoutubeTitle, m.YoutubeDesc, m.YoutubeTags)
	if err != nil {
		return fmt.Errorf("upsert metadata for clip %s: %w", m.ClipID, err)
	}
	return nil
}
