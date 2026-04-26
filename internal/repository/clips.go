package repository

import (
	"context"
	"fmt"

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
	publish_date::text, created_at, updated_at`

func scanClip(scanner interface{ Scan(dest ...any) error }) (models.Clip, error) {
	var c models.Clip
	err := scanner.Scan(
		&c.ID, &c.Title, &c.Question, &c.QuestionerName,
		&c.AnswerScript, &c.VoiceScript, &c.Category, &c.Status,
		&c.Video169URL, &c.Video916URL, &c.ThumbnailURL,
		&c.PublishDate, &c.CreatedAt, &c.UpdatedAt,
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
		`INSERT INTO clips (title, question, questioner_name, category, publish_date)
		 VALUES ($1, $2, $3, $4, $5::date)
		 RETURNING `+clipColumns,
		req.Title, req.Question, req.QuestionerName, req.Category, req.PublishDate,
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
			updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+clipColumns,
		id, req.Title, req.Question, req.QuestionerName,
		req.AnswerScript, req.VoiceScript, req.Category, req.Status,
		req.Video169URL, req.Video916URL, req.ThumbnailURL, req.PublishDate,
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
