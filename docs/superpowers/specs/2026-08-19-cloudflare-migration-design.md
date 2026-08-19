# ย้าย adsvance-v2 จาก Railway → Cloudflare (ยกเว้น database)

## Context

ระบบผลิตคลิปอัตโนมัติ (Go ~30,000 บรรทัดใน `internal/`, ไม่นับ test + React frontend) รันอยู่บน Railway 2 service (`adsvance-v2`, `adsvance-frontend`) ต่อ Neon Postgres และเก็บไฟล์บน Cloudflare R2 อยู่แล้ว

แผนย้ายนี้เขียนไว้ครั้งแรกเมื่อ 2026-08-02 (`~/.claude/plans/harmonic-kindling-fairy.md`) แต่ค้างไว้เพราะราคาบน Cloudflare ยังไม่ verify จริง ตอนนี้ (2026-08-19) เจ้าของยืนยันว่า **ราคาไม่ใช่เกณฑ์ตัดสินใจอีกต่อไป** ต้องการย้ายเพราะเหตุผลอื่น (ระบบผลิตคลิปปัจจุบันเป็น detached goroutine — deploy ทับตอนกำลังผลิต = คลิปค้าง, อยากรวมของไว้ที่เดียวบน Cloudflare, Railway มีปัญหาสะสม)

ตรวจโค้ดปัจจุบันแล้ว (2026-08-19) — โครงสร้างที่แผนเดิมอ้างอิงยังตรง: router มี 55 routes (`internal/router/router.go`), checkpoint `stageContentReady`/`resumeHyperframesProduction`/`RetryAllFailed`/`AutoReviewPending` ยังอยู่ตำแหน่งเดิมใน `internal/orchestrator/orchestrator.go`, chromium pin ใน `Dockerfile` ยังอยู่ มีแค่ handler+repository โตขึ้นจาก ~4,500 → 5,357 บรรทัด (ไม่ใช่การเปลี่ยนแปลงเชิงโครงสร้าง)

**ขอบเขต: ย้ายทุกอย่าง ยกเว้น database — Neon Postgres คงเดิม 100% ไม่แตะ ไม่ย้าย**

## การตัดสินใจที่ยืนยันแล้ว

- **ราคาไม่ใช่เกณฑ์** — ไม่ต้อง block ด้วยตัวเลขต้นทุนที่ยังไม่ verify อีก
- **cutover ตัดจบทีเดียว** (big-bang) — ไม่รันคู่ขนานถาวร ทดสอบ 1 คลิปก่อนชี้ DNS เป็นด่านเดียวที่กันความเสี่ยง
- **render container หลับเมื่อไม่ผลิต** (sleepAfter สั้น) — ยอมรับ cold start แลกกับประหยัดค่า แม้ราคาไม่ใช่เกณฑ์หลักแล้ว แต่ยังเลือกแบบประหยัดเพราะไม่มีเหตุผลให้ตื่นค้าง
- **API layer เขียนใหม่เป็น Worker TypeScript ทั้งหมด** (ไม่ใช่คง Go เดิมในคอนเทนเนอร์) — ยอมรับความเสี่ยงพอร์ต 55 routes + repository 5,357 บรรทัด เพื่อได้สถาปัตยกรรม Cloudflare-native เต็มรูปแบบที่ edge
- business logic (agent/orchestrator/producer) **ไม่พอร์ต** — ยังเป็น Go เดิมทั้งหมด ย้ายแค่ที่โฮสต์

## สถาปัตยกรรมเป้าหมาย

