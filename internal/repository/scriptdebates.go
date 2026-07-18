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
// a nil verdict (judge skipped or failed) is stored as NULL — pgx encodes a
// nil []byte as SQL NULL.
func (r *ScriptDebatesRepo) Insert(ctx context.Context, clipID string, candidates, verdict []byte, source string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO script_debates (clip_id, candidates, verdict, source)
		 VALUES ($1, $2, $3, $4)`,
		clipID, candidates, verdict, source)
	return err
}
