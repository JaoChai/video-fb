package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
)

type TutorialFeaturesRepo struct {
	pool *pgxpool.Pool
}

func NewTutorialFeaturesRepo(pool *pgxpool.Pool) *TutorialFeaturesRepo {
	return &TutorialFeaturesRepo{pool: pool}
}

const tutorialFeatureCols = `id, feature_key, display_name_th, surface, audience, menu_path, ui_vocab,
	steps, trap_th, pain_point, why_matters_th, weight, enabled`

// tutorialParkDays คือระยะเวลาที่ฟีเจอร์ถูกพักหลัง research บอกว่าเมนูย้าย
// การพักต้องมีวันหมดอายุเสมอ: เวอร์ชันแรกพักถาวรและไม่มีทางปลด ทำให้คลัง 8 แถว
// เหลือใช้ได้แถวเดียวภายใน 2 วัน แล้วคลิปสอนซ้ำหัวข้อเดิมทุกวัน
const tutorialParkDays = 14

// TutorialMinPool คือจำนวนฟีเจอร์ที่ต้องเหลือใช้ได้เป็นอย่างน้อย ต่ำกว่านี้แล้ว
// ห้ามพักเพิ่ม ไม่งั้นคำตัดสิน "ย้ายแล้ว" ที่ผิดแค่ครั้งเดียวก็ทำให้คลังหมดเกลี้ยง
// และ ProduceTutorial จะ error ทั้งรอบ
const TutorialMinPool = 3

// tutorialAvailableWhere นิยาม "ฟีเจอร์ที่หยิบมาทำคลิปได้ตอนนี้" ที่เดียว เพื่อให้
// ตัวเลือก (PickNext) กับตัวกันคลังหมด (Park) มองเห็นคลังชุดเดียวกันเสมอ
const tutorialAvailableWhere = `enabled = TRUE AND (parked_until IS NULL OR parked_until <= NOW())`

