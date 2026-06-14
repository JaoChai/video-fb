package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
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