```
Browser
  └─> Worker "adsvance-edge" (TS)
        ├── Static Assets  ← frontend/dist (แทน nginx)
        ├── /api/v1/*      ← พอร์ตจาก internal/handler + internal/repository (55 routes, 5,357 บรรทัด)
        ├── Hyperdrive     ← Neon (ไม่ย้าย, connection string เดิม)
        ├── Cron Trigger   ← แทน internal/scheduler (อ่านตาราง schedules เอง, UTC→Asia/Bangkok)
        └── Workflow "produce-clip" (durable, retry ราย step)
              └─> Container "adsvance-render" (Go binary เดิม + Dockerfile เดิม, chromium pin คงไว้)
                    step endpoints: /internal/step/{content,render,publish,retry,autoreview}
                    standard-4 · sleepAfter สั้น · หลับเมื่อไม่ผลิต
R2 (cdn.thinkclip.xyz) — อยู่บน Cloudflare แล้ว ไม่ต้องแตะ ผูก binding ตรงจาก Worker/Container ได้เลย
Neon Postgres — คงเดิม 100% ต่อผ่าน Hyperdrive จาก Worker และต่อตรงจาก Container
```

## ก้อนงาน

### 1. Container: Go เดิม → step endpoints
- `cmd/server/main.go` — เพิ่มโหมด `-mode=worker`: ไม่ start `scheduler.New()`, ไม่ serve public API, เปิดเฉพาะ step endpoints บน port 8080
- เพิ่ม `internal/handler/steps.go` — HTTP endpoint หุ้มขั้นตอนที่มีอยู่แล้วใน `internal/orchestrator/orchestrator.go`:
  - `POST /internal/step/content` → `produceClipWithID` ส่วนต้นถึงเช็คพอยต์ `stageContentReady`
  - `POST /internal/step/render` → `renderAndFinalize`
  - `POST /internal/step/publish` → publisher เดิม
  - `POST /internal/step/retry`, `/internal/step/autoreview` → `RetryAllFailed`, `AutoReviewPending`
  - `POST /internal/migrate` → `database.RunMigrations`
  - ทุก endpoint รับ/คืน JSON ที่ serialize ได้ และ **idempotent ต่อ clipID** (Workflow retry ได้)
- `Dockerfile` — ใช้ของเดิมทั้งไฟล์ รวม chromium snapshot pin ห้ามแตะ
- ตัด `internal/router/router.go` ออกจาก path นี้ (router เดิมยังใช้สำหรับ local dev)

### 2. Cloudflare project scaffold
- สร้าง `cf/` ใน repo: `wrangler.jsonc`, `src/index.ts`, `src/container.ts`, `src/workflow.ts`
- `wrangler.jsonc`: containers (`image = "../Dockerfile"`, `instance_type = "standard-4"`), workflows, hyperdrive, r2_buckets, assets (`frontend/dist`), triggers.crons, `limits.cpu_ms`
- `src/container.ts`: `class RenderContainer extends Container` — `defaultPort = 8080`, `sleepAfter = "2m"`, `envVars` ยกจาก Railway (`DATABASE_URL`, flags ทั้ง 8 ตัว, `TZ`, `FONTS_DIR`, `PUPPETEER_*`)

### 3. Workflow `produce-clip`
- `cf/src/workflow.ts` — `WorkflowEntrypoint` ต่อ 1 คลิป:
  - `step.do("content", {timeout: "15 minutes", retries:{limit:2}})` → เรียก container `/internal/step/content`
  - `step.do("render", {timeout: "30 minutes", retries:{limit:1}})` → `/internal/step/render` (ต้องยาวกว่า `HyperframesRenderer.timeout` 20 นาทีที่ `internal/producer/hyperframes.go:77`)
  - `step.do("publish", ...)` → `/internal/step/publish`
  - เรียก `this.renewActivityTimeout()` ระหว่าง step ยาว กัน container หลับกลางทาง
- รอบ produce หลายคลิป = สร้าง instance ละคลิป (แทน loop ใน `ProduceWeekly`) — ได้ isolation + retry รายคลิป

### 4. Worker API + frontend
- พอร์ต 55 routes จาก `internal/router/router.go` → `cf/src/api/*.ts` โดยยึด response shape เดิม 1:1 (frontend ห้ามแก้)
  - **กติกา:** list endpoint ต้องคืน `[]` ไม่ใช่ `null`
