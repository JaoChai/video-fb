package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type SkillRevisionsRepo struct {
	pool *pgxpool.Pool
}

func NewSkillRevisionsRepo(pool *pgxpool.Pool) *SkillRevisionsRepo {
	return &SkillRevisionsRepo{pool: pool}
}

// List returns the most recent skill revisions, newest first, capped at limit rows.
func (r *SkillRevisionsRepo) List(ctx context.Context, limit int) ([]models.SkillRevision, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT agent_name, rationale, critique_window, created_at
		 FROM skill_revisions ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list skill revisions: %w", err)
	}
	defer rows.Close()
	out := []models.SkillRevision{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		var s models.SkillRevision
		if err := rows.Scan(&s.AgentName, &s.Rationale, &s.CritiqueWindow, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan skill revision: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Record appends one audit row capturing the full old + new skills, the
// rationale, and the critique window. Append-only: never updates or deletes, so
// the table is a complete, revertable history of every auto-applied change.
func (r *SkillRevisionsRepo) Record(ctx context.Context, agentName, oldSkills, newSkills, rationale string, critiqueWindow int) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO skill_revisions (agent_name, old_skills, new_skills, rationale, critique_window)
		 VALUES ($1, $2, $3, $4, $5)`,
		agentName, oldSkills, newSkills, rationale, critiqueWindow)
	return err
}
