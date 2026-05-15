# adsvance-v2 DB Cost Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ลด Neon compute cost ของ project `adsvance-v2` (snowy-grass-75448787) จาก ~$2/เดือน เหลือ ~$0.50-1/เดือน พร้อมเปิด observability (`pg_stat_statements`) และพิสูจน์การลดได้ภายใน 7 วัน

**Architecture:** เปลี่ยน Neon settings 3 อย่าง (autoscaling min CU 1→0.25, history retention 24h→6h, เปิด `pg_stat_statements`) + เปิด connection pooler ที่ Neon endpoint แล้ว update `DATABASE_URL` ใน Railway. baseline ก่อน, รัน 7 วัน, เปรียบเทียบ `cpu_used_sec` กับ baseline ผ่าน Neon API

**Tech Stack:** Neon Postgres 17 (us-east-1), pgx/v5 connection pool, Go backend, React frontend (Railway), Neon MCP API

---

## Baseline สถานะปัจจุบัน (2026-05-15)

```
Project ID:                 snowy-grass-75448787
Branch ID:                  br-quiet-feather-and3bal7
Endpoint ID:                ep-restless-sun-an7oh37c
Region:                     aws-us-east-1
DB size:                    33 MB
autoscaling_limit_min_cu:   1
autoscaling_limit_max_cu:   1
suspend_timeout_seconds:    0  (Neon default = 5 min)
history_retention_seconds:  86400 (24h)
pooler_enabled:             false
pg_stat_statements:         not installed
cpu_used_sec (19 days):     32378
active_time_seconds (19d):  32388
pace:                       1705 sec/day @ 1 CU
estimated cost:             ~$2/month compute
```

## File Structure

- Modify: Neon project config via Console/CLI (`snowy-grass-75448787`)
- Modify: Railway env `DATABASE_URL` (adsvance-v2 backend service)
- Create: `docs/superpowers/plans/2026-05-15-db-cost-baseline.txt` (baseline + verification log)
- Modify: `.env.example` (document pooler URL convention)
- No changes: `internal/database/db.go` (pgxpool MaxConns=10 ทำงานกับ pooler ได้)

---

### Task 1: Snapshot baseline

**Files:** Create `docs/superpowers/plans/2026-05-15-db-cost-baseline.txt`

- [ ] **Step 1: Capture current state via Neon MCP**

Run:
```
mcp__neon__describe_project(projectId="snowy-grass-75448787")
mcp__neon__list_branch_computes(projectId="snowy-grass-75448787")
```

Record fields: `cpu_used_sec`, `active_time_seconds`, `compute_size`, `suspend_timeout_seconds`, `history_retention_seconds`, `pooler_enabled`

- [ ] **Step 2: Write baseline file**

Create `docs/superpowers/plans/2026-05-15-db-cost-baseline.txt`:
```
Date: 2026-05-15
Project: snowy-grass-75448787 (adsvance-v2)
Endpoint: ep-restless-sun-an7oh37c

[BASELINE]
cpu_used_sec: 32378
active_time_seconds: 32388
days_since_creation: 19
pace_sec_per_day: 1705
compute_size: 1
suspend_timeout_seconds: 0
history_retention_seconds: 86400
pooler_enabled: false
pg_stat_statements: not_installed
db_size_mb: 33
estimated_cost_per_month_usd: 2.00
```

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/plans/2026-05-15-db-cost-baseline.txt
git commit -m "chore(db): record baseline metrics for adsvance-v2 cost optimization"
```

---

### Task 2: Enable pg_stat_statements

**Files:** SQL on `snowy-grass-75448787` main branch

- [ ] **Step 1: Verify extension missing**

Run:
```
mcp__neon__run_sql(projectId="snowy-grass-75448787",
  sql="SELECT extname FROM pg_extension WHERE extname='pg_stat_statements';")
```
Expected: empty result (0 rows)

- [ ] **Step 2: Install extension**

Run:
```
mcp__neon__run_sql(projectId="snowy-grass-75448787",
  sql="CREATE EXTENSION IF NOT EXISTS pg_stat_statements;")
```
Expected: success message

- [ ] **Step 3: Verify installed**

Run:
```
mcp__neon__run_sql(projectId="snowy-grass-75448787",
  sql="SELECT extname, extversion FROM pg_extension WHERE extname='pg_stat_statements';")
```
Expected: 1 row with extname='pg_stat_statements'

- [ ] **Step 4: Verify it captures queries**

Wait 1 minute then run:
```
mcp__neon__run_sql(projectId="snowy-grass-75448787",
  sql="SELECT calls, total_exec_time, query FROM pg_stat_statements ORDER BY calls DESC LIMIT 5;")
