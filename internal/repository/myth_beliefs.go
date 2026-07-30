package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type MythBeliefsRepo struct {
	pool *pgxpool.Pool
}

func NewMythBeliefsRepo(pool *pgxpool.Pool) *MythBeliefsRepo {
	return &MythBeliefsRepo{pool: pool}
}

const mythBeliefCols = `id, belief_key, belief_th, why_believed_th, verdict, fact_th,
	source_label, source_url, nuance_th, cost_th, audience, weight, enabled`

// MythParkDays คือระยะพักหลังใช้หัวข้อไปแล้ว — ระยะเดียวกับคลัง tutorial เพราะ
// เหตุผลเดียวกัน: คนดูจำได้ว่าเพิ่งเห็นเรื่องนี้ และการพักต้องหมดอายุเสมอ
const MythParkDays = 14

// MythMinPool คือจำนวนแถวที่ต้องเหลือใช้ได้เป็นอย่างน้อย ถ้า park แถวถัดไปจะทำให้
// เหลือน้อยกว่านี้ ให้ข้ามการ park (ยังบวก used_count) — คลิปซ้ำหัวข้อยังดีกว่า
// ไม่มีคลิปเลย และคลังตอนเริ่มมีแถวที่ยืนยันแล้วเพียง 6 แถว
const MythMinPool = 3

// mythAvailableWhere นิยาม "แถวที่หยิบมาทำคลิปได้ตอนนี้" ที่เดียว เพื่อให้ตัวเลือก
// กับตัวนับพื้นมองคลังชุดเดียวกันเสมอ
const mythAvailableWhere = `enabled = TRUE AND needs_verify = FALSE
	AND source_label <> ''
	AND (parked_until IS NULL OR parked_until <= NOW())`

const mythPickSQL = `SELECT ` + mythBeliefCols + `, used_count
	FROM myth_beliefs
	WHERE ` + mythAvailableWhere + `
	ORDER BY belief_key`

// mythPickFallbackSQL ใช้เมื่อทุกแถวติด park — หยิบแถวที่พักนานที่สุด แทนการคืน nil
// ประตูทางเดียวเคยทำให้รอบผลิตคืน 0 คลิปเงียบๆ มาแล้วสองครั้งในระบบนี้
const mythPickFallbackSQL = `SELECT ` + mythBeliefCols + `, used_count
	FROM myth_beliefs
	WHERE enabled = TRUE AND needs_verify = FALSE AND source_label <> ''
	ORDER BY parked_until NULLS FIRST, used_count
	LIMIT 1`

// mythMarkUsedSQL บวกตัวนับแล้ว park แถวนี้ 14 วัน — แต่ park เฉพาะเมื่อคลังยังเหลือ
// แถวใช้ได้มากกว่าพื้น การนับอยู่ใน statement เดียวกันโดยตั้งใจ: แยกนับแล้วค่อย
// UPDATE จะทำให้สองรอบที่ชนกันเห็น "เหลือแถวสุดท้าย" พร้อมกันแล้ว park ทิ้งทั้งคู่
const mythMarkUsedSQL = `
	UPDATE myth_beliefs
	SET used_count = used_count + 1,
	    last_used_at = NOW(),
	    parked_until = CASE
	        WHEN (SELECT COUNT(*) FROM myth_beliefs m2
	              WHERE m2.enabled = TRUE AND m2.needs_verify = FALSE AND m2.source_label <> ''
	                AND (m2.parked_until IS NULL OR m2.parked_until <= NOW())
	                AND m2.id <> myth_beliefs.id) >= $2
	        THEN NOW() + make_interval(days => $3)
	        ELSE parked_until
	    END
	WHERE id = $1`

type mythUsage struct {
	Belief    models.MythBelief
	UsedCount int
}

// pickMythLeastUsed เลือกแถวที่ใช้น้อยสุด ตัดสินเสมอด้วย weight สูงกว่า แล้วจึงด้วย
// belief_key เพื่อให้ผลลัพธ์เสถียร (เทสต์ได้โดยไม่ต้องมีฐานข้อมูล)
func pickMythLeastUsed(usages []mythUsage) models.MythBelief {
	best := usages[0]
	for _, u := range usages[1:] {
		switch {
		case u.UsedCount < best.UsedCount:
			best = u
		case u.UsedCount == best.UsedCount && u.Belief.Weight > best.Belief.Weight:
			best = u
		}
	}
	return best.Belief
}

