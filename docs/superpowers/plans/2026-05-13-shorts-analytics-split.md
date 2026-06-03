# Shorts Analytics Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** แยกข้อมูล analytics ของคลิปยาว (regular) กับ shorts บนหน้า Analytics ให้ครบไม่ทับกัน

**Problem:** ตาราง `clip_analytics` เก็บคีย์แค่ `(clip_id, platform)` — publisher insert ทั้ง regular และ shorts post ลง row ที่ใช้คีย์เดียวกัน + repository ใช้ `DISTINCT ON (clip_id, platform)` → row ตัวที่ insert ทีหลังทับตัวก่อน ทำให้บนหน้า Analytics เห็นข้อมูลแค่ post type เดียวต่อ (clip, platform)

**Architecture:** เพิ่ม column `post_type` ใน `clip_analytics` (`regular` / `shorts`) + ขยาย DISTINCT ON เป็น `(clip_id, platform, post_type)` + repo/handler/frontend ส่งต่อ field ใหม่ + frontend แสดงการ์ดแยก regular และ shorts

**Tech Stack:** PostgreSQL (Neon), Go (pgx, chi), React + TypeScript + tanstack/react-query

---

## File Structure

**Create:**
- `migrations/015_clip_analytics_post_type.sql` — เพิ่ม column `post_type`, backfill, index

**Modify:**
- `internal/models/clip.go:53-64` — เพิ่ม `PostType` ใน `ClipAnalytics`
- `internal/repository/analytics.go:12-18` — DISTINCT ON tuple ใหม่
- `internal/repository/analytics.go:28-51` — `ListByClip` คืน `post_type`
- `internal/repository/analytics.go:112-121` — `Create` insert `post_type`
- `internal/publisher/publisher.go:248-265` — ส่ง `PostType: post.label` ตอน Create
- `frontend/src/pages/Analytics.tsx:37-41` — เพิ่ม `post_type` ใน interface
- `frontend/src/pages/Analytics.tsx:106-111` — เปลี่ยน key ของ platformMap เป็น `${platform}-${post_type}`
- `frontend/src/pages/Analytics.tsx:213-235` — render การ์ดแยก regular/shorts

**Test:**
- `internal/repository/analytics_test.go` — Create / Summary / TopClips / ListByClip กับ regular+shorts mix

---

## Task 1: Migration — เพิ่ม column post_type

**Files:**
- Create: `migrations/015_clip_analytics_post_type.sql`

- [ ] **Step 1: เขียน migration**

```sql
-- 015_clip_analytics_post_type.sql
-- แยก analytics ของคลิปยาว (regular) กับ shorts
-- ก่อนหน้านี้ row regular + shorts ใช้คีย์ (clip_id, platform) เหมือนกัน → ทับกัน

ALTER TABLE clip_analytics
    ADD COLUMN IF NOT EXISTS post_type TEXT NOT NULL DEFAULT 'regular';

-- ลบของเก่าทิ้งเพื่อให้รอบ fetch ถัดไป backfill ใหม่ด้วย post_type ที่ถูก
-- (ของเก่ารวม regular+shorts ทับกันอยู่แล้ว — เก็บไว้ก็ตีความไม่ได้)
DELETE FROM clip_analytics WHERE post_type = 'regular';

CREATE INDEX IF NOT EXISTS idx_clip_analytics_lookup
    ON clip_analytics (clip_id, platform, post_type, fetched_at DESC);
```

- [ ] **Step 2: รัน migration**

Run: `psql "$DATABASE_URL" -f migrations/015_clip_analytics_post_type.sql`
Expected: `ALTER TABLE` / `DELETE n` / `CREATE INDEX`

- [ ] **Step 3: ยืนยัน schema**

Run: `psql "$DATABASE_URL" -c "\d clip_analytics"`
Expected: เห็น column `post_type` `text NOT NULL DEFAULT 'regular'::text`

- [ ] **Step 4: Commit**

```bash
git add migrations/015_clip_analytics_post_type.sql
git commit -m "feat(analytics): add post_type column to clip_analytics"
```

---

## Task 2: Go model — เพิ่ม PostType

**Files:**
- Modify: `internal/models/clip.go:53-64`

- [ ] **Step 1: แก้ struct ClipAnalytics**

`internal/models/clip.go:53-64` เปลี่ยนเป็น:

```go
type ClipAnalytics struct {
	ID               string    `json:"id"`
	ClipID           string    `json:"clip_id"`
	Platform         string    `json:"platform"`
	PostType         string    `json:"post_type"`
	Views            int       `json:"views"`
	Likes            int       `json:"likes"`
	Comments         int       `json:"comments"`
	Shares           int       `json:"shares"`
	WatchTimeSeconds float64   `json:"watch_time_seconds"`
	RetentionRate    float64   `json:"retention_rate"`
	FetchedAt        time.Time `json:"fetched_at"`
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: FAIL — repo และ publisher ยังไม่ส่ง `PostType`

(ปล่อยให้ fail ในขั้นนี้ — task ถัดไปจะแก้)

---

## Task 3: Repository — รองรับ post_type

**Files:**
- Modify: `internal/repository/analytics.go:12-18` (CTE)
- Modify: `internal/repository/analytics.go:28-51` (ListByClip)
- Modify: `internal/repository/analytics.go:53-67` (Summary)
- Modify: `internal/repository/analytics.go:69-101` (TopClips)
- Modify: `internal/repository/analytics.go:112-121` (Create)

- [ ] **Step 1: เปลี่ยน CTE**

`internal/repository/analytics.go:12-18` เปลี่ยนเป็น:

```go
const latestAnalyticsCTE = `WITH latest AS (
	SELECT DISTINCT ON (clip_id, platform, post_type)
		clip_id, platform, post_type, views, likes, comments, shares,
		watch_time_seconds, retention_rate
	FROM clip_analytics
	ORDER BY clip_id, platform, post_type, fetched_at DESC
)`
```

- [ ] **Step 2: แก้ ListByClip**

`internal/repository/analytics.go:28-51` เปลี่ยนเป็น:

```go
func (r *AnalyticsRepo) ListByClip(ctx context.Context, clipID string) ([]models.ClipAnalytics, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, clip_id, platform, post_type, views, likes, comments, shares,
		        watch_time_seconds, retention_rate, fetched_at
		 FROM clip_analytics WHERE clip_id = $1 ORDER BY fetched_at DESC`, clipID)
	if err != nil {
		return nil, fmt.Errorf("query analytics: %w", err)
	}
	defer rows.Close()

	var results []models.ClipAnalytics
	for rows.Next() {
		var a models.ClipAnalytics
		if err := rows.Scan(&a.ID, &a.ClipID, &a.Platform, &a.PostType, &a.Views, &a.Likes,
			&a.Comments, &a.Shares, &a.WatchTimeSeconds, &a.RetentionRate, &a.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan analytics: %w", err)
		}
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate analytics: %w", err)
	}
	return results, nil
}
```

- [ ] **Step 3: แก้ Summary**

`internal/repository/analytics.go:53-67` แทนที่ body ของ `Summary` ด้วย:

```go
func (r *AnalyticsRepo) Summary(ctx context.Context) (models.AnalyticsSummary, error) {
	var s models.AnalyticsSummary
	err := r.pool.QueryRow(ctx, latestAnalyticsCTE+`
		SELECT COALESCE(SUM(l.views),0), COALESCE(SUM(l.likes),0),
			   COALESCE(SUM(l.comments),0), COALESCE(SUM(l.shares),0),
			   COALESCE(AVG(NULLIF(l.retention_rate, 0)),0),
			   COALESCE(SUM(l.watch_time_seconds),0),
			   (SELECT COUNT(*) FROM clips WHERE status = 'published')
		FROM latest l`).Scan(
		&s.TotalViews, &s.TotalLikes, &s.TotalComments, &s.TotalShares,
		&s.AvgRetention, &s.TotalWatchTime, &s.ClipCount)
	if err != nil {
		return s, fmt.Errorf("query analytics summary: %w", err)
	}
	return s, nil
}
```

(`AVG(NULLIF(retention_rate, 0))` กัน shorts ที่ยังไม่มี retention มา zero-out ค่าเฉลี่ย)

- [ ] **Step 4: TopClips ไม่ต้องแก้ logic — แค่ตรวจว่ายัง SUM ถูก**

`internal/repository/analytics.go:69-101` ใช้ `SUM(l.views)` อยู่แล้ว → เมื่อ CTE มี row regular + shorts แยก จะถูกรวมกันเป็นยอดต่อ clip โดยอัตโนมัติ → **ไม่แก้**

- [ ] **Step 5: แก้ Create**

`internal/repository/analytics.go:112-121` เปลี่ยนเป็น:

```go
func (r *AnalyticsRepo) Create(ctx context.Context, a models.ClipAnalytics) error {
	if a.PostType == "" {
		a.PostType = "regular"
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_analytics (clip_id, platform, post_type, views, likes, comments, shares, watch_time_seconds, retention_rate)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		a.ClipID, a.Platform, a.PostType, a.Views, a.Likes, a.Comments, a.Shares, a.WatchTimeSeconds, a.RetentionRate)
	if err != nil {
		return fmt.Errorf("create analytics: %w", err)
	}
	return nil
}
```

- [ ] **Step 6: Build check**

Run: `go build ./...`
Expected: ยัง FAIL ที่ publisher (ยังไม่ส่ง PostType) — task ถัดไปแก้

---

## Task 4: Publisher — ส่ง PostType ตอน Create

**Files:**
- Modify: `internal/publisher/publisher.go:248-265`

- [ ] **Step 1: แก้ Create call ใน FetchAnalytics**

`internal/publisher/publisher.go:252-260` ภายใน inner loop เปลี่ยน Create call เป็น:

```go
if err := p.analytics.Create(ctx, models.ClipAnalytics{
	ClipID:           cp.ClipID,
	Platform:         platform,
	PostType:         post.label,
	Views:            metrics.Views,
	Likes:            metrics.Likes,
	Comments:         metrics.Comments,
	Shares:           metrics.Shares,
	WatchTimeSeconds: watchTime,
	RetentionRate:    retention,
}); err != nil {
```

(post.label ถูก set เป็น `"regular"` หรือ `"shorts"` ใน publisher.go:218-225 อยู่แล้ว → ใช้ตรงๆ ได้)

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit Task 2-4**

```bash
git add internal/models/clip.go internal/repository/analytics.go internal/publisher/publisher.go
git commit -m "feat(analytics): split regular/shorts metrics by post_type"
```

---

## Task 5: Repository tests — Create + Summary + TopClips กับ mix

**Files:**
- Create: `internal/repository/analytics_test.go`

- [ ] **Step 1: ตรวจว่ามี test helper สำหรับ DB pool อยู่แล้วไหม**

Run: `grep -rn "func newTestPool\|TestMain" internal/repository`
Expected: ถ้าไม่มี → step 2 ต้องเขียน helper (skip ถ้ามีอยู่แล้วและตามรูปแบบนั้น)

- [ ] **Step 2: เขียน test (ใช้ helper เดิมถ้ามี ไม่งั้น skip ถ้าไม่มี test infra)**

ถ้าโปรเจคไม่มี integration test infra สำหรับ pgxpool → **ข้าม task นี้ทั้งหมด** และเดินไป Task 6 (จะ verify ด้วย manual fetch + SQL spot check แทน)

ถ้ามี newTestPool หรือคล้ายกัน → เขียนใน `internal/repository/analytics_test.go`:

```go
package repository

import (
	"context"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func TestAnalytics_SplitsRegularAndShorts(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	r := NewAnalyticsRepo(pool)

	clipID := seedPublishedClip(t, pool)

	must(t, r.Create(ctx, models.ClipAnalytics{
		ClipID: clipID, Platform: "youtube", PostType: "regular",
		Views: 1000, Likes: 50, RetentionRate: 0.40,
	}))
	must(t, r.Create(ctx, models.ClipAnalytics{
		ClipID: clipID, Platform: "youtube", PostType: "shorts",
		Views: 200, Likes: 10, RetentionRate: 0.65,
	}))

	rows, err := r.ListByClip(ctx, clipID)
	must(t, err)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows (regular+shorts), got %d", len(rows))
	}

	sum, err := r.Summary(ctx)
	must(t, err)
	if sum.TotalViews != 1200 {
		t.Fatalf("want TotalViews=1200, got %d", sum.TotalViews)
	}
	if sum.TotalLikes != 60 {
		t.Fatalf("want TotalLikes=60, got %d", sum.TotalLikes)
	}

	top, err := r.TopClips(ctx, 5)
	must(t, err)
	if len(top) != 1 || top[0].Views != 1200 {
		t.Fatalf("want 1 clip with 1200 views, got %+v", top)
	}
}
```

- [ ] **Step 3: รัน test**

Run: `go test ./internal/repository/... -run TestAnalytics_SplitsRegularAndShorts -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/repository/analytics_test.go
git commit -m "test(analytics): cover regular/shorts split"
```

---

## Task 6: Frontend — แสดงการ์ดแยก regular/shorts

**Files:**
- Modify: `frontend/src/pages/Analytics.tsx:37-41` (interface)
- Modify: `frontend/src/pages/Analytics.tsx:106-111` (platformMap)
- Modify: `frontend/src/pages/Analytics.tsx:213-235` (render)

- [ ] **Step 1: เพิ่ม post_type ใน interface**

`frontend/src/pages/Analytics.tsx:37-41` เปลี่ยนเป็น:

```typescript
interface ClipAnalytics {
  id: string; clip_id: string; platform: string; post_type: string;
  views: number; likes: number; comments: number; shares: number;
  watch_time_seconds: number; retention_rate: number; fetched_at: string;
}
```

- [ ] **Step 2: เปลี่ยน key ของ platformMap**

`frontend/src/pages/Analytics.tsx:106-111` เปลี่ยนเป็น:

```typescript
const platformMap = useMemo(() => {
  if (detailLoading) return new Map<string, ClipAnalytics>();
  const map = new Map<string, ClipAnalytics>();
  clipAnalytics?.forEach(a => map.set(`${a.platform}-${a.post_type}`, a));
  return map;
}, [clipAnalytics, detailLoading]);
```

- [ ] **Step 3: render การ์ดแยก regular/shorts**

`frontend/src/pages/Analytics.tsx:213-235` เปลี่ยน inner mapping เป็น:

```tsx
<div className="flex flex-wrap gap-2">
  {(['youtube', 'tiktok', 'instagram', 'facebook'] as const).flatMap(p =>
    (['regular', 'shorts'] as const).map(t => {
      const d = platformMap.get(`${p}-${t}`);
      if (!d) return null;
      return (
        <div key={`${p}-${t}`} className="rounded-lg border bg-background p-2.5 min-w-[140px]">
          <div className="text-xs font-medium capitalize mb-1.5">
            {p} <span className="text-muted-foreground">· {t}</span>
          </div>
          <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
            <span className="text-muted-foreground">Views</span>
            <span className="tabular-nums text-right">{formatNum(d.views)}</span>
            <span className="text-muted-foreground">Likes</span>
            <span className="tabular-nums text-right">{formatNum(d.likes)}</span>
            <span className="text-muted-foreground">Retention</span>
            <span className="tabular-nums text-right">{(d.retention_rate * 100).toFixed(1)}%</span>
            <span className="text-muted-foreground">Watch</span>
            <span className="tabular-nums text-right">{formatWatchTime(d.watch_time_seconds)}</span>
          </div>
        </div>
      );
    })
  )}
</div>
```

- [ ] **Step 4: TypeScript/build check**

Run: `cd frontend && npm run build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Analytics.tsx
git commit -m "feat(analytics-ui): show regular and shorts metrics side by side"
```

---

## Task 7: Manual verification — fetch จริง + SQL spot check

- [ ] **Step 1: รัน server local + trigger fetch**

Run: `go run ./cmd/server` (อีก terminal: `curl -X POST http://localhost:8080/api/v1/analytics/fetch`)
Expected: HTTP 202 `{"data":{"status":"triggered"}}`, log จะมี `FetchAnalytics done: ... success`

- [ ] **Step 2: เช็คใน DB ว่ามี row ทั้ง 2 post_type**

Run:
```bash
psql "$DATABASE_URL" -c "
  SELECT clip_id, platform, post_type, views, fetched_at
  FROM clip_analytics
  WHERE fetched_at > NOW() - INTERVAL '5 minutes'
  ORDER BY clip_id, platform, post_type;"
```
Expected: เห็น row `post_type = 'regular'` และ `post_type = 'shorts'` (clip ที่ publish shorts แล้ว) — ไม่ทับกัน

- [ ] **Step 3: เปิดหน้า Analytics ใน browser**

Run: `cd frontend && npm run dev` → เปิด `/analytics` → คลิกแถว Top Clip ที่มีทั้ง regular + shorts
Expected: เห็นการ์ด 2 ใบ "youtube · regular" และ "youtube · shorts" พร้อมตัวเลขแยกกัน

- [ ] **Step 4: ถ้าทุกอย่างถูก — push**

Run: `git push origin master`

---

## Self-Review Notes

- Migration ลบ row เก่า (regular default) เพื่อให้ fetch รอบใหม่ backfill — ก่อน deploy ต้องแน่ใจว่ามี `fetch_analytics` schedule ทำงาน (มีตาม `migrations/014_daily_analytics_cleanup.sql`)
- `AVG(NULLIF(retention_rate, 0))` ใน Summary กัน shorts ที่ยังไม่มี retention ไม่ให้ดึงค่าเฉลี่ยลง — ถ้าทีมอยากให้ shorts ที่ retention=0 นับด้วย ค่อยถอด NULLIF
- `idx_clip_analytics_lookup` รองรับ DISTINCT ON ใหม่ + ordering ของ `latest` CTE
