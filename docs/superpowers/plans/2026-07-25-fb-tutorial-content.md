# FB Ads Tutorial Content Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ผลิตคลิป tutorial ฟีเจอร์จริงใน Facebook Ads วันละ 1 คลิปตอน 21:00 โดยจำลองหน้าจอ Ads Manager ด้วย HTML/CSS และรับประกันว่าชื่อเมนูทุกคำมาจาก catalog ที่คนคุมเอง

**Architecture:** ยกระดับ `CaseInfo` (env-flag ระดับ process) ให้เป็น `FormatInfo` ที่มี `Mode` ต่อคลิป (`""` / `"case"` / `"tutorial"`) โดย derive จาก `clips.content_format` ที่มีคอลัมน์อยู่แล้ว จากนั้นเพิ่ม catalog `tutorial_features` เป็นแหล่งความจริงของขั้นตอน + `ui_vocab` เพิ่ม layout ใหม่ตัวเดียว `uistep` ในเทมเพลตเดิม และเพิ่ม guard ที่เป็นโค้ด deterministic ตรวจว่าทุกชื่อเมนูบนจอมาจาก catalog

**Tech Stack:** Go 1.22+ · pgx/v5 · Postgres (Neon) · html/template + GSAP 3.14 (hyperframes) · kie.ai (LLM + image) · Railway

## Global Constraints

- **คลิปแฟ้มคดี 12:00 / 18:00 ต้อง byte-identical** — ทุกงานที่แตะ path ร่วมต้องมีเทสยืนยันว่า `Format: "case"` และ `Format: ""` ให้ผลเดิม
- **ห้ามสอนวิธีหลบ detection หรือทำผิดนโยบาย Meta** — สอนเฉพาะฟีเจอร์จริงที่กดได้ (ตรงกับ BOUNDARY ใน `topic_categories.grey-operator`)
- **`renderTemplate` เป็น string-replace ไม่ใช่ text/template** (`internal/agent/template.go:10`) — prompt template ใช้ได้แค่ `{{.FieldName}}` **ห้าม `{{if}}` / `{{range}}`**
- **`RunMigrations` ไม่หุ้ม transaction** (`internal/database/migrations.go:40`) — ไฟล์ migration ทุกไฟล์ต้องเขียน `BEGIN;` … `COMMIT;` เอง
- **ห้ามเขียน `-->` ใน `<script>` ของ Go template** — `html/template` ตัดบรรทัด ทำให้ JS พังทั้งคลิป (บทเรียน blank-video regression)
- **ห้ามใช้ emoji / glyph สัญลักษณ์ (`✓` `◀` `●`) เป็นตัวอักษรใน CSS `content:` หรือใน content** — ฟอนต์ Sarabun/Kanit ที่ bundle ไว้ไม่มี glyph พวกนี้ → tofu box ใช้รูปทรงที่วาดด้วย CSS แทน
- **ทุก filter ที่ drop ตัวเลือกได้ ต้องมี retry + fail-open** ห้ามคืน 0 (บทเรียน `cooldown_deadlock` ที่ทำให้ produce 0 คลิปเงียบๆ 2 รอบ)
- Layout ที่ template รองรับ clamp ที่ `agent.ClampLayout` (`internal/agent/scene_content.go:19`) — layout ที่ไม่รู้จักกลายเป็น `hero`
- คำสั่งเทสมาตรฐาน: `go test ./internal/...` · เทสเฉพาะแพ็กเกจ: `go test ./internal/producer/ -run TestName -v`

---

## File Structure

**สร้างใหม่**

| ไฟล์ | ความรับผิดชอบ |
|---|---|
| `migrations/061_tutorial_catalog.sql` | ตาราง `tutorial_features` + `clips.tutorial_feature` |
| `migrations/062_tutorial_catalog_seed.sql` | seed 8 ฟีเจอร์ตั้งต้น |
| `migrations/063_tutorial_agents.sql` | agent rows `script_tutorial` / `scene_tutorial` / `critic_tutorial` + เกณฑ์ `visual_qa` |
| `migrations/064_tutorial_schedule.sql` | content_formats row + ย้าย schedule 06:00 → 21:00 |
| `internal/models/tutorial.go` | `TutorialFeature`, `TutorialStep` |
| `internal/repository/tutorial_features.go` | repo + picker (pure) |
| `internal/repository/tutorial_features_test.go` | เทส picker |
| `internal/agent/tutorial.go` | `TutorialBrief`, `UIVocabViolations`, `CountUIStepScenes`, `ClampUIState` |
| `internal/agent/tutorial_test.go` | เทส validator |
| `internal/producer/tutorial_format.go` | `TutorialPreset`, mode constants, cover prompt |
| `internal/producer/tutorial_format_test.go` | เทส preset + image policy |
| `internal/producer/scene_adapter_tutorial_test.go` | เทส mapping `uistep` |
| `internal/producer/composition_tutorial_render_test.go` | เทส render `uistep` |
| `internal/orchestrator/tutorial.go` | `ProduceTutorial`, gate, `clipMode` |
| `internal/orchestrator/tutorial_test.go` | เทส `clipMode` + gate + scene shape |

**แก้ไข**

| ไฟล์ | แก้อะไร |
|---|---|
| `internal/producer/case_format.go` | `CaseInfo` → `FormatInfo`, `promptForScene(mode)`, `imageScenesForMode` |
| `internal/producer/producer.go:313,365,538` | เปลี่ยนชนิดพารามิเตอร์เป็น `FormatInfo` |
| `internal/producer/composition_types.go` | เพิ่ม `Panel` / `Callout` / `StepTotal` ใน `SceneContent` |
| `internal/producer/scene_adapter.go` | parse `panel`/`callout`, เติม `StepTotal`, กัน hero-fallback |
| `internal/producer/templates/layout_multi_scene.html.tmpl` | CSS `[data-format='tutorial']` + renderer `uistep` |
| `internal/agent/scene_content.go:12` | เพิ่ม `"uistep"` ใน `sceneLayouts` |
| `internal/agent/scene.go` | `SceneTemplateData.TutorialBrief` + พารามิเตอร์ `Generate` |
| `internal/orchestrator/orchestrator.go` | `modeAgentConfig`, `resolveFormatInfo`, พารามิเตอร์ `feat` |
| `internal/orchestrator/script_debate.go:98` | `generateScript` รับ `brief` |
| `internal/scheduler/scheduler.go:199` | `case "produce_tutorial"` |
| `cmd/` (จุดต่อ repo) | wire `TutorialFeaturesRepo` เข้า orchestrator |

---

## Task 1: `FormatInfo` — ยกระดับ `CaseInfo` เป็นโหมดต่อคลิป

Pure refactor: ไม่มีการเปลี่ยนพฤติกรรมใดๆ ทั้งสิ้น เป้าหมายคือให้ `promptForScene` / image policy / `params.Format` รับ "mode" แทน bool

**Files:**
- Modify: `internal/producer/case_format.go`
- Modify: `internal/producer/producer.go:313-334`, `365-416`, `428-446`, `538-541`
- Modify: `internal/orchestrator/orchestrator.go:609`, `923-941`
- Modify: `internal/producer/case_format_test.go:60-102`
- Modify: `internal/producer/producer_hyperframes_test.go:55`
- Modify: `internal/producer/producer_images_parallel_test.go:33`

**Interfaces:**
- Produces: `producer.ModeClassic = ""`, `producer.ModeCase = "case"`, `producer.ModeTutorial = "tutorial"`;
  `producer.FormatInfo{Mode string; CaseNumber int}` พร้อมเมธอด `IsCase() bool` / `IsTutorial() bool`;
  `producer.imageScenesForMode(scenes []agent.GeneratedScene, mode string) map[int]bool`;
  `producer.promptForScene(s agent.GeneratedScene, preset StylePreset, clipToken, mode string) string`;
  `(*Producer).AssembleHyperframes916(ctx, clipID string, scenes []agent.GeneratedScene, preset StylePreset, fi FormatInfo)`;
  `(*Producer).ProduceHyperframes916(ctx, clipID string, scenes []agent.GeneratedScene, preset StylePreset, fi FormatInfo) (*ProduceResult, error)`

- [ ] **Step 1: เขียนเทสที่ยังไม่ผ่าน**

สร้างไฟล์ `internal/producer/format_info_test.go`:

```go
package producer

import "testing"

func TestFormatInfoModes(t *testing.T) {
	var zero FormatInfo
	if zero.IsCase() || zero.IsTutorial() {
		t.Error("zero FormatInfo must be classic (neither case nor tutorial)")
	}
	if !(FormatInfo{Mode: ModeCase}).IsCase() {
		t.Error("Mode=case must report IsCase")
	}
	if !(FormatInfo{Mode: ModeTutorial}).IsTutorial() {
		t.Error("Mode=tutorial must report IsTutorial")
	}
	if (FormatInfo{Mode: ModeTutorial}).IsCase() {
		t.Error("tutorial must never report IsCase")
	}
}

func TestImageScenesForModeClassicUnrestricted(t *testing.T) {
	scenes := []agent.GeneratedScene{{SceneNumber: 1, Layout: "hero", ImagePrompt: "a graph"}}
	if imageScenesForMode(scenes, ModeClassic) != nil {
		t.Error("classic mode must return nil = no restriction")
	}
}
```

เพิ่ม import `"github.com/jaochai/video-fb/internal/agent"` ในไฟล์เทส

- [ ] **Step 2: รันเทสให้เห็นว่า fail**

Run: `go test ./internal/producer/ -run 'TestFormatInfo|TestImageScenesForMode' -v`
Expected: FAIL — `undefined: FormatInfo`, `undefined: ModeCase`, `undefined: imageScenesForMode`

- [ ] **Step 3: แก้ `internal/producer/case_format.go`**

แทนที่ `type CaseInfo struct {...}` (บรรทัด 62-67) และ `caseImageScenes` (73-88) ด้วย:

```go
// Content mode of a clip. Derived per-clip (from clips.content_format), never
// from a process-wide flag alone — so a tutorial clip and a case clip can be
// produced by the same running server.
const (
	ModeClassic  = ""
	ModeCase     = "case"
	ModeTutorial = "tutorial"
)

// FormatInfo carries the per-clip content mode down the producer path.
// Zero value = classic format (byte-identical to today's output).
type FormatInfo struct {
	Mode       string // "" | "case" | "tutorial"
	CaseNumber int    // case mode only; 0 = unknown, template omits the number
}

func (f FormatInfo) IsCase() bool     { return f.Mode == ModeCase }
func (f FormatInfo) IsTutorial() bool { return f.Mode == ModeTutorial }

// imageScenesForMode returns the scene numbers eligible for AI image generation.
// nil = no restriction (classic). Case format: casefile cover + evidence, cap 2.
// Tutorial format: the first scene carrying an image_prompt only, cap 1 — every
// other tutorial scene is an HTML UI mock and must not compete with a photo.
func imageScenesForMode(scenes []agent.GeneratedScene, mode string) map[int]bool {
	switch mode {
	case ModeCase:
		allowed := map[int]bool{}
		for _, s := range scenes {
			if len(allowed) >= 2 {
				break
			}
			layout := agent.ClampLayout(s.Layout)
			if (layout == "casefile" || layout == "evidence") && strings.TrimSpace(s.ImagePrompt) != "" {
				allowed[s.SceneNumber] = true
			}
		}
		return allowed
	case ModeTutorial:
		for _, s := range scenes {
			if strings.TrimSpace(s.ImagePrompt) != "" {
				return map[int]bool{s.SceneNumber: true}
			}
		}
		return map[int]bool{}
	default:
		return nil
	}
}
```

แก้ `promptForScene` (บรรทัด 52-60) ให้รับ mode:

```go
func promptForScene(s agent.GeneratedScene, preset StylePreset, clipToken, mode string) string {
	switch mode {
	case ModeCase:
		if agent.ClampLayout(s.Layout) == "casefile" {
			return buildCoverPrompt(s.ImagePrompt, preset, clipToken)
		}
		return buildEvidencePrompt(s.ImagePrompt, preset, clipToken)
	case ModeTutorial:
		return buildTutorialCoverPrompt(s.ImagePrompt, preset, clipToken)
	default:
		return buildScenePrompt(s.ImagePrompt, "9:16", preset, clipToken)
	}
}
```

`buildTutorialCoverPrompt` ยังไม่มี — เพิ่ม stub ชั่วคราวใน `case_format.go` (Task 7 จะย้ายไป `tutorial_format.go` พร้อมเนื้อจริง):

```go
// buildTutorialCoverPrompt renders the single cover image of a tutorial clip.
// Replaced with the real art direction in the tutorial-format task.
func buildTutorialCoverPrompt(concept string, preset StylePreset, clipToken string) string {
	return buildScenePrompt(concept, "9:16", preset, clipToken)
}
```

- [ ] **Step 4: อัปเดตจุดเรียกใน `producer.go`**

- `generateSceneImagesParallel` (บรรทัด 313): พารามิเตอร์ `caseInfo CaseInfo` → `fi FormatInfo`; บรรทัด 314 `caseImageScenes(scenes, caseInfo.Enabled)` → `imageScenesForMode(scenes, fi.Mode)`; บรรทัด 330 `promptForScene(s, preset, clipID, caseInfo.Enabled)` → `promptForScene(s, preset, clipID, fi.Mode)`
- `AssembleHyperframes916` (365): `caseInfo CaseInfo` → `fi FormatInfo`; บรรทัด 392, 394, 405 เปลี่ยนตามข้างบน
- บรรทัด 428-431 แทนด้วย:

```go
	params := ScenesParams{
		AspectRatio:     "9:16",
		BrandName:       BrandName,
		CTAText:         BrandCTA,
		VoiceSrc:        "assets/voice.wav",
		DurationSeconds: total,
		Scenes:          specs,
		Segments:        segments,
		Palette:         preset.Palette,
		BrandCSS:        preset.BrandCSS(),
		ThemeKey:        preset.Key,
		Motion:          preset.Motion,
		Format:          fi.Mode,
		CaseNumber:      fi.CaseNumber,
	}
```

(ลบตัวแปร `format := ""` / `if caseInfo.Enabled` ทิ้ง — `fi.Mode` คือค่าเดียวกัน)

- `ProduceHyperframes916` (538, 540): เปลี่ยนชนิดและชื่อพารามิเตอร์เช่นกัน

- [ ] **Step 5: อัปเดต orchestrator**

`internal/orchestrator/orchestrator.go` บรรทัด 919-941 — เปลี่ยนชื่อและชนิด:

```go
// resolveFormatInfo builds the FormatInfo for a clip about to render. Case mode
// resolves/persists the running case number; every error path fails open — the
// clip renders without a number rather than block production.
func (o *Orchestrator) resolveFormatInfo(ctx context.Context, clipID string, preset producer.StylePreset) producer.FormatInfo {
	if preset.Key != producer.CaseFilePreset.Key {
		return producer.FormatInfo{}
	}
	if clip, err := o.clipsRepo.GetByID(ctx, clipID); err == nil &&
		clip.CaseNumber != nil && *clip.CaseNumber > 0 {
		return producer.FormatInfo{Mode: producer.ModeCase, CaseNumber: *clip.CaseNumber}
	}
	n, err := o.clipsRepo.NextCaseNumber(ctx)
	if err != nil {
		log.Printf("case number: next failed (fail-open, clip renders without number): %v", err)
		return producer.FormatInfo{Mode: producer.ModeCase}
	}
	if err := o.clipsRepo.SetCaseNumber(ctx, clipID, n); err != nil {
		log.Printf("case number: set failed (fail-open, clip renders without number): %v", err)
		return producer.FormatInfo{Mode: producer.ModeCase}
	}
	return producer.FormatInfo{Mode: producer.ModeCase, CaseNumber: n}
}
```

บรรทัด 609-610 เปลี่ยนเป็น:

```go
	fi := o.resolveFormatInfo(ctx, clipID, preset)
	result, err := o.producer.ProduceHyperframes916(ctx, clipID, scenes, preset, fi)
```

(Task 9 จะเติมสาขา tutorial ใน `resolveFormatInfo`)

- [ ] **Step 6: อัปเดตเทสเดิมที่อ้าง `CaseInfo`**

- `internal/producer/producer_hyperframes_test.go:55` → `CaseInfo{}` เป็น `FormatInfo{}`
- `internal/producer/producer_images_parallel_test.go:33` → `CaseInfo{}` เป็น `FormatInfo{}`
- `internal/producer/case_format_test.go`:
  - บรรทัด 64 `caseImageScenes(scenes, false)` → `imageScenesForMode(scenes, ModeClassic)`
  - บรรทัด 67 `caseImageScenes(scenes, true)` → `imageScenesForMode(scenes, ModeCase)`
  - บรรทัด 84-86 พารามิเตอร์สุดท้าย `true`/`true`/`false` → `ModeCase`/`ModeCase`/`ModeClassic`
  - บรรทัด 98-102 แทนที่ `TestCaseInfoZeroValueIsClassic` ทั้งฟังก์ชันด้วย:

```go
func TestFormatInfoZeroValueIsClassic(t *testing.T) {
	var fi FormatInfo
	if fi.Mode != ModeClassic {
		t.Error("zero FormatInfo must mean classic format")
	}
}
```

- [ ] **Step 7: รันเทสทั้งหมดให้ผ่าน**

Run: `go build ./... && go test ./internal/...`
Expected: PASS ทั้งหมด — โดยเฉพาะ `TestRenderCaseFormat` และ `TestRenderClassicFormatUnchanged` ต้องยังผ่าน (พิสูจน์ว่าไม่มีการเปลี่ยนพฤติกรรม)

