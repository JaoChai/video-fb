package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type AgentsRepo struct {
	pool *pgxpool.Pool
}

func NewAgentsRepo(pool *pgxpool.Pool) *AgentsRepo {
	return &AgentsRepo{pool: pool}
}

func (r *AgentsRepo) List(ctx context.Context) ([]models.AgentConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, agent_name, system_prompt, model, temperature, enabled, config
		 FROM agent_configs ORDER BY agent_name`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []models.AgentConfig
	for rows.Next() {
		var a models.AgentConfig
		if err := rows.Scan(&a.ID, &a.AgentName, &a.SystemPrompt, &a.Model,
			&a.Temperature, &a.Enabled, &a.Config); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func (r *AgentsRepo) GetByName(ctx context.Context, name string) (*models.AgentConfig, error) {
	var a models.AgentConfig
	err := r.pool.QueryRow(ctx,
		`SELECT id, agent_name, system_prompt, model, temperature, enabled, config
		 FROM agent_configs WHERE agent_name = $1`, name,
	).Scan(&a.ID, &a.AgentName, &a.SystemPrompt, &a.Model, &a.Temperature, &a.Enabled, &a.Config)
	if err != nil {
		return nil, fmt.Errorf("get agent %s: %w", name, err)
	}
	return &a, nil
}

func (r *AgentsRepo) Update(ctx context.Context, id string, prompt, model string, temp float64, enabled bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_configs SET system_prompt=$2, model=$3, temperature=$4, enabled=$5 WHERE id=$1`,
		id, prompt, model, temp, enabled)
	if err != nil {
		return fmt.Errorf("update agent %s: %w", id, err)
	}
	return nil
}
