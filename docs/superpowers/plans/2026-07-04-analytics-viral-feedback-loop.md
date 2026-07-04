# Analytics Viral Feedback Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Store every useful field Zernio provides (YouTube Shorts + TikTok), detect silently-failed publishes, and feed real performance data (views + distribution) back into the Analyzer and topic selection so the Content agents optimize for viral reach.

**Architecture:** Extend the existing pipeline in place — richer parsing in `internal/publisher`, additive migration 049, new repo queries in `internal/repository/analytics.go`, Analyzer v2 prompt/data in `internal/analyzer`, a weighted category pick in `internal/orchestrator`, topic-stats injection into the Question agent, and new sections on the React Analytics page. No new services, no new schedulers.

**Tech Stack:** Go (chi, pgx), PostgreSQL (Neon), React + TypeScript + TanStack Query + Tailwind (shadcn-style components).

**Spec:** `docs/superpowers/specs/2026-07-04-analytics-viral-feedback-loop-design.md`

## Global Constraints

- Next migration number is **049** (048 exists).
- All UI copy in **Thai**.
- List endpoints must return `[]T{}`, never a nil slice (JSON `null` crashes the frontend).
- All new Zernio parsing is **fail-open**: missing field ⇒ zero value; never break the existing metrics path.
- Migration is **additive only** — no ALTER of existing columns, no data rewrites.
- Scale conventions (critical, three different scales):
  - `retention_rate` (existing): fraction, `0.83` = 83%, capped at 1.0.
  - `avg_view_percentage` (new): fraction, `0.83` = 83%, **NOT capped** (Shorts loops can exceed 1.0). Zernio sends percent (`483.11`) — divide by 100 before storing.
  - `engagement_rate` (new): percent as sent by Zernio, `0.83` = 0.83%. Store as-is.
- Verify Go with `go build ./... && go test ./...` from repo root; frontend with `cd frontend && npm run build`.
- Commit after every task. Work on branch `feat/analytics-viral-feedback-loop`.
- Real Zernio responses were captured live on 2026-07-04 (see fixtures in Task 1) — tests assert against those, not invented shapes.

---

### Task 1: Branch, Zernio struct fields, real-response fixtures

**Files:**
- Modify: `internal/publisher/zernio.go:103-130`
- Create: `internal/publisher/testdata/analytics_tiktok_published.json`
- Create: `internal/publisher/testdata/analytics_tiktok_failed.json`
- Create: `internal/publisher/testdata/analytics_youtube_shorts.json`
- Create: `internal/publisher/testdata/youtube_daily_views.json`
- Test: `internal/publisher/zernio_parse_test.go`

**Interfaces:**
- Produces: `PlatformAnalyticsEntry.Status string`, `PlatformAnalyticsEntry.ErrorMessage string`, `DailyViewEntry.AverageViewPercentage float64` — Tasks 4+ rely on these exact names.

- [ ] **Step 1: Create branch**

```bash
git checkout -b feat/analytics-viral-feedback-loop
```

- [ ] **Step 2: Write fixtures from the real captured responses**

`internal/publisher/testdata/analytics_tiktok_published.json` (trimmed real response, post `6a45c62213915deb5bf73441`):

```json
{
  "postId": "6a45c62213915deb5bf73441",
  "status": "published",
  "publishedAt": "2026-07-02T02:00:20.183Z",
  "analytics": {
    "impressions": 0, "reach": 0, "likes": 1, "comments": 0, "shares": 0,
    "saves": 0, "clicks": 0, "views": 120,
    "lastUpdated": "2026-07-04 02:25:57", "engagementRate": 0.83
  },
  "platformAnalytics": [
    {
      "platform": "tiktok",
      "status": "published",
      "platformPostId": "7657744600987274516",
      "accountId": "6a2aafab5f7d1751ab805933",
      "analytics": {
        "impressions": 0, "reach": 0, "likes": 1, "comments": 0, "shares": 0,
        "saves": 0, "clicks": 0, "views": 120,
        "engagementRate": 0.83, "lastUpdated": "2026-07-04 02:25:57"
      },
      "syncStatus": "synced",
      "errorMessage": null
    }
  ],
  "syncStatus": "synced"
}
```

`internal/publisher/testdata/analytics_tiktok_failed.json` (trimmed real response, post `6a3c0d160cdd4abde1986dff`):

```json
{
  "postId": "6a3c0d160cdd4abde1986dff",
  "status": "failed",
  "publishedAt": null,
  "analytics": {
    "impressions": 0, "reach": 0, "likes": 0, "comments": 0, "shares": 0,
    "saves": 0, "clicks": 0, "views": 0, "engagementRate": 0, "lastUpdated": null
  },
  "platformAnalytics": [
    {
      "platform": "tiktok",
      "status": "failed",
      "platformPostId": null,
      "accountId": "6a2aafab5f7d1751ab805933",
      "analytics": null,
      "syncStatus": "unavailable",
      "errorMessage": "TikTok could not download the video. This may be a temporary network issue - please try again."
    }
  ],
  "syncStatus": "unavailable",
  "message": "This post failed to publish. Analytics are not available. Please check the error details for each platform."
}
```

`internal/publisher/testdata/analytics_youtube_shorts.json` (trimmed real response, post `6a28f171a4e37c2c37c3e9e5`):

```json
{
  "postId": "6a28f171a4e37c2c37c3e9e5",
  "status": "published",
  "publishedAt": "2026-06-10T05:09:45.045Z",
  "analytics": {
    "impressions": 0, "reach": 0, "likes": 0, "comments": 0, "shares": 0,
    "saves": 0, "clicks": 0, "views": 29,
    "lastUpdated": "2026-07-03 18:51:17", "engagementRate": 0
  },
  "platformAnalytics": [
    {
      "platform": "youtube",
      "status": "published",
      "platformPostId": "vb0MWSQLw4g",
      "accountId": "69eee157985e734bf3bd8128",
      "analytics": {
        "impressions": 0, "reach": 0, "likes": 0, "comments": 0, "shares": 0,
        "saves": 0, "clicks": 0, "views": 29,
        "engagementRate": 0, "lastUpdated": "2026-07-03 18:51:17"
      },
      "syncStatus": "synced",
      "errorMessage": null
    }
  ],
  "syncStatus": "synced"
}
```

`internal/publisher/testdata/youtube_daily_views.json` (trimmed real response, video `vb0MWSQLw4g` — note the >100% averageViewPercentage, which is real Shorts-loop behavior):

```json
{
  "success": true,
  "videoId": "vb0MWSQLw4g",
  "durationSeconds": 61,
  "totalViews": 30,
  "dailyViews": [
    {
      "date": "2026-06-28", "views": 0, "estimatedMinutesWatched": 0,
      "averageViewDuration": 0, "averageViewPercentage": 0,
      "subscribersGained": 0, "subscribersLost": 0,
      "likes": 0, "comments": 0, "shares": 0
    },
    {
      "date": "2026-06-26", "views": 5, "estimatedMinutesWatched": 4,
      "averageViewDuration": 294, "averageViewPercentage": 483.11,
      "subscribersGained": 0, "subscribersLost": 0,
      "likes": 0, "comments": 0, "shares": 0
    },
    {
      "date": "2026-06-25", "views": 10, "estimatedMinutesWatched": 6,
      "averageViewDuration": 36, "averageViewPercentage": 59.0,
      "subscribersGained": 1, "subscribersLost": 0,
      "likes": 1, "comments": 0, "shares": 0
    }
  ],
  "scopeStatus": { "hasAnalyticsScope": true },
  "lastSyncedAt": "2026-07-03T18:51:17.000Z"
}
```

- [ ] **Step 3: Write the failing test**

`internal/publisher/zernio_parse_test.go`:

```go
package publisher

import (
	"encoding/json"
	"os"
	"testing"
)

func loadFixture(t *testing.T, name string, v any) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
}

func TestParseTikTokPublished(t *testing.T) {
	var resp AnalyticsResponse
	loadFixture(t, "analytics_tiktok_published.json", &resp)

	if resp.Status != "published" {
		t.Errorf("status = %q, want published", resp.Status)
	}
	pa := resp.PlatformAnalytics[0]
	if pa.Status != "published" {
		t.Errorf("platform status = %q, want published", pa.Status)
	}
	if pa.ErrorMessage != "" {
		t.Errorf("errorMessage = %q, want empty", pa.ErrorMessage)
	}
	if pa.Analytics.Views != 120 || pa.Analytics.EngagementRate != 0.83 {
		t.Errorf("views/engagement = %d/%v, want 120/0.83", pa.Analytics.Views, pa.Analytics.EngagementRate)
	}
}

func TestParseTikTokFailed(t *testing.T) {
	var resp AnalyticsResponse
	loadFixture(t, "analytics_tiktok_failed.json", &resp)

	if resp.Status != "failed" {
		t.Errorf("status = %q, want failed", resp.Status)
	}
	pa := resp.PlatformAnalytics[0]
	if pa.Status != "failed" {
		t.Errorf("platform status = %q, want failed", pa.Status)
	}
	if pa.ErrorMessage == "" {
		t.Error("errorMessage empty, want TikTok download error")
	}
	// analytics is JSON null for failed posts — must not panic, must zero-value.
	if pa.Analytics.Views != 0 {
		t.Errorf("views = %d, want 0", pa.Analytics.Views)
	}
	if resp.Analytics.LastUpdated != "" {
		t.Errorf("flat lastUpdated = %q, want empty (failed post has no analytics)", resp.Analytics.LastUpdated)
	}
}

func TestParseYouTubeDailyViews(t *testing.T) {
	var resp YouTubeDailyViewsResponse
	loadFixture(t, "youtube_daily_views.json", &resp)

	if len(resp.DailyViews) != 3 {
		t.Fatalf("daily entries = %d, want 3", len(resp.DailyViews))
	}
	d := resp.DailyViews[1]
	if d.AverageViewPercentage != 483.11 {
		t.Errorf("averageViewPercentage = %v, want 483.11", d.AverageViewPercentage)
	}
	if resp.DailyViews[2].SubscribersGained != 1 {
		t.Errorf("subscribersGained = %d, want 1", resp.DailyViews[2].SubscribersGained)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/publisher/ -run 'TestParse' -v`
