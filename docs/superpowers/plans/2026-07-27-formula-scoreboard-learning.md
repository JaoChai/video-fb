# Formula Scoreboard Learning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ให้ยอดวิว/retention จริงป้อนกลับเข้าระบบสองทาง — หมุน `weight` ของ `content_formats`/`topic_categories` แบบ deterministic และป้อนกระดานคะแนนให้ agent ที่เขียน `insights`/`skills`

**Architecture:** SQL คำนวณสถิติดิบต่อสูตร → ฟังก์ชัน pure ใน Go คำนวณคะแนน+shrinkage → snapshot ลง `formula_scores` → tuner (pure) แปลงคะแนนเป็น `weight` ใหม่ภายใต้เบรก 4 ชั้น → เขียน audit ก่อน apply เสมอ การเลือกสูตรตอนผลิตไม่ต้องแก้เลย เพราะ `PickNext` ใช้ least-used/weight อยู่แล้ว

**Tech Stack:** Go 1.x + pgx/v5 + chi router + robfig/cron, Postgres (Neon), React + TanStack Query (frontend)

## Global Constraints

- Spec ต้นทาง: `docs/superpowers/specs/2026-07-27-formula-scoreboard-learning-design.md` — ค่าคงที่ทุกตัวในแผนนี้คัดมาจาก spec ห้ามเปลี่ยนเอง
- หน้าต่างวัดผล **60 วัน**; เกณฑ์ขั้นต่ำก่อนขยับ weight **n ≥ 8**; ความเร็วเคลื่อน **alpha = 0.25**/สัปดาห์; พื้น **0.5 × uniform**, เพดาน **2 × uniform** โดย `uniform = 1 / จำนวนสูตรที่ enabled ในมิตินั้น`
- น้ำหนักคะแนน: `0.5 × median_pct + 0.3 × retention_norm + 0.2 × (1 − flop_rate)`; ถ้าไม่มี retention → renormalize เป็น `0.714 / 0.286` (ห้ามแทนด้วย 0)
- Shrinkage: `score_final = (n × score_raw + 5 × 0.5) / (n + 5)`
- แหล่ง retention คือ **`clip_analytics.avg_view_percentage`** เท่านั้น (ไม่ใช่ `retention_rate` ซึ่งคนละสเกล)
- **ห้ามแตะคอลัมน์ `enabled` ของสูตรใดๆ** ระบบ retire สูตรเองไม่ได้
- **audit ต้องเขียนสำเร็จก่อน apply เสมอ** ถ้าเขียน audit ไม่ได้ ห้ามเขียน weight
- มิติที่อยู่ในสโคป: `content_format`, `category` (หมุน weight ได้) และ `style_preset` (วัดผลอย่างเดียว ห้ามหมุน — ดู spec หัวข้อ 7)
- `title_archetype`, `clip_role`, `audience_persona` **อยู่นอกสโคป** ห้ามแตะ
- Go list endpoint ต้อง init `out := []T{}` ไม่ใช่ `var out []T` มิฉะนั้น JSON เป็น `null` แล้วหน้าเว็บพัง
- `RunMigrations` ไม่หุ้ม transaction ให้ — ไฟล์ migration ต้องมี `BEGIN;` / `COMMIT;` เอง
- Module path: `github.com/jaochai/video-fb`
- รันเทสต์ทั้งหมดด้วย `go test ./...` ก่อน commit ทุกครั้ง

---

### Task 1: ฟังก์ชันคะแนน (pure)

**Files:**
- Create: `internal/scoreboard/score.go`
- Test: `internal/scoreboard/score_test.go`

**Interfaces:**
- Consumes: ไม่มี (task แรก)
- Produces:
  - `type Stat struct { Dimension, Value, Platform string; N int; MedianPct, MedianRetention, FlopRate float64 }`
  - `type Score struct { Stat; ScoreRaw, ScoreFinal float64 }`
  - `func MinMaxNorm(vals []float64) []float64`
  - `func ScoreRaw(medianPct, retentionNorm, flopRate float64, hasRetention bool) float64`
  - `func Shrink(scoreRaw float64, n int) float64`
  - `func ScoreAll(stats []Stat) []Score`

- [ ] **Step 1: เขียนเทสต์ที่ต้อง fail**

สร้าง `internal/scoreboard/score_test.go`:

```go
package scoreboard

import (
	"math"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestMinMaxNorm(t *testing.T) {
	got := MinMaxNorm([]float64{0.2, 0.4, 0.6})
	want := []float64{0, 0.5, 1}
	for i := range want {
		if !almost(got[i], want[i]) {
			t.Errorf("MinMaxNorm[%d] = %v, want %v", i, got[i], want[i])
		}
	}
	// ทุกค่าเท่ากัน → 0.5 ทั้งหมด (ห้ามหาร 0)
	flat := MinMaxNorm([]float64{0.3, 0.3})
	for i, v := range flat {
		if !almost(v, 0.5) {
			t.Errorf("flat[%d] = %v, want 0.5", i, v)
		}
	}
	if len(MinMaxNorm(nil)) != 0 {
		t.Error("MinMaxNorm(nil) ต้องคืน slice ว่าง")
	}
}

func TestScoreRawWithRetention(t *testing.T) {
	// 0.5*0.8 + 0.3*1.0 + 0.2*(1-0.25) = 0.4 + 0.3 + 0.15 = 0.85
	got := ScoreRaw(0.8, 1.0, 0.25, true)
	if !almost(got, 0.85) {
		t.Errorf("ScoreRaw = %v, want 0.85", got)
	}
}

func TestScoreRawWithoutRetentionRenormalizes(t *testing.T) {
	// ไม่มี retention → น้ำหนักเหลือ 0.5/0.7 และ 0.2/0.7
	// (0.5*0.8 + 0.2*(1-0.25)) / 0.7 = (0.4 + 0.15)/0.7 = 0.7857142857...
	got := ScoreRaw(0.8, 0, 0.25, false)
	want := 0.55 / 0.7
	if !almost(got, want) {
		t.Errorf("ScoreRaw without retention = %v, want %v", got, want)
	}
	// ห้ามเท่ากับกรณีใส่ retention = 0 (นั่นคือบั๊กที่ทำให้ case-file ตกรอบ)
	if almost(got, ScoreRaw(0.8, 0, 0.25, true)) {
		t.Error("ไม่มี retention ต้องไม่เท่ากับ retention = 0")
	}
}

func TestShrink(t *testing.T) {
	// n=1: (1*1.0 + 5*0.5)/(1+5) = 3.5/6 = 0.58333...
	if got := Shrink(1.0, 1); !almost(got, 3.5/6) {
		t.Errorf("Shrink(1.0, 1) = %v, want %v", got, 3.5/6)
	}
	// n=75: เกือบเท่าเดิม
	if got := Shrink(1.0, 75); math.Abs(got-1.0) > 0.07 {
		t.Errorf("Shrink(1.0, 75) = %v, ควรใกล้ 1.0", got)
	}
	// n=0 ต้องได้ 0.5 พอดี ไม่ใช่ NaN
	if got := Shrink(0.9, 0); !almost(got, 0.5) {
		t.Errorf("Shrink(_, 0) = %v, want 0.5", got)
	}
}

func TestScoreAllNormalizesWithinDimensionAndPlatform(t *testing.T) {
	stats := []Stat{
		{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30, MedianPct: 0.53, MedianRetention: 0.71, FlopRate: 0.31},
		{Dimension: "content_format", Value: "news", Platform: "youtube", N: 25, MedianPct: 0.53, MedianRetention: 0.75, FlopRate: 0.16},
		// คนละแพลตฟอร์ม: ต้อง normalize แยกกลุ่ม ไม่ปนกับ youtube
		{Dimension: "content_format", Value: "tips", Platform: "tiktok", N: 17, MedianPct: 0.58, MedianRetention: 0, FlopRate: 0.18},
	}
	got := ScoreAll(stats)
	if len(got) != 3 {
		t.Fatalf("ScoreAll คืน %d แถว, want 3", len(got))
	}
	byKey := map[string]Score{}
	for _, s := range got {
		byKey[s.Value+"|"+s.Platform] = s
	}
	// news flop ต่ำกว่าและ retention สูงกว่า qa ที่ median_pct เท่ากัน → ต้องชนะ
	if byKey["news|youtube"].ScoreFinal <= byKey["qa|youtube"].ScoreFinal {
		t.Errorf("news (%v) ต้องมากกว่า qa (%v)", byKey["news|youtube"].ScoreFinal, byKey["qa|youtube"].ScoreFinal)
	}
	// tiktok อยู่กลุ่มเดียวโดดๆ และไม่มี retention → ต้องไม่ NaN และอยู่ในช่วง 0..1
	tips := byKey["tips|tiktok"].ScoreFinal
	if math.IsNaN(tips) || tips < 0 || tips > 1 {
		t.Errorf("tips score = %v ต้องอยู่ใน 0..1 และไม่ NaN", tips)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่า fail**

Run: `go test ./internal/scoreboard/ -v`
Expected: FAIL — `undefined: MinMaxNorm` (แพ็กเกจยังไม่มี)

- [ ] **Step 3: เขียน implementation ขั้นต่ำ**

สร้าง `internal/scoreboard/score.go`:

```go
// Package scoreboard คำนวณ "กระดานคะแนนสูตร": คะแนนของแต่ละสูตรที่ใช้ผลิตคลิป
// (content_format / category / style_preset) จากผลลัพธ์จริงบนแพลตฟอร์ม
// ฟังก์ชันในไฟล์นี้เป็น pure ทั้งหมด — ไม่แตะ DB ไม่แตะเวลา — เพื่อให้ทดสอบได้ตรงๆ
package scoreboard

// น้ำหนักคะแนนตาม spec: views 0.5, retention 0.3, ความไม่ flop 0.2
const (
	weightPct       = 0.5
	weightRetention = 0.3
	weightFlop      = 0.2

	// shrinkStrength คือจำนวนคลิป "สมมติ" ที่ค่ากลาง 0.5 ถ่วงไว้ ยิ่ง n จริงน้อย
	// คะแนนยิ่งถูกดึงเข้าหากลาง — กันสูตรที่มี 1-2 คลิปเด้งไปสุดขั้ว
	shrinkStrength = 5.0
	shrinkPrior    = 0.5
)

// Stat คือสถิติดิบของหนึ่งสูตรบนหนึ่งแพลตฟอร์ม MedianRetention = 0 หมายถึง
// "ไม่มีข้อมูล" (TikTok ไม่รายงาน หรือคลิปใหม่ที่ YouTube ยังไม่ออกรายงาน)
// ไม่ได้แปลว่า retention แย่ — ตัวเรียกต้องแยกสองกรณีนี้ให้ออก
type Stat struct {
	Dimension       string
	Value           string
	Platform        string
	N               int
	MedianPct       float64
	MedianRetention float64
	FlopRate        float64
}

// Score คือ Stat ที่คำนวณคะแนนแล้ว ScoreRaw ยังไม่ผ่าน shrinkage, ScoreFinal ผ่านแล้ว
// เก็บทั้งคู่เพื่อให้ debug ได้ว่าคะแนนต่ำเพราะผลงานจริงหรือเพราะข้อมูลน้อย
type Score struct {
	Stat
	ScoreRaw   float64
	ScoreFinal float64
}

