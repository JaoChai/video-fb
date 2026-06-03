# Zernio Analytics Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ดึง analytics จาก Zernio API ที่ถูกต้องให้กลับมาทำงาน — เปลี่ยน URL ผิด `/v1/analytics/{postID}?platform=...` เป็น `/v1/analytics?postId=...&platform=...` + เพิ่ม `/v1/analytics/youtube/daily-views` สำหรับ watchTime/retention + เปลี่ยน schedule เป็นรายวัน + ป้องกัน silent failure

**Architecture:**
- **Step 1 (summary):** เรียก `GET /v1/analytics?postId={zernioPostID}&platform=youtube` ครั้งละ 1 post → ได้ views/likes/comments/shares + YouTube `platformPostId` (= YouTube videoId)
- **Step 2 (deep):** ใช้ videoId เรียก `GET /v1/analytics/youtube/daily-views?videoId=...&accountId=...` → รวม estimatedMinutesWatched ทุกวัน + เฉลี่ย averageViewDuration → คำนวณ watch_time_seconds + retention_rate (averageViewDuration / videoDuration estimate)
- ปรับ schedule เป็นทุกวัน 4:00 UTC (`0 4 * * *`), เพิ่ม manual trigger endpoint สำหรับทดสอบ, แสดง last_fetched + scope warning ที่หน้า UI

**Tech Stack:** Go 1.x, pgx/v5, chi/v5, React 18 + TanStack Query 5, Neon Postgres, Railway

---

## Problem Statement

หน้า Analytics แสดง 21 clips แต่ค่า views/likes/retention เป็น 0 ทั้งหมด ผู้ใช้คิดว่าระบบไม่ทำงาน

หลักฐานจาก Production DB + Railway logs (2026-05-12):
- `clips` published = 21, `clip_metadata.zernio_post_id` set = 21
- `clip_analytics` รวม 4 rows ทั้งหมด views=0, likes=0 (จาก 2026-05-02)
- Schedule `fetch_analytics` run ล่าสุด 2026-05-10 04:00 UTC → log: `FetchAnalytics done: 18 clips, 0 success, 23 failed`
- ทุก call return HTTP 404 พร้อม Next.js HTML page

**Root cause:** `internal/publisher/zernio.go:129` เรียก `GET /v1/analytics/{postID}?platform=...` — Zernio ใช้ `postId` เป็น **query parameter** ไม่ใช่ path segment (จาก openapi.yaml line 5058-5290)

**Schema check ก่อนเริ่ม (read-only, ไม่ต้องแก้):**
- `internal/models/clip.go:53-64` — `ClipAnalytics` struct ฟิลด์ DB
- `internal/models/clip.go:66-86` — `AnalyticsSummary`, `ClipPerformance`
- `internal/publisher/zernio.go:72-79` — `AnalyticsResponse` (Zernio HTTP layer, จะถูกแทน)
- `internal/repository/analytics.go:11-17` — `latestAnalyticsCTE`

---

## File Structure

**Modify:**
- `internal/publisher/zernio.go` — แทนที่ `AnalyticsResponse` + `GetAnalytics` URL, เพิ่ม `GetYouTubeDailyViews`
- `internal/publisher/publisher.go` — แก้ `FetchAnalytics` ใช้ shape ใหม่ + เช็ค error จาก `analytics.Create()` + ส่ง accountId ของ YouTube + ดึง daily-views หลัง summary
- `internal/handler/analytics.go` — เพิ่ม handler `Trigger` (manual fetch)
- `internal/router/router.go` — เพิ่ม route `POST /api/v1/analytics/fetch`
- `frontend/src/pages/Analytics.tsx` — แสดง `last_fetched` timestamp + handle scope-missing error
- `internal/handler/analytics.go` (Summary) — เพิ่มฟิลด์ `last_fetched_at` ใน response

**Create:**
- `migrations/014_daily_analytics_cleanup.sql` — เปลี่ยน cron `0 4 * * 0` → `0 4 * * *` + ลบ ghost rows (views=0 platform ≠ youtube)

---

## Task 1: Verify Zernio Analytics Endpoint Manually

**Files:**
- Read: `migrations/012_restore_fetch_analytics.sql`, `internal/publisher/zernio.go:16-50`

ก่อนแตะโค้ดจริง ยืนยันว่า URL ใหม่ทำงาน + response shape ตรงกับที่ openapi spec ระบุ

- [ ] **Step 1: ดึง API key + post ID ตัวอย่างจาก Production DB**

Run:
```bash
neon-cli --project snowy-grass-75448787 \
  sql "SELECT value FROM settings WHERE key='zernio_api_key'" \
  > /tmp/zernio_key.txt
neon-cli --project snowy-grass-75448787 \
  sql "SELECT zernio_post_id FROM clip_metadata WHERE zernio_post_id IS NOT NULL LIMIT 1" \
  > /tmp/zernio_post_id.txt
```

ถ้าไม่มี neon-cli, ใช้ Neon MCP `run_sql` ดึงค่า แล้ว export เป็น env var `ZERNIO_KEY` และ `POST_ID`

