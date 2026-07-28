# ช่วงคอนเทนต์พื้นฐาน 15:00 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** เพิ่มคลิปสอนพื้นฐาน Facebook Ads วันละ 1 ใบเวลา 15:00 โดยใช้เครื่องผลิตของคลิป 21:00 ทั้งหมด เปลี่ยนเฉพาะชุดพรอมป์ที่เขียนบทและคลังหัวข้อ

**Architecture:** แยก "โหมดภาพ" (`clipMode` → ยังคืน `ModeTutorial` ให้คลิป basic เพื่อให้หน้าตาเหมือนเดิมเป๊ะ) ออกจาก "โหมดพรอมป์" (`agentModeFor` ใหม่ → คืน `"basic"` เพื่อไปหยิบ agent row `script_basic`/`scene_basic`/`critic_basic`) คลังหัวข้อเดิมเพิ่มคอลัมน์ `level` ทำให้ 21:00 กับ 15:00 มีคิวหมุนและพื้นกันคลังยุบของตัวเองแยกกัน

**Tech Stack:** Go 1.x · pgx/v5 · Postgres (Neon) · chi router · migrations แบบไฟล์ SQL รันตอน boot

**Spec:** `docs/superpowers/specs/2026-07-28-basic-tutorial-slot-design.md`

## Global Constraints

- **ห้ามแตะตะแกรง `ui_vocab`** (`tutorialGateFailure`, `agent.UIVocabViolations`) — คลิป auto-publish โดยไม่มีคนดูก่อน นี่คือสิ่งเดียวที่กันคลิปสอนเมนูปลอม
- **หน้าตาคลิป basic ต้องเหมือน tutorial เป๊ะ** — ห้ามเพิ่มค่า mode ใหม่ใน `internal/producer/` ห้ามแตะ `case_format.go`, `TutorialPreset`, template
- **ห้ามแก้พรอมป์ของ 3 ช่วงที่วิ่งอยู่** — row `script`, `script_tutorial`, `script_case`, `scene_tutorial`, `critic_tutorial` ต้องไม่ถูก UPDATE
- migration ใหม่เริ่มที่ **070** (069 ถูกใช้แล้ว) ทุกไฟล์ต้องหุ้ม `BEGIN; … COMMIT;` เอง (RunMigrations ไม่หุ้มให้) และ idempotent (`IF NOT EXISTS` / `ON CONFLICT DO NOTHING`)
- **ทุกตัวกรองที่ทำให้ "ไม่เหลือหัวข้อ" ต้อง fail-open ห้ามคืน 0 คลิปเงียบๆ** (บทเรียนจาก `project_question_cooldown_deadlock`)
- ห้ามเพิ่ม env var ใหม่
- schedule ใหม่ต้อง `enabled = FALSE` — เปิดทีหลังผ่าน PATCH API เท่านั้น (scheduler อ่าน DB แค่ตอน `Start()`)
- คอมเมนต์: ภาษาไทยสำหรับ constant/เหตุผลเชิงธุรกิจ, อังกฤษสำหรับ doc comment ของฟังก์ชัน — ตามสไตล์ที่ไฟล์นั้นใช้อยู่จริง
- `go build`/`go test` ล้มใน sandbox ("operation not permitted" ที่ go build cache) — ทุกคำสั่ง go ต้องรันด้วย `dangerouslyDisableSandbox: true`
- ห้าม reformat โค้ดข้างเคียง (`internal/router/router.go` มี gofmt drift เดิม อย่าไปแก้)
- ทุก task ต้องผ่าน: `go build ./...` · `go vet ./internal/...` · `go test -count=1 ./internal/...`

---

## File Structure

| ไฟล์ | หน้าที่ |
|---|---|
| `migrations/070_tutorial_level.sql` (สร้าง) | คอลัมน์ `level` + backfill แถวเดิมเป็น `advanced` |
| `migrations/071_basic_slot_rows.sql` (สร้าง) | content_format `basic`, agent rows 3 ตัว, topic_category `beginner`, schedule 15:00 (ปิด) |
| `migrations/072_basic_catalog_seed.sql` (สร้าง) | 12 หัวข้อพื้นฐาน |
| `internal/repository/tutorial_features.go` (แก้) | `PickNext`/`Park` รับ/อิง `level` |
| `internal/orchestrator/tutorial.go` (แก้) | `clipMode` รับ basic, `agentModeFor` ใหม่, `produceCatalogClip` ที่ `ProduceTutorial`/`ProduceBasic` ใช้ร่วมกัน |
| `internal/orchestrator/orchestrator.go` (แก้) | 4 จุดหา agent config → `agentModeFor` |
| `internal/scheduler/scheduler.go` (แก้) | `case "produce_basic"` |
| `internal/handler/orchestrator.go` + `internal/router/router.go` (แก้) | endpoint ยิงมือ |
| `internal/agent/tutorial_seed_test.go` (แก้) | เพิ่มไฟล์ seed ใหม่เข้าตารางตรวจ |

---

## Task 1: คลังหัวข้อรองรับ 2 ระดับ

**Files:**
- Create: `migrations/070_tutorial_level.sql`
- Modify: `internal/repository/tutorial_features.go`
- Modify: `internal/orchestrator/tutorial.go` (เฉพาะจุดที่เรียก `PickNext`/`Park`)
- Test: `internal/repository/tutorial_features_test.go`

**Interfaces:**
- Consumes: `tutorialAvailableWhere`, `TutorialMinPool`, `tutorialParkDays`, `tutorialStrikeWindowDays` (มีอยู่แล้ว)
- Produces:
  - `const TutorialLevelAdvanced = "advanced"` / `const TutorialLevelBasic = "basic"`
  - `func (r *TutorialFeaturesRepo) PickNext(ctx context.Context, level string, exclude []string) (*models.TutorialFeature, error)`
  - `func (o *Orchestrator) pickVerifiedFeature(ctx context.Context, level string) (*models.TutorialFeature, error)`
  - `Park` signature ไม่เปลี่ยน แต่พื้นนับเฉพาะ level เดียวกับแถวที่กำลังพัก

