# Render protocolTimeout Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ทำให้การเรนเดอร์ 9:16 บน Railway ไม่ค้างจนตาย `protocolTimeout` อีก โดยแก้ที่ต้นเหตุ (hyperframes 0.6.70 อ่าน RAM ของโฮสต์แทนลิมิต cgroup ของคอนเทนเนอร์) พร้อมติดเครื่องวัดที่บอกได้ว่าครั้งหน้าตันที่แรมหรือดิสก์ และกู้คลิป `8885bcc8` ก่อนถูกลบอัตโนมัติ

**Architecture:** ห้าอย่างที่แยกกันได้: (1) สนามซ้อมเรนเดอร์ในเครื่องจาก scene rows จริง — ใช้ตัดสินว่าเวอร์ชันใหม่ของ CLI ปลอดภัยไหม (2) log สภาพเครื่อง (cgroup memory + ที่ว่างดิสก์) คร่อมและระหว่างขั้นเรนเดอร์ (3) ลบโฟลเดอร์งานของคลิปที่ผลิตสำเร็จ (4) อัป hyperframes ข้าม v0.6.99 ซึ่งเป็นเวอร์ชันที่ต้นน้ำแก้ "Respect cgroup memory limits in low-memory detection" + ปิดฝาค่าที่ override ได้ผ่าน ENV ใน Dockerfile (5) ดีพลอยแล้วกู้คลิป

**Tech Stack:** Go 1.25.6 (stdlib ล้วน ไม่เพิ่ม dependency), hyperframes CLI (Node 22), Docker/Debian bookworm snapshot 2026-07-04 + chromium 149, Railway, Neon Postgres

## Global Constraints

- ห้ามเพิ่ม dependency ใหม่ใน `go.mod` — ทุกอย่างในแผนนี้ใช้ stdlib
- `hyperframesVersion` ใน `internal/producer/hyperframes.go` กับเลขเวอร์ชันใน `Dockerfile` **ต้องตรงกันเสมอ** (คอมเมนต์ใน Dockerfile ระบุกฎนี้ไว้แล้ว)
- เวอร์ชันเป้าหมายของ hyperframes ต้อง **≥ 0.6.99** (เวอร์ชันที่ต้นน้ำเริ่มอ่าน cgroup memory limit) — ต่ำกว่านี้ไม่แก้ปัญหา
- คอมเมนต์โค้ดใหม่เขียนภาษาไทยแบบเดียวกับไฟล์ที่แก้ (โค้ดเบสนี้ผสมไทย/อังกฤษ — ตามไฟล์ที่แตะ)
- ห้ามดีพลอย/ตั้งค่า Railway variable ขณะมีคลิปกำลังผลิต (ตัวแปรเปลี่ยน = Railway รีดีพลอย = ฆ่าการผลิตที่ค้างอยู่)
- ตารางผลิตบน prod (UTC): 02:00, 05:00, 08:00, 11:00, 14:00 — เลือกดีพลอยในช่องว่างและก่อนรอบถัดไปอย่างน้อย 20 นาที
- ห้ามยิง `/api/v1/orchestrator/produce*` เพื่อทดสอบ — endpoint พวกนี้ผลิตคลิปจริงบน prod
- **เส้นตาย:** คลิป `8885bcc8-9c23-475d-aaf5-21af48959b77` (status=failed, retry_count=2, updated_at 2026-08-20T05:36Z) จะถูก `DeleteOldFailed` ลบทิ้งเมื่อครบ 24 ชม. ≈ **2026-08-21T05:36Z (12:36 น. ไทย)** — Task 5 ต้องเสร็จก่อนเวลานี้

---

## File Structure

| ไฟล์ | สถานะ | หน้าที่ |
|---|---|---|
| `internal/producer/testdata/clip_8885bcc8_scenes.json` | มีแล้ว (ยังไม่ commit) | scene rows จริงของคลิปที่พัง 20 ส.ค. |
| `internal/producer/testdata/clip_bbb42f7b_scenes.json` | มีแล้ว (ยังไม่ commit) | scene rows ของคลิปคุมเทียบ (preset เดียวกัน ผ่านปกติ) |
| `internal/producer/repro_case_render_test.go` | มีแล้ว (ยังไม่ commit) + ต่อเติมใน Task 1 | สนามซ้อม: สร้างโปรเจกต์จาก fixture แล้วรัน render/lint/inspect |
| `internal/producer/hostmetrics.go` | สร้างใหม่ (Task 2) | อ่าน cgroup memory + ที่ว่างดิสก์ → บรรทัด log |
| `internal/producer/hostmetrics_test.go` | สร้างใหม่ (Task 2) | เทสต์ตัวแปลงค่า cgroup (ฟังก์ชันบริสุทธิ์) |
| `internal/producer/hyperframes.go` | แก้ (Task 2, 4) | เรียก host snapshot คร่อม Render + เลข `hyperframesVersion` |
| `internal/producer/producer.go` | แก้ (Task 3) | เพิ่ม `CleanupClipDir` |
| `internal/producer/producer_cleanup_test.go` | สร้างใหม่ (Task 3) | เทสต์ `CleanupClipDir` |
| `internal/orchestrator/orchestrator.go` | แก้ (Task 3) | เรียก `CleanupClipDir` ท้าย `renderAndFinalize` |
| `Dockerfile` | แก้ (Task 4) | เลขเวอร์ชัน hyperframes + ENV ปิดฝาแคชเฟรม/protocolTimeout |

---

### Task 1: สนามซ้อมเรนเดอร์จากคลิปจริง (เครื่องมือที่ Task 4 ใช้ตัดสิน)