- [ ] **Step 8: Commit**

```bash
git add internal/producer internal/orchestrator
git commit -m "refactor(producer): CaseInfo -> FormatInfo with per-clip mode

เตรียมทางให้โหมด tutorial อยู่ร่วมกับโหมดคดีในเซิร์ฟเวอร์เดียวกัน
ไม่มีการเปลี่ยนพฤติกรรม: case/classic ให้ผลเดิมทุกประการ"
```

---

## Task 2: Catalog schema + model + picker

**Files:**
- Create: `migrations/061_tutorial_catalog.sql`
- Create: `internal/models/tutorial.go`
- Create: `internal/repository/tutorial_features.go`
- Create: `internal/repository/tutorial_features_test.go`

**Interfaces:**
- Consumes: ไม่มี
- Produces: `models.TutorialFeature`, `models.TutorialStep`;
  `repository.NewTutorialFeaturesRepo(pool *pgxpool.Pool) *TutorialFeaturesRepo`;
  `(*TutorialFeaturesRepo).PickNext(ctx context.Context, exclude []string) (*models.TutorialFeature, error)`;
  `(*TutorialFeaturesRepo).GetByKey(ctx context.Context, key string) (*models.TutorialFeature, error)`;
  `(*TutorialFeaturesRepo).MarkUsed(ctx context.Context, id string) error`;
  `(*TutorialFeaturesRepo).MarkNeedsVerify(ctx context.Context, id, reason string) error`

- [ ] **Step 1: เขียนเทส picker ที่ยังไม่ผ่าน**

สร้าง `internal/repository/tutorial_features_test.go`:

```go
package repository

import (
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func tf(key string, weight int) models.TutorialFeature {
	return models.TutorialFeature{FeatureKey: key, Weight: weight, Enabled: true}
}

func TestPickTutorialFeatureLeastUsed(t *testing.T) {
	usages := []tutorialUsage{
		{Feat: tf("a", 1), UsedCount: 3},
		{Feat: tf("b", 1), UsedCount: 1},
		{Feat: tf("c", 1), UsedCount: 5},
	}
	if got := pickTutorialFeatureLeastUsed(usages, nil); got.FeatureKey != "b" {
		t.Errorf("picked %q, want b (lowest used/weight)", got.FeatureKey)
	}
}

func TestPickTutorialFeatureRespectsWeight(t *testing.T) {
	usages := []tutorialUsage{
		{Feat: tf("a", 1), UsedCount: 2}, // ratio 2.0
		{Feat: tf("b", 4), UsedCount: 4}, // ratio 1.0
	}
	if got := pickTutorialFeatureLeastUsed(usages, nil); got.FeatureKey != "b" {
		t.Errorf("picked %q, want b (weight lowers the ratio)", got.FeatureKey)
	}
}

func TestPickTutorialFeatureExcludes(t *testing.T) {
	usages := []tutorialUsage{
		{Feat: tf("a", 1), UsedCount: 0},
		{Feat: tf("b", 1), UsedCount: 9},
	}
	if got := pickTutorialFeatureLeastUsed(usages, []string{"a"}); got.FeatureKey != "b" {
		t.Errorf("picked %q, want b (a is excluded)", got.FeatureKey)
	}
}

// กันบทเรียน cooldown_deadlock: ถ้า exclude กินหมด ต้อง fail-open ไม่ใช่คืนค่าว่าง
func TestPickTutorialFeatureFailsOpenWhenAllExcluded(t *testing.T) {
	usages := []tutorialUsage{
		{Feat: tf("a", 1), UsedCount: 1},
		{Feat: tf("b", 1), UsedCount: 0},
	}
	got := pickTutorialFeatureLeastUsed(usages, []string{"a", "b"})
	if got.FeatureKey == "" {
		t.Fatal("must fail open with a pick, never an empty feature")
	}
}

func TestPickTutorialFeatureEmptyPool(t *testing.T) {
	if got := pickTutorialFeatureLeastUsed(nil, nil); got.FeatureKey != "" {
		t.Errorf("empty pool must return the zero feature, got %q", got.FeatureKey)
	}
}
```

- [ ] **Step 2: รันเทสให้เห็นว่า fail**

Run: `go test ./internal/repository/ -run TestPickTutorialFeature -v`
Expected: FAIL — `undefined: tutorialUsage`, `undefined: pickTutorialFeatureLeastUsed`, `models.TutorialFeature` ไม่มี

- [ ] **Step 3: สร้าง `internal/models/tutorial.go`**

```go
package models

// TutorialStep is one step of a feature walkthrough. n is 1-based and matches
// the "ขั้นที่ n / N" rail rendered on screen.
type TutorialStep struct {
	N        int    `json:"n"`
	TitleTH  string `json:"title_th"`  // ชื่อขั้น เช่น "ตั้งเงื่อนไขให้ตัดงบ"
	ActionTH string `json:"action_th"` // สิ่งที่ต้องกด เช่น "เลือก Cost per result แล้วใส่ 400 บาท"
	UITarget string `json:"ui_target"` // ชื่อเมนู/ปุ่มที่ต้องไฮไลต์ — ต้องอยู่ใน UIVocab
	ValueTH  string `json:"value_th,omitempty"`
}

// TutorialFeature is one row of the tutorial catalog = one clip.
// UIVocab is the ONLY vocabulary allowed to appear on screen inside a UI mock;
// the render gate rejects any label the model invents outside this list.
type TutorialFeature struct {
	ID            string         `json:"id"`
	FeatureKey    string         `json:"feature_key"`
	DisplayNameTH string         `json:"display_name_th"`
	Surface       string         `json:"surface"`
	MenuPath      []string       `json:"menu_path"`
	UIVocab       []string       `json:"ui_vocab"`
	Steps         []TutorialStep `json:"steps"`
	TrapTH        string         `json:"trap_th"`
	PainPoint     string         `json:"pain_point"`
	WhyMattersTH  string         `json:"why_matters_th"`
	NeedsVerify   bool           `json:"needs_verify"`
	Weight        int            `json:"weight"`
	Enabled       bool           `json:"enabled"`
}
```

- [ ] **Step 4: สร้าง `internal/repository/tutorial_features.go`**