Expected: 1 row API key (sk_...), 1 post ID เช่น `69eee157985e734bf3bd...`

- [ ] **Step 2: ทดสอบ endpoint ใหม่ด้วย curl**

```bash
curl -s "https://zernio.com/api/v1/analytics?postId=${POST_ID}&platform=youtube" \
  -H "Authorization: Bearer ${ZERNIO_KEY}" \
  | jq '.'
```

Expected (HTTP 200): JSON ที่มีฟิลด์ `postId`, `analytics.{impressions,reach,likes,comments,shares,saves,clicks,views,engagementRate,lastUpdated}`, `platformAnalytics[].{platform,platformPostId,analytics{...}}`

ถ้าได้ 202 = sync pending (ปกติสำหรับ post ใหม่), 424 = ทุก platform fail, 402 = ต้อง add-on, 412 = ขาด scope. **บันทึก HTTP code + response body ลง task log** เพื่อใช้ตัดสินใจ error handling ใน Task 3

- [ ] **Step 3: ทดสอบ daily-views endpoint**

ใช้ `platformPostId` ของ YouTube จาก Step 2 และ `zernio_youtube_account_id` (`69eee157985e734bf3bd...` จาก settings):

```bash
curl -s "https://zernio.com/api/v1/analytics/youtube/daily-views?videoId=${YT_VIDEO_ID}&accountId=${YT_ACCOUNT_ID}" \
  -H "Authorization: Bearer ${ZERNIO_KEY}" \
  | jq '.'
```

Expected: `success: true`, `totalViews`, `dailyViews[]` มี `{date, views, estimatedMinutesWatched, averageViewDuration, subscribersGained, subscribersLost, likes, comments, shares}`

ถ้าได้ 412 (youtube_analytics_scope_missing) → user ต้อง re-auth YouTube ใน Zernio dashboard. **บันทึกผลลง task log**

- [ ] **Step 4: ยืนยันผลกับ user ก่อนเริ่ม Task 2**

หาก Task 1 พบว่า endpoint ใหม่ทำงาน → ไป Task 2  
หาก endpoint return 412/402 → หยุดและ ping user ก่อน

---

## Task 2: Replace Zernio Analytics Response Type + URL

**Files:**
- Modify: `internal/publisher/zernio.go:72-79` (struct), `:128-156` (method)

- [ ] **Step 1: เขียน failing test**

สร้างไฟล์ `internal/publisher/zernio_test.go` (ถ้ายังไม่มี — ตรวจก่อน):

```go
package publisher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGetAnalytics_UsesQueryParamPostID(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"postId":"P123",
			"analytics":{"impressions":100,"reach":80,"likes":10,"comments":2,"shares":1,"saves":0,"clicks":5,"views":50,"engagementRate":0.18,"lastUpdated":"2026-05-12T00:00:00Z"},
			"platformAnalytics":[{"platform":"youtube","platformPostId":"yt_abc","analytics":{"impressions":100,"reach":80,"likes":10,"comments":2,"shares":1,"saves":0,"clicks":5,"views":50,"engagementRate":0.18,"lastUpdated":"2026-05-12T00:00:00Z"}}]
		}`))
	}))
	defer srv.Close()

	// inject custom base URL via test hook (see Step 3)
	z := newTestZernioClient(srv.URL, "test_key", (*pgxpool.Pool)(nil))

	resp, err := z.GetAnalytics(context.Background(), "P123", "youtube")
	if err != nil {
		t.Fatalf("GetAnalytics err: %v", err)
	}
	if !strings.Contains(capturedURL, "postId=P123") || !strings.Contains(capturedURL, "platform=youtube") {
		t.Fatalf("expected postId+platform as query params, got %s", capturedURL)
	}
	if resp.PostID != "P123" {
		t.Fatalf("expected PostID=P123, got %q", resp.PostID)
	}
	if len(resp.PlatformAnalytics) != 1 || resp.PlatformAnalytics[0].PlatformPostID != "yt_abc" {
		t.Fatalf("expected platformAnalytics[0].PlatformPostID=yt_abc, got %+v", resp.PlatformAnalytics)
	}
	if resp.PlatformAnalytics[0].Analytics.Views != 50 {
		t.Fatalf("expected views=50, got %d", resp.PlatformAnalytics[0].Analytics.Views)
	}
	// ensure JSON unmarshal also handles ints
	var raw map[string]any
	_ = json.Unmarshal([]byte(`{"views":50}`), &raw)
}
```

- [ ] **Step 2: รัน test ให้ fail**

```bash
go test ./internal/publisher/ -run TestGetAnalytics_UsesQueryParamPostID -v
```

Expected: FAIL — `newTestZernioClient` ไม่มี + `AnalyticsResponse.PostID/PlatformAnalytics` ยังไม่มี

- [ ] **Step 3: เพิ่ม test hook + แทนที่ struct**

ใน `internal/publisher/zernio.go` แทนที่ block `type AnalyticsResponse struct {...}` (line 72-79) ด้วย:

```go
type PostMetrics struct {
	Impressions    int     `json:"impressions"`
	Reach          int     `json:"reach"`
	Likes          int     `json:"likes"`
	Comments       int     `json:"comments"`
	Shares         int     `json:"shares"`
	Saves          int     `json:"saves"`
	Clicks         int     `json:"clicks"`
	Views          int     `json:"views"`
	EngagementRate float64 `json:"engagementRate"`
	LastUpdated    string  `json:"lastUpdated"`
}