ไฟล์ทั้งสามมีอยู่ในเครื่องแล้วจากการสืบสวน แต่ยังไม่ได้ commit และยังรันได้แค่ขั้น render — Task นี้เติมขั้น lint/inspect เข้าไปเพื่อให้ Task 4 เทียบพฤติกรรมของ CLI สองเวอร์ชันได้ครบทุกด่านที่ prod ใช้จริง

**Files:**
- Modify: `internal/producer/repro_case_render_test.go` (เติมฟังก์ชันเทสต์ที่สอง)
- Existing (commit ในงานนี้): `internal/producer/testdata/clip_8885bcc8_scenes.json`, `internal/producer/testdata/clip_bbb42f7b_scenes.json`

**Interfaces:**
- Consumes: `realClipScenes`, `realClipBounds` (มีอยู่แล้วใน `composition_realclip_render_test.go`), `buildSceneSpecs`, `captionSegmentsFromScenes`, `CaseFilePreset`, `ModeCase`, `NewCompositionBuilder`, `NewHyperframesRenderer`
- Produces: `reproCaseParams(t *testing.T, fixture string, caseNumber int) ScenesParams`, `writeSilentWAV(t *testing.T, path string, seconds float64)`, เทสต์ `TestReproCaseRender` และ `TestReproCaseChecks` — ทั้งคู่ข้ามเมื่อไม่ได้ตั้ง `HF_RENDER=1`

- [ ] **Step 1: ดูว่าสนามซ้อมเดิมรันผ่านจริงก่อนแตะอะไร**

```bash
PATH="$(dirname $(ls -d ~/.npm/_npx/*/node_modules/hyperframes 2>/dev/null | head -1))/.bin:$PATH" \
HF_RENDER=1 SCENE_MOTION_V2_ENABLED=true COVER_SCENE_ENABLED=true \
PUPPETEER_EXECUTABLE_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
go test ./internal/producer/ -run TestReproCaseRender -count=1 -v -timeout 40m
```

Expected: PASS ทั้งสอง subtest (`fail_8885bcc8` ~30 วิ, `ok_bbb42f7b` ~24 วิ)

> หมายเหตุ: คำสั่ง `go test`/`npm` ในแผนนี้ต้องรัน **นอกแซนด์บ็อกซ์** — go build cache กับ npm cache อยู่นอกพื้นที่ที่แซนด์บ็อกซ์เขียนได้ (อาการ: `operation not permitted`)

- [ ] **Step 2: เขียนเทสต์ที่ล้ม — ด่าน lint + inspect**

เติมท้าย `internal/producer/repro_case_render_test.go`:

```go
// TestReproCaseChecks รันด่าน lint กับ inspect ชุดเดียวกับที่ prod รันก่อนเรนเดอร์
// แยกจาก TestReproCaseRender เพราะตอนอัปเวอร์ชัน CLI สิ่งที่ต้องเทียบไม่ใช่แค่
// "เรนเดอร์ออกไหม" แต่คือ "inspect ตีตกเพิ่มไหม" — inspect เป็นด่านที่ส่งคลิปไป
// needs_review ได้จริง การอัปเวอร์ชันที่เข้มขึ้นเงียบๆ จะกักคลิปทั้งสายโดยไม่มีใครรู้
func TestReproCaseChecks(t *testing.T) {
	if os.Getenv("HF_RENDER") != "1" {
		t.Skip("set HF_RENDER=1 to run the checks harness")
	}
	for _, c := range reproFixtures() {
		t.Run(c.name, func(t *testing.T) {
			params := reproCaseParams(t, c.fixture, c.caseNum)
			dir := t.TempDir()
			voice := filepath.Join(t.TempDir(), "voice.wav")
			writeSilentWAV(t, voice, params.DurationSeconds)

			builder := NewCompositionBuilder("assets/fonts")
			if _, err := builder.BuildScenes(params, c.name, dir, voice, map[int]string{}); err != nil {
				t.Fatalf("build scenes: %v", err)
			}

			r := NewHyperframesRenderer()
			lintRes, lintErr := r.Lint(context.Background(), dir)
			t.Logf("%s lint: passed=%v %dms findings=%v", c.name, lintRes.Passed, lintRes.DurationMS, lintRes.Findings)
			inspectRes, inspectErr := r.Inspect(context.Background(), dir)
			t.Logf("%s inspect: passed=%v %dms findings=%v", c.name, inspectRes.Passed, inspectRes.DurationMS, inspectRes.Findings)
			if lintErr != nil {
				t.Logf("%s lint error: %v", c.name, lintErr)
			}
			if inspectErr != nil {
				t.Errorf("%s inspect flagged (เทียบกับผลของเวอร์ชันเดิมก่อนสรุปว่าเป็น regression): %v", c.name, inspectErr)
			}
		})
	}
}
```

- [ ] **Step 3: ดึงรายการ fixture ออกมาเป็นฟังก์ชันเดียวให้เทสต์ทั้งสองใช้ร่วมกัน**

ใน `repro_case_render_test.go` แทนที่ตัวแปร `cases := []struct{...}{...}` ที่อยู่ใน `TestReproCaseRender` ด้วยการเรียก `reproFixtures()` และเพิ่มฟังก์ชันนี้ไว้เหนือ `TestReproCaseRender`:

```go
type reproFixture struct {
	name    string
	fixture string
	caseNum int
}

// reproFixtures คือคลิปคู่เทียบ: ตัวที่ prod เรนเดอร์ค้าง 20 ส.ค. 2026 กับตัวที่
// preset/รูปแบบเดียวกันแต่ผ่านปกติเมื่อ 19 ส.ค. — ต้องรันคู่กันเสมอ ตัวเลขของ
// ตัวเดียวไม่บอกอะไร
func reproFixtures() []reproFixture {
	return []reproFixture{
		{"fail_8885bcc8", "clip_8885bcc8_scenes.json", 247},
		{"ok_bbb42f7b", "clip_bbb42f7b_scenes.json", 246},
	}
}
```

