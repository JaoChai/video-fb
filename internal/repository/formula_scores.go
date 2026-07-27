package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/scoreboard"
)

// dimensionColumn แม็ปชื่อมิติไปยังคอลัมน์ใน clips เป็น allowlist ปิด — ชื่อคอลัมน์
// ถูกต่อเข้า SQL โดยตรงจึงห้ามรับค่าจากภายนอกเด็ดขาด
var dimensionColumn = map[string]string{
	"content_format": "content_format",
	"category":       "category",
	"style_preset":   "style_preset",
}

// weightTable แม็ปมิติไปยังตารางที่เก็บ weight มิติที่ไม่มีในแม็ปนี้ = วัดผลได้
// แต่หมุนน้ำหนักไม่ได้ (style_preset อยู่ในกรณีนี้ ดู spec หัวข้อ 7)
var weightTable = map[string]struct{ table, keyColumn string }{
	"content_format": {"content_formats", "format_name"},
	"category":       {"topic_categories", "category_name"},
}

type FormulaScoresRepo struct {
	pool *pgxpool.Pool
}

func NewFormulaScoresRepo(pool *pgxpool.Pool) *FormulaScoresRepo {
	return &FormulaScoresRepo{pool: pool}
}

// RawStats คำนวณสถิติดิบต่อ (มิติ, ค่า, แพลตฟอร์ม) จาก analytics ล่าสุดต่อคลิป
// ใช้ median ทุกที่ที่ทำได้เพราะคลิปไวรัลตัวเดียวลากค่าเฉลี่ยทั้งกลุ่มได้
// (ยืนยันจากข้อมูลจริง: preset ตัวหนึ่ง avg 0.130 แต่ median 0.041)
func (r *FormulaScoresRepo) RawStats(ctx context.Context, windowDays int) ([]scoreboard.Stat, error) {
	out := []scoreboard.Stat{}
	for dim, col := range dimensionColumn {
		q := fmt.Sprintf(`
WITH latest AS (
    SELECT DISTINCT ON (ca.clip_id, ca.platform)
        ca.clip_id, ca.platform, ca.views, ca.avg_view_percentage
    FROM clip_analytics ca
    WHERE ca.fetched_at >= NOW() - make_interval(days => $1)
      AND ca.platform IN ('youtube','tiktok')
      AND NOT EXISTS (
          SELECT 1 FROM clip_publish_status ps
          WHERE ps.clip_id = ca.clip_id AND ps.platform = ca.platform
            AND ps.status = 'failed')
    ORDER BY ca.clip_id, ca.platform, ca.fetched_at DESC
), ranked AS (
    SELECT l.platform, l.avg_view_percentage, c.%s AS value,
           PERCENT_RANK() OVER (PARTITION BY l.platform ORDER BY l.views) AS pct
    FROM latest l
    JOIN clips c ON c.id = l.clip_id
    WHERE c.status = 'published' AND COALESCE(c.%s, '') <> ''
)
SELECT value, platform, COUNT(*) AS n,
       PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY pct) AS median_pct,
       COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (
           ORDER BY NULLIF(avg_view_percentage, 0)), 0) AS median_retention,
       COUNT(*) FILTER (WHERE pct < 0.25)::float / COUNT(*) AS flop_rate
FROM ranked
GROUP BY value, platform`, col, col)

		rows, err := r.pool.Query(ctx, q, windowDays)
		if err != nil {
			return nil, fmt.Errorf("raw stats %s: %w", dim, err)
		}
		for rows.Next() {
			s := scoreboard.Stat{Dimension: dim}
			if err := rows.Scan(&s.Value, &s.Platform, &s.N, &s.MedianPct, &s.MedianRetention, &s.FlopRate); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan raw stat %s: %w", dim, err)
			}
			out = append(out, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate raw stats %s: %w", dim, err)
		}
	}
	return out, nil
}

