package models

import "time"

type Schedule struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	CronExpression string     `json:"cron_expression"`
	Action         string     `json:"action"`
	Enabled        bool       `json:"enabled"`
	LastRunAt      *time.Time `json:"last_run_at"`
	NextRunAt      *time.Time `json:"next_run_at"`
}