Expected: FAIL — `pa.Status undefined`, `pa.ErrorMessage undefined`, `d.AverageViewPercentage undefined` (compile errors).

- [ ] **Step 5: Add the struct fields**

In `internal/publisher/zernio.go`, change `PlatformAnalyticsEntry` to:

```go
type PlatformAnalyticsEntry struct {
	Platform       string      `json:"platform"`
	Status         string      `json:"status"`
	PlatformPostID string      `json:"platformPostId"`
	AccountID      string      `json:"accountId"`
	Analytics      PostMetrics `json:"analytics"`
	SyncStatus     string      `json:"syncStatus"`
	ErrorMessage   string      `json:"errorMessage"`
}
```

And add one field to `DailyViewEntry` (after `AverageViewDuration`):

```go
	AverageViewPercentage   float64 `json:"averageViewPercentage"`
```

Note: `"analytics": null` unmarshals into the `PostMetrics` value as all-zeros without error — no pointer change needed.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/publisher/ -v`
Expected: PASS (including pre-existing tests in `zernio_test.go`).

- [ ] **Step 7: Commit**

```bash
git add internal/publisher/zernio.go internal/publisher/zernio_parse_test.go internal/publisher/testdata/
git commit -m "feat(analytics): parse Zernio publish status, errorMessage, averageViewPercentage + real-response fixtures"
```

---

### Task 2: Migration 049 + Go models

**Files:**
- Create: `migrations/049_analytics_full_fetch.sql`
- Modify: `internal/models/clip.go:96-108` (ClipAnalytics) and append new types at end of file

**Interfaces:**
- Produces: DB columns `clip_analytics.engagement_rate/avg_view_percentage/subscribers_gained/subscribers_lost`; tables `clip_analytics_daily`, `clip_publish_status`; settings row `topic_stats_enabled='true'`.
- Produces Go types: `models.ClipAnalyticsDaily`, `models.ClipPublishStatus`, `models.PublishFailure`, `models.CategoryScore`; extended `models.ClipAnalytics` and `models.PlatformTotals`.

- [ ] **Step 1: Write the migration**

`migrations/049_analytics_full_fetch.sql`:

```sql
-- 049: store everything Zernio actually provides + publish status.
-- Additive only. Rollback: revert the code; these columns/tables are inert without it.

ALTER TABLE clip_analytics ADD COLUMN IF NOT EXISTS engagement_rate FLOAT NOT NULL DEFAULT 0;
ALTER TABLE clip_analytics ADD COLUMN IF NOT EXISTS avg_view_percentage FLOAT NOT NULL DEFAULT 0;
ALTER TABLE clip_analytics ADD COLUMN IF NOT EXISTS subscribers_gained INT NOT NULL DEFAULT 0;
ALTER TABLE clip_analytics ADD COLUMN IF NOT EXISTS subscribers_lost INT NOT NULL DEFAULT 0;

-- Per-day YouTube analytics (Zernio daily-views endpoint). TikTok has no daily
-- endpoint; its trend is derived from successive clip_analytics snapshots instead.
CREATE TABLE IF NOT EXISTS clip_analytics_daily (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    post_type TEXT NOT NULL DEFAULT 'shorts',
    date DATE NOT NULL,
    views INT NOT NULL DEFAULT 0,
    estimated_minutes_watched FLOAT NOT NULL DEFAULT 0,
    average_view_duration FLOAT NOT NULL DEFAULT 0,
    avg_view_percentage FLOAT NOT NULL DEFAULT 0, -- fraction: 0.83 = 83%
    subscribers_gained INT NOT NULL DEFAULT 0,
    subscribers_lost INT NOT NULL DEFAULT 0,
    likes INT NOT NULL DEFAULT 0,
    comments INT NOT NULL DEFAULT 0,
    shares INT NOT NULL DEFAULT 0,
    UNIQUE (clip_id, platform, post_type, date)
);
CREATE INDEX IF NOT EXISTS idx_clip_analytics_daily_clip ON clip_analytics_daily (clip_id, date DESC);

-- Publish outcome per Zernio post, refreshed on every analytics fetch.
-- status='failed' rows are excluded from all learning queries.
CREATE TABLE IF NOT EXISTS clip_publish_status (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    post_type TEXT NOT NULL DEFAULT 'regular',
    zernio_post_id TEXT NOT NULL,
    status TEXT NOT NULL, -- published | failed | scheduled | unknown
    error_message TEXT,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (clip_id, platform, post_type)
);
CREATE INDEX IF NOT EXISTS idx_clip_publish_status_failed ON clip_publish_status (status) WHERE status = 'failed';

-- Kill switch for feeding topic performance into category pick + question prompt.
INSERT INTO settings (key, value) VALUES ('topic_stats_enabled', 'true')
ON CONFLICT (key) DO NOTHING;
```

- [ ] **Step 2: Extend models**

In `internal/models/clip.go`, extend `ClipAnalytics` (insert after `RetentionRate`):

```go
	EngagementRate    float64 `json:"engagement_rate"`     // percent: 0.83 = 0.83%
	AvgViewPercentage float64 `json:"avg_view_percentage"` // fraction: 0.83 = 83%, may exceed 1 (Shorts loops)
	SubscribersGained int     `json:"subscribers_gained"`
	SubscribersLost   int     `json:"subscribers_lost"`
```

Extend `PlatformTotals` (after `AvgRetention`):

```go
	EngagementRate    float64 `json:"engagement_rate"`
	SubscribersGained int     `json:"subscribers_gained"`
```

Extend `ClipPerformance` (after `WatchTimeSeconds`):

```go
	Sparkline       []int    `json:"sparkline"`        // daily view deltas, oldest→newest
	FailedPlatforms []string `json:"failed_platforms"` // platforms whose publish failed
```

Append new types at end of file:

```go
// ClipAnalyticsDaily is one day of YouTube analytics for one post,
// upserted from Zernio's daily-views endpoint on every fetch.
type ClipAnalyticsDaily struct {
	ClipID                  string  `json:"clip_id"`
	Platform                string  `json:"platform"`
	PostType                string  `json:"post_type"`
	Date                    string  `json:"date"` // YYYY-MM-DD as sent by Zernio
	Views                   int     `json:"views"`
	EstimatedMinutesWatched float64 `json:"estimated_minutes_watched"`
	AverageViewDuration     float64 `json:"average_view_duration"`
	AvgViewPercentage       float64 `json:"avg_view_percentage"` // fraction
	SubscribersGained       int     `json:"subscribers_gained"`
	SubscribersLost         int     `json:"subscribers_lost"`
	Likes                   int     `json:"likes"`
	Comments                int     `json:"comments"`
	Shares                  int     `json:"shares"`
}

// ClipPublishStatus is the last-seen publish outcome of one Zernio post.
type ClipPublishStatus struct {
	ClipID       string  `json:"clip_id"`
	Platform     string  `json:"platform"`
	PostType     string  `json:"post_type"`
	ZernioPostID string  `json:"zernio_post_id"`
	Status       string  `json:"status"`
	ErrorMessage *string `json:"error_message"`
}

// PublishFailure is one failed publish surfaced on the Analytics page.
type PublishFailure struct {
	ClipID       string    `json:"clip_id"`
	Title        string    `json:"title"`
	Platform     string    `json:"platform"`
	PostType     string    `json:"post_type"`
	ErrorMessage string    `json:"error_message"`
	CheckedAt    time.Time `json:"checked_at"`
}