```
Expected: ≥ 1 row returned

- [ ] **Step 5: Document and commit**

Append to baseline file:
```
[CHANGE 2026-05-15]
pg_stat_statements: installed
```
```bash
git add docs/superpowers/plans/2026-05-15-db-cost-baseline.txt
git commit -m "chore(db): enable pg_stat_statements on adsvance-v2"
```

---

### Task 3: Reduce autoscaling min CU 1 → 0.25

**Files:** Endpoint `ep-restless-sun-an7oh37c` config (Neon Console / CLI)

- [ ] **Step 1: Verify current state**

Run:
```
mcp__neon__list_branch_computes(projectId="snowy-grass-75448787")
```
Expected: `autoscaling_limit_min_cu: 1`, `autoscaling_limit_max_cu: 1`

- [ ] **Step 2: Apply change (Neon Console)**

Manual (Neon MCP ไม่มี endpoint update tool):
1. เปิด https://console.neon.tech/app/projects/snowy-grass-75448787/branches/br-quiet-feather-and3bal7
2. เปิด endpoint `ep-restless-sun-an7oh37c`
3. แก้ Compute size: min `0.25` CU, max `1` CU
4. กด Save

หรือผ่าน Neon CLI:
```
neonctl branches set-default-compute-units br-quiet-feather-and3bal7 \
  --project-id snowy-grass-75448787 \
  --min 0.25 --max 1
