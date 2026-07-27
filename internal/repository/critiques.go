package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

// sceneIndexPattern จับ index ของซีนในชื่อ field (`scene[5].image_prompt`)
// เพื่อยุบให้เป็นปัญหาเดียวกัน — ไม่งั้นปัญหาเดียวกันที่เกิดคนละซีนจะถูกนับแยก
// จนไม่มีวันถึงเกณฑ์ความถี่
var sceneIndexPattern = regexp.MustCompile(`\[\d+\]`)

// NormalizeField ตัด index ของซีนออกจากชื่อ field
func NormalizeField(field string) string {
	return sceneIndexPattern.ReplaceAllString(field, "")
}

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

// FieldIssue is one recurring critic edit (a changes[] entry, with the scene
// index stripped from the field name). Count is how many DISTINCT critique rows
// touched this field in the window — NOT how many times the edit appears, since
// one critique can edit the same field across multiple scenes (e.g. scene[0],
// scene[1], scene[2].voice_text) and that should count once, not thrice. The
// frequency gate in the learner compares Count against the number of critique
// rows (N), so it must be a critique count.
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

// LowScorePatterns aggregates clip_critiques over the window
// [now-sinceDays, now-untilDays): untilDays = 0 คือถึงปัจจุบัน ใช้ baseline
// ที่ไม่ทับ window ได้ด้วยการเรียกด้วย (90, 30)
// topN caps how many recurring issues are returned (0 or negative -> 10).
func (r *CritiquesRepo) LowScorePatterns(ctx context.Context, sinceDays, untilDays, topN int) (ScorePatterns, error) {
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
  AND created_at <  NOW() - make_interval(days => $2)
  AND applied = TRUE`,
		sinceDays, untilDays,
	).Scan(&p.N, &p.AvgHook, &p.AvgClarity, &p.AvgBrandFit, &p.AvgOverall)
	if err != nil {
		return ScorePatterns{}, fmt.Errorf("aggregate score patterns: %w", err)
	}

	if p.N == 0 {
		return p, nil
	}

	// จัดกลุ่มด้วย field ที่ตัด index ซีนออกแล้วเท่านั้น — reason เป็นข้อความอิสระ
	// ที่โมเดลเขียนใหม่ทุกครั้ง จึงเก็บไว้แค่เป็นตัวอย่าง ไม่ใช้จัดกลุ่ม
	// cnt นับจำนวน critique (DISTINCT cc.id) ที่แตะ field นี้ ไม่ใช่จำนวนครั้งที่แก้ —
	// critique ใบเดียวแก้ scene[0].voice_text, scene[1].voice_text, scene[2].voice_text
	// ถือเป็น field เดียวใน 1 ใบ ไม่ใช่ 3 ครั้ง เพราะเกณฑ์ frequency ใน learner
	// เทียบ Count กับจำนวนใบ critique (N) ถ้านับเป็นครั้ง ตัวเลขจะเกิน N และประตู
	// จะเปิดเกือบทุกครั้งโดยไม่มีความหมาย
	rows, err := r.pool.Query(ctx, `
SELECT
  regexp_replace(c->>'field', '\[[0-9]+\]', '', 'g') AS field,
  (array_agg(c->>'reason' ORDER BY cc.created_at DESC))[1] AS reason,
  COUNT(DISTINCT cc.id) AS cnt
FROM clip_critiques cc,
     LATERAL jsonb_array_elements(cc.changes) AS c
WHERE cc.created_at >= NOW() - make_interval(days => $1)
  AND cc.created_at <  NOW() - make_interval(days => $2)
  AND cc.applied = TRUE
  AND c->>'field' IS NOT NULL
GROUP BY 1
ORDER BY cnt DESC, field ASC
LIMIT $3`,
		sinceDays, untilDays, topN,
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

// scoreDim pairs a dimension name with its aggregated average. dims() is the
// single source of truth for the dimension set — LowestDimension and Dim both
// read from it, so adding a dimension is a one-place change.
type scoreDim struct {
	name string
	val  float64
}

func (p ScorePatterns) dims() []scoreDim {
	return []scoreDim{
		{"hook", p.AvgHook},
		{"clarity", p.AvgClarity},
		{"brand_fit", p.AvgBrandFit},
		{"overall", p.AvgOverall},
	}
}

// LowestDimension returns the name and average of the weakest score dimension.
// Pure helper over already-aggregated data (no DB) so the strong-signal gate is
// testable. On N == 0 it returns ("", 0).
func (p ScorePatterns) LowestDimension() (string, float64) {
	if p.N == 0 {
		return "", 0
	}
	dims := p.dims()
	lowName, lowVal := dims[0].name, dims[0].val
	for _, d := range dims[1:] {
		if d.val < lowVal {
			lowName, lowVal = d.name, d.val
		}
	}
	return lowName, lowVal
}

// Dim returns the average for a named score dimension; unknown names fall back
// to the overall average.
func (p ScorePatterns) Dim(name string) float64 {
	for _, d := range p.dims() {
		if d.name == name {
			return d.val
		}
	}
	return p.AvgOverall
}