// CategoryScore is one topic category's measured performance: mean within-platform
// views percentile (0..1) across its clips over a recent window.
type CategoryScore struct {
	Category      string  `json:"category"`
	AvgPercentile float64 `json:"avg_percentile"`
	AvgViews      float64 `json:"avg_views"`
	N             int     `json:"n"`
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add migrations/049_analytics_full_fetch.sql internal/models/clip.go
git commit -m "feat(analytics): migration 049 (daily analytics, publish status, new metric columns) + models"
```

---

### Task 3: Repository writes + widened CTE

**Files:**
- Modify: `internal/repository/analytics.go:12-18` (CTE), `:279-291` (Create), `:144-173` (SummaryByPlatform), and append new methods

**Interfaces:**
- Consumes: models from Task 2.
- Produces: `(r *AnalyticsRepo) UpsertDaily(ctx, d models.ClipAnalyticsDaily) error`, `(r *AnalyticsRepo) UpsertPublishStatus(ctx, s models.ClipPublishStatus) error` — Task 4 calls both with exactly these signatures.

- [ ] **Step 1: Widen the latest-analytics CTE**

Replace `latestAnalyticsCTE` (analytics.go:12-18) with:

```go
const latestAnalyticsCTE = `WITH latest AS (
	SELECT DISTINCT ON (clip_id, platform, post_type)
		clip_id, platform, post_type, views, likes, comments, shares,
		watch_time_seconds, retention_rate,
		engagement_rate, avg_view_percentage, subscribers_gained
	FROM clip_analytics
	ORDER BY clip_id, platform, post_type, fetched_at DESC
)`
```

- [ ] **Step 2: Extend Create**

Replace the `Create` method body's INSERT with:

```go
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_analytics (clip_id, platform, post_type, views, likes, comments, shares,
		    watch_time_seconds, retention_rate, engagement_rate, avg_view_percentage,
		    subscribers_gained, subscribers_lost)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		a.ClipID, a.Platform, a.PostType, a.Views, a.Likes, a.Comments, a.Shares,
		a.WatchTimeSeconds, a.RetentionRate, a.EngagementRate, a.AvgViewPercentage,
		a.SubscribersGained, a.SubscribersLost)