```go
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

const tutorialFeatureCols = `id, feature_key, display_name_th, surface, menu_path, ui_vocab,
	steps, trap_th, pain_point, why_matters_th, needs_verify, weight, enabled`

func scanTutorialFeature(scan func(dest ...any) error) (models.TutorialFeature, error) {
	var f models.TutorialFeature
	var stepsRaw []byte
	err := scan(&f.ID, &f.FeatureKey, &f.DisplayNameTH, &f.Surface, &f.MenuPath, &f.UIVocab,
		&stepsRaw, &f.TrapTH, &f.PainPoint, &f.WhyMattersTH, &f.NeedsVerify, &f.Weight, &f.Enabled)
	if err != nil {
		return f, err
	}
	if len(stepsRaw) > 0 {
		if err := json.Unmarshal(stepsRaw, &f.Steps); err != nil {
			return f, fmt.Errorf("unmarshal steps for %s: %w", f.FeatureKey, err)
		}
	}
	return f, nil
}

// GetByKey resolves a feature by its stable key (used by the retry path, which
// only has clips.tutorial_feature to go on).
func (r *TutorialFeaturesRepo) GetByKey(ctx context.Context, key string) (*models.TutorialFeature, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+tutorialFeatureCols+` FROM tutorial_features WHERE feature_key = $1`, key)
	f, err := scanTutorialFeature(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("get tutorial feature %s: %w", key, err)
	}
	return &f, nil
}

// PickNext returns the least-used enabled feature that is not flagged for
// re-verification, skipping any key in exclude. Returns nil when the catalog is
// empty. Never returns an error for "everything excluded" — see
// pickTutorialFeatureLeastUsed, which fails open by design.
func (r *TutorialFeaturesRepo) PickNext(ctx context.Context, exclude []string) (*models.TutorialFeature, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+tutorialFeatureCols+`, used_count
		 FROM tutorial_features
		 WHERE enabled = TRUE AND needs_verify = FALSE
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
			&u.Feat.MenuPath, &u.Feat.UIVocab, &stepsRaw, &u.Feat.TrapTH, &u.Feat.PainPoint,
			&u.Feat.WhyMattersTH, &u.Feat.NeedsVerify, &u.Feat.Weight, &u.Feat.Enabled,
			&u.UsedCount); err != nil {
			return nil, fmt.Errorf("scan tutorial feature usage: %w", err)
		}
		if len(stepsRaw) > 0 {
			if err := json.Unmarshal(stepsRaw, &u.Feat.Steps); err != nil {
				return nil, fmt.Errorf("unmarshal steps for %s: %w", u.Feat.FeatureKey, err)
			}
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
	_, err := r.pool.Exec(ctx,
		`UPDATE tutorial_features SET used_count = used_count + 1, last_used_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark tutorial feature used: %w", err)
	}
	return nil
}

// MarkNeedsVerify parks a feature whose menu path research says has changed, so
// no clip teaches a stale path until a human re-checks and clears the flag.
func (r *TutorialFeaturesRepo) MarkNeedsVerify(ctx context.Context, id, reason string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tutorial_features SET needs_verify = TRUE, verify_reason = $2 WHERE id = $1`, id, reason)
	if err != nil {
		return fmt.Errorf("mark tutorial feature needs_verify: %w", err)
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
```

(`catUsageRatio` มีอยู่แล้วใน `internal/repository/topics.go` — แพ็กเกจเดียวกัน ใช้ซ้ำได้)

- [ ] **Step 5: สร้าง `migrations/061_tutorial_catalog.sql`**

```sql
-- 061: tutorial catalog (spec docs/superpowers/specs/2026-07-25-fb-tutorial-content-design.md)
-- แหล่งความจริงเดียวของขั้นตอน + ชื่อเมนูที่อนุญาตให้ขึ้นจอ
-- Rollback: DROP TABLE tutorial_features; ALTER TABLE clips DROP COLUMN tutorial_feature;
BEGIN;

CREATE TABLE IF NOT EXISTS tutorial_features (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    feature_key      TEXT UNIQUE NOT NULL,
    display_name_th  TEXT NOT NULL,
    surface          TEXT NOT NULL,
    menu_path        TEXT[] NOT NULL DEFAULT '{}',
    ui_vocab         TEXT[] NOT NULL DEFAULT '{}',
    steps            JSONB  NOT NULL DEFAULT '[]',
    trap_th          TEXT   NOT NULL DEFAULT '',
    pain_point       TEXT   NOT NULL DEFAULT '',
    why_matters_th   TEXT   NOT NULL DEFAULT '',
    needs_verify     BOOLEAN NOT NULL DEFAULT FALSE,
    verify_reason    TEXT   NOT NULL DEFAULT '',
    last_verified_at TIMESTAMPTZ,
    used_count       INTEGER NOT NULL DEFAULT 0,
    last_used_at     TIMESTAMPTZ,
    weight           INTEGER NOT NULL DEFAULT 1,
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ผูกคลิปกับฟีเจอร์ เพื่อให้ retry เอา ui_vocab เดิมมา validate ได้ และดู analytics รายฟีเจอร์ได้
ALTER TABLE clips ADD COLUMN IF NOT EXISTS tutorial_feature TEXT NOT NULL DEFAULT '';

COMMIT;
```

- [ ] **Step 6: รันเทสให้ผ่าน**

Run: `go build ./... && go test ./internal/repository/ -run TestPickTutorialFeature -v`
Expected: PASS ทั้ง 5 เทส

- [ ] **Step 7: Commit**

```bash
git add migrations/061_tutorial_catalog.sql internal/models/tutorial.go internal/repository/tutorial_features.go internal/repository/tutorial_features_test.go
git commit -m "feat(tutorial): catalog schema + least-used feature picker"
```

---

## Task 3: Seed catalog 8 ฟีเจอร์ + เทสความสมบูรณ์ของ seed

**ขอบเขตโดยเจตนา:** seed 8 แถว = ผลิตได้ 8 วันโดยไม่ซ้ำ พอสำหรับพิสูจน์ว่าฟอร์แมตเวิร์กก่อนจะลงแรงเขียนอีก 40 แถว ไม่ใช่ placeholder — เป็นการตัดสินใจเรื่องขอบเขต

**Files:**
- Create: `migrations/062_tutorial_catalog_seed.sql`
- Create: `internal/agent/tutorial_seed_test.go`

**Interfaces:**
- Consumes: ตาราง `tutorial_features` จาก Task 2
- Produces: 8 แถวใน catalog ที่ทุก `steps[].ui_target` อยู่ใน `ui_vocab` ของแถวเดียวกัน

- [ ] **Step 1: เขียนเทสความสมบูรณ์ของ seed ที่ยังไม่ผ่าน**

สร้าง `internal/agent/tutorial_seed_test.go` — เทสนี้อ่านไฟล์ migration แล้วตรวจว่าทุก `ui_target` ที่ seed ไว้ครอบคลุมด้วย `ui_vocab` (ถ้าไม่ครอบคลุม gate ตอนรันจริงจะตีคลิปตกทุกใบ):

```go
package agent

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// seedRe จับคู่ (ui_vocab array literal, steps json) ของแต่ละแถวใน migration 062.
var seedRe = regexp.MustCompile(`(?s)\$vocab\$(.*?)\$vocab\$.*?\$steps\$(.*?)\$steps\$`)

func TestSeedStepsCoveredByUIVocab(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/062_tutorial_catalog_seed.sql")
	if err != nil {
		t.Fatalf("read seed migration: %v", err)
	}
	matches := seedRe.FindAllStringSubmatch(string(raw), -1)
	if len(matches) != 8 {
		t.Fatalf("found %d seeded features, want 8", len(matches))
	}
	for i, m := range matches {
		vocab := map[string]bool{}
		for _, v := range strings.Split(m[1], "|") {
			if v = strings.TrimSpace(v); v != "" {
				vocab[strings.ToLower(v)] = true
			}
		}
		var steps []struct {
			N        int    `json:"n"`
			UITarget string `json:"ui_target"`
			TitleTH  string `json:"title_th"`
		}
		if err := json.Unmarshal([]byte(m[2]), &steps); err != nil {
			t.Fatalf("feature %d: steps is not valid JSON: %v", i, err)
		}
		if len(steps) < 3 || len(steps) > 5 {
			t.Errorf("feature %d: %d steps, want 3-5 (see spec §6)", i, len(steps))
		}
		for _, s := range steps {
			if s.TitleTH == "" {
				t.Errorf("feature %d step %d: empty title_th", i, s.N)
			}
			if !vocab[strings.ToLower(strings.TrimSpace(s.UITarget))] {
				t.Errorf("feature %d step %d: ui_target %q missing from ui_vocab", i, s.N, s.UITarget)
			}
		}
	}
}
```

- [ ] **Step 2: รันเทสให้เห็นว่า fail**

Run: `go test ./internal/agent/ -run TestSeedStepsCoveredByUIVocab -v`
Expected: FAIL — `read seed migration: ... no such file or directory`

- [ ] **Step 3: สร้าง `migrations/062_tutorial_catalog_seed.sql`**

`ui_vocab` ใช้ dollar-quote `$vocab$…$vocab$` คั่นด้วย `|` แล้วแปลงเป็น array ด้วย `string_to_array` (รูปแบบนี้ทำให้เทสด้านบนอ่านได้ และ SQL ยังอ่านง่าย)

```sql
-- 062: seed tutorial catalog (8 ฟีเจอร์ตั้งต้น = ผลิตได้ 8 วันโดยไม่ซ้ำ)
-- ทุก steps[].ui_target ต้องอยู่ใน ui_vocab ของแถวเดียวกัน (บังคับด้วย TestSeedStepsCoveredByUIVocab)
-- ⚠ menu_path/ui_vocab ต้องถูกตรวจกับหน้าจอ Ads Manager จริงก่อนเปิด schedule (ดู Task 10)
-- Rollback: DELETE FROM tutorial_features;
BEGIN;

INSERT INTO tutorial_features
    (feature_key, display_name_th, surface, menu_path, ui_vocab, steps, trap_th, pain_point, why_matters_th)
VALUES
('automated_rules_cpm_guard', 'ตั้ง Automated Rules ตัดงบเมื่อ CPM พุ่ง', 'ads_manager',
 ARRAY['Ads Manager','Rules','Create new rule'],
 string_to_array($vocab$Ads Manager|Campaigns|Ad sets|Ads|Rules|Create new rule|Rule name|Apply to|Action|Turn off ad set|Conditions|Cost per result|Greater than|Time range|Schedule|Continuously|Create$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าสร้างกฎ","action_th":"ในเมนูซ้ายของ Ads Manager กด Rules แล้วกด Create new rule","ui_target":"Rules"},
  {"n":2,"title_th":"เลือกสิ่งที่จะให้ระบบปิด","action_th":"ที่ช่อง Action เลือก Turn off ad set","ui_target":"Turn off ad set"},
  {"n":3,"title_th":"ตั้งเงื่อนไขให้ตัดงบ","action_th":"ที่ Conditions เลือก Cost per result / Greater than แล้วใส่ตัวเลขที่รับไม่ได้","ui_target":"Cost per result","value_th":"400 บาท"},
  {"n":4,"title_th":"ให้กฎทำงานตลอดเวลา","action_th":"ที่ Schedule เลือก Continuously แล้วกด Create","ui_target":"Continuously"}
 ]$steps$,
 'คนส่วนใหญ่ตั้ง Time range เป็น Lifetime ทำให้กฎไม่ยิงตอนบัญชีเพิ่งเริ่มพัง — ต้องตั้งช่วงสั้น เช่น วันนี้หรือ 3 วันล่าสุด',
 'scaling_velocity_ceiling',
 'บัญชีโดนปิดตอนตีสามแต่งบยังวิ่ง เพราะไม่มีอะไรกดปิดให้'),

('account_spending_limit', 'ตั้งเพดานงบระดับบัญชี กันงบวิ่งเกินตอนไม่ได้ดู', 'business_settings',
 ARRAY['Billing','Payment settings','Account spending limit'],
 string_to_array($vocab$Billing|Payment settings|Account spending limit|Set limit|Amount|Save|Reset spending limit|Remove limit$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เข้าหน้าตั้งค่าการชำระเงิน","action_th":"จากเมนู Billing กด Payment settings","ui_target":"Payment settings"},
  {"n":2,"title_th":"เปิดเพดานงบของบัญชี","action_th":"หา Account spending limit แล้วกด Set limit","ui_target":"Account spending limit"},
  {"n":3,"title_th":"ใส่เพดานที่ยอมเสียได้จริง","action_th":"ที่ช่อง Amount ใส่ยอดรวมสูงสุดที่ยอมให้บัญชีนี้ใช้ แล้วกด Save","ui_target":"Amount","value_th":"เท่ากับงบ 3 วัน"}
 ]$steps$,
 'เพดานนี้นับสะสมจากยอดใช้จ่ายเดิมของบัญชีด้วย ถ้าตั้งต่ำกว่ายอดที่ใช้ไปแล้ว แอดจะหยุดทันที ต้องกด Reset spending limit ก่อน',
 'account_burn_economics',
 'บัญชีเดียวเผางบเกินแผนได้ในคืนเดียวถ้าไม่มีเพดานกั้น'),

('backup_payment_method', 'ผูกบัตรสำรอง กันแอดหยุดเพราะตัดเงินไม่ผ่าน', 'business_settings',
 ARRAY['Billing','Payment settings','Add payment method'],
 string_to_array($vocab$Billing|Payment settings|Add payment method|Credit or debit card|Card number|Save|Set as primary|Backup payment method$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เข้าหน้าวิธีชำระเงิน","action_th":"จากเมนู Billing กด Payment settings","ui_target":"Payment settings"},
  {"n":2,"title_th":"เพิ่มบัตรใบที่สอง","action_th":"กด Add payment method แล้วเลือก Credit or debit card","ui_target":"Add payment method"},
  {"n":3,"title_th":"ตั้งให้เป็นบัตรสำรอง","action_th":"บันทึกบัตรแล้วปล่อยไว้เป็น Backup payment method ไม่ต้องกด Set as primary","ui_target":"Backup payment method"}
 ]$steps$,
 'ถ้าใช้บัตรชื่อเดียวกันธนาคารเดียวกันกับใบหลัก พอโดนบล็อกจะโดนพร้อมกันทั้งคู่ — บัตรสำรองต้องคนละธนาคาร',
 'payment_method_flag',
 'ตัดเงินไม่ผ่านรอบเดียว บัญชีหยุดวิ่งทั้งคืน'),

('export_custom_audience', 'สำรอง Custom Audience ก่อนบัญชีตาย', 'ads_manager',
 ARRAY['Audiences','Share audience'],
 string_to_array($vocab$Audiences|Custom Audiences|Share audience|Share to|Business|Audience name|Share|Permissions$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดรายการกลุ่มเป้าหมาย","action_th":"จากเมนู Ads Manager เข้า Audiences แล้วดูที่ Custom Audiences","ui_target":"Custom Audiences"},
  {"n":2,"title_th":"เลือกกลุ่มที่ต้องเก็บไว้","action_th":"ติ๊กกลุ่มที่สร้างจากลูกค้าจริง แล้วกด Share audience","ui_target":"Share audience"},
  {"n":3,"title_th":"แชร์ไปยัง BM สำรอง","action_th":"ที่ Share to เลือก Business ปลายทางที่แยกไว้ แล้วกด Share","ui_target":"Share to"}
 ]$steps$,
 'แชร์ audience ไม่ได้ก๊อปข้อมูล ถ้า BM ต้นทางโดนปิด audience ที่แชร์ไปก็หายด้วย — ต้องแชร์จาก BM ที่สะอาดที่สุดเป็นต้นทาง',
 'asset_backup_strategy',
 'บัญชีตายพร้อมกลุ่มลูกค้าที่สะสมมาเป็นปี'),

('domain_verification', 'ยืนยันโดเมน กันคนอื่นแย่งสิทธิ์และลด flag ที่ landing page', 'business_settings',
 ARRAY['Business settings','Brand safety','Domains'],
 string_to_array($vocab$Business settings|Brand safety|Domains|Add|Domain name|DNS Verification|Meta-tag Verification|HTML File Upload|Verify domain|Verified$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าโดเมน","action_th":"ใน Business settings เข้า Brand safety แล้วกด Domains","ui_target":"Domains"},
  {"n":2,"title_th":"เพิ่มโดเมนที่ใช้ยิงจริง","action_th":"กด Add แล้วพิมพ์โดเมนที่ช่อง Domain name","ui_target":"Domain name"},
  {"n":3,"title_th":"เลือกวิธียืนยันที่ทำได้เร็วสุด","action_th":"เลือก DNS Verification แล้วเอา TXT record ไปใส่ที่ผู้ให้บริการโดเมน","ui_target":"DNS Verification"},
  {"n":4,"title_th":"กดยืนยันให้ขึ้น Verified","action_th":"กลับมากด Verify domain แล้วรอสถานะเปลี่ยนเป็น Verified","ui_target":"Verify domain"}
 ]$steps$,
 'ต้องยืนยันโดเมนหลักแบบไม่มี www และไม่มี path ถ้าใส่ทั้ง URL ระบบจะไม่ผ่านและวนอยู่แบบนั้น',
 'landing_page_flag',
 'โดเมนไม่ได้ยืนยัน = ใครก็อ้างสิทธิ์ได้ และ ad ผ่านแต่ landing โดนตี'),

('backup_admin_2fa', 'ตั้งแอดมินสำรอง + บังคับ 2FA กันโดนล็อกออกทั้งทีม', 'business_settings',
 ARRAY['Business settings','People','Add people'],
 string_to_array($vocab$Business settings|People|Add people|Email address|Admin access|Assign assets|Security Center|Two-factor authentication|Required for everyone|Save changes$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เพิ่มคนที่สองเป็นแอดมิน","action_th":"ใน Business settings เข้า People กด Add people ใส่ Email address แล้วเลือก Admin access","ui_target":"Admin access"},
  {"n":2,"title_th":"ให้สิทธิ์เข้าถึง asset","action_th":"กด Assign assets แล้วเลือกบัญชีโฆษณาและเพจที่ต้องเข้าถึงได้","ui_target":"Assign assets"},
  {"n":3,"title_th":"บังคับ 2FA ทั้งทีม","action_th":"เข้า Security Center ที่ Two-factor authentication เลือก Required for everyone แล้วกด Save changes","ui_target":"Two-factor authentication"}
 ]$steps$,
 'ถ้าแอดมินสำรองใช้เบอร์และอีเมลชุดเดียวกับคนแรก พอโดน checkpoint จะเข้าไม่ได้ทั้งคู่ — ต้องคนละเบอร์คนละอีเมล',
 'checkpoint_lock_2fa',
 'แอดมินคนเดียวติด checkpoint = ทั้งพอร์ตเข้าไม่ได้'),

('account_quality_check', 'อ่าน Account Quality ให้ออกว่าโดนอะไรและอุทธรณ์ตรงไหน', 'account_quality',
 ARRAY['Account Quality','Account status'],
 string_to_array($vocab$Account Quality|Account status|Issues|See details|Policy|Request review|Appeal|Restricted|Ad accounts|Pages$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เปิดหน้าสถานะบัญชี","action_th":"เข้า Account Quality แล้วดูที่ Account status","ui_target":"Account status"},
  {"n":2,"title_th":"หาสาเหตุจริงไม่ใช่ข้อความสรุป","action_th":"ที่รายการใน Issues กด See details เพื่อดูว่าชนนโยบายข้อไหน","ui_target":"See details"},
  {"n":3,"title_th":"ยื่นอุทธรณ์จากหน้านี้เท่านั้น","action_th":"กด Request review ในรายการนั้นโดยตรง อย่าไปยื่นจากหน้าอื่น","ui_target":"Request review"}
 ]$steps$,
 'ปุ่ม Request review เป็นสีเทาเมื่อมียอดค้างชำระ ต้องเคลียร์ยอดก่อนถึงจะกดได้ ไม่ใช่ระบบพัง',
 'appeal_bot_rejection',
 'อุทธรณ์ผิดที่ผิดเวลา = โดนปัดตกใน 10 นาทีโดยไม่มีคนอ่าน'),

('audience_overlap_check', 'เช็ก Audience Overlap กันแอดตัวเองแย่ง auction กันเอง', 'ads_manager',
 ARRAY['Audiences','Show audience overlap'],
 string_to_array($vocab$Audiences|Custom Audiences|Show audience overlap|Compare|Overlap|Selected audience|Ad sets$vocab$, '|'),
 $steps$[
  {"n":1,"title_th":"เลือกกลุ่มตั้งต้น","action_th":"ใน Audiences ติ๊กกลุ่มที่ใช้เป็นตัวหลักที่ Custom Audiences","ui_target":"Custom Audiences"},
  {"n":2,"title_th":"เปิดเครื่องมือเทียบ","action_th":"กด Show audience overlap แล้วเลือกกลุ่มอื่นที่ใช้ยิงพร้อมกันมา Compare","ui_target":"Show audience overlap"},
  {"n":3,"title_th":"อ่านตัวเลขให้เป็น","action_th":"ดูค่า Overlap ถ้าเกิน 30 เปอร์เซ็นต์ ให้ยุบเหลือ ad set เดียว","ui_target":"Overlap","value_th":"เกิน 30%"}
 ]$steps$,
 'เครื่องมือนี้เทียบได้เฉพาะ Custom Audience ไม่ครอบคลุมกลุ่ม interest ที่ซ้อนกัน — ซ้อนกันจริงอาจมากกว่าที่เห็น',
 'audience_overlap_self_bid',
 'ยิงหลาย ad set ทับกลุ่มเดียวกัน = จ่าย CPM แพงขึ้นเพราะสู้กับตัวเอง')
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
```

- [ ] **Step 4: รันเทสให้ผ่าน**

Run: `go test ./internal/agent/ -run TestSeedStepsCoveredByUIVocab -v`
Expected: PASS — 8 แถว ทุก `ui_target` ครอบคลุมด้วย `ui_vocab`

- [ ] **Step 5: ตรวจว่า SQL รันได้จริง**

Run: `psql "$DATABASE_URL" -f migrations/061_tutorial_catalog.sql -f migrations/062_tutorial_catalog_seed.sql`
(หรือถ้าไม่มี psql ในเครื่อง ให้รันผ่าน Neon MCP `run_sql` ทีละไฟล์บน **branch ทดสอบ ไม่ใช่ default branch**)
Expected: `INSERT 0 8`

- [ ] **Step 6: Commit**

```bash
git add migrations/062_tutorial_catalog_seed.sql internal/agent/tutorial_seed_test.go
git commit -m "feat(tutorial): seed 8 catalog features + ui_vocab coverage test"
```

---

## Task 4: agent — brief builder + validators + ลงทะเบียน layout `uistep`

**Files:**
- Create: `internal/agent/tutorial.go`
- Create: `internal/agent/tutorial_test.go`
- Modify: `internal/agent/scene_content.go:12-16`

**Interfaces:**
- Consumes: `models.TutorialFeature` (Task 2)
- Produces: `agent.TutorialBrief(f *models.TutorialFeature) string`;
  `agent.UIVocabViolations(scenes []GeneratedScene, vocab []string) []string`;
  `agent.CountUIStepScenes(scenes []GeneratedScene) int`;
  `agent.ClampUIState(s string) string`;
  layout `"uistep"` ผ่าน `ClampLayout`

- [ ] **Step 1: เขียนเทสที่ยังไม่ผ่าน**

สร้าง `internal/agent/tutorial_test.go`:

```go
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func uistepScene(n int, panel string) GeneratedScene {
	return GeneratedScene{SceneNumber: n, Layout: "uistep", Content: json.RawMessage(panel)}
}

func TestUIStepIsASupportedLayout(t *testing.T) {
	if ClampLayout("uistep") != "uistep" {
		t.Error("uistep must survive ClampLayout")
	}
	if ClampLayout("ui_step") != "hero" {
		t.Error("unknown layouts must still clamp to hero")
	}
}

func TestClampUIState(t *testing.T) {
	for in, want := range map[string]string{
		"target": "target", "done": "done", "normal": "normal",
		"": "normal", "TARGET": "normal", "highlighted": "normal",
	} {
		if got := ClampUIState(in); got != want {
			t.Errorf("ClampUIState(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUIVocabViolationsAcceptsCatalogLabels(t *testing.T) {
	scenes := []GeneratedScene{uistepScene(1, `{"panel":{"chrome":"Ads Manager",
		"breadcrumb":"Rules › Create new rule",
		"items":[{"label":"Campaigns","state":"normal"},{"label":"Rules","state":"target"}],
		"field":{"label":"Cost per result","value":"400 THB"}}}`)}
	vocab := []string{"Ads Manager", "Rules", "Create new rule", "Campaigns", "Cost per result"}
	if v := UIVocabViolations(scenes, vocab); len(v) != 0 {
		t.Errorf("expected no violations, got %v", v)
	}
}

func TestUIVocabViolationsCatchesInventedLabel(t *testing.T) {
	scenes := []GeneratedScene{uistepScene(1, `{"panel":{"chrome":"Ads Manager",
		"items":[{"label":"Advanced Rules Manager","state":"target"}]}}`)}
	v := UIVocabViolations(scenes, []string{"Ads Manager", "Rules"})
	if len(v) != 1 || !strings.Contains(v[0], "Advanced Rules Manager") {
		t.Errorf("expected the invented label to be reported, got %v", v)
	}
}

// ชื่อเมนูต่างกันแค่ตัวพิมพ์/ช่องว่าง ไม่ใช่การแต่งเมนูใหม่ — ต้องผ่าน
func TestUIVocabViolationsNormalizesCaseAndSpace(t *testing.T) {
	scenes := []GeneratedScene{uistepScene(1, `{"panel":{"items":[{"label":"  cost  per result "}]}}`)}
	if v := UIVocabViolations(scenes, []string{"Cost per result"}); len(v) != 0 {
		t.Errorf("normalized match must pass, got %v", v)
	}
}

// ซีนที่ไม่ใช่ uistep ไม่ถูกตรวจ (ข้อความไทยเขียนอิสระได้)
func TestUIVocabViolationsIgnoresNonUIStepScenes(t *testing.T) {
	scenes := []GeneratedScene{{SceneNumber: 1, Layout: "hero",
		Content: json.RawMessage(`{"title":"อะไรก็เขียนได้"}`)}}
	if v := UIVocabViolations(scenes, []string{"Rules"}); len(v) != 0 {
		t.Errorf("non-uistep scenes must not be validated, got %v", v)
	}
}

func TestUIVocabViolationsMalformedContentIsAViolation(t *testing.T) {
	scenes := []GeneratedScene{uistepScene(1, `{"panel": broken`)}
	if v := UIVocabViolations(scenes, []string{"Rules"}); len(v) == 0 {
		t.Error("malformed uistep content must be reported, not silently accepted")
	}
}

func TestCountUIStepScenes(t *testing.T) {
	scenes := []GeneratedScene{
		{Layout: "hook"}, uistepScene(2, `{}`), {Layout: "tip"}, uistepScene(4, `{}`), {Layout: "cta"},
	}
	if got := CountUIStepScenes(scenes); got != 2 {
		t.Errorf("CountUIStepScenes = %d, want 2", got)
	}
}

func TestTutorialBriefContainsEverythingTheModelNeeds(t *testing.T) {
	f := &models.TutorialFeature{
		DisplayNameTH: "ตั้ง Automated Rules",
		MenuPath:      []string{"Ads Manager", "Rules"},
		UIVocab:       []string{"Ads Manager", "Rules", "Cost per result"},
		Steps: []models.TutorialStep{
			{N: 1, TitleTH: "เปิดหน้าสร้างกฎ", ActionTH: "กด Rules", UITarget: "Rules"},
		},
		TrapTH:       "อย่าตั้ง Lifetime",
		WhyMattersTH: "งบวิ่งตอนบัญชีพัง",
	}
	b := TutorialBrief(f)
	for _, want := range []string{
		"ตั้ง Automated Rules", "Ads Manager", "Cost per result",
		"เปิดหน้าสร้างกฎ", "อย่าตั้ง Lifetime", "งบวิ่งตอนบัญชีพัง",
	} {
		if !strings.Contains(b, want) {
			t.Errorf("brief missing %q", want)
		}
	}
}

func TestTutorialBriefNilIsEmpty(t *testing.T) {
	if TutorialBrief(nil) != "" {
		t.Error("nil feature must render an empty brief (non-tutorial modes)")
	}
}
```

- [ ] **Step 2: รันเทสให้เห็นว่า fail**

Run: `go test ./internal/agent/ -run 'TestUIStep|TestClampUIState|TestUIVocab|TestCountUIStep|TestTutorialBrief' -v`
Expected: FAIL — `undefined: ClampUIState`, `undefined: UIVocabViolations`, `undefined: TutorialBrief`

- [ ] **Step 3: เพิ่ม `uistep` ใน `internal/agent/scene_content.go`**

แทนบรรทัด 12-16:

```go
var sceneLayouts = map[string]bool{
	"hook": true, "hero": true, "stat": true, "step": true, "tip": true, "cta": true,
	// case-file format (spec 2026-07-24): investigation storytelling layouts
	"casefile": true, "comic": true, "evidence": true, "board": true, "verdict": true,
	// tutorial format (spec 2026-07-25): simulated Ads Manager UI walkthrough
	"uistep": true,
}
```

- [ ] **Step 4: สร้าง `internal/agent/tutorial.go`**

```go
package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)

// UIPanelItem is one row inside the simulated Ads Manager panel of a uistep scene.
type UIPanelItem struct {
	Label string `json:"label"`
	State string `json:"state"` // normal|target|done
}

// UIPanelField is the highlighted input of a uistep scene ("Greater than: 400 THB").
type UIPanelField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// UIPanel is the simulated screen of a uistep scene. Every English string in it
// must come from the catalog's ui_vocab — see UIVocabViolations.
type UIPanel struct {
	Chrome     string        `json:"chrome"`
	Breadcrumb string        `json:"breadcrumb"`
	Items      []UIPanelItem `json:"items"`
	Field      *UIPanelField `json:"field,omitempty"`
}

// uistepContent is the parse target for a uistep scene's content object.
type uistepContent struct {
	Panel   *UIPanel `json:"panel"`
	Callout string   `json:"callout"`
	Title   string   `json:"title"`
	Num     string   `json:"num"`
	Of      string   `json:"of"`
}

// ClampUIState clamps an LLM panel-item state to the three the template styles;
// anything else (including "") becomes "normal".
func ClampUIState(s string) string {
	switch s {
	case "target", "done", "normal":
		return s
	default:
		return "normal"
	}
}

// normalizeUILabel makes vocabulary comparison forgiving about case and spacing
// but nothing else — "cost  per result" matches "Cost per result", while
// "Advanced Rules Manager" never matches "Rules".
func normalizeUILabel(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// breadcrumbParts splits "Rules › Create new rule" into its individual labels.
// Both the typographic and the ASCII separator are accepted.
func breadcrumbParts(s string) []string {
	f := strings.FieldsFunc(s, func(r rune) bool { return r == '›' || r == '>' || r == '/' })
	out := make([]string, 0, len(f))
	for _, p := range f {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// UIVocabViolations returns a human-readable description of every UI string in
// the scenes' uistep panels that is NOT in vocab. An empty result means every
// menu name on screen came from the catalog.
//
// This is the deterministic half of the tutorial gate: it does not ask an LLM
// whether a menu exists, it checks against the catalog a human curated. A
// uistep scene whose content JSON does not parse is itself a violation — a
// silently-dropped panel would ship a tutorial with a blank step.
func UIVocabViolations(scenes []GeneratedScene, vocab []string) []string {
	allowed := make(map[string]bool, len(vocab))
	for _, v := range vocab {
		if n := normalizeUILabel(v); n != "" {
			allowed[n] = true
		}
	}

	var out []string
	check := func(sceneNo int, field, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		if !allowed[normalizeUILabel(value)] {
			out = append(out, fmt.Sprintf("scene %d %s: %q not in ui_vocab", sceneNo, field, value))
		}
	}

	for _, s := range scenes {
		if ClampLayout(s.Layout) != "uistep" {
			continue
		}
		var c uistepContent
		if err := json.Unmarshal(s.Content, &c); err != nil {
			out = append(out, fmt.Sprintf("scene %d: uistep content is not valid JSON: %v", s.SceneNumber, err))
			continue
		}
		if c.Panel == nil {
			out = append(out, fmt.Sprintf("scene %d: uistep has no panel object", s.SceneNumber))
			continue
		}
		check(s.SceneNumber, "chrome", c.Panel.Chrome)
		for _, p := range breadcrumbParts(c.Panel.Breadcrumb) {
			check(s.SceneNumber, "breadcrumb", p)
		}
		for _, it := range c.Panel.Items {
			check(s.SceneNumber, "item", it.Label)
		}
		if c.Panel.Field != nil {
			check(s.SceneNumber, "field", c.Panel.Field.Label)
		}
	}
	return out
}

// CountUIStepScenes reports how many scenes render the uistep layout. The
// tutorial gate requires this to equal the catalog's step count, so a clip can
// never silently drop or invent a step.
func CountUIStepScenes(scenes []GeneratedScene) int {
	n := 0
	for _, s := range scenes {
		if ClampLayout(s.Layout) == "uistep" {
			n++
		}
	}
	return n
}

// TutorialBrief renders the catalog row as the Thai prompt block the script and
// scene agents read. Returns "" for a nil feature so non-tutorial modes render
// an empty {{.TutorialBrief}} substitution.
func TutorialBrief(f *models.TutorialFeature) string {
	if f == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## ข้อมูลฟีเจอร์ที่ต้องสอน (ใช้ได้เฉพาะข้อมูลในบล็อกนี้ ห้ามเพิ่มชื่อเมนูเอง)\n")
	b.WriteString("ชื่อฟีเจอร์: " + f.DisplayNameTH + "\n")
	b.WriteString("ความเสียหายที่ฟีเจอร์นี้กันได้ (ใช้เป็นวัตถุดิบของ hook): " + f.WhyMattersTH + "\n")
	b.WriteString("เส้นทางเมนูจริง: " + strings.Join(f.MenuPath, " › ") + "\n")
	b.WriteString(fmt.Sprintf("จำนวนขั้นตอน: %d (ต้องมีซีน layout \"uistep\" เท่ากับจำนวนนี้พอดี)\n", len(f.Steps)))
	b.WriteString("\nขั้นตอน:\n")
	for _, s := range f.Steps {
		line := fmt.Sprintf("%d) %s — %s [ไฮไลต์: %s]", s.N, s.TitleTH, s.ActionTH, s.UITarget)
		if s.ValueTH != "" {
			line += " [ค่าที่ใส่: " + s.ValueTH + "]"
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\nกับดักที่คนพลาด (ใช้เป็นซีน re-hook กลางคลิป): " + f.TrapTH + "\n")
	b.WriteString("\nคำศัพท์ UI ที่อนุญาตให้ปรากฏบนจอได้เท่านั้น (ห้ามใช้คำอื่นเด็ดขาด):\n- ")
	b.WriteString(strings.Join(f.UIVocab, "\n- "))
	b.WriteString("\n")
	return b.String()
}
```

- [ ] **Step 5: รันเทสให้ผ่าน**

Run: `go test ./internal/agent/ -v`
Expected: PASS ทั้งหมด (รวมเทสเดิมของ agent)

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tutorial.go internal/agent/tutorial_test.go internal/agent/scene_content.go
git commit -m "feat(tutorial): ui_vocab validator + brief builder + uistep layout"
```

---

## Task 5: `SceneContent` + scene_adapter mapping สำหรับ `uistep`

**Files:**
- Modify: `internal/producer/composition_types.go:40-90`
- Modify: `internal/producer/scene_adapter.go:48-87`, `126-208`
- Create: `internal/producer/scene_adapter_tutorial_test.go`

**Interfaces:**
- Consumes: `agent.ClampUIState` (Task 4)
- Produces: `producer.ContentUIPanel`, `producer.ContentUIItem`, `producer.ContentUIField`;
  `SceneContent.Panel *ContentUIPanel` (JSON `panel`), `SceneContent.Callout string` (JSON `callout`),
  `SceneContent.StepTotal int` (JSON `stepTotal`)

- [ ] **Step 1: เขียนเทสที่ยังไม่ผ่าน**

สร้าง `internal/producer/scene_adapter_tutorial_test.go`:

```go
package producer

import (
	"encoding/json"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func uistepScene(n int, content string) agent.GeneratedScene {
	return agent.GeneratedScene{SceneNumber: n, Layout: "uistep", OnScreenText: "fallback",
		Content: json.RawMessage(content)}
}

func TestBuildSceneContentUIStep(t *testing.T) {
	s := uistepScene(3, `{"num":"2","of":"ขั้นที่ 2 / 3","title":"ตั้งเงื่อนไขให้ตัดงบ",
		"panel":{"chrome":"Ads Manager","breadcrumb":"Rules › Create new rule",
			"items":[{"label":"Campaigns","state":"normal"},{"label":"Ad sets","state":"done"},
			         {"label":"Cost per result","state":"target"}],
			"field":{"label":"Greater than","value":"400 THB"}},
		"callout":"เลือก Cost per result แล้วใส่ 400 บาท"}`)
	c := buildSceneContent(s, sceneBound{Start: 9, End: 17})

	if c.Layout != "uistep" {
		t.Fatalf("layout = %q, want uistep (must NOT fall back to hero)", c.Layout)
	}
	if c.Panel == nil {
		t.Fatal("panel not parsed")
	}
	if c.Panel.Chrome != "Ads Manager" || c.Panel.Breadcrumb != "Rules › Create new rule" {
		t.Errorf("panel chrome/breadcrumb wrong: %+v", c.Panel)
	}
	if len(c.Panel.Items) != 3 || c.Panel.Items[1].State != "done" || c.Panel.Items[2].State != "target" {
		t.Errorf("panel items wrong: %+v", c.Panel.Items)
	}
	if c.Panel.Field == nil || c.Panel.Field.Value != "400 THB" {
		t.Errorf("panel field wrong: %+v", c.Panel.Field)
	}
	if c.Callout != "เลือก Cost per result แล้วใส่ 400 บาท" || c.Num != "2" || c.Of != "ขั้นที่ 2 / 3" {
		t.Errorf("rail/callout wrong: num=%q of=%q callout=%q", c.Num, c.Of, c.Callout)
	}
}

func TestBuildSceneContentUIStepClampsState(t *testing.T) {
	s := uistepScene(3, `{"panel":{"items":[{"label":"Rules","state":"highlighted"}]}}`)
	c := buildSceneContent(s, sceneBound{Start: 0, End: 4})
	if c.Panel.Items[0].State != "normal" {
		t.Errorf("unknown state = %q, want normal", c.Panel.Items[0].State)
	}
}

func TestBuildSceneContentUIStepCapsItemsAndLengths(t *testing.T) {
	s := uistepScene(3, `{"title":"ชื่อขั้นที่ยาวเกินสามสิบสี่ตัวอักษรแน่นอนเพราะพิมพ์ยาวมากจริงๆนะ",
		"callout":"คำอธิบายที่ยาวมากเกินหกสิบตัวอักษรแน่นอนเพราะเขียนบรรยายยืดยาวเกินกรอบที่ออกแบบไว้จริงๆ",
		"panel":{"items":[{"label":"a"},{"label":"b"},{"label":"c"},{"label":"d"},{"label":"e"},{"label":"f"},{"label":"g"}]}}`)
	c := buildSceneContent(s, sceneBound{Start: 0, End: 4})
	if len(c.Panel.Items) != 5 {
		t.Errorf("items = %d, want capped at 5", len(c.Panel.Items))
	}
	if n := len([]rune(c.Title)); n > 34 {
		t.Errorf("title = %d runes, want <= 34", n)
	}
	if n := len([]rune(c.Callout)); n > 60 {
		t.Errorf("callout = %d runes, want <= 60", n)
	}
}

func TestBuildSceneSpecsFillsStepTotal(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "hook", Content: json.RawMessage(`{"rows":[{"t":"x"}]}`)},
		uistepScene(2, `{"num":"1","panel":{"items":[{"label":"Rules"}]}}`),
		uistepScene(3, `{"num":"2","panel":{"items":[{"label":"Rules"}]}}`),
		uistepScene(4, `{"num":"3","panel":{"items":[{"label":"Rules"}]}}`),
		{SceneNumber: 5, Layout: "cta", Content: json.RawMessage(`{"title":"จบ","cta":"ทักมา","brand":"ADS VANCE"}`)},
	}
	bounds := []sceneBound{{0, 3}, {3, 8}, {8, 13}, {13, 18}, {18, 24}}
	specs := buildSceneSpecs(scenes, bounds)
	for _, sp := range specs {
		if sp.Content.Layout == "uistep" && sp.Content.StepTotal != 3 {
			t.Errorf("scene %d StepTotal = %d, want 3", sp.SceneNumber, sp.Content.StepTotal)
		}
		if sp.Content.Layout != "uistep" && sp.Content.StepTotal != 0 {
			t.Errorf("non-uistep scene %d must not carry StepTotal", sp.SceneNumber)
		}
	}
}
```

- [ ] **Step 2: รันเทสให้เห็นว่า fail**

Run: `go test ./internal/producer/ -run 'UIStep|StepTotal' -v`
Expected: FAIL — `c.Panel undefined`, `c.Callout undefined`, `StepTotal undefined`

- [ ] **Step 3: เพิ่ม field ใน `internal/producer/composition_types.go`**

เพิ่มต่อท้าย block case-file ใน `SceneContent` (หลังบรรทัด 68 `Hook string ...`):

```go
	// tutorial format (spec 2026-07-25). Panel is the simulated Ads Manager
	// screen of a uistep scene; StepTotal is Go-derived (never LLM) so the
	// progress rail can draw N dots without parsing the Thai "ขั้นที่ n / N".
	Panel     *ContentUIPanel `json:"panel,omitempty"`
	Callout   string          `json:"callout,omitempty"`
	StepTotal int             `json:"stepTotal,omitempty"`
```

และเพิ่มชนิดใหม่ท้ายไฟล์ (หลัง `ContentPanel`):

```go
// ContentUIPanel is the simulated product screen rendered inside a uistep scene.
type ContentUIPanel struct {
	Chrome     string           `json:"chrome,omitempty"`
	Breadcrumb string           `json:"breadcrumb,omitempty"`
	Items      []ContentUIItem  `json:"items,omitempty"`
	Field      *ContentUIField  `json:"field,omitempty"`
}

// ContentUIItem is one menu row inside a ContentUIPanel.
// State is one of normal|target|done (clamped by agent.ClampUIState).
type ContentUIItem struct {
	Label string `json:"label"`
	State string `json:"state"`
}

// ContentUIField is the highlighted input row of a ContentUIPanel.
type ContentUIField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
```

- [ ] **Step 4: แก้ `internal/producer/scene_adapter.go`**

(4a) เพิ่มใน struct `raw` ของ `buildSceneContent` (หลัง `Panels []struct{...}` บรรทัด 146-150):

```go
		Callout string `json:"callout"`
		Panel   *struct {
			Chrome     string `json:"chrome"`
			Breadcrumb string `json:"breadcrumb"`
			Items      []struct {
				Label string `json:"label"`
				State string `json:"state"`
			} `json:"items"`
			Field *struct {
				Label string `json:"label"`
				Value string `json:"value"`
			} `json:"field"`
		} `json:"panel"`
```

(4b) เพิ่มการ map ก่อนบรรทัด `for _, pn := range raw.Panels {` (บรรทัด 182):

```go
	// tutorial uistep: the simulated screen. Item labels are NOT truncated here —
	// they must stay byte-comparable against the catalog's ui_vocab, which the
	// render gate already validated. Only Thai copy gets clamped.
	c.Callout = agent.TruncateRunes(clean(raw.Callout), 60)
	if raw.Panel != nil {
		p := &ContentUIPanel{
			Chrome:     clean(raw.Panel.Chrome),
			Breadcrumb: clean(raw.Panel.Breadcrumb),
		}
		for _, it := range raw.Panel.Items {
			if len(p.Items) >= 5 { // เกิน 5 แถวล้นกรอบ 9:16
				break
			}
			if lb := strings.TrimSpace(clean(it.Label)); lb != "" {
				p.Items = append(p.Items, ContentUIItem{Label: lb, State: agent.ClampUIState(it.State)})
			}
		}
		if raw.Panel.Field != nil {
			p.Field = &ContentUIField{
				Label: clean(raw.Panel.Field.Label),
				Value: clean(raw.Panel.Field.Value),
			}
		}
		c.Panel = p
	}
```

(4c) `Title` ของ uistep ต้อง clamp ที่ 34 rune ไม่ใช่ปล่อยยาว — แก้บรรทัด 160 `c.Title = clean(raw.Title)` เป็น:

```go
	c.Title = clean(raw.Title) // may legitimately contain <span class="acc">…</span>
	if c.Layout == "uistep" {
		c.Title = agent.TruncateRunes(c.Title, 34)
	}
```

(4d) **สำคัญมาก** — แก้เงื่อนไข hero-fallback (บรรทัด 197-199) ให้นับ Panel/Callout ด้วย ไม่งั้นซีน uistep ที่มีแต่ panel จะถูกตีเป็น `hero` แล้วหน้าจอหายทั้งซีน:

```go
	empty := c.Title == "" && len(c.Rows) == 0 && c.Stat == "" && c.CTA == "" &&
		len(c.Chips) == 0 && c.Pill == "" && c.Sub == "" && c.StatLabel == "" &&
		c.Stamp == "" && len(c.Panels) == 0 && c.Panel == nil && c.Callout == ""
```

(4e) เติม `StepTotal` ใน `buildSceneSpecs` — เพิ่มหลัง loop หลัก (หลังบรรทัด 85 `}`) ก่อน `return specs`:

```go
	// StepTotal is derived after the loop because a scene cannot know how many
	// siblings share its layout. Go owns this value so the progress rail never
	// depends on parsing the model's Thai "ขั้นที่ n / N" string.
	stepTotal := 0
	for i := range specs {
		if specs[i].Content.Layout == "uistep" {
			stepTotal++
		}
	}
	for i := range specs {
		if specs[i].Content.Layout == "uistep" {
			specs[i].Content.StepTotal = stepTotal
		}
	}
```

- [ ] **Step 5: รันเทสให้ผ่าน**

Run: `go test ./internal/producer/ -v`
Expected: PASS ทั้งหมด รวมเทส case เดิม (`TestBuildSceneContentCasefile`, `TestBuildSceneContentComicPanels`)

- [ ] **Step 6: Commit**

```bash
git add internal/producer/composition_types.go internal/producer/scene_adapter.go internal/producer/scene_adapter_tutorial_test.go
git commit -m "feat(tutorial): map uistep panel/callout into SceneContent"
```

---

## Task 6: Template — CSS โหมด tutorial + renderer `uistep` + progress rail

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl:198` (ต่อท้าย block case CSS), `:253` (constants), `:273-274` (bg reuse), `:374` (ต่อท้าย chain ของ renderer)
- Create: `internal/producer/composition_tutorial_render_test.go`

**Interfaces:**
- Consumes: `SceneContent.Panel/Callout/StepTotal` (Task 5), `ScenesParams.Format` (Task 1)
- Produces: HTML ที่มี `data-format="tutorial"`, `const FORMAT_TUTORIAL = true`, class `ui-panel` / `ui-item` / `ui-rail` / `ui-callout`

- [ ] **Step 1: เขียนเทส render ที่ยังไม่ผ่าน**

สร้าง `internal/producer/composition_tutorial_render_test.go`:

```go
package producer

import (
	"strings"
	"testing"
)

func tutorialParams() ScenesParams {
	mk := func(n int, layout string, c SceneContent) SceneSpec {
		c.SceneNumber, c.Layout = n, layout
		c.Start, c.End = float64(n-1)*5, float64(n)*5
		return SceneSpec{SceneNumber: n, StartSec: c.Start, EndSec: c.End,
			LayoutVariant: "hook_big", CaptionStyle: "phrase_block", Content: c}
	}
	step := func(n int, num string) SceneContent {
		return SceneContent{
			Num: num, Of: "ขั้นที่ " + num + " / 3", StepTotal: 3,
			Title: "ตั้งเงื่อนไขให้ตัดงบ",
			Panel: &ContentUIPanel{
				Chrome: "Ads Manager", Breadcrumb: "Rules › Create new rule",
				Items: []ContentUIItem{
					{Label: "Campaigns", State: "normal"},
					{Label: "Ad sets", State: "done"},
					{Label: "Cost per result", State: "target"},
				},
				Field: &ContentUIField{Label: "Greater than", Value: "400 THB"},
			},
			Callout: "เลือก Cost per result แล้วใส่ 400 บาท",
		}
	}
	return ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 30, Format: "tutorial", ThemeKey: "tutorial",
		Scenes: []SceneSpec{
			mk(1, "hook", SceneContent{Rows: []ContentRow{{Text: "ตีสามบัญชีโดนปิด งบยังวิ่ง"}},
				BackgroundImage: "assets/bg-scene1.png"}),
			mk(2, "uistep", step(2, "1")),
			mk(3, "uistep", step(3, "2")),
			mk(4, "uistep", step(4, "3")),
			mk(5, "cta", SceneContent{Title: "ทำครั้งเดียวจบ", CTA: "ทักมาเช็ค", Brand: "ADS VANCE"}),
		},
		Segments: []TranscriptSegment{{Text: "ตีสามบัญชีโดนปิด", Start: 0, End: 2}},
	}
}

func TestRenderTutorialFormat(t *testing.T) {
	out, err := RenderCompositionScenes(tutorialParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{
		`data-format="tutorial"`,
		"const FORMAT_TUTORIAL = true",
		"ui-panel", "ui-chrome", "ui-crumb", "ui-item", "ui-field", "ui-rail", "ui-callout",
		"Cost per result", "Ads Manager", "400 THB",
		"เลือก Cost per result แล้วใส่ 400 บาท",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q", want)
		}
	}
	// กันบทเรียน blank-video regression: "-->" ใน <script> ทำให้ JS พังทั้งคลิป
	if strings.Contains(html[strings.Index(html, "<script>"):], "-->") {
		t.Error("inline script must never contain the sequence minus-minus-gt")
	}
	// กัน tofu: ห้ามใช้ glyph ที่ฟอนต์ bundle ไม่มีเป็น CSS content
	for _, glyph := range []string{`content:"✓"`, `content:"◀"`, `content:"●"`} {
		if strings.Contains(html, glyph) {
			t.Errorf("template must not use %s — bundled fonts render it as tofu", glyph)
		}
	}
}

func TestRenderCaseFormatUnaffectedByTutorialBlock(t *testing.T) {
	out, err := RenderCompositionScenes(caseParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if !strings.Contains(html, `data-format="case"`) {
		t.Error("case format regressed")
	}
	if strings.Contains(html, "const FORMAT_TUTORIAL = true") {
		t.Error("case clips must not enable the tutorial renderer")
	}
}
```

- [ ] **Step 2: รันเทสให้เห็นว่า fail**

Run: `go test ./internal/producer/ -run 'TestRenderTutorial|TestRenderCaseFormatUnaffected' -v`
Expected: FAIL — `output missing "const FORMAT_TUTORIAL = true"` และ class ต่างๆ

- [ ] **Step 3: เพิ่ม CSS — แทรกหลังบรรทัด 198 (ท้าย block `[data-format='case']`)**

```html
      /* ── tutorial format: simulated product UI ── */
      [data-format='tutorial'] .scene[data-layout="uistep"] .scene-content{top:150px;bottom:400px;
        justify-content:flex-start;align-items:stretch;gap:26px}
      [data-format='tutorial'] .scene[data-layout="hook"] .scene-content{bottom:520px}
      [data-format='tutorial'] .scene[data-layout="hero"] .scene-content,
      [data-format='tutorial'] .scene[data-layout="tip"] .scene-content,
      [data-format='tutorial'] .scene[data-layout="step"] .scene-content{top:150px;bottom:430px;justify-content:center}
      [data-format='tutorial'] .scene[data-layout="hero"] .scene-bg,
      [data-format='tutorial'] .scene[data-layout="tip"] .scene-bg,
      [data-format='tutorial'] .scene[data-layout="cta"] .scene-bg{opacity:.35;filter:saturate(.6)}
      .ui-rail{display:flex;align-items:center;gap:18px}
      .ui-rail .rl{font-weight:700;font-size:32px;letter-spacing:.04em;color:var(--amber-bright)}
      .ui-rail .dots{display:flex;gap:12px}
      .ui-rail .dot{width:18px;height:18px;border-radius:50%;background:rgba(188,210,255,.25);display:block}
      .ui-rail .dot.on{background:var(--amber-bright);box-shadow:0 0 14px rgba(255,180,84,.55)}
      .ui-step-title{font-weight:800;font-size:58px;line-height:1.24;color:#fff;overflow-wrap:break-word}
      .ui-panel{background:#F2F4F8;border-radius:24px;overflow:hidden;
        box-shadow:0 26px 64px rgba(0,0,0,.48);border:2px solid rgba(255,255,255,.14)}
      .ui-chrome{display:flex;align-items:center;gap:16px;padding:20px 26px;
        background:#E2E6ED;border-bottom:2px solid #CDD3DD}
      .ui-chrome .dotset{display:flex;gap:10px}
      .ui-chrome .dotset i{width:16px;height:16px;border-radius:50%;background:#B9C0CC;display:block}
      .ui-chrome .title{font-weight:700;font-size:30px;color:#3A4351}
      .ui-crumb{padding:18px 26px;font-weight:600;font-size:27px;color:#66707F;border-bottom:1px solid #DDE2EA}
      .ui-list{padding:14px 20px 18px;display:flex;flex-direction:column;gap:8px}
      .ui-item{display:flex;align-items:center;gap:16px;padding:18px 20px;border-radius:14px;
        font-weight:600;font-size:33px;color:#2C3341}
      .ui-item .lb{overflow-wrap:break-word}
      .ui-item .mk{margin-left:auto;flex:0 0 auto}
      .ui-item.done{color:#5A6373}
      .ui-item.done .mk{width:22px;height:22px;border-radius:50%;background:#2FA36B}
      .ui-item.target{background:#FFF3E0;border:3px solid var(--amber);color:#1D2430;font-weight:800}
      .ui-item.target .mk{width:0;height:0;border-top:15px solid transparent;
        border-bottom:15px solid transparent;border-right:22px solid var(--amber)}
      .ui-field{margin:0 20px 20px;padding:20px 22px;border:3px solid var(--amber);
        border-radius:14px;background:#FFF8EE}
      .ui-field .fl{font-weight:600;font-size:27px;color:#66707F}
      .ui-field .fv{font-weight:800;font-size:40px;color:#1D2430;overflow-wrap:break-word}
      .ui-callout{font-weight:700;font-size:42px;line-height:1.32;color:var(--ink);overflow-wrap:break-word}
```

- [ ] **Step 4: เพิ่ม constant — แก้บรรทัด 253-254**

```html
      const FORMAT_CASE = {{if eq .Format "case"}}true{{else}}false{{end}};
      const FORMAT_TUTORIAL = {{if eq .Format "tutorial"}}true{{else}}false{{end}};
      const CASE_NO = {{.CaseNumber}} || 0;
```

- [ ] **Step 5: ให้ซีนไร้ภาพในโหมด tutorial ยืมภาพปก — แก้บรรทัด 271-274**

```js
        // hero/verdict (case) and hero/tip/cta (tutorial) without their own image
        // reuse the cover shot (scene 0's bg) as a dim backdrop — bookends the
        // clip at zero extra image cost.
        const reuseCover =
          (FORMAT_CASE && (sc.type === "hero" || sc.type === "verdict")) ||
          (FORMAT_TUTORIAL && (sc.type === "hero" || sc.type === "tip" || sc.type === "cta"));
        const bgSrc = (!sc.bg && reuseCover && SCENES[0] && SCENES[0].bg) ? SCENES[0].bg : sc.bg;
```

- [ ] **Step 6: เพิ่ม renderer — แทรกก่อน `else if(sc.type==="cta"){` (บรรทัด 368)**

```js
        else if(sc.type==="uistep"){
          const rail=el("div","ui-rail");
          if(sc.of) rail.appendChild(el("div","rl",sc.of));
          const total=sc.stepTotal||0, cur=parseInt(sc.num,10)||0;
          if(total>0){
            const dots=el("div","dots");
            for(let k=1;k<=total;k++) dots.appendChild(el("i","dot"+(k<=cur?" on":"")));
            rail.appendChild(dots);
          }
          c.appendChild(rail);
          if(sc.title) c.appendChild(el("div","ui-step-title",sc.title));
          const p=sc.panel||{};
          const box=el("div","ui-panel");
          const ch=el("div","ui-chrome");
          const ds=el("div","dotset");
          for(let k=0;k<3;k++) ds.appendChild(el("i"));
          ch.appendChild(ds);
          if(p.chrome) ch.appendChild(el("div","title",p.chrome));
          box.appendChild(ch);
          if(p.breadcrumb) box.appendChild(el("div","ui-crumb",p.breadcrumb));
          const list=el("div","ui-list");
          (p.items||[]).forEach(it=>{
            const row=el("div","ui-item "+(it.state||"normal"));
            row.appendChild(el("span","lb",it.label));
            row.appendChild(el("span","mk"));
            list.appendChild(row);
          });
          box.appendChild(list);
          if(p.field){
            const f=el("div","ui-field");
            f.appendChild(el("div","fl",p.field.label));
            f.appendChild(el("div","fv",p.field.value));
            box.appendChild(f);
          }
          c.appendChild(box);
          if(sc.callout) c.appendChild(el("div","ui-callout",sc.callout));
        }
```

- [ ] **Step 7: รันเทสให้ผ่าน**

Run: `go test ./internal/producer/ -v`
Expected: PASS — รวม `TestRenderCaseFormat` และ `TestRenderClassicFormatUnchanged` ที่ต้องไม่กระทบ

- [ ] **Step 8: เรนเดอร์ HTML ออกมาดูด้วยตา**

```bash
go test ./internal/producer/ -run TestRenderTutorialFormat -v
```
แล้วเปิดไฟล์ HTML ที่ `render-test-out/` (ถ้าเทสไม่เขียนไฟล์ ให้เพิ่มชั่วคราวใน test: `os.WriteFile("../../render-test-out/tutorial.html", out, 0o644)` รันดู แล้วลบบรรทัดนั้นทิ้งก่อน commit)
Expected: การ์ดหน้าจอสีอ่อน แถวไฮไลต์ส้ม สามเหลี่ยมชี้ ไม่มีกล่องสี่เหลี่ยม tofu

- [ ] **Step 9: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_tutorial_render_test.go
git commit -m "feat(tutorial): uistep renderer + tutorial CSS block + progress rail"
```

---

## Task 7: `TutorialPreset` + นโยบายภาพ 1 ใบ

**Files:**
- Create: `internal/producer/tutorial_format.go`
- Create: `internal/producer/tutorial_format_test.go`
- Modify: `internal/producer/case_format.go` (ลบ stub `buildTutorialCoverPrompt` ที่ใส่ไว้ใน Task 1)

**Interfaces:**
- Consumes: `StylePreset`, `Brand`, `buildImagePromptCore` (มีอยู่แล้วใน `case_format.go`)
- Produces: `producer.TutorialPreset` (Key `"tutorial"`), `producer.buildTutorialCoverPrompt(concept string, preset StylePreset, clipToken string) string`

- [ ] **Step 1: เขียนเทสที่ยังไม่ผ่าน**

สร้าง `internal/producer/tutorial_format_test.go`:

```go
package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestTutorialPresetNotInRandomPool(t *testing.T) {
	for _, p := range Presets {
		if p.Key == "tutorial" {
			t.Fatal("tutorial must NOT be in Presets (random pool)")
		}
	}
	if PresetByKey("tutorial").Key != "tutorial" {
		t.Error("PresetByKey must resolve tutorial (resume path)")
	}
	if TutorialPreset.BrandCSS() == "" {
		t.Error("tutorial BrandCSS must render")
	}
	if TutorialPreset.Palette != Brand {
		t.Error("tutorial must keep the brand navy+orange palette")
	}
}

func TestTutorialCoverPromptIsEditorialNotForensic(t *testing.T) {
	out := buildTutorialCoverPrompt("a phone showing a night alert", TutorialPreset, "clip-x")
	if !strings.Contains(out, "a phone showing a night alert") {
		t.Errorf("cover prompt lost the subject: %s", out)
	}
	if !strings.Contains(out, "clip-x") {
		t.Error("cover prompt must keep the cohesion style-set token")
	}
	if strings.Contains(strings.ToLower(out), "forensic") {
		t.Error("tutorial cover must not inherit the case-file forensic anchor")
	}
}

func TestImageScenesForModeTutorialCapsAtOne(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "hook", ImagePrompt: "a phone at night"},
		{SceneNumber: 2, Layout: "uistep", ImagePrompt: "should be ignored"},
		{SceneNumber: 3, Layout: "hero", ImagePrompt: "also ignored"},
	}
	allowed := imageScenesForMode(scenes, ModeTutorial)
	if len(allowed) != 1 || !allowed[1] {
		t.Errorf("tutorial must allow exactly scene 1, got %v", allowed)
	}
}

func TestImageScenesForModeTutorialNoPrompts(t *testing.T) {
	scenes := []agent.GeneratedScene{{SceneNumber: 1, Layout: "hook"}}
	allowed := imageScenesForMode(scenes, ModeTutorial)
	if allowed == nil {
		t.Fatal("tutorial must return an (empty) allow-set, never nil = unrestricted")
	}
	if len(allowed) != 0 {
		t.Errorf("no image prompts must yield an empty allow-set, got %v", allowed)
	}
}

func TestImageScenesForModeCaseUnchanged(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "casefile", ImagePrompt: "a desk"},
		{SceneNumber: 2, Layout: "evidence", ImagePrompt: "a jar"},
		{SceneNumber: 3, Layout: "evidence", ImagePrompt: "a card"},
	}
	allowed := imageScenesForMode(scenes, ModeCase)
	if len(allowed) != 2 || !allowed[1] || !allowed[2] {
		t.Errorf("case cap of 2 regressed: %v", allowed)
	}
}
```

- [ ] **Step 2: รันเทสให้เห็นว่า fail**

Run: `go test ./internal/producer/ -run 'TestTutorialPreset|TestTutorialCover|TestImageScenesForMode' -v`
Expected: FAIL — `undefined: TutorialPreset`

- [ ] **Step 3: สร้าง `internal/producer/tutorial_format.go`**

```go
package producer

// TutorialPreset is the visual identity of the tutorial format: a clean manual,
// not the case-file investigation. Like CaseFilePreset it is deliberately NOT in
// Presets — the random/weighted pickers must never select it; the orchestrator
// chooses it explicitly for tutorial-mode clips.
//
// Palette stays Brand (navy + orange): the channel must still read as one brand,
// only the mood changes. Motion is fast and crisp — a walkthrough should feel
// efficient, never cinematic.
var TutorialPreset = StylePreset{
	Key:         "tutorial",
	DisplayName: "Tutorial Manual",
	Palette:     Brand,
	ImageAnchor: "Cinematic editorial PHOTOGRAPHY, shot on 35mm, moody low-key lighting with a deep-navy #0047AF wash " +
		"and a single warm amber #F0A030 practical light. Real-world scene of a marketer's workspace at night " +
		"(a phone face-up showing an alert, a laptop glow, an empty desk chair). Photorealistic, premium, restrained. " +
		"NO illustration, NO 3D render, NO cartoon, no text, no user interface, no screens with readable content. " +
		"Atmosphere: the quiet moment before something expensive goes wrong.",
	Font:        TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	HeadingFont: TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	Motion:      MotionProfile{EntranceDur: 0.28, EntranceEase: "power2.out", BGZoomTo: 1.04},
}

// buildTutorialCoverPrompt renders the image prompt for the ONE AI image a
// tutorial clip gets: the opening cover shot. Composition reserves the lower
// half for the hook copy. Every later scene is an HTML UI mock and gets no image
// — a photo there would compete with the menu the viewer must actually read.
func buildTutorialCoverPrompt(concept string, preset StylePreset, clipToken string) string {
	return buildImagePromptCore(concept,
		"cinematic wide shot, the key subject placed in the UPPER half of the frame, lower half dark and uncluttered",
		preset, clipToken)
}
```

- [ ] **Step 4: ลบ stub ใน `case_format.go`**

ลบฟังก์ชัน `buildTutorialCoverPrompt` ชั่วคราวที่เพิ่มไว้ใน Task 1 Step 3 ออกจาก `internal/producer/case_format.go`

- [ ] **Step 5: ให้ `PresetByKey` resolve `"tutorial"` ได้**

`internal/producer/presets.go:153-163` มีสาขา `CaseFilePreset` อยู่แล้ว เพิ่มสาขา tutorial ถัดจากมัน (จำเป็นสำหรับ resume path ที่อ่าน `clip.style_preset` กลับมา):

```go
func PresetByKey(key string) StylePreset {
	if key == CaseFilePreset.Key {
		return CaseFilePreset
	}
	if key == TutorialPreset.Key {
		return TutorialPreset
	}
	for _, p := range Presets {
		if p.Key == key {
			return p
		}
	}
	return Presets[0]
}
```

- [ ] **Step 6: รันเทสให้ผ่าน**

Run: `go test ./internal/producer/ -v`
Expected: PASS ทั้งหมด

- [ ] **Step 7: Commit**

```bash
git add internal/producer/tutorial_format.go internal/producer/tutorial_format_test.go internal/producer/case_format.go internal/producer/presets.go
git commit -m "feat(tutorial): TutorialPreset + single-cover-image policy"
```

---

## Task 8: Agent rows — `script_tutorial` / `scene_tutorial` / `critic_tutorial` + เกณฑ์ visual_qa

**Files:**
- Create: `migrations/063_tutorial_agents.sql`
- Modify: `internal/agent/scene.go:11-17` (`SceneTemplateData`), `:34` (`Generate`)
- Modify: `internal/orchestrator/orchestrator.go:481` (จุดเรียก `sceneAgent.Generate`)

**Interfaces:**
- Consumes: `agent.TutorialBrief` (Task 4)
- Produces: `SceneTemplateData.TutorialBrief string`;
  `(*SceneAgent).Generate(ctx, script string, targetSceneCount, targetDurationSec int, theme *models.BrandTheme, tutorialBrief string, cfg *models.AgentConfig) ([]GeneratedScene, error)`;
  agent rows `script_tutorial`, `scene_tutorial`, `critic_tutorial`

- [ ] **Step 1: เขียนเทสที่ยังไม่ผ่าน**

เพิ่มลงท้าย `internal/agent/tutorial_test.go`:

```go
func TestSceneTemplateDataSubstitutesTutorialBrief(t *testing.T) {
	out, err := renderTemplate("script:{{.Script}}|brief:{{.TutorialBrief}}|dur:{{.TargetDurationSec}}",
		SceneTemplateData{Script: "S", TutorialBrief: "B", TargetDurationSec: 55})
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	if out != "script:S|brief:B|dur:55" {
		t.Errorf("got %q", out)
	}
}

// โหมดอื่นต้องได้ค่าว่าง ไม่ใช่ตัว placeholder ค้างใน prompt
func TestSceneTemplateDataEmptyBriefLeavesNoPlaceholder(t *testing.T) {
	out, _ := renderTemplate("A{{.TutorialBrief}}B", SceneTemplateData{})
	if out != "AB" {
		t.Errorf("got %q, want %q", out, "AB")
	}
}
```

- [ ] **Step 2: รันเทสให้เห็นว่า fail**

Run: `go test ./internal/agent/ -run TestSceneTemplateData -v`
Expected: FAIL — `unknown field TutorialBrief in struct literal`

- [ ] **Step 3: แก้ `internal/agent/scene.go`**

```go
type SceneTemplateData struct {
	Script            string
	TargetSceneCount  int
	TargetDurationSec int
	ThemeDescription  string
	// TutorialBrief is the catalog block for tutorial-mode clips (menu path,
	// steps, trap, and the ONLY UI vocabulary allowed on screen). Empty string
	// in every other mode — renderTemplate is a plain string replacer, so an
	// empty value simply erases the placeholder.
	TutorialBrief string
}
```

และในเมธอด `Generate` เพิ่มพารามิเตอร์ `tutorialBrief string` ก่อน `cfg`:

```go
func (a *SceneAgent) Generate(ctx context.Context, script string, targetSceneCount, targetDurationSec int, theme *models.BrandTheme, tutorialBrief string, cfg *models.AgentConfig) ([]GeneratedScene, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, SceneTemplateData{
		Script:            script,
		TargetSceneCount:  targetSceneCount,
		TargetDurationSec: targetDurationSec,
		ThemeDescription:  buildSceneThemeDescription(theme),
		TutorialBrief:     tutorialBrief,
	})
	// ...ที่เหลือเหมือนเดิม
```

- [ ] **Step 4: อัปเดตจุดเรียกใน orchestrator**

`internal/orchestrator/orchestrator.go:481`:

```go
	scenes, err := o.sceneAgent.Generate(ctx, narration, targetSceneCount, targetDurationSec, clipTheme, agent.TutorialBrief(feat), sceneCfg)
```

(พารามิเตอร์ `feat *models.TutorialFeature` จะถูกเพิ่มเข้า `produceClipWithID` ใน Task 9 — ใน Task นี้ให้ส่ง `""` ไปก่อนแล้ว Task 9 ค่อยเปลี่ยนเป็น `agent.TutorialBrief(feat)`)

ดังนั้นใน Task นี้ให้เขียน:

```go
	scenes, err := o.sceneAgent.Generate(ctx, narration, targetSceneCount, targetDurationSec, clipTheme, "", sceneCfg)
```

- [ ] **Step 5: สร้าง `migrations/063_tutorial_agents.sql`**

```sql
-- 063: tutorial agent rows (spec docs/superpowers/specs/2026-07-25-fb-tutorial-content-design.md)
-- ก๊อป output contract จากแถวเดิมทั้งหมด เปลี่ยนเฉพาะวิธีเล่า (บทเรียน regression 052)
-- Rollback: DELETE FROM agent_configs WHERE agent_name LIKE '%_tutorial';
BEGIN;

-- script_tutorial: โครง 8 จังหวะของ tutorial ต่อหน้า prompt เดิม
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled)
SELECT 'script_tutorial',
       system_prompt || E'\n\nโหมดนำเสนอ: คู่มือทำตามได้จริง — คุณคือรุ่นพี่ที่นั่งข้างๆ แล้วบอกว่ากดตรงไหนใส่อะไร พูดสั้น ชัด ไม่อ้อม ห้ามใช้คำว่า คดี/หลักฐาน/ปิดคดี. ห้ามแนะนำวิธีหลบการตรวจจับหรือทำผิดนโยบาย Meta เด็ดขาด — สอนเฉพาะฟีเจอร์จริงที่กดได้',
       $tut_pfx$【โหมดคู่มือ — โครงบังคับ】
1) เปิดด้วยความเสียหาย (3 วินาทีแรก): บอกสิ่งที่จะเสียถ้าไม่ทำ ห้ามขึ้นต้นด้วยชื่อฟีเจอร์ ห้ามทักทาย เช่น "ตีสามบัญชีโดนปิด งบยังวิ่งอยู่สี่หมื่น"
2) สัญญา + บอกจำนวนขั้น: "ฟีเจอร์นี้กันได้ ทำครั้งเดียวจบ มีสามขั้น" — ต้องบอกจำนวนขั้นให้ตรงกับข้อมูลฟีเจอร์
3) เดินขั้นตอนทีละขั้นตามลำดับในข้อมูลฟีเจอร์ ห้ามข้าม ห้ามเพิ่ม ห้ามสลับลำดับ
4) ก่อนขั้นสุดท้าย แทรกกับดัก: บอกจุดที่คนตั้งผิดแล้วฟีเจอร์ไม่ทำงาน
5) สรุปทุกขั้นในประโยคเดียว บอกให้คนแคปหน้าจอเก็บไว้
6) ปิดแบบชวนคุย ไม่ขายของ

กฎเหล็ก: ชื่อเมนู ปุ่ม และฟิลด์ทุกคำ ต้องมาจากรายการคำศัพท์ UI ในข้อมูลฟีเจอร์เท่านั้น ห้ามแต่งชื่อเมนูขึ้นมาเอง ถ้าไม่มีในรายการให้เลี่ยงไปอธิบายเป็นภาษาไทยแทน

$tut_pfx$ || prompt_template,
       model, temperature, TRUE
FROM agent_configs WHERE agent_name = 'script'
  AND NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'script_tutorial');

-- scene_tutorial: template ใหม่ทั้งฉบับ (คง JSON output contract เดิมของ scene)
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled)
SELECT 'scene_tutorial',
       'คุณคือ Director สายคู่มือการใช้งาน แตกสคริปเป็นซีนสำหรับ explainer แนวตั้ง 9:16 ภาษาไทย โหมด "คู่มือ". เป้าหมายสูงสุด: 3 วินาทีแรกต้องหยุดนิ้วคนดูด้วยความเสียหาย และคนดูต้องทำตามได้จริงโดยไม่ต้องหาเมนูเอง. ห้ามใส่ emoji เด็ดขาด ตอบเป็น JSON เท่านั้น.',
       $scene_tut$แตกสคริปนี้ออกเป็นซีนสำหรับวิดีโอแนวตั้ง 9:16 ยาว {{.TargetDurationSec}} วินาที — โหมด "คู่มือ"

สคริป:
{{.Script}}

{{.TutorialBrief}}

ธีมแบรนด์: {{.ThemeDescription}}

โครงบังคับ:
- ซีนแรก layout "hook": rows 1-2 บรรทัดบอกความเสียหาย (ไม่เกิน 36 ตัวอักษรต่อบรรทัด) — เป็นซีนเดียวที่มี image_prompt ได้
- ซีนที่สอง layout "hero": title = สัญญา + จำนวนขั้น (ไม่เกิน 40)
- ตรงกลาง layout "uistep" หนึ่งซีนต่อหนึ่งขั้น จำนวนต้องเท่ากับจำนวนขั้นในข้อมูลฟีเจอร์พอดี เรียงตามลำดับ n
- ก่อนซีน uistep ตัวสุดท้าย แทรก layout "tip" หนึ่งซีน = กับดักที่คนพลาด
- ซีนรองสุดท้าย layout "step": การ์ดสรุปทุกขั้นใน 1 เฟรม ให้คนแคปเก็บ
- ซีนสุดท้าย layout "cta"

กฎภาพ (สำคัญมาก): image_prompt ใส่ได้เฉพาะซีนแรก (layout "hook") เท่านั้น หนึ่งใบต่อคลิป — บรรยายภาษาอังกฤษ เป็นภาพบรรยากาศที่สื่อความเสียหาย ห้ามบรรยายหน้าจอ ห้ามมีตัวอักษรในภาพ. ทุกซีนที่เหลือ image_prompt = ""

กฎคำศัพท์ UI (เข้มที่สุด): ค่าใน panel.chrome, panel.breadcrumb, panel.items[].label และ panel.field.label ต้องเป็นคำที่ปรากฏในรายการคำศัพท์ UI ข้างบนแบบตรงตัวเท่านั้น ห้ามแปล ห้ามย่อ ห้ามแต่งใหม่ ระบบจะตีคลิปตกทันทีถ้าพบคำนอกรายการ

ตอบเป็น JSON array เท่านั้น แต่ละ object มี:
- "scene_number": ลำดับซีน (เริ่ม 1 ต่อเนื่อง)
- "voice_text": ประโยคพากย์ไทยของซีนนี้ (สั้น พูดลื่น เหมือนรุ่นพี่สอน)
- "on_screen_text": ข้อความบนจอสั้นๆ (ซีนแรกไม่เกิน 7 คำ)
- "emphasis_words": array 1-2 คำที่ต้องเน้น (ห้ามว่าง)
- "caption_style": "word_pop" (ซีนเปิด) หรือ "phrase_block" (ซีนเนื้อหา)
- "image_prompt": ตามกฎภาพข้างบน
- "layout": หนึ่งใน "hook" | "hero" | "uistep" | "tip" | "step" | "cta"
- "content": object ตาม layout (ด้านล่าง)

content แยกตาม layout:
- hook: {"rows":[{"t":"ตีสามบัญชีโดนปิด"},{"t":"งบยังวิ่งอยู่ 40,000"}]}
- hero: {"title":"ฟีเจอร์นี้กันได้ มีแค่ 3 ขั้น","sub":"ทำครั้งเดียวจบ"}
- uistep: {"num":"2","of":"ขั้นที่ 2 / 3","title":"ชื่อขั้นภาษาไทย","panel":{"chrome":"Ads Manager","breadcrumb":"Rules > Create new rule","items":[{"label":"Campaigns","state":"normal"},{"label":"Ad sets","state":"done"},{"label":"Cost per result","state":"target"}],"field":{"label":"Greater than","value":"400 THB"}},"callout":"เลือก Cost per result แล้วใส่ 400 บาท"}
- tip: {"pill":"กับดัก","rows":[{"t":"ข้อความกับดักไม่เกิน 36"}]}
- step: {"num":"","of":"สรุปทั้งหมด","title":"แคปเก็บไว้ทำตาม","rows":[{"t":"1. ..."},{"t":"2. ..."},{"t":"3. ..."}]}
- cta: {"title":"ปิดท้ายสั้นๆ","cta":"ทักมาเช็ค","brand":"ADS VANCE","sub":"คำโปรยสั้น"}

กฎ uistep: state มีแค่ "normal" | "target" | "done" — ขั้นที่ผ่านมาแล้วเป็น done ขั้นปัจจุบันเป็น target หนึ่งซีนมี target ได้แค่หนึ่งอัน. items ไม่เกิน 5 แถว. num เป็นตัวเลขขั้นปัจจุบันเท่านั้น

กฎเหล็ก: ห้าม emoji หรือสัญลักษณ์ภาพในทุก field. ความยาว: title ไม่เกิน 34, callout ไม่เกิน 60, cta ไม่เกิน 14, sub ไม่เกิน 50, rows แต่ละแถวไม่เกิน 36$scene_tut$,
       model, temperature, TRUE
FROM agent_configs WHERE agent_name = 'scene'
  AND NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'scene_tutorial');

-- critic_tutorial: เกณฑ์คุณภาพเฉพาะโหมดคู่มือ
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled)
SELECT 'critic_tutorial',
       system_prompt || E'\n\nโหมด "คู่มือ": (1) ซีนแรกต้องเป็นความเสียหายที่จับต้องได้ ถ้าเปิดด้วยชื่อฟีเจอร์หรือคำทักทายให้เขียนใหม่ (2) ซีน layout step ที่เป็นการ์ดสรุปต้องมีครบทุกขั้นที่ปรากฏในซีน uistep ถ้าขาดให้เติม (3) ห้ามมีคำแนะนำที่เป็นการหลบการตรวจจับหรือทำผิดนโยบาย Meta ถ้าพบให้ตัดทิ้ง (4) ห้ามแก้ค่าใน panel.chrome, panel.breadcrumb, panel.items[].label, panel.field.label เด็ดขาด — ค่าเหล่านี้มาจากรายการที่คนคุมไว้ แก้แล้วคลิปจะถูกตีตก',
       prompt_template, model, temperature, TRUE
FROM agent_configs WHERE agent_name = 'critic'
  AND NOT EXISTS (SELECT 1 FROM agent_configs WHERE agent_name = 'critic_tutorial');

-- Visual QA: องค์ประกอบดีไซน์โหมดคู่มือไม่ใช่ defect (กัน false positive แบบ PR#14/#17)
UPDATE agent_configs SET system_prompt = system_prompt || E'\n\nโหมด "คู่มือ" (ถ้าเฟรมมีการ์ดหน้าจอสีอ่อนบนพื้นน้ำเงิน): การ์ดจำลองหน้าจอสีเทาอ่อน แถบหัวการ์ดที่มีจุดกลมสามจุด แถวเมนูที่มีกรอบส้มไฮไลต์ สามเหลี่ยมส้มชี้เข้าแถว จุดกลมเขียวท้ายแถว และแถบจุดบอกความคืบหน้า — ทั้งหมดคือดีไซน์ที่ตั้งใจ ไม่ใช่ข้อบกพร่อง อย่าตั้ง ok=false เพราะสิ่งเหล่านี้ และอย่าตั้ง ok=false เพราะข้อความในการ์ดเป็นภาษาอังกฤษ (เป็นชื่อเมนูจริงของ Meta)'
WHERE agent_name = 'visual_qa'
  AND system_prompt NOT LIKE '%โหมด "คู่มือ"%';

COMMIT;
```

- [ ] **Step 6: รันเทสให้ผ่าน**

Run: `go build ./... && go test ./internal/...`
Expected: PASS ทั้งหมด

- [ ] **Step 7: ตรวจว่า migration รันได้จริงบน branch ทดสอบ**

Run: `psql "$DATABASE_URL" -f migrations/063_tutorial_agents.sql`
Expected: `INSERT 0 1` สามครั้ง + `UPDATE 1`

- [ ] **Step 8: Commit**

```bash
git add migrations/063_tutorial_agents.sql internal/agent/scene.go internal/orchestrator/orchestrator.go
git commit -m "feat(tutorial): script/scene/critic tutorial agent rows + brief plumbing"
```

---

## Task 9: Orchestrator — `ProduceTutorial` + mode resolution + gate

**Files:**
- Create: `internal/orchestrator/tutorial.go`
- Create: `internal/orchestrator/tutorial_test.go`
- Modify: `internal/orchestrator/orchestrator.go:342`, `:456`, `:475`, `:481`, `:601`, `:839-851`, `:893-901`, `:923`
- Modify: `internal/orchestrator/script_debate.go:98-101`
- Modify: จุด wire repo (ตัวสร้าง `Orchestrator` — ค้นด้วย `grep -rn "NewOrchestrator" cmd/ internal/`)

**Interfaces:**
- Consumes: `repository.TutorialFeaturesRepo` (Task 2), `agent.TutorialBrief` / `agent.UIVocabViolations` / `agent.CountUIStepScenes` (Task 4), `producer.TutorialPreset` (Task 7)
- Produces: `(*Orchestrator).ProduceTutorial(ctx context.Context) error`;
  `orchestrator.clipMode(contentFormat string) string`;
  `orchestrator.tutorialSceneShape(steps int) (sceneCount, durationSec int)`;
  `orchestrator.tutorialGateFailure(scenes []agent.GeneratedScene, feat *models.TutorialFeature) string`

- [ ] **Step 1: เขียนเทสที่ยังไม่ผ่าน**

สร้าง `internal/orchestrator/tutorial_test.go`:

```go
package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
)

func TestClipModeFromContentFormat(t *testing.T) {
	t.Setenv("CASE_FORMAT_ENABLED", "true")
	if got := clipMode("tutorial"); got != producer.ModeTutorial {
		t.Errorf("tutorial format = %q, want tutorial mode even while the case flag is on", got)
	}
	if got := clipMode("qa"); got != producer.ModeCase {
		t.Errorf("qa with case flag on = %q, want case", got)
	}
	t.Setenv("CASE_FORMAT_ENABLED", "")
	if got := clipMode("qa"); got != producer.ModeClassic {
		t.Errorf("qa with case flag off = %q, want classic", got)
	}
	if got := clipMode("tutorial"); got != producer.ModeTutorial {
		t.Errorf("tutorial must not depend on the case flag, got %q", got)
	}
}

func TestTutorialSceneShape(t *testing.T) {
	for _, tc := range []struct{ steps, scenes, dur int }{
		{3, 8, 55}, {4, 9, 64}, {5, 10, 73},
	} {
		sc, d := tutorialSceneShape(tc.steps)
		if sc != tc.scenes || d != tc.dur {
			t.Errorf("steps=%d -> (%d scenes, %ds), want (%d, %d)", tc.steps, sc, d, tc.scenes, tc.dur)
		}
	}
}

func gateFeature() *models.TutorialFeature {
	return &models.TutorialFeature{
		FeatureKey: "f",
		UIVocab:    []string{"Ads Manager", "Rules", "Cost per result"},
		Steps: []models.TutorialStep{
			{N: 1, UITarget: "Rules"}, {N: 2, UITarget: "Cost per result"},
		},
	}
}

func gateScene(n int, label string) agent.GeneratedScene {
	return agent.GeneratedScene{SceneNumber: n, Layout: "uistep",
		Content: json.RawMessage(`{"panel":{"chrome":"Ads Manager","items":[{"label":"` + label + `","state":"target"}]}}`)}
}

func TestTutorialGatePassesCleanClip(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "hook"},
		gateScene(2, "Rules"), gateScene(3, "Cost per result"),
		{SceneNumber: 4, Layout: "cta"},
	}
	if msg := tutorialGateFailure(scenes, gateFeature()); msg != "" {
		t.Errorf("clean clip must pass, got %q", msg)
	}
}

func TestTutorialGateBlocksInventedMenu(t *testing.T) {
	scenes := []agent.GeneratedScene{
		gateScene(1, "Advanced Rules Manager"), gateScene(2, "Cost per result"),
	}
	msg := tutorialGateFailure(scenes, gateFeature())
	if !strings.Contains(msg, "Advanced Rules Manager") {
		t.Errorf("gate must name the invented label, got %q", msg)
	}
}

func TestTutorialGateBlocksWrongStepCount(t *testing.T) {
	scenes := []agent.GeneratedScene{gateScene(1, "Rules")} // 1 uistep, catalog says 2
	msg := tutorialGateFailure(scenes, gateFeature())
	if !strings.Contains(msg, "1") || !strings.Contains(msg, "2") {
		t.Errorf("gate must report got/want step counts, got %q", msg)
	}
}

func TestTutorialGateNilFeatureIsNoOp(t *testing.T) {
	if msg := tutorialGateFailure([]agent.GeneratedScene{{Layout: "hero"}}, nil); msg != "" {
		t.Errorf("non-tutorial clips must never be gated, got %q", msg)
	}
}
```

- [ ] **Step 2: รันเทสให้เห็นว่า fail**

Run: `go test ./internal/orchestrator/ -run 'TestClipMode|TestTutorialSceneShape|TestTutorialGate' -v`
Expected: FAIL — `undefined: clipMode`, `undefined: tutorialSceneShape`, `undefined: tutorialGateFailure`

- [ ] **Step 3: สร้าง `internal/orchestrator/tutorial.go`**

```go
package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
)

// tutorialFormatName is the content_formats row (seeded disabled so the normal
// weighted picker can never select it) that marks a clip as tutorial mode.
const tutorialFormatName = "tutorial"

// clipMode derives the render mode from the clip's persisted content_format.
// Tutorial wins over the process-wide case flag so a tutorial clip renders as a
// manual even on a server where CASE_FORMAT_ENABLED is on. Deriving from the
// stored column (not a flag) is what makes resume and retry pick the right mode.
func clipMode(contentFormat string) string {
	if contentFormat == tutorialFormatName {
		return producer.ModeTutorial
	}
	if producer.CaseFormatEnabled() {
		return producer.ModeCase
	}
	return producer.ModeClassic
}

// tutorialSceneShape maps a catalog step count onto the clip's scene budget:
// hook + promise + N uisteps + trap + recap + cta. Duration grows ~9s per step.
func tutorialSceneShape(steps int) (sceneCount, durationSec int) {
	return steps + 5, 28 + steps*9
}

// tutorialGateFailure is the deterministic half of the tutorial safety net: it
// returns a non-empty reason when the generated scenes would teach something the
// catalog never authorized. Returns "" for a nil feature (non-tutorial clips).
//
// This runs INSTEAD of a human reviewer — the clip auto-publishes when it comes
// back empty, so it must be strict about the two things a wrong tutorial gets
// wrong: an invented menu name, and a missing or invented step.
func tutorialGateFailure(scenes []agent.GeneratedScene, feat *models.TutorialFeature) string {
	if feat == nil {
		return ""
	}
	if v := agent.UIVocabViolations(scenes, feat.UIVocab); len(v) > 0 {
		return "ui_vocab violation: " + v[0] + fmt.Sprintf(" (and %d more)", len(v)-1)
	}
	if got, want := agent.CountUIStepScenes(scenes), len(feat.Steps); got != want {
		return fmt.Sprintf("step count mismatch: %d uistep scenes, catalog has %d steps", got, want)
	}
	return ""
}

// ProduceTutorial produces exactly one tutorial clip from the catalog. It mirrors
// ProduceWeekly's gate/tracker discipline but skips the question agent entirely —
// the topic IS the catalog row, so there is nothing to invent.
func (o *Orchestrator) ProduceTutorial(ctx context.Context) error {
	if credits, err := o.producer.KieCredits(ctx); err != nil {
		log.Printf("kie credit pre-check skipped (non-fatal): %v", err)
	} else if credits <= 0 {
		return fmt.Errorf("kie เครดิตหมด (เหลือ %d) — เติมเครดิตที่ kie.ai ก่อนผลิต", credits)
	}

	feat, err := o.pickVerifiedFeature(ctx)
	if err != nil {
		return err
	}
	if feat == nil {
		return fmt.Errorf("tutorial catalog is empty or every feature is parked for re-verification")
	}
	log.Printf("Producing tutorial clip — feature: %s (%d steps)", feat.FeatureKey, len(feat.Steps))

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
	scriptCfg, err := o.modeAgentConfig(ctx, "script", producer.ModeTutorial)
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
	format, err := o.formatsRepo.GetByName(ctx, tutorialFormatName)
	if err != nil {
		return fmt.Errorf("get tutorial content format: %w", err)
	}
	persona, _ := o.settingsRepo.Get(ctx, "audience_persona")

	q := agent.GeneratedQuestion{
		Question:  feat.DisplayNameTH,
		Category:  "grey-operator",
		PainPoint: feat.PainPoint,
	}

	o.tracker.SetTotalClips(1)
	o.tracker.StartClip(1, feat.DisplayNameTH)
	if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format,
		persona, models.TitleArchetype{}, "reach", feat); err != nil {
		o.tracker.AddErrorLog(fmt.Sprintf("tutorial clip failed: %v", err))
		o.nudgeRetry()
		return err
	}
	if mErr := o.tutorialFeaturesRepo.MarkUsed(ctx, feat.ID); mErr != nil {
		log.Printf("tutorial: MarkUsed failed (non-fatal): %v", mErr)
	}
	o.tracker.CompleteStep("complete")
	log.Println("Tutorial production complete")
	return nil
}

// pickVerifiedFeature picks the least-used feature and asks research whether its
// menu path still matches Meta's current UI. A feature research says has moved
// gets parked (needs_verify) and the next one is tried — up to 2 skips, then the
// last pick is produced anyway. Never returning a feature would mean 0 clips,
// which is worse than one clip on a possibly-moved menu (the ui_vocab gate still
// guards the labels themselves).
func (o *Orchestrator) pickVerifiedFeature(ctx context.Context) (*models.TutorialFeature, error) {
	const maxSkips = 2
	var skipped []string
	var last *models.TutorialFeature

	for attempt := 0; attempt <= maxSkips; attempt++ {
		feat, err := o.tutorialFeaturesRepo.PickNext(ctx, skipped)
		if err != nil {
			return nil, fmt.Errorf("pick tutorial feature: %w", err)
		}
		if feat == nil {
			break
		}
		last = feat
		if attempt == maxSkips {
			break // out of skips — produce this one
		}
		stale, reason := o.featureLooksStale(ctx, feat)
		if !stale {
			return feat, nil
		}
		log.Printf("tutorial: feature %s parked for re-verification: %s", feat.FeatureKey, reason)
		if mErr := o.tutorialFeaturesRepo.MarkNeedsVerify(ctx, feat.ID, reason); mErr != nil {
			log.Printf("tutorial: MarkNeedsVerify failed (non-fatal): %v", mErr)
		}
		skipped = append(skipped, feat.FeatureKey)
	}
	return last, nil
}

// featureLooksStale asks the research agent whether the feature's menu path has
// moved. Fails OPEN in every direction: any error, empty research, or an
// unparseable answer means "not stale" — a flaky web search must never stop the
// daily clip.
func (o *Orchestrator) featureLooksStale(ctx context.Context, feat *models.TutorialFeature) (bool, string) {
	rctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	research, err := o.researchAgent.Research(rctx,
		fmt.Sprintf("Facebook Ads %s menu location %s 2026 changed moved renamed",
			feat.FeatureKey, joinPath(feat.MenuPath)))
	if err != nil || research == "" {
		return false, ""
	}
	verdict, vErr := o.tutorialVerifier(ctx, feat, research)
	if vErr != nil {
		log.Printf("tutorial: freshness verdict failed (fail-open): %v", vErr)
		return false, ""
	}
	return verdict.Moved, verdict.Reason
}

func joinPath(p []string) string {
	out := ""
	for i, s := range p {
		if i > 0 {
			out += " > "
		}
		out += s
	}
	return out
}
```

**หมายเหตุการต่อ `tutorialVerifier`:** ให้ implement เป็นเมธอดเล็กบน `Orchestrator` ที่เรียก LLM ผ่าน client เดิม (`o.criticAgent` ใช้ client ตัวไหน ให้ใช้ตัวเดียวกัน — ค้นด้วย `grep -rn "GenerateJSON" internal/agent/ | head`) โดยส่ง system prompt สั้นๆ ว่า "ตอบ JSON `{\"moved\":bool,\"reason\":\"...\"}` เท่านั้น — moved=true ก็ต่อเมื่อมีหลักฐานชัดว่าเมนูนี้ย้าย/เปลี่ยนชื่อ/ถูกยกเลิก ถ้าไม่แน่ใจให้ moved=false" และ struct:

```go
type tutorialFreshnessVerdict struct {
	Moved  bool   `json:"moved"`
	Reason string `json:"reason"`
}
```

- [ ] **Step 4: เพิ่ม `feat` param เข้าเส้นทางผลิต**

(4a) `internal/orchestrator/orchestrator.go:342` — `produceClip` เพิ่มพารามิเตอร์ท้ายสุด:

```go
func (o *Orchestrator) produceClip(ctx context.Context, q agent.GeneratedQuestion, theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string, format *models.ContentFormat, persona string, archetype models.TitleArchetype, role string, feat *models.TutorialFeature) error {
	preset := producer.PresetByKey("editorial-bold")
	switch {
	case format.FormatName == tutorialFormatName:
		preset = producer.TutorialPreset // tutorial: fixed identity, skip random pickers
	case producer.CaseFormatEnabled():
		preset = producer.CaseFilePreset
	case producer.StylePresetsEnabled():
		// ...บล็อกเดิมทั้งหมดไม่เปลี่ยน
	}
```

และใน `clipsRepo.Create` เพิ่มการเก็บ feature key — เพิ่มฟิลด์ `TutorialFeature` ใน `models.CreateClipRequest` + คอลัมน์ใน `repository/clips.go` INSERT (คอลัมน์ `tutorial_feature` สร้างแล้วใน migration 061):

```go
	featKey := ""
	if feat != nil {
		featKey = feat.FeatureKey
	}
	clip, err := o.clipsRepo.Create(ctx, models.CreateClipRequest{
		Title:           q.Question,
		Question:        q.Question,
		QuestionerName:  q.QuestionerName,
		Category:        q.Category,
		PublishDate:     &today,
		ContentFormat:   format.FormatName,
		ClipRole:        role,
		TitleArchetype:  archetype.ArchetypeName,
		AudiencePersona: persona,
		TutorialFeature: featKey,
	})
```

ท้ายฟังก์ชันส่ง `feat` ต่อ:

```go
	return o.produceClipWithID(ctx, clip.ID, q, theme, preset, scriptCfg, imageCfg, brandAliases, format, persona, archetype, role, feat)
```

(4b) `produceClipWithID` (บรรทัด 456) เพิ่มพารามิเตอร์ `feat *models.TutorialFeature` ท้ายสุดเช่นกัน และแก้ 3 จุดข้างใน:

- บรรทัด 465 → `script, err := o.generateScript(ctx, clipID, q, format, persona, archetype.Instruction, RoleInstruction(role), agent.TutorialBrief(feat), scriptCfg)`
- บรรทัด 475 → `sceneCfg, err := o.modeAgentConfig(ctx, "scene", clipMode(format.FormatName))`
- บรรทัด 481 → เปลี่ยน `""` (ที่ใส่ไว้ใน Task 8) เป็น:

```go
	sceneCount, durationSec := targetSceneCount, targetDurationSec
	if feat != nil {
		sceneCount, durationSec = tutorialSceneShape(len(feat.Steps))
	}
	scenes, err := o.sceneAgent.Generate(ctx, narration, sceneCount, durationSec, clipTheme, agent.TutorialBrief(feat), sceneCfg)
```

- บรรทัด 490 (critic) → `o.modeAgentConfig(ctx, "critic", clipMode(format.FormatName))` แทน `o.agentsRepo.GetByName(ctx, "critic")`

(4c) เพิ่ม gate ก่อนบรรทัด 601 (`return o.renderAndFinalize(...)`), หลังจาก scenes ถูก persist แล้ว:

```go
	// Tutorial gate — this clip auto-publishes, so a deterministic check replaces
	// the human reviewer. A clip that would teach a menu the catalog never
	// authorized (or the wrong number of steps) is routed to needs_review instead.
	if msg := tutorialGateFailure(scenes, feat); msg != "" {
		log.Printf("tutorial gate blocked clip %s: %s", clipID, msg)
		reviewStatus := "needs_review"
		o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{Status: &reviewStatus})
		return fmt.Errorf("tutorial gate: %s", msg)
	}
```

(4d) `generateScript` ใน `internal/orchestrator/script_debate.go:98` เพิ่มพารามิเตอร์ `brief string` ก่อน `scriptCfg` และส่งต่อเป็น ragContext:

```go
func (o *Orchestrator) generateScript(ctx context.Context, clipID string, q agent.GeneratedQuestion, format *models.ContentFormat, persona, archetypeInstr, roleInstr, brief string, scriptCfg *models.AgentConfig) (*agent.GeneratedScript, error) {
	single := func() (*agent.GeneratedScript, error) {
		if brief != "" {
			// Tutorial mode: the catalog block IS the grounding context. Skip the
			// KB/web lookup — inventing extra facts is exactly what we're preventing.
			return o.scriptAgent.GenerateWithContext(ctx, q.Question, q.QuestionerName, q.Category,
				format, persona, archetypeInstr, roleInstr, brief, "", scriptCfg)
		}
		return o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category, format, persona, archetypeInstr, roleInstr, "", scriptCfg)
	}
	if brief != "" {
		return single() // tutorial never runs the 3-lens debate — the structure is fixed
	}
	// ...ที่เหลือเหมือนเดิม
```

(4e) `caseAgentConfig` (บรรทัด 893) เปลี่ยนเป็น `modeAgentConfig`:

```go
// modeAgentConfig resolves the agent row for a role in a given content mode:
// it prefers "<name>_<mode>" and fails open to the classic row so a missing or
// disabled mode-specific row never blocks production.
func (o *Orchestrator) modeAgentConfig(ctx context.Context, name, mode string) (*models.AgentConfig, error) {
	if mode != producer.ModeClassic {
		if cfg, err := o.agentsRepo.GetByName(ctx, name+"_"+mode); err == nil && cfg.Enabled {
			return cfg, nil
		}
		log.Printf("mode %s: %s_%s row missing/disabled — falling back to %s", mode, name, mode, name)
	}
	return o.agentsRepo.GetByName(ctx, name)
}
```

อัปเดตทุกจุดที่เรียก `caseAgentConfig`: บรรทัด 306, 475, 831 → `o.modeAgentConfig(ctx, "<name>", clipMode(format.FormatName))`
(บรรทัด 831 อยู่ใน `retryFull` ซึ่งมี `clip.ContentFormat` อยู่แล้ว — ใช้ `clipMode(clip.ContentFormat)`)

(4f) `resolveFormatInfo` (Task 1) เพิ่มสาขา tutorial:

```go
func (o *Orchestrator) resolveFormatInfo(ctx context.Context, clipID string, preset producer.StylePreset) producer.FormatInfo {
	if preset.Key == producer.TutorialPreset.Key {
		return producer.FormatInfo{Mode: producer.ModeTutorial}
	}
	if preset.Key != producer.CaseFilePreset.Key {
		return producer.FormatInfo{}
	}
	// ...บล็อก case เดิมทั้งหมดไม่เปลี่ยน
}
```

(4g) `retryFull` (บรรทัด 810-852): โหลด feature กลับมาจาก `clip.TutorialFeature` และส่งเข้า `produceClipWithID`; `retryPresetForCurrentMode` เพิ่มสาขา tutorial:

```go
	var feat *models.TutorialFeature
	if clip.ContentFormat == tutorialFormatName && clip.TutorialFeature != "" {
		if f, fErr := o.tutorialFeaturesRepo.GetByKey(ctx, clip.TutorialFeature); fErr == nil {
			feat = f
		} else {
			log.Printf("retry: tutorial feature %s unavailable (%v) — clip will fail the gate and park at needs_review", clip.TutorialFeature, fErr)
		}
	}
	format, err := o.formatsRepo.GetByName(ctx, clip.ContentFormat)
	if err != nil {
		format = &models.ContentFormat{FormatName: "qa", DisplayName: "Q&A"}
	}
```

(แทนบรรทัด 839-842 เดิมที่ hard-code `"qa"` — เดิมทำให้ retry ของทุกคลิปกลายเป็น qa; ใช้ `clip.ContentFormat` ถูกต้องกว่าและจำเป็นสำหรับ tutorial)

```go
func retryPresetForCurrentMode(stored, contentFormat string) producer.StylePreset {
	if contentFormat == tutorialFormatName {
		return producer.TutorialPreset
	}
	if producer.CaseFormatEnabled() {
		return producer.CaseFilePreset
	}
	p := producer.PresetByKey(stored)
	if p.Key == producer.CaseFilePreset.Key || p.Key == producer.TutorialPreset.Key {
		return producer.PresetByKey("")
	}
	return p
}
```

(4h) `ProduceWeekly` บรรทัด 325 → ส่ง `nil` เป็น `feat`:

```go
		if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format, persona, archetype, role, nil); err != nil {
```

(4i) `resumeHyperframesProduction` บรรทัด 799 → preset ต้องตามโหมดของคลิป:

```go
	preset := producer.PresetByKey(clip.StylePreset)
```
(ไม่ต้องแก้ — `PresetByKey` resolve `"tutorial"` ได้แล้วจาก Task 7 Step 5)

- [ ] **Step 5: wire repo เข้า Orchestrator**

- เพิ่มฟิลด์ `tutorialFeaturesRepo *repository.TutorialFeaturesRepo` และ `researchAgent *agent.ResearchAgent` (ถ้ายังไม่มี) ใน struct `Orchestrator`
- เพิ่มพารามิเตอร์ในตัวสร้าง แล้วอัปเดตจุดเรียก — ค้นด้วย:

```bash
grep -rn "NewOrchestrator" cmd/ internal/
```

- เพิ่ม `TutorialFeature string` ใน `models.CreateClipRequest` และ `models.Clip` แล้วเติมคอลัมน์ `tutorial_feature` ใน INSERT/SELECT ของ `internal/repository/clips.go`

- [ ] **Step 6: รันเทสให้ผ่าน**

Run: `go build ./... && go test ./internal/...`
Expected: PASS ทั้งหมด รวมเทสเดิมของ orchestrator (`TestValidateScript`, `resume_test`, `status_gate_test`)

- [ ] **Step 7: Commit**

```bash
git add internal/orchestrator internal/models internal/repository
git commit -m "feat(tutorial): ProduceTutorial + per-clip mode resolution + fail-closed gate"
```

---

## Task 10: Scheduler action + ย้าย schedule + ตรวจของจริงก่อนเปิด

**Files:**
- Modify: `internal/scheduler/scheduler.go:199-218`
- Create: `migrations/064_tutorial_schedule.sql`
- Create: `internal/scheduler/tutorial_action_test.go`

**Interfaces:**
- Consumes: `(*Orchestrator).ProduceTutorial` (Task 9)
- Produces: schedule action `"produce_tutorial"`; content_formats row `tutorial` (enabled=FALSE); schedule row 21:00

- [ ] **Step 1: เขียนเทสที่ยังไม่ผ่าน**

สร้าง `internal/scheduler/tutorial_action_test.go`:

```go
package scheduler

import "testing"

// handlerFor ต้องรู้จัก action ใหม่ ไม่งั้น schedule row จะเงียบไม่ทำอะไรเลย
// ตรวจเฉพาะ 2 เคสที่ไม่ต้อง dereference field ใดๆ ของ Scheduler (เลี่ยง nil panic
// จาก method value ของ field ที่ยังไม่ถูก wire ในเทส)
func TestHandlerForProduceTutorial(t *testing.T) {
	s := &Scheduler{}
	if s.handlerFor("produce_tutorial") == nil {
		t.Error(`handlerFor("produce_tutorial") = nil — the schedule row would silently do nothing`)
	}
	if s.handlerFor("nonsense") != nil {
		t.Error("unknown actions must return nil")
	}
}
```

- [ ] **Step 2: รันเทสให้เห็นว่า fail**

Run: `go test ./internal/scheduler/ -run TestHandlerForProduceTutorial -v`
Expected: FAIL — `handlerFor("produce_tutorial") = nil`

- [ ] **Step 3: เพิ่ม case + แยกตัว tick ที่ใช้ร่วมกัน**

`produceAndPublish` (บรรทัด 127-176) มี pre-flight + circuit breaker + retry gate ที่ tutorial ต้องได้เหมือนกันทุกอย่าง — ห้ามเขียนใหม่ให้ขาด ให้แยกตัวกลางออกมาแล้วเรียกร่วมกัน

(3a) หลัง `case "produce_and_publish":` (บรรทัด 205-206) เพิ่ม:

```go
	case "produce_tutorial":
		return s.produceTutorial
```

(3b) เปลี่ยนหัวฟังก์ชัน `produceAndPublish` เป็นตัวห่อบางๆ แล้วย้ายเนื้อเดิม (บรรทัด 128-176) ไปเป็น `produceTick` ที่รับฟังก์ชันผลิต:

```go
// produceAndPublish and produceTutorial share one tick body: the same pre-flight,
// the same circuit breaker, and the same production gate. Only the produce call
// differs — a tutorial tick must never skip those guards.
func (s *Scheduler) produceAndPublish(ctx context.Context) error {
	return s.produceTick(ctx, "1 new clip", func(c context.Context) error {
		return s.orchestrator.ProduceWeekly(c, 1)
	})
}

// produceTutorial produces the daily tutorial clip from the catalog.
func (s *Scheduler) produceTutorial(ctx context.Context) error {
	return s.produceTick(ctx, "the daily tutorial clip", s.orchestrator.ProduceTutorial)
}

func (s *Scheduler) produceTick(ctx context.Context, what string, produce func(context.Context) error) error {
	// ...ยกเนื้อ produceAndPublish เดิมมาทั้งหมด (บรรทัด 128-176) โดยเปลี่ยน 2 จุด:
	//   log.Println("Scheduler: producing 1 new clip...")   -> log.Printf("Scheduler: producing %s...", what)
	//   if err := s.orchestrator.ProduceWeekly(ctx, 1); ... -> if err := produce(ctx); ...
	// ที่เหลือ (preflight.Run, CountConsecutiveFailed, RetryAllFailed, PublishReady,
	// DeleteOldFailed) คงเดิมทุกบรรทัด
}
```

- [ ] **Step 4: สร้าง `migrations/064_tutorial_schedule.sql`**

```sql
-- 064: tutorial content format row + ย้ายสล็อตผลิต 06:00 -> 21:00
-- content_formats.tutorial ต้อง enabled = FALSE เพื่อไม่ให้ FormatsRepo.PickNext
-- (WHERE cf.enabled = TRUE) หยิบไปใช้ในรอบผลิตปกติ 12:00/18:00 — ProduceTutorial
-- ดึงแถวนี้ด้วย GetByName ซึ่งไม่กรอง enabled
--
-- Rollback (คืนสล็อตเช้า):
--   UPDATE schedules SET name='Morning Produce & Publish', cron_expression='0 6 * * *',
--          action='produce_and_publish' WHERE action='produce_tutorial';
BEGIN;

INSERT INTO content_formats (format_name, display_name, question_instruction, script_instruction, enabled, weight)
VALUES ('tutorial', 'สอนใช้ฟีเจอร์จริง',
 $$หัวข้อมาจาก catalog tutorial_features เท่านั้น — agent ไม่ต้องคิดหัวข้อเอง$$,
 $$เขียนสคริปต์แบบคู่มือ: เปิดด้วยความเสียหาย -> สัญญา+จำนวนขั้น -> เดินขั้นตอนตามข้อมูลฟีเจอร์ -> กับดัก -> การ์ดสรุป -> ปิดแบบชวนคุย$$,
 FALSE, 1)
ON CONFLICT (format_name) DO NOTHING;

UPDATE schedules
SET name = 'Tutorial Produce & Publish',
    cron_expression = '0 21 * * *',
    action = 'produce_tutorial'
WHERE action = 'produce_and_publish' AND cron_expression = '0 6 * * *';

COMMIT;
```

- [ ] **Step 5: รันเทสทั้งหมด + build**

Run: `go build ./... && go test ./internal/...`
Expected: PASS ทั้งหมด

- [ ] **Step 6: Commit**

```bash
git add internal/scheduler migrations/064_tutorial_schedule.sql
git commit -m "feat(tutorial): produce_tutorial scheduler action + 21:00 slot"
```

- [ ] **Step 7: ตรวจของจริงก่อนปล่อย — ห้ามข้าม**

ก่อนให้ schedule ยิงเองบน prod:

1. **ตรวจ menu_path กับหน้าจอจริง** — เปิด Ads Manager / Business settings จริงแล้วไล่เช็คทั้ง 8 แถวว่าชื่อเมนูใน `ui_vocab` ตรงกับที่เห็นบนจอ แถวไหนไม่ตรงให้แก้ด้วย SQL:
   ```sql
   UPDATE tutorial_features
   SET ui_vocab = string_to_array('...|...', '|'), menu_path = ARRAY['...','...']
   WHERE feature_key = '...';
   ```
   แถวไหนไม่แน่ใจให้ปิดไปก่อน: `UPDATE tutorial_features SET enabled = FALSE WHERE feature_key = '...';`

2. **ผลิตคลิปทดสอบ 1 ตัวด้วยมือ** — ยิง endpoint ของ orchestrator (ดู `internal/handler/` ว่ามี route ไหนที่เรียก produce) หรือรันในเครื่อง แล้วดูวิดีโอที่ได้ทั้งคลิป

3. **เช็ค 5 ข้อกับคลิปทดสอบ**
   - ชื่อเมนูบนการ์ดหน้าจอตรงกับ Ads Manager จริงทุกคำ
   - จำนวนซีน `uistep` เท่ากับจำนวนขั้นใน catalog
   - ไม่มีกล่องสี่เหลี่ยม tofu (ฟอนต์ขาด glyph)
   - เสียงพากย์กับสิ่งที่เห็นบนจอตรงกัน
   - การ์ดสรุปมีครบทุกขั้น

4. **ดูสถานะคลิปใน DB** — ถ้าเป็น `needs_review` ให้อ่าน log หา `tutorial gate blocked clip` เพื่อดูว่า guard ตีเพราะอะไร (นี่คือ guard ทำงานถูก ไม่ใช่บั๊ก)

5. **เปิด schedule** — deploy แล้วรอรอบ 21:00 จริง แล้วเช็คว่าคลิปขึ้น YouTube

6. **ติดตาม 14 คลิป** — เทียบ `retention_rate` / `avg_view_percentage` ระหว่าง `content_format='tutorial'` กับคลิปแฟ้มคดี:
   ```sql
   SELECT c.content_format, COUNT(*) AS clips,
          ROUND(AVG(a.retention_rate)::numeric, 4)      AS avg_retention,
          ROUND(AVG(a.avg_view_percentage)::numeric, 2) AS avg_view_pct,
          ROUND(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY a.views)) AS median_views
   FROM clips c
   JOIN LATERAL (SELECT * FROM clip_analytics ca
                 WHERE ca.clip_id = c.id AND ca.platform = 'youtube'
                 ORDER BY ca.fetched_at DESC LIMIT 1) a ON TRUE
   WHERE c.created_at > NOW() - INTERVAL '30 days'
   GROUP BY 1 ORDER BY 1;
   ```

---

## Self-Review

**Spec coverage**

| ส่วนใน spec | งานที่ทำ |
|---|---|
| §3 โหมดต่อคลิป derive จาก content_format | Task 1 (`FormatInfo`), Task 9 (`clipMode`) |
| §4 กลุ่มเป้าหมาย / หัวข้อ 3 กลุ่ม | Task 3 (seed 8 แถวครอบทั้ง 3 กลุ่ม) |
| §5 catalog + picker + research freshness | Task 2, Task 3, Task 9 (`pickVerifiedFeature`) |
| §6 โครงคลิป + จำนวนซีนตาม steps | Task 8 (prompt), Task 9 (`tutorialSceneShape`) |
| §7 layout `uistep` + สัญญา content | Task 4 (validator), Task 5 (mapping), Task 6 (renderer) |
| §8 `TutorialPreset` | Task 7 |
| §9 ภาพ AI 1 ใบ | Task 1 + Task 7 (`imageScenesForMode`) |
| §10 guard 3 ชั้น | Task 4 + Task 9 (ชั้น 1-2), Task 8 (ชั้น 3 `critic_tutorial` + visual_qa) |
| §11 schedule 21:00 + rollback + วัดผล | Task 10 |
| §13 ไม่แตะคลิปแฟ้มคดี | Global Constraints + เทส `TestRenderCaseFormat*` ใน Task 1/6 |

**ช่องว่างที่รู้ตัวและตัดสินใจแล้ว**
- **`tutorialVerifier` เขียนเป็นคำอธิบายไม่ใช่โค้ดเต็ม** ใน Task 9 Step 3 เพราะ LLM client ที่ต้องใช้ขึ้นกับว่า `Orchestrator` ถือ client ตัวไหนอยู่ ผู้ทำต้อง `grep -rn "GenerateJSON" internal/agent/` แล้วลอกท่าเดิม — สัญญาที่ต้องได้คือ `(tutorialFreshnessVerdict, error)` และ **fail-open ทุกกรณี**
- **seed 8 แถวไม่ใช่ 50** ตามที่ระบุใน Task 3 — ตัดสินใจโดยเจตนา ไม่ใช่ placeholder
- **ยังไม่มี UI หน้าจัดการ catalog** ตาม spec §13 — แก้ผ่าน SQL

**ความสม่ำเสมอของชนิด/ชื่อ** — ตรวจแล้ว: `FormatInfo`/`Mode` (Task 1) ใช้เหมือนกันใน Task 7/9 · `ContentUIPanel`/`ContentUIItem`/`ContentUIField` (Task 5) ตรงกับ JSON key `panel`/`items`/`field` ที่ template อ่าน (Task 6) และตรงกับ `agent.UIPanel` ที่ validator parse (Task 4) · `StepTotal` → JSON `stepTotal` → `sc.stepTotal` ใน JS · `tutorialFormatName = "tutorial"` ใช้เป็นทั้ง `content_formats.format_name`, `TutorialPreset.Key` และ suffix ของ agent row `*_tutorial` สอดคล้องกับ `modeAgentConfig(name, mode)`