- [ ] **Step 1: เขียน migration**

สร้าง `migrations/070_tutorial_level.sql`:

```sql
-- 070: แยกคลังหัวข้อเป็น 2 ระดับ — คลิป 21:00 (advanced) กับคลิป 15:00 (basic)
--
-- ถ้าใช้คลังเดียวกัน คลิป 21:00 ที่ทำให้คนยิงหนักจะสุ่มได้ "CPM คืออะไร"
-- ซึ่งทำลายจุดขายของช่วงนั้น การแยกด้วย level ทำให้แต่ละช่วงมีคิวหมุนของตัวเอง
-- และมีพื้นกันคลังยุบของตัวเอง โดยไม่ต้องแก้ตรรกะการเลือกหัวข้อเลย
--
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback: ALTER TABLE tutorial_features DROP COLUMN level;
BEGIN;

ALTER TABLE tutorial_features
  ADD COLUMN IF NOT EXISTS level TEXT NOT NULL DEFAULT 'advanced';

COMMIT;
```

- [ ] **Step 2: เขียนเทสต์ที่ต้องแดง**

เพิ่มใน `internal/repository/tutorial_features_test.go`:

```go
// คลัง basic กับ advanced ต้องนับพื้นแยกกัน ไม่งั้นการพักหัวข้อ basic จะถูกปฏิเสธ
// เพราะไปนับรวมกับแถว advanced ที่ยังว่างอยู่ (หรือกลับกัน)
func TestParkFloorCountsOnlyTheSameLevel(t *testing.T) {
	if !strings.Contains(tutorialParkSQL, "level") {
		t.Error("park floor subquery must be scoped to the row's own level")
	}
	if !strings.Contains(tutorialPickSQL, "level") {
		t.Error("PickNext must filter by level — one catalog, two independent queues")
	}
}

func TestTutorialLevelsAreDistinct(t *testing.T) {
	if TutorialLevelAdvanced == TutorialLevelBasic {
		t.Fatal("levels must differ")
	}
	for _, l := range []string{TutorialLevelAdvanced, TutorialLevelBasic} {
		if strings.TrimSpace(l) == "" {
			t.Error("level constants must not be empty — an empty level would match no rows")
		}
	}
}
```

- [ ] **Step 3: รันเทสต์ให้เห็นว่าแดง**

Run: `go test -count=1 -run "TestParkFloorCountsOnlyTheSameLevel|TestTutorialLevelsAreDistinct" ./internal/repository/`
Expected: FAIL — `undefined: TutorialLevelAdvanced`, `undefined: tutorialPickSQL`

- [ ] **Step 4: เพิ่ม constant + แยก SQL ออกมาเป็นตัวแปรระดับ package**

ใน `internal/repository/tutorial_features.go` เพิ่มใต้ `tutorialAvailableWhere`:

```go
// ระดับของหัวข้อ = ช่วงเวลาที่หยิบไปใช้ ไม่ใช่แค่ป้ายกำกับ
// advanced = คลิป 21:00 สำหรับคนยิงแอดอยู่แล้ว · basic = คลิป 15:00 สำหรับคนเพิ่งเริ่ม
const (
	TutorialLevelAdvanced = "advanced"
	TutorialLevelBasic    = "basic"
)

// tutorialPickSQL และ tutorialParkSQL แยกออกมาเป็นตัวแปรเพื่อให้เทสต์อ่านรูปร่าง
// ของเงื่อนไขได้โดยไม่ต้องมีฐานข้อมูล
const tutorialPickSQL = `SELECT ` + tutorialFeatureCols + `, used_count
	 FROM tutorial_features
	 WHERE ` + tutorialAvailableWhere + ` AND level = $1
	 ORDER BY feature_key`
```

**หมายเหตุ:** ไฟล์นี้อาจมี const SQL ของ two-strike อยู่แล้วจาก migration 069
(เทสต์ `TestParkStrikeSQLShape` อ้างถึงมัน) — ถ้าชื่อที่มีอยู่ต่างจาก `tutorialParkSQL`
ให้**ใช้ชื่อเดิมที่มีอยู่** แล้วปรับเทสต์ใน Step 2 ให้อ้างชื่อนั้นแทน ห้ามสร้าง const
ซ้ำซ้อนสองตัวที่เก็บ SQL อันเดียวกัน

- [ ] **Step 5: เปลี่ยน `PickNext` ให้รับ level**

แทนที่ body ของ query ใน `PickNext`:

```go
func (r *TutorialFeaturesRepo) PickNext(ctx context.Context, level string, exclude []string) (*models.TutorialFeature, error) {
	rows, err := r.pool.Query(ctx, tutorialPickSQL, level)
	if err != nil {
		return nil, fmt.Errorf("query tutorial features: %w", err)
	}
	// ...ส่วนที่เหลือของฟังก์ชันคงเดิมทุกบรรทัด...
```

- [ ] **Step 6: ทำให้พื้นนับเฉพาะ level เดียวกัน**

แทนที่ SQL ของ statement ที่สองใน `Park` ด้วยตัวแปร package แล้วใช้ correlated subquery
(ไม่ต้องเพิ่มพารามิเตอร์ level — อ่านจากแถวที่กำลังพักเอง):

