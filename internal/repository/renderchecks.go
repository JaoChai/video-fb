package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ด่านตรวจของ Hyperframes ที่เราบันทึกผล — ชื่อต้องตรงกับคอลัมน์ stage
const (
	StageLint    = "lint"
	StageInspect = "inspect"
	StageRender  = "render"
)

// ValidRenderStage กันชื่อด่านที่พิมพ์ผิดไม่ให้ลง DB เงียบๆ — คอลัมน์ stage เป็น
// TEXT อิสระ ถ้าเขียน "Lint" ปนไป สถิติที่ GROUP BY stage จะแตกโดยไม่มีใครรู้
func ValidRenderStage(stage string) bool {
	return stage == StageLint || stage == StageInspect || stage == StageRender
}

type RenderChecksRepo struct {
	pool *pgxpool.Pool
}

func NewRenderChecksRepo(pool *pgxpool.Pool) *RenderChecksRepo {
	return &RenderChecksRepo{pool: pool}
}

// Create appends one render-check row. findings คือ JSON array ที่ encode มาแล้ว
// (ผู้เรียกใช้ producer.CheckResult.FindingsJSON()).
func (r *RenderChecksRepo) Create(ctx context.Context, clipID, stage string, passed bool, durationMS int, findings []byte) error {
	if !ValidRenderStage(stage) {
		return fmt.Errorf("invalid render check stage %q", stage)
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO render_checks (clip_id, stage, passed, duration_ms, findings)
		 VALUES ($1, $2, $3, $4, $5)`,
		clipID, stage, passed, durationMS, findings)
	return err
}