// SaveSnapshot เขียนกระดานคะแนนทั้งชุดในทรานแซกชันเดียว — snapshot ครึ่งใบ
// อ่านผิดกว่าไม่มี snapshot เลย
func (r *FormulaScoresRepo) SaveSnapshot(ctx context.Context, computedAt time.Time, scores []scoreboard.Score) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin snapshot: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, s := range scores {
		if _, err := tx.Exec(ctx,
			`INSERT INTO formula_scores
			 (computed_at, dimension, value, platform, n, median_pct,
			  median_retention, flop_rate, score_raw, score_final)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			computedAt, s.Dimension, s.Value, s.Platform, s.N, s.MedianPct,
			s.MedianRetention, s.FlopRate, s.ScoreRaw, s.ScoreFinal); err != nil {
			return fmt.Errorf("insert score %s/%s: %w", s.Dimension, s.Value, err)
		}
	}
	return tx.Commit(ctx)
}

// Latest คืน snapshot ล่าสุดทั้งใบ ถ้ายังไม่เคยมี snapshot จะคืน slice ว่างและ error nil
func (r *FormulaScoresRepo) Latest(ctx context.Context) (time.Time, []scoreboard.Score, error) {
	// MAX(...) เป็น aggregate จึงคืนมาเสมอ 1 แถว (ไม่มีทาง ErrNoRows) แต่ตาราง
	// ว่างจะได้ SQL NULL กลับมา ต้อง scan เข้า *time.Time ถึงจะรับ NULL ได้
	// (รูปแบบเดียวกับ AnalyticsRepo.LastFetchedAt)
	var computedAt *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT MAX(computed_at) FROM formula_scores`).Scan(&computedAt)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("latest snapshot time: %w", err)
	}
	if computedAt == nil {
		return time.Time{}, []scoreboard.Score{}, nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT dimension, value, platform, n, median_pct, median_retention,
		        flop_rate, score_raw, score_final
		 FROM formula_scores WHERE computed_at = $1
		 ORDER BY dimension, platform, value`, *computedAt)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("load snapshot: %w", err)
	}
	defer rows.Close()

	out := []scoreboard.Score{}
	for rows.Next() {
		var s scoreboard.Score
		if err := rows.Scan(&s.Dimension, &s.Value, &s.Platform, &s.N, &s.MedianPct,
			&s.MedianRetention, &s.FlopRate, &s.ScoreRaw, &s.ScoreFinal); err != nil {
			return time.Time{}, nil, fmt.Errorf("scan snapshot row: %w", err)
		}
		out = append(out, s)
	}
	return *computedAt, out, rows.Err()
}

// CurrentWeights คืน weight ปัจจุบันของสูตรที่ enabled ในมิตินั้น
// มิติที่หมุนน้ำหนักไม่ได้ (เช่น style_preset) คืน map ว่างและ error nil
func (r *FormulaScoresRepo) CurrentWeights(ctx context.Context, dimension string) (map[string]int, error) {
	t, ok := weightTable[dimension]
	if !ok {
		return map[string]int{}, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(
		`SELECT %s, weight FROM %s WHERE enabled = TRUE`, t.keyColumn, t.table))
	if err != nil {
		return nil, fmt.Errorf("current weights %s: %w", dimension, err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var k string
		var w int
		if err := rows.Scan(&k, &w); err != nil {
			return nil, fmt.Errorf("scan weight %s: %w", dimension, err)
		}
		out[k] = w
	}
	return out, rows.Err()
}

// ApplyWeights เขียน weight ใหม่ทั้งมิติในทรานแซกชันเดียว ห้ามแตะคอลัมน์ enabled
// ต้องเช็ค RowsAffected ทุกแถว — ถ้าสูตรถูกปิด (enabled=FALSE) ระหว่าง snapshot
// กับตอน apply, WHERE ... AND enabled = TRUE จะกรองแถวนั้นทิ้งเงียบๆ ทำให้ audit
// (RecordRevisions ที่ commit ไปแล้ว) กับ weight จริงเพี้ยนกันโดยไม่มี error ใดๆ
// เตือน จึงต้อง error ทันทีเพื่อให้ผู้เรียกรู้ว่าต้อง reconcile
func (r *FormulaScoresRepo) ApplyWeights(ctx context.Context, dimension string, weights map[string]int) error {
	t, ok := weightTable[dimension]
	if !ok {
		return fmt.Errorf("dimension %q หมุนน้ำหนักไม่ได้", dimension)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin apply weights: %w", err)
	}
	defer tx.Rollback(ctx)

	for k, w := range weights {
		tag, err := tx.Exec(ctx, fmt.Sprintf(
			`UPDATE %s SET weight = $1 WHERE %s = $2 AND enabled = TRUE`,
			t.table, t.keyColumn), w, k)
		if err != nil {
			return fmt.Errorf("update weight %s/%s: %w", dimension, k, err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("apply weight %s/%s: 0 rows updated (สูตรอาจถูกปิด enabled=FALSE ไปแล้ว)", dimension, k)
		}
	}
	return tx.Commit(ctx)
}

// RecordRevisions เขียน audit ทั้งชุด ต้องเรียกให้สำเร็จ *ก่อน* ApplyWeights เสมอ
func (r *FormulaScoresRepo) RecordRevisions(ctx context.Context, revs []models.WeightRevision) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin revisions: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, v := range revs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO weight_revisions
			 (dimension, value, old_weight, new_weight, score_final, n, computed_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			v.Dimension, v.Value, v.OldWeight, v.NewWeight, v.ScoreFinal, v.N, v.ComputedAt); err != nil {
			return fmt.Errorf("insert revision %s/%s: %w", v.Dimension, v.Value, err)
		}
	}
	return tx.Commit(ctx)
}

// ListRevisions คืนประวัติล่าสุด newest-first
func (r *FormulaScoresRepo) ListRevisions(ctx context.Context, limit int) ([]models.WeightRevision, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT dimension, value, old_weight, new_weight, score_final, n, computed_at, created_at
		 FROM weight_revisions ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list weight revisions: %w", err)
	}
	defer rows.Close()

	out := []models.WeightRevision{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		var v models.WeightRevision
		if err := rows.Scan(&v.Dimension, &v.Value, &v.OldWeight, &v.NewWeight,
			&v.ScoreFinal, &v.N, &v.ComputedAt, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan weight revision: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// LastRevisionBatch คืน audit ของการหมุนรอบล่าสุด (batch เดียวกัน = computed_at เดียวกัน)
// ใช้สำหรับ rollback: เขียน old_weight กลับ
func (r *FormulaScoresRepo) LastRevisionBatch(ctx context.Context) ([]models.WeightRevision, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT dimension, value, old_weight, new_weight, score_final, n, computed_at, created_at
		 FROM weight_revisions
		 WHERE computed_at = (SELECT MAX(computed_at) FROM weight_revisions)
		 ORDER BY dimension, value`)
	if err != nil {
		return nil, fmt.Errorf("last revision batch: %w", err)
	}
	defer rows.Close()

	out := []models.WeightRevision{}
	for rows.Next() {
		var v models.WeightRevision
		if err := rows.Scan(&v.Dimension, &v.Value, &v.OldWeight, &v.NewWeight,
			&v.ScoreFinal, &v.N, &v.ComputedAt, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan revision batch row: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