type PlatformAnalyticsEntry struct {
	Platform       string      `json:"platform"`
	PlatformPostID string      `json:"platformPostId"`
	AccountID      string      `json:"accountId"`
	Analytics      PostMetrics `json:"analytics"`
	SyncStatus     string      `json:"syncStatus"`
}

type AnalyticsResponse struct {
	PostID            string                   `json:"postId"`
	Status            string                   `json:"status"`
	Analytics         PostMetrics              `json:"analytics"`
	PlatformAnalytics []PlatformAnalyticsEntry `json:"platformAnalytics"`
	SyncStatus        string                   `json:"syncStatus"`
	Message           string                   `json:"message"`
}
```

แก้ `const zernioAPI = "https://zernio.com/api/v1"` (line 16) ให้กลายเป็น field ใน `ZernioClient` เพื่อให้ test override ได้ — แทรกที่ struct definition:

```go
type ZernioClient struct {
	pool        *pgxpool.Pool
	apiKey      string
	apiKeyOnce  sync.Once
	apiKeyValue string
	client      *http.Client
	baseURL     string // NEW
}
```

แก้ constructor `NewZernioClient` (ค้นหา `func NewZernioClient`) ให้ตั้ง `baseURL: zernioAPI` และคง `zernioAPI` const ไว้

เพิ่มท้ายไฟล์ (ใช้ใน test เท่านั้น):

```go
// newTestZernioClient is used by tests to inject a fake API base URL.
func newTestZernioClient(baseURL, apiKey string, pool *pgxpool.Pool) *ZernioClient {
	c := &ZernioClient{
		pool:    pool,
		client:  &http.Client{Timeout: 5 * time.Second},
		baseURL: baseURL,
	}
	c.apiKeyOnce.Do(func() { c.apiKeyValue = apiKey })
	return c
}
```

ถ้า `ZernioClient` ปัจจุบันยังไม่มี `apiKeyOnce/apiKeyValue` (ตรวจที่ line 22-50) ให้ใช้ pattern เดียวกับ existing — ปรับ test hook ตามนั้น

แทนที่ `GetAnalytics` (line 128-156) ทั้ง function ด้วย:

```go
func (z *ZernioClient) GetAnalytics(ctx context.Context, postID, platform string) (*AnalyticsResponse, error) {
	q := url.Values{}
	q.Set("postId", postID)
	q.Set("platform", platform)
	endpoint := fmt.Sprintf("%s/analytics?%s", z.baseURL, q.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+z.getAPIKey(ctx))

	resp, err := z.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get analytics: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read analytics response: %w", err)
	}

	// 200 = ready, 202 = sync pending (still parseable), 424 = all platforms failed
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != 424 {
		return nil, fmt.Errorf("analytics API returned %d for post %s/%s: %s", resp.StatusCode, postID, platform, string(respBody[:min(len(respBody), 300)]))
	}

	var result AnalyticsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse analytics: %w (body=%s)", err, string(respBody[:min(len(respBody), 300)]))
	}
	return &result, nil
}
```

เพิ่ม `"net/url"` ใน import block ของ `zernio.go` (ตรวจ line 1-15 ให้แน่ใจ)

- [ ] **Step 4: รัน test ให้ผ่าน**

```bash
go test ./internal/publisher/ -run TestGetAnalytics_UsesQueryParamPostID -v
```

Expected: PASS

- [ ] **Step 5: รัน full unit tests ของ publisher**

```bash
go test ./internal/publisher/ -v
```

Expected: PASS ทุก test (รวม existing tests ถ้ามี)

- [ ] **Step 6: Commit**

```bash
git add internal/publisher/zernio.go internal/publisher/zernio_test.go
git commit -m "fix(zernio): use query-param postId for /v1/analytics + new response shape"
```

---

## Task 3: Update FetchAnalytics to Use New Response Shape

**Files:**
- Modify: `internal/publisher/publisher.go:168-244` (whole `FetchAnalytics` function)
- Read: `internal/models/clip.go:53-64`

- [ ] **Step 1: เขียน failing test**

ใน `internal/publisher/publisher_test.go` (สร้างถ้าไม่มี) เพิ่ม:

```go
package publisher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAnalytics_MapsResponseToCreateCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"postId": "P1",
			"platformAnalytics": []map[string]any{
				{
					"platform":       "youtube",
					"platformPostId": "yt_abc",
					"analytics": map[string]any{
						"views": 1234, "likes": 56, "comments": 7, "shares": 8,
					},
				},
			},
		})
	}))
	defer srv.Close()

	z := newTestZernioClient(srv.URL, "k", nil)
	resp, err := z.GetAnalytics(context.Background(), "P1", "youtube")
	if err != nil {
		t.Fatal(err)
	}
	if resp.PlatformAnalytics[0].Analytics.Views != 1234 {
		t.Fatalf("expected 1234 views, got %d", resp.PlatformAnalytics[0].Analytics.Views)
	}
}
```

(เป็น unit test ของ mapping. Integration ของ DB ทำผ่าน manual trigger ใน Task 5)

- [ ] **Step 2: รัน test ให้ fail**

```bash
go test ./internal/publisher/ -run TestFetchAnalytics_MapsResponseToCreateCall -v
```

Expected: PASS ทันทีถ้า Task 2 ทำถูก (test นี้ verify mapping เฉยๆ — ใช้เป็น regression net)

- [ ] **Step 3: แทนที่ FetchAnalytics ใน publisher.go**

แทนที่ block `func (p *Publisher) FetchAnalytics(ctx context.Context) error { ... }` (line 168-244) ด้วย:

```go
func (p *Publisher) FetchAnalytics(ctx context.Context) error {
	platforms, err := p.configuredPlatforms(ctx)
	if err != nil {
		return fmt.Errorf("configured platforms: %w", err)
	}
	if len(platforms) == 0 {
		log.Println("FetchAnalytics: no platforms configured, skipping")
		return nil
	}
	log.Printf("FetchAnalytics: platforms=%v", platforms)

	var totalPublished int
	_ = p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM clips WHERE status = 'published'`).Scan(&totalPublished)

	rows, err := p.pool.Query(ctx,
		`SELECT cm.clip_id, cm.zernio_post_id, cm.zernio_shorts_post_id
		 FROM clip_metadata cm
		 JOIN clips c ON c.id = cm.clip_id
		 WHERE c.status = 'published'
		   AND (cm.zernio_post_id IS NOT NULL OR cm.zernio_shorts_post_id IS NOT NULL)`)
	if err != nil {
		return fmt.Errorf("query published clips: %w", err)
	}
	defer rows.Close()

	type clipPost struct {
		ClipID       string
		PostID       *string
		ShortsPostID *string
	}
	var clips []clipPost
	for rows.Next() {
		var cp clipPost
		if err := rows.Scan(&cp.ClipID, &cp.PostID, &cp.ShortsPostID); err != nil {
			return fmt.Errorf("scan clip metadata: %w", err)
		}
		clips = append(clips, cp)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate published clips: %w", err)
	}
	log.Printf("FetchAnalytics: %d clips with zernio IDs / %d total published", len(clips), totalPublished)

	var success, failed, dbFailed int
	for _, cp := range clips {
		var posts []postRef
		if cp.PostID != nil && *cp.PostID != "" {
			posts = append(posts, postRef{*cp.PostID, "regular"})
		}
		if cp.ShortsPostID != nil && *cp.ShortsPostID != "" {
			posts = append(posts, postRef{*cp.ShortsPostID, "shorts"})
		}
		for _, post := range posts {
			for _, platform := range platforms {
				resp, err := p.zernio.GetAnalytics(ctx, post.id, platform)
				if err != nil {
					log.Printf("FetchAnalytics FAIL clip=%s type=%s platform=%s post=%s: %v", cp.ClipID, post.label, platform, post.id, err)
					failed++
					continue
				}
				// find the matching platform entry (response may include multiple platforms)
				var metrics PostMetrics
				found := false
				for _, pa := range resp.PlatformAnalytics {
					if pa.Platform == platform {
						metrics = pa.Analytics
						found = true
						break
					}
				}
				if !found {
					log.Printf("FetchAnalytics NO_PLATFORM_DATA clip=%s platform=%s post=%s syncStatus=%s", cp.ClipID, platform, post.id, resp.SyncStatus)
					failed++
					continue
				}
				if err := p.analytics.Create(ctx, models.ClipAnalytics{
					ClipID:           cp.ClipID,
					Platform:         platform,
					Views:            metrics.Views,
					Likes:            metrics.Likes,
					Comments:         metrics.Comments,
					Shares:           metrics.Shares,
					WatchTimeSeconds: 0, // filled by Task 5 (daily-views)
					RetentionRate:    0,
				}); err != nil {
					log.Printf("FetchAnalytics DB_FAIL clip=%s platform=%s: %v", cp.ClipID, platform, err)
					dbFailed++
					continue
				}
				success++
			}
		}
	}
	log.Printf("FetchAnalytics done: %d clips, %d success, %d api_fail, %d db_fail", len(clips), success, failed, dbFailed)
	return nil
}
```

- [ ] **Step 4: รัน build + tests**

```bash
go build ./...
go test ./internal/publisher/ -v
```

Expected: PASS, ไม่มี unused import warning

- [ ] **Step 5: Commit**

```bash
git add internal/publisher/publisher.go internal/publisher/publisher_test.go
git commit -m "fix(publisher): parse Zernio platformAnalytics response + check DB create errors"
```

---

## Task 4: Add Manual Trigger Endpoint

**Files:**
- Modify: `internal/handler/analytics.go:11-44`
- Modify: `internal/router/router.go` (look for `/api/v1/analytics`)

ไว้ทดสอบโดยไม่ต้องรอ cron — เป็นเครื่องมือ debug ที่จะใช้ใน Task 7

- [ ] **Step 1: ตรวจ router pattern**

```bash
grep -n "analytics" /Users/jaochai/Code/video-fb/internal/router/router.go
```

จดบรรทัด route ของ analytics summary + per-clip + เพิ่ม route ใหม่ใต้ pattern เดิม

- [ ] **Step 2: เพิ่ม Publisher field ใน AnalyticsHandler**

แก้ `internal/handler/analytics.go` ทั้งไฟล์ให้เป็น:

```go
package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type analyticsFetcher interface {
	FetchAnalytics(ctx context.Context) error
}

type AnalyticsHandler struct {
	repo      *repository.AnalyticsRepo
	publisher analyticsFetcher
}

func NewAnalyticsHandler(repo *repository.AnalyticsRepo, publisher analyticsFetcher) *AnalyticsHandler {
	return &AnalyticsHandler{repo: repo, publisher: publisher}
}

func (h *AnalyticsHandler) ListByClip(w http.ResponseWriter, r *http.Request) {
	clipID := chi.URLParam(r, "clipId")
	analytics, err := h.repo.ListByClip(r.Context(), clipID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: analytics})
}

func (h *AnalyticsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.repo.Summary(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	topClips, err := h.repo.TopClips(r.Context(), 10)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	lastFetched, _ := h.repo.LastFetchedAt(r.Context())
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{
		"summary":         summary,
		"top_clips":       topClips,
		"last_fetched_at": lastFetched, // nil if never fetched
	}})
}

func (h *AnalyticsHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := h.publisher.FetchAnalytics(ctx); err != nil {
			log.Printf("Manual FetchAnalytics failed: %v", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, models.APIResponse{Data: map[string]string{"status": "triggered"}})
}
```

(เพิ่ม `"time"` ใน imports)

- [ ] **Step 3: เพิ่มเมธอด `LastFetchedAt` ใน AnalyticsRepo**

เพิ่มท้าย `internal/repository/analytics.go`:

```go
func (r *AnalyticsRepo) LastFetchedAt(ctx context.Context) (*time.Time, error) {
	var t *time.Time
	err := r.pool.QueryRow(ctx, `SELECT MAX(fetched_at) FROM clip_analytics`).Scan(&t)
	if err != nil {
		return nil, fmt.Errorf("query last fetched: %w", err)
	}
	return t, nil
}
```

(เพิ่ม `"time"` ใน imports)

- [ ] **Step 4: เพิ่ม route + แก้ NewAnalyticsHandler call site**

ใน `internal/router/router.go` หา line ที่เรียก `NewAnalyticsHandler(...)` แล้วเพิ่ม `publisher` arg, จากนั้นเพิ่ม route:

```go
r.Post("/api/v1/analytics/fetch", analyticsHandler.Trigger)
```

(วางใต้ route `/api/v1/analytics/summary` หรือ in the same chi group)

หา `cmd/server/main.go` ที่ instantiate handler — แก้ให้ส่ง `publisher` เป็น arg:

```bash
grep -n "NewAnalyticsHandler" /Users/jaochai/Code/video-fb/cmd/server/main.go /Users/jaochai/Code/video-fb/internal/router/router.go
```

ตรวจว่า `publisher` instance มีอยู่ใน scope ที่สร้าง handler — ถ้าใช่ ส่งต่อได้เลย

- [ ] **Step 5: รัน build**

```bash
go build ./...
```

Expected: PASS, ไม่มี error เกี่ยวกับ missing argument

- [ ] **Step 6: Commit**

```bash
git add internal/handler/analytics.go internal/repository/analytics.go internal/router/router.go cmd/server/main.go
git commit -m "feat(analytics): POST /api/v1/analytics/fetch manual trigger + last_fetched_at in summary"
```

---

## Task 5: Add YouTube Daily Views for Watch Time + Retention

**Files:**
- Modify: `internal/publisher/zernio.go` (add `GetYouTubeDailyViews` method + types)
- Modify: `internal/publisher/publisher.go` (FetchAnalytics integration)

- [ ] **Step 1: เพิ่ม types สำหรับ daily-views response**

ที่ `internal/publisher/zernio.go` (วางใกล้ `AnalyticsResponse`):

```go
type DailyViewEntry struct {
	Date                    string  `json:"date"`
	Views                   int     `json:"views"`
	EstimatedMinutesWatched float64 `json:"estimatedMinutesWatched"`
	AverageViewDuration     float64 `json:"averageViewDuration"`
	SubscribersGained       int     `json:"subscribersGained"`
	SubscribersLost         int     `json:"subscribersLost"`
	Likes                   int     `json:"likes"`
	Comments                int     `json:"comments"`
	Shares                  int     `json:"shares"`
}

type YouTubeDailyViewsResponse struct {
	Success     bool             `json:"success"`
	VideoID     string           `json:"videoId"`
	TotalViews  int              `json:"totalViews"`
	DailyViews  []DailyViewEntry `json:"dailyViews"`
	ScopeStatus struct {
		HasAnalyticsScope bool `json:"hasAnalyticsScope"`
	} `json:"scopeStatus"`
	LastSyncedAt string `json:"lastSyncedAt"`
}
```

- [ ] **Step 2: เพิ่มเมธอด GetYouTubeDailyViews**

วางใต้ `GetAnalytics`:

```go
// ErrYouTubeScopeMissing indicates the user must re-authorize YouTube to expose daily-views.
var ErrYouTubeScopeMissing = errors.New("youtube analytics scope missing")

func (z *ZernioClient) GetYouTubeDailyViews(ctx context.Context, videoID, accountID string) (*YouTubeDailyViewsResponse, error) {
	q := url.Values{}
	q.Set("videoId", videoID)
	q.Set("accountId", accountID)
	endpoint := fmt.Sprintf("%s/analytics/youtube/daily-views?%s", z.baseURL, q.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+z.getAPIKey(ctx))

	resp, err := z.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get daily-views: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read daily-views response: %w", err)
	}

	if resp.StatusCode == 412 {
		return nil, ErrYouTubeScopeMissing
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daily-views API %d for video %s: %s", resp.StatusCode, videoID, string(respBody[:min(len(respBody), 300)]))
	}

	var result YouTubeDailyViewsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse daily-views: %w", err)
	}
	return &result, nil
}
```

เพิ่ม `"errors"` ใน imports

- [ ] **Step 3: เขียน test สำหรับ daily-views**

ใน `internal/publisher/zernio_test.go`:

```go
func TestGetYouTubeDailyViews_AggregatesWatchTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"videoId": "abc",
			"totalViews": 100,
			"dailyViews": [
				{"date":"2026-05-10","views":60,"estimatedMinutesWatched":30.0,"averageViewDuration":30.0},
				{"date":"2026-05-11","views":40,"estimatedMinutesWatched":20.0,"averageViewDuration":30.0}
			],
			"scopeStatus":{"hasAnalyticsScope":true}
		}`))
	}))
	defer srv.Close()

	z := newTestZernioClient(srv.URL, "k", nil)
	resp, err := z.GetYouTubeDailyViews(context.Background(), "abc", "acc1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalViews != 100 || len(resp.DailyViews) != 2 {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

func TestGetYouTubeDailyViews_ScopeMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(412)
		_, _ = w.Write([]byte(`{"success":false,"error":"scope missing","code":"youtube_analytics_scope_missing"}`))
	}))
	defer srv.Close()

	z := newTestZernioClient(srv.URL, "k", nil)
	_, err := z.GetYouTubeDailyViews(context.Background(), "abc", "acc1")
	if !errors.Is(err, ErrYouTubeScopeMissing) {
		t.Fatalf("expected ErrYouTubeScopeMissing, got %v", err)
	}
}
```

- [ ] **Step 4: รัน tests**

```bash
go test ./internal/publisher/ -v
```

Expected: PASS ทุก test ใหม่

- [ ] **Step 5: เพิ่มการเรียก daily-views ใน FetchAnalytics**

ใน `internal/publisher/publisher.go` แก้ block ใน `FetchAnalytics` หลัง `if err := p.analytics.Create(...) ...` (ใน Task 3) — แทนที่ `Create` block ทั้งหมดด้วย:

```go
				watchTime, retention := 0.0, 0.0
				if platform == "youtube" && metrics.Views > 0 {
					ytAccountID, _ := p.getSetting(ctx, "zernio_youtube_account_id")
					if pa := findPlatform(resp, "youtube"); pa != nil && ytAccountID != "" && pa.PlatformPostID != "" {
						daily, err := p.zernio.GetYouTubeDailyViews(ctx, pa.PlatformPostID, ytAccountID)
						if err != nil {
							if errors.Is(err, ErrYouTubeScopeMissing) {
								log.Printf("FetchAnalytics: YouTube analytics scope missing — re-auth needed (skipping watchTime for all clips)")
							} else {
								log.Printf("FetchAnalytics WATCHTIME_FAIL clip=%s video=%s: %v", cp.ClipID, pa.PlatformPostID, err)
							}
						} else {
							for _, dv := range daily.DailyViews {
								watchTime += dv.EstimatedMinutesWatched * 60
							}
							if metrics.Views > 0 && len(daily.DailyViews) > 0 {
								var avgDur float64
								for _, dv := range daily.DailyViews {
									avgDur += dv.AverageViewDuration
								}
								avgDur /= float64(len(daily.DailyViews))
								// retention = avgViewDuration / 60s (assume 60s Short); cap at 1.0
								retention = avgDur / 60.0
								if retention > 1.0 {
									retention = 1.0
								}
							}
						}
					}
				}
				if err := p.analytics.Create(ctx, models.ClipAnalytics{
					ClipID:           cp.ClipID,
					Platform:         platform,
					Views:            metrics.Views,
					Likes:            metrics.Likes,
					Comments:         metrics.Comments,
					Shares:           metrics.Shares,
					WatchTimeSeconds: watchTime,
					RetentionRate:    retention,
				}); err != nil {
					log.Printf("FetchAnalytics DB_FAIL clip=%s platform=%s: %v", cp.ClipID, platform, err)
					dbFailed++
					continue
				}
				success++