```go
// พื้นต้องนับเฉพาะหัวข้อระดับเดียวกัน: คลัง basic 12 แถวกับ advanced 18 แถว
// เป็นคนละคิว ถ้านับรวมกัน การพักแถว basic จะถูกอนุญาตทั้งที่คลัง basic ใกล้หมด
const tutorialParkSQL = `UPDATE tutorial_features
	 SET verify_reason = $2, parked_until = NOW() + make_interval(days => $3), flagged_at = NULL
	 WHERE id = $1
	   AND (SELECT COUNT(*) FROM tutorial_features t2
	        WHERE t2.enabled = TRUE
	          AND (t2.parked_until IS NULL OR t2.parked_until <= NOW())
	          AND t2.level = tutorial_features.level) > $4`
```

แล้วให้ `Park` ใช้ `tutorialParkSQL` แทน SQL ที่ฝังอยู่เดิม (พารามิเตอร์ชุดเดิมทุกตัว)

- [ ] **Step 7: อัปเดตผู้เรียกใน orchestrator**

ใน `internal/orchestrator/tutorial.go` เปลี่ยน `pickVerifiedFeature` ให้รับ level แล้วส่งต่อ:

```go
func (o *Orchestrator) pickVerifiedFeature(ctx context.Context, level string) (*models.TutorialFeature, error) {
	// maxSkips bounds latency, not catalog size: each skip costs a research call
	// plus a verify call. The floor that keeps the catalog usable lives in Park.
	const maxSkips = 2
	var skipped []string
	var last *models.TutorialFeature

	for attempt := 0; attempt <= maxSkips; attempt++ {
		feat, err := o.tutorialFeaturesRepo.PickNext(ctx, level, skipped)
		// ...ส่วนที่เหลือคงเดิมทุกบรรทัด...
```

และที่ `ProduceTutorial` เปลี่ยนการเรียกเป็น:

```go
feat, err := o.pickVerifiedFeature(ctx, repository.TutorialLevelAdvanced)
```

(ต้อง import `internal/repository` ใน `tutorial.go` — ตอนนี้ยังไม่ได้ import)

- [ ] **Step 8: รันเทสต์ให้เขียว**

Run: `go build ./... && go test -count=1 ./internal/...`
Expected: PASS ทุก package

- [ ] **Step 9: Commit**

```bash
git add migrations/070_tutorial_level.sql internal/repository/tutorial_features.go internal/repository/tutorial_features_test.go internal/orchestrator/tutorial.go
git commit -m "feat(tutorial): แยกคลังหัวข้อเป็น 2 ระดับ ด้วยคอลัมน์ level"
```

---

## Task 2: แยกโหมดพรอมป์ออกจากโหมดภาพ + เส้นทางผลิตคลิป basic

**Files:**
- Modify: `internal/orchestrator/tutorial.go`
- Modify: `internal/orchestrator/orchestrator.go` (บรรทัด 318, 494, 513, 875 — จุดหา agent config)
- Modify: `internal/scheduler/scheduler.go`
- Modify: `internal/handler/orchestrator.go`
- Modify: `internal/router/router.go`
- Test: `internal/orchestrator/tutorial_test.go`

**Interfaces:**
- Consumes: `repository.TutorialLevelBasic` / `TutorialLevelAdvanced` (Task 1)
- Produces:
  - `const basicFormatName = "basic"` และ `const basicAgentMode = "basic"`
  - `func agentModeFor(contentFormat string) string`
  - `func (o *Orchestrator) ProduceBasic(ctx context.Context) error`
  - `func (o *Orchestrator) produceCatalogClip(ctx context.Context, level, formatName, agentMode, label string) error`

- [ ] **Step 1: เขียนเทสต์ที่ต้องแดง**

เพิ่มใน `internal/orchestrator/tutorial_test.go`:

```go
// คลิป basic ต้องหน้าตาเหมือนคลิป tutorial เป๊ะ (โหมดภาพเดียวกัน) แต่ใช้พรอมป์
// คนละชุด ถ้าสองอย่างนี้ผูกติดกันเมื่อไหร่ ต้องเลือกอย่างใดอย่างหนึ่งเสียเสมอ
func TestBasicClipLooksLikeTutorialButUsesItsOwnPrompts(t *testing.T) {
	if got := clipMode(basicFormatName); got != producer.ModeTutorial {
		t.Errorf("clipMode(basic) = %q, want tutorial — visuals must be identical", got)
	}
	if got := agentModeFor(basicFormatName); got != basicAgentMode {
		t.Errorf("agentModeFor(basic) = %q, want %q — basic needs its own voice", got, basicAgentMode)
	}
	if got := agentModeFor(tutorialFormatName); got != producer.ModeTutorial {
		t.Errorf("agentModeFor(tutorial) = %q, want tutorial — must not change", got)
	}
	if got := agentModeFor("qa"); got != clipMode("qa") {
		t.Errorf("agentModeFor(qa) = %q, want the same as clipMode — other formats unchanged", got)
	}
}

// โหมด tutorial ยังต้องได้ผลเดิมทุกอย่างหลังเพิ่ม basic เข้ามา
func TestTutorialModeUnchangedByBasic(t *testing.T) {
	t.Setenv("CASE_FORMAT_ENABLED", "true")
	if got := clipMode(tutorialFormatName); got != producer.ModeTutorial {
		t.Errorf("tutorial = %q, want tutorial even with the case flag on", got)
	}
	if got := clipMode(basicFormatName); got != producer.ModeTutorial {
		t.Errorf("basic = %q, want tutorial even with the case flag on", got)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าแดง**

Run: `go test -count=1 -run "TestBasicClipLooksLikeTutorial|TestTutorialModeUnchangedByBasic" ./internal/orchestrator/`
Expected: FAIL — `undefined: basicFormatName`

- [ ] **Step 3: เพิ่ม constant + `agentModeFor` + แก้ `clipMode`**

ใน `internal/orchestrator/tutorial.go` แทนที่ `clipMode` เดิมด้วย:

```go
// basicFormatName คือ content_formats row ของคลิปสอนพื้นฐาน (seed ไว้แบบ disabled
// เพื่อไม่ให้ตัวสุ่ม format ของรอบปกติหยิบไปใช้) — คู่กับ tutorialFormatName
const basicFormatName = "basic"

