# ย้าย adsvance-v2 จาก Railway → Cloudflare (ยกเว้น database)

## Context

ระบบผลิตคลิปอัตโนมัติ (Go ~30,000 บรรทัดใน `internal/`, ไม่นับ test + React frontend) รันอยู่บน Railway 2 service (`adsvance-v2`, `adsvance-frontend`) ต่อ Neon Postgres และเก็บไฟล์บน Cloudflare R2 อยู่แล้ว

แผนย้ายนี้เขียนไว้ครั้งแรกเมื่อ 2026-08-02 (`~/.claude/plans/harmonic-kindling-fairy.md`) แต่ค้างไว้เพราะราคาบน Cloudflare ยังไม่ verify จริง ตอนนี้ (2026-08-19) เจ้าของยืนยันว่า **ราคาไม่ใช่เกณฑ์ตัดสินใจอีกต่อไป** ต้องการย้ายเพราะเหตุผลอื่น (ระบบผลิตคลิปปัจจุบันเป็น detached goroutine — deploy ทับตอนกำลังผลิต = คลิปค้าง, อยากรวมของไว้ที่เดียวบน Cloudflare, Railway มีปัญหาสะสม)

ตรวจโค้ดปัจจุบันแล้ว (2026-08-19) — โครงสร้างที่แผนเดิมอ้างอิงยังตรง: router มี 55 routes (`internal/router/router.go`), checkpoint `stageContentReady`/`resumeHyperframesProduction`/`RetryAllFailed`/`AutoReviewPending` ยังอยู่ตำแหน่งเดิมใน `internal/orchestrator/orchestrator.go`, chromium pin ใน `Dockerfile` ยังอยู่ มีแค่ handler+repository โตขึ้นจาก ~4,500 → 5,357 บรรทัด (ไม่ใช่การเปลี่ยนแปลงเชิงโครงสร้าง)

**ขอบเขต: ย้ายทุกอย่าง ยกเว้น database — Neon Postgres คงเดิม 100% ไม่แตะ ไม่ย้าย**

## Revision 2026-08-19 (ระหว่างเขียน implementation plan)

ตอนเริ่มเขียนแผน implementation พบว่าโมเดล Workflow เดิมด้านล่าง ("1 instance ต่อ 1 คลิป, step content→render→publish") **ทำไม่ได้จริงโดยไม่แก้ `internal/orchestrator/orchestrator.go`** — ตรวจโค้ดแล้วพบว่าไม่มีฟังก์ชันไหนรับ clipID เดียวแล้วทำ content-step/render-step แยกกันจากภายนอกได้ ของจริงคือ `ProduceWeekly` สร้าง+ผลิตคลิปใหม่ N ใบในลูปเดียวจบ และการ resume ทำผ่าน DB checkpoint (`production_stage`) ที่ `RetryAllFailed`/`resumeHyperframesProduction` จัดการเองอยู่แล้ว ส่วนที่ระบบทำจริงคือ 12 "action" ระดับ tick ที่ `internal/scheduler/scheduler.go:241-272` แม็พจากตาราง `schedules` ไปยังฟังก์ชัน zero-arg แต่ละตัว (แต่ละ action มี gate กันชนกันเองอยู่แล้ว: `tracker.StartProduction` mutex + circuit breaker)

**แก้ไข (ยืนยันกับเจ้าของแล้ว):** Workflow เปลี่ยนจาก "1 instance/1 คลิป" เป็น **"1 instance/1 schedule tick"** — เหมือนที่ `scheduler.go` ทำอยู่ทุกวันนี้ทุกประการ เปลี่ยนแค่ตัวสั่ง (Go cron ภายใน → Cloudflare Cron Trigger + Workflow ภายนอก) Container เพิ่ม endpoint เดียว `POST /internal/tick/{action}` ที่เรียก dispatch table เดิมผ่าน `Scheduler.Dispatch` (method ใหม่ export จาก `handlerFor` ที่มีอยู่แล้ว) — **ไม่แตะ `orchestrator.go` เลย** ตรงกับที่ตกลงไว้แต่แรกว่า business logic ไม่พอร์ต

ผลคือ durability ที่ได้จาก Workflow แคบกว่าที่ระบุไว้เดิม: ได้การรอด "container ตายกลางทาง (crash/OOM) ระหว่าง tick" เพราะ Workflow retry เรียก tick ใหม่ได้ — **ไม่ได้** ได้ per-clip step-level resume ใหม่ที่ orchestrator.go ไม่มีให้ (resume ระดับคลิปยังเป็นกลไกเดิมที่ `retry_failed` action จัดการอยู่แล้ว ซึ่งดีกว่าของ Railway เดิมอยู่แล้วเพราะจะไม่ค้างเป็น orphaned goroutine เมื่อ container ตาย)