และใน `TestReproCaseRender` เปลี่ยนหัวลูปเป็น:

```go
	only := os.Getenv("HF_ONLY")
	for _, c := range reproFixtures() {
		if only != "" && only != c.name {
			continue
		}
```

- [ ] **Step 4: รันเทสต์ใหม่ให้เห็นผลจริงของเวอร์ชันปัจจุบัน (0.6.70)**

```bash
PATH="/Users/jaochai/.npm/_npx/110f701c48e68d66/node_modules/.bin:$PATH" \
HF_RENDER=1 SCENE_MOTION_V2_ENABLED=true COVER_SCENE_ENABLED=true \
PUPPETEER_EXECUTABLE_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
go test ./internal/producer/ -run 'TestReproCase' -count=1 -v -timeout 40m 2>&1 | tee /tmp/claude/hf-0.6.70-baseline.txt
```

Expected: `TestReproCaseRender` PASS ทั้งคู่ · `TestReproCaseChecks` แสดงบรรทัด `lint:` และ `inspect:` ของทั้งสองคลิป — **เก็บไฟล์ผลนี้ไว้เป็นฐานเทียบของ Task 4** (ถ้า inspect ของเวอร์ชันเดิมตีตกอยู่แล้ว ให้บันทึกไว้ว่าตีตกอะไร ไม่ใช่ถือว่าเทสต์พัง)

- [ ] **Step 5: รันเทสต์ทั้งแพ็กเกจว่าไม่พังของเดิม**

```bash
go test ./internal/producer/ -count=1
```

Expected: `ok` (สนามซ้อมข้ามตัวเองเมื่อไม่มี `HF_RENDER=1`)

- [ ] **Step 6: /simplify กับ diff แล้ว commit**

รัน `/simplify` กับ diff ของงานนี้ก่อน แล้วค่อย:

```bash
git add internal/producer/repro_case_render_test.go internal/producer/testdata/clip_8885bcc8_scenes.json internal/producer/testdata/clip_bbb42f7b_scenes.json
git commit -m "test(producer): สนามซ้อมเรนเดอร์จาก scene rows จริง 2 คลิป (ตัวที่ค้าง + ตัวคุมเทียบ)"
```

---

### Task 2: log สภาพเครื่องคร่อมและระหว่างขั้นเรนเดอร์

เหตุที่ต้องมี: ตอนเรนเดอร์ค้าง 20 ส.ค. เราไม่มีตัวเลขแรม/ดิสก์ของคอนเทนเนอร์เลยสักตัว จึงแยกไม่ออกว่าตันที่อะไร · `lint`/`inspect` ที่เป็นงานสั้นยังเร็วปกติ แต่ขั้นเรนเดอร์ที่กินเครื่องยาวๆ แพงขึ้น ~60% ใน 12 วัน — ต้องวัดของจริงไม่ใช่เดาต่อ

**Files:**
- Create: `internal/producer/hostmetrics.go`
- Create: `internal/producer/hostmetrics_test.go`
- Modify: `internal/producer/hyperframes.go` (ฟังก์ชัน `Render`, ~บรรทัด 204-207)

**Interfaces:**
- Consumes: —
- Produces:
  - `parseCgroupBytes(content string) (int64, bool)`
  - `readFirstCgroupBytes(paths []string) (int64, bool)`
  - `diskFreeMB(dir string) (int64, bool)`
  - `hostSnapshot(dir string) string` — เช่น `mem 2143/8192MB disk_free 11820MB`
  - `logHostDuring(tag, dir string, interval time.Duration, stop <-chan struct{})`

- [ ] **Step 1: เขียนเทสต์ที่ล้ม**

สร้าง `internal/producer/hostmetrics_test.go`:

```go
package producer

import (
	"strings"
	"testing"
)

func TestParseCgroupBytes(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    int64
		wantOK  bool
	}{
		{"cgroup v2 ไม่จำกัด", "max\n", 0, false},
		{"ค่าปกติ 8GB", "8589934592\n", 8589934592, true},
		{"มีช่องว่างหน้าหลัง", "  1048576 \n", 1048576, true},
		{"sentinel ของ v1 = ไม่จำกัด", "9223372036854771712", 0, false},
		{"ว่าง", "", 0, false},
		{"ไม่ใช่ตัวเลข", "abc", 0, false},
		{"ศูนย์", "0", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseCgroupBytes(c.in)
			if got != c.want || ok != c.wantOK {
				t.Errorf("parseCgroupBytes(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.wantOK)
			}
		})
	}
}

func TestHostSnapshotAlwaysReportsDisk(t *testing.T) {
	got := hostSnapshot(t.TempDir())
	if !strings.Contains(got, "disk_free") {
		t.Errorf("hostSnapshot = %q, ต้องมี disk_free เสมอ (statfs ใช้ได้ทุก OS ที่เรารัน)", got)
	}
	if !strings.Contains(got, "mem ") {
		t.Errorf("hostSnapshot = %q, ต้องมีช่อง mem เสมอ (นอก Linux ให้เป็น n/a ไม่ใช่หายไป)", got)
	}
}
```

- [ ] **Step 2: รันให้เห็นว่าล้ม**

```bash
go test ./internal/producer/ -run 'TestParseCgroupBytes|TestHostSnapshot' -count=1
```

Expected: FAIL — `undefined: parseCgroupBytes`, `undefined: hostSnapshot`

- [ ] **Step 3: เขียนโค้ดให้ผ่าน**

สร้าง `internal/producer/hostmetrics.go`:

```go
package producer

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// เส้นทางลิมิต/การใช้หน่วยความจำของ cgroup ตามที่ "มองจากในคอนเทนเนอร์" — v2 ก่อน
// แล้วค่อยถอยไป v1 · ตั้งใจไม่ไล่ตาม /proc/self/cgroup เพราะบนเครื่องจริงลิมิตอาจอยู่
// ใน slice ซ้อน ซึ่งไม่ใช่เคสที่เราสนใจ (เราสนใจคอนเทนเนอร์ Railway)
var cgroupMemCurrentPaths = []string{
	"/sys/fs/cgroup/memory.current",
	"/sys/fs/cgroup/memory/memory.usage_in_bytes",
}

var cgroupMemMaxPaths = []string{
	"/sys/fs/cgroup/memory.max",
	"/sys/fs/cgroup/memory/memory.limit_in_bytes",
}

// parseCgroupBytes แปลงเนื้อไฟล์ cgroup memory เป็นไบต์ · "max" คือไม่จำกัด และ
// cgroup v1 ใช้ตัวเลขมหึมา (2^63-1 ปัดหน้าเพจ) แทนคำว่าไม่จำกัด — ทั้งสองแบบคืน false
func parseCgroupBytes(content string) (int64, bool) {
	s := strings.TrimSpace(content)
	if s == "" || s == "max" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 || n >= 1<<60 {
		return 0, false
	}
	return n, true
}

// readFirstCgroupBytes อ่านเส้นทางแรกที่อ่านได้และให้ค่าที่ใช้ได้จริง
func readFirstCgroupBytes(paths []string) (int64, bool) {
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if n, ok := parseCgroupBytes(string(b)); ok {
			return n, true
		}
	}
	return 0, false
}

// diskFreeMB คืนที่ว่าง (MB) ของ filesystem ที่ dir อยู่
func diskFreeMB(dir string) (int64, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, false
	}
	return int64(st.Bavail) * int64(st.Bsize) / (1024 * 1024), true
}

// hostSnapshot สรุปสภาพเครื่องเป็นบรรทัดเดียวสำหรับ log — เขียนไว้เพราะตอนเรนเดอร์
// ค้างจน protocolTimeout (20 ส.ค. 2026) เราไม่มีตัวเลขแรมหรือดิสก์ของคอนเทนเนอร์
// สักตัว จึงสรุปไม่ได้ว่าตันที่อะไร ครั้งหน้าต้องมี
func hostSnapshot(dir string) string {
	mem := "n/a"
	if cur, ok := readFirstCgroupBytes(cgroupMemCurrentPaths); ok {
		if max, okMax := readFirstCgroupBytes(cgroupMemMaxPaths); okMax {
			mem = fmt.Sprintf("%d/%dMB", cur/(1024*1024), max/(1024*1024))
		} else {
			mem = fmt.Sprintf("%dMB/unlimited", cur/(1024*1024))
		}
	}
	disk := "n/a"
	if free, ok := diskFreeMB(dir); ok {
		disk = fmt.Sprintf("%dMB", free)
	}
	return fmt.Sprintf("mem %s disk_free %s", mem, disk)
}

// hostSampleInterval คือจังหวะสุ่มวัดระหว่างเรนเดอร์ · 30 วินาทีให้ภาพพอเห็นการไต่
// ของหน่วยความจำในงานที่ปกติใช้ 75-180 วินาที โดยไม่ทำให้ log ท่วม
const hostSampleInterval = 30 * time.Second

// logHostDuring ยิง log สภาพเครื่องเป็นระยะจนกว่า stop จะถูกปิด
func logHostDuring(tag, dir string, interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			log.Printf("host %s: %s", tag, hostSnapshot(dir))
		}
	}
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

```bash
go test ./internal/producer/ -run 'TestParseCgroupBytes|TestHostSnapshot' -count=1 -v
```

Expected: PASS ทุก subtest

- [ ] **Step 5: ต่อสายเข้าขั้นเรนเดอร์**

ใน `internal/producer/hyperframes.go` แทนที่ฟังก์ชัน `Render` เดิมทั้งก้อนด้วย:

```go
// Render produces an MP4 at outputPath from the composition in dir. Quality is
// standard/24fps so the memory-heavy multi-scene render fits the ~8GB container
// without OOM. ค่าสภาพเครื่องถูก log คร่อมและระหว่างทาง — ขั้นนี้เป็นขั้นเดียวที่
// เคยค้างจนตาย และตอนนั้นเราไม่มีตัวเลขอะไรจะอ่านย้อนหลังเลย
func (h *HyperframesRenderer) Render(ctx context.Context, dir, outputPath string) (CheckResult, error) {
	log.Printf("host render-start: %s", hostSnapshot(dir))
	stop := make(chan struct{})
	go logHostDuring("render", dir, hostSampleInterval, stop)

	res, err := h.runCheck(ctx, "render", h.timeout, dir,
		"render", "--output", outputPath, "--quality", "standard", "--fps", "24", "-w", renderWorkers)

	close(stop)
	log.Printf("host render-end: %s", hostSnapshot(dir))
	return res, err
}
```

- [ ] **Step 6: รันสนามซ้อมเพื่อดูว่า log ออกจริงและเรนเดอร์ไม่พัง**

```bash
PATH="/Users/jaochai/.npm/_npx/110f701c48e68d66/node_modules/.bin:$PATH" \
HF_RENDER=1 HF_ONLY=fail_8885bcc8 SCENE_MOTION_V2_ENABLED=true COVER_SCENE_ENABLED=true \
PUPPETEER_EXECUTABLE_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
go test ./internal/producer/ -run TestReproCaseRender -count=1 -v -timeout 40m
```

Expected: PASS และเห็นบรรทัด `host render-start: mem n/a disk_free ...MB` กับ `host render-end: ...` (บนแมคช่อง mem เป็น n/a ถูกต้องแล้ว — บน Linux/คอนเทนเนอร์ถึงจะมีตัวเลข)

- [ ] **Step 7: รันเทสต์ทั้งแพ็กเกจ**

```bash
go test ./internal/producer/ ./internal/orchestrator/ -count=1
```

Expected: `ok` ทั้งสองแพ็กเกจ

- [ ] **Step 8: /simplify กับ diff แล้ว commit**

```bash
git add internal/producer/hostmetrics.go internal/producer/hostmetrics_test.go internal/producer/hyperframes.go
git commit -m "feat(producer): log หน่วยความจำ cgroup + ที่ว่างดิสก์ คร่อมและระหว่างขั้นเรนเดอร์"
```

---

### Task 3: ลบโฟลเดอร์งานของคลิปที่ผลิตสำเร็จ

`/tmp/adsvance-output/<clipID>` ไม่เคยถูกลบเลย — กองสะสมทั้งอายุคอนเทนเนอร์ (รอบล่าสุดคือ 5 วัน ~19 คลิป) แต่ละคลิปมี voice wav ต่อฉาก + voice รวม + ambient + โปรเจกต์ composition + mp4 · คลิปที่ **ล้มเหลว** ต้องไม่ถูกลบ เพราะเส้นทาง resume ใช้ไฟล์ภาพเดิมซ้ำ

**Files:**
- Modify: `internal/producer/producer.go` (เพิ่มเมธอดต่อท้าย `uploadPersistent`, ~บรรทัด 561)
- Create: `internal/producer/producer_cleanup_test.go`
- Modify: `internal/orchestrator/orchestrator.go` (ท้าย `renderAndFinalize` ก่อน `return nil` บรรทัด ~835)

**Interfaces:**
- Consumes: `Producer.workDir` (ฟิลด์ที่มีอยู่แล้ว), `o.producer` ใน orchestrator เป็น `*producer.Producer` (ชนิดจริง ไม่ใช่ interface — เพิ่มเมธอดแล้วเรียกได้เลย)
- Produces: `func (p *Producer) CleanupClipDir(clipID string) error`

- [ ] **Step 1: เขียนเทสต์ที่ล้ม**

สร้าง `internal/producer/producer_cleanup_test.go`:

```go
package producer