// basicAgentMode คือ suffix ของ agent row ที่เขียนบทคลิปพื้นฐาน (script_basic ฯลฯ)
// จงใจไม่เพิ่มค่านี้เป็นโหมดใน internal/producer เพราะ "ใครเขียนบท" กับ
// "หน้าตาเป็นแบบไหน" เป็นคนละคำถาม และคลิป basic ต้องหน้าตาเหมือน tutorial เป๊ะ
const basicAgentMode = "basic"

// clipMode derives the RENDER mode from the clip's persisted content_format.
// Tutorial wins over the process-wide case flag so a tutorial clip renders as a
// manual even on a server where CASE_FORMAT_ENABLED is on. Basic clips render
// identically to tutorial clips — same preset, same image policy, same gate.
func clipMode(contentFormat string) string {
	if contentFormat == tutorialFormatName || contentFormat == basicFormatName {
		return producer.ModeTutorial
	}
	if producer.CaseFormatEnabled() {
		return producer.ModeCase
	}
	return producer.ModeClassic
}

// agentModeFor answers a different question from clipMode: which prompt rows
// write this clip. A basic clip looks exactly like a tutorial clip but is
// written for someone who has never run an ad, so it needs its own
// script/scene/critic rows while sharing every pixel of the tutorial's visuals.
func agentModeFor(contentFormat string) string {
	if contentFormat == basicFormatName {
		return basicAgentMode
	}
	return clipMode(contentFormat)
}
```

- [ ] **Step 4: เปลี่ยน 4 จุดที่หา agent config ให้ใช้ `agentModeFor`**

ใน `internal/orchestrator/orchestrator.go` เปลี่ยนเฉพาะ 4 บรรทัดนี้ (จุดอื่นที่ใช้ `clipMode` ห้ามแตะ):

```go
// บรรทัด ~318
scriptCfg, err := o.modeAgentConfig(ctx, "script", agentModeFor(format.FormatName))
// บรรทัด ~494
sceneCfg, err := o.modeAgentConfig(ctx, "scene", agentModeFor(format.FormatName))
// บรรทัด ~513
if criticCfg, cErr := o.modeAgentConfig(ctx, "critic", agentModeFor(format.FormatName)); cErr == nil && criticCfg.Enabled {
// บรรทัด ~875 (เส้นทาง retry — ได้เสียงถูกชุดอัตโนมัติเพราะอ่านจาก clips.content_format)
scriptCfg, err := o.modeAgentConfig(ctx, "script", agentModeFor(clip.ContentFormat))
```

- [ ] **Step 5: รันเทสต์ให้เขียว**

Run: `go test -count=1 -run "TestBasicClipLooksLikeTutorial|TestTutorialModeUnchangedByBasic" ./internal/orchestrator/`
Expected: PASS

- [ ] **Step 6: รวมร่าง `ProduceTutorial` เป็นฟังก์ชันที่ใช้ร่วมกัน**

แทนที่ `func (o *Orchestrator) ProduceTutorial(...)` ทั้งฟังก์ชันด้วยสามส่วนนี้
(เนื้อในยกมาจากของเดิมทุกบรรทัด เปลี่ยนเฉพาะที่ทำเป็นพารามิเตอร์):

```go
// ProduceTutorial produces the daily advanced tutorial clip (21:00 slot).
func (o *Orchestrator) ProduceTutorial(ctx context.Context) error {
	return o.produceCatalogClip(ctx, repository.TutorialLevelAdvanced,
		tutorialFormatName, producer.ModeTutorial, "tutorial")
}

// ProduceBasic produces the daily beginner clip (15:00 slot). Same machinery,
// same visuals, different catalog level and a script agent that explains every
// term instead of assuming the viewer already runs ads.
func (o *Orchestrator) ProduceBasic(ctx context.Context) error {
	return o.produceCatalogClip(ctx, repository.TutorialLevelBasic,
		basicFormatName, basicAgentMode, "basic")
}

// produceCatalogClip produces exactly one clip whose topic comes from the
// tutorial catalog. It mirrors ProduceWeekly's gate/tracker discipline but skips
// the question agent entirely — the topic IS the catalog row, so there is
// nothing to invent. label appears in logs and error messages only.
func (o *Orchestrator) produceCatalogClip(ctx context.Context, level, formatName, agentMode, label string) error {
	if credits, err := o.producer.KieCredits(ctx); err != nil {
		log.Printf("kie credit pre-check skipped (non-fatal): %v", err)
	} else if credits <= 0 {
		return fmt.Errorf("kie เครดิตหมด (เหลือ %d) — เติมเครดิตที่ kie.ai ก่อนผลิต", credits)
	}

	feat, err := o.pickVerifiedFeature(ctx, level)
	if err != nil {
		return err
	}
	if feat == nil {
		return fmt.Errorf("%s catalog is empty or every feature is parked for re-verification", label)
	}
	if len(feat.Steps) == 0 {
		return fmt.Errorf("%s feature %s has no steps — fix the catalog row", label, feat.FeatureKey)
	}
	log.Printf("Producing %s clip — feature: %s (%d steps)", label, feat.FeatureKey, len(feat.Steps))

	if !o.tracker.StartProduction(1) {
		return ErrProductionRunning
	}
	defer o.tracker.FinishProduction()

	ctx, cancel := context.WithCancel(ctx)
	o.tracker.SetCancelFunc(cancel)
	defer cancel()

	theme, err := o.themesRepo.GetActive(ctx)
	if err != nil {
		return fmt.Errorf("get active theme: %w", err)
	}
	scriptCfg, err := o.modeAgentConfig(ctx, "script", agentMode)
	if err != nil {
		return fmt.Errorf("get script agent config: %w", err)
	}
	imageCfg, err := o.agentsRepo.GetByName(ctx, "image")
	if err != nil {
		return fmt.Errorf("get image agent config: %w", err)
	}
	brandAliases, err := o.settingsRepo.GetBrandAliases(ctx)
	if err != nil {
		return fmt.Errorf("read brand aliases: %w", err)
	}
	format, err := o.formatsRepo.GetByName(ctx, formatName)
	if err != nil {
		return fmt.Errorf("get %s content format: %w", label, err)
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var archetype models.TitleArchetype
	if a, aerr := o.titleArchetypesRepo.PickNext(ctx); aerr != nil || a == nil {
		log.Printf("%s: archetype pick failed, using empty: %v", label, aerr)
	} else {
		archetype = tutorialArchetype(a)
	}

	var persona string
	personasJSON, _ := o.settingsRepo.Get(ctx, "audience_personas")
	var personas []string
	if json.Unmarshal([]byte(personasJSON), &personas) == nil && len(personas) > 0 {
		persona = PickPersona(personas, rng)
	} else {
		persona, _ = o.settingsRepo.Get(ctx, "audience_persona")
	}

	q := agent.GeneratedQuestion{
		Question:  feat.DisplayNameTH,
		Category:  feat.Audience,
		PainPoint: feat.PainPoint,
	}

	o.tracker.SetTotalClips(1)
	o.tracker.StartClip(1, feat.DisplayNameTH)
	if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format,
		persona, archetype, "reach", feat); err != nil {
		o.tracker.AddErrorLog(fmt.Sprintf("%s clip failed: %v", label, err))
		o.nudgeRetry()
		return err
	}
	if mErr := o.tutorialFeaturesRepo.MarkUsed(ctx, feat.ID); mErr != nil {
		log.Printf("%s: MarkUsed failed (non-fatal): %v", label, mErr)
	}
	o.tracker.CompleteStep("complete")
	log.Printf("%s production complete", label)
	return nil
}
```

- [ ] **Step 7: ต่อ scheduler**

ใน `internal/scheduler/scheduler.go` เพิ่ม wrapper ใต้ `produceTutorial`:

```go
// produceBasic produces the daily beginner clip from the catalog.
func (s *Scheduler) produceBasic(ctx context.Context) error {
	return s.produceTick(ctx, "the daily basic clip", s.orchestrator.ProduceBasic)
}
```

และเพิ่ม case ใน `handlerFor` ใต้ `case "produce_tutorial":`

```go
	case "produce_basic":
		return s.produceBasic
```

- [ ] **Step 8: ต่อ endpoint ยิงมือ**

ใน `internal/handler/orchestrator.go` เพิ่มใต้ `TriggerTutorial`:

```go
func (h *OrchestratorHandler) TriggerBasic(w http.ResponseWriter, r *http.Request) {
	if s := h.tracker.GetStatus(); s.Active {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: "Production already in progress"})
		return
	}

	writeJSON(w, http.StatusAccepted, models.APIResponse{
		Message: "Basic production started in background",
	})

	go func() {
		// ProduceBasic owns the production gate AND its cancellation
		// registration, so the handler just kicks it off with a base context.
		if err := h.orch.ProduceBasic(context.Background()); err != nil {
			log.Printf("Basic production failed: %v", err)
			h.tracker.AddErrorLog(err.Error())
		}
	}()
}
```

ใน `internal/router/router.go` ใน `SetOrchestrator` เพิ่มใต้บรรทัด `produce-tutorial`:

```go
	r.Post("/api/v1/orchestrator/produce-basic", h.TriggerBasic)