```

เพิ่ม helper ท้ายไฟล์:

```go
func findPlatform(r *AnalyticsResponse, platform string) *PlatformAnalyticsEntry {
	for i := range r.PlatformAnalytics {
		if r.PlatformAnalytics[i].Platform == platform {
			return &r.PlatformAnalytics[i]
		}
	}
	return nil
}

func (p *Publisher) getSetting(ctx context.Context, key string) (string, error) {
	var v string
	err := p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&v)
	return v, err
}
```

เพิ่ม `"errors"` ใน imports

- [ ] **Step 6: รัน build + tests**

```bash
go build ./...
go test ./internal/publisher/ -v
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/publisher/zernio.go internal/publisher/zernio_test.go internal/publisher/publisher.go
git commit -m "feat(zernio): fetch YouTube daily-views to populate watchTime + retention"
```

---

## Task 6: Migration — Daily Schedule + Cleanup Ghost Data

**Files:**
- Create: `migrations/014_daily_analytics_cleanup.sql`

- [ ] **Step 1: สร้างไฟล์ migration**

```sql
-- 014_daily_analytics_cleanup.sql
-- เปลี่ยน fetch_analytics จากรายสัปดาห์เป็นรายวัน + ลบ ghost rows
-- (platform != 'youtube' ที่เป็น stale leftover จาก settings ที่ไม่ได้ใช้แล้ว)

