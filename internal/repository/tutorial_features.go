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

// TutorialMinPool คือจำนวนฟีเจอร์ที่ต้องเหลือใช้ได้เป็นอย่างน้อย พื้นนี้กัน
// "คลังยุบจนวนถี่" ไม่ใช่แค่กัน "ไม่มีคลิป" — คลังที่เหลือใช้ได้ไม่กี่แถวทำให้คลิป
// สอนซ้ำหัวข้อเดิมทุกวันแม้ ProduceTutorial จะยังไม่ error
const TutorialMinPool = 10

// tutorialStrikeWindowDays คือหน้าต่างเวลาที่ธง "เมนูย้าย" ใบแรกยังนับ ถ้าไม่โดน
// ธงซ้ำภายในนี้ ครั้งถัดไปจะถูกนับเป็นธงใบแรกใหม่แทนที่จะพักทันที
const tutorialStrikeWindowDays = 30

// ParkOutcome คือผลลัพธ์ของ Park หนึ่งครั้ง — สื่อว่าเกิดอะไรขึ้นจริงแทนการคืน
// bool เฉยๆ ซึ่งแยกไม่ออกว่า "โดนธงใบแรก" กับ "ชนพื้นคลัง"
type ParkOutcome string

const (
	// ParkFirstStrike: บันทึก flagged_at ไว้ ข้ามรอบนี้ ยังไม่พักจริง
	ParkFirstStrike ParkOutcome = "first_strike"
	// ParkedForVerify: โดนธงซ้ำภายในหน้าต่าง → พักจริง tutorialParkDays วัน
	ParkedForVerify ParkOutcome = "parked"
	// ParkRefusedFloor: คลังเหลือน้อยถึงพื้น TutorialMinPool → ห้ามพัก
	ParkRefusedFloor ParkOutcome = "floor"
)

// tutorialAvailableWhere นิยาม "ฟีเจอร์ที่หยิบมาทำคลิปได้ตอนนี้" ที่เดียว เพื่อให้
// ตัวเลือก (PickNext) กับตัวกันคลังหมด (Park) มองเห็นคลังชุดเดียวกันเสมอ
const tutorialAvailableWhere = `enabled = TRUE AND (parked_until IS NULL OR parked_until <= NOW())`

// ระดับของหัวข้อ = ช่วงเวลาที่หยิบไปใช้ ไม่ใช่แค่ป้ายกำกับ
// advanced = คลิป 21:00 สำหรับคนยิงแอดอยู่แล้ว · basic = คลิป 15:00 สำหรับคนเพิ่งเริ่ม
const (
	TutorialLevelAdvanced = "advanced"
	TutorialLevelBasic    = "basic"
)