```

- [ ] **Step 9: เพิ่มเทสต์ว่า route ถูก register**

`internal/router/router_test.go` มีเทสต์ตรวจรายการ route อยู่แล้ว — เพิ่ม
`"/api/v1/orchestrator/produce-basic"` เข้าไปในลิสต์ที่มันวนตรวจ

- [ ] **Step 10: รันทั้งหมดให้เขียว**

Run: `go build ./... && go vet ./internal/... && go test -count=1 ./internal/...`
Expected: PASS ทุก package

- [ ] **Step 11: Commit**

```bash
git add internal/orchestrator/tutorial.go internal/orchestrator/orchestrator.go internal/orchestrator/tutorial_test.go internal/scheduler/scheduler.go internal/handler/orchestrator.go internal/router/router.go internal/router/router_test.go
git commit -m "feat(basic): เส้นทางผลิตคลิปพื้นฐาน แยกโหมดพรอมป์ออกจากโหมดภาพ"
```

---

## Task 3: แถวข้อมูลของช่วง 15:00

**Files:**
- Create: `migrations/071_basic_slot_rows.sql`

**Interfaces:**
- Consumes: `basicFormatName` = `"basic"`, `basicAgentMode` = `"basic"` (Task 2) — ชื่อ row ต้องตรงกันเป๊ะ
- Produces: agent rows `script_basic`, `scene_basic`, `critic_basic`; content_format `basic`; topic_category `beginner`; schedule `produce_basic`

- [ ] **Step 1: เขียน migration**

สร้าง `migrations/071_basic_slot_rows.sql`:

```sql
-- 071: แถวข้อมูลของช่วงคอนเทนต์พื้นฐาน 15:00
-- spec: docs/superpowers/specs/2026-07-28-basic-tutorial-slot-design.md
--
-- agent row ทั้ง 3 ตัวคัดลอกจากฝั่ง tutorial ด้วย INSERT..SELECT เพื่อให้
-- ทุกอย่างที่ไม่ได้ตั้งใจเปลี่ยน (โครงบังคับ 6 ข้อ, กฎ ui_vocab, รูปแบบ JSON)
-- เหมือนกันแน่นอน — แล้วเปลี่ยนเฉพาะบล็อก "เสียงคนวงใน" ของ script_basic
--
-- ห้าม UPDATE row ฝั่ง tutorial เด็ดขาด — 3 ช่วงที่วิ่งอยู่ต้องไม่กระทบ
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
-- Rollback: DELETE FROM agent_configs WHERE agent_name IN ('script_basic','scene_basic','critic_basic');
--           DELETE FROM content_formats WHERE format_name='basic';
--           DELETE FROM topic_categories WHERE category_name='beginner';
--           DELETE FROM schedules WHERE action='produce_basic';
BEGIN;