// GetByKey resolves a feature by its stable key (used by the retry path, which
// only has clips.tutorial_feature to go on).
func (r *TutorialFeaturesRepo) GetByKey(ctx context.Context, key string) (*models.TutorialFeature, error) {
	var f models.TutorialFeature
	var stepsRaw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT `+tutorialFeatureCols+` FROM tutorial_features WHERE feature_key = $1`, key).
		Scan(&f.ID, &f.FeatureKey, &f.DisplayNameTH, &f.Surface, &f.Audience, &f.MenuPath, &f.UIVocab,
			&stepsRaw, &f.TrapTH, &f.PainPoint, &f.WhyMattersTH, &f.Weight, &f.Enabled)
	if err != nil {
		return nil, fmt.Errorf("get tutorial feature %s: %w", key, err)
	}
	if err := unmarshalSteps(stepsRaw, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// PickNext returns the least-used enabled feature whose park window has expired,
// skipping any key in exclude. Returns nil when the catalog is empty. It never
// errors on "everything excluded" — see pickTutorialFeatureLeastUsed, which fails
// open by design.
func (r *TutorialFeaturesRepo) PickNext(ctx context.Context, exclude []string) (*models.TutorialFeature, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+tutorialFeatureCols+`, used_count
		 FROM tutorial_features
		 WHERE `+tutorialAvailableWhere+`
		 ORDER BY feature_key`)
	if err != nil {
		return nil, fmt.Errorf("query tutorial features: %w", err)
	}
	defer rows.Close()

	usages := []tutorialUsage{}
	for rows.Next() {
		var u tutorialUsage
		var stepsRaw []byte
		if err := rows.Scan(&u.Feat.ID, &u.Feat.FeatureKey, &u.Feat.DisplayNameTH, &u.Feat.Surface,
			&u.Feat.Audience, &u.Feat.MenuPath, &u.Feat.UIVocab, &stepsRaw, &u.Feat.TrapTH, &u.Feat.PainPoint,
			&u.Feat.WhyMattersTH, &u.Feat.Weight, &u.Feat.Enabled,
			&u.UsedCount); err != nil {
			return nil, fmt.Errorf("scan tutorial feature usage: %w", err)
		}
		if err := unmarshalSteps(stepsRaw, &u.Feat); err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(usages) == 0 {
		return nil, nil
	}
	picked := pickTutorialFeatureLeastUsed(usages, exclude)
	return &picked, nil
}

// MarkUsed bumps the rotation counters after a clip actually got produced.
func (r *TutorialFeaturesRepo) MarkUsed(ctx context.Context, id string) error {
	if _, err := r.pool.Exec(ctx,
		`UPDATE tutorial_features SET used_count = used_count + 1, last_used_at = NOW() WHERE id = $1`,
		id); err != nil {
		return fmt.Errorf("mark tutorial feature used: %w", err)
	}
	return nil
}

// Park benches a feature whose menu path research says has changed, so no clip
// teaches a stale path. Two rules are baked into the statement itself:
//
//   - the park EXPIRES after tutorialParkDays. An LLM verdict is not certain
//     enough to retire a feature forever, and a catalog that only ever shrinks
//     ends up producing the same clip every day.
//   - it refuses to park below TutorialMinPool. Counting in the same statement
//     is what makes the floor unbreakable: a separate count then update could
//     let two runs both see "one spare left" and park it.
//
// Returns false when the floor stopped the park — the caller then produces the
// feature anyway rather than letting the catalog run dry.
func (r *TutorialFeaturesRepo) Park(ctx context.Context, id, reason string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tutorial_features
		 SET verify_reason = $2, parked_until = NOW() + make_interval(days => $3)
		 WHERE id = $1
		   AND (SELECT COUNT(*) FROM tutorial_features WHERE `+tutorialAvailableWhere+`) > $4`,
		id, reason, tutorialParkDays, TutorialMinPool)
	if err != nil {
		return false, fmt.Errorf("park tutorial feature: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// Unpark puts a feature back in the pool immediately — the manual counterpart of
// waiting out the park window, for when a human has checked the menu themselves.
// Keyed by feature_key so it can be called with the name that shows up in logs.
func (r *TutorialFeaturesRepo) Unpark(ctx context.Context, featureKey string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tutorial_features SET parked_until = NULL WHERE feature_key = $1`, featureKey)
	if err != nil {
		return fmt.Errorf("unpark tutorial feature: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("tutorial feature %s not found", featureKey)
	}
	return nil
}

// unmarshalSteps decodes the steps JSONB column. A malformed value is an error,
// not a silent empty list: a feature with no steps would produce a tutorial that
// teaches nothing and still pass the step-count gate at zero.
func unmarshalSteps(raw []byte, f *models.TutorialFeature) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &f.Steps); err != nil {
		return fmt.Errorf("unmarshal steps for %s: %w", f.FeatureKey, err)
	}
	return nil
}

// tutorialUsage pairs a feature with how many clips have used it.
type tutorialUsage struct {
	Feat      models.TutorialFeature
	UsedCount int
}

// pickTutorialFeatureLeastUsed selects the feature with the lowest used/weight
// ratio, skipping any key in exclude. If every feature is excluded the exclude
// rule is dropped for this pick — producing a slightly-repeated clip always
// beats producing none. Pure function — testable without a DB.
func pickTutorialFeatureLeastUsed(usages []tutorialUsage, exclude []string) models.TutorialFeature {
	if len(usages) == 0 {
		return models.TutorialFeature{}
	}
	excludeSet := map[string]bool{}
	for _, e := range exclude {
		excludeSet[e] = true
	}
	pool := make([]tutorialUsage, 0, len(usages))
	for _, u := range usages {
		if !excludeSet[u.Feat.FeatureKey] {
			pool = append(pool, u)
		}
	}
	if len(pool) == 0 {
		pool = usages
	}
	best := pool[0]
	bestRatio := catUsageRatio(best.UsedCount, best.Feat.Weight)
	for _, u := range pool[1:] {
		if r := catUsageRatio(u.UsedCount, u.Feat.Weight); r < bestRatio {
			best, bestRatio = u, r
		}
	}
	return best.Feat
}
