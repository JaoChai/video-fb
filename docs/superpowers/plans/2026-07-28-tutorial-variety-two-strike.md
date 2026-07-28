# Tutorial: คืนความหลากหลาย + กันคลังยุบ (two-strike)

## บริบท

คลิปสอน 21:00 ของ 26 กับ 27 ก.ค. เป็นหัวข้อเดียวกัน และ **บทเกือบเหมือนกัน ~90%**
ทั้งที่โมเดลเขียนใหม่คนละรอบ:

> 26: "บัญชีคุณอาจกำลังแย่ง auction กับตัวเองอยู่ตอนนี้ จ่าย CPM แพงขึ้นฟรีๆ โดยไม่รู้ตัว"
> 27: "บัญชีคุณอาจกำลังแย่ง auction กับตัวเอง แล้วจ่าย CPM แพงขึ้นโดยไม่รู้ตัว"

สาเหตุ: โหมด tutorial **ข้ามกลไกความหลากหลายทั้งหมด** ที่คลิปปกติมี เพราะมันข้าม
question agent ไปเลย

| กลไก | คลิปปกติ | tutorial ตอนนี้ |
|---|---|---|
| archetype | `titleArchetypesRepo.PickNext` | ส่ง `models.TitleArchetype{}` (ว่าง) |
| persona | สุ่มจาก setting `audience_personas` (5 ตัว) | ใช้ `audience_persona` ก้อนเดิมตายตัว |
| ห้ามซ้ำมุมเดิม | question agent ได้ `PreviousTopics` | ไม่มีอะไรเลย |

และการพักหัวข้อยังไวเกิน: พักไป 7 ครั้งจาก ~5 รอบผลิต ถ้าอัตรานี้คงอยู่ คลัง 18
จะยุบชนพื้น (`TutorialMinPool` = 3) ใน ~10 วัน แล้ววนอยู่ 3 หัวข้อ = ซ้ำทุก 3 วัน

## Global Constraints

- **ห้ามแตะตะแกรง `ui_vocab`** (`tutorialGateFailure`) และห้ามให้โมเดลได้สิทธิ์
  แต่งชื่อเมนูเอง — นี่คือสิ่งเดียวที่ทำให้ auto-publish ปลอดภัย (spec §10)
- **scene agent ต้องได้ brief เดิมไม่เปลี่ยน** — `agent.TutorialBrief(feat)` ที่
  `orchestrator.go:504` ห้ามมีข้อความมุมเก่าปน (มันสร้าง UI mock ไม่ใช่บท)
- migration ใหม่ = ไฟล์ `069_*.sql` (068 ถูกใช้แล้ว) ต้องหุ้ม `BEGIN; … COMMIT;`
  เอง (RunMigrations ไม่หุ้มให้) และต้อง idempotent (`IF NOT EXISTS`)
- ห้ามเพิ่ม env var ใหม่
- คอมเมนต์ภาษาไทยตามสไตล์ไฟล์เดิม อธิบาย "ทำไม" ไม่ใช่ "ทำอะไร"
- ทุก task ต้องผ่าน: `go build ./...` · `go vet ./internal/...` ·
  `go test ./internal/...` (รันด้วย `dangerouslyDisableSandbox` เพราะ sandbox
  บล็อก go build cache)
- ห้าม reformat โค้ดข้างเคียงที่ไม่เกี่ยว (`internal/router/router.go` มี gofmt
  drift เดิมอยู่ อย่าไปแก้)

## Task 1 — two-strike + ยกพื้นคลัง

**ไฟล์:** `migrations/069_tutorial_two_strike.sql` (ใหม่),
`internal/repository/tutorial_features.go`, `internal/orchestrator/tutorial.go`

### 1a. Migration

```sql
ALTER TABLE tutorial_features ADD COLUMN IF NOT EXISTS flagged_at TIMESTAMPTZ;
```

คอมเมนต์หัวไฟล์ต้องอธิบายว่า: คำตัดสิน "เมนูย้าย" จาก LLM แม่นไม่พอที่จะพักหัวข้อ
ตั้งแต่ครั้งแรก (ข้อมูลจริง: พัก 7 ครั้งจาก ~5 รอบผลิต) `flagged_at` = เวลาที่โดน
ตัดสินครั้งแรก ต้องโดนซ้ำอีกครั้งภายในหน้าต่างเดิมถึงจะพักจริง

### 1b. Repository

