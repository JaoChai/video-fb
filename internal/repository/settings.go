package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsRepo struct {
	pool *pgxpool.Pool
}

func NewSettingsRepo(pool *pgxpool.Pool) *SettingsRepo {
	return &SettingsRepo{pool: pool}
}

func (r *SettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("query settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		settings[k] = v
	}
	return settings, nil
}

func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	var v string
	err := r.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&v)
	if err != nil {
		return "", fmt.Errorf("get setting %s: %w", key, err)
	}
	return v, nil
}

func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
		key, value)
	if err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}

func (r *SettingsRepo) GetCategories(ctx context.Context) ([]string, error) {
	raw, err := r.Get(ctx, "categories")
	if err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}
	var categories []string
	if err := json.Unmarshal([]byte(raw), &categories); err != nil {
		return nil, fmt.Errorf("parse categories: %w", err)
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("categories setting is empty")
	}
	return categories, nil
}

func (r *SettingsRepo) GetBrandAliases(ctx context.Context) (map[string]string, error) {
	raw, err := r.Get(ctx, "brand_aliases")
	if err != nil {
		return map[string]string{}, nil
	}
	var aliases map[string]string
	if err := json.Unmarshal([]byte(raw), &aliases); err != nil {
		return map[string]string{}, nil
	}
	return aliases, nil
}