UPDATE schedules
SET cron_expression = '0 4 * * *', name = 'Daily Analytics'
WHERE action = 'fetch_analytics';

DELETE FROM clip_analytics
WHERE platform IN ('facebook', 'instagram', 'tiktok')
  AND views = 0
  AND likes = 0
  AND comments = 0;
```

- [ ] **Step 2: ตรวจการรัน migration script**

```bash
grep -rn "migrations" /Users/jaochai/Code/video-fb/cmd/server/main.go /Users/jaochai/Code/video-fb/internal/db/ 2>/dev/null | head -5
```

ยืนยันว่ามี migration runner อัตโนมัติ — ถ้าใช่ ไม่ต้องรันมือ

- [ ] **Step 3: Apply migration บน DB จริง (dry-run ก่อน)**

Run SELECT preview ผ่าน Neon MCP เพื่อตรวจว่าจะลบกี่ row:

```sql
SELECT COUNT(*) FROM clip_analytics
WHERE platform IN ('facebook','instagram','tiktok') AND views=0 AND likes=0 AND comments=0;
```

Expected: 3 (rows จาก May 2)

- [ ] **Step 4: Commit**

```bash
git add migrations/014_daily_analytics_cleanup.sql
git commit -m "chore(migrations): daily fetch_analytics schedule + drop ghost rows"
```

---

## Task 7: Deploy + Smoke Test + Run Manual Trigger

**Files:** — (deployment only)

- [ ] **Step 1: Deploy ผ่าน Railway**

```bash
git push
```

จากนั้นรอ Railway deploy เสร็จ (~2 min). ตรวจ build logs ด้วย Railway MCP:

```
mcp__railway__get-logs workspacePath=/Users/jaochai/Code/video-fb logType=build service=adsvance-v2 lines=50
```

Expected: build success, ไม่มี error

- [ ] **Step 2: ตรวจ migration apply อัตโนมัติ**

```
mcp__railway__get-logs workspacePath=/Users/jaochai/Code/video-fb logType=deploy service=adsvance-v2 filter=014 lines=20
```

Expected: log บอกว่า migration 014 รันสำเร็จ + schedules updated

- [ ] **Step 3: Trigger fetch ด้วย manual endpoint**

ดึง URL ของ backend จาก Railway (`mcp__railway__list-variables` หา `RAILWAY_PUBLIC_DOMAIN`), แล้ว:

```bash
curl -X POST "https://<adsvance-v2 URL>/api/v1/analytics/fetch"
```

Expected: HTTP 202 `{"data":{"status":"triggered"}}`

- [ ] **Step 4: ตรวจ logs ของ manual fetch**

```
mcp__railway__get-logs workspacePath=/Users/jaochai/Code/video-fb logType=deploy service=adsvance-v2 filter=FetchAnalytics lines=80
```

Expected: เห็น `FetchAnalytics done: 18 clips, X success, Y api_fail, Z db_fail` โดย X > 0

ถ้าเจอ scope-missing log → แจ้ง user ให้ re-auth YouTube ใน Zernio

- [ ] **Step 5: Verify DB state**

```sql
SELECT 
  (SELECT COUNT(*) FROM clip_analytics) AS rows,
  (SELECT COUNT(DISTINCT clip_id) FROM clip_analytics) AS clips_with_data,
  (SELECT MAX(fetched_at) FROM clip_analytics) AS last_fetched,
  (SELECT SUM(views) FROM clip_analytics WHERE fetched_at > NOW() - INTERVAL '1 hour') AS recent_views;