- `TutorialMinPool`: `3` → `10` (แก้คอมเมนต์ให้ตรง: พื้นนี้กัน "คลังยุบจนวนถี่"
  ไม่ใช่แค่กัน "ไม่มีคลิป")
- เพิ่ม `const tutorialStrikeWindowDays = 30`
- เพิ่ม type ผลลัพธ์การพัก 3 ค่า ชื่อสื่อความหมาย เช่น:
  ```go
  type ParkOutcome string
  const (
      ParkFirstStrike ParkOutcome = "first_strike" // บันทึกไว้ ข้ามรอบนี้ ยังไม่พัก
      ParkedForVerify ParkOutcome = "parked"       // โดนซ้ำ → พัก 14 วัน
      ParkRefusedFloor ParkOutcome = "floor"       // คลังเหลือน้อย → ห้ามพัก
  )
  ```
- เปลี่ยน `Park(ctx, id, reason string) (bool, error)` →
  `Park(ctx, id, reason string) (ParkOutcome, error)` ด้วยตรรกะ 2 statement:

  1. **strike แรก** — ยิงก่อนเสมอ:
     ```sql
     UPDATE tutorial_features
     SET flagged_at = NOW(), verify_reason = $2
     WHERE id = $1
       AND (flagged_at IS NULL OR flagged_at < NOW() - make_interval(days => $3))
     ```
     `RowsAffected() > 0` → คืน `ParkFirstStrike`
  2. **strike ที่สอง** — ถ้า statement แรกไม่โดนแถวไหน แปลว่าเคยโดนธงมาแล้วใน
     หน้าต่าง 30 วัน → พักจริง **พร้อมพื้นในคำสั่งเดียวกัน** (ห้ามนับแยก):
     ```sql
     UPDATE tutorial_features
     SET verify_reason = $2, parked_until = NOW() + make_interval(days => $3),
         flagged_at = NULL
     WHERE id = $1
       AND (SELECT COUNT(*) FROM tutorial_features WHERE <tutorialAvailableWhere>) > $4
     ```
     `RowsAffected() > 0` → `ParkedForVerify` ไม่งั้น `ParkRefusedFloor`
     (`flagged_at = NULL` เพื่อให้เริ่มนับ strike ใหม่หลังการพักหมดอายุ)
- `Unpark` ต้องล้าง `flagged_at` ด้วย (คนตรวจด้วยตาแล้วว่าเมนูยังใช้ได้ =
  ล้างประวัติธง ไม่ใช่ปล่อยให้ธงเก่าค้างแล้วโดนพักทันทีที่โดนธงอีกครั้งเดียว)

### 1c. Orchestrator `pickVerifiedFeature`

แทนที่บล็อก `parked, pErr := …` ปัจจุบัน ด้วยการแยกตาม outcome:

| outcome | ทำอะไร |
|---|---|
| `ParkFirstStrike` | log ว่าเป็นธงใบแรก, `skipped = append(...)`, วนต่อ (ไม่ผลิตตัวนี้) |
| `ParkedForVerify` | log ว่าพักแล้ว, `skipped = append(...)`, วนต่อ |
| `ParkRefusedFloor` | log ว่าคลังชนพื้น, `return feat, nil` (ผลิตตัวนี้) |
| error | log non-fatal แล้วทำเหมือน `ParkRefusedFloor` (fail-open เดิม) |

### 1d. Tests

- `TestParkOutcomesAreDistinct` — ค่า 3 ตัวไม่ซ้ำกันและไม่ใช่ค่าว่าง
- ขยาย `TestAvailableFilterExpiresInsteadOfLatching` (หรือเพิ่มเทสต์ใหม่) ให้
  ยืนยันว่า `TutorialMinPool >= 10`
- เทสต์ใหม่ยืนยันว่า SQL ของ strike แรกอ้าง `flagged_at` และ SQL ของการพักมี
  ทั้ง `parked_until` และ subquery นับพื้น — ถ้าต้องแยก SQL ออกมาเป็น const
  ระดับ package เพื่อให้เทสต์อ่านได้ ให้ทำ

## Task 2 — คืนความหลากหลายให้คลิปสอน

**ไฟล์:** `internal/orchestrator/tutorial.go`, `internal/orchestrator/orchestrator.go`,
`internal/agent/tutorial.go`, `internal/repository/tutorial_features.go`

### 2a. หมุน archetype (ใน `ProduceTutorial`)