-- 1) content format (ปิดไว้ เพื่อไม่ให้ FormatsRepo.PickNext ของรอบปกติหยิบไปใช้)
INSERT INTO content_formats (format_name, display_name, script_instruction, enabled, weight)
VALUES ('basic', 'สอนพื้นฐานจากหน้าจอ',
        'เขียนสคริปต์แบบคู่มือสำหรับคนเพิ่งเริ่ม: เปิดด้วยความเสียหาย -> สัญญา+จำนวนขั้น -> เดินขั้นตอนตามข้อมูลฟีเจอร์ -> กับดัก -> สรุป โดยนิยามศัพท์อังกฤษทุกคำที่โผล่ครั้งแรก',
        FALSE, 1)
ON CONFLICT (format_name) DO NOTHING;

-- 2) scene_basic / critic_basic = สำเนาตรงของฝั่ง tutorial
INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, config, skills, prompt_template, insights)
SELECT 'scene_basic', system_prompt, model, temperature, enabled, config, skills, prompt_template, insights
FROM agent_configs WHERE agent_name = 'scene_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, config, skills, prompt_template, insights)
SELECT 'critic_basic', system_prompt, model, temperature, enabled, config, skills, prompt_template, insights
FROM agent_configs WHERE agent_name = 'critic_tutorial'
ON CONFLICT (agent_name) DO NOTHING;

-- 3) script_basic = สำเนาของ script_tutorial ที่สลับบล็อกเสียงเป็นเสียงสำหรับมือใหม่
--    ใช้ DO block เพื่อ "ล้มให้ดัง" ถ้าหาข้อความต้นทางไม่เจอ — replace() ที่ไม่เจอ
--    จะเงียบและได้พรอมป์เสียงคนวงในไปสอนมือใหม่โดยไม่มีใครรู้
DO $mig$
DECLARE
  src TEXT;
  anchor TEXT := 'เสียงคนวงใน: พูดเหมือนคนที่บริหารบัญชีจำนวนมากมาเอง ใช้ศัพท์ที่คนวงในใช้ อ้างสถานการณ์ที่เฉพาะคนยิงหนักเจอ ห้ามเนื้อหาระดับพื้นฐาน';
  replacement TEXT := 'เสียงสำหรับคนเพิ่งเริ่ม: คนดูยังไม่เคยยิงแอดมาก่อน ศัพท์อังกฤษทุกคำที่โผล่ครั้งแรกต้องมีคำอธิบายไทยสั้นๆ ต่อท้ายทันที ห้ามสมมติว่าคนดูรู้อยู่แล้ว แต่ห้ามพูดจาดูถูก (ห้ามใช้ "ง่ายมาก" "ใครๆ ก็ทำได้") ยกตัวอย่างด้วยงบหลักร้อยถึงหลักพันบาทต่อวันแบบคนเพิ่งเริ่มใช้จริง ไม่ใช่หลักหมื่น';
BEGIN
  IF EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'script_basic') THEN
    RETURN; -- idempotent: มีแล้วไม่ต้องทำซ้ำ
  END IF;

  SELECT prompt_template INTO src FROM agent_configs WHERE agent_name = 'script_tutorial';
  IF src IS NULL THEN
    RAISE EXCEPTION 'script_tutorial row missing — cannot derive script_basic';
  END IF;
  IF position(anchor IN src) = 0 THEN
    RAISE EXCEPTION 'voice anchor not found in script_tutorial — update migration 071 before shipping';
  END IF;

  INSERT INTO agent_configs (agent_name, system_prompt, model, temperature, enabled, config, skills, prompt_template, insights)
  SELECT 'script_basic', system_prompt, model, temperature, enabled, config, skills,
         replace(prompt_template, anchor, replacement), insights
  FROM agent_configs WHERE agent_name = 'script_tutorial';
END $mig$;

