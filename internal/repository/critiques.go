package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CritiquesRepo struct {
	pool *pgxpool.Pool
}

func NewCritiquesRepo(pool *pgxpool.Pool) *CritiquesRepo {
	return &CritiquesRepo{pool: pool}
}

// Create appends one critique row. score and changes are JSON-encoded bytes.
func (r *CritiquesRepo) Create(ctx context.Context, clipID string, score, changes []byte, applied bool) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_critiques (clip_id, score, changes, applied) VALUES ($1, $2, $3, $4)`,
		clipID, score, changes, applied)
	return err
}