ตอนนี้ส่ง `models.TitleArchetype{}` เข้า `produceClip` (บรรทัด ~142) เปลี่ยนเป็นหยิบ
จาก `o.titleArchetypesRepo.PickNext(ctx)` แบบ fail-open: error หรือ nil → ใช้
`models.TitleArchetype{}` เหมือนเดิม (ห้ามทำให้รอบผลิตล้มเพราะเรื่องนี้)

### 2b. หมุน persona (ใน `ProduceTutorial`)

ตอนนี้: `persona, _ := o.settingsRepo.Get(ctx, "audience_persona")`
เปลี่ยนเป็นตรรกะเดียวกับ `orchestrator.go:193-199` เป๊ะ — อ่าน `audience_personas`
(JSON array) → `PickPersona(personas, rng)` → ถ้า unmarshal ไม่ได้หรือ list ว่าง
ค่อย fallback ไป `audience_persona`
rng สร้างแบบเดียวกับ `orchestrator.go:158`:
`rand.New(rand.NewSource(time.Now().UnixNano()))`

### 2c. บอกโมเดลว่ามุมไหนเคยใช้ไปแล้ว

**repository** — เพิ่มเมธอดบน `TutorialFeaturesRepo`:
```go
// RecentAngles คืน youtube_title ของคลิปก่อนหน้าที่สอนฟีเจอร์เดียวกัน
func (r *TutorialFeaturesRepo) RecentAngles(ctx context.Context, featureKey string, limit int) ([]string, error)
```
SQL: join `clips` c กับ `clip_metadata` m บน `m.clip_id = c.id`
where `c.tutorial_feature = $1` และ `m.youtube_title <> ''`
order by `c.created_at DESC` limit `$2`
คืน `[]string{}` (ไม่ใช่ nil) เมื่อไม่มีแถว

**agent** — เพิ่มฟังก์ชันใหม่ใน `internal/agent/tutorial.go` (อย่าแก้ signature ของ
`TutorialBrief` เพราะ scene agent ใช้ตัวเดียวกัน):
```go
// TutorialAvoidRepeatBlock …
func TutorialAvoidRepeatBlock(previousAngles []string) string
```
- ลิสต์ว่าง → คืน `""`
- ไม่ว่าง → บล็อกภาษาไทยที่ระบุ: มุม/hook ที่เคยใช้กับฟีเจอร์นี้ (bullet ตามลิสต์),
  สั่งให้เลือกมุมเปิดและสถานการณ์ตัวอย่าง**คนละแบบ**, และย้ำว่า
  **ขั้นตอนกับชื่อเมนูต้องเหมือนเดิมเป๊ะ ห้ามเปลี่ยนเพื่อให้ดูใหม่**

**orchestrator** — `produceClipWithID` บรรทัด ~484 (ทางเข้า script agent เท่านั้น):
```go
agent.TutorialBrief(feat) + o.tutorialAvoidRepeat(ctx, feat)
```
เพิ่มเมธอด helper `tutorialAvoidRepeat(ctx, feat) string` บน Orchestrator:
`feat == nil` → `""`; query `RecentAngles(ctx, feat.FeatureKey, 5)`;
error → log non-fatal แล้วคืน `""`; ไม่งั้นคืน `agent.TutorialAvoidRepeatBlock(...)`

**บรรทัด ~504 (scene agent) ห้ามแตะ** — ดู Global Constraints

### 2d. Tests

- `TestTutorialAvoidRepeatBlockEmpty` — ลิสต์ว่าง/nil → `""`
- `TestTutorialAvoidRepeatBlockListsAngles` — มีมุมเก่าครบทุกตัวในผลลัพธ์ และมี
  ข้อความห้ามเปลี่ยนขั้นตอน/ชื่อเมนู
- เทสต์ยืนยันว่าบล็อกมุมเก่า **ไม่** ถูกต่อเข้ากับ brief ของ scene agent
  (เช่น ตรวจว่า `TutorialBrief` เดิมไม่มีคำว่ามุมเก่าอยู่ในผลลัพธ์)

## ตรวจของจริงหลังจบ (ผู้คุมงานทำเอง ไม่ใช่ subagent)

1. ยิง migration ชุดจริงผ่าน runner ตัวจริงบน Neon branch แยก
2. จำลอง two-strike ด้วย SQL: ธงใบแรก → คลังไม่ลด · ธงใบสอง → พัก ·
   ยิงจนชนพื้น → คลังไม่ต่ำกว่า 10
3. เช็คว่า `RecentAngles` คืน youtube_title จริงของ 2 คลิปที่ซ้ำกัน