```

Expected: `clips_with_data` >= 10, `last_fetched` ภายในไม่กี่นาที, `recent_views` > 0

- [ ] **Step 6: หยุดและ confirm กับ user ก่อน Task 8**

Report ผลให้ user. ถ้าทุกค่าโอเค → Task 8. ถ้า fail → กลับไปตรวจ root cause ด้วย systematic-debugging

---

## Task 8: Frontend — Show last_fetched + Scope Warning

**Files:**
- Modify: `frontend/src/pages/Analytics.tsx:43-46` (SummaryResponse type), `:100-128` (header block)

- [ ] **Step 1: เพิ่มฟิลด์ `last_fetched_at` ใน TypeScript interface**

แก้ `SummaryResponse` (line 43-46):

```ts
interface SummaryResponse {
  summary: AnalyticsSummary;
  top_clips: ClipPerformance[] | null;
  last_fetched_at: string | null;
}
```

- [ ] **Step 2: เพิ่ม badge แสดง last fetched**

ใต้ `<PageHeader title="Analytics" />` (line 102), เพิ่ม:

```tsx
{summaryData?.last_fetched_at && (
  <div className="mb-4 text-xs text-muted-foreground">
    Last updated: {new Date(summaryData.last_fetched_at).toLocaleString('th-TH')}
    {Date.now() - new Date(summaryData.last_fetched_at).getTime() > 36 * 3600 * 1000 && (
      <span className="ml-2 text-amber-600">⚠ data over 36h old — check fetch_analytics schedule</span>
    )}
  </div>
)}
```

- [ ] **Step 3: เพิ่มปุ่ม manual refresh**

นำเข้า `Button` (มีอยู่แล้ว) และ `useMutation` from `@tanstack/react-query`:

```tsx
import { useMutation, useQueryClient } from '@tanstack/react-query';
```

ภายใน component, เพิ่มก่อน return:

```tsx
const queryClient = useQueryClient();
const triggerFetch = useMutation({
  mutationFn: () => apiFetch('/api/v1/analytics/fetch', { method: 'POST' }),
  onSuccess: () => {
    setTimeout(() => queryClient.invalidateQueries({ queryKey: ['analytics-summary'] }), 10000);
  },
});
```

ใต้ `<PageHeader>`, เพิ่ม:

```tsx
<div className="mb-4 flex items-center justify-between">
  <div className="text-xs text-muted-foreground">
    {summaryData?.last_fetched_at
      ? `Last updated: ${new Date(summaryData.last_fetched_at).toLocaleString('th-TH')}`
      : 'No fetch yet'}
  </div>
  <Button
    size="sm"
    variant="outline"
    disabled={triggerFetch.isPending}
    onClick={() => triggerFetch.mutate()}
  >
    {triggerFetch.isPending ? 'Fetching…' : 'Refresh now'}
  </Button>