// MinMaxNorm ยืดค่าชุดหนึ่งให้อยู่ในช่วง 0..1 ถ้าทุกค่าเท่ากัน (หรือมีค่าเดียว)
// จะคืน 0.5 ทั้งหมด เพราะไม่มีข้อมูลพอจะบอกว่าใครดีกว่าใคร
func MinMaxNorm(vals []float64) []float64 {
	out := make([]float64, len(vals))
	if len(vals) == 0 {
		return out
	}
	min, max := vals[0], vals[0]
	for _, v := range vals[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	if max == min {
		for i := range out {
			out[i] = 0.5
		}
		return out
	}
	for i, v := range vals {
		out[i] = (v - min) / (max - min)
	}
	return out
}

// ScoreRaw รวมสามสัญญาณเป็นคะแนนเดียว 0..1 เมื่อ hasRetention = false จะตัดพจน์
// retention ทิ้งแล้วปรับสัดส่วนที่เหลือให้รวมเป็น 1 (ไม่ใช่แทนค่าด้วย 0 ซึ่งจะ
// ลงโทษสูตรที่ยังไม่มีรายงาน retention ทั้งที่ยอดวิวอาจดี)
func ScoreRaw(medianPct, retentionNorm, flopRate float64, hasRetention bool) float64 {
	if hasRetention {
		return weightPct*medianPct + weightRetention*retentionNorm + weightFlop*(1-flopRate)
	}
	denom := weightPct + weightFlop
	return (weightPct*medianPct + weightFlop*(1-flopRate)) / denom
}

// Shrink ดึงคะแนนเข้าหาค่ากลาง 0.5 ตามจำนวนตัวอย่างที่มี
func Shrink(scoreRaw float64, n int) float64 {
	return (float64(n)*scoreRaw + shrinkStrength*shrinkPrior) / (float64(n) + shrinkStrength)
}

// ScoreAll คำนวณคะแนนให้ทุก Stat โดย normalize retention ภายในกลุ่ม
// (dimension, platform) เดียวกัน — เทียบ retention ข้ามแพลตฟอร์มไม่ได้เพราะ
// สเกลต่างกัน และเทียบข้ามมิติไม่มีความหมาย
func ScoreAll(stats []Stat) []Score {
	out := make([]Score, len(stats))
	groups := map[string][]int{}
	for i, s := range stats {
		out[i] = Score{Stat: s}
		groups[s.Dimension+"|"+s.Platform] = append(groups[s.Dimension+"|"+s.Platform], i)
	}

	for _, idxs := range groups {
		// normalize เฉพาะสมาชิกที่มี retention จริง
		var withRet []int
		var vals []float64
		for _, i := range idxs {
			if stats[i].MedianRetention > 0 {
				withRet = append(withRet, i)
				vals = append(vals, stats[i].MedianRetention)
			}
		}
		norm := MinMaxNorm(vals)
		normByIdx := map[int]float64{}
		for k, i := range withRet {
			normByIdx[i] = norm[k]
		}

		for _, i := range idxs {
			retNorm, has := normByIdx[i]
			out[i].ScoreRaw = ScoreRaw(stats[i].MedianPct, retNorm, stats[i].FlopRate, has)
			out[i].ScoreFinal = Shrink(out[i].ScoreRaw, stats[i].N)
		}
	}
	return out
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/scoreboard/ -v`
Expected: PASS ทั้ง 5 เทสต์

- [ ] **Step 5: Commit**

```bash
git add internal/scoreboard/score.go internal/scoreboard/score_test.go
git commit -m "feat(scoreboard): ฟังก์ชันคะแนนสูตร + shrinkage (pure)"
```

---

### Task 2: ตัวหมุนน้ำหนัก (pure)

**Files:**
- Create: `internal/scoreboard/tune.go`
- Test: `internal/scoreboard/tune_test.go`

**Interfaces:**
- Consumes: `Score` จาก Task 1
- Produces:
  - `type Combined struct { Value string; ScoreFinal float64; N int }`
  - `func CombinePlatforms(scores []Score, dimension string) []Combined`
  - `func TuneWeights(current map[string]int, combined []Combined, minN int, alpha float64) map[string]int`

- [ ] **Step 1: เขียนเทสต์ที่ต้อง fail**

สร้าง `internal/scoreboard/tune_test.go`:

```go
package scoreboard

import "testing"

func sum(m map[string]int) int {
	t := 0
	for _, v := range m {
		t += v
	}
	return t
}

func TestCombinePlatformsWeightsByN(t *testing.T) {
	scores := []Score{
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30}, ScoreFinal: 0.8},
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "tiktok", N: 10}, ScoreFinal: 0.4},
		{Stat: Stat{Dimension: "category", Value: "payment", Platform: "youtube", N: 9}, ScoreFinal: 0.9},
	}
	got := CombinePlatforms(scores, "content_format")
	if len(got) != 1 {
		t.Fatalf("คืน %d รายการ, want 1 (กรองมิติอื่นออก)", len(got))
	}
	// (30*0.8 + 10*0.4) / 40 = 0.7
	if got[0].ScoreFinal < 0.699 || got[0].ScoreFinal > 0.701 {
		t.Errorf("ScoreFinal = %v, want 0.7", got[0].ScoreFinal)
	}
	if got[0].N != 40 {
		t.Errorf("N = %d, want 40", got[0].N)
	}
}

func TestTuneWeightsMovesOnlyAQuarterOfTheWay(t *testing.T) {
	current := map[string]int{"qa": 25, "news": 25, "tips": 25, "case_story": 25}
	combined := []Combined{
		{Value: "qa", ScoreFinal: 0.9, N: 30},
		{Value: "news", ScoreFinal: 0.3, N: 30},
		{Value: "tips", ScoreFinal: 0.5, N: 30},
		{Value: "case_story", ScoreFinal: 0.5, N: 30},
	}
	got := TuneWeights(current, combined, 8, 0.25)
	if sum(got) != 100 {
		t.Errorf("ผลรวม = %d, want 100", sum(got))
	}
	// qa ชนะ ต้องขึ้น แต่ห้ามพุ่งเกิน 25 + (เพดาน 50 - 25)*0.25 = 31.25 → ปัดแล้วไม่เกิน 32
	if got["qa"] <= 25 || got["qa"] > 32 {
		t.Errorf("qa = %d, ต้องมากกว่า 25 แต่ไม่เกิน 32 (เคลื่อน 25%%)", got["qa"])
	}
	if got["news"] >= 25 {
		t.Errorf("news = %d, ต้องลดลงจาก 25", got["news"])
	}
}

func TestTuneWeightsRespectsFloorAndCeiling(t *testing.T) {
	current := map[string]int{"a": 25, "b": 25, "c": 25, "d": 25}
	// a ชนะขาด อีกสามตัวศูนย์ — วิ่งซ้ำจนลู่เข้าที่ ต้องไม่มีใครหลุดพื้น 12.5%
	combined := []Combined{
		{Value: "a", ScoreFinal: 1.0, N: 50},
		{Value: "b", ScoreFinal: 0.01, N: 50},
		{Value: "c", ScoreFinal: 0.01, N: 50},
		{Value: "d", ScoreFinal: 0.01, N: 50},
	}
	w := current
	for i := 0; i < 40; i++ {
		w = TuneWeights(w, combined, 8, 0.25)
	}
	if sum(w) != 100 {
		t.Errorf("ผลรวมหลังลู่เข้า = %d, want 100", sum(w))
	}
	// พื้น = 0.5 * (1/4) = 12.5% → อย่างน้อย 12 หลังปัดเศษ
	for _, k := range []string{"b", "c", "d"} {
		if w[k] < 12 {
			t.Errorf("%s = %d หลุดพื้น 12.5%%", k, w[k])
		}
	}
	// เพดาน = 2 * 25% = 50%
	if w["a"] > 50 {
		t.Errorf("a = %d ทะลุเพดาน 50", w["a"])
	}
}

func TestTuneWeightsFreezesLowSampleValues(t *testing.T) {
	current := map[string]int{"qa": 40, "news": 40, "tutorial": 20}
	combined := []Combined{
		{Value: "qa", ScoreFinal: 0.9, N: 30},
		{Value: "news", ScoreFinal: 0.3, N: 30},
		{Value: "tutorial", ScoreFinal: 0.99, N: 2}, // n < 8 → ตรึง ห้ามได้ประโยชน์จากคะแนนสูง
	}
	got := TuneWeights(current, combined, 8, 0.25)
	if got["tutorial"] != 20 {
		t.Errorf("tutorial = %d, ต้องถูกตรึงไว้ที่ 20", got["tutorial"])
	}
	if sum(got) != 100 {
		t.Errorf("ผลรวม = %d, want 100", sum(got))
	}
}

func TestTuneWeightsEqualScoresStayUniform(t *testing.T) {
	current := map[string]int{"a": 33, "b": 33, "c": 34}
	combined := []Combined{
		{Value: "a", ScoreFinal: 0.5, N: 20},
		{Value: "b", ScoreFinal: 0.5, N: 20},
		{Value: "c", ScoreFinal: 0.5, N: 20},
	}
	got := TuneWeights(current, combined, 8, 0.25)
	for k, v := range got {
		if v < 32 || v > 35 {
			t.Errorf("%s = %d, คะแนนเท่ากันต้องอยู่ใกล้ uniform", k, v)
		}
	}
	if sum(got) != 100 {
		t.Errorf("ผลรวม = %d, want 100", sum(got))
	}
}

