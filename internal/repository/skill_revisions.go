package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SkillRevisionsRepo struct {
	pool *pgxpool.Pool
}

func NewSkillRevisionsRepo(pool *pgxpool.Pool) *SkillRevisionsRepo {
	return &SkillRevisionsRepo{pool: pool}
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