// tutorialPickSQL แยกออกมาเป็น const เพื่อให้เทสต์อ่านรูปร่างของเงื่อนไขได้โดยไม่
// ต้องมีฐานข้อมูล — คลังสองระดับใช้ตารางเดียวกัน การลืม level ตรงนี้แปลว่าคลิป
// 21:00 จะสุ่มได้หัวข้อพื้นฐาน
const tutorialPickSQL = `SELECT ` + tutorialFeatureCols + `, used_count
	 FROM tutorial_features
	 WHERE ` + tutorialAvailableWhere + ` AND level = $1
	 ORDER BY feature_key`

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
func (r *TutorialFeaturesRepo) PickNext(ctx context.Context, level string, exclude []string) (*models.TutorialFeature, error) {
	rows, err := r.pool.Query(ctx, tutorialPickSQL, level)
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

// tutorialFirstStrikeSQL records the first "menu moved" verdict without parking
// anything. It only fires when the feature has no flag within tutorialStrikeWindowDays
// — a stale flag from outside the window is treated as if it never happened.
const tutorialFirstStrikeSQL = `
	UPDATE tutorial_features
	SET flagged_at = NOW(), verify_reason = $2
	WHERE id = $1
	  AND (flagged_at IS NULL OR flagged_at < NOW() - make_interval(days => $3))`

// tutorialSecondStrikeSQL parks the feature for real. It only runs when the first
// strike statement above touched zero rows, meaning the feature was already
// flagged inside the window — this is strike two. The floor count is in the same
// statement as the park, which is what makes the floor unbreakable: a separate
// count then update could let two runs both see "one spare left" and park it.
//
// พื้นนับเฉพาะหัวข้อระดับเดียวกับแถวที่กำลังพัก (correlated subquery) — คลัง basic
// กับ advanced เป็นคนละคิว ถ้านับรวมกัน การพักแถว basic จะผ่านตลอดเพราะไปนับ
// แถว advanced ที่ยังว่างอยู่ แล้วคลัง basic ก็ยุบจนวนซ้ำทุกไม่กี่วัน
//
// ใช้ tutorialAvailableWhere ตัวเดียวกับ PickNext (ชื่อคอลัมน์ที่ไม่ระบุตารางใน
// subquery ผูกกับ t2) เพื่อไม่ให้พื้นนับคนละพูลกับตัวเลือกเมื่อมีใครแก้เงื่อนไข
const tutorialSecondStrikeSQL = `
	UPDATE tutorial_features
	SET verify_reason = $2, parked_until = NOW() + make_interval(days => $3), flagged_at = NULL
	WHERE id = $1
	  AND (SELECT COUNT(*) FROM tutorial_features t2
	       WHERE ` + tutorialAvailableWhere + `
	         AND t2.level = tutorial_features.level) > $4`

// Park benches a feature whose menu path research says has changed, so no clip
// teaches a stale path. A single "moved" verdict is not accurate enough to park
// on the spot — production data showed 7 parks from ~5 production runs — so
// parking now takes two strikes within tutorialStrikeWindowDays:
//
//   - 1st strike: flagged_at is stamped, nothing is parked yet (ParkFirstStrike).
//   - 2nd strike inside the window: the feature actually parks for
//     tutorialParkDays, and flagged_at resets to NULL so the next flag after the
//     park expires starts a fresh count (ParkedForVerify).
//   - the 2nd strike still refuses to park below TutorialMinPool (ParkRefusedFloor)
//     — the floor count and the park write are one statement so two concurrent
//     runs can't both see "one spare left" and park it.
func (r *TutorialFeaturesRepo) Park(ctx context.Context, id, reason string) (ParkOutcome, error) {
	firstTag, err := r.pool.Exec(ctx, tutorialFirstStrikeSQL, id, reason, tutorialStrikeWindowDays)
	if err != nil {
		return "", fmt.Errorf("park tutorial feature (first strike): %w", err)
	}
	if firstTag.RowsAffected() > 0 {
		return ParkFirstStrike, nil
	}

	secondTag, err := r.pool.Exec(ctx, tutorialSecondStrikeSQL,
		id, reason, tutorialParkDays, TutorialMinPool)
	if err != nil {
		return "", fmt.Errorf("park tutorial feature (second strike): %w", err)
	}
	if secondTag.RowsAffected() > 0 {
		return ParkedForVerify, nil
	}
	return ParkRefusedFloor, nil
}

// Unpark puts a feature back in the pool immediately — the manual counterpart of
// waiting out the park window, for when a human has checked the menu themselves.
// Keyed by feature_key so it can be called with the name that shows up in logs.
// Clears flagged_at too: a human has confirmed the menu is fine, so the old flag
// history must not survive to park the feature on a single flag next time.
func (r *TutorialFeaturesRepo) Unpark(ctx context.Context, featureKey string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tutorial_features SET parked_until = NULL, flagged_at = NULL WHERE feature_key = $1`, featureKey)
	if err != nil {
		return fmt.Errorf("unpark tutorial feature: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("tutorial feature %s not found", featureKey)
	}
	return nil
}

// RecentAngles returns the youtube_title of previous clips that taught the
// same catalog feature, most recent first. It lives here (not on ClipsRepo)
// because the feature key is what the caller has, even though the query joins
// through clips/clip_metadata to get the title. Used to tell the script agent
// which hooks/angles are already spent so it doesn't repeat one nearly
// verbatim. Returns []string{} (never nil) when there are no prior clips.
func (r *TutorialFeaturesRepo) RecentAngles(ctx context.Context, featureKey string, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT m.youtube_title
		 FROM clips c
		 JOIN clip_metadata m ON m.clip_id = c.id
		 WHERE c.tutorial_feature = $1 AND m.youtube_title <> ''
		 ORDER BY c.created_at DESC
		 LIMIT $2`, featureKey, limit)
	if err != nil {
		return nil, fmt.Errorf("recent angles for %s: %w", featureKey, err)
	}
	defer rows.Close()

	angles := []string{}
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, fmt.Errorf("scan recent angle: %w", err)
		}
		angles = append(angles, title)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return angles, nil
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