ส่วน "สถาปัตยกรรมเป้าหมาย" และ "ก้อนงาน" ด้านล่างถูกอัปเดตให้ตรงกับการแก้ไขนี้แล้ว — ไม่ใช่เวอร์ชันเดิมที่ approve ตอนแรก

## การตัดสินใจที่ยืนยันแล้ว

- **ราคาไม่ใช่เกณฑ์** — ไม่ต้อง block ด้วยตัวเลขต้นทุนที่ยังไม่ verify อีก
- **cutover ตัดจบทีเดียว** (big-bang) — ไม่รันคู่ขนานถาวร ทดสอบ 1 คลิปก่อนชี้ DNS เป็นด่านเดียวที่กันความเสี่ยง
- **render container หลับเมื่อไม่ผลิต** (sleepAfter สั้น) — ยอมรับ cold start แลกกับประหยัดค่า แม้ราคาไม่ใช่เกณฑ์หลักแล้ว แต่ยังเลือกแบบประหยัดเพราะไม่มีเหตุผลให้ตื่นค้าง
- **API layer เขียนใหม่เป็น Worker TypeScript ทั้งหมด** (ไม่ใช่คง Go เดิมในคอนเทนเนอร์) — ยอมรับความเสี่ยงพอร์ต 55 routes + repository 5,357 บรรทัด เพื่อได้สถาปัตยกรรม Cloudflare-native เต็มรูปแบบที่ edge
- business logic (agent/orchestrator/producer) **ไม่พอร์ต** — ยังเป็น Go เดิมทั้งหมด ย้ายแค่ที่โฮสต์

## สถาปัตยกรรมเป้าหมาย (แก้ตาม Revision ด้านบน)

```
Browser
  └─> Worker "adsvance-edge" (TS)
        ├── Static Assets  ← frontend/dist (แทน nginx)
        ├── /api/v1/*      ← พอร์ตจาก internal/handler + internal/repository (55 routes, 5,357 บรรทัด)
        ├── Hyperdrive     ← Neon (ไม่ย้าย, connection string เดิม)
        └── Cron Trigger ("0 * * * *", UTC) → อ่านตาราง schedules (cron_expression, Asia/Bangkok)
              ตัดสินเองว่า schedule row ไหนถึงรอบ → สร้าง Workflow instance ต่อ 1 tick ที่ถึงรอบ
              └─> Workflow "run-tick" (durable, 1 instance ต่อ 1 schedule action ที่ถึงรอบ)
                    └─> Container "adsvance-render" (Go binary เดิม + Dockerfile เดิม, chromium pin คงไว้)
                          POST /internal/tick/{action}  ← เรียก Scheduler.Dispatch(action) เดิม
                          (action คือ 1 ใน 12 ตัวที่ scheduler.go:241-272 แม็พอยู่แล้ว:
                           produce_and_publish, produce_evening, produce_tutorial, produce_basic,
                           produce_myth, publish_daily, publish_tiktok, retry_failed, auto_review,
                           analyze_and_improve, fetch_analytics, learn, tune_weights)
                          standard-4 · sleepAfter สั้น · หลับเมื่อไม่มี tick
R2 (cdn.thinkclip.xyz) — อยู่บน Cloudflare แล้ว ไม่ต้องแตะ ผูก binding ตรงจาก Worker/Container ได้เลย
Neon Postgres — คงเดิม 100% ต่อผ่าน Hyperdrive จาก Worker และต่อตรงจาก Container
```

**หมายเหตุ durability:** Workflow retry ที่ step "run-tick" คุ้มครองแค่กรณี container ตายกลางทาง (crash/OOM) — ไม่ได้เพิ่ม resume ระดับคลิปใหม่ เพราะกลไกนั้น (`production_stage` checkpoint + action `retry_failed`) มีอยู่แล้วในโค้ดเดิมและยังทำงานเหมือนเดิมทุกประการ ควร set `retries:{limit: 0 หรือ 1}` เท่านั้นที่ step นี้ — action อย่าง `produce_and_publish` ไม่ idempotent ต่อการเรียกซ้ำทันที (สร้างคลิปใหม่ทุกครั้งที่ถูกเรียกสำเร็จ) retry ที่มีความหมายจริงคือรอบ `retry_failed` ครั้งถัดไป (action แยกต่างหาก ถูก schedule ไว้อยู่แล้ว) ไม่ใช่ Workflow เรียก action เดิมซ้ำทันที

## ก้อนงาน