- พอร์ต `internal/repository/*.go` → query ผ่าน Hyperdrive ด้วย `postgres.js`
- middleware API key (`internal/handler/middleware.go`) → Worker middleware, ค่าเดิม `API_KEY` เก็บเป็น secret
- routes ที่เรียก orchestrator (`/orchestrator/produce*`, `/publish*`, `/retry`, `/analytics/fetch`) → เปลี่ยนเป็นสร้าง Workflow instance แทนการเรียก Go ตรง
- `frontend/` — เปลี่ยนแค่ base URL ของ API, build เดิม, ทิ้ง `frontend/Dockerfile` + `nginx.conf`

### 5. Cron
- `internal/scheduler/scheduler.go` อ่านตาราง `schedules` แล้วลง cron ตอน boot — Cloudflare cron เป็น static ใน config และเป็น **UTC**
- ทำ cron เดียว `"0 * * * *"` → handler อ่านตาราง `schedules` (คอลัมน์ `cron_expression`, timezone Asia/Bangkok) แล้วตัดสินเองว่าถึงรอบไหน — รูปแบบเดียวกับ publish queue ที่ระบบใช้อยู่แล้ว
- ต้องคง advisory lock ของรอบส่งไว้ (มีอยู่แล้วใน DB) กันยิงซ้ำ

### 6. Cutover (big-bang)
1. `wrangler deploy` ขึ้น Cloudflare โดย Railway ยังรันอยู่ (ยังไม่ปิด)
2. ปิด schedule ทั้งหมดบน Railway ผ่าน **API PATCH** (ห้าม UPDATE DB ตรง)
3. ยิงผลิต 1 คลิปบน Cloudflare → ตรวจครบวงจร
4. ชี้ DNS/โดเมนมาที่ Worker
5. ลบ Railway service เมื่อผ่าน 1 รอบผลิตเต็มวัน

## ความเสี่ยงที่ต้องรู้ก่อนเริ่ม

- **พอร์ต 55 routes + repository 5,357 บรรทัด = แหล่ง regression หลัก** งานส่วนนี้ใหญ่กว่าส่วนอื่นรวมกัน — ยอมรับความเสี่ยงนี้แล้วเพื่อได้สถาปัตยกรรม Worker เต็มรูปแบบที่ edge (ทางเลือกที่เสี่ยงน้อยกว่าคือคง Go เดิมรันในคอนเทนเนอร์เดียวไม่พอร์ต แต่ถูกปฏิเสธแล้ว)
- **ตัดจบทีเดียว** = ถ้า cutover พัง คลิปหยุดผลิตจนกว่าจะแก้ ขั้นตอน cutover ข้อ 3 (ทดสอบ 1 คลิปก่อนชี้ DNS) เป็นด่านเดียวที่กันเรื่องนี้ ห้ามข้าม
- **cold start** ของ container ก่อน render (เพราะเลือกให้หลับ) — ยังไม่รู้ตัวเลขจริง ต้องวัดในขั้นตอน cutover ข้อ 3
- ทุกอย่างที่ Neon/R2 ถืออยู่ **ไม่ย้าย** — DB และไฟล์วิดีโอไม่ถูกแตะตลอดแผนนี้

## Verification

1. `go build ./...` + `go test ./...` ผ่าน หลังเพิ่มโหมด worker
2. `npx wrangler dev` — เรียกทุก route ที่พอร์ตแล้ว เทียบ JSON กับ prod Railway ตัวต่อตัว (55 routes)
3. `npx wrangler deploy` สำเร็จ, `wrangler containers list` เห็น image
4. ยิง Workflow 1 instance → `wrangler workflows instances describe` ต้องเห็นทั้ง 3 step สถานะ success
5. คลิปที่ได้: มี MP4 บน R2, `clips.status` เป็น ready/needs_review, `render_checks` มีแถว, เปิดดูวิดีโอแล้วภาพ+เสียงครบ
6. ทดสอบ durability: kill container ระหว่าง step render → Workflow ต้อง retry แล้วจบงานได้เอง (นี่คือสิ่งที่ระบบเดิมทำไม่ได้)