import (
	"os"
	"path/filepath"
	"testing"
)

// newCleanupProducer สร้าง Producer ที่มีแค่ workDir — CleanupClipDir ไม่แตะ
// dependency อื่นเลย จึงส่ง nil ได้ทั้งหมด
func newCleanupProducer(t *testing.T) (*Producer, string) {
	t.Helper()
	work := t.TempDir()
	return NewProducer(nil, nil, nil, nil, nil, "", work, nil), work
}

func TestCleanupClipDirRemovesClipFiles(t *testing.T) {
	p, work := newCleanupProducer(t)
	clipDir := filepath.Join(work, "clip-1")
	if err := os.MkdirAll(filepath.Join(clipDir, "composition-916"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clipDir, "voice.wav"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := p.CleanupClipDir("clip-1"); err != nil {
		t.Fatalf("CleanupClipDir: %v", err)
	}
	if _, err := os.Stat(clipDir); !os.IsNotExist(err) {
		t.Errorf("โฟลเดอร์คลิปต้องหายไป แต่ stat ได้ err=%v", err)
	}
}

func TestCleanupClipDirRefusesEmptyID(t *testing.T) {
	p, work := newCleanupProducer(t)
	other := filepath.Join(work, "clip-2")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := p.CleanupClipDir("  "); err == nil {
		t.Error("clipID ว่างต้องคืน error ไม่ใช่ลบ workDir ทั้งก้อน")
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("workDir ต้องไม่ถูกแตะเมื่อ clipID ว่าง: %v", err)
	}
}
```

- [ ] **Step 2: รันให้เห็นว่าล้ม**

```bash
go test ./internal/producer/ -run TestCleanupClipDir -count=1
```

Expected: FAIL — `p.CleanupClipDir undefined`

- [ ] **Step 3: เขียนเมธอด**

เพิ่มใน `internal/producer/producer.go` ต่อจากฟังก์ชัน `uploadPersistent`:

```go
// CleanupClipDir ลบโฟลเดอร์งานของคลิปหนึ่งใบ · เรียกเมื่อผลิตสำเร็จเท่านั้น —
// วิดีโอกับปกขึ้น R2 ไปแล้ว ไฟล์ที่เหลือในเครื่องไม่มีใครใช้ต่อ แต่ก่อนหน้านี้ไม่มี
// ใครลบ มันจึงกองอยู่ใน /tmp จนกว่าคอนเทนเนอร์จะรีสตาร์ต · คลิปที่ล้มเหลวต้องไม่
// ถูกลบ เพราะเส้นทาง resume ใช้ภาพพื้นหลังเดิมซ้ำแทนที่จะจ่ายค่าเจนใหม่
//
// clipID ว่างเป็น error ไม่ใช่ no-op: filepath.Join(workDir, "") = workDir และ
// RemoveAll จะกวาดงานของทุกคลิปทิ้ง
func (p *Producer) CleanupClipDir(clipID string) error {
	if strings.TrimSpace(clipID) == "" {
		return fmt.Errorf("CleanupClipDir: clipID ว่าง")
	}
	return os.RemoveAll(filepath.Join(p.workDir, clipID))
}
```

(ไฟล์นี้ import `strings`, `fmt`, `os`, `path/filepath` อยู่แล้ว — ไม่ต้องเพิ่ม import)

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

```bash
go test ./internal/producer/ -run TestCleanupClipDir -count=1 -v
```

Expected: PASS ทั้งสองเทสต์

- [ ] **Step 5: เรียกจากท้ายเส้นทางผลิตที่สำเร็จ**

ใน `internal/orchestrator/orchestrator.go` ฟังก์ชัน `renderAndFinalize` แทรกก่อน `return nil` บรรทัดสุดท้าย (ถัดจากบล็อก `if status == "ready" { log.Printf("Clip ready (hyperframes): %s", clipID) }`):

```go
	// ผลิตสำเร็จแล้ว (เส้นทางล้มเหลวคืนค่าไปตั้งแต่ failClip ด้านบน) — เก็บกวาดไฟล์งาน
	// ของคลิปนี้ · ล้มเหลวตรงนี้ไม่ใช่เรื่องใหญ่พอจะทำให้คลิปที่เสร็จแล้วกลายเป็นคลิปพัง
	if cErr := o.producer.CleanupClipDir(clipID); cErr != nil {
		log.Printf("cleanup clip dir failed (non-fatal) for clip %s: %v", clipID, cErr)
	}
	return nil
```

- [ ] **Step 6: รันเทสต์ทั้งสองแพ็กเกจ**

```bash
go test ./internal/producer/ ./internal/orchestrator/ -count=1
```

Expected: `ok` ทั้งคู่

- [ ] **Step 7: /simplify กับ diff แล้ว commit**

```bash
git add internal/producer/producer.go internal/producer/producer_cleanup_test.go internal/orchestrator/orchestrator.go
git commit -m "feat(producer): ลบโฟลเดอร์งานของคลิปหลังผลิตสำเร็จ (คลิปที่ล้มเหลวยังเก็บไว้ให้ resume)"
```

---

### Task 4: อัป hyperframes ข้าม v0.6.99 + ปิดฝาค่าหน่วยความจำใน Dockerfile

หัวใจของแผน · 0.6.70 คำนวณค่าที่ปรับตามแรมจาก `os.totalmem()` ซึ่งในคอนเทนเนอร์คือแรมของ **โฮสต์** ไม่ใช่ลิมิต cgroup — ผลคือ 3 จุดตั้งค่าเกินตัวคอนเทนเนอร์: `getLowMemoryFlags()` ไม่จำกัด heap ของ Chrome เลย, `getGpuMemBudgetMb()` แจ้งงบหน่วยความจำ = ครึ่งหนึ่งของแรมโฮสต์, และแคชเฟรมเป็น 256 ใบ/1500MB แทน 64 ใบ/256MB · ต้นน้ำแก้ใน **v0.6.99 "Respect cgroup memory limits in low-memory detection"** · ENV override มีเฉพาะแคชเฟรมกับ protocolTimeout อีกสองจุดต้องอัปเวอร์ชันเท่านั้น

**Files:**
- Modify: `internal/producer/hyperframes.go` บรรทัด 16 (`hyperframesVersion`)
- Modify: `Dockerfile` (บรรทัด `RUN npx --yes hyperframes@0.6.70 --version` + คอมเมนต์เหนือมัน + บล็อก ENV)

**Interfaces:**
- Consumes: สนามซ้อมจาก Task 1 (`TestReproCaseRender`, `TestReproCaseChecks`) และไฟล์ฐานเทียบ `/tmp/claude/hf-0.6.70-baseline.txt`
- Produces: ค่าคงที่ `hyperframesVersion` ค่าใหม่ที่ Dockerfile ต้องสะกดให้ตรง

- [ ] **Step 1: ติดตั้งเวอร์ชันผู้สมัครไว้ในที่ชั่วคราว**

npm cache หลักของเครื่องนี้มีไฟล์ของ root ปนอยู่ (`EPERM`) จึงต้องชี้ cache ไปที่อื่น:

```bash
mkdir -p /tmp/claude/hfcand && cd /tmp/claude/hfcand
npm install --cache /tmp/claude/npm-cache --prefix /tmp/claude/hfcand/v0790 hyperframes@0.7.90
npm install --cache /tmp/claude/npm-cache --prefix /tmp/claude/hfcand/v0699 hyperframes@0.6.99
cd /Users/jaochai/Code/video-fb
```

Expected: มีไฟล์ `/tmp/claude/hfcand/v0790/node_modules/.bin/hyperframes` และ `/tmp/claude/hfcand/v0699/node_modules/.bin/hyperframes`

- [ ] **Step 2: รันสนามซ้อมกับตัวใหม่สุดก่อน (0.7.90)**

```bash
PATH="/tmp/claude/hfcand/v0790/node_modules/.bin:$PATH" \
HF_RENDER=1 SCENE_MOTION_V2_ENABLED=true COVER_SCENE_ENABLED=true \
PUPPETEER_EXECUTABLE_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
go test ./internal/producer/ -run 'TestReproCase' -count=1 -v -timeout 40m 2>&1 | tee /tmp/claude/hf-0.7.90.txt
```

Expected: `TestReproCaseRender` PASS ทั้งสองคลิป และมี `output.mp4` ออกจริง

- [ ] **Step 3: เทียบผลกับฐานของ 0.6.70 แล้วตัดสินเวอร์ชันเป้าหมาย**

```bash
diff <(grep -E 'lint:|inspect:|render took' /tmp/claude/hf-0.6.70-baseline.txt) \
     <(grep -E 'lint:|inspect:|render took' /tmp/claude/hf-0.7.90.txt)
```

**กฎการตัดสิน:**
- render PASS ทั้งสองคลิป **และ** `inspect` ไม่ตีตกเพิ่มจากฐาน → ใช้ **0.7.90**
- render พัง หรือ inspect ตีตกเพิ่ม → รัน Step 2 ซ้ำโดยเปลี่ยน PATH เป็น `/tmp/claude/hfcand/v0699/node_modules/.bin` แล้วใช้ **0.6.99** (ขั้นต่ำที่มีการแก้ cgroup) ถ้าผ่าน
- ทั้งสองเวอร์ชันพัง → **หยุด รายงานเจ้าของ** อย่าดันต่อ (แปลว่าเทมเพลตของเราไม่เข้ากับ CLI รุ่นใหม่ ซึ่งเป็นงานคนละก้อน)

บันทึกเวอร์ชันที่เลือกไว้ใช้ใน Step 4-5

- [ ] **Step 4: แก้เลขเวอร์ชันในโค้ด**

`internal/producer/hyperframes.go` บรรทัด 15-16 — แทน `0.6.70` ด้วยเวอร์ชันที่เลือก (ตัวอย่างเขียนเป็น `0.7.90`):

```go
// hyperframesVersion pins the CLI so renders are reproducible across machines.
// 0.6.70 → 0.7.90: 0.6.70 อ่าน os.totalmem() (= แรมของ "โฮสต์") ไปตั้งเพดาน heap
// ของ Chrome, งบหน่วยความจำ GPU และขนาดแคชเฟรม ทั้งที่คอนเทนเนอร์ถูกจำกัดด้วย
// cgroup — ค่ามันจึงเกินตัวเสมอ และการเรนเดอร์คลิปยาว 20 ส.ค. 2026 ค้างจน
// protocolTimeout ทั้ง 3 worker · ต้นน้ำแก้ใน v0.6.99 ("Respect cgroup memory
// limits in low-memory detection") ห้ามถอยต่ำกว่านั้น
const hyperframesVersion = "0.7.90"
```

- [ ] **Step 5: แก้ Dockerfile ให้ตรงกัน + ปิดฝาค่าที่ override ได้**

ใน `Dockerfile` แก้บรรทัด warm cache (ท้ายคอมเมนต์บล็อกให้แก้เลขในวงเล็บด้วย):

```dockerfile
RUN npx --yes hyperframes@0.7.90 --version
```

และเพิ่มบล็อกนี้ต่อจาก `ENV FONTS_DIR=/app/assets/fonts`:

```dockerfile
# เพดานหน่วยความจำของตัวเรนเดอร์ · CLI ปรับค่าพวกนี้เองตามแรมที่มันมองเห็น ซึ่งใน
# คอนเทนเนอร์เคยเป็นแรมของโฮสต์ (เหตุคลิปค้าง 20 ส.ค. 2026) · เวอร์ชันปัจจุบันอ่าน
# cgroup เป็นแล้ว ค่าพวกนี้จึงเป็นเข็มขัดนิรภัยเส้นที่สอง ไม่ใช่ตัวแก้หลัก
ENV PRODUCER_FRAME_DATA_URI_CACHE_BYTES_MB=256
ENV PRODUCER_FRAME_DATA_URI_CACHE_LIMIT=64
# 10 นาทีต่อหนึ่งคำสั่ง CDP: การจับภาพที่อืดแต่ยังเดินอยู่จะได้ไปต่อจนจบ แทนที่จะ
# ตายที่ 5 นาทีแบบเดิม · เพดานรวมยังคุมด้วย HyperframesRenderer.timeout (20 นาที)
ENV PRODUCER_PUPPETEER_PROTOCOL_TIMEOUT_MS=600000
```

- [ ] **Step 6: ตรวจว่าเลขสองที่ตรงกันจริง**

```bash
grep -n 'hyperframes@' Dockerfile; grep -n 'hyperframesVersion =' internal/producer/hyperframes.go
```

Expected: เลขเวอร์ชันตัวเดียวกันทั้งสองบรรทัด

- [ ] **Step 7: build image จริงให้แน่ใจว่า npm ดึงเวอร์ชันนี้ได้**

```bash
docker build -t adsvance-v2:hfbump . 2>&1 | tail -20
```

Expected: build ผ่าน (บรรทัด `RUN npx --yes hyperframes@<ver> --version` ไม่ error)
ถ้าเครื่องไม่มี docker ให้ข้ามและบอกเจ้าของว่าข้ามข้อนี้ — Railway จะ build เอง แต่ความเสี่ยงจะไปโผล่ตอนดีพลอย

- [ ] **Step 8: รันเทสต์ทั้งโปรเจกต์**

```bash
go test ./... -count=1 2>&1 | grep -v "^ok" | head -20
```

Expected: ไม่มีบรรทัด FAIL

- [ ] **Step 9: /simplify กับ diff แล้ว commit**

```bash
git add Dockerfile internal/producer/hyperframes.go
git commit -m "fix(render): อัป hyperframes ข้าม v0.6.99 (อ่านลิมิต cgroup) + ปิดฝาแคชเฟรมและ protocolTimeout"
```

---

### Task 5: ดีพลอย เฝ้าดู และกู้คลิป 8885bcc8

**Files:** ไม่แก้ไฟล์ — เป็นงานปฏิบัติการบน prod

**Interfaces:**
- Consumes: commit จาก Task 1-4
- Produces: คลิป `8885bcc8` กลับเข้าสายผลิต และบรรทัด `host render-*` บน prod log

- [ ] **Step 1: ดูว่ามีอะไรจะขึ้นไปกับ push นี้บ้าง**

```bash
git log --oneline origin/master..master
```

master ในเครื่องมีคอมมิตที่ยังไม่ push อยู่ก่อนแล้ว (WORKER_MODE + tick endpoint จาก 19 ส.ค. ซึ่งไม่ทำงานถ้าไม่ตั้งแฟล็ก) · **รายงานรายการนี้ให้เจ้าของก่อน push** — frontend เป็น service แยกแต่ auto-deploy จาก push เดียวกัน

- [ ] **Step 2: ยืนยันว่าไม่มีคลิปกำลังผลิตและยังไม่ชนรอบถัดไป**

```bash
date -u
railway logs -d --lines 50 | tail -20
```

Expected: ไม่มี "Producing"/"Processing" ค้างอยู่ และเหลือเวลา ≥ 20 นาทีก่อนรอบถัดไป (02/05/08/11/14 UTC) · ถ้าใกล้เกินไป ให้รอ

- [ ] **Step 3: push แล้วดูให้ build ผ่าน**

```bash
git push origin master
railway logs -b | tail -30
```

Expected: build สำเร็จ แล้วเห็น `Starting server on :8080` ใน deploy log

- [ ] **Step 4: ยืนยันว่าเวอร์ชันใหม่ทำงานอยู่จริง**

```bash
railway ssh "hyperframes --version" 2>/dev/null || railway logs -d --lines 30 | grep -i "host render\|hyperframes"
```

(`railway ssh` ต้องมี SSH key ลงทะเบียนไว้ — ถ้ายังไม่มี ให้ข้ามไปพึ่งบรรทัด `host render-*` ใน Step 6 แทน อย่าเพิ่งไปตั้ง key ใหม่กลางคัน)

- [ ] **Step 5: ปลดล็อกคลิปให้ระบบรีทรายเอง — ต้องขออนุมัติเจ้าของก่อนรัน**

คลิปติด `retry_count=2` ซึ่งเกินเพดาน `maxClipRetries=2` สคีดูลจึงไม่แตะ · endpoint `POST /api/v1/clips/{id}/rerender` ก็ใช้ไม่ได้เพราะรับเฉพาะสถานะ `needs_review`/`ready` · ทางที่ถูกคือรีเซ็ตตัวนับ แล้วปล่อยให้ tick "Retry Failed" (ทุก 5 นาที, cooldown 10 นาที) หยิบไปเอง:

```sql
UPDATE clips SET retry_count = 0, updated_at = now() - interval '11 minutes'
WHERE id = '8885bcc8-9c23-475d-aaf5-21af48959b77' AND status = 'failed';
```

รันผ่าน Neon MCP `run_sql` (project `snowy-grass-75448787`) · **นี่คือ UPDATE บน prod — ต้องได้คำอนุมัติจากเจ้าของก่อนเสมอ**

- [ ] **Step 6: เฝ้าดูรอบเรนเดอร์ของคลิปนี้**

```bash
railway logs -d --lines 200 | grep -E "Retrying clip 8885bcc8|Resuming clip 8885bcc8|host render-|Clip ready|failed:"
```

Expected: เห็น `Resuming clip 8885bcc8 at render stage` → `host render-start: mem <ใช้>/<เพดาน>MB disk_free ...` → บรรทัด `host render:` ระหว่างทางทุก 30 วิ → `host render-end:` → คลิปได้สถานะ ready/needs_review

**ตัวเลขที่ต้องอ่านให้ออก:** ถ้า `mem` ไต่ใกล้เพดานตอนใกล้พัง = ยืนยันสมมติฐานหน่วยความจำ · ถ้า `disk_free` ตกฮวบ = คนละเรื่อง ต้องกลับไปสืบใหม่ · ถ้าทั้งคู่นิ่งแต่ยังค้าง = สมมติฐานเราผิด ให้กลับไป Phase 1 ของ systematic-debugging พร้อมข้อมูลชุดใหม่

- [ ] **Step 7: ยืนยันผลจากฐานข้อมูล**

รันผ่าน Neon MCP `run_sql` (project `snowy-grass-75448787`):

```sql
SELECT c.status, c.retry_count, rc.stage, rc.passed, rc.duration_ms, rc.created_at
FROM clips c LEFT JOIN render_checks rc ON rc.clip_id = c.id
WHERE c.id = '8885bcc8-9c23-475d-aaf5-21af48959b77'
ORDER BY rc.created_at DESC LIMIT 5;
```

Expected: มีแถว `stage='render'` ที่ `passed=true` และ `duration_ms` ราวๆ 150,000-250,000 (คลิป 84 วินาที) · คลิปมี `video_9_16_url`

- [ ] **Step 8: เช็คว่าการเก็บกวาดทำงานและวัดผลรอบต่อไป**

หลังคลิปรอบถัดไปของสคีดูลผลิตเสร็จ:

```bash
railway logs -d --lines 300 | grep -E "host render-start|host render-end|cleanup clip dir"
```

Expected: `disk_free` ที่ `render-start` ของคลิปถัดไป **ไม่ต่ำลง** เมื่อเทียบกับคลิปก่อนหน้า (แปลว่าการลบโฟลเดอร์งานได้ผล) และไม่มีบรรทัด `cleanup clip dir failed`

---

## Self-Review

**ครอบคลุมข้อเสนอครบ 5 ข้อ:** อัป CLI ≥0.6.99 (Task 4) · env บรรเทา (Task 4 Step 5 — ย้ายไปไว้ใน Dockerfile แทนการตั้งผ่าน Railway UI เพราะทั้งสองทางต้องรีดีพลอยเท่ากัน แต่ทางนี้อยู่ในโค้ดและมีคนรีวิว) · log cgroup/disk (Task 2) · ลบ /tmp ต่อคลิป (Task 3) · กู้คลิป 8885bcc8 (Task 5)

**ข้อที่ยังเปิดอยู่โดยตั้งใจ:** ต้นเหตุระดับ "ทรัพยากรตัวไหนตัน" ยังไม่ได้ยืนยันในคอนเทนเนอร์จริง — Task 2 คือเครื่องมือที่จะยืนยันหรือหักล้างมันในเหตุการณ์ครั้งหน้า และ Task 5 Step 6 เขียนกฎการอ่านผลไว้แล้วทั้งกรณีที่สมมติฐานถูกและผิด
