package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type FormatsRepo struct {
	pool *pgxpool.Pool
}

func NewFormatsRepo(pool *pgxpool.Pool) *FormatsRepo {
	return &FormatsRepo{pool: pool}
}

// PickNext returns the enabled format that has been used least (relative to its
// weight) in the last 7 days — guarantees every format gets airtime.
func (r *FormatsRepo) PickNext(ctx context.Context) (*models.ContentFormat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cf.id, cf.format_name, cf.display_name, cf.question_instruction,
		       cf.script_instruction, cf.enabled, cf.weight,
		       COALESCE(u.cnt, 0) AS used_count
		FROM content_formats cf
		LEFT JOIN (
			SELECT content_format, COUNT(*) AS cnt
			FROM clips
			WHERE created_at > NOW() - INTERVAL '7 days'
			GROUP BY content_format
		) u ON u.content_format = cf.format_name
		WHERE cf.enabled = TRUE
		ORDER BY cf.format_name`)
	if err != nil {
		return nil, fmt.Errorf("query format usage: %w", err)
	}
	defer rows.Close()

	var usages []models.FormatUsage
	for rows.Next() {
		var u models.FormatUsage
		if err := rows.Scan(&u.Format.ID, &u.Format.FormatName, &u.Format.DisplayName,
			&u.Format.QuestionInstruction, &u.Format.ScriptInstruction,
			&u.Format.Enabled, &u.Format.Weight, &u.UsedCount); err != nil {
			return nil, fmt.Errorf("scan format usage: %w", err)
		}
		usages = append(usages, u)
	}

	picked := pickLeastUsed(usages)
	return &picked, nil
}

// GetByName returns a single content format by its format_name.
func (r *FormatsRepo) GetByName(ctx context.Context, name string) (*models.ContentFormat, error) {
	var f models.ContentFormat
	err := r.pool.QueryRow(ctx,
		`SELECT id, format_name, display_name, question_instruction, script_instruction, enabled, weight
		 FROM content_formats WHERE format_name = $1`, name,
	).Scan(&f.ID, &f.FormatName, &f.DisplayName, &f.QuestionInstruction, &f.ScriptInstruction, &f.Enabled, &f.Weight)
	if err != nil {
		return nil, fmt.Errorf("get format %s: %w", name, err)
	}
	return &f, nil
}

// pickLeastUsed selects the format with the lowest used/weight ratio.
// Pure function — testable without DB. Falls back to a plain Q&A format.
func pickLeastUsed(usages []models.FormatUsage) models.ContentFormat {
	if len(usages) == 0 {
		return models.ContentFormat{FormatName: "qa", DisplayName: "Q&A", Weight: 1}
	}
	best := usages[0]
	bestRatio := usageRatio(best)
	for _, u := range usages[1:] {
		if r := usageRatio(u); r < bestRatio {
			best = u
			bestRatio = r
		}
	}
	return best.Format
}

func usageRatio(u models.FormatUsage) float64 {
	w := u.Format.Weight
	if w < 1 {
		w = 1
	}
	return float64(u.UsedCount) / float64(w)
}