-- 4) กลุ่มเป้าหมายใหม่ (ปิดไว้ เพื่อไม่ให้รอบ 12:00/18:00 หยิบไปใช้)
INSERT INTO topic_categories (category_name, display_name, angle_instruction, weight, enabled)
VALUES ('beginner', 'คนเพิ่งเริ่มยิงแอด',
$bg$Persona: The First-Timer — คนที่เพิ่งเปิดบัญชีโฆษณา หรือเคยกดโปรโมทโพสต์อย่างเดียว ยังไม่เคยเข้า Ads Manager เต็มรูปแบบ กลัวเสียเงินฟรี ไม่กล้ากดเพราะไม่รู้ว่าปุ่มไหนทำอะไร งบหลักร้อยถึงหลักพันต่อวัน

เมนู pain_point (เลือกค่า pain_point จากนี้เท่านั้น):
- report_columns_meaning: อ่านคอลัมน์ในรายงานไม่ออก ตัวเลขแต่ละตัวแปลว่าอะไร
- campaign_structure_confusion: Campaign กับ Ad set กับ Ad ต่างกันยังไง อะไรอยู่ชั้นไหน
- budget_type_choice: งบรายวันกับงบตลอดอายุแคมเปญ เลือกอันไหน
- objective_mismatch: เลือกวัตถุประสงค์แคมเปญผิด เลยไม่ได้ผลลัพธ์ที่อยากได้
- boost_vs_adsmanager: กดโปรโมทโพสต์กับยิงผ่าน Ads Manager ต่างกันตรงไหน
- ad_not_delivering: ไม่รู้ว่าแอดวิ่งอยู่จริงไหม ดูตรงไหน
- date_range_confusion: เปลี่ยนช่วงวันที่แล้วตัวเลขเปลี่ยน ไม่เข้าใจว่าทำไม
- basic_audience_setup: ตั้งกลุ่มเป้าหมายเบื้องต้นยังไงให้ไม่กว้างเกินไป
- page_account_link: ผูกเพจกับบัญชีโฆษณาไม่เป็น
- first_payment_setup: ตั้งวิธีชำระเงินครั้งแรก
- billing_receipt_reading: ดูใบเสร็จ/ยอดที่ถูกตัดจริงตรงไหน
- breakdown_basics: ดูผลแยกตามอายุ-เพศ ทำยังไง$bg$, 1, FALSE)
ON CONFLICT (category_name) DO NOTHING;

-- 5) ตารางเวลา 15:00 — ปิดไว้ก่อน เปิดหลัง eyeball คลิปแรก
--    GOTCHA: เปิด/ปิดต้อง PATCH ผ่าน API เท่านั้น scheduler อ่าน DB แค่ตอน Start()
INSERT INTO schedules (name, cron_expression, action, enabled)
VALUES ('Basic Produce & Publish', '0 15 * * *', 'produce_basic', FALSE)
ON CONFLICT DO NOTHING;

