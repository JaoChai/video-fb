package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type VisualQARepo struct {
	pool *pgxpool.Pool
}

func NewVisualQARepo(pool *pgxpool.Pool) *VisualQARepo {
	return &VisualQARepo{pool: pool}
}

// Create appends one Visual QA row. issues is the JSON-encoded per-scene verdict
// array.
func (r *VisualQARepo) Create(ctx context.Context, clipID string, passed bool, issues []byte) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO visual_qa (clip_id, passed, issues) VALUES ($1, $2, $3)`,
		clipID, passed, issues)
	return err
}

// GetLatestByClipID returns the most recent Visual QA run for a clip — the one
// that drove its current status. Returns (nil, nil) when the clip has no QA row.
func (r *VisualQARepo) GetLatestByClipID(ctx context.Context, clipID string) (*models.VisualQA, error) {
	var qa models.VisualQA
	err := r.pool.QueryRow(ctx,
		`SELECT id, clip_id, passed, issues, created_at
		 FROM visual_qa WHERE clip_id = $1 ORDER BY created_at DESC LIMIT 1`, clipID).
		Scan(&qa.ID, &qa.ClipID, &qa.Passed, &qa.Issues, &qa.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest visual_qa for clip %s: %w", clipID, err)
	}
	return &qa, nil
}

// Stats returns the all-time Visual QA tally (total runs, passed, blocked).
// Rows persist after their clip is deleted, so blocked counts stay accurate.
func (r *VisualQARepo) Stats(ctx context.Context) (models.VisualQAStats, error) {
	var s models.VisualQAStats
	err := r.pool.QueryRow(ctx,
		`SELECT count(*),
		        count(*) FILTER (WHERE passed),
		        count(*) FILTER (WHERE NOT passed)
		 FROM visual_qa`).Scan(&s.Total, &s.Passed, &s.Blocked)
	if err != nil {
		return s, fmt.Errorf("visual_qa stats: %w", err)
	}
	return s, nil
}