func TestTuneWeightsNoScoresIsNoOp(t *testing.T) {
	current := map[string]int{"a": 60, "b": 40}
	got := TuneWeights(current, nil, 8, 0.25)
	if got["a"] != 60 || got["b"] != 40 {
		t.Errorf("ไม่มีคะแนน ต้องคืนค่าเดิม ได้ %v", got)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่า fail**

Run: `go test ./internal/scoreboard/ -run TestTune -v`
Expected: FAIL — `undefined: TuneWeights`

- [ ] **Step 3: เขียน implementation ขั้นต่ำ**

สร้าง `internal/scoreboard/tune.go`:

```go
package scoreboard

import "sort"

// weightScale คือสเกลที่เก็บใน DB: ผลรวม weight ของสูตรที่ enabled ในมิติหนึ่ง = 100
// เลือกจำนวนเต็มฐาน 100 เพราะคอลัมน์ weight เป็น INTEGER และ 1 หน่วย = 1% อ่านง่าย
const weightScale = 100

// floorFactor / ceilFactor คือพื้นและเพดานเทียบกับส่วนแบ่งเท่าๆ กัน (uniform)
// ใช้ตัวคูณแทนเปอร์เซ็นต์ตายตัวเพราะจำนวนสูตรเปลี่ยนได้ — พื้น 10% ตายตัวจะเป็นไปไม่ได้
// ทางคณิตศาสตร์ทันทีที่มีสูตรเกิน 10 ตัว
const (
	floorFactor = 0.5
	ceilFactor  = 2.0
	// clampPasses คือจำนวนรอบสลับ clamp/normalize การ clamp ทำให้ผลรวมเพี้ยน
	// การ normalize ทำให้หลุด clamp — วนไม่กี่รอบก็ลู่เข้าพอสำหรับสเกลจำนวนเต็ม
	clampPasses = 3
)

// Combined คือคะแนนของหนึ่งสูตรที่รวมทุกแพลตฟอร์มแล้ว
type Combined struct {
	Value      string
	ScoreFinal float64
	N          int
}

// CombinePlatforms ยุบคะแนนข้ามแพลตฟอร์มของมิติที่ระบุ ถ่วงด้วยจำนวนคลิป —
// แพลตฟอร์มที่มีคลิปมากกว่าจึงมีน้ำหนักในการตัดสินมากกว่าโดยอัตโนมัติ และ
// แพลตฟอร์มที่หยุดโพสต์จะจางหายไปเองเมื่อหลุดหน้าต่างวัดผล
func CombinePlatforms(scores []Score, dimension string) []Combined {
	type acc struct {
		weighted float64
		n        int
	}
	byValue := map[string]*acc{}
	for _, s := range scores {
		if s.Dimension != dimension || s.N == 0 {
			continue
		}
		a := byValue[s.Value]
		if a == nil {
			a = &acc{}
			byValue[s.Value] = a
		}
		a.weighted += s.ScoreFinal * float64(s.N)
		a.n += s.N
	}

	out := make([]Combined, 0, len(byValue))
	for v, a := range byValue {
		out = append(out, Combined{Value: v, ScoreFinal: a.weighted / float64(a.n), N: a.n})
	}
	// เรียงชื่อเพื่อให้ผลลัพธ์ deterministic (การปัดเศษท้ายสุดขึ้นกับลำดับ)
	sort.Slice(out, func(i, j int) bool { return out[i].Value < out[j].Value })
	return out
}

// TuneWeights คำนวณ weight ชุดใหม่จากคะแนน โดยมีเบรกสามชั้นในฟังก์ชันนี้
// (ชั้นที่สี่ — ห้ามแตะ enabled — บังคับด้วยการที่ฟังก์ชันนี้ไม่รู้จัก enabled เลย):
//
//	1. ค่าที่ N < minN ถูกตรึงไว้ที่ส่วนแบ่งเดิม
//	2. ส่วนแบ่งถูก clamp ไว้ใน [floorFactor, ceilFactor] เท่าของ uniform
//	3. ขยับได้แค่ alpha ของระยะทางไปยังเป้าหมายต่อการเรียกหนึ่งครั้ง
//
// current คือ weight ปัจจุบันของสูตรที่ enabled ทั้งหมดในมิตินั้น (สเกลใดก็ได้)
// ผลลัพธ์เป็นสเกลผลรวม 100 เสมอ
func TuneWeights(current map[string]int, combined []Combined, minN int, alpha float64) map[string]int {
	if len(current) == 0 {
		return map[string]int{}
	}
	out := map[string]int{}
	if len(combined) == 0 {
		// ไม่มีคะแนน = ไม่มีข้อมูลใหม่ ห้ามขยับอะไรทั้งนั้น
		for k, v := range current {
			out[k] = v
		}
		return out
	}

	total := 0
	for _, w := range current {
		total += w
	}
	if total == 0 {
		total = len(current) // กันหารศูนย์: ถือว่าทุกตัวเท่ากัน
		for k := range current {
			current[k] = 1
		}
	}
	shareCurrent := map[string]float64{}
	for k, w := range current {
		shareCurrent[k] = float64(w) / float64(total)
	}

	scoreByValue := map[string]Combined{}
	for _, c := range combined {
		scoreByValue[c.Value] = c
	}

	uniform := 1.0 / float64(len(current))
	low, high := floorFactor*uniform, ceilFactor*uniform

	// แยกตัวที่ตรึง (ข้อมูลน้อย หรือไม่มีคะแนนเลย) ออกจากตัวที่ขยับได้
	var movable []string
	frozenShare := 0.0
	for k := range current {
		c, ok := scoreByValue[k]
		if !ok || c.N < minN {
			out[k] = -1 // ทำเครื่องหมายว่าตรึง เดี๋ยวเติมค่าตอนท้าย
			frozenShare += shareCurrent[k]
			continue
		}
		movable = append(movable, k)
	}
	sort.Strings(movable)

	if len(movable) == 0 {
		for k, v := range current {
			out[k] = v
		}
		return out
	}

	budget := 1.0 - frozenShare
	scoreSum := 0.0
	for _, k := range movable {
		scoreSum += scoreByValue[k].ScoreFinal
	}

	target := map[string]float64{}
	for _, k := range movable {
		if scoreSum > 0 {
			target[k] = scoreByValue[k].ScoreFinal / scoreSum * budget
		} else {
			target[k] = budget / float64(len(movable))
		}
	}

	// สลับ clamp กับ normalize จนกว่าจะลู่เข้า: clamp ทำให้ผลรวมเพี้ยน,
	// normalize ทำให้บางตัวหลุดขอบอีก — ไม่กี่รอบก็พอสำหรับสเกลจำนวนเต็ม
	for pass := 0; pass < clampPasses; pass++ {
		s := 0.0
		for _, k := range movable {
			if target[k] < low {
				target[k] = low
			}
			if target[k] > high {
				target[k] = high
			}
			s += target[k]
		}
		if s == 0 {
			break
		}
		for _, k := range movable {
			target[k] = target[k] / s * budget
		}
	}

	// เคลื่อนเข้าหาเป้าหมายแค่ alpha ของระยะทาง
	shareNew := map[string]float64{}
	for _, k := range movable {
		shareNew[k] = shareCurrent[k] + alpha*(target[k]-shareCurrent[k])
	}

	// แปลงเป็นจำนวนเต็มสเกล 100 แล้วแจกเศษที่เหลือแบบ largest-remainder
	// เพื่อให้ผลรวมเป็น 100 พอดีเสมอ
	type rem struct {
		key  string
		frac float64
	}
	assigned := 0
	var rems []rem
	for k := range current {
		var share float64
		if out[k] == -1 {
			share = shareCurrent[k]
		} else {
			share = shareNew[k]
		}
		exact := share * weightScale
		w := int(exact)
		out[k] = w
		assigned += w
		rems = append(rems, rem{key: k, frac: exact - float64(w)})
	}
	sort.Slice(rems, func(i, j int) bool {
		if rems[i].frac != rems[j].frac {
			return rems[i].frac > rems[j].frac
		}
		return rems[i].key < rems[j].key
	})
	for i := 0; assigned < weightScale; i, assigned = i+1, assigned+1 {
		out[rems[i%len(rems)].key]++
	}
	return out
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/scoreboard/ -v`
Expected: PASS ทั้งหมด (Task 1 + Task 2)

- [ ] **Step 5: Commit**

```bash
git add internal/scoreboard/tune.go internal/scoreboard/tune_test.go
git commit -m "feat(scoreboard): ตัวหมุนน้ำหนักพร้อมพื้น/เพดาน/ตรึงข้อมูลน้อย (pure)"
```

---

### Task 3: Migration + ตาราง + repository

**Files:**
- Create: `migrations/066_formula_scoreboard.sql`
- Create: `internal/repository/formula_scores.go`
- Modify: `internal/models/clip.go` (เพิ่ม struct ท้ายไฟล์)

**Interfaces:**
- Consumes: `scoreboard.Stat`, `scoreboard.Score` (Task 1)
- Produces:
  - `models.FormulaScoreRow`, `models.WeightRevision`
  - `func NewFormulaScoresRepo(pool *pgxpool.Pool) *FormulaScoresRepo`
  - `func (r *FormulaScoresRepo) RawStats(ctx context.Context, windowDays int) ([]scoreboard.Stat, error)`
  - `func (r *FormulaScoresRepo) SaveSnapshot(ctx context.Context, computedAt time.Time, scores []scoreboard.Score) error`
  - `func (r *FormulaScoresRepo) Latest(ctx context.Context) (time.Time, []scoreboard.Score, error)`
  - `func (r *FormulaScoresRepo) CurrentWeights(ctx context.Context, dimension string) (map[string]int, error)`
  - `func (r *FormulaScoresRepo) ApplyWeights(ctx context.Context, dimension string, weights map[string]int) error`
  - `func (r *FormulaScoresRepo) RecordRevisions(ctx context.Context, rows []models.WeightRevision) error`
  - `func (r *FormulaScoresRepo) ListRevisions(ctx context.Context, limit int) ([]models.WeightRevision, error)`
  - `func (r *FormulaScoresRepo) LastRevisionBatch(ctx context.Context) ([]models.WeightRevision, error)`

- [ ] **Step 1: เขียน migration**

สร้าง `migrations/066_formula_scoreboard.sql`:

```sql
-- 066: กระดานคะแนนสูตร + audit การหมุนน้ำหนัก
--
-- formula_scores เก็บเป็น snapshot (ไม่ใช่ view) เพื่อให้ตอบได้เสมอว่า weight
-- ของสัปดาห์หนึ่งมาจากคะแนนชุดไหน weight_revisions เป็น append-only เหมือน
-- skill_revisions — ไม่มี UPDATE/DELETE เพื่อให้ rollback ได้ตลอดเวลา
--
-- weight ถูกปรับเป็นสเกลผลรวม 100 ต่อมิติ (1 หน่วย = 1%) FormatsRepo.PickNext
-- และ TopicCategoriesRepo.PickNextExclude เทียบ used/weight ภายในตารางเดียวกัน
-- การเปลี่ยนสเกลจึงไม่กระทบพฤติกรรมการเลือก
--
-- RunMigrations ไม่หุ้ม transaction ให้ ต้อง BEGIN/COMMIT เอง
--
-- Rollback:
--   UPDATE settings SET value='false' WHERE key='weight_tuner_enabled';
--   UPDATE schedules SET enabled=FALSE WHERE action='tune_weights';
--   (ตาราง formula_scores / weight_revisions ทิ้งไว้ได้ ไม่มีผลข้างเคียง)
BEGIN;

CREATE TABLE IF NOT EXISTS formula_scores (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    computed_at      TIMESTAMPTZ NOT NULL,
    dimension        TEXT NOT NULL,              -- content_format | category | style_preset
    value            TEXT NOT NULL,
    platform         TEXT NOT NULL,              -- youtube | tiktok
    n                INTEGER NOT NULL,
    median_pct       DOUBLE PRECISION NOT NULL,
    median_retention DOUBLE PRECISION NOT NULL DEFAULT 0,  -- 0 = ไม่มีข้อมูล ไม่ใช่แย่
    flop_rate        DOUBLE PRECISION NOT NULL,
    score_raw        DOUBLE PRECISION NOT NULL,
    score_final      DOUBLE PRECISION NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_formula_scores_snapshot
    ON formula_scores (computed_at DESC, dimension, platform);

CREATE TABLE IF NOT EXISTS weight_revisions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dimension   TEXT NOT NULL,
    value       TEXT NOT NULL,
    old_weight  INTEGER NOT NULL,
    new_weight  INTEGER NOT NULL,
    score_final DOUBLE PRECISION NOT NULL,
    n           INTEGER NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL,            -- โยงกลับไป formula_scores.computed_at
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_weight_revisions_recent
    ON weight_revisions (created_at DESC);

-- ปรับ weight ของสูตรที่ enabled ให้เป็นสเกลผลรวม 100 แบบเท่าๆ กัน
-- (แถวที่ enabled = FALSE ไม่ต้องแตะ เพราะ PickNext กรองออกอยู่แล้ว)
UPDATE content_formats SET weight = 25 WHERE enabled = TRUE;
UPDATE topic_categories SET weight = 33 WHERE enabled = TRUE;
UPDATE topic_categories SET weight = 34
WHERE category_name = (SELECT category_name FROM topic_categories WHERE enabled = TRUE ORDER BY category_name LIMIT 1);

INSERT INTO settings (key, value)
VALUES ('weight_tuner_enabled', 'false')
ON CONFLICT (key) DO NOTHING;

INSERT INTO schedules (name, action, cron_expression, enabled)
VALUES ('Weekly Weight Tune', 'tune_weights', '30 3 * * 1', FALSE)
ON CONFLICT DO NOTHING;

COMMIT;
```

**หมายเหตุสำคัญก่อนรัน:** `UPDATE content_formats SET weight = 25 WHERE enabled = TRUE` ถูกต้องก็ต่อเมื่อมี 4 แถวที่ enabled (ตรวจแล้วเมื่อ 2026-07-27: qa, news, tips, case_story enabled; tutorial disabled) ถ้าจำนวนเปลี่ยนไป ให้แก้ตัวเลขให้ผลรวม = 100 ก่อนรัน ตรวจด้วย:
`SELECT count(*) FROM content_formats WHERE enabled = TRUE;` และ `SELECT count(*) FROM topic_categories WHERE enabled = TRUE;`

- [ ] **Step 2: เพิ่ม model structs**

ต่อท้าย `internal/models/clip.go`:

```go
// FormulaScoreRow คือหนึ่งแถวของกระดานคะแนนสูตรที่อ่านกลับมาจาก DB
// (สำหรับส่งออก API — ฝั่งคำนวณใช้ scoreboard.Score)
type FormulaScoreRow struct {
	ComputedAt      time.Time `json:"computed_at"`
	Dimension       string    `json:"dimension"`
	Value           string    `json:"value"`
	Platform        string    `json:"platform"`
	N               int       `json:"n"`
	MedianPct       float64   `json:"median_pct"`
	MedianRetention float64   `json:"median_retention"`
	FlopRate        float64   `json:"flop_rate"`
	ScoreFinal      float64   `json:"score_final"`
}

// WeightRevision คือ audit หนึ่งบรรทัดของการหมุนน้ำหนักหนึ่งสูตร
type WeightRevision struct {
	Dimension  string    `json:"dimension"`
	Value      string    `json:"value"`
	OldWeight  int       `json:"old_weight"`
	NewWeight  int       `json:"new_weight"`
	ScoreFinal float64   `json:"score_final"`
	N          int       `json:"n"`
	ComputedAt time.Time `json:"computed_at"`
	CreatedAt  time.Time `json:"created_at"`
}
```

ถ้า `internal/models/clip.go` ยังไม่ได้ import `time` ให้เพิ่มใน import block

- [ ] **Step 3: เขียน repository**

สร้าง `internal/repository/formula_scores.go`:

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/scoreboard"
)

// dimensionColumn แม็ปชื่อมิติไปยังคอลัมน์ใน clips เป็น allowlist ปิด — ชื่อคอลัมน์
// ถูกต่อเข้า SQL โดยตรงจึงห้ามรับค่าจากภายนอกเด็ดขาด
var dimensionColumn = map[string]string{
	"content_format": "content_format",
	"category":       "category",
	"style_preset":   "style_preset",
}

// weightTable แม็ปมิติไปยังตารางที่เก็บ weight มิติที่ไม่มีในแม็ปนี้ = วัดผลได้
// แต่หมุนน้ำหนักไม่ได้ (style_preset อยู่ในกรณีนี้ ดู spec หัวข้อ 7)
var weightTable = map[string]struct{ table, keyColumn string }{
	"content_format": {"content_formats", "format_name"},
	"category":       {"topic_categories", "category_name"},
}

type FormulaScoresRepo struct {
	pool *pgxpool.Pool
}

func NewFormulaScoresRepo(pool *pgxpool.Pool) *FormulaScoresRepo {
	return &FormulaScoresRepo{pool: pool}
}

// RawStats คำนวณสถิติดิบต่อ (มิติ, ค่า, แพลตฟอร์ม) จาก analytics ล่าสุดต่อคลิป
// ใช้ median ทุกที่ที่ทำได้เพราะคลิปไวรัลตัวเดียวลากค่าเฉลี่ยทั้งกลุ่มได้
// (ยืนยันจากข้อมูลจริง: preset ตัวหนึ่ง avg 0.130 แต่ median 0.041)
func (r *FormulaScoresRepo) RawStats(ctx context.Context, windowDays int) ([]scoreboard.Stat, error) {
	var out []scoreboard.Stat
	for dim, col := range dimensionColumn {
		q := fmt.Sprintf(`
WITH latest AS (
    SELECT DISTINCT ON (ca.clip_id, ca.platform)
        ca.clip_id, ca.platform, ca.views, ca.avg_view_percentage
    FROM clip_analytics ca
    WHERE ca.fetched_at >= NOW() - make_interval(days => $1)
      AND ca.platform IN ('youtube','tiktok')
      AND NOT EXISTS (
          SELECT 1 FROM clip_publish_status ps
          WHERE ps.clip_id = ca.clip_id AND ps.platform = ca.platform
            AND ps.status = 'failed')
    ORDER BY ca.clip_id, ca.platform, ca.fetched_at DESC
), ranked AS (
    SELECT l.platform, l.avg_view_percentage, c.%s AS value,
           PERCENT_RANK() OVER (PARTITION BY l.platform ORDER BY l.views) AS pct
    FROM latest l
    JOIN clips c ON c.id = l.clip_id
    WHERE c.status = 'published' AND COALESCE(c.%s, '') <> ''
)
SELECT value, platform, COUNT(*) AS n,
       PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY pct) AS median_pct,
       COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (
           ORDER BY NULLIF(avg_view_percentage, 0)), 0) AS median_retention,
       COUNT(*) FILTER (WHERE pct < 0.25)::float / COUNT(*) AS flop_rate
FROM ranked
GROUP BY value, platform`, col, col)

		rows, err := r.pool.Query(ctx, q, windowDays)
		if err != nil {
			return nil, fmt.Errorf("raw stats %s: %w", dim, err)
		}
		for rows.Next() {
			s := scoreboard.Stat{Dimension: dim}
			if err := rows.Scan(&s.Value, &s.Platform, &s.N, &s.MedianPct, &s.MedianRetention, &s.FlopRate); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan raw stat %s: %w", dim, err)
			}
			out = append(out, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate raw stats %s: %w", dim, err)
		}
	}
	return out, nil
}

// SaveSnapshot เขียนกระดานคะแนนทั้งชุดในทรานแซกชันเดียว — snapshot ครึ่งใบ
// อ่านผิดกว่าไม่มี snapshot เลย
func (r *FormulaScoresRepo) SaveSnapshot(ctx context.Context, computedAt time.Time, scores []scoreboard.Score) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin snapshot: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, s := range scores {
		if _, err := tx.Exec(ctx,
			`INSERT INTO formula_scores
			 (computed_at, dimension, value, platform, n, median_pct,
			  median_retention, flop_rate, score_raw, score_final)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			computedAt, s.Dimension, s.Value, s.Platform, s.N, s.MedianPct,
			s.MedianRetention, s.FlopRate, s.ScoreRaw, s.ScoreFinal); err != nil {
			return fmt.Errorf("insert score %s/%s: %w", s.Dimension, s.Value, err)
		}
	}
	return tx.Commit(ctx)
}

// Latest คืน snapshot ล่าสุดทั้งใบ ถ้ายังไม่เคยมี snapshot จะคืน slice ว่างและ error nil
func (r *FormulaScoresRepo) Latest(ctx context.Context) (time.Time, []scoreboard.Score, error) {
	var computedAt time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT MAX(computed_at) FROM formula_scores`).Scan(&computedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return time.Time{}, []scoreboard.Score{}, nil
		}
		return time.Time{}, nil, fmt.Errorf("latest snapshot time: %w", err)
	}
	if computedAt.IsZero() {
		return time.Time{}, []scoreboard.Score{}, nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT dimension, value, platform, n, median_pct, median_retention,
		        flop_rate, score_raw, score_final
		 FROM formula_scores WHERE computed_at = $1
		 ORDER BY dimension, platform, value`, computedAt)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("load snapshot: %w", err)
	}
	defer rows.Close()

	out := []scoreboard.Score{}
	for rows.Next() {
		var s scoreboard.Score
		if err := rows.Scan(&s.Dimension, &s.Value, &s.Platform, &s.N, &s.MedianPct,
			&s.MedianRetention, &s.FlopRate, &s.ScoreRaw, &s.ScoreFinal); err != nil {
			return time.Time{}, nil, fmt.Errorf("scan snapshot row: %w", err)
		}
		out = append(out, s)
	}
	return computedAt, out, rows.Err()
}

// CurrentWeights คืน weight ปัจจุบันของสูตรที่ enabled ในมิตินั้น
// มิติที่หมุนน้ำหนักไม่ได้ (เช่น style_preset) คืน map ว่างและ error nil
func (r *FormulaScoresRepo) CurrentWeights(ctx context.Context, dimension string) (map[string]int, error) {
	t, ok := weightTable[dimension]
	if !ok {
		return map[string]int{}, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(
		`SELECT %s, weight FROM %s WHERE enabled = TRUE`, t.keyColumn, t.table))
	if err != nil {
		return nil, fmt.Errorf("current weights %s: %w", dimension, err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var k string
		var w int
		if err := rows.Scan(&k, &w); err != nil {
			return nil, fmt.Errorf("scan weight %s: %w", dimension, err)
		}
		out[k] = w
	}
	return out, rows.Err()
}

// ApplyWeights เขียน weight ใหม่ทั้งมิติในทรานแซกชันเดียว ห้ามแตะคอลัมน์ enabled
func (r *FormulaScoresRepo) ApplyWeights(ctx context.Context, dimension string, weights map[string]int) error {
	t, ok := weightTable[dimension]
	if !ok {
		return fmt.Errorf("dimension %q หมุนน้ำหนักไม่ได้", dimension)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin apply weights: %w", err)
	}
	defer tx.Rollback(ctx)

	for k, w := range weights {
		if _, err := tx.Exec(ctx, fmt.Sprintf(
			`UPDATE %s SET weight = $1 WHERE %s = $2 AND enabled = TRUE`,
			t.table, t.keyColumn), w, k); err != nil {
			return fmt.Errorf("update weight %s/%s: %w", dimension, k, err)
		}
	}
	return tx.Commit(ctx)
}

// RecordRevisions เขียน audit ทั้งชุด ต้องเรียกให้สำเร็จ *ก่อน* ApplyWeights เสมอ
func (r *FormulaScoresRepo) RecordRevisions(ctx context.Context, revs []models.WeightRevision) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin revisions: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, v := range revs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO weight_revisions
			 (dimension, value, old_weight, new_weight, score_final, n, computed_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			v.Dimension, v.Value, v.OldWeight, v.NewWeight, v.ScoreFinal, v.N, v.ComputedAt); err != nil {
			return fmt.Errorf("insert revision %s/%s: %w", v.Dimension, v.Value, err)
		}
	}
	return tx.Commit(ctx)
}

// ListRevisions คืนประวัติล่าสุด newest-first
func (r *FormulaScoresRepo) ListRevisions(ctx context.Context, limit int) ([]models.WeightRevision, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT dimension, value, old_weight, new_weight, score_final, n, computed_at, created_at
		 FROM weight_revisions ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list weight revisions: %w", err)
	}
	defer rows.Close()

	out := []models.WeightRevision{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		var v models.WeightRevision
		if err := rows.Scan(&v.Dimension, &v.Value, &v.OldWeight, &v.NewWeight,
			&v.ScoreFinal, &v.N, &v.ComputedAt, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan weight revision: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// LastRevisionBatch คืน audit ของการหมุนรอบล่าสุด (batch เดียวกัน = computed_at เดียวกัน)
// ใช้สำหรับ rollback: เขียน old_weight กลับ
func (r *FormulaScoresRepo) LastRevisionBatch(ctx context.Context) ([]models.WeightRevision, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT dimension, value, old_weight, new_weight, score_final, n, computed_at, created_at
		 FROM weight_revisions
		 WHERE computed_at = (SELECT MAX(computed_at) FROM weight_revisions)
		 ORDER BY dimension, value`)
	if err != nil {
		return nil, fmt.Errorf("last revision batch: %w", err)
	}
	defer rows.Close()

	out := []models.WeightRevision{}
	for rows.Next() {
		var v models.WeightRevision
		if err := rows.Scan(&v.Dimension, &v.Value, &v.OldWeight, &v.NewWeight,
			&v.ScoreFinal, &v.N, &v.ComputedAt, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan revision batch row: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
```

**หมายเหตุเรื่องความปลอดภัยของ SQL:** `RawStats` และ `CurrentWeights`/`ApplyWeights` ต่อชื่อคอลัมน์/ตารางเข้า SQL ด้วย `fmt.Sprintf` ได้เพราะค่ามาจาก `dimensionColumn`/`weightTable` ซึ่งเป็น allowlist ปิดในไฟล์เดียวกัน **ห้ามเปลี่ยนให้รับชื่อมิติจาก HTTP หรือ DB โดยตรง**

- [ ] **Step 4: คอมไพล์ให้ผ่าน**

Run: `go build ./... && go vet ./internal/repository/ ./internal/scoreboard/`
Expected: ไม่มี error

- [ ] **Step 5: ตรวจ SQL กับ DB จริงแบบอ่านอย่างเดียว**

รัน query ของ `RawStats` ตรงๆ กับ prod (แทน `%s` ด้วย `content_format` และ `$1` ด้วย `60`) แล้วยืนยันว่าได้ผลใกล้เคียงกับที่ spec บันทึกไว้: TikTok `tips` median_pct ≈ 0.58 (n=17), `qa` ≈ 0.54 (n=29), `news` ≈ 0.31 (n=23)
ถ้าตัวเลขต่างมาก ให้หยุดและตรวจเงื่อนไข `status='published'` กับตัวกรอง failed post ก่อนไปต่อ

- [ ] **Step 6: Commit**

```bash
git add migrations/066_formula_scoreboard.sql internal/repository/formula_scores.go internal/models/clip.go
git commit -m "feat(scoreboard): ตาราง formula_scores/weight_revisions + repository"
```

---

### Task 4: Service — คำนวณ snapshot และหมุนน้ำหนัก

**Files:**
- Create: `internal/scoreboard/service.go`
- Test: `internal/scoreboard/service_test.go`

**Interfaces:**
- Consumes: `ScoreAll`, `CombinePlatforms`, `TuneWeights` (Task 1-2), repository (Task 3)
- Produces:
  - `func NewService(repo Repo, settings Settings, now func() time.Time) *Service`
  - `func (s *Service) ComputeSnapshot(ctx context.Context) (int, error)`
  - `func (s *Service) TuneOnce(ctx context.Context) error`
  - `func (s *Service) Rollback(ctx context.Context) (int, error)`

- [ ] **Step 1: เขียนเทสต์ที่ต้อง fail**

สร้าง `internal/scoreboard/service_test.go`:

```go
package scoreboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jaochai/video-fb/internal/models"
)

type fakeRepo struct {
	stats       []Stat
	saved       []Score
	weights     map[string]map[string]int
	applied     map[string]map[string]int
	revisions   []models.WeightRevision
	failRecord  bool
	lastBatch   []models.WeightRevision
	latestScore []Score
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		weights: map[string]map[string]int{},
		applied: map[string]map[string]int{},
	}
}

func (f *fakeRepo) RawStats(ctx context.Context, windowDays int) ([]Stat, error) { return f.stats, nil }
func (f *fakeRepo) SaveSnapshot(ctx context.Context, at time.Time, scores []Score) error {
	f.saved = scores
	f.latestScore = scores
	return nil
}
func (f *fakeRepo) Latest(ctx context.Context) (time.Time, []Score, error) {
	return time.Unix(1000, 0), f.latestScore, nil
}
func (f *fakeRepo) CurrentWeights(ctx context.Context, dim string) (map[string]int, error) {
	if w, ok := f.weights[dim]; ok {
		return w, nil
	}
	return map[string]int{}, nil
}
func (f *fakeRepo) ApplyWeights(ctx context.Context, dim string, w map[string]int) error {
	f.applied[dim] = w
	return nil
}
func (f *fakeRepo) RecordRevisions(ctx context.Context, revs []models.WeightRevision) error {
	if f.failRecord {
		return errors.New("audit ล่ม")
	}
	f.revisions = append(f.revisions, revs...)
	return nil
}
func (f *fakeRepo) LastRevisionBatch(ctx context.Context) ([]models.WeightRevision, error) {
	return f.lastBatch, nil
}

type fakeSettings struct{ v string }

func (f fakeSettings) Get(ctx context.Context, key string) (string, error) { return f.v, nil }

func fixedNow() time.Time { return time.Unix(1000, 0).UTC() }

func TestComputeSnapshotScoresAndSaves(t *testing.T) {
	repo := newFakeRepo()
	repo.stats = []Stat{
		{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30, MedianPct: 0.53, MedianRetention: 0.7, FlopRate: 0.3},
		{Dimension: "content_format", Value: "news", Platform: "youtube", N: 25, MedianPct: 0.53, MedianRetention: 0.75, FlopRate: 0.16},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	n, err := svc.ComputeSnapshot(context.Background())
	if err != nil {
		t.Fatalf("ComputeSnapshot error: %v", err)
	}
	if n != 2 || len(repo.saved) != 2 {
		t.Fatalf("บันทึก %d แถว (คืน %d), want 2", len(repo.saved), n)
	}
	if repo.saved[0].ScoreFinal == 0 {
		t.Error("ScoreFinal ต้องถูกคำนวณก่อนบันทึก")
	}
}

func TestTuneOnceDisabledIsNoOp(t *testing.T) {
	repo := newFakeRepo()
	repo.weights["content_format"] = map[string]int{"qa": 50, "news": 50}
	repo.latestScore = []Score{
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
		{Stat: Stat{Dimension: "content_format", Value: "news", Platform: "youtube", N: 30}, ScoreFinal: 0.2},
	}
	svc := NewService(repo, fakeSettings{v: "false"}, fixedNow)
	if err := svc.TuneOnce(context.Background()); err != nil {
		t.Fatalf("TuneOnce error: %v", err)
	}
	if len(repo.applied) != 0 {
		t.Errorf("flag ปิดแล้วยังเขียน weight: %v", repo.applied)
	}
}

func TestTuneOnceWritesAuditBeforeApplying(t *testing.T) {
	repo := newFakeRepo()
	repo.failRecord = true
	repo.weights["content_format"] = map[string]int{"qa": 50, "news": 50}
	repo.latestScore = []Score{
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
		{Stat: Stat{Dimension: "content_format", Value: "news", Platform: "youtube", N: 30}, ScoreFinal: 0.2},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	_ = svc.TuneOnce(context.Background())
	if len(repo.applied) != 0 {
		t.Error("audit ล้มแล้วห้าม apply weight เด็ดขาด")
	}
}

func TestTuneOnceAppliesAndAudits(t *testing.T) {
	repo := newFakeRepo()
	repo.weights["content_format"] = map[string]int{"qa": 50, "news": 50}
	repo.latestScore = []Score{
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
		{Stat: Stat{Dimension: "content_format", Value: "news", Platform: "youtube", N: 30}, ScoreFinal: 0.2},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	if err := svc.TuneOnce(context.Background()); err != nil {
		t.Fatalf("TuneOnce error: %v", err)
	}
	applied := repo.applied["content_format"]
	if applied["qa"] <= 50 {
		t.Errorf("qa = %d ควรเพิ่มขึ้น", applied["qa"])
	}
	if len(repo.revisions) == 0 {
		t.Error("ต้องมี audit อย่างน้อยหนึ่งแถว")
	}
	for _, r := range repo.revisions {
		if r.Dimension == "style_preset" {
			t.Error("style_preset ห้ามถูกหมุนน้ำหนัก")
		}
	}
}

func TestTuneOnceSkipsStylePreset(t *testing.T) {
	repo := newFakeRepo()
	repo.latestScore = []Score{
		{Stat: Stat{Dimension: "style_preset", Value: "case-file", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	if err := svc.TuneOnce(context.Background()); err != nil {
		t.Fatalf("TuneOnce error: %v", err)
	}
	if _, ok := repo.applied["style_preset"]; ok {
		t.Error("style_preset ต้องไม่ถูกเขียน weight")
	}
}

func TestRollbackRestoresOldWeights(t *testing.T) {
	repo := newFakeRepo()
	repo.lastBatch = []models.WeightRevision{
		{Dimension: "content_format", Value: "qa", OldWeight: 25, NewWeight: 31},
		{Dimension: "content_format", Value: "news", OldWeight: 25, NewWeight: 19},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	n, err := svc.Rollback(context.Background())
	if err != nil {
		t.Fatalf("Rollback error: %v", err)
	}
	if n != 2 {
		t.Errorf("คืนค่า %d แถว, want 2", n)
	}
	if repo.applied["content_format"]["qa"] != 25 {
		t.Errorf("qa = %d, want 25", repo.applied["content_format"]["qa"])
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่า fail**

Run: `go test ./internal/scoreboard/ -run TestTuneOnce -v`
Expected: FAIL — `undefined: NewService`

- [ ] **Step 3: เขียน implementation ขั้นต่ำ**

สร้าง `internal/scoreboard/service.go`:

```go
package scoreboard

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jaochai/video-fb/internal/models"
)

const (
	// WindowDays คือหน้าต่างวัดผล 60 วัน — 30 วันให้ category แค่ ~5 คลิป/หมวด ไม่พอตัดสิน
	WindowDays = 60
	// MinN คือจำนวนคลิปขั้นต่ำก่อนยอมให้สูตรหนึ่งขยับน้ำหนัก
	MinN = 8
	// Alpha คือสัดส่วนของระยะทางที่ยอมให้เคลื่อนต่อรอบ (สัปดาห์ละครั้ง)
	Alpha = 0.25
	// SettingKey คือสวิตช์ปิดฉุกเฉิน ปิดแล้วหยุดหมุนทันทีโดยไม่แตะค่าที่มีอยู่
	SettingKey = "weight_tuner_enabled"
)

// TunableDimensions คือมิติที่หมุนน้ำหนักได้จริง style_preset ไม่อยู่ในนี้โดยตั้งใจ:
// การเลือก preset บน prod ถูก CaseFormatEnabled คร่อมไว้ก่อน จึงวัดผลอย่างเดียว
var TunableDimensions = []string{"content_format", "category"}

// Repo คือสิ่งที่ service ต้องการจากชั้นข้อมูล — แคบไว้เพื่อให้ทดสอบด้วย fake ได้
type Repo interface {
	RawStats(ctx context.Context, windowDays int) ([]Stat, error)
	SaveSnapshot(ctx context.Context, computedAt time.Time, scores []Score) error
	Latest(ctx context.Context) (time.Time, []Score, error)
	CurrentWeights(ctx context.Context, dimension string) (map[string]int, error)
	ApplyWeights(ctx context.Context, dimension string, weights map[string]int) error
	RecordRevisions(ctx context.Context, revs []models.WeightRevision) error
	LastRevisionBatch(ctx context.Context) ([]models.WeightRevision, error)
}

// Settings อ่านสวิตช์เปิด/ปิด
type Settings interface {
	Get(ctx context.Context, key string) (string, error)
}

type Service struct {
	repo     Repo
	settings Settings
	now      func() time.Time
}

func NewService(repo Repo, settings Settings, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, settings: settings, now: now}
}

// ComputeSnapshot คำนวณกระดานคะแนนใหม่หนึ่งใบ ไม่แตะ weight ใดๆ
// คืนจำนวนแถวที่บันทึก
func (s *Service) ComputeSnapshot(ctx context.Context) (int, error) {
	stats, err := s.repo.RawStats(ctx, WindowDays)
	if err != nil {
		return 0, fmt.Errorf("scoreboard: raw stats: %w", err)
	}
	if len(stats) == 0 {
		log.Printf("scoreboard: ไม่มีสถิติในหน้าต่าง %d วัน — ไม่บันทึก snapshot", WindowDays)
		return 0, nil
	}
	scores := ScoreAll(stats)
	if err := s.repo.SaveSnapshot(ctx, s.now(), scores); err != nil {
		return 0, fmt.Errorf("scoreboard: save snapshot: %w", err)
	}
	log.Printf("scoreboard: บันทึก %d แถว (หน้าต่าง %d วัน)", len(scores), WindowDays)
	return len(scores), nil
}

// TuneOnce หมุนน้ำหนักหนึ่งรอบจาก snapshot ล่าสุด เขียน audit ให้สำเร็จก่อนเสมอ
// ความล้มเหลวของมิติหนึ่งไม่หยุดมิติอื่น
func (s *Service) TuneOnce(ctx context.Context) error {
	enabled, err := s.settings.Get(ctx, SettingKey)
	if err != nil {
		return fmt.Errorf("scoreboard: อ่าน %s: %w", SettingKey, err)
	}
	if enabled != "true" {
		log.Printf("scoreboard: %s != true — ข้ามรอบนี้", SettingKey)
		return nil
	}

	computedAt, scores, err := s.repo.Latest(ctx)
	if err != nil {
		return fmt.Errorf("scoreboard: อ่าน snapshot ล่าสุด: %w", err)
	}
	if len(scores) == 0 {
		log.Printf("scoreboard: ยังไม่มี snapshot — ไม่หมุนน้ำหนัก")
		return nil
	}

	for _, dim := range TunableDimensions {
		current, err := s.repo.CurrentWeights(ctx, dim)
		if err != nil {
			log.Printf("scoreboard: [%s] อ่าน weight ไม่ได้ (ข้าม): %v", dim, err)
			continue
		}
		if len(current) == 0 {
			log.Printf("scoreboard: [%s] ไม่มีสูตรที่ enabled — ข้าม", dim)
			continue
		}

		combined := CombinePlatforms(scores, dim)
		next := TuneWeights(current, combined, MinN, Alpha)

		scoreByValue := map[string]Combined{}
		for _, c := range combined {
			scoreByValue[c.Value] = c
		}
		var revs []models.WeightRevision
		for k, newW := range next {
			if newW == current[k] {
				continue
			}
			revs = append(revs, models.WeightRevision{
				Dimension: dim, Value: k,
				OldWeight: current[k], NewWeight: newW,
				ScoreFinal: scoreByValue[k].ScoreFinal, N: scoreByValue[k].N,
				ComputedAt: computedAt,
			})
		}
		if len(revs) == 0 {
			log.Printf("scoreboard: [%s] น้ำหนักไม่เปลี่ยน — ข้าม", dim)
			continue
		}

		// audit ก่อน apply เสมอ: ถ้าเขียนประวัติไม่ได้ ห้ามเปลี่ยนของจริง
		if err := s.repo.RecordRevisions(ctx, revs); err != nil {
			log.Printf("scoreboard: [%s] เขียน audit ไม่สำเร็จ — ไม่ apply: %v", dim, err)
			continue
		}
		if err := s.repo.ApplyWeights(ctx, dim, next); err != nil {
			log.Printf("scoreboard: [%s] apply ล้มหลังเขียน audit แล้ว (กู้จาก weight_revisions ได้): %v", dim, err)
			continue
		}
		log.Printf("scoreboard: [%s] หมุนน้ำหนัก %d ค่า: %v", dim, len(revs), next)
	}
	return nil
}

// Rollback เขียน old_weight ของ batch ล่าสุดกลับ คืนจำนวนแถวที่คืนค่า
// ไม่ลบประวัติ — ประวัติเป็น append-only เสมอ
func (s *Service) Rollback(ctx context.Context) (int, error) {
	batch, err := s.repo.LastRevisionBatch(ctx)
	if err != nil {
		return 0, fmt.Errorf("scoreboard: อ่าน batch ล่าสุด: %w", err)
	}
	if len(batch) == 0 {
		return 0, nil
	}
	byDim := map[string]map[string]int{}
	for _, v := range batch {
		if byDim[v.Dimension] == nil {
			byDim[v.Dimension] = map[string]int{}
		}
		byDim[v.Dimension][v.Value] = v.OldWeight
	}
	for dim, weights := range byDim {
		if err := s.repo.ApplyWeights(ctx, dim, weights); err != nil {
			return 0, fmt.Errorf("scoreboard: rollback %s: %w", dim, err)
		}
	}
	log.Printf("scoreboard: rollback คืนค่า %d แถว", len(batch))
	return len(batch), nil
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/scoreboard/ -v`
Expected: PASS ทั้งหมด

- [ ] **Step 5: Commit**

```bash
git add internal/scoreboard/service.go internal/scoreboard/service_test.go
git commit -m "feat(scoreboard): service คำนวณ snapshot + หมุนน้ำหนัก (audit ก่อน apply)"
```

---

### Task 5: ต่อเข้า scheduler และ main

**Files:**
- Modify: `internal/scheduler/scheduler.go` (struct, `New`, `handlerFor`)
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes: `scoreboard.Service` (Task 4), `repository.FormulaScoresRepo` (Task 3)
- Produces: scheduler action `"tune_weights"`

- [ ] **Step 1: เพิ่ม field และ action ใน scheduler**

ใน `internal/scheduler/scheduler.go`:

เพิ่ม import `"github.com/jaochai/video-fb/internal/scoreboard"`

เพิ่ม field ท้าย struct `Scheduler`:

```go
	scoreboard    *scoreboard.Service
```

เปลี่ยนลายเซ็น `New` ให้รับพารามิเตอร์เพิ่มท้ายสุด และเซ็ตค่าในทั้งสอง return path (path fallback UTC และ path ปกติ):

```go
func New(pool *pgxpool.Pool, pub *publisher.Publisher, anlz *analyzer.Analyzer, orch *orchestrator.Orchestrator, schedRepo *repository.SchedulesRepo, clipsRepo *repository.ClipsRepo, lrn *learner.Learner, sb *scoreboard.Service) *Scheduler {
```

ทั้งสอง `return &Scheduler{...}` ให้เพิ่มบรรทัด `scoreboard: sb,`

เพิ่ม case ใน `handlerFor`:

```go
	case "tune_weights":
		return s.tuneWeights
```

เพิ่มเมธอดท้ายไฟล์:

```go
// tuneWeights คำนวณกระดานคะแนนใหม่แล้วหมุนน้ำหนัก การคำนวณต้องมาก่อนเสมอ
// เพื่อให้ weight รอบนี้อิงข้อมูลล่าสุด ไม่ใช่ snapshot ค้างจากสัปดาห์ก่อน
func (s *Scheduler) tuneWeights(ctx context.Context) error {
	if _, err := s.scoreboard.ComputeSnapshot(ctx); err != nil {
		return fmt.Errorf("compute scoreboard: %w", err)
	}
	return s.scoreboard.TuneOnce(ctx)
}
```

- [ ] **Step 2: ต่อสายใน main**

ใน `cmd/server/main.go` เพิ่มก่อนบรรทัดที่สร้าง scheduler (`sched := scheduler.New(...)`):

```go
	formulaScoresRepo := repository.NewFormulaScoresRepo(pool)
	scoreboardSvc := scoreboard.NewService(formulaScoresRepo, settingsRepo, time.Now)
```

แล้วแก้การเรียก `scheduler.New(...)` ให้ส่ง `scoreboardSvc` เป็นอาร์กิวเมนต์สุดท้าย

เพิ่ม import `"github.com/jaochai/video-fb/internal/scoreboard"` (และ `"time"` ถ้ายังไม่มี)

ถ้าใน `main.go` ยังไม่มีตัวแปร `settingsRepo` ให้สร้างด้วย `settingsRepo := repository.NewSettingsRepo(pool)` ก่อนบรรทัดข้างบน (ตรวจก่อนด้วย `grep -n "settingsRepo\|NewSettingsRepo" cmd/server/main.go`)

- [ ] **Step 3: คอมไพล์และรันเทสต์ทั้งหมด**

Run: `go build ./... && go test ./...`
Expected: build ผ่าน, เทสต์เดิมทั้งหมดยังผ่าน

- [ ] **Step 4: ตรวจว่า action ถูกลงทะเบียนจริง**

Run: `grep -n "tune_weights" internal/scheduler/scheduler.go migrations/066_formula_scoreboard.sql`
Expected: เจอทั้งใน `handlerFor` และในแถว schedule ของ migration — ชื่อต้องตรงกันเป๊ะ ไม่งั้น scheduler จะ log "unknown action" แล้วข้ามเงียบๆ

- [ ] **Step 5: Commit**

```bash
git add internal/scheduler/scheduler.go cmd/server/main.go
git commit -m "feat(scoreboard): ลงทะเบียน action tune_weights + ต่อสายใน main"
```

---

### Task 6: HTTP endpoints

**Files:**
- Create: `internal/handler/scoreboard.go`
- Modify: `internal/router/router.go`

**Interfaces:**
- Consumes: `scoreboard.Service`, `repository.FormulaScoresRepo`
- Produces: `GET /api/v1/formula-scores`, `POST /api/v1/formula-scores/compute`, `GET /api/v1/weight-revisions`, `POST /api/v1/weights/rollback`

- [ ] **Step 1: เขียน handler**

สร้าง `internal/handler/scoreboard.go`:

```go
package handler

import (
	"net/http"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/jaochai/video-fb/internal/scoreboard"
)

type ScoreboardHandler struct {
	repo *repository.FormulaScoresRepo
	svc  *scoreboard.Service
}

func NewScoreboardHandler(repo *repository.FormulaScoresRepo, svc *scoreboard.Service) *ScoreboardHandler {
	return &ScoreboardHandler{repo: repo, svc: svc}
}

// Latest คืนกระดานคะแนนใบล่าสุด
func (h *ScoreboardHandler) Latest(w http.ResponseWriter, r *http.Request) {
	computedAt, scores, err := h.repo.Latest(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	rows := []models.FormulaScoreRow{} // non-nil so an empty result marshals to [] not null
	for _, s := range scores {
		rows = append(rows, models.FormulaScoreRow{
			ComputedAt: computedAt, Dimension: s.Dimension, Value: s.Value,
			Platform: s.Platform, N: s.N, MedianPct: s.MedianPct,
			MedianRetention: s.MedianRetention, FlopRate: s.FlopRate,
			ScoreFinal: s.ScoreFinal,
		})
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{
		"computed_at": computedAt,
		"scores":      rows,
	}})
}

// Compute คำนวณ snapshot ใหม่ทันทีโดยไม่แตะ weight — ใช้ตรวจก่อนเปิดสวิตช์
func (h *ScoreboardHandler) Compute(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.ComputeSnapshot(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{"rows": n}})
}

// Revisions คืนประวัติการหมุนน้ำหนัก 20 รายการล่าสุด
func (h *ScoreboardHandler) Revisions(w http.ResponseWriter, r *http.Request) {
	revs, err := h.repo.ListRevisions(r.Context(), 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: revs})
}

// Rollback คืน weight ของรอบล่าสุดกลับเป็นค่าเดิม
func (h *ScoreboardHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.Rollback(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{"restored": n}})
}
```

- [ ] **Step 2: ลงทะเบียน route**

ใน `internal/router/router.go` ต่อจากบล็อก `presets` (ราวบรรทัด 90-94) เพิ่ม:

```go
	formulaRepo := repository.NewFormulaScoresRepo(pool)
	sbHandler := handler.NewScoreboardHandler(formulaRepo,
		scoreboard.NewService(formulaRepo, repository.NewSettingsRepo(pool), time.Now))
	r.Get("/api/v1/formula-scores", sbHandler.Latest)
	r.Post("/api/v1/formula-scores/compute", sbHandler.Compute)
	r.Get("/api/v1/weight-revisions", sbHandler.Revisions)
	r.Post("/api/v1/weights/rollback", sbHandler.Rollback)
```

เพิ่ม import `"time"` และ `"github.com/jaochai/video-fb/internal/scoreboard"` ถ้ายังไม่มี

- [ ] **Step 3: คอมไพล์**

Run: `go build ./... && go test ./...`
Expected: ผ่านทั้งหมด

- [ ] **Step 4: ทดสอบด้วยเซิร์ฟเวอร์จริง (local)**

รันเซิร์ฟเวอร์ในเครื่อง แล้ว:

```bash
curl -s localhost:8080/api/v1/formula-scores -H "X-API-Key: $API_KEY" | head -c 400
```

Expected: JSON ที่ `data.scores` เป็น `[]` (ยังไม่มี snapshot) **ไม่ใช่ `null`** — ถ้าเป็น `null` แปลว่าลืม init `[]T{}` และหน้าเว็บจะพัง

- [ ] **Step 5: Commit**

```bash
git add internal/handler/scoreboard.go internal/router/router.go
git commit -m "feat(scoreboard): endpoints อ่านกระดานคะแนน/ประวัติ + สั่งคำนวณ/rollback"
```

---

### Task 7: แผงกระดานคะแนนในหน้า Analytics

**Files:**
- Modify: `frontend/src/api.ts`
- Modify: `frontend/src/pages/Analytics.tsx`

**Interfaces:**
- Consumes: `GET /api/v1/formula-scores`, `GET /api/v1/weight-revisions` (Task 6)
- Produces: แผงอ่านอย่างเดียวในหน้า Analytics

- [ ] **Step 1: เพิ่ม type และ fetcher**

ต่อท้าย `frontend/src/api.ts`:

```ts
export interface FormulaScore {
  computed_at: string;
  dimension: string;
  value: string;
  platform: string;
  n: number;
  median_pct: number;
  median_retention: number;
  flop_rate: number;
  score_final: number;
}

export interface WeightRevision {
  dimension: string;
  value: string;
  old_weight: number;
  new_weight: number;
  score_final: number;
  n: number;
  computed_at: string;
  created_at: string;
}

export const getFormulaScores = () =>
  apiFetch<{ computed_at: string; scores: FormulaScore[] }>('/api/v1/formula-scores');
export const getWeightRevisions = () =>
  apiFetch<WeightRevision[]>('/api/v1/weight-revisions');
```

- [ ] **Step 2: เพิ่มแผงในหน้า Analytics**

ใน `frontend/src/pages/Analytics.tsx` เพิ่ม import:

```ts
import { getFormulaScores, getWeightRevisions, type FormulaScore } from '../api';
```

เพิ่มคอมโพเนนต์ (วางก่อน export ของหน้า):

```tsx
function FormulaScoreboard() {
  const { data, isLoading } = useQuery({
    queryKey: ['formula-scores'],
    queryFn: getFormulaScores,
  });
  const { data: revisions } = useQuery({
    queryKey: ['weight-revisions'],
    queryFn: getWeightRevisions,
  });

  if (isLoading) return null;
  const scores = data?.scores ?? [];
  if (scores.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>กระดานคะแนนสูตร</CardTitle>
          <CardDescription>ยังไม่มีข้อมูล — รอรอบคำนวณรอบแรก</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  const byDimension = scores.reduce<Record<string, FormulaScore[]>>((acc, s) => {
    (acc[s.dimension] ??= []).push(s);
    return acc;
  }, {});

  return (
    <Card>
      <CardHeader>
        <CardTitle>กระดานคะแนนสูตร</CardTitle>
        <CardDescription>
          คำนวณเมื่อ {new Date(scores[0].computed_at).toLocaleString('th-TH')} · หน้าต่าง 60 วัน
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {Object.entries(byDimension).map(([dim, rows]) => (
          <div key={dim}>
            <h4 className="text-sm font-medium mb-2">{dim}</h4>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="text-muted-foreground text-xs uppercase">
                  <tr>
                    <th className="text-left py-1">สูตร</th>
                    <th className="text-left py-1">แพลตฟอร์ม</th>
                    <th className="text-right py-1">n</th>
                    <th className="text-right py-1">median วิว</th>
                    <th className="text-right py-1">flop</th>
                    <th className="text-right py-1">คะแนน</th>
                  </tr>
                </thead>
                <tbody>
                  {[...rows]
                    .sort((a, b) => b.score_final - a.score_final)
                    .map(r => (
                      <tr key={`${r.value}-${r.platform}`} className="border-t border-border">
                        <td className="py-1">{r.value}</td>
                        <td className="py-1">{r.platform}</td>
                        <td className="py-1 text-right">{r.n}</td>
                        <td className="py-1 text-right">{(r.median_pct * 100).toFixed(0)}</td>
                        <td className="py-1 text-right">{(r.flop_rate * 100).toFixed(0)}%</td>
                        <td className="py-1 text-right font-medium">{r.score_final.toFixed(3)}</td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </div>
          </div>
        ))}
        {revisions && revisions.length > 0 && (
          <div>
            <h4 className="text-sm font-medium mb-2">การหมุนน้ำหนักล่าสุด</h4>
            <ul className="text-sm space-y-1">
              {revisions.slice(0, 8).map((r, i) => (
                <li key={i} className="text-muted-foreground">
                  {new Date(r.created_at).toLocaleDateString('th-TH')} · {r.dimension}/{r.value}:{' '}
                  {r.old_weight} → {r.new_weight}
                </li>
              ))}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
```

แล้ววาง `<FormulaScoreboard />` ในเลย์เอาต์ของหน้า Analytics ต่อจากแผงที่มีอยู่

**ก่อนเขียน:** ตรวจว่า `Analytics.tsx` import `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent` และ `useQuery` แล้วหรือยัง (`grep -n "CardContent\|useQuery" frontend/src/pages/Analytics.tsx`) ถ้ายัง ให้เพิ่มตามรูปแบบใน `frontend/src/pages/Theme.tsx`

- [ ] **Step 3: ตรวจ build ฝั่งหน้าเว็บ**

Run: `cd frontend && npm run build`
Expected: build ผ่าน ไม่มี type error

- [ ] **Step 4: Commit**

```bash
git add frontend/src/api.ts frontend/src/pages/Analytics.tsx
git commit -m "feat(analytics): แผงกระดานคะแนนสูตร + ประวัติการหมุนน้ำหนัก"
```

---

### Task 8: ปลุก learner — หน้าต่างไม่ทับกัน + นับปัญหาซ้ำให้ถูก

**Files:**
- Modify: `internal/repository/critiques.go:71-130`
- Modify: `internal/learner/learner.go:17-33, 103-186`
- Test: `internal/repository/critiques_test.go` (สร้างถ้ายังไม่มี), `internal/learner/learner_test.go`

**Interfaces:**
- Consumes: —
- Produces:
  - `func NormalizeField(field string) string` (ใน `repository`)
  - `func (r *CritiquesRepo) LowScorePatterns(ctx context.Context, sinceDays, untilDays, topN int) (ScorePatterns, error)` — **เปลี่ยนลายเซ็น เพิ่ม `untilDays`**

- [ ] **Step 1: เขียนเทสต์ที่ต้อง fail**

สร้าง/แก้ `internal/repository/critiques_test.go`:

```go
package repository

import "testing"

func TestNormalizeField(t *testing.T) {
	cases := map[string]string{
		"scene[5].image_prompt":   "scene.image_prompt",
		"scene[12].voice_text":    "scene.voice_text",
		"metadata.youtube_title":  "metadata.youtube_title",
		"scene[0].on_screen_text": "scene.on_screen_text",
		"":                        "",
	}
	for in, want := range cases {
		if got := NormalizeField(in); got != want {
			t.Errorf("NormalizeField(%q) = %q, want %q", in, got, want)
		}
	}
}
```

แก้ `internal/learner/learner_test.go` เพิ่มเทสต์ (เก็บเทสต์เดิมไว้ ปรับให้เข้ากับลายเซ็นใหม่ถ้าจำเป็น):

```go
func TestStrongSignalFiresOnRegressionAgainstNonOverlappingBaseline(t *testing.T) {
	// window แย่กว่า baseline เกิน 0.5 → ต้องยิง
	window := repository.ScorePatterns{N: 20, AvgHook: 7.0, AvgClarity: 8.0, AvgBrandFit: 8.5, AvgOverall: 7.9}
	baseline := repository.ScorePatterns{N: 20, AvgHook: 7.8, AvgClarity: 8.0, AvgBrandFit: 8.5, AvgOverall: 7.9}
	fire, dim, _, gate := strongSignal(window, baseline)
	if !fire || dim != "hook" || gate != "regression" {
		t.Errorf("ควรยิง regression ที่ hook ได้ fire=%v dim=%s gate=%s", fire, dim, gate)
	}
}

func TestStrongSignalFiresOnNormalizedIssueFrequency(t *testing.T) {
	// ปัญหาเดียวรวมแล้ว 9 ครั้งจาก 20 critique = 45% >= 40%
	window := repository.ScorePatterns{
		N: 20, AvgHook: 8.0, AvgClarity: 8.0, AvgBrandFit: 8.5, AvgOverall: 8.0,
		TopIssues: []repository.FieldIssue{{Field: "scene.image_prompt", Reason: "เติมโทนสีแบรนด์", Count: 9}},
	}
	baseline := repository.ScorePatterns{N: 20, AvgHook: 8.0, AvgClarity: 8.0, AvgBrandFit: 8.5, AvgOverall: 8.0}
	fire, _, _, gate := strongSignal(window, baseline)
	if !fire || gate != "frequency" {
		t.Errorf("ควรยิง frequency ได้ fire=%v gate=%s", fire, gate)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่า fail**

Run: `go test ./internal/repository/ -run TestNormalizeField -v && go test ./internal/learner/ -v`
Expected: FAIL — `undefined: NormalizeField`

- [ ] **Step 3: เขียน implementation**

ใน `internal/repository/critiques.go` เพิ่มบนสุดหลัง import:

```go
// sceneIndexPattern จับ index ของซีนในชื่อ field (`scene[5].image_prompt`)
// เพื่อยุบให้เป็นปัญหาเดียวกัน — ไม่งั้นปัญหาเดียวกันที่เกิดคนละซีนจะถูกนับแยก
// จนไม่มีวันถึงเกณฑ์ความถี่
var sceneIndexPattern = regexp.MustCompile(`\[\d+\]`)

// NormalizeField ตัด index ของซีนออกจากชื่อ field
func NormalizeField(field string) string {
	return sceneIndexPattern.ReplaceAllString(field, "")
}
```

เพิ่ม `"regexp"` ใน import

แก้ `LowScorePatterns` ให้รับ `untilDays` และจัดกลุ่มด้วย field ที่ normalize แล้ว:

```go
// LowScorePatterns aggregates clip_critiques over the window
// [now-sinceDays, now-untilDays): untilDays = 0 คือถึงปัจจุบัน ใช้ baseline
// ที่ไม่ทับ window ได้ด้วยการเรียกด้วย (90, 30)
func (r *CritiquesRepo) LowScorePatterns(ctx context.Context, sinceDays, untilDays, topN int) (ScorePatterns, error) {
	if topN <= 0 {
		topN = 10
	}
	var p ScorePatterns

	err := r.pool.QueryRow(ctx, `
SELECT
  COUNT(*)                                                       AS n,
  COALESCE(AVG((score->>'hook')::numeric),      0)              AS avg_hook,
  COALESCE(AVG((score->>'clarity')::numeric),   0)              AS avg_clarity,
  COALESCE(AVG((score->>'brand_fit')::numeric), 0)              AS avg_brand_fit,
  COALESCE(AVG((score->>'overall')::numeric),   0)              AS avg_overall
FROM clip_critiques
WHERE created_at >= NOW() - make_interval(days => $1)
  AND created_at <  NOW() - make_interval(days => $2)
  AND applied = TRUE`,
		sinceDays, untilDays,
	).Scan(&p.N, &p.AvgHook, &p.AvgClarity, &p.AvgBrandFit, &p.AvgOverall)
	if err != nil {
		return ScorePatterns{}, fmt.Errorf("aggregate score patterns: %w", err)
	}

	if p.N == 0 {
		return p, nil
	}

	// จัดกลุ่มด้วย field ที่ตัด index ซีนออกแล้วเท่านั้น — reason เป็นข้อความอิสระ
	// ที่โมเดลเขียนใหม่ทุกครั้ง จึงเก็บไว้แค่เป็นตัวอย่าง ไม่ใช้จัดกลุ่ม
	rows, err := r.pool.Query(ctx, `
SELECT
  regexp_replace(c->>'field', '\[[0-9]+\]', '', 'g') AS field,
  (array_agg(c->>'reason' ORDER BY cc.created_at DESC))[1] AS reason,
  COUNT(*) AS cnt
FROM clip_critiques cc,
     LATERAL jsonb_array_elements(cc.changes) AS c
WHERE cc.created_at >= NOW() - make_interval(days => $1)
  AND cc.created_at <  NOW() - make_interval(days => $2)
  AND cc.applied = TRUE
  AND c->>'field' IS NOT NULL
GROUP BY 1
ORDER BY cnt DESC, field ASC
LIMIT $3`,
		sinceDays, untilDays, topN,
	)
	if err != nil {
		return ScorePatterns{}, fmt.Errorf("aggregate top issues: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var fi FieldIssue
		if err := rows.Scan(&fi.Field, &fi.Reason, &fi.Count); err != nil {
			return ScorePatterns{}, fmt.Errorf("scan top issue: %w", err)
		}
		p.TopIssues = append(p.TopIssues, fi)
	}
	if err := rows.Err(); err != nil {
		return ScorePatterns{}, fmt.Errorf("iterate top issues: %w", err)
	}
	return p, nil
}
```

ใน `internal/learner/learner.go`:

เพิ่มค่าคงที่ `baselineUntilDays = 30` ในบล็อก const และแก้คอมเมนต์ของ `baselineDays` ให้บอกว่าไม่ทับ window แล้ว

แก้ interface และการเรียกใน `RunOnce`:

```go
type critiquesRepoIface interface {
	LowScorePatterns(ctx context.Context, sinceDays, untilDays, topN int) (repository.ScorePatterns, error)
}
```

```go
	patterns, err := l.critiques.LowScorePatterns(ctx, windowDays, 0, topIssuesN)
	...
	baseline, err := l.critiques.LowScorePatterns(ctx, baselineDays, baselineUntilDays, topIssuesN)
```

ใน `agentForField` ไม่ต้องแก้ — field ที่ normalize แล้ว (`scene.image_prompt`) ยังเข้าเงื่อนไข `strings.Contains(f, "image_prompt")` เหมือนเดิม

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/repository/ ./internal/learner/ -v`
Expected: PASS ทั้งหมด (เทสต์เดิมที่เรียก `LowScorePatterns` แบบเก่าต้องถูกแก้ให้ส่ง `untilDays` ด้วย)

- [ ] **Step 5: ตรวจกับข้อมูลจริงแบบอ่านอย่างเดียว**

รัน SQL ของ top issues ตัวใหม่กับ prod (`$1=30, $2=0, $3=8`) แล้วยืนยันว่าปัญหาอันดับหนึ่งมี `cnt` สูงกว่าเดิมมาก (เดิมสูงสุด 3) — ถ้ายังได้ 3 แปลว่า regexp ไม่ทำงาน ให้ตรวจ escape ของ `\[[0-9]+\]`

- [ ] **Step 6: Commit**

```bash
git add internal/repository/critiques.go internal/repository/critiques_test.go internal/learner/learner.go internal/learner/learner_test.go
git commit -m "fix(learner): baseline ไม่ทับ window + ยุบ index ซีนก่อนนับปัญหาซ้ำ"
```

---

### Task 9: ประตูที่สามของ learner — flop rate แย่ลง

**Files:**
- Modify: `internal/repository/analytics.go` (เพิ่มเมธอด)
- Modify: `internal/learner/learner.go`
- Test: `internal/learner/learner_test.go`

**Interfaces:**
- Consumes: `LowScorePatterns` ลายเซ็นใหม่ (Task 8)
- Produces:
  - `func (r *AnalyticsRepo) FlopRate(ctx context.Context, sinceDays, untilDays int) (float64, int, error)`
  - `func outcomeGate(windowFlop, baselineFlop float64, windowN, baselineN int) bool`

- [ ] **Step 1: เขียนเทสต์ที่ต้อง fail**

เพิ่มใน `internal/learner/learner_test.go`:

```go
func TestOutcomeGate(t *testing.T) {
	cases := []struct {
		name                     string
		wFlop, bFlop             float64
		wN, bN                   int
		want                     bool
	}{
		{"flop แย่ลงเกิน 0.10 → ยิง", 0.45, 0.30, 20, 20, true},
		{"flop แย่ลงนิดเดียว → ไม่ยิง", 0.35, 0.30, 20, 20, false},
		{"flop ดีขึ้น → ไม่ยิง", 0.20, 0.40, 20, 20, false},
		{"ตัวอย่าง window น้อยเกิน → ไม่ยิง", 0.60, 0.20, 3, 20, false},
		{"ตัวอย่าง baseline น้อยเกิน → ไม่ยิง", 0.60, 0.20, 20, 3, false},
	}
	for _, c := range cases {
		if got := outcomeGate(c.wFlop, c.bFlop, c.wN, c.bN); got != c.want {
			t.Errorf("%s: outcomeGate(%v,%v,%d,%d) = %v, want %v",
				c.name, c.wFlop, c.bFlop, c.wN, c.bN, got, c.want)
		}
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่า fail**

Run: `go test ./internal/learner/ -run TestOutcomeGate -v`
Expected: FAIL — `undefined: outcomeGate`

- [ ] **Step 3: เขียน implementation**

เพิ่มใน `internal/repository/analytics.go`:

```go
// FlopRate คืนสัดส่วนคลิปที่อยู่ท้ายตาราง (percentile < 0.25 ภายในแพลตฟอร์ม)
// ในช่วง [now-sinceDays, now-untilDays) พร้อมจำนวนคลิปที่นับได้
// ใช้เป็นสัญญาณ "ผลลัพธ์จริงแย่ลง" ที่ไม่ผ่านสายตาโมเดลใดๆ
func (r *AnalyticsRepo) FlopRate(ctx context.Context, sinceDays, untilDays int) (float64, int, error) {
	var rate float64
	var n int
	err := r.pool.QueryRow(ctx, `
WITH latest AS (
    SELECT DISTINCT ON (ca.clip_id, ca.platform)
        ca.clip_id, ca.platform, ca.views
    FROM clip_analytics ca
    WHERE ca.fetched_at >= NOW() - make_interval(days => $1)
      AND ca.fetched_at <  NOW() - make_interval(days => $2)
      AND ca.platform IN ('youtube','tiktok')
      AND NOT EXISTS (
          SELECT 1 FROM clip_publish_status ps
          WHERE ps.clip_id = ca.clip_id AND ps.platform = ca.platform
            AND ps.status = 'failed')
    ORDER BY ca.clip_id, ca.platform, ca.fetched_at DESC
), ranked AS (
    SELECT PERCENT_RANK() OVER (PARTITION BY l.platform ORDER BY l.views) AS pct
    FROM latest l JOIN clips c ON c.id = l.clip_id
    WHERE c.status = 'published'
)
SELECT COALESCE(COUNT(*) FILTER (WHERE pct < 0.25)::float / NULLIF(COUNT(*), 0), 0),
       COUNT(*)
FROM ranked`, sinceDays, untilDays).Scan(&rate, &n)
	if err != nil {
		return 0, 0, fmt.Errorf("flop rate: %w", err)
	}
	return rate, n, nil
}
```

ใน `internal/learner/learner.go`:

เพิ่มค่าคงที่ในบล็อก const:

```go
	// flopRegressionMargin: flop rate ของ window ต้องแย่กว่า baseline เท่านี้
	// ถึงจะถือว่าผลลัพธ์จริงถดถอย (คะแนน critic เป็นตัวแทน ยอดวิวคือของจริง)
	flopRegressionMargin = 0.10
	// minFlopSample: จำนวนคลิปขั้นต่ำในแต่ละช่วงก่อนเชื่อ flop rate
	minFlopSample = 8
```

เพิ่มฟังก์ชัน pure:

```go
// outcomeGate เปิดเมื่อสัดส่วนคลิปที่แป้กในหน้าต่างล่าสุดแย่กว่าช่วงก่อนหน้า
// อย่างมีนัย และทั้งสองช่วงมีคลิปมากพอจะเชื่อได้ Pure — ทดสอบได้โดยไม่ต้องมี DB
func outcomeGate(windowFlop, baselineFlop float64, windowN, baselineN int) bool {
	if windowN < minFlopSample || baselineN < minFlopSample {
		return false
	}
	return windowFlop-baselineFlop >= flopRegressionMargin
}
```

เพิ่ม interface และ field ใน `Learner`:

```go
// flopRepoIface คือสิ่งเดียวที่ learner ต้องการจาก analytics
type flopRepoIface interface {
	FlopRate(ctx context.Context, sinceDays, untilDays int) (float64, int, error)
}
```

เพิ่ม `flops flopRepoIface` ใน struct `Learner` และพารามิเตอร์สุดท้ายของ `New(...)` แล้วอัปเดตการเรียกใน `cmd/server/main.go`:

```go
	learnerSvc := learner.New(agentsRepo, critiquesRepo, learnerAgent, skillRevisionsRepo, analyticsRepo)
```

(ถ้า `analyticsRepo` ยังไม่มีใน main ให้สร้างด้วย `analyticsRepo := repository.NewAnalyticsRepo(pool)`)

ใน `RunOnce` หลังคำนวณ `baseline` เพิ่ม:

```go
	// ประตูที่สาม: ผลลัพธ์จริงถดถอย แม้คะแนน critic จะปกติ
	outcomeFired := false
	if l.flops != nil {
		wFlop, wN, ferr := l.flops.FlopRate(ctx, windowDays, 0)
		bFlop, bN, berr := l.flops.FlopRate(ctx, baselineDays, baselineUntilDays)
		if ferr != nil || berr != nil {
			log.Printf("learner: อ่าน flop rate ไม่ได้ (ข้าม outcome gate): %v %v", ferr, berr)
		} else {
			outcomeFired = outcomeGate(wFlop, bFlop, wN, bN)
			log.Printf("learner: outcome gate = %v (window flop %.2f n=%d, baseline flop %.2f n=%d)",
				outcomeFired, wFlop, wN, bFlop, bN)
		}
	}
```

แล้วในลูปต่อ agent เปลี่ยนการตัดสินใจให้เป็น:

```go
		ok, lowDim, lowVal, gate := strongSignal(agentPatterns, baseline)
		if !ok && outcomeFired {
			ok, gate = true, "outcome"
		}
		if !ok {
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./... `
Expected: PASS ทั้งหมด รวมเทสต์เดิมของ learner

- [ ] **Step 5: Commit**

```bash
git add internal/repository/analytics.go internal/learner/learner.go internal/learner/learner_test.go cmd/server/main.go
git commit -m "feat(learner): ประตู outcome — ยิงเมื่อสัดส่วนคลิปแป้กแย่ลงจริง"
```

---

### Task 10: เปลี่ยนอาหารของ analytics agent เป็นกระดานคะแนน

**Files:**
- Modify: `internal/analyzer/analyzer.go:37-133` (`AnalyzeAndImprove`, `gatherData`)
- Modify: `internal/analyzer/stats.go` (เพิ่มฟังก์ชัน)
- Test: `internal/analyzer/stats_test.go`

**Interfaces:**
- Consumes: `scoreboard.Score` (Task 1), `FormulaScoresRepo.Latest` (Task 3)
- Produces: `func BuildScoreboardSection(scores []scoreboard.Score) string`, `func TopBottom(stats []ClipStat, k int) []ClipStat`

- [ ] **Step 1: เขียนเทสต์ที่ต้อง fail**

เพิ่มใน `internal/analyzer/stats_test.go`:

```go
func TestTopBottomKeepsExtremesPerPlatform(t *testing.T) {
	stats := []ClipStat{
		{ID: "a", Platform: "youtube", Percentile: 0.99},
		{ID: "b", Platform: "youtube", Percentile: 0.50},
		{ID: "c", Platform: "youtube", Percentile: 0.01},
		{ID: "d", Platform: "tiktok", Percentile: 0.90},
	}
	got := TopBottom(stats, 1)
	ids := map[string]bool{}
	for _, s := range got {
		ids[s.ID] = true
	}
	if !ids["a"] || !ids["c"] {
		t.Errorf("ต้องเก็บทั้งบนสุดและล่างสุดของ youtube ได้ %v", ids)
	}
	if ids["b"] {
		t.Error("ตัวกลางต้องถูกตัดออก")
	}
	if !ids["d"] {
		t.Error("แพลตฟอร์มที่มีคลิปเดียวต้องยังอยู่")
	}
}

func TestBuildScoreboardSectionRendersEveryDimension(t *testing.T) {
	scores := []scoreboard.Score{
		{Stat: scoreboard.Stat{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30, MedianPct: 0.53, FlopRate: 0.31}, ScoreFinal: 0.61},
		{Stat: scoreboard.Stat{Dimension: "category", Value: "account-buyer", Platform: "youtube", N: 10, MedianPct: 0.60, FlopRate: 0.20}, ScoreFinal: 0.65},
	}
	out := BuildScoreboardSection(scores)
	for _, want := range []string{"content_format", "category", "qa", "account-buyer", "0.61", "0.65"} {
		if !strings.Contains(out, want) {
			t.Errorf("ผลลัพธ์ต้องมี %q\n%s", want, out)
		}
	}
}
```

เพิ่ม import `"github.com/jaochai/video-fb/internal/scoreboard"` ในไฟล์เทสต์

- [ ] **Step 2: รันเทสต์ให้เห็นว่า fail**

Run: `go test ./internal/analyzer/ -run "TestTopBottom|TestBuildScoreboard" -v`
Expected: FAIL — `undefined: TopBottom`

- [ ] **Step 3: เขียน implementation**

เพิ่มท้าย `internal/analyzer/stats.go`:

```go
// TopBottom คัดเฉพาะคลิปที่ดีที่สุดและแย่ที่สุด k ตัวของแต่ละแพลตฟอร์ม
// ตัวกลางไม่ได้สอนอะไรโมเดล และการส่งคลิปทั้งหมดไปทำให้โมเดลเสียความสนใจ
// กับรายละเอียดแทนที่จะอธิบายว่าอะไรทำให้ตัวบนชนะ
func TopBottom(stats []ClipStat, k int) []ClipStat {
	byPlatform := map[string][]ClipStat{}
	for _, s := range stats {
		byPlatform[s.Platform] = append(byPlatform[s.Platform], s)
	}
	var out []ClipStat
	for _, group := range byPlatform {
		sort.Slice(group, func(a, b int) bool { return group[a].Percentile > group[b].Percentile })
		n := len(group)
		if n <= 2*k {
			out = append(out, group...)
			continue
		}
		out = append(out, group[:k]...)
		out = append(out, group[n-k:]...)
	}
	return out
}

// BuildScoreboardSection เรนเดอร์กระดานคะแนนเป็นข้อความสำหรับใส่ในพรอมป์
// ตัวเลขถูกคำนวณมาแล้ว โมเดลจึงไม่ต้องเดาว่าสูตรไหนชนะ — หน้าที่มันคืออธิบายว่าทำไม
func BuildScoreboardSection(scores []scoreboard.Score) string {
	if len(scores) == 0 {
		return "(ยังไม่มีกระดานคะแนน)"
	}
	byDim := map[string][]scoreboard.Score{}
	var dims []string
	for _, s := range scores {
		if _, seen := byDim[s.Dimension]; !seen {
			dims = append(dims, s.Dimension)
		}
		byDim[s.Dimension] = append(byDim[s.Dimension], s)
	}
	sort.Strings(dims)

	var b strings.Builder
	for _, d := range dims {
		group := byDim[d]
		sort.Slice(group, func(i, j int) bool { return group[i].ScoreFinal > group[j].ScoreFinal })
		fmt.Fprintf(&b, "\n### %s\n", d)
		for _, s := range group {
			fmt.Fprintf(&b, "- %s (%s): คะแนน %.2f | n=%d | median วิว P%.0f | flop %.0f%%",
				s.Value, s.Platform, s.ScoreFinal, s.N, s.MedianPct*100, s.FlopRate*100)
			if s.MedianRetention > 0 {
				fmt.Fprintf(&b, " | retention %.2f", s.MedianRetention)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
```

เพิ่ม import `"github.com/jaochai/video-fb/internal/scoreboard"` ใน `stats.go`

ใน `internal/analyzer/analyzer.go`:

เพิ่ม field `formulaRepo *repository.FormulaScoresRepo` ใน struct `Analyzer` และพารามิเตอร์ท้ายของ `New(...)` แล้วอัปเดตการเรียกใน `cmd/server/main.go` (`anlz := analyzer.New(pool, llm, agentsRepo, formulaScoresRepo)`)

ใน `AnalyzeAndImprove` หลังบรรทัด `data, clipCount := BuildAnalysisData(stats)` แทรก:

```go
	// ป้อนกระดานคะแนนแทนตารางคลิปทั้งหมด: ตัวเลขว่าสูตรไหนชนะคำนวณมาแล้ว
	// โมเดลมีหน้าที่อธิบายว่าทำไม ไม่ใช่เดาความสัมพันธ์เอง
	scoreboardSection := "(ยังไม่มีกระดานคะแนน)"
	if _, scores, serr := a.formulaRepo.Latest(ctx); serr != nil {
		log.Printf("Analyzer: อ่านกระดานคะแนนไม่ได้ (ใช้ตารางคลิปอย่างเดียว): %v", serr)
	} else {
		scoreboardSection = BuildScoreboardSection(scores)
	}
	data, _ = BuildAnalysisData(TopBottom(stats, 10))
```

แล้วแก้ `userPrompt` โดยแทรกกระดานคะแนนก่อนรายการคลิป — เปลี่ยนย่อหน้าแรกของ prompt เป็น:

```go
	userPrompt := fmt.Sprintf(`Here is the performance data from our YouTube Shorts + TikTok posts for the last 14 days (n=%d clips — a small sample; calibrate your confidence accordingly).

FORMULA SCOREBOARD (60-day window, already computed — do NOT re-derive which formula wins; explain WHY the winners win):
%s

EXAMPLE CLIPS (top 10 and bottom 10 per platform only):
%s
`, clipCount, scoreboardSection, data)
```

แล้วต่อด้วยส่วน "Notes on the data:" เดิมทั้งหมดโดยไม่เปลี่ยนแปลง (รวมทั้ง `Current agent configurations:` และบล็อก JSON ที่ต้องการ)

**ระวัง:** `clipCount` ยังต้องมาจาก `BuildAnalysisData(stats)` ชุดเต็ม (ก่อนกรอง top/bottom) เพราะประตู `clipCount < 8` วัดขนาดตัวอย่างจริง ไม่ใช่จำนวนตัวอย่างที่ส่งให้โมเดล — เก็บค่าเดิมไว้ก่อนเรียกทับด้วย `data, _ =`

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./... && go build ./...`
Expected: PASS ทั้งหมด

- [ ] **Step 5: Commit**

```bash
git add internal/analyzer/analyzer.go internal/analyzer/stats.go internal/analyzer/stats_test.go cmd/server/main.go
git commit -m "feat(analyzer): ป้อนกระดานคะแนน + คลิปหัวท้ายแทนตารางดิบทั้งชุด"
```

---

### Task 11: ปล่อยขึ้น prod ตามลำดับ

**Files:** ไม่มีการแก้โค้ด — เป็นขั้นตอนปฏิบัติการ

- [ ] **Step 1: ตรวจก่อน deploy**

Run: `go test ./... && go build ./... && cd frontend && npm run build`
Expected: ผ่านทั้งหมด

ตรวจจำนวนแถวที่ enabled ให้ตรงกับตัวเลขใน migration:

```sql
SELECT count(*) FROM content_formats WHERE enabled = TRUE;   -- ต้อง = 4
SELECT count(*) FROM topic_categories WHERE enabled = TRUE;  -- ต้อง = 3
```

ถ้าไม่ตรง ให้แก้ตัวเลขใน `066_formula_scoreboard.sql` ให้ผลรวมเป็น 100 ก่อน deploy

- [ ] **Step 2: Deploy โดยที่สวิตช์ยังปิด**

push ขึ้น master ให้ Railway auto-deploy แล้วยืนยันว่า migration 066 รันผ่าน:

```sql
SELECT key, value FROM settings WHERE key = 'weight_tuner_enabled';  -- false
SELECT name, action, enabled FROM schedules WHERE action = 'tune_weights';  -- enabled = false
SELECT weight, count(*) FROM content_formats WHERE enabled = TRUE GROUP BY weight;  -- 25 x4
```

- [ ] **Step 3: คำนวณ snapshot แรกแล้วตรวจด้วยตา**

```bash
curl -s -X POST https://<prod-host>/api/v1/formula-scores/compute -H "X-API-Key: $API_KEY"
curl -s https://<prod-host>/api/v1/formula-scores -H "X-API-Key: $API_KEY"
```

ตรวจว่า: ทุกมิติมีแถว, `n` ของ `content_format` อยู่ราว 17-30 ต่อแพลตฟอร์ม, ไม่มี `score_final` เป็น 0 หรือ 1 เป๊ะ (สัญญาณว่า normalize พัง)
**ถ้าตัวเลขขัดสามัญสำนึก ให้หยุดตรงนี้ อย่าเปิดสวิตช์**

- [ ] **Step 4: เปิดการหมุนน้ำหนัก**

```sql
UPDATE settings SET value = 'true' WHERE key = 'weight_tuner_enabled';
```

เปิด schedule ผ่าน API เท่านั้น (scheduler โหลดใหม่จาก API เท่านั้น การ UPDATE DB ตรงๆ จะไม่มีผลจนกว่าจะ restart):

```bash
curl -X PATCH https://<prod-host>/api/v1/schedules/<id> -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" -d '{"enabled": true}'
```

- [ ] **Step 5: ตรวจผลรอบแรก (จันทร์ถัดไป)**

```sql
SELECT dimension, value, old_weight, new_weight, n, score_final
FROM weight_revisions ORDER BY created_at DESC LIMIT 20;
```

ยืนยัน: ไม่มีค่าไหนเปลี่ยนเกิน ~7 หน่วย (25% ของระยะทางสูงสุด), ไม่มีค่าไหนต่ำกว่า 12 (`content_format`) หรือ 16 (`category`), ผลรวมต่อมิติ = 100
ถ้าผิดจากนี้: `curl -X POST .../api/v1/weights/rollback` แล้วปิดสวิตช์

---

## Self-Review

**ความครอบคลุมเทียบ spec:**

| หัวข้อ spec | Task ที่ทำ |
|---|---|
| 5 กระดานคะแนน (นิยาม, สูตร, shrinkage) | 1, 3 |
| 6 ตัวหมุนน้ำหนัก + เบรก 4 ชั้น | 2, 4 |
| 7 preset — เลื่อนออก | (ตั้งใจไม่ทำ ระบุใน Global Constraints) |
| 8 ฝั่งข้อความ: analytics agent | 10 |
| 8 ฝั่งข้อความ: learner 3 จุด | 8 (จุด 1-2), 9 (จุด 3) |
| 9 audit + rollback + endpoint อ่าน | 3, 4, 6, 7 |
| 10 error handling | ฝังในทุก task (audit-first, ล้มทีละมิติ, log เหตุผลที่ข้าม) |
| 11 testing | 1, 2, 4, 8, 9, 10 |
| 12 migration + ลำดับปล่อย | 3, 11 |

**ช่องว่างที่ยอมรับไว้:** spec หัวข้อ 8 บอกว่า `LearnInput` ควรมีกระดานคะแนนของ agent นั้นด้วย — แผนนี้ยังไม่ทำ เพราะ `formula_scores` แยกตามสูตรที่ใช้ผลิต ไม่ได้แยกตาม agent จึงแม็ปเข้า `scene`/`script` ไม่ได้ตรงๆ ถ้าต้องการจริงต้องออกแบบการแม็ปเพิ่ม — ทำเป็นรอบถัดไปหลังเห็นว่า outcome gate ยิงจริงหรือไม่

**ความสอดคล้องของชนิดข้อมูล:** `Stat`/`Score` (Task 1) ถูกใช้ซ้ำใน repository (Task 3), service (Task 4) และ analyzer (Task 10) โดยไม่มีการนิยามซ้ำ · `LowScorePatterns` เปลี่ยนลายเซ็นใน Task 8 และผู้เรียกทั้งหมดอยู่ใน Task 8 เดียวกัน · `models.WeightRevision` นิยามครั้งเดียวใน Task 3 ใช้ใน Task 4, 6, 7
