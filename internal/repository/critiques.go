package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)



type CritiquesRepo struct {
	pool *pgxpool.Pool
}

func NewCritiquesRepo(pool *pgxpool.Pool) *CritiquesRepo {
	return &CritiquesRepo{pool: pool}
}

// GetByClip returns the most recent critique for a clip, or (nil, nil) when none exists.
func (r *CritiquesRepo) GetByClip(ctx context.Context, clipID string) (*models.ClipCritique, error) {
	var c models.ClipCritique
	err := r.pool.QueryRow(ctx,
		`SELECT clip_id, score, changes, applied, created_at
		 FROM clip_critiques WHERE clip_id = $1 ORDER BY created_at DESC LIMIT 1`, clipID).
		Scan(&c.ClipID, &c.Score, &c.Changes, &c.Applied, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get critique by clip: %w", err)
	}
	return &c, nil
}

// Create appends one critique row. score and changes are JSON-encoded bytes.
func (r *CritiquesRepo) Create(ctx context.Context, clipID string, score, changes []byte, applied bool) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_critiques (clip_id, score, changes, applied) VALUES ($1, $2, $3, $4)`,
		clipID, score, changes, applied)
	return err
}

// FieldIssue is one recurring critic edit (a changes[] entry) and how often it
// occurred in the window.
type FieldIssue struct {
	Field  string
	Reason string
	Count  int
}

// ScorePatterns is the aggregated quality signal over a recent critique window.
// Avg* are the mean of each score dimension (0 when N == 0). N is how many
// critique rows fell in the window. TopIssues are the most common changes[]
// field+reason pairs, most frequent first.
type ScorePatterns struct {
	N           int
	AvgHook     float64
	AvgClarity  float64
	AvgBrandFit float64
	AvgOverall  float64
	TopIssues   []FieldIssue
}

// LowScorePatterns aggregates clip_critiques over the last sinceDays days:
// per-dimension score averages + count, plus the most common changes[] entries.
// topN caps how many recurring issues are returned (0 or negative -> 10).
func (r *CritiquesRepo) LowScorePatterns(ctx context.Context, sinceDays, topN int) (ScorePatterns, error) {
	if topN <= 0 {
		topN = 10
	}
	var p ScorePatterns

	// Per-dimension averages + count over the window (applied rows only).
	err := r.pool.QueryRow(ctx, `
SELECT
  COUNT(*)                                                       AS n,
  COALESCE(AVG((score->>'hook')::numeric),      0)              AS avg_hook,
  COALESCE(AVG((score->>'clarity')::numeric),   0)              AS avg_clarity,
  COALESCE(AVG((score->>'brand_fit')::numeric), 0)              AS avg_brand_fit,
  COALESCE(AVG((score->>'overall')::numeric),   0)              AS avg_overall
FROM clip_critiques
WHERE created_at >= NOW() - make_interval(days => $1)
  AND applied = TRUE`,
		sinceDays,
	).Scan(&p.N, &p.AvgHook, &p.AvgClarity, &p.AvgBrandFit, &p.AvgOverall)
	if err != nil {
		return ScorePatterns{}, fmt.Errorf("aggregate score patterns: %w", err)
	}

	if p.N == 0 {
		return p, nil
	}

	// Most common changes[] field+reason pairs over the same window (applied rows only).
	rows, err := r.pool.Query(ctx, `
SELECT
  c->>'field'  AS field,
  c->>'reason' AS reason,
  COUNT(*)     AS cnt
FROM clip_critiques cc,
     LATERAL jsonb_array_elements(cc.changes) AS c
WHERE cc.created_at >= NOW() - make_interval(days => $1)
  AND cc.applied = TRUE
  AND c->>'field' IS NOT NULL
GROUP BY c->>'field', c->>'reason'
ORDER BY cnt DESC, field ASC
LIMIT $2`,
		sinceDays, topN,
	)
	if err != nil {
		return ScorePatterns{}, fmt.Errorf("aggregate top issues: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var fi FieldIssue
		if err := rows.Scan(&fi.Field, &fi.Reason, &fi.Count); err != nil {
			return ScorePatterns{}, fmt.Errorf("scan top issue: %w", err)
		}
		p.TopIssues = append(p.TopIssues, fi)
	}
	if err := rows.Err(); err != nil {
		return ScorePatterns{}, fmt.Errorf("iterate top issues: %w", err)
	}
	return p, nil
}

// LowestDimension returns the name and average of the weakest score dimension.
// Pure helper over already-aggregated data (no DB) so the strong-signal gate is
// testable. On N == 0 it returns ("", 0).
func (p ScorePatterns) LowestDimension() (string, float64) {
	if p.N == 0 {
		return "", 0
	}
	dims := []struct {
		name string
		val  float64
	}{
		{"hook", p.AvgHook},
		{"clarity", p.AvgClarity},
		{"brand_fit", p.AvgBrandFit},
		{"overall", p.AvgOverall},
	}
	lowName, lowVal := dims[0].name, dims[0].val
	for _, d := range dims[1:] {
		if d.val < lowVal {
			lowName, lowVal = d.name, d.val
		}
	}
	return lowName, lowVal
}
