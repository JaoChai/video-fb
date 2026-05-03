package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type PromptHistoryEntry struct {
	ID        string    `json:"id"`
	AgentName string    `json:"agent_name"`
	OldPrompt string    `json:"old_prompt"`
	NewPrompt string    `json:"new_prompt"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type AgentsRepo struct {
	pool *pgxpool.Pool
}

func NewAgentsRepo(pool *pgxpool.Pool) *AgentsRepo {
	return &AgentsRepo{pool: pool}
}

func (r *AgentsRepo) List(ctx context.Context) ([]models.AgentConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, agent_name, system_prompt, prompt_template, model, temperature, enabled, skills, config
		 FROM agent_configs ORDER BY agent_name`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []models.AgentConfig
	for rows.Next() {
		var a models.AgentConfig
		if err := rows.Scan(&a.ID, &a.AgentName, &a.SystemPrompt, &a.PromptTemplate, &a.Model,
			&a.Temperature, &a.Enabled, &a.Skills, &a.Config); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func (r *AgentsRepo) GetByName(ctx context.Context, name string) (*models.AgentConfig, error) {
	var a models.AgentConfig
	err := r.pool.QueryRow(ctx,
		`SELECT id, agent_name, system_prompt, prompt_template, model, temperature, enabled, skills, config
		 FROM agent_configs WHERE agent_name = $1`, name,
	).Scan(&a.ID, &a.AgentName, &a.SystemPrompt, &a.PromptTemplate, &a.Model, &a.Temperature, &a.Enabled, &a.Skills, &a.Config)
	if err != nil {
		return nil, fmt.Errorf("get agent %s: %w", name, err)
	}
	return &a, nil
}

func (r *AgentsRepo) Update(ctx context.Context, id string, prompt, promptTemplate, model string, temp float64, enabled bool, skills string) error {
	if promptTemplate == "" {
		_, err := r.pool.Exec(ctx,
			`UPDATE agent_configs SET system_prompt=$2, model=$3, temperature=$4, enabled=$5, skills=$6 WHERE id=$1`,
			id, prompt, model, temp, enabled, skills)
		if err != nil {
			return fmt.Errorf("update agent %s: %w", id, err)
		}
		return nil
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_configs SET system_prompt=$2, prompt_template=$3, model=$4, temperature=$5, enabled=$6, skills=$7 WHERE id=$1`,
		id, prompt, promptTemplate, model, temp, enabled, skills)
	if err != nil {
		return fmt.Errorf("update agent %s: %w", id, err)
	}
	return nil
}

func (r *AgentsRepo) UpdatePromptByName(ctx context.Context, agentName, newPrompt string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_configs SET system_prompt = $2 WHERE agent_name = $1`,
		agentName, newPrompt)
	if err != nil {
		return fmt.Errorf("update prompt for agent %s: %w", agentName, err)
	}
	return nil
}

func (r *AgentsRepo) UpdateSkillsByName(ctx context.Context, agentName, newSkills string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_configs SET skills = $2 WHERE agent_name = $1`,
		agentName, newSkills)
	if err != nil {
		return fmt.Errorf("update skills for agent %s: %w", agentName, err)
	}
	return nil
}

func (r *AgentsRepo) SavePromptHistory(ctx context.Context, agentName, oldPrompt, newPrompt, reason string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO agent_prompt_history (agent_name, old_prompt, new_prompt, reason)
		 VALUES ($1, $2, $3, $4)`,
		agentName, oldPrompt, newPrompt, reason)
	if err != nil {
		return fmt.Errorf("save prompt history for %s: %w", agentName, err)
	}
	return nil
}

func (r *AgentsRepo) ListPromptHistory(ctx context.Context, limit int) ([]PromptHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, agent_name, old_prompt, new_prompt, reason, created_at
		 FROM agent_prompt_history
		 ORDER BY created_at DESC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query prompt history: %w", err)
	}
	defer rows.Close()

	var entries []PromptHistoryEntry
	for rows.Next() {
		var e PromptHistoryEntry
		if err := rows.Scan(&e.ID, &e.AgentName, &e.OldPrompt, &e.NewPrompt, &e.Reason, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan prompt history: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}