```

- [ ] **Step 3: Verify change applied**

Run:
```
mcp__neon__list_branch_computes(projectId="snowy-grass-75448787")
```
Expected: `autoscaling_limit_min_cu: 0.25`, `autoscaling_limit_max_cu: 1`

- [ ] **Step 4: Smoke test from app (cold start)**

รอให้ endpoint suspend (~5 นาทีหลังไม่ใช้) แล้วยิง:
```bash
curl -i https://<railway-frontend-url>/api/health
```
Expected: HTTP 200 ภายใน 2 วินาที (รวม cold start)

ยิงครั้งที่ 2 ทันที:
```bash
curl -i https://<railway-frontend-url>/api/health
```
Expected: HTTP 200 ภายใน 200ms (warm)

- [ ] **Step 5: Document and commit**

Append to baseline file:
```
[CHANGE 2026-05-15]
autoscaling_limit_min_cu: 0.25
autoscaling_limit_max_cu: 1
cold_start_ms: <measured>
warm_response_ms: <measured>
```
```bash
git add docs/superpowers/plans/2026-05-15-db-cost-baseline.txt
git commit -m "chore(db): reduce min CU 1->0.25 for adsvance-v2 endpoint"
```

---

### Task 4: Reduce history retention 24h → 6h

**Files:** Project `snowy-grass-75448787` settings (Neon Console)

- [ ] **Step 1: Verify current value**

Run:
```
mcp__neon__describe_project(projectId="snowy-grass-75448787")
```
Expected: `history_retention_seconds: 86400`

- [ ] **Step 2: Apply change (Neon Console)**

Manual:
1. เปิด https://console.neon.tech/app/projects/snowy-grass-75448787/settings/storage
2. แก้ "History retention" จาก 24 hours เป็น 6 hours
3. กด Save

- [ ] **Step 3: Verify**

Run:
```
mcp__neon__describe_project(projectId="snowy-grass-75448787")
```
Expected: `history_retention_seconds: 21600`

- [ ] **Step 4: Document and commit**

Append to baseline file:
```
[CHANGE 2026-05-15]
history_retention_seconds: 21600 (6h)
```
```bash
git add docs/superpowers/plans/2026-05-15-db-cost-baseline.txt
git commit -m "chore(db): reduce PITR retention 24h->6h for adsvance-v2"
```

---

### Task 5: Enable connection pooler

**Files:**
- Endpoint `ep-restless-sun-an7oh37c` config (Neon Console)
- Railway env: `DATABASE_URL` (backend service `adsvance-v2`)
- Modify: `.env.example`

- [ ] **Step 1: Verify pooler currently disabled**

Run:
```
mcp__neon__list_branch_computes(projectId="snowy-grass-75448787")
```
Expected: `pooler_enabled: false`

Note pooled host จาก response: `ep-restless-sun-an7oh37c-pooler.c-6.us-east-1.aws.neon.tech`

- [ ] **Step 2: Enable pooler in Neon Console**

Manual:
1. เปิด endpoint `ep-restless-sun-an7oh37c` ใน Console
2. Toggle "Connection pooling" → ON
3. Mode: `transaction`
4. กด Save

- [ ] **Step 3: Verify pooler enabled**

Run:
```
mcp__neon__list_branch_computes(projectId="snowy-grass-75448787")
```
Expected: `pooler_enabled: true`

- [ ] **Step 4: Get current Railway DATABASE_URL**

```
mcp__railway__list-variables(workspacePath="/Users/jaochai/Code/video-fb")
```
Note ค่าเก่าของ `DATABASE_URL`

- [ ] **Step 5: Update Railway DATABASE_URL**

แก้ host จาก `ep-restless-sun-an7oh37c.c-6.us-east-1.aws.neon.tech` → `ep-restless-sun-an7oh37c-pooler.c-6.us-east-1.aws.neon.tech` (ส่วนอื่นเหมือนเดิม)

```
mcp__railway__set-variables(
  workspacePath="/Users/jaochai/Code/video-fb",
  variables=["DATABASE_URL=postgresql://USER:PASS@ep-restless-sun-an7oh37c-pooler.c-6.us-east-1.aws.neon.tech/DBNAME?sslmode=require"]
)
```
Expected: trigger redeploy

- [ ] **Step 6: Verify backend reconnects**

```
mcp__railway__get-logs(workspacePath="/Users/jaochai/Code/video-fb", logType="deploy", lines=100)
```
Expected ใน logs:
- ไม่มี `connect: connection refused`
- ไม่มี `pgxpool: failed to connect`
- เห็น `Scheduler started with 4 jobs`
- เห็น handler routes registered

- [ ] **Step 7: Smoke test endpoints**

```bash
curl -i https://<railway-frontend-url>/api/health
curl -i https://<railway-frontend-url>/api/clips
```
Expected: ทั้ง 2 endpoint คืน HTTP 200

- [ ] **Step 8: Update .env.example**

Modify `.env.example`:
```
# Use -pooler suffix in host for transaction-mode pooling (recommended on Neon)
DATABASE_URL=postgresql://user:pass@ep-xxx-pooler.region.aws.neon.tech/dbname?sslmode=require
```

- [ ] **Step 9: Commit**

Append to baseline file:
```
[CHANGE 2026-05-15]
pooler_enabled: true
railway_DATABASE_URL_host: ep-restless-sun-an7oh37c-pooler.c-6.us-east-1.aws.neon.tech
```
```bash
git add .env.example docs/superpowers/plans/2026-05-15-db-cost-baseline.txt
git commit -m "chore(db): enable Neon transaction pooler for adsvance-v2"
```

---

### Task 6: Verify cost reduction (run after 7 days, ≥ 2026-05-22)

**Files:** Append to `docs/superpowers/plans/2026-05-15-db-cost-baseline.txt`

- [ ] **Step 1: Re-capture metrics**

Run:
```
mcp__neon__describe_project(projectId="snowy-grass-75448787")
```
Note: `cpu_used_sec`, `active_time_seconds`

- [ ] **Step 2: Compute deltas**

```
days_elapsed = 7  (ปรับตามจริง)
new_cpu_used_delta = current_cpu_used_sec - 32378
new_active_time_delta = current_active_time_seconds - 32388
new_pace_cpu = new_cpu_used_delta / days_elapsed
new_pace_active = new_active_time_delta / days_elapsed
% reduction = 1 - (new_pace_cpu / 1705)
projected_monthly_cost_usd = (new_pace_cpu / 3600) * 0.25 * 30 * 0.16
```

- [ ] **Step 3: Pass criteria check**

PASS ถ้าครบทุกข้อ:
- [ ] % reduction in `cpu_used_sec` per day ≥ 30%
- [ ] Railway error rate (`get-logs logType=deploy`) ไม่เพิ่มเทียบสัปดาห์ก่อน
- [ ] `/api/*` p95 latency ใน Railway metrics เพิ่มไม่เกิน +200ms
- [ ] Cron jobs ทำงานครบ: query `SELECT max(created_at) FROM agent_runs;` (หรือตารางที่บันทึก produce/analytics) ต้องมี row จาก 7 วันที่ผ่านมา
- [ ] Slow queries ตรวจ: `SELECT calls, mean_exec_time, query FROM pg_stat_statements WHERE mean_exec_time > 500 ORDER BY total_exec_time DESC LIMIT 10;` ไม่มี new outlier

- [ ] **Step 4: Document result**

Append to baseline file:
```
[VERIFICATION 2026-05-22]
days_elapsed: 7
cpu_used_sec_delta: <X>
active_time_delta: <Y>
new_pace_sec_per_day: <Z>
% reduction: <W>%
projected_monthly_cost_usd: <C>
status: PASS/FAIL
notes: <observations>
```

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/plans/2026-05-15-db-cost-baseline.txt
git commit -m "chore(db): verify 7-day cost reduction for adsvance-v2"
```

---

## Rollback (ถ้า Task 6 FAIL หรือเจอปัญหาก่อน)

| Task | Rollback |
|---|---|
| Task 2 | `mcp__neon__run_sql(... sql="DROP EXTENSION pg_stat_statements;")` |
| Task 3 | Console: เปลี่ยน CU min/max กลับเป็น 1/1 |
| Task 4 | Console: เปลี่ยน history retention กลับเป็น 24h |
| Task 5 | Railway: เปลี่ยน `DATABASE_URL` host เอา `-pooler` ออก + redeploy; Console: ปิด pooler toggle |

ทุก rollback ใช้เวลา < 2 นาที, ไม่มี data loss