COMMIT;
```

- [ ] **Step 2: ตรวจว่า `ON CONFLICT` มี unique constraint รองรับจริง**

Run: `grep -n "UNIQUE\|PRIMARY KEY" migrations/*.sql | grep -i "format_name\|agent_name\|category_name"`
Expected: เห็น unique constraint ของทั้ง 3 คอลัมน์
ถ้าคอลัมน์ไหนไม่มี unique constraint ให้เปลี่ยน `ON CONFLICT ... DO NOTHING` ของแถวนั้น
เป็นการหุ้มด้วย `WHERE NOT EXISTS (SELECT 1 FROM <table> WHERE <col> = '<value>')` แทน
(ตาราง `schedules` ใช้ `ON CONFLICT DO NOTHING` แบบไม่ระบุคอลัมน์อยู่แล้วจึงต้องเช็คด้วย
ว่ามันกันซ้ำได้จริง ถ้าไม่ ให้ใช้ `WHERE NOT EXISTS` เช่นกัน)

- [ ] **Step 3: รันเทสต์ทั้งหมด (ไม่ควรมีอะไรพัง — migration ยังไม่ถูกรันในเทสต์)**

Run: `go build ./... && go test -count=1 ./internal/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add migrations/071_basic_slot_rows.sql
git commit -m "feat(basic): แถวข้อมูลช่วง 15:00 — format, agent rows, กลุ่มเป้าหมาย, ตารางเวลา"
```

---

## Task 4: คลังหัวข้อพื้นฐาน 12 เรื่อง

**Files:**
- Create: `migrations/072_basic_catalog_seed.sql`
- Modify: `internal/agent/tutorial_seed_test.go`

**Interfaces:**
- Consumes: คอลัมน์ `level` (Task 1), `pain_point` ของกลุ่ม `beginner` (Task 3)
- Produces: 12 แถวใน `tutorial_features` ที่ `level = 'basic'`, `audience = 'beginner'`

- [ ] **Step 1: เขียน seed migration**

สร้าง `migrations/072_basic_catalog_seed.sql` ตามรูปแบบเดียวกับ `068_tutorial_catalog_seed_v2.sql`
เป๊ะ (คั่น vocab ด้วย `$vocab$…$vocab$` และ steps ด้วย `$steps$…$steps$` เพราะ
`TestSeedStepsCoveredByUIVocab` จับด้วย regex คู่นี้) โดย:

- `audience` = `'beginner'` ทุกแถว · `level` = `'basic'` ทุกแถว
- `pain_point` ต้องเป็นค่าจากเมนูของกลุ่ม `beginner` ใน migration 071 เท่านั้น
- `steps` ต้องมี 3–5 ขั้น และ `ui_target` ทุกตัวต้องอยู่ใน `ui_vocab` ของแถวเดียวกัน
- ปิดท้ายด้วย `ON CONFLICT (feature_key) DO NOTHING;`

12 แถวตามลำดับนี้ (feature_key → หัวข้อ → pain_point):

| feature_key | หัวข้อ | pain_point |
|---|---|---|
| `report_columns_basics` | อ่านคอลัมน์หลักในรายงาน Results / Reach / Impressions / Cost per result | `report_columns_meaning` |
| `breakdown_age_gender` | ดูผลแยกตามอายุและเพศ | `breakdown_basics` |
| `delivery_column_check` | เช็กคอลัมน์ Delivery ว่าแอดวิ่งอยู่จริงไหม | `ad_not_delivering` |
| `date_range_picker` | เลือกช่วงวันที่ให้ถูก ทำไมตัวเลขเปลี่ยนตามวันที่ | `date_range_confusion` |
| `campaign_structure_tour` | Campaign / Ad set / Ad อะไรอยู่ชั้นไหน | `campaign_structure_confusion` |
| `daily_vs_lifetime_budget` | งบรายวัน กับ งบตลอดอายุแคมเปญ | `budget_type_choice` |
| `campaign_objective_pick` | เลือกวัตถุประสงค์แคมเปญให้ตรงกับสิ่งที่อยากได้ | `objective_mismatch` |
| `boost_vs_ads_manager` | กดโปรโมทโพสต์ กับ Ads Manager ต่างกันตรงไหน | `boost_vs_adsmanager` |
| `basic_audience_setup` | ตั้งกลุ่มเป้าหมายเบื้องต้น | `basic_audience_setup` |
| `page_ad_account_link` | ผูกเพจกับบัญชีโฆษณา | `page_account_link` |
| `first_payment_method` | ตั้งวิธีชำระเงินครั้งแรก | `first_payment_setup` |
| `billing_receipt_read` | ดูใบเสร็จและยอดที่ถูกตัดจริง | `billing_receipt_reading` |

**เนื้อหาของ 11 แถวที่เหลือเป็นงานเขียน ไม่ใช่งานคัดลอก** — ผู้ลงมือต้องเขียน
`steps`/`trap_th`/`why_matters_th` ของแต่ละหัวข้อเอง โดยยึดตารางข้างบนเป็นขอบเขต
รูปแบบตามตัวอย่างล่างนี้ และมี `TestSeedStepsCoveredByUIVocab` เป็นตัวตรวจ
เกณฑ์คุณภาพของ `trap_th`: ต้องเป็นจุดที่**คนเพิ่งเริ่มเข้าใจผิดจริง** ไม่ใช่คำเตือนกว้างๆ
(ดูตัวอย่าง: สับสน Reach กับ Impressions)

ตัวอย่างแถวแรกให้เขียนแบบนี้:

```sql
('report_columns_basics', 'อ่านคอลัมน์หลักในรายงาน ว่าตัวเลขแต่ละตัวแปลว่าอะไร', 'ads_manager', 'beginner', 'basic',
 ARRAY['Ads Manager','Campaigns','Columns'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Ads|Columns|Performance|Results|Reach|Impressions|Cost per result|Amount spent|Apply$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้ารายงานของแคมเปญ","action_th":"ใน Ads Manager กดแท็บ Campaigns เพื่อดูรายการแคมเปญทั้งหมด","ui_target":"Campaigns"},
  {"n":2,"title_th":"เลือกชุดคอลัมน์มาตรฐาน","action_th":"กด Columns แล้วเลือก Performance เพื่อให้เห็นตัวเลขชุดพื้นฐาน","ui_target":"Performance"},
  {"n":3,"title_th":"อ่านสามตัวที่ต้องดูก่อนเสมอ","action_th":"ดู Results คือจำนวนผลลัพธ์ที่ได้ Reach คือจำนวนคนที่เห็น Impressions คือจำนวนครั้งที่ถูกแสดง","ui_target":"Results"},
  {"n":4,"title_th":"ดูว่าผลลัพธ์หนึ่งครั้งจ่ายเท่าไหร่","action_th":"ดู Cost per result เทียบกับ Amount spent เพื่อรู้ว่าคุ้มไหม","ui_target":"Cost per result","value_th":"เทียบกับกำไรต่อออเดอร์"}
 ]$steps$,
 'คนใหม่มักดู Impressions แล้วดีใจว่าคนเห็นเยอะ ทั้งที่ Reach คือจำนวนคนจริง Impressions นับซ้ำคนเดิมได้หลายครั้ง',
 'report_columns_meaning',
 'อ่านรายงานไม่ออก แปลว่าเผางบต่อโดยไม่รู้ว่ากำลังได้หรือเสีย'),
```

- [ ] **Step 2: เพิ่มไฟล์ seed ใหม่เข้าตารางเทสต์**

ใน `internal/agent/tutorial_seed_test.go` เพิ่มบรรทัดในตาราง seed:

```go
		{"../../migrations/072_basic_catalog_seed.sql", 12},
```

- [ ] **Step 3: รันเทสต์ seed**

Run: `go test -count=1 -run TestSeedStepsCoveredByUIVocab ./internal/agent/ -v`
Expected: PASS — ถ้า FAIL ให้อ่านข้อความว่า `ui_target` ตัวไหนไม่อยู่ใน `ui_vocab` แล้วแก้แถวนั้น

- [ ] **Step 4: รันทั้งหมด**

Run: `go build ./... && go vet ./internal/... && go test -count=1 ./internal/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add migrations/072_basic_catalog_seed.sql internal/agent/tutorial_seed_test.go
git commit -m "feat(basic): คลังหัวข้อพื้นฐาน 12 เรื่อง"
```

---

## การตรวจของจริงหลังจบทุก task (ผู้คุมงานทำเอง ไม่ใช่ subagent)

1. รัน migration 070–072 ผ่าน runner ตัวจริงบน Neon branch แยก แล้วรันซ้ำเพื่อพิสูจน์ idempotent
2. `SELECT level, count(*) FROM tutorial_features GROUP BY level` → `advanced` 18, `basic` 12
3. ยืนยันว่า `script_basic.prompt_template` **ไม่มี** ข้อความ "ห้ามเนื้อหาระดับพื้นฐาน" แล้ว
   และยัง**มี**กฎ ui_vocab เดิมครบ
4. ยืนยันว่า `script_tutorial` / `scene_tutorial` / `critic_tutorial` ไม่ถูกแก้ (เทียบ `md5(prompt_template)` ก่อน-หลัง)
5. จำลองพื้นแยก level: พักหัวข้อ basic จนชนพื้น แล้วยืนยันว่าคลัง advanced ไม่ถูกกระทบ
6. หลัง deploy: ยิง `POST /api/v1/orchestrator/produce-basic` ด้วยมือ 1 ครั้ง แล้ว eyeball คลิป
   ก่อนเปิด schedule ด้วย PATCH