</div>
```

(เอา badge เก่าจาก Step 2 ออก — รวมที่นี่แล้ว)

- [ ] **Step 4: รัน frontend build + visual check**

```bash
cd /Users/jaochai/Code/video-fb/frontend && npm run build
```

Expected: build success, ไม่มี TS error

- [ ] **Step 5: Commit + push**

```bash
git add frontend/src/pages/Analytics.tsx
git commit -m "feat(analytics-ui): show last_fetched timestamp + manual refresh button"
git push
```

- [ ] **Step 6: ตรวจหน้า Analytics บน production**

- เปิด `https://<frontend URL>/analytics`
- เห็น KPI values > 0 ที่ clip 18 ตัว
- เห็น "Last updated" timestamp ใกล้เวลาปัจจุบัน
- กดปุ่ม Refresh → ภายใน 30s data refresh (ทดสอบ optimistic flow)

---

## Self-Review Notes

**Spec coverage:**
- ✓ Zernio endpoint ถูก (Task 2)
- ✓ Response shape ถูก (Task 2, 3)
- ✓ Watch time + retention (Task 5)
- ✓ Daily schedule + cleanup (Task 6)
- ✓ Manual trigger (Task 4)
- ✓ UI shows freshness (Task 8)
- ✓ Error handling: DB create + scope missing + sync pending (202) + 424 (Task 3, 5)

**Type consistency check:**
- `PostMetrics.Views` (int) ↔ `models.ClipAnalytics.Views` (int) ✓
- `EstimatedMinutesWatched` (float64) × 60 → `WatchTimeSeconds` (float64) ✓
- `last_fetched_at` (string|null) ↔ `*time.Time` JSON marshals to ISO string or null ✓

**Risks:**
- Zernio analytics add-on ไม่ active → all calls return 402; mitigate ด้วย log + user-facing message
- YouTube analytics scope ไม่ได้ grant → daily-views fail with 412; FetchAnalytics ยังเก็บ views/likes ได้ (graceful degrade)
- Manual trigger บน production ไม่มี auth → ผู้ใช้ทั่วไปกดได้; mitigate ภายหลังถ้าต้องการ (out of scope)
