package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type SchedulesRepo struct {
	pool *pgxpool.Pool
}

func NewSchedulesRepo(pool *pgxpool.Pool) *SchedulesRepo {
	return &SchedulesRepo{pool: pool}
}

func (r *SchedulesRepo) List(ctx context.Context) ([]models.Schedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, cron_expression, action, enabled, last_run_at, next_run_at
		 FROM schedules ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query schedules: %w", err)
	}
	defer rows.Close()

	var schedules []models.Schedule
	for rows.Next() {
		var s models.Schedule
		if err := rows.Scan(&s.ID, &s.Name, &s.CronExpression, &s.Action,
			&s.Enabled, &s.LastRunAt, &s.NextRunAt); err != nil {
			return nil, fmt.Errorf("scan schedule: %w", err)
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}

func (r *SchedulesRepo) Update(ctx context.Context, id, cron, action string, enabled bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE schedules SET cron_expression=$2, action=$3, enabled=$4 WHERE id=$1`,
		id, cron, action, enabled)
	if err != nil {
		return fmt.Errorf("update schedule %s: %w", id, err)
	}
	return nil
}
