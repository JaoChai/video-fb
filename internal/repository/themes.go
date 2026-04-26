package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type ThemesRepo struct {
	pool *pgxpool.Pool
}

func NewThemesRepo(pool *pgxpool.Pool) *ThemesRepo {
	return &ThemesRepo{pool: pool}
}

func (r *ThemesRepo) GetActive(ctx context.Context) (*models.BrandTheme, error) {
	var t models.BrandTheme
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, primary_color, secondary_color, accent_color, font_name,
		        logo_url, mascot_description, image_style, active
		 FROM brand_themes WHERE active = TRUE ORDER BY name LIMIT 1`,
	).Scan(&t.ID, &t.Name, &t.PrimaryColor, &t.SecondaryColor, &t.AccentColor,
		&t.FontName, &t.LogoURL, &t.MascotDescription, &t.ImageStyle, &t.Active)
	if err != nil {
		return nil, fmt.Errorf("get active theme: %w", err)
	}
	return &t, nil
}

func (r *ThemesRepo) List(ctx context.Context) ([]models.BrandTheme, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, primary_color, secondary_color, accent_color, font_name,
		        logo_url, mascot_description, image_style, active
		 FROM brand_themes ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query themes: %w", err)
	}
	defer rows.Close()

	var themes []models.BrandTheme
	for rows.Next() {
		var t models.BrandTheme
		if err := rows.Scan(&t.ID, &t.Name, &t.PrimaryColor, &t.SecondaryColor, &t.AccentColor,
			&t.FontName, &t.LogoURL, &t.MascotDescription, &t.ImageStyle, &t.Active); err != nil {
			return nil, fmt.Errorf("scan theme: %w", err)
		}
		themes = append(themes, t)
	}
	return themes, nil
}

func (r *ThemesRepo) Update(ctx context.Context, id string, t models.BrandTheme) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE brand_themes SET name=$2, primary_color=$3, secondary_color=$4,
		 accent_color=$5, font_name=$6, logo_url=$7, mascot_description=$8, image_style=$9 WHERE id=$1`,
		id, t.Name, t.PrimaryColor, t.SecondaryColor, t.AccentColor,
		t.FontName, t.LogoURL, t.MascotDescription, t.ImageStyle)
	if err != nil {
		return fmt.Errorf("update theme %s: %w", id, err)
	}
	return nil
}
