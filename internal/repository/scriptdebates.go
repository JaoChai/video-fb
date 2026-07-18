package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ScriptDebatesRepo struct{ pool *pgxpool.Pool }

func NewScriptDebatesRepo(pool *pgxpool.Pool) *ScriptDebatesRepo {
	return &ScriptDebatesRepo{pool: pool}
}

// Insert appends one debate audit row. candidates/verdict are JSON-encoded;
// verdict may be nil when the judge was skipped (single candidate) or failed.
func (r *ScriptDebatesRepo) Insert(ctx context.Context, clipID string, candidates, verdict []byte, source string) error {
	if verdict == nil {
		_, err := r.pool.Exec(ctx,
			`INSERT INTO script_debates (clip_id, candidates, verdict, source)
			 VALUES ($1, $2, NULL, $3)`,
			clipID, candidates, source)
		return err
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO script_debates (clip_id, candidates, verdict, source)
		 VALUES ($1, $2, $3, $4)`,
		clipID, candidates, verdict, source)
	return err
}
