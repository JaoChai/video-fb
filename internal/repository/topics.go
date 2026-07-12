package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type TopicCategoriesRepo struct {
	pool *pgxpool.Pool
}

func NewTopicCategoriesRepo(pool *pgxpool.Pool) *TopicCategoriesRepo {
	return &TopicCategoriesRepo{pool: pool}
}

// GetAll — ทุก category เรียงตามชื่อ
func (r *TopicCategoriesRepo) GetAll(ctx context.Context) ([]models.TopicCategory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, category_name, display_name, angle_instruction, enabled, weight
		FROM topic_categories
		ORDER BY category_name`)
	if err != nil {
		return nil, fmt.Errorf("query topic_categories: %w", err)
	}
	defer rows.Close()
	out := []models.TopicCategory{}
	for rows.Next() {
		var c models.TopicCategory
		if err := rows.Scan(&c.ID, &c.CategoryName, &c.DisplayName, &c.AngleInstruction, &c.Enabled, &c.Weight); err != nil {
			return nil, fmt.Errorf("scan topic_category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PickNextExclude — least-used/7d + weight (ท่า formats.PickNext) และ exclude category ที่ใช้ใน 24h ล่าสุด
// (กันซ้ำในวันเดียวกัน). excludeToday empty = ไม่ exclude.
func (r *TopicCategoriesRepo) PickNextExclude(ctx context.Context, excludeToday []string) (*models.TopicCategory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tc.id, tc.category_name, tc.display_name, tc.angle_instruction, tc.enabled, tc.weight,
		       COALESCE(u.cnt, 0) AS used_count
		FROM topic_categories tc
		LEFT JOIN (
			SELECT category, COUNT(*) AS cnt
			FROM clips
			WHERE created_at > NOW() - INTERVAL '7 days'
			GROUP BY category
		) u ON u.category = tc.category_name
		WHERE tc.enabled = TRUE
		ORDER BY tc.category_name`)
	if err != nil {
		return nil, fmt.Errorf("query topic category usage: %w", err)
	}
	defer rows.Close()

	usages := []topicUsage{}
	for rows.Next() {
		var u topicUsage
		if err := rows.Scan(&u.Cat.ID, &u.Cat.CategoryName, &u.Cat.DisplayName, &u.Cat.AngleInstruction, &u.Cat.Enabled, &u.Cat.Weight, &u.UsedCount); err != nil {
			return nil, fmt.Errorf("scan topic category usage: %w", err)
		}
		usages = append(usages, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(usages) == 0 {
		return nil, nil
	}

	picked := pickTopicCategoryLeastUsed(usages, excludeToday)
	return &picked, nil
}

// topicUsage pairs a topic category with how many clips used it in the last 7 days.
type topicUsage struct {
	Cat       models.TopicCategory
	UsedCount int
}

// pickTopicCategoryLeastUsed selects the category with the lowest used/weight ratio,
// excluding any category name in exclude. If every category is excluded, the exclude
// rule is dropped for this pick (fallback to the full set) so a pick is always returned
// when usages is non-empty. Pure function — testable without DB.
func pickTopicCategoryLeastUsed(usages []topicUsage, exclude []string) models.TopicCategory {
	if len(usages) == 0 {
		return models.TopicCategory{}
	}

	excludeSet := map[string]bool{}
	for _, e := range exclude {
		excludeSet[e] = true
	}

	pool := make([]topicUsage, 0, len(usages))
	for _, u := range usages {
		if !excludeSet[u.Cat.CategoryName] {
			pool = append(pool, u)
		}
	}
	if len(pool) == 0 {
		pool = usages
	}

	best := pool[0]
	bestRatio := catUsageRatio(best.UsedCount, best.Cat.Weight)
	for _, u := range pool[1:] {
		if r := catUsageRatio(u.UsedCount, u.Cat.Weight); r < bestRatio {
			best, bestRatio = u, r
		}
	}
	return best.Cat
}

// catUsageRatio — used/weight (เหมือน formats.usageRatio แต่ scope เฉพาะ file นี้ ชื่อไม่ชน)
func catUsageRatio(used, weight int) float64 {
	w := weight
	if w < 1 {
		w = 1
	}
	return float64(used) / float64(w)
}

type TitleArchetypesRepo struct {
	pool *pgxpool.Pool
}

func NewTitleArchetypesRepo(pool *pgxpool.Pool) *TitleArchetypesRepo {
	return &TitleArchetypesRepo{pool: pool}
}

// GetAll — ทุก archetype เรียงตามชื่อ
func (r *TitleArchetypesRepo) GetAll(ctx context.Context) ([]models.TitleArchetype, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, archetype_name, display_name, instruction, example, enabled, weight
		FROM title_archetypes
		ORDER BY archetype_name`)
	if err != nil {
		return nil, fmt.Errorf("query title_archetypes: %w", err)
	}
	defer rows.Close()
	out := []models.TitleArchetype{}
	for rows.Next() {
		var a models.TitleArchetype
		if err := rows.Scan(&a.ID, &a.ArchetypeName, &a.DisplayName, &a.Instruction, &a.Example, &a.Enabled, &a.Weight); err != nil {
			return nil, fmt.Errorf("scan title_archetype: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// PickNext — least-used/7d + weight นับจาก clips.title_archetype
func (r *TitleArchetypesRepo) PickNext(ctx context.Context) (*models.TitleArchetype, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ta.id, ta.archetype_name, ta.display_name, ta.instruction, ta.example, ta.enabled, ta.weight,
		       COALESCE(u.cnt, 0) AS used_count
		FROM title_archetypes ta
		LEFT JOIN (
			SELECT title_archetype, COUNT(*) AS cnt
			FROM clips
			WHERE created_at > NOW() - INTERVAL '7 days'
			  AND title_archetype <> ''
			GROUP BY title_archetype
		) u ON u.title_archetype = ta.archetype_name
		WHERE ta.enabled = TRUE
		ORDER BY ta.archetype_name`)
	if err != nil {
		return nil, fmt.Errorf("query title archetype usage: %w", err)
	}
	defer rows.Close()

	usages := []archetypeUsage{}
	for rows.Next() {
		var u archetypeUsage
		if err := rows.Scan(&u.Arch.ID, &u.Arch.ArchetypeName, &u.Arch.DisplayName, &u.Arch.Instruction, &u.Arch.Example, &u.Arch.Enabled, &u.Arch.Weight, &u.UsedCount); err != nil {
			return nil, fmt.Errorf("scan title archetype usage: %w", err)
		}
		usages = append(usages, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(usages) == 0 {
		return nil, nil
	}

	picked := pickTitleArchetypeLeastUsed(usages)
	return &picked, nil
}

// archetypeUsage pairs a title archetype with how many clips used it in the last 7 days.
type archetypeUsage struct {
	Arch      models.TitleArchetype
	UsedCount int
}

// pickTitleArchetypeLeastUsed selects the archetype with the lowest used/weight ratio.
// Pure function — testable without DB.
func pickTitleArchetypeLeastUsed(usages []archetypeUsage) models.TitleArchetype {
	if len(usages) == 0 {
		return models.TitleArchetype{}
	}
	best := usages[0]
	bestRatio := catUsageRatio(best.UsedCount, best.Arch.Weight)
	for _, u := range usages[1:] {
		if r := catUsageRatio(u.UsedCount, u.Arch.Weight); r < bestRatio {
			best, bestRatio = u, r
		}
	}
	return best.Arch
}
