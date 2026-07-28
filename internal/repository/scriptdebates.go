package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
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

// GetByClip คืนการดีเบตล่าสุดของคลิป หรือ (nil, nil) เมื่อคลิปนั้นผลิตตอน
// flag script_debate_enabled ปิดอยู่ จึงไม่มีแถว
func (r *ScriptDebatesRepo) GetByClip(ctx context.Context, clipID string) (*models.ScriptDebate, error) {
	var d models.ScriptDebate
	err := r.pool.QueryRow(ctx,
		`SELECT id, clip_id, candidates, verdict, source, created_at
		 FROM script_debates WHERE clip_id = $1 ORDER BY created_at DESC LIMIT 1`, clipID).
		Scan(&d.ID, &d.ClipID, &d.Candidates, &d.Verdict, &d.Source, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get script_debate for clip %s: %w", clipID, err)
	}
	return &d, nil
}
