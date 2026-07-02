package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type AutoReviewsRepo struct{ pool *pgxpool.Pool }

func NewAutoReviewsRepo(pool *pgxpool.Pool) *AutoReviewsRepo { return &AutoReviewsRepo{pool: pool} }

// Create appends one decision row. reasons is JSON-encoded []string.
func (r *AutoReviewsRepo) Create(ctx context.Context, clipID, decision, defectType string, confidence float64, reasons []byte) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO auto_reviews (clip_id, decision, confidence, defect_type, reasons)
		 VALUES ($1, $2, $3, $4, $5)`,
		clipID, decision, confidence, defectType, reasons)
	return err
}

// GetByClip returns the most recent decision for a clip, or (nil, nil) if none.
func (r *AutoReviewsRepo) GetByClip(ctx context.Context, clipID string) (*models.AutoReview, error) {
	var a models.AutoReview
	err := r.pool.QueryRow(ctx,
		`SELECT id, clip_id, decision, confidence, defect_type, reasons, created_at
		 FROM auto_reviews WHERE clip_id = $1 ORDER BY created_at DESC LIMIT 1`, clipID,
	).Scan(&a.ID, &a.ClipID, &a.Decision, &a.Confidence, &a.DefectType, &a.Reasons, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get auto_review for clip %s: %w", clipID, err)
	}
	return &a, nil
}