```

- [ ] **Step 3: Add upsert methods** (append to analytics.go)

```go
// UpsertDaily records one day of YouTube analytics for a post. Re-fetches
// overwrite the same (clip, platform, post_type, date) row because Zernio's
// recent days keep moving for ~48h.
func (r *AnalyticsRepo) UpsertDaily(ctx context.Context, d models.ClipAnalyticsDaily) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_analytics_daily
		    (clip_id, platform, post_type, date, views, estimated_minutes_watched,
		     average_view_duration, avg_view_percentage, subscribers_gained,
		     subscribers_lost, likes, comments, shares)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 ON CONFLICT (clip_id, platform, post_type, date) DO UPDATE SET
		    views = EXCLUDED.views,
		    estimated_minutes_watched = EXCLUDED.estimated_minutes_watched,
		    average_view_duration = EXCLUDED.average_view_duration,
		    avg_view_percentage = EXCLUDED.avg_view_percentage,
		    subscribers_gained = EXCLUDED.subscribers_gained,
		    subscribers_lost = EXCLUDED.subscribers_lost,
		    likes = EXCLUDED.likes,
		    comments = EXCLUDED.comments,
		    shares = EXCLUDED.shares`,
		d.ClipID, d.Platform, d.PostType, d.Date, d.Views, d.EstimatedMinutesWatched,
		d.AverageViewDuration, d.AvgViewPercentage, d.SubscribersGained,
		d.SubscribersLost, d.Likes, d.Comments, d.Shares)
	if err != nil {
		return fmt.Errorf("upsert daily analytics: %w", err)
	}
	return nil
}

// UpsertPublishStatus records the last-seen publish outcome of one Zernio post.
func (r *AnalyticsRepo) UpsertPublishStatus(ctx context.Context, s models.ClipPublishStatus) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO clip_publish_status (clip_id, platform, post_type, zernio_post_id, status, error_message, checked_at)
		 VALUES ($1,$2,$3,$4,$5,$6,NOW())
		 ON CONFLICT (clip_id, platform, post_type) DO UPDATE SET
		    zernio_post_id = EXCLUDED.zernio_post_id,
		    status = EXCLUDED.status,
		    error_message = EXCLUDED.error_message,
		    checked_at = NOW()`,
		s.ClipID, s.Platform, s.PostType, s.ZernioPostID, s.Status, s.ErrorMessage)
	if err != nil {
		return fmt.Errorf("upsert publish status: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: SummaryByPlatform reads the new columns**

Replace the SELECT inside `SummaryByPlatform` with:

```go
	rows, err := r.pool.Query(ctx, latestAnalyticsCTE+`
		SELECT l.platform,
		       COALESCE(SUM(l.views),0),
		       COALESCE(SUM(l.likes),0),
		       COALESCE(SUM(l.comments),0),
		       COALESCE(SUM(l.shares),0),
		       COALESCE(SUM(l.watch_time_seconds),0),
		       COALESCE(AVG(NULLIF(l.avg_view_percentage, 0)), AVG(NULLIF(l.retention_rate, 0)), 0),
		       COALESCE(AVG(NULLIF(l.engagement_rate, 0)), 0),
		       COALESCE(SUM(l.subscribers_gained), 0)
		FROM latest l
		GROUP BY l.platform
		ORDER BY SUM(l.views) DESC`)
```

And extend the Scan to match:

```go
		if err := rows.Scan(&p.Platform, &p.Views, &p.Likes, &p.Comments,
			&p.Shares, &p.WatchTimeSeconds, &p.AvgRetention,
			&p.EngagementRate, &p.SubscribersGained); err != nil {
```

- [ ] **Step 5: Verify build + existing tests**

Run: `go build ./... && go test ./...`
Expected: success (repo methods are DB-backed and exercised in prod verification, matching this codebase's convention).

- [ ] **Step 6: Commit**

```bash
git add internal/repository/analytics.go
git commit -m "feat(analytics): repo writes for daily rows + publish status, platform totals expose engagement/subscribers"
```

---

### Task 4: FetchAnalytics stores everything

**Files:**
- Modify: `internal/publisher/publisher.go:294-467`
- Test: `internal/publisher/publisher_status_test.go` (new)

**Interfaces:**
- Consumes: `UpsertDaily`, `UpsertPublishStatus` (Task 3); struct fields (Task 1).
- Produces: `normalizePublishStatus(s string) string`, `resolvePostStatus(resp *AnalyticsResponse, platform string) (status, errMsg string)` — pure helpers with tests; enriched `ytDetail` struct.

- [ ] **Step 1: Write the failing test**

`internal/publisher/publisher_status_test.go`:

```go
package publisher

import "testing"

func TestNormalizePublishStatus(t *testing.T) {
	cases := map[string]string{
		"published":  "published",
		"failed":     "failed",
		"scheduled":  "scheduled",
		"pending":    "scheduled",
		"processing": "scheduled",
		"":           "unknown",
		"weird":      "unknown",
	}
	for in, want := range cases {
		if got := normalizePublishStatus(in); got != want {
			t.Errorf("normalizePublishStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolvePostStatus(t *testing.T) {
	var failed AnalyticsResponse
	loadFixture(t, "analytics_tiktok_failed.json", &failed)
	status, errMsg := resolvePostStatus(&failed, "tiktok")
	if status != "failed" {
		t.Errorf("status = %q, want failed", status)
	}
	if errMsg == "" {
		t.Error("errMsg empty, want TikTok error text")
	}

	var ok AnalyticsResponse
	loadFixture(t, "analytics_tiktok_published.json", &ok)
	status, errMsg = resolvePostStatus(&ok, "tiktok")
	if status != "published" || errMsg != "" {
		t.Errorf("status/err = %q/%q, want published/empty", status, errMsg)
	}

	// Platform entry missing entirely → fall back to top-level status.
	empty := &AnalyticsResponse{Status: "scheduled"}
	status, _ = resolvePostStatus(empty, "tiktok")
	if status != "scheduled" {
		t.Errorf("fallback status = %q, want scheduled", status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/publisher/ -run 'TestNormalizePublishStatus|TestResolvePostStatus' -v`
Expected: FAIL — `normalizePublishStatus`/`resolvePostStatus` undefined.

- [ ] **Step 3: Implement helpers** (append to publisher.go)

```go
// normalizePublishStatus maps Zernio's status strings onto our 4-value enum.
func normalizePublishStatus(s string) string {
	switch s {
	case "published", "failed", "scheduled":
		return s
	case "pending", "processing":
		return "scheduled"
	default:
		return "unknown"
	}
}

// resolvePostStatus prefers the matching platformAnalytics entry's status and
// errorMessage, falling back to the response-level status.
func resolvePostStatus(resp *AnalyticsResponse, platform string) (string, string) {
	for _, pa := range resp.PlatformAnalytics {
		if pa.Platform == platform && pa.Status != "" {
			return normalizePublishStatus(pa.Status), pa.ErrorMessage
		}
	}
	return normalizePublishStatus(resp.Status), ""
}
```

- [ ] **Step 4: Run helper tests**

Run: `go test ./internal/publisher/ -run 'TestNormalizePublishStatus|TestResolvePostStatus' -v`
Expected: PASS.

- [ ] **Step 5: Wire into FetchAnalytics**

Inside the `for _, post := range posts` loop in `FetchAnalytics`, right after the successful `GetAnalytics` call (publisher.go:365-370), insert:

```go
			status, errMsg := resolvePostStatus(resp, post.platform)
			var errPtr *string
			if errMsg != "" {
				errPtr = &errMsg
			}
			if err := p.analytics.UpsertPublishStatus(ctx, models.ClipPublishStatus{
				ClipID: cp.ClipID, Platform: post.platform, PostType: post.label,
				ZernioPostID: post.id, Status: status, ErrorMessage: errPtr,
			}); err != nil {
				log.Printf("FetchAnalytics STATUS_FAIL clip=%s platform=%s: %v", cp.ClipID, post.platform, err) // non-fatal
			}
```

Replace the YouTube-detail block (publisher.go:392-395):

```go
			watchTime, retention := 0.0, 0.0
			var detail ytDetail
			if post.platform == platformYouTube && metrics.Views > 0 {
				detail = p.fetchYouTubeDetail(ctx, cp.ClipID, ytAccountID, resp)
				watchTime, retention = detail.WatchTime, detail.Retention
			}
```

Replace the `p.analytics.Create` call's struct literal with:

```go
			if err := p.analytics.Create(ctx, models.ClipAnalytics{
				ClipID:            cp.ClipID,
				Platform:          post.platform,
				PostType:          post.label,
				Views:             metrics.Views,
				Likes:             metrics.Likes,
				Comments:          metrics.Comments,
				Shares:            metrics.Shares,
				WatchTimeSeconds:  watchTime,
				RetentionRate:     retention,
				EngagementRate:    metrics.EngagementRate,
				AvgViewPercentage: detail.AvgViewPct,
				SubscribersGained: detail.SubsGained,
				SubscribersLost:   detail.SubsLost,
			}); err != nil {
```

After the successful `Create` (before `success++`), persist daily rows:

```go
			for _, dv := range detail.Daily {
				if err := p.analytics.UpsertDaily(ctx, models.ClipAnalyticsDaily{
					ClipID: cp.ClipID, Platform: post.platform, PostType: post.label,
					Date: dv.Date, Views: dv.Views,
					EstimatedMinutesWatched: dv.EstimatedMinutesWatched,
					AverageViewDuration:     dv.AverageViewDuration,
					AvgViewPercentage:       dv.AverageViewPercentage / 100.0,
					SubscribersGained:       dv.SubscribersGained,
					SubscribersLost:         dv.SubscribersLost,
					Likes:                   dv.Likes, Comments: dv.Comments, Shares: dv.Shares,
				}); err != nil {
					log.Printf("FetchAnalytics DAILY_FAIL clip=%s date=%s: %v", cp.ClipID, dv.Date, err) // non-fatal
				}
			}
```

- [ ] **Step 6: Rework fetchYouTubeWatchTime into fetchYouTubeDetail**

Replace the whole `fetchYouTubeWatchTime` function (publisher.go:418-467) with:

```go
// ytDetail carries everything the daily-views endpoint gives us for one video.
type ytDetail struct {
	WatchTime  float64 // seconds, summed over the window
	Retention  float64 // legacy: avg view duration / clip duration, capped at 1
	AvgViewPct float64 // fraction from YouTube's own averageViewPercentage, view-weighted, NOT capped
	SubsGained int
	SubsLost   int
	Daily      []DailyViewEntry
}

func (p *Publisher) fetchYouTubeDetail(ctx context.Context, clipID, ytAccountID string, resp *AnalyticsResponse) ytDetail {
	var d ytDetail
	if ytAccountID == "" {
		return d
	}
	var videoID string
	for _, pa := range resp.PlatformAnalytics {
		if pa.Platform == platformYouTube {
			videoID = pa.PlatformPostID
			break
		}
	}
	if videoID == "" {
		return d
	}
	daily, err := p.zernio.GetYouTubeDailyViews(ctx, videoID, ytAccountID)
	if err != nil {
		if errors.Is(err, ErrYouTubeScopeMissing) {
			log.Printf("FetchAnalytics: YouTube analytics scope missing — re-auth needed (skipping watchTime)")
		} else {
			log.Printf("FetchAnalytics WATCHTIME_FAIL clip=%s video=%s: %v", clipID, videoID, err)
		}
		return d
	}
	if len(daily.DailyViews) == 0 {
		return d
	}
	d.Daily = daily.DailyViews

	var totalMinutes, avgDurSum, pctWeighted float64
	var viewSum int
	for _, dv := range daily.DailyViews {
		totalMinutes += dv.EstimatedMinutesWatched
		avgDurSum += dv.AverageViewDuration
		d.SubsGained += dv.SubscribersGained
		d.SubsLost += dv.SubscribersLost
		if dv.Views > 0 {
			pctWeighted += dv.AverageViewPercentage * float64(dv.Views)
			viewSum += dv.Views
		}
	}
	d.WatchTime = totalMinutes * 60
	if viewSum > 0 {
		d.AvgViewPct = pctWeighted / float64(viewSum) / 100.0
	}

	// Legacy retention: avg view duration / total clip duration, capped at 1.
	var clipDuration float64
	if err := p.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(duration_seconds), 0) FROM scenes WHERE clip_id = $1`, clipID,
	).Scan(&clipDuration); err != nil {
		log.Printf("FetchAnalytics RETENTION_FAIL clip=%s: query clip duration: %v", clipID, err)
		return d
	}
	if clipDuration > 0 {
		avgViewDuration := avgDurSum / float64(len(daily.DailyViews))
		d.Retention = avgViewDuration / clipDuration
		if d.Retention > 1.0 {
			d.Retention = 1.0
		}
	}
	return d
}
```

- [ ] **Step 7: Build + full package tests**

Run: `go build ./... && go test ./internal/publisher/ -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/publisher/
git commit -m "feat(analytics): FetchAnalytics stores engagement/avg-view-pct/subscribers, daily rows, publish status"
```

---

### Task 5: Exclude failed publishes from preset learning

**Files:**
- Modify: `internal/repository/analytics.go:244-277` (PresetRetention)

**Interfaces:**
- Consumes: `clip_publish_status` table (Task 2).

- [ ] **Step 1: Add the exclusion to PresetRetention**

In the `latest` CTE inside `PresetRetention`, add after the `WHERE fetched_at >= ...` line:

```sql
			  AND NOT EXISTS (
				SELECT 1 FROM clip_publish_status ps
				WHERE ps.clip_id = clip_analytics.clip_id
				  AND ps.platform = clip_analytics.platform
				  AND ps.post_type = clip_analytics.post_type
				  AND ps.status = 'failed')
```

(Table-qualify the CTE's source columns: the subquery correlates against `clip_analytics`.)

- [ ] **Step 2: Build + test**

Run: `go build ./... && go test ./internal/producer/ ./internal/repository/ 2>/dev/null; go test ./...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/repository/analytics.go
git commit -m "fix(analytics): exclude failed publishes from preset retention learning"
```

---

### Task 6: Analyzer v2 — both platforms, hooks, percentiles, trends, topic learning

**Files:**
- Modify: `internal/analyzer/analyzer.go` (gatherData + prompt + gate)
- Create: `internal/analyzer/stats.go`
- Test: `internal/analyzer/stats_test.go`

**Interfaces:**
- Produces: `TrendLabel(dailyViews []int) string` (returns `"rising" | "peaked" | "steady" | "unknown"`), `FillPercentiles(stats []ClipStat)`, `BuildAnalysisData(stats []ClipStat) (string, int)` where the int is the count of distinct clips.
- `ClipStat` struct (exported from `stats.go`):

```go
type ClipStat struct {
	ID, Title, Category, Hook string
	Platform                  string
	Views, Likes, Comments, Shares int
	EngagementRate            float64
	AvgViewPct                float64
	SubsGained                int
	Percentile                float64
	Trend                     string
}
```

- [ ] **Step 1: Write the failing tests**

`internal/analyzer/stats_test.go`:

```go
package analyzer

import (
	"strings"
	"testing"
)

func TestTrendLabel(t *testing.T) {
	cases := []struct {
		name  string
		views []int
		want  string
	}{
		{"too few points", []int{10, 20}, "unknown"},
		{"no growth", []int{100, 100, 100, 100}, "steady"},
		{"rising: most growth is recent", []int{100, 110, 150, 220}, "rising"},
		{"peaked: growth stopped", []int{10, 80, 100, 102}, "peaked"},
		{"steady climb", []int{10, 40, 70, 100}, "rising"},
	}
	for _, c := range cases {
		if got := TrendLabel(c.views); got != c.want {
			t.Errorf("%s: TrendLabel(%v) = %q, want %q", c.name, c.views, got, c.want)
		}
	}
}

func TestFillPercentiles(t *testing.T) {
	stats := []ClipStat{
		{ID: "a", Platform: "youtube", Views: 10},
		{ID: "b", Platform: "youtube", Views: 100},
		{ID: "c", Platform: "youtube", Views: 50},
		{ID: "d", Platform: "tiktok", Views: 5}, // alone on its platform → percentile 1.0
	}
	FillPercentiles(stats)
	if stats[1].Percentile != 1.0 {
		t.Errorf("top youtube percentile = %v, want 1.0", stats[1].Percentile)
	}
	if stats[0].Percentile != 0.0 {
		t.Errorf("bottom youtube percentile = %v, want 0.0", stats[0].Percentile)
	}
	if stats[2].Percentile != 0.5 {
		t.Errorf("mid youtube percentile = %v, want 0.5", stats[2].Percentile)
	}
	if stats[3].Percentile != 1.0 {
		t.Errorf("solo tiktok percentile = %v, want 1.0", stats[3].Percentile)
	}
}

func TestBuildAnalysisData(t *testing.T) {
	stats := []ClipStat{
		{ID: "aaaaaaaa-1111", Title: "คลิปหนึ่ง", Category: "payment", Hook: "เคยไหมโดนแบน?",
			Platform: "tiktok", Views: 120, Likes: 1, EngagementRate: 0.83,
			Percentile: 0.9, Trend: "rising"},
	}
	data, n := BuildAnalysisData(stats)
	if n != 1 {
		t.Errorf("clip count = %d, want 1", n)
	}
	for _, want := range []string{"payment", "เคยไหมโดนแบน?", "tiktok", "P90", "rising"} {
		if !strings.Contains(data, want) {
			t.Errorf("data missing %q:\n%s", want, data)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/analyzer/ -run 'TestTrendLabel|TestFillPercentiles|TestBuildAnalysisData' -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement stats.go**

`internal/analyzer/stats.go`:

```go
package analyzer

import (
	"fmt"
	"sort"
	"strings"
)

// ClipStat is one clip's latest performance on one platform, assembled for the
// weekly LLM analysis.
type ClipStat struct {
	ID, Title, Category, Hook string
	Platform                  string
	Views, Likes, Comments, Shares int
	EngagementRate            float64
	AvgViewPct                float64
	SubsGained                int
	Percentile                float64 // 0..1 within platform, filled by FillPercentiles
	Trend                     string  // rising | peaked | steady | unknown
}

// TrendLabel classifies a clip's daily cumulative view counts (oldest→newest).
// "rising" = at least half of the window's growth happened in the last two days;
// "peaked" = the window grew but the last two days contributed under 20%.
func TrendLabel(dailyViews []int) string {
	if len(dailyViews) < 3 {
		return "unknown"
	}
	total := dailyViews[len(dailyViews)-1] - dailyViews[0]
	if total <= 0 {
		return "steady"
	}
	recent := dailyViews[len(dailyViews)-1] - dailyViews[len(dailyViews)-3]
	switch {
	case float64(recent) >= float64(total)/2:
		return "rising"
	case float64(recent) <= float64(total)/5:
		return "peaked"
	default:
		return "steady"
	}
}

// FillPercentiles sets each stat's within-platform views percentile (0 = worst,
// 1 = best). A platform with a single clip gets 1.0.
func FillPercentiles(stats []ClipStat) {
	byPlatform := map[string][]int{}
	for i := range stats {
		byPlatform[stats[i].Platform] = append(byPlatform[stats[i].Platform], i)
	}
	for _, idxs := range byPlatform {
		sort.Slice(idxs, func(a, b int) bool { return stats[idxs[a]].Views < stats[idxs[b]].Views })
		n := len(idxs)
		for rank, i := range idxs {
			if n == 1 {
				stats[i].Percentile = 1.0
			} else {
				stats[i].Percentile = float64(rank) / float64(n-1)
			}
		}
	}
}

// BuildAnalysisData renders stats as one line per clip-platform for the LLM
// and returns the number of distinct clips.
func BuildAnalysisData(stats []ClipStat) (string, int) {
	seen := map[string]bool{}
	var lines []string
	for _, s := range stats {
		seen[s.ID] = true
		id := s.ID
		if len(id) > 8 {
			id = id[:8]
		}
		line := fmt.Sprintf(
			"- Clip %s | Platform: %s | Category: %s | Title: %s | Hook: %s | Views: %d (P%.0f within platform) | Likes: %d | Comments: %d | Shares: %d | Engagement: %.2f%% | Trend: %s",
			id, s.Platform, s.Category, s.Title, s.Hook,
			s.Views, s.Percentile*100, s.Likes, s.Comments, s.Shares, s.EngagementRate, s.Trend)
		if s.Platform == "youtube" {
			line += fmt.Sprintf(" | AvgViewPct: %.0f%% | SubsGained: %d", s.AvgViewPct*100, s.SubsGained)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), len(seen)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/analyzer/ -v`
Expected: PASS (including existing guardrail tests).

- [ ] **Step 5: Rewrite gatherData and the prompt in analyzer.go**

Replace `gatherData` (analyzer.go:125-173) with a version returning `([]ClipStat, error)`:

```go
func (a *Analyzer) gatherData(ctx context.Context) ([]ClipStat, error) {
	rows, err := a.pool.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (ca.clip_id, ca.platform)
				ca.clip_id, ca.platform, ca.views, ca.likes, ca.comments, ca.shares,
				ca.engagement_rate, ca.avg_view_percentage, ca.subscribers_gained
			FROM clip_analytics ca
			WHERE ca.fetched_at >= NOW() - INTERVAL '14 days'
			  AND ca.platform IN ('youtube', 'tiktok')
			ORDER BY ca.clip_id, ca.platform, ca.fetched_at DESC
		)
		SELECT c.id, c.title, c.category, COALESCE(s.voice_text, ''),
		       l.platform, l.views, l.likes, l.comments, l.shares,
		       l.engagement_rate, l.avg_view_percentage, l.subscribers_gained
		FROM latest l
		JOIN clips c ON c.id = l.clip_id
		LEFT JOIN LATERAL (
			SELECT voice_text FROM scenes WHERE clip_id = c.id ORDER BY scene_number ASC LIMIT 1
		) s ON true
		WHERE c.status = 'published'
		  AND NOT EXISTS (
			SELECT 1 FROM clip_publish_status ps
			WHERE ps.clip_id = l.clip_id AND ps.platform = l.platform AND ps.status = 'failed')
		ORDER BY l.platform, l.views DESC
		LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("query recent analytics: %w", err)
	}
	defer rows.Close()

	var stats []ClipStat
	for rows.Next() {
		var s ClipStat
		if err := rows.Scan(&s.ID, &s.Title, &s.Category, &s.Hook,
			&s.Platform, &s.Views, &s.Likes, &s.Comments, &s.Shares,
			&s.EngagementRate, &s.AvgViewPct, &s.SubsGained); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate analytics: %w", err)
	}

	trends, err := a.fetchTrends(ctx)
	if err != nil {
		log.Printf("Analyzer: trend query failed (labels default to unknown): %v", err)
	}
	for i := range stats {
		if label, ok := trends[stats[i].ID+"|"+stats[i].Platform]; ok {
			stats[i].Trend = label
		} else {
			stats[i].Trend = "unknown"
		}
	}
	FillPercentiles(stats)
	return stats, nil
}

// fetchTrends derives a trend label per clip+platform from daily snapshot maxima
// over the last 8 days (the daily 04:00 fetch gives one snapshot per day).
func (a *Analyzer) fetchTrends(ctx context.Context) (map[string]string, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT clip_id, platform, DATE_TRUNC('day', fetched_at)::date AS day, MAX(views)
		FROM clip_analytics
		WHERE fetched_at >= NOW() - INTERVAL '8 days'
		  AND platform IN ('youtube', 'tiktok')
		GROUP BY clip_id, platform, day
		ORDER BY clip_id, platform, day ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	series := map[string][]int{}
	for rows.Next() {
		var clipID, platform string
		var day time.Time
		var views int
		if err := rows.Scan(&clipID, &platform, &day, &views); err != nil {
			return nil, err
		}
		key := clipID + "|" + platform
		series[key] = append(series[key], views)
	}
	out := map[string]string{}
	for key, views := range series {
		out[key] = TrendLabel(views)
	}
	return out, rows.Err()
}
```

(Add `"time"` to analyzer.go imports.)

Replace the start of `AnalyzeAndImprove` (analyzer.go:36-76) with:

```go
func (a *Analyzer) AnalyzeAndImprove(ctx context.Context) error {
	stats, err := a.gatherData(ctx)
	if err != nil {
		return fmt.Errorf("gather analytics data: %w", err)
	}

	data, clipCount := BuildAnalysisData(stats)
	// Small-sample gate: below 8 measurable clips the signal is noise.
	if clipCount < 8 {
		log.Printf("Analyzer: only %d measurable clips in window (need 8), skipping", clipCount)
		return nil
	}

	analyticsAgent, err := a.agentsRepo.GetByName(ctx, "analytics")
	if err != nil {
		return fmt.Errorf("get analytics agent config: %w", err)
	}

	userPrompt := fmt.Sprintf(`Here is the performance data from our YouTube Shorts + TikTok posts for the last 14 days (n=%d clips — a small sample; calibrate your confidence accordingly):

%s

Notes on the data:
- "P<n> within platform" is the views percentile compared to other clips on the SAME platform (P90 = top 10%%).
- "Trend: rising" means most view growth happened in the last 2 days (likely entering the recommendation feed); "peaked" means growth stopped.
- TikTok has no watch-time/retention data — judge TikTok clips by views percentile, shares, engagement, and trend.

Current agent configurations:
%s

Analyze BOTH of these dimensions:
1. STORYTELLING STYLE — openings, hooks (the "Hook" field is the clip's real first line), pacing, tone, length. Which styles earn high view percentiles and "rising" trends?
2. TOPICS — which categories and question angles earn high views/shares on each platform?

Requirements for your insights:
- Preserve content diversity: recommend leaning into winning topics for roughly HALF of future clips, never exclusively. Say this explicitly in the question agent's insights.
- Ground every recommendation in the data (cite the pattern: views percentile, shares, or trend).
- Each insight must be under 1000 characters, written in Thai.

Return JSON only:
{
  "agents": [
    {"agent_name": "question", "new_insights": "...", "reason": "..."},
    {"agent_name": "script", "new_insights": "...", "reason": "..."},
    {"agent_name": "image", "new_insights": "...", "reason": "..."}
  ]
}`, clipCount, data, a.currentPrompts(ctx))
```

(The rest of `AnalyzeAndImprove` — LLM call, validation, save-with-history — is unchanged.)

- [ ] **Step 6: Build + tests**

Run: `go build ./... && go test ./internal/analyzer/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/analyzer/
git commit -m "feat(analyzer): v2 — both platforms, real hooks, within-platform percentiles, trend labels, topic learning with diversity guard"
```

---

### Task 7: Topic performance query + weighted category pick

**Files:**
- Modify: `internal/repository/analytics.go` (append TopicPerformance)
- Create: `internal/orchestrator/topic_pick.go`
- Test: `internal/orchestrator/topic_pick_test.go`
- Modify: `internal/orchestrator/orchestrator.go:131-140` (category pick)

**Interfaces:**
- Consumes: `models.CategoryScore` (Task 2).
- Produces: `(r *AnalyticsRepo) TopicPerformance(ctx context.Context, windowDays, minClips int) ([]models.CategoryScore, error)`; `PickCategoryWeighted(categories []string, scores []models.CategoryScore, weekNum int, rng func(int) int) string`; `FormatTopicStats(scores []models.CategoryScore) string` — Task 8 and Task 9 consume these exact signatures.

- [ ] **Step 1: Write the failing tests**

`internal/orchestrator/topic_pick_test.go`:

```go
package orchestrator

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func TestPickCategoryWeighted(t *testing.T) {
	categories := []string{"account", "payment", "campaign"}
	scores := []models.CategoryScore{
		{Category: "payment", AvgPercentile: 0.8, N: 4},
		{Category: "account", AvgPercentile: 0.3, N: 5},
		{Category: "retired-category", AvgPercentile: 0.99, N: 9}, // not configured → ignored
	}

	exploit := func(int) int { return 0 }  // rng(100)=0 < 50 → exploit
	explore := func(int) int { return 99 } // rng(100)=99 ≥ 50 → round-robin

	if got := PickCategoryWeighted(categories, scores, 7, exploit); got != "payment" {
		t.Errorf("exploit pick = %q, want payment (best configured category)", got)
	}
	if got := PickCategoryWeighted(categories, scores, 7, explore); got != categories[7%3] {
		t.Errorf("explore pick = %q, want round-robin %q", got, categories[7%3])
	}
	if got := PickCategoryWeighted(categories, nil, 7, exploit); got != categories[7%3] {
		t.Errorf("no-scores pick = %q, want round-robin fallback", got)
	}
}

func TestFormatTopicStats(t *testing.T) {
	if got := FormatTopicStats(nil); got != "" {
		t.Errorf("empty scores should render empty string, got %q", got)
	}
	out := FormatTopicStats([]models.CategoryScore{
		{Category: "payment", AvgPercentile: 0.8, AvgViews: 95, N: 4},
	})
	for _, want := range []string{"payment", "95", "80", "4", "ครึ่งหนึ่ง"} {
		if !strings.Contains(out, want) {
			t.Errorf("stats missing %q:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/orchestrator/ -run 'TestPickCategoryWeighted|TestFormatTopicStats' -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement topic_pick.go**

`internal/orchestrator/topic_pick.go`:

```go
package orchestrator

import (
	"fmt"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)

// PickCategoryWeighted picks the production category. Half the time (rng(100) < 50)
// it exploits the best-performing configured category; otherwise — and whenever no
// scores are available — it keeps the legacy week-based round-robin so topic
// coverage stays diverse. Scores for categories no longer configured are ignored.
func PickCategoryWeighted(categories []string, scores []models.CategoryScore, weekNum int, rng func(int) int) string {
	fallback := categories[weekNum%len(categories)]
	if len(scores) == 0 {
		return fallback
	}
	configured := make(map[string]bool, len(categories))
	for _, c := range categories {
		configured[c] = true
	}
	best := ""
	bestPct := -1.0
	for _, s := range scores {
		if configured[s.Category] && s.AvgPercentile > bestPct {
			best, bestPct = s.Category, s.AvgPercentile
		}
	}
	if best == "" || rng(100) >= 50 {
		return fallback
	}
	return best
}

// FormatTopicStats renders category performance as a Thai prompt block for the
// question agent, or "" when there is nothing to show.
func FormatTopicStats(scores []models.CategoryScore) string {
	if len(scores) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## ผลงานหัวข้อ 30 วันล่าสุด (ยอดจริงจาก YouTube Shorts + TikTok)\n")
	for _, s := range scores {
		b.WriteString(fmt.Sprintf("- หมวด %s: ยอดวิวเฉลี่ย %.0f ต่อคลิป (percentile เฉลี่ย %.0f จาก 100, วัดจาก %d คลิป)\n",
			s.Category, s.AvgViews, s.AvgPercentile*100, s.N))
	}
	b.WriteString("\nใช้ข้อมูลนี้เป็นบริบท: เลือกประเด็น/มุมที่ใกล้เคียงหมวดผลงานดีราวครึ่งหนึ่ง ที่เหลือกระจายมุมใหม่เพื่อความหลากหลาย และห้ามซ้ำกับหัวข้อเดิมตามรายการห้ามซ้ำ")
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/orchestrator/ -v`
Expected: PASS.

- [ ] **Step 5: Add TopicPerformance to the repo** (append to `internal/repository/analytics.go`)

```go
// TopicPerformance scores each clip category by its clips' mean within-platform
// views percentile over the last windowDays, excluding failed publishes.
// Categories with fewer than minClips measurable clips are omitted.
func (r *AnalyticsRepo) TopicPerformance(ctx context.Context, windowDays, minClips int) ([]models.CategoryScore, error) {
	rows, err := r.pool.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (ca.clip_id, ca.platform)
				ca.clip_id, ca.platform, ca.views
			FROM clip_analytics ca
			WHERE ca.fetched_at >= NOW() - make_interval(days => $1)
			  AND ca.platform IN ('youtube', 'tiktok')
			ORDER BY ca.clip_id, ca.platform, ca.fetched_at DESC
		), ranked AS (
			SELECT l.clip_id, l.views,
			       PERCENT_RANK() OVER (PARTITION BY l.platform ORDER BY l.views) AS pct
			FROM latest l
			WHERE NOT EXISTS (
				SELECT 1 FROM clip_publish_status ps
				WHERE ps.clip_id = l.clip_id AND ps.platform = l.platform AND ps.status = 'failed')
		)
		SELECT c.category,
		       AVG(r.pct), AVG(r.views), COUNT(DISTINCT r.clip_id)
		FROM ranked r
		JOIN clips c ON c.id = r.clip_id
		WHERE c.status = 'published'
		GROUP BY c.category
		HAVING COUNT(DISTINCT r.clip_id) >= $2
		ORDER BY AVG(r.pct) DESC`, windowDays, minClips)
	if err != nil {
		return nil, fmt.Errorf("topic performance: %w", err)
	}
	defer rows.Close()

	out := []models.CategoryScore{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		var s models.CategoryScore
		if err := rows.Scan(&s.Category, &s.AvgPercentile, &s.AvgViews, &s.N); err != nil {
			return nil, fmt.Errorf("scan category score: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate category scores: %w", err)
	}
	return out, nil
}
```

- [ ] **Step 6: Wire the weighted pick into the orchestrator**

In `internal/orchestrator/orchestrator.go`, replace line 140 (`category := categories[weekNum%len(categories)]`) with:

```go
	category := categories[weekNum%len(categories)]
	var topicStats string
	if v, err := o.settingsRepo.Get(ctx, "topic_stats_enabled"); err != nil || v != "false" {
		// Enabled by default; only the explicit value "false" disables it (kill switch).
		if scores, err := o.analyticsRepo.TopicPerformance(ctx, 30, 3); err != nil {
			log.Printf("topic performance unavailable, using round-robin category: %v", err)
		} else {
			category = PickCategoryWeighted(categories, scores, weekNum, rand.Intn)
			topicStats = FormatTopicStats(scores)
		}
	}
```

Check the imports: `math/rand` must be imported. Check that the orchestrator struct already holds the analytics repo (it calls `PresetRetention` around line 244 — reuse the same field name found there; if that call goes through a different field, e.g. `o.analytics`, use that name instead of `o.analyticsRepo`).

`topicStats` is consumed in Task 8's call-site change (same function, ~40 lines below). If committing Task 7 standalone, silence the unused variable with `_ = topicStats` and remove it in Task 8.

- [ ] **Step 7: Build + tests**

Run: `go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/repository/analytics.go internal/orchestrator/
git commit -m "feat(topics): performance-weighted category pick (50/50 exploit/explore) behind topic_stats_enabled"
```

---

### Task 8: Topic stats into the Question agent prompt

**Files:**
- Modify: `internal/agent/question.go:49` (signature) and `:113-124` (prompt assembly)
- Modify: `internal/orchestrator/orchestrator.go:181,190` (both Generate call sites)

**Interfaces:**
- Consumes: `topicStats` string built in Task 7.
- Produces: new signature `(a *QuestionAgent) Generate(ctx context.Context, count int, category string, format *models.ContentFormat, persona string, topicStats string, cfg *models.AgentConfig) ([]GeneratedQuestion, error)`.

- [ ] **Step 1: Change the signature and append the stats block**

In `internal/agent/question.go`, change the `Generate` signature to:

```go
func (a *QuestionAgent) Generate(ctx context.Context, count int, category string, format *models.ContentFormat, persona string, topicStats string, cfg *models.AgentConfig) ([]GeneratedQuestion, error) {
```

Immediately after the `renderTemplate` call succeeds (after the `if err != nil { ... }` block at question.go:113-124), add:

```go
	// Real-performance context (empty when the topic_stats kill switch is off).
	userPrompt += topicStats
```

The dedup-retry path builds `retryPrompt := userPrompt + ...` — it inherits the stats automatically; no further change.

- [ ] **Step 2: Update both call sites**

In `internal/orchestrator/orchestrator.go` line 181:

```go
	questions, err := o.questionAgent.Generate(ctx, count, category, format, persona, topicStats, qaCfg)
```

and line 190 (the news-fallback retry):

```go
		questions, err = o.questionAgent.Generate(ctx, count, category, format, persona, topicStats, qaCfg)
```

(Remove the `_ = topicStats` placeholder if Task 7 added it.)

- [ ] **Step 3: Build + tests**

Run: `go build ./... && go test ./...`
Expected: PASS. `go build` failing on any other `Generate` caller means a call site was missed — fix it the same way (pass `""` if no stats are available there).

- [ ] **Step 4: Commit**

```bash
git add internal/agent/question.go internal/orchestrator/orchestrator.go
git commit -m "feat(topics): inject real topic performance into question agent prompt"
```

---

### Task 9: Summary API — publish failures, topic performance, sparklines

**Files:**
- Modify: `internal/repository/analytics.go` (append PublishFailures + Sparklines)
- Modify: `internal/handler/analytics.go:39-96` (Summary)

**Interfaces:**
- Produces JSON keys on `/api/v1/analytics/summary`: `publish_failures: PublishFailure[]`, `topic_performance: CategoryScore[]`; each `top_clips` row gains `sparkline: number[]` and `failed_platforms: string[]`. Task 10 consumes these exact keys.

- [ ] **Step 1: Add PublishFailures to the repo**

```go
// PublishFailures lists posts whose last-seen Zernio status is 'failed'.
func (r *AnalyticsRepo) PublishFailures(ctx context.Context) ([]models.PublishFailure, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ps.clip_id, c.title, ps.platform, ps.post_type,
		       COALESCE(ps.error_message, ''), ps.checked_at
		FROM clip_publish_status ps
		JOIN clips c ON c.id = ps.clip_id
		WHERE ps.status = 'failed'
		ORDER BY ps.checked_at DESC
		LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("publish failures: %w", err)
	}
	defer rows.Close()

	out := []models.PublishFailure{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		var f models.PublishFailure
		if err := rows.Scan(&f.ClipID, &f.Title, &f.Platform, &f.PostType, &f.ErrorMessage, &f.CheckedAt); err != nil {
			return nil, fmt.Errorf("scan publish failure: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate publish failures: %w", err)
	}
	return out, nil
}
```

- [ ] **Step 2: Add Sparklines to the repo**

```go
// Sparklines returns per-clip daily view deltas (all platforms summed, oldest→newest)
// over the last `days` days, derived from the daily snapshot maxima.
func (r *AnalyticsRepo) Sparklines(ctx context.Context, days int) (map[string][]int, error) {
	rows, err := r.pool.Query(ctx, `
		WITH per_day AS (
			SELECT clip_id, platform, post_type,
			       DATE_TRUNC('day', fetched_at)::date AS day, MAX(views) AS views
			FROM clip_analytics
			WHERE fetched_at >= NOW() - make_interval(days => $1)
			GROUP BY clip_id, platform, post_type, day
		)
		SELECT clip_id, day, SUM(views)::int
		FROM per_day
		GROUP BY clip_id, day
		ORDER BY clip_id, day ASC`, days)
	if err != nil {
		return nil, fmt.Errorf("sparklines: %w", err)
	}
	defer rows.Close()

	cumulative := map[string][]int{}
	for rows.Next() {
		var clipID string
		var day time.Time
		var views int
		if err := rows.Scan(&clipID, &day, &views); err != nil {
			return nil, fmt.Errorf("scan sparkline row: %w", err)
		}
		cumulative[clipID] = append(cumulative[clipID], views)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sparkline rows: %w", err)
	}

	// Convert cumulative snapshots to daily deltas (clamped at 0 — platform
	// corrections can shrink counts and a negative bar is meaningless).
	out := map[string][]int{}
	for clipID, series := range cumulative {
		deltas := make([]int, 0, len(series))
		for i := 1; i < len(series); i++ {
			d := series[i] - series[i-1]
			if d < 0 {
				d = 0
			}
			deltas = append(deltas, d)
		}
		out[clipID] = deltas
	}
	return out, nil
}
```

- [ ] **Step 3: Extend the Summary handler**

In `internal/handler/analytics.go` `Summary`, after the `lastFetched` line (analytics.go:82), add — all fail-open so a bad new query never blanks the whole page:

```go
	failures, err := h.repo.PublishFailures(ctx)
	if err != nil {
		log.Printf("analytics summary: publish failures unavailable: %v", err)
		failures = []models.PublishFailure{}
	}
	topics, err := h.repo.TopicPerformance(ctx, 30, 3)
	if err != nil {
		log.Printf("analytics summary: topic performance unavailable: %v", err)
		topics = []models.CategoryScore{}
	}
	sparks, err := h.repo.Sparklines(ctx, 14)
	if err != nil {
		log.Printf("analytics summary: sparklines unavailable: %v", err)
		sparks = map[string][]int{}
	}

	failedByClip := map[string][]string{}
	for _, f := range failures {
		failedByClip[f.ClipID] = append(failedByClip[f.ClipID], f.Platform)
	}
	for i := range topClips {
		if s, ok := sparks[topClips[i].ClipID]; ok {
			topClips[i].Sparkline = s
		} else {
			topClips[i].Sparkline = []int{}
		}
		if fp, ok := failedByClip[topClips[i].ClipID]; ok {
			topClips[i].FailedPlatforms = fp
		} else {
			topClips[i].FailedPlatforms = []string{}
		}
	}
```

And add two keys to the response map:

```go
		"publish_failures":  failures,
		"topic_performance": topics,
```

- [ ] **Step 4: Build + tests**

Run: `go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/analytics.go internal/handler/analytics.go
git commit -m "feat(analytics): summary API exposes publish failures, topic performance, per-clip sparklines"
```

---

### Task 10: Frontend — alert, sparkline, topic section, platform card extras

**Files:**
- Create: `frontend/src/components/analytics/failed-posts-alert.tsx`
- Create: `frontend/src/components/analytics/sparkline.tsx`
- Create: `frontend/src/components/analytics/topic-performance.tsx`
- Modify: `frontend/src/components/analytics/platform-card.tsx`
- Modify: `frontend/src/components/analytics/top-clips-table.tsx`
- Modify: `frontend/src/pages/Analytics.tsx`

**Interfaces:**
- Consumes: JSON keys from Task 9 (`publish_failures`, `topic_performance`, `sparkline`, `failed_platforms`, `engagement_rate`, `subscribers_gained`).

- [ ] **Step 1: Sparkline component**

`frontend/src/components/analytics/sparkline.tsx`:

```tsx
// เส้นแนวโน้มจิ๋ว: ยอดวิวที่เพิ่มขึ้นต่อวัน (จาก snapshot รายวัน)
export function Sparkline({ points }: { points: number[] }) {
  if (!points || points.length < 2) return null
  const w = 72
  const h = 20
  const max = Math.max(...points, 1)
  const step = w / (points.length - 1)
  const d = points
    .map((v, i) => `${i === 0 ? 'M' : 'L'}${(i * step).toFixed(1)},${(h - 1 - (v / max) * (h - 2)).toFixed(1)}`)
    .join(' ')
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} className="text-primary shrink-0" aria-hidden>
      <path d={d} fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
    </svg>
  )
}
```

- [ ] **Step 2: FailedPostsAlert component**

`frontend/src/components/analytics/failed-posts-alert.tsx`:

```tsx
import { AlertTriangle } from 'lucide-react'
import { Card, CardContent } from '../ui/card'

export interface PublishFailure {
  clip_id: string
  title: string
  platform: string
  post_type: string
  error_message: string
  checked_at: string
}

const PLATFORM_LABEL: Record<string, string> = {
  youtube: 'YouTube',
  tiktok: 'TikTok',
}

// แปลง error จาก Zernio เป็นภาษาคน — เคสที่รู้จักแปลไทย ที่เหลือโชว์ข้อความดิบ
function reasonThai(msg: string): string {
  if (!msg) return 'ไม่ทราบสาเหตุ'
  if (msg.toLowerCase().includes('could not download')) {
    return 'ไฟล์วิดีโอหมดอายุก่อนแพลตฟอร์มดึงไปโพสต์'
  }
  return msg
}

export function FailedPostsAlert({ failures }: { failures: PublishFailure[] }) {
  if (!failures.length) return null
  return (
    <Card className="border-amber-300 bg-amber-50 dark:bg-amber-950/20">
      <CardContent className="pt-4">
        <div className="mb-2 flex items-center gap-2 text-sm font-semibold text-amber-700 dark:text-amber-400">
          <AlertTriangle className="size-4" aria-hidden />
          โพสต์ไม่สำเร็จ {failures.length} รายการ — คลิปเหล่านี้ไม่มียอดและถูกกันออกจากข้อมูลที่ AI ใช้เรียนรู้
        </div>
        <ul className="space-y-1.5">
          {failures.map(f => (
            <li key={`${f.clip_id}-${f.platform}-${f.post_type}`} className="text-sm">
              <span className="font-medium">{f.title}</span>
              <span className="text-muted-foreground"> · {PLATFORM_LABEL[f.platform] ?? f.platform} — {reasonThai(f.error_message)}</span>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 3: TopicPerformance component**

`frontend/src/components/analytics/topic-performance.tsx`:

```tsx
import { Card, CardContent } from '../ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table'
import { MetricTooltip } from './metric-tooltip'
import { formatNum } from '../../lib/format'

export interface CategoryScore {
  category: string
  avg_percentile: number
  avg_views: number
  n: number
}

export function TopicPerformance({ scores }: { scores: CategoryScore[] }) {
  return (
    <div>
      <div className="mb-2 flex items-center gap-1.5">
        <h2 className="text-sm font-semibold">หัวข้อไหนทำยอดดี</h2>
        <MetricTooltip text="อันดับหมวดหัวข้อตามยอดวิวจริง 30 วันล่าสุด — ข้อมูลชุดเดียวกับที่ AI ใช้เลือกหัวข้อคลิปถัดไป (แสดงเฉพาะหมวดที่มีอย่างน้อย 3 คลิป)" />
      </div>
      <Card>
        <CardContent className="pt-4">
          {scores.length === 0 ? (
            <p className="text-sm text-muted-foreground py-2">
              ยังมีข้อมูลไม่พอ — ต้องมีอย่างน้อย 3 คลิปต่อหมวดจึงจะจัดอันดับได้
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>หมวดหัวข้อ</TableHead>
                  <TableHead>ยอดวิวเฉลี่ย/คลิป</TableHead>
                  <TableHead>คะแนนเทียบคลิปอื่น</TableHead>
                  <TableHead>จำนวนคลิป</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {scores.map(s => (
                  <TableRow key={s.category}>
                    <TableCell className="font-medium">{s.category}</TableCell>
                    <TableCell>{formatNum(Math.round(s.avg_views))}</TableCell>
                    <TableCell>{Math.round(s.avg_percentile * 100)} / 100</TableCell>
                    <TableCell>{s.n}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
```

- [ ] **Step 4: PlatformCard — engagement + subscribers**

In `frontend/src/components/analytics/platform-card.tsx`, extend the `PlatformTotals` interface:

```tsx
  engagement_rate: number
  subscribers_gained: number
```

Then in the component, replace the retention line block (lines 56-60) with:

```tsx
              {showRetention && (
                <div className="text-xs text-muted-foreground">
                  ดูจบเฉลี่ย {Math.min(data.avg_retention_rate * 100, 100).toFixed(0)}%
                  {data.avg_retention_rate > 1 && ' (ดูวนซ้ำ)'}
                </div>
              )}
              {data.engagement_rate > 0 && (
                <div className="text-xs text-muted-foreground">มีส่วนร่วม {data.engagement_rate.toFixed(2)}%</div>
              )}
              {data.subscribers_gained > 0 && (
                <div className="text-xs text-muted-foreground">ผู้ติดตามใหม่ +{data.subscribers_gained}</div>
              )}
```

- [ ] **Step 5: Top clips table — sparkline + failure badge**

In `frontend/src/components/analytics/top-clips-table.tsx`:
1. Read the file first; locate the exported `ClipRow` interface and add:

```tsx
  sparkline?: number[] | null
  failed_platforms?: string[] | null
```

2. Import the sparkline at the top:

```tsx
import { Sparkline } from './sparkline'
```

3. In the desktop table's views cell (the cell rendering the clip's view count), wrap the number and add the sparkline beside it:

```tsx
  <span className="inline-flex items-center gap-2">
    {formatNum(clip.views)}
    <Sparkline points={clip.sparkline ?? []} />
  </span>
```

4. In the title cell (both desktop row and mobile card variants), after the title text add:

```tsx
  {(clip.failed_platforms?.length ?? 0) > 0 && (
    <span className="ml-1.5 inline-flex items-center rounded bg-red-100 px-1.5 py-0.5 text-[10px] font-medium text-red-700 dark:bg-red-950 dark:text-red-400">
      โพสต์ไม่สำเร็จ: {clip.failed_platforms!.join(', ')}
    </span>
  )}
```

Match the file's existing JSX structure — the snippets slot into existing cells; do not restructure the table.

- [ ] **Step 6: Analytics.tsx — wire the new sections**

In `frontend/src/pages/Analytics.tsx`:

1. Add imports:

```tsx
import { FailedPostsAlert, type PublishFailure } from '../components/analytics/failed-posts-alert'
import { TopicPerformance, type CategoryScore } from '../components/analytics/topic-performance'
```

2. Extend the local `PlatformTotals` interface (lines 61-69) with:

```tsx
  engagement_rate: number
  subscribers_gained: number
```

3. Extend `SummaryResponse` (lines 71-80) with:

```tsx
  publish_failures: PublishFailure[] | null
  topic_performance: CategoryScore[] | null
```

4. Inside the main content `<div className="space-y-6">` (line 182), insert as the FIRST child (above the hero cards):

```tsx
          <FailedPostsAlert failures={data?.publish_failures ?? []} />
```

5. Between the `SegmentCompare` block and the "คลิปที่ดีที่สุด" block, insert:

```tsx
          {/* หัวข้อไหนทำยอดดี */}
          <TopicPerformance scores={data?.topic_performance ?? []} />
```

- [ ] **Step 7: Build the frontend**

Run: `cd frontend && npm run build`
Expected: build succeeds with no TypeScript errors.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/
git commit -m "feat(analytics-ui): failed-posts alert, per-clip sparkline, topic performance section, richer platform cards"
```

---

### Task 11: Settings allowlist, full verification, merge prep

**Files:**
- Modify: `internal/handler/settings.go:55-67` (allowed map)

- [ ] **Step 1: Allow the kill switch to be edited from the UI**

In the `allowed` map in `internal/handler/settings.go` `Update`, add:

```go
		"topic_stats_enabled":       true,
```

- [ ] **Step 2: Full verification**

```bash
go build ./... && go test ./... && (cd frontend && npm run build)
```

Expected: everything passes.

- [ ] **Step 3: Commit + push branch**

```bash
git add internal/handler/settings.go
git commit -m "feat(settings): allow topic_stats_enabled kill switch from UI"
git push -u origin feat/analytics-viral-feedback-loop
```

- [ ] **Step 4: Run /simplify on the branch diff** (user's standing preference: simplify before the commit/merge step)

Review the full branch diff for accidental complexity; apply and commit any simplifications.

- [ ] **Step 5: Open PR to master**

```bash
gh pr create --title "Analytics viral feedback loop: full Zernio capture + failed-publish detection + topic learning" --body "Implements docs/superpowers/specs/2026-07-04-analytics-viral-feedback-loop-design.md"
```

---

### Task 12: Post-deploy prod verification (after merge; both Railway services auto-deploy from master)

No code. Verification checklist against prod (Neon project `snowy-grass-75448787`):

- [ ] **Step 1: Confirm migration applied**

```sql
SELECT column_name FROM information_schema.columns WHERE table_name='clip_analytics' AND column_name IN ('engagement_rate','avg_view_percentage','subscribers_gained');
SELECT COUNT(*) FROM clip_publish_status;  -- table exists (0 rows OK before first fetch)
SELECT value FROM settings WHERE key='topic_stats_enabled';  -- 'true'
```

- [ ] **Step 2: Trigger a manual fetch and verify data lands**

`POST /api/v1/analytics/fetch`, wait ~2 minutes, then:

```sql
SELECT platform, engagement_rate, avg_view_percentage, subscribers_gained FROM clip_analytics ORDER BY fetched_at DESC LIMIT 6;
SELECT COUNT(*) FROM clip_analytics_daily;              -- > 0 (YouTube rows)
SELECT status, COUNT(*) FROM clip_publish_status GROUP BY status;  -- expect some 'failed' TikTok rows
```

- [ ] **Step 3: Eyeball the Analytics page**

Failed-posts alert visible (there is at least one known failed TikTok post), sparklines render in the top-clips table, "หัวข้อไหนทำยอดดี" section shows categories or the not-enough-data message, TikTok card shows "มีส่วนร่วม X.XX%".

- [ ] **Step 4: Kill-switch smoke test**

Set `topic_stats_enabled=false` in settings via UI, confirm the next production run logs the round-robin category (no `TopicPerformance` errors), then set back to `true`.

---

## Self-Review Notes (already applied)

- Spec §1a–1c → Tasks 2–4; §2a–2c → Task 6; §2d → Tasks 7–8 (50/50 rule implemented in Go at the orchestrator's category pick — stronger than prompt-only, since the category is chosen by code, not by the LLM); §2e → Task 6 gate + Task 7 `minClips`; §3 UI → Tasks 9–10; error handling → fail-open paths in Tasks 4, 6, 9; rollback → kill switch (Task 7/11), additive migration (Task 2).
- Type consistency: `topicStats` produced in Task 7 Step 6 and consumed in Task 8; `Sparkline`/`FailedPlatforms` fields defined in Task 2 and used in Tasks 9–10; `ytDetail` defined and used only within Task 4.