func scanMythUsage(scan func(dest ...any) error) (mythUsage, error) {
	var u mythUsage
	b := &u.Belief
	err := scan(&b.ID, &b.BeliefKey, &b.BeliefTH, &b.WhyBelievedTH, &b.Verdict, &b.FactTH,
		&b.SourceLabel, &b.SourceURL, &b.NuanceTH, &b.CostTH, &b.Audience, &b.Weight, &b.Enabled,
		&u.UsedCount)
	return u, err
}

// PickNext คืนแถวที่ควรทำคลิปรอบนี้ · nil = คลังไม่มีแถวที่ยืนยันแล้วเลย
// แถวที่ verdict เพี้ยน (ไม่อยู่ใน 3 ค่าที่รู้จัก) ถูกข้าม เพราะมันจะเรนเดอร์เป็น
// มิเตอร์เปล่าในคลิปที่เผยแพร่แล้ว
func (r *MythBeliefsRepo) PickNext(ctx context.Context) (*models.MythBelief, error) {
	rows, err := r.pool.Query(ctx, mythPickSQL)
	if err != nil {
		return nil, fmt.Errorf("query myth beliefs: %w", err)
	}
	defer rows.Close()

	usages := []mythUsage{}
	for rows.Next() {
		u, sErr := scanMythUsage(rows.Scan)
		if sErr != nil {
			return nil, fmt.Errorf("scan myth belief: %w", sErr)
		}
		if !models.ValidMythVerdict(u.Belief.Verdict) {
			continue
		}
		usages = append(usages, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(usages) > 0 {
		picked := pickMythLeastUsed(usages)
		return &picked, nil
	}

	// ทุกแถวติด park — หยิบแถวที่พักนานสุดแทนการคืน nil
	u, err := scanMythUsage(r.pool.QueryRow(ctx, mythPickFallbackSQL).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // คลังว่างจริง ให้ผู้เรียกรายงานเป็น error ที่อ่านออก
	}
	if err != nil {
		// DB ล่ม/คิวรีพัง ต้องแยกจาก "คลังว่าง" ให้ได้ ไม่งั้นข้อความที่ผู้เรียกรายงาน
		// จะบอกให้ไปเติมคลังทั้งที่ปัญหาอยู่ที่ฐานข้อมูล
		return nil, fmt.Errorf("pick myth belief (fallback): %w", err)
	}
	if !models.ValidMythVerdict(u.Belief.Verdict) {
		return nil, nil
	}
	return &u.Belief, nil
}

// GetByKey ใช้โดยเส้นทาง retry ที่มีแต่ clips.myth_belief ให้ไปต่อ
func (r *MythBeliefsRepo) GetByKey(ctx context.Context, key string) (*models.MythBelief, error) {
	var b models.MythBelief
	err := r.pool.QueryRow(ctx,
		`SELECT `+mythBeliefCols+` FROM myth_beliefs WHERE belief_key = $1`, key).
		Scan(&b.ID, &b.BeliefKey, &b.BeliefTH, &b.WhyBelievedTH, &b.Verdict, &b.FactTH,
			&b.SourceLabel, &b.SourceURL, &b.NuanceTH, &b.CostTH, &b.Audience, &b.Weight, &b.Enabled)
	if err != nil {
		return nil, fmt.Errorf("get myth belief %s: %w", key, err)
	}
	return &b, nil
}

// MarkUsed บวกตัวนับและพักแถวนี้ เรียกหลังคลิปผลิตสำเร็จเท่านั้น
func (r *MythBeliefsRepo) MarkUsed(ctx context.Context, id string) error {
	if _, err := r.pool.Exec(ctx, mythMarkUsedSQL, id, MythMinPool, MythParkDays); err != nil {
		return fmt.Errorf("mark myth belief used: %w", err)
	}
	return nil
}
