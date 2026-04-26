package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type ScenesRepo struct {
	pool *pgxpool.Pool
}

func NewScenesRepo(pool *pgxpool.Pool) *ScenesRepo {
	return &ScenesRepo{pool: pool}
}

func (r *ScenesRepo) ListByClip(ctx context.Context, clipID string) ([]models.Scene, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, clip_id, scene_number, scene_type, text_content, image_prompt,
		        image_16_9_url, image_9_16_url, voice_text, duration_seconds, text_overlays
		 FROM scenes WHERE clip_id = $1 ORDER BY scene_number`, clipID)
	if err != nil {
		return nil, fmt.Errorf("query scenes: %w", err)
	}
	defer rows.Close()

	var scenes []models.Scene
	for rows.Next() {
		var s models.Scene
		if err := rows.Scan(&s.ID, &s.ClipID, &s.SceneNumber, &s.SceneType,
			&s.TextContent, &s.ImagePrompt, &s.Image169URL, &s.Image916URL,
			&s.VoiceText, &s.DurationSeconds, &s.TextOverlays); err != nil {
			return nil, fmt.Errorf("scan scene: %w", err)
		}
		scenes = append(scenes, s)
	}
	return scenes, nil
}

func (r *ScenesRepo) Create(ctx context.Context, req models.CreateSceneRequest) (*models.Scene, error) {
	var s models.Scene
	err := r.pool.QueryRow(ctx,
		`INSERT INTO scenes (clip_id, scene_number, scene_type, text_content, image_prompt, voice_text, duration_seconds, text_overlays)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, clip_id, scene_number, scene_type, text_content, image_prompt,
		           image_16_9_url, image_9_16_url, voice_text, duration_seconds, text_overlays`,
		req.ClipID, req.SceneNumber, req.SceneType, req.TextContent,
		req.ImagePrompt, req.VoiceText, req.DurationSeconds, req.TextOverlays,
	).Scan(&s.ID, &s.ClipID, &s.SceneNumber, &s.SceneType,
		&s.TextContent, &s.ImagePrompt, &s.Image169URL, &s.Image916URL,
		&s.VoiceText, &s.DurationSeconds, &s.TextOverlays)
	if err != nil {
		return nil, fmt.Errorf("create scene: %w", err)
	}
	return &s, nil
}

func (r *ScenesRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM scenes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete scene %s: %w", id, err)
	}
	return nil
}