### 1. Container: Go เดิม → tick-dispatch endpoint (แผนละเอียด: `docs/superpowers/plans/2026-08-19-cloudflare-migration-worker-mode.md`)
- `internal/scheduler/scheduler.go` — export `Dispatch(ctx, action) error` ที่หุ้ม `handlerFor` (private) ที่มีอยู่แล้ว ไม่แตะ `orchestrator.go`
- `internal/handler/tick.go` — `POST /internal/tick/{action}` หุ้ม `Dispatch`, map error → HTTP status (unknown action → 404, `orchestrator.ErrProductionRunning` → 409, อื่นๆ → 500)
- `cmd/server/main.go` — เพิ่ม config `WORKER_MODE` (env bool, default false): เมื่อ true เปิดเฉพาะ `/health` + `/internal/tick/{action}` แทน public router เต็มรูปแบบ — `sched := scheduler.New(...)` ยังสร้างเหมือนเดิมทั้งสองโหมด แค่ `sched.Start(ctx)` (internal cron) ปิดผ่าน env `SCHEDULER_ENABLED=false` ที่มีอยู่แล้วตอน deploy จริง ไม่ใช่โค้ดใหม่
- `Dockerfile` — ใช้ของเดิมทั้งไฟล์ รวม chromium snapshot pin ห้ามแตะ
- ไม่ต้องการ `/internal/migrate` endpoint แยก — `database.RunMigrations` ถูกเรียกอัตโนมัติทุกครั้งที่ binary boot อยู่แล้ว (`cmd/server/main.go:54`)

### 2. Cloudflare project scaffold
- สร้าง `cf/` ใน repo: `wrangler.jsonc`, `src/index.ts`, `src/container.ts`, `src/workflow.ts`
- `wrangler.jsonc`: containers (`image = "../Dockerfile"`, `instance_type = "standard-4"`), workflows, hyperdrive, r2_buckets, assets (`frontend/dist`), triggers.crons, `limits.cpu_ms`
- `src/container.ts`: `class RenderContainer extends Container` — `defaultPort = 8080`, `sleepAfter = "2m"`, `envVars` ยกจาก Railway (`DATABASE_URL`, flags ทั้ง 8 ตัว, `TZ`, `FONTS_DIR`, `PUPPETEER_*`, `WORKER_MODE=true`, `SCHEDULER_ENABLED=false`)

### 3. Workflow `run-tick`
- `cf/src/workflow.ts` — `WorkflowEntrypoint` ต่อ 1 schedule action ที่ถึงรอบ:
  - `step.do("run", {timeout: "35 minutes", retries:{limit: 0}})` → เรียก container `POST /internal/tick/{action}` (ดูหมายเหตุ retry ด้านบน — เหตุผลที่ไม่ retry อัตโนมัติ)
  - เรียก `this.renewActivityTimeout()` ระหว่าง step ยาว กัน container หลับกลางทาง
- 1 Workflow instance ต่อ 1 (schedule row × รอบที่ถึงเวลา) — Cron Trigger เป็นคนสร้าง instance ไม่ใช่ Workflow เองวนลูป

### 4. Worker API + frontend
- พอร์ต 55 routes จาก `internal/router/router.go` → `cf/src/api/*.ts` โดยยึด response shape เดิม 1:1 (frontend ห้ามแก้)
  - **กติกา:** list endpoint ต้องคืน `[]` ไม่ใช่ `null`
- พอร์ต `internal/repository/*.go` → query ผ่าน Hyperdrive ด้วย `postgres.js`
- middleware API key (`internal/handler/middleware.go`) → Worker middleware, ค่าเดิม `API_KEY` เก็บเป็น secret
- routes ที่เรียก orchestrator ตรงจาก UI (เช่น "สั่งเรนเดอร์ใหม่" ต่อคลิปเดียว) — ยังเรียก container ตรงแบบ request-response เหมือนเดิม (คนละกรณีกับ tick แบบ scheduled ที่ผ่าน Workflow) ไม่ต้องสร้าง Workflow instance สำหรับ action ที่คนกดเอง
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
4. ยิง Workflow 1 instance ด้วย action `produce_and_publish` → `wrangler workflows instances describe` ต้องเห็น step "run" สถานะ success
5. คลิปที่ได้: มี MP4 บน R2, `clips.status` เป็น ready/needs_review, `render_checks` มีแถว, เปิดดูวิดีโอแล้วภาพ+เสียงครบ
6. ทดสอบ durability: kill container ระหว่าง tick กำลังรัน → Workflow เห็น step ล้มเหลว (ไม่ auto-retry ตาม `retries:{limit:0}`) — คลิปที่ทัน checkpoint `stageContentReady` ก่อนตายต้องกู้ได้ด้วยรอบ `retry_failed` tick ถัดไป ไม่ค้างเป็น orphan เหมือน Railway เดิม
