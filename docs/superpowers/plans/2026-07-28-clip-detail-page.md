# Clip Detail Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** กดคลิปแถวไหนก็ได้ในหน้า Content แล้วเปิดหน้า `/clips/:id` ที่แสดงทุกอย่างที่ระบบรู้เกี่ยวกับคลิปนั้น พร้อมปุ่มอนุมัติ/ตีกลับ/override/ลบ

**Architecture:** เพิ่ม endpoint รวมตัวเดียว `GET /api/v1/clips/{id}/detail` ที่ handler ใหม่เรียก repository 7 ตัวแล้วประกอบเป็น object เดียว (ส่วนย่อยที่ไม่มีข้อมูลเป็น `null`/`[]` ไม่ทำให้ทั้ง response พัง) ฝั่ง frontend เพิ่มหน้า React หนึ่งหน้าที่แบ่งเป็น 6 แท็บ แต่ละแท็บเป็นไฟล์ของตัวเอง แล้วลบ `ReviewDialog.tsx` โดยยกแอ็กชันและการแสดงผล QA ไปไว้ในหน้าใหม่

**Tech Stack:** Go 1.x + chi router + pgx ฝั่ง backend, React 18 + TypeScript + TanStack Query + react-router-dom + Tailwind + Radix UI ฝั่ง frontend

**Spec:** `docs/superpowers/specs/2026-07-28-clip-detail-page-design.md`

## Global Constraints

- **ห้ามคืน nil slice เป็น JSON** — ทุก list ที่ส่งออก API ต้อง `[]T{}` ไม่ใช่ `var xs []T` เพราะ nil slice serialize เป็น `null` แล้ว `.map()` ฝั่ง React พัง (`repository.ScenesRepo.ListByClip` และ `AnalyticsRepo.ListByClip` คืน nil slice เมื่อไม่มีแถว — handler ต้อง normalize เอง)
- **fail-soft** — คลิปไม่มีจริง = 404 เท่านั้น ส่วนย่อยอื่นที่ query พังหรือไม่มีข้อมูล ต้องกลายเป็น `null`/`[]` ไม่ใช่ 500
- **ข้อความ UI ทั้งหมดเป็นภาษาไทย** ตามหน้าอื่นในระบบ (ยกเว้นชื่อ field ทางเทคนิคที่ผู้ใช้คุ้นแล้ว เช่น `style_preset`)
- **เทสต์ในโปรเจกต์นี้ไม่ต่อ DB** — Go test ทุกไฟล์เป็น unit test ล้วน ห้ามเขียนเทสต์ที่ต้องมี Postgres
- **frontend ไม่มี test runner** — พิสูจน์ด้วย `npm run build` ผ่าน + เปิดหน้าจริงดูตา
- **Go command กับ sandbox** — `go build` / `go test` ในเครื่องนี้อาจโดน sandbox บล็อกตอนเขียน module cache (`operation not permitted`) ถ้าเจอ ให้รันซ้ำด้วย `dangerouslyDisableSandbox: true` ตัว build จริงยังผ่าน

---

### Task 1: โครงข้อมูลและ repository ที่ยังขาด

เพิ่ม struct 2 ตัว, 2 ฟิลด์ที่หายไปใน `ClipMetadata`, และ query 2 ตัวที่ยังไม่มี (`GetMetadata`, `ScriptDebatesRepo.GetByClip`) งานนี้ยังไม่มีเทสต์ของตัวเอง (แตะ DB ล้วน) — Task 2 คือตัวที่เทสต์ผลลัพธ์ของมัน

**Files:**
- Modify: `internal/models/clip.go` (เพิ่ม `ScriptDebate`, `ClipDetail`, และ 2 ฟิลด์ใน `ClipMetadata:86-96`)
- Modify: `internal/repository/clips.go` (เพิ่ม `GetMetadata` ท้ายไฟล์)
- Modify: `internal/repository/scriptdebates.go` (เพิ่ม `GetByClip`)

**Interfaces:**
- Consumes: `models.Clip`, `models.Scene`, `models.VisualQA`, `models.AutoReview`, `models.ClipAnalytics`, `models.ClipCritique` (มีอยู่แล้ว)
- Produces:
  - `models.ScriptDebate{ID, ClipID string; Candidates, Verdict json.RawMessage; Source string; CreatedAt time.Time}`
  - `models.ClipDetail{Clip *Clip; Metadata *ClipMetadata; Scenes []Scene; VisualQA *VisualQA; Critique *ClipCritique; AutoReview *AutoReview; Analytics []ClipAnalytics; ScriptDebate *ScriptDebate}`
  - `(*repository.ClipsRepo).GetMetadata(ctx context.Context, clipID string) (*models.ClipMetadata, error)`
  - `(*repository.ScriptDebatesRepo).GetByClip(ctx context.Context, clipID string) (*models.ScriptDebate, error)`

- [ ] **Step 1: เพิ่ม 2 ฟิลด์ที่ตกหล่นใน `ClipMetadata`**

migration 013 และ 032 เพิ่มคอลัมน์ `zernio_shorts_post_id` / `zernio_tiktok_post_id` เข้าตารางแล้ว แต่ struct ไม่เคยตามไปเก็บ ใน `internal/models/clip.go` แก้ struct `ClipMetadata` ให้เป็น:

```go
type ClipMetadata struct {
	ClipID              string   `json:"clip_id"`
	YoutubeTitle        *string  `json:"youtube_title"`
	YoutubeDesc         *string  `json:"youtube_description"`
	YoutubeTags         []string `json:"youtube_tags"`
	ZernioPostID        *string  `json:"zernio_post_id"`
	YoutubeVideoID      *string  `json:"youtube_video_id"`
	TiktokPostID        *string  `json:"tiktok_post_id"`
	IGPostID            *string  `json:"ig_post_id"`
	FBPostID            *string  `json:"fb_post_id"`
	ZernioShortsPostID  *string  `json:"zernio_shorts_post_id"`
	ZernioTiktokPostID  *string  `json:"zernio_tiktok_post_id"`
}
```

- [ ] **Step 2: เพิ่ม `ScriptDebate` และ `ClipDetail` ท้าย `internal/models/clip.go`**

```go
// ScriptDebate คือผลการดีเบตสคริปต์หนึ่งครั้ง (3 มุมมองเขียนแข่งกัน + judge ตัดสิน)
// Verdict เป็น NULL เมื่อข้าม judge หรือ judge พัง — json.RawMessage ที่เป็น nil
// marshal ออกมาเป็น null ตามที่ frontend คาดไว้
type ScriptDebate struct {
	ID         string          `json:"id"`
	ClipID     string          `json:"clip_id"`
	Candidates json.RawMessage `json:"candidates"`
	Verdict    json.RawMessage `json:"verdict"`
	Source     string          `json:"source"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ClipDetail คือทุกอย่างที่ระบบรู้เกี่ยวกับคลิปหนึ่งตัว รวมร่างไว้ให้หน้า
// /clips/:id ดึงครั้งเดียวจบ ฟิลด์ที่เป็น pointer จะเป็น null เมื่อคลิปนั้น
// ไม่มีข้อมูลส่วนนั้น (คลิปเก่าจำนวนมากไม่มี critique/debate/analytics)
type ClipDetail struct {
	Clip         *Clip           `json:"clip"`
	Metadata     *ClipMetadata   `json:"metadata"`
	Scenes       []Scene         `json:"scenes"`
	VisualQA     *VisualQA       `json:"visual_qa"`
	Critique     *ClipCritique   `json:"critique"`
	AutoReview   *AutoReview     `json:"auto_review"`
	Analytics    []ClipAnalytics `json:"analytics"`
	ScriptDebate *ScriptDebate   `json:"script_debate"`
}
```

`encoding/json` และ `time` ถูก import อยู่แล้วในไฟล์นี้

- [ ] **Step 3: เพิ่ม `GetMetadata` ท้าย `internal/repository/clips.go`**

คืน `(nil, nil)` เมื่อไม่มีแถว — รูปแบบเดียวกับ `CritiquesRepo.GetByClip`

```go
// GetMetadata คืน metadata ของคลิป (ชื่อ/คำบรรยาย YouTube + post id ของแต่ละ
// แพลตฟอร์ม) หรือ (nil, nil) เมื่อคลิปยังไม่ถูก publish จึงยังไม่มีแถว
func (r *ClipsRepo) GetMetadata(ctx context.Context, clipID string) (*models.ClipMetadata, error) {
	var m models.ClipMetadata
	err := r.pool.QueryRow(ctx,
		`SELECT clip_id, youtube_title, youtube_description, youtube_tags,
		        zernio_post_id, youtube_video_id, tiktok_post_id, ig_post_id, fb_post_id,
		        zernio_shorts_post_id, zernio_tiktok_post_id
		 FROM clip_metadata WHERE clip_id = $1`, clipID).
		Scan(&m.ClipID, &m.YoutubeTitle, &m.YoutubeDesc, &m.YoutubeTags,
			&m.ZernioPostID, &m.YoutubeVideoID, &m.TiktokPostID, &m.IGPostID, &m.FBPostID,
			&m.ZernioShortsPostID, &m.ZernioTiktokPostID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get metadata for clip %s: %w", clipID, err)
	}
	return &m, nil
}
```

ไฟล์นี้ import แค่ `context`, `fmt`, `pgxpool`, `models` — ต้องเพิ่ม `errors` และ `github.com/jackc/pgx/v5` เข้า import block

- [ ] **Step 4: เพิ่ม `GetByClip` ท้าย `internal/repository/scriptdebates.go`**

```go
// GetByClip คืนการดีเบตล่าสุดของคลิป หรือ (nil, nil) เมื่อคลิปนั้นผลิตตอน
// flag script_debate_enabled ปิดอยู่ จึงไม่มีแถว
func (r *ScriptDebatesRepo) GetByClip(ctx context.Context, clipID string) (*models.ScriptDebate, error) {
	var d models.ScriptDebate
	err := r.pool.QueryRow(ctx,
		`SELECT id, clip_id, candidates, verdict, source, created_at
		 FROM script_debates WHERE clip_id = $1 ORDER BY created_at DESC LIMIT 1`, clipID).
		Scan(&d.ID, &d.ClipID, &d.Candidates, &d.Verdict, &d.Source, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get script_debate for clip %s: %w", clipID, err)
	}
	return &d, nil
}
```

ต้องเพิ่ม import: `errors`, `fmt`, `github.com/jackc/pgx/v5`, `github.com/jaochai/video-fb/internal/models`

- [ ] **Step 5: คอมไพล์ผ่าน**

Run: `go build ./...`
Expected: ไม่มี output (สำเร็จ) — ถ้าเจอ `operation not permitted` เรื่อง module cache ให้รันซ้ำแบบปิด sandbox

- [ ] **Step 6: Commit**

```bash
git add internal/models/clip.go internal/repository/clips.go internal/repository/scriptdebates.go
git commit -m "feat(api): โครงข้อมูล ClipDetail + query metadata/script_debate ที่ยังขาด"
```

---

### Task 2: Endpoint `GET /api/v1/clips/{id}/detail`

Handler ใหม่ที่เรียก repo 7 ตัวแล้วประกอบร่าง แยกส่วนประกอบร่างออกเป็นฟังก์ชันบริสุทธิ์ `buildClipDetail` เพื่อให้เทสต์ normalization ได้โดยไม่ต้องมี DB ส่วนคลิปไม่เจอ = 404 เทสต์ผ่าน fake ที่ implement แค่ interface `clipSource`

**Files:**
- Create: `internal/handler/clip_detail.go`
- Create: `internal/handler/clip_detail_test.go`
- Modify: `internal/router/router.go:44` (เพิ่ม route ถัดจากบล็อก clips)

**Interfaces:**
- Consumes: `models.ClipDetail`, `models.ScriptDebate`, `(*ClipsRepo).GetMetadata`, `(*ScriptDebatesRepo).GetByClip` (Task 1)
- Produces:
  - `handler.NewClipDetailHandler(clips clipSource, scenes *repository.ScenesRepo, qa *repository.VisualQARepo, critiques *repository.CritiquesRepo, autoReviews *repository.AutoReviewsRepo, analytics *repository.AnalyticsRepo, debates *repository.ScriptDebatesRepo) *ClipDetailHandler`
  - `(*ClipDetailHandler).Get(w http.ResponseWriter, r *http.Request)` — chi URL param ชื่อ `clipId` (ชื่อเดียวกับ endpoint ย่อยอื่นของคลิปทุกตัวในไฟล์ router)
  - `buildClipDetail(...) models.ClipDetail` (ภายใน package)

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

สร้าง `internal/handler/clip_detail_test.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
)

// fakeClipSource ยืนแทน ClipsRepo — เทสต์ทั้งหมดในไฟล์นี้ไม่แตะ Postgres
type fakeClipSource struct {
	clip *models.Clip
	err  error
}

func (f fakeClipSource) GetByID(ctx context.Context, id string) (*models.Clip, error) {
	return f.clip, f.err
}

func (f fakeClipSource) GetMetadata(ctx context.Context, clipID string) (*models.ClipMetadata, error) {
	return nil, nil
}

// คลิปที่ไม่มีในระบบต้องได้ 404 ไม่ใช่ 500 — และต้องคืนก่อนแตะ repo ตัวอื่น
// (ตัวอื่นเป็น nil ในเทสต์นี้ ถ้าโค้ดเผลอเรียกจะ panic ให้เห็นทันที)
func TestClipDetailNotFound(t *testing.T) {
	h := NewClipDetailHandler(
		fakeClipSource{nil, errors.New("no rows")},
		nil, nil, nil, nil, nil, nil,
	)
	r := chi.NewRouter()
	r.Get("/clips/{clipId}/detail", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/clips/ไม่มีจริง/detail", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}

// คลิปที่ไม่มี scene/analytics ต้อง serialize เป็น [] ไม่ใช่ null — ไม่งั้น
// .map() ฝั่ง React พังทั้งหน้า (เคยเกิดกับ /prompt-history มาแล้ว)
func TestBuildClipDetailEmptyListsSerializeAsArrays(t *testing.T) {
	d := buildClipDetail(&models.Clip{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(b)
	if !strings.Contains(body, `"scenes":[]`) {
		t.Errorf("scenes ต้องเป็น [] ไม่ใช่ null: %s", body)
	}
	if !strings.Contains(body, `"analytics":[]`) {
		t.Errorf("analytics ต้องเป็น [] ไม่ใช่ null: %s", body)
	}
}

// ส่วนที่คลิปไม่มีจริงๆ (critique/auto_review/debate/metadata) ต้องเป็น null
// เพื่อให้หน้าเว็บแยกออกว่า "ไม่มีข้อมูล" ต่างจาก "มีแต่ว่าง"
func TestBuildClipDetailMissingOptionalsStayNull(t *testing.T) {
	d := buildClipDetail(&models.Clip{ID: "c1"}, nil, nil, nil, nil, nil, nil, nil)

	b, _ := json.Marshal(d)
	body := string(b)
	for _, want := range []string{`"metadata":null`, `"critique":null`, `"auto_review":null`, `"script_debate":null`, `"visual_qa":null`} {
		if !strings.Contains(body, want) {
			t.Errorf("ขาด %s ใน %s", want, body)
		}
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่ามันพัง**

Run: `go test ./internal/handler/ -run 'ClipDetail' -v`
Expected: คอมไพล์ไม่ผ่าน — `undefined: NewClipDetailHandler`, `undefined: buildClipDetail`

- [ ] **Step 3: เขียน handler**

สร้าง `internal/handler/clip_detail.go`:

```go
package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

// clipSource คือสองอย่างที่ต้องได้จาก ClipsRepo — แยกเป็น interface เพื่อให้
// เทสต์ 404 รันได้โดยไม่ต้องมี Postgres
type clipSource interface {
	GetByID(ctx context.Context, id string) (*models.Clip, error)
	GetMetadata(ctx context.Context, clipID string) (*models.ClipMetadata, error)
}

type ClipDetailHandler struct {
	clips       clipSource
	scenes      *repository.ScenesRepo
	qa          *repository.VisualQARepo
	critiques   *repository.CritiquesRepo
	autoReviews *repository.AutoReviewsRepo
	analytics   *repository.AnalyticsRepo
	debates     *repository.ScriptDebatesRepo
}

func NewClipDetailHandler(
	clips clipSource,
	scenes *repository.ScenesRepo,
	qa *repository.VisualQARepo,
	critiques *repository.CritiquesRepo,
	autoReviews *repository.AutoReviewsRepo,
	analytics *repository.AnalyticsRepo,
	debates *repository.ScriptDebatesRepo,
) *ClipDetailHandler {
	return &ClipDetailHandler{clips, scenes, qa, critiques, autoReviews, analytics, debates}
}

// Get คืนทุกอย่างที่ระบบรู้เกี่ยวกับคลิปหนึ่งตัวในก้อนเดียว หน้า /clips/:id
// ใช้ครบทุกก้อนเสมอ การแยกเป็น 7 request จึงได้แค่ loading state 7 อัน
//
// คลิปไม่มีจริง = 404 ส่วนก้อนอื่นที่ query พัง = null/[] ไม่ลาก response ทั้งก้อนตายไปด้วย
// เพราะคลิปเก่าจำนวนมากไม่มี critique/debate/analytics อยู่แล้ว
func (h *ClipDetailHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "clipId")

	clip, err := h.clips.GetByID(ctx, id)
	if err != nil || clip == nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "clip not found"})
		return
	}

	metadata, _ := h.clips.GetMetadata(ctx, id)
	scenes, _ := h.scenes.ListByClip(ctx, id)
	qa, _ := h.qa.GetLatestByClipID(ctx, id)
	critique, _ := h.critiques.GetByClip(ctx, id)
	autoReview, _ := h.autoReviews.GetByClip(ctx, id)
	analytics, _ := h.analytics.ListByClip(ctx, id)
	debate, _ := h.debates.GetByClip(ctx, id)

	writeJSON(w, http.StatusOK, models.APIResponse{
		Data: buildClipDetail(clip, metadata, scenes, qa, critique, autoReview, analytics, debate),
	})
}

// buildClipDetail ประกอบก้อนข้อมูลเข้าด้วยกันและบังคับให้ list ที่ว่างเป็น []
// ไม่ใช่ null — nil slice ของ Go serialize เป็น null แล้ว .map() ฝั่ง React พัง
func buildClipDetail(
	clip *models.Clip,
	metadata *models.ClipMetadata,
	scenes []models.Scene,
	qa *models.VisualQA,
	critique *models.ClipCritique,
	autoReview *models.AutoReview,
	analytics []models.ClipAnalytics,
	debate *models.ScriptDebate,
) models.ClipDetail {
	if scenes == nil {
		scenes = []models.Scene{}
	}
	if analytics == nil {
		analytics = []models.ClipAnalytics{}
	}
	return models.ClipDetail{
		Clip:         clip,
		Metadata:     metadata,
		Scenes:       scenes,
		VisualQA:     qa,
		Critique:     critique,
		AutoReview:   autoReview,
		Analytics:    analytics,
		ScriptDebate: debate,
	}
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/handler/ -run 'ClipDetail' -v`
Expected: PASS ทั้ง 3 เทสต์

- [ ] **Step 5: ผูก route**

ใน `internal/router/router.go` — บล็อก `r.Route("/api/v1/clips", ...)` จบที่บรรทัด 42 และ `visualQA` เริ่มที่ 44 ให้แทรกระหว่างนั้น (ต้องสร้าง repo แต่ละตัวใหม่ ไม่ reuse ตัวแปรที่ประกาศทีหลังในไฟล์ เพราะลำดับการประกาศใน Go ต้องมาก่อนใช้):

```go
	clipDetail := handler.NewClipDetailHandler(
		repository.NewClipsRepo(pool),
		repository.NewScenesRepo(pool),
		repository.NewVisualQARepo(pool),
		repository.NewCritiquesRepo(pool),
		repository.NewAutoReviewsRepo(pool),
		repository.NewAnalyticsRepo(pool),
		repository.NewScriptDebatesRepo(pool),
	)
	r.Get("/api/v1/clips/{clipId}/detail", clipDetail.Get)
```

ใช้ `{clipId}` ไม่ใช่ `{id}` เพราะ endpoint ย่อยของคลิปทุกตัวในไฟล์นี้ (`/visual-qa`,
`/scenes`, `/analytics`, `/critique`, `/auto-review`) ใช้ชื่อนี้อยู่แล้ว

- [ ] **Step 6: รันเทสต์ทั้ง package + build**

Run: `go build ./... && go test ./internal/...`
Expected: build ผ่าน, เทสต์ทั้งหมด PASS (ไม่มีเทสต์เดิมพัง)

- [ ] **Step 7: Commit**

```bash
git add internal/handler/clip_detail.go internal/handler/clip_detail_test.go internal/router/router.go
git commit -m "feat(api): GET /clips/{id}/detail รวมทุกอย่างของคลิปไว้ในก้อนเดียว"
```

---

### Task 3: โครงหน้า `/clips/:id` + ทำให้ทุกแถวกดได้

หน้าที่ดึงข้อมูลได้ครบ มี header + วิดีโอ + แท็บว่าง 6 อัน (เนื้อหาแท็บมาใน Task 4-6) และตารางที่กดแถวไหนก็เข้าหน้านี้ได้ ตอนจบ task นี้ต้องเปิดหน้าได้จริงและเห็นชื่อคลิปกับวิดีโอ

**Files:**
- Modify: `frontend/src/lib/routes.ts`
- Modify: `frontend/src/api.ts` (ต่อท้ายไฟล์)
- Create: `frontend/src/pages/ClipDetail.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/pages/Content.tsx:406-417` (แถวกดได้ทุกสถานะ)

**Interfaces:**
- Consumes: `GET /api/v1/clips/{id}/detail` (Task 2)
- Produces:
  - `api.ts`: `interface ClipFull`, `interface ClipMetadata`, `interface Scene`, `interface VisualQAResult`, `interface SceneVerdict`, `interface ClipAnalyticsRow`, `interface ScriptDebate`, `interface ClipDetail`, `getClipDetail(id: string): Promise<ClipDetail>`
  - `ROUTES.CLIP_DETAIL = '/clips/:id'`
  - `pages/ClipDetail.tsx` default export `ClipDetailPage`

- [ ] **Step 1: เพิ่ม type และ fetcher ท้าย `frontend/src/api.ts`**

`ClipCritique` และ `AutoReview` มีอยู่แล้วในไฟล์นี้ (บรรทัด 58 และ 78) — reuse ไม่ประกาศซ้ำ

```ts
export interface ClipFull {
  id: string;
  title: string;
  question: string;
  questioner_name: string;
  answer_script: string;
  voice_script: string;
  category: string;
  status: string;
  video_16_9_url: string | null;
  video_9_16_url: string | null;
  thumbnail_url: string | null;
  publish_date: string | null;
  created_at: string;
  updated_at: string;
  fail_reason?: string;
  retry_count: number;
  review_retry_count: number;
  auto_review_held: boolean;
  style_preset: string;
  content_format: string;
  production_stage: string;
  case_number?: number;
  tutorial_feature: string;
}

export interface ClipMetadata {
  clip_id: string;
  youtube_title: string | null;
  youtube_description: string | null;
  youtube_tags: string[] | null;
  zernio_post_id: string | null;
  youtube_video_id: string | null;
  tiktok_post_id: string | null;
  ig_post_id: string | null;
  fb_post_id: string | null;
  zernio_shorts_post_id: string | null;
  zernio_tiktok_post_id: string | null;
}

export interface Scene {
  id: string;
  scene_number: number;
  scene_type: string;
  image_9_16_url: string | null;
  voice_text: string;
  duration_seconds: number;
  on_screen_text: string;
  beat: string;
  layout: string;
  caption_style: string;
}

export interface SceneVerdict {
  scene_number: number;
  ok: boolean;
  issues: string[];
}

export interface VisualQAResult {
  id: string;
  clip_id: string;
  passed: boolean;
  issues: SceneVerdict[];
  created_at: string;
}

export interface ClipAnalyticsRow {
  id: string;
  platform: string;
  post_type: string;
  views: number;
  likes: number;
  comments: number;
  shares: number;
  watch_time_seconds: number;
  retention_rate: number;
  fetched_at: string;
}

export interface ScriptDebate {
  id: string;
  source: string;
  candidates: unknown;
  verdict: unknown;
  created_at: string;
}

export interface ClipDetail {
  clip: ClipFull;
  metadata: ClipMetadata | null;
  scenes: Scene[];
  visual_qa: VisualQAResult | null;
  critique: ClipCritique | null;
  auto_review: AutoReview | null;
  analytics: ClipAnalyticsRow[];
  script_debate: ScriptDebate | null;
}

export const getClipDetail = (id: string) =>
  apiFetch<ClipDetail>(`/api/v1/clips/${id}/detail`);
```

- [ ] **Step 2: เพิ่ม route**

`frontend/src/lib/routes.ts` — เพิ่มบรรทัดต่อจาก `CONTENT`:

```ts
  CLIP_DETAIL: '/clips/:id',
```

`frontend/src/App.tsx` — เพิ่ม import และ `<Route>` ต่อจากบรรทัด 38:

```tsx
import ClipDetailPage from "./pages/ClipDetail"
```
```tsx
                    <Route path={ROUTES.CLIP_DETAIL} element={<ClipDetailPage />} />
```

- [ ] **Step 3: สร้างหน้า `frontend/src/pages/ClipDetail.tsx`**

แท็บทั้ง 6 ยังว่าง — Task 4-6 จะเติมทีละอัน ตอนนี้ขอแค่โหลดข้อมูลได้ เห็นหัวเรื่องกับวิดีโอ

```tsx
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { getClipDetail } from '../api';
import { StatusBadge } from '../components/status-badge';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Skeleton } from '../components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../components/ui/tabs';
import { ArrowLeft, Lock, VideoOff } from 'lucide-react';

export default function ClipDetailPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const [videoError, setVideoError] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ['clip-detail', id],
    queryFn: () => getClipDetail(id),
    enabled: id !== '',
  });

  if (isLoading) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-8 w-2/3" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="text-center py-12">
        <p className="text-sm text-muted-foreground">ไม่พบคลิปนี้ (อาจถูกลบไปแล้ว)</p>
        <Button variant="ghost" className="mt-2" onClick={() => navigate('/')}>
          <ArrowLeft className="size-4" /> กลับไปหน้ารายการ
        </Button>
      </div>
    );
  }

  const { clip } = data;
  const held = clip.status === 'ready' && clip.auto_review_held;

  return (
    <div>
      <div className="mb-4">
        <Button variant="ghost" size="sm" className="-ml-2 mb-2" onClick={() => navigate('/')}>
          <ArrowLeft className="size-4" /> กลับ
        </Button>
        <div className="flex items-start gap-2 flex-wrap">
          <h1 className="text-lg font-semibold leading-snug flex-1 min-w-0">{clip.title}</h1>
          <StatusBadge status={clip.status} />
          {held && (
            <Badge variant="outline" className="gap-1 border-transparent bg-amber-100 text-amber-700 text-[10px]">
              <Lock className="size-2.5" /> ถูกกัก QA
            </Badge>
          )}
        </div>
        <p className="text-xs text-muted-foreground mt-1">
          {clip.category}
          {clip.content_format ? ` · ${clip.content_format}` : ''}
          {clip.style_preset ? ` · ${clip.style_preset}` : ''}
        </p>
      </div>

      <div className="grid gap-5 sm:grid-cols-[240px_1fr] items-start">
        <div className="sm:sticky sm:top-4">
          {clip.video_9_16_url && !videoError ? (
            <video
              src={clip.video_9_16_url}
              controls
              onError={() => setVideoError(true)}
              className="w-full rounded-lg bg-black aspect-[9/16]"
            />
          ) : (
            <div className="w-full aspect-[9/16] rounded-lg bg-muted flex flex-col items-center justify-center gap-1.5 text-xs text-muted-foreground text-center p-4">
              <VideoOff className="size-5 opacity-50" />
              {clip.video_9_16_url ? (
                <span>วิดีโอหมดอายุแล้ว<br />(ไฟล์ชั่วคราวถูกลบ)</span>
              ) : (
                <span>ยังไม่มีไฟล์วิดีโอ</span>
              )}
            </div>
          )}
        </div>

        <Tabs defaultValue="overview" className="min-w-0">
          <TabsList className="flex-wrap h-auto">
            <TabsTrigger value="overview">ภาพรวม</TabsTrigger>
            <TabsTrigger value="script">สคริปต์</TabsTrigger>
            <TabsTrigger value="scenes">ฉาก ({data.scenes.length})</TabsTrigger>
            <TabsTrigger value="qa">QA & รีวิว</TabsTrigger>
            <TabsTrigger value="stats">ตัวเลข</TabsTrigger>
            <TabsTrigger value="publish">เผยแพร่</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" />
          <TabsContent value="script" />
          <TabsContent value="scenes" />
          <TabsContent value="qa" />
          <TabsContent value="stats" />
          <TabsContent value="publish" />
        </Tabs>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: ทำให้ทุกแถวในตารางกดได้**

ใน `frontend/src/pages/Content.tsx`:

เพิ่ม import:
```tsx
import { useNavigate } from 'react-router-dom';
```

ในตัว component เพิ่มบรรทัดถัดจาก `const queryClient = useQueryClient();`:
```tsx
  const navigate = useNavigate();
```

แก้บล็อก `paged.map` (บรรทัด 406-417) — ตัวแปร `clickable` หายไป ทุกแถวกดได้หมด ส่วน `reviewable`/`held` ยังใช้แสดงป้ายเตือนใต้ชื่อคลิป:

```tsx
              {paged.map(clip => {
                const reviewable = clip.status === 'needs_review';
                // A held clip is 'ready' but the publisher skips it (Visual QA gate).
                const held = clip.status === 'ready' && !!clip.auto_review_held;
                return (
                <TableRow
                  key={clip.id}
                  onClick={() => navigate(`/clips/${clip.id}`)}
                  className="cursor-pointer hover:bg-muted/50"
                >
```

ปุ่มลบในแถวมี `e.stopPropagation()` อยู่แล้ว (บรรทัด 479) จึงไม่เด้งเข้าหน้ารายละเอียด — ไม่ต้องแก้

- [ ] **Step 5: Build ผ่าน**

Run: `cd frontend && npm run build`
Expected: build สำเร็จ ไม่มี TypeScript error

หมายเหตุ: ตอนนี้ `ReviewDialog` ยังถูก import อยู่ใน `Content.tsx` และยังทำงานได้ (state `reviewClip` ไม่มีอะไรมา set แล้ว) — Task 7 จะลบทิ้ง อย่าเพิ่งลบตอนนี้เพราะจะทำให้ Task 4-6 ไม่มีที่อ้างอิงโค้ด QA เดิม

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/routes.ts frontend/src/api.ts frontend/src/App.tsx frontend/src/pages/ClipDetail.tsx frontend/src/pages/Content.tsx
git commit -m "feat(ui): หน้า /clips/:id + ทำให้ทุกแถวในตารางกดเข้าไปดูได้"
```

---

### Task 4: แท็บภาพรวม + แท็บสคริปต์

**Files:**
- Create: `frontend/src/components/clip-detail/OverviewTab.tsx`
- Create: `frontend/src/components/clip-detail/ScriptTab.tsx`
- Modify: `frontend/src/pages/ClipDetail.tsx` (แทน `<TabsContent value="overview" />` และ `"script"`)

**Interfaces:**
- Consumes: `ClipFull`, `ScriptDebate` จาก `api.ts` (Task 3)
- Produces:
  - `OverviewTab({ clip }: { clip: ClipFull })`
  - `ScriptTab({ clip, debate }: { clip: ClipFull; debate: ScriptDebate | null })`

- [ ] **Step 1: สร้าง `OverviewTab.tsx`**

แถวที่ค่าว่างไม่ต้องแสดง — คลิปแต่ละประเภทมีฟิลด์ที่ใช้ต่างกัน (คลิป tutorial มี `tutorial_feature`, คลิป case-file มี `case_number`)

```tsx
import type { ClipFull } from '../../api';

function Row({ label, value }: { label: string; value: string | number | null | undefined }) {
  if (value === null || value === undefined || value === '') return null;
  return (
    <div className="flex gap-3 py-2 border-b last:border-0 text-sm">
      <span className="text-muted-foreground w-32 shrink-0">{label}</span>
      <span className="min-w-0 break-words">{value}</span>
    </div>
  );
}

function thaiDateTime(s: string | null): string | null {
  if (!s) return null;
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  return d.toLocaleString('th-TH', { dateStyle: 'medium', timeStyle: 'short' });
}

export function OverviewTab({ clip }: { clip: ClipFull }) {
  return (
    <div>
      <Row label="คำถาม" value={clip.question} />
      <Row label="ผู้ถาม" value={clip.questioner_name} />
      <Row label="หมวดหมู่" value={clip.category} />
      <Row label="รูปแบบเนื้อหา" value={clip.content_format} />
      <Row label="ชุดสไตล์" value={clip.style_preset} />
      <Row label="ขั้นการผลิต" value={clip.production_stage} />
      <Row label="เลขคดี" value={clip.case_number} />
      <Row label="หัวข้อสอน" value={clip.tutorial_feature} />
      <Row label="retry" value={clip.retry_count > 0 ? `${clip.retry_count}/2` : null} />
      <Row label="รีวิวซ้ำ" value={clip.review_retry_count > 0 ? clip.review_retry_count : null} />
      <Row label="สาเหตุที่ล้มเหลว" value={clip.fail_reason} />
      <Row label="สร้างเมื่อ" value={thaiDateTime(clip.created_at)} />
      <Row label="แก้ไขล่าสุด" value={thaiDateTime(clip.updated_at)} />
      <Row label="กำหนดเผยแพร่" value={clip.publish_date} />
    </div>
  );
}
```

- [ ] **Step 2: สร้าง `ScriptTab.tsx`**

`candidates` และ `verdict` มาจาก JSONB จึงเป็น `unknown` — ต้อง narrow ก่อนใช้ ถ้ารูปทรงไม่ตรงที่คาด ให้ข้ามส่วนดีเบตไปเงียบๆ แทนที่จะพังทั้งแท็บ

```tsx
import type { ClipFull, ScriptDebate } from '../../api';

interface Candidate { lens?: string; answer_script?: string }
interface Score { lens?: string; hook?: number; accuracy?: number; audience_fit?: number }
interface Verdict { scores?: Score[]; winner_lens?: string; rationale?: string }

function asCandidates(raw: unknown): Candidate[] {
  return Array.isArray(raw) ? (raw as Candidate[]) : [];
}

function asVerdict(raw: unknown): Verdict | null {
  return raw !== null && typeof raw === 'object' ? (raw as Verdict) : null;
}

function Block({ title, text }: { title: string; text: string }) {
  if (!text) return null;
  return (
    <div className="mb-4">
      <h3 className="text-sm font-semibold mb-1.5">{title}</h3>
      <p className="text-sm leading-relaxed whitespace-pre-wrap bg-muted/40 rounded-lg p-3">{text}</p>
    </div>
  );
}

export function ScriptTab({ clip, debate }: { clip: ClipFull; debate: ScriptDebate | null }) {
  const candidates = asCandidates(debate?.candidates);
  const verdict = asVerdict(debate?.verdict);

  return (
    <div>
      <Block title="สคริปต์คำตอบ" text={clip.answer_script} />
      <Block title="สคริปต์เสียงพากย์" text={clip.voice_script} />
      {!clip.answer_script && !clip.voice_script && (
        <p className="text-sm text-muted-foreground">คลิปนี้ยังไม่มีสคริปต์</p>
      )}

      {debate && (
        <div className="border-t pt-3 mt-2">
          <h3 className="text-sm font-semibold mb-1">การดีเบตสคริปต์</h3>
          <p className="text-xs text-muted-foreground mb-3">
            ที่มาของฉบับที่ใช้จริง: {debate.source}
            {verdict?.winner_lens ? ` · ผู้ชนะ: ${verdict.winner_lens}` : ''}
          </p>

          {verdict?.rationale && (
            <p className="text-sm leading-relaxed bg-muted/40 rounded-lg p-3 mb-3">{verdict.rationale}</p>
          )}

          {(verdict?.scores ?? []).map((s, i) => (
            <div key={i} className="text-xs text-muted-foreground mb-1">
              {s.lens}: hook {s.hook} · accuracy {s.accuracy} · audience_fit {s.audience_fit}
            </div>
          ))}

          {candidates.map((c, i) => (
            <details key={i} className="mt-2 rounded-lg border p-3">
              <summary className="text-sm font-medium cursor-pointer">
                ฉบับของ {c.lens ?? `มุมมองที่ ${i + 1}`}
              </summary>
              <p className="text-sm leading-relaxed whitespace-pre-wrap mt-2">{c.answer_script ?? ''}</p>
            </details>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: เสียบเข้าหน้าหลัก**

ใน `frontend/src/pages/ClipDetail.tsx` เพิ่ม import:

```tsx
import { OverviewTab } from '../components/clip-detail/OverviewTab';
import { ScriptTab } from '../components/clip-detail/ScriptTab';
```

แล้วแทน 2 บรรทัด:

```tsx
          <TabsContent value="overview"><OverviewTab clip={clip} /></TabsContent>
          <TabsContent value="script"><ScriptTab clip={clip} debate={data.script_debate} /></TabsContent>
```

- [ ] **Step 4: Build ผ่าน**

Run: `cd frontend && npm run build`
Expected: build สำเร็จ ไม่มี TypeScript error

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/clip-detail/ frontend/src/pages/ClipDetail.tsx
git commit -m "feat(ui): แท็บภาพรวมกับแท็บสคริปต์ (รวมผลดีเบต 3 มุมมอง)"
```

---

### Task 5: แท็บฉาก

**Files:**
- Create: `frontend/src/components/clip-detail/ScenesTab.tsx`
- Modify: `frontend/src/pages/ClipDetail.tsx`

**Interfaces:**
- Consumes: `Scene[]` จาก `api.ts` (Task 3)
- Produces: `ScenesTab({ scenes }: { scenes: Scene[] })`

- [ ] **Step 1: สร้าง `ScenesTab.tsx`**

ภาพของคลิปเก่าอาจโหลดไม่ขึ้น (URL ชั่วคราวจาก kie หมดอายุ) — ต้องมี placeholder เหมือนที่หน้าหลักทำกับวิดีโอ

```tsx
import { useState } from 'react';
import type { Scene } from '../../api';
import { ImageOff } from 'lucide-react';

function SceneImage({ url }: { url: string | null }) {
  const [failed, setFailed] = useState(false);

  if (!url || failed) {
    return (
      <div className="w-[104px] shrink-0 aspect-[9/16] rounded-md bg-muted flex items-center justify-center">
        <ImageOff className="size-4 text-muted-foreground opacity-50" />
      </div>
    );
  }
  return (
    <img
      src={url}
      onError={() => setFailed(true)}
      className="w-[104px] shrink-0 aspect-[9/16] object-cover rounded-md bg-black"
    />
  );
}

export function ScenesTab({ scenes }: { scenes: Scene[] }) {
  if (scenes.length === 0) {
    return <p className="text-sm text-muted-foreground">คลิปนี้ยังไม่มีฉาก (ผลิตไม่ถึงขั้นแตกฉาก)</p>;
  }

  return (
    <div className="space-y-3">
      {scenes.map(s => (
        <div key={s.id} className="flex gap-3 rounded-lg border p-3">
          <SceneImage url={s.image_9_16_url} />
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap text-xs text-muted-foreground mb-1">
              <span className="font-medium text-foreground">ฉาก {s.scene_number}</span>
              {s.scene_type && <span>{s.scene_type}</span>}
              {s.beat && <span>· {s.beat}</span>}
              {s.layout && <span>· {s.layout}</span>}
              <span>· {s.duration_seconds.toFixed(1)} วิ</span>
            </div>
            {s.on_screen_text && (
              <p className="text-sm font-medium leading-snug mb-1 break-words">{s.on_screen_text}</p>
            )}
            {s.voice_text && (
              <p className="text-xs text-muted-foreground leading-relaxed whitespace-pre-wrap break-words">
                {s.voice_text}
              </p>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 2: เสียบเข้าหน้าหลัก**

```tsx
import { ScenesTab } from '../components/clip-detail/ScenesTab';
```
```tsx
          <TabsContent value="scenes"><ScenesTab scenes={data.scenes} /></TabsContent>
```

- [ ] **Step 3: Build ผ่าน**

Run: `cd frontend && npm run build`
Expected: build สำเร็จ

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/clip-detail/ScenesTab.tsx frontend/src/pages/ClipDetail.tsx
git commit -m "feat(ui): แท็บฉาก — ภาพ/ข้อความบนจอ/บทพากย์/ความยาวรายฉาก"
```

---

### Task 6: แท็บ QA & รีวิว + ตัวเลข + เผยแพร่

**Files:**
- Create: `frontend/src/components/clip-detail/QaTab.tsx`
- Create: `frontend/src/components/clip-detail/StatsTab.tsx`
- Create: `frontend/src/components/clip-detail/PublishTab.tsx`
- Modify: `frontend/src/pages/ClipDetail.tsx`

**Interfaces:**
- Consumes: `VisualQAResult`, `ClipCritique`, `AutoReview`, `ClipAnalyticsRow[]`, `ClipMetadata`, `ClipFull` จาก `api.ts`
- Produces:
  - `QaTab({ qa, critique, autoReview }: { qa: VisualQAResult | null; critique: ClipCritique | null; autoReview: AutoReview | null })`
  - `StatsTab({ rows }: { rows: ClipAnalyticsRow[] })`
  - `PublishTab({ clip, metadata }: { clip: ClipFull; metadata: ClipMetadata | null })`

- [ ] **Step 1: สร้าง `QaTab.tsx`**

ยกการแสดงผลจาก `ReviewDialog.tsx:164-261` มา รวมทั้งฟังก์ชัน `parseScoreFields` ที่ `critique.score` เป็น `unknown` (อาจเป็น string JSON หรือ object แล้วแต่รอบ)

```tsx
import type { AutoReview, ClipCritique, VisualQAResult } from '../../api';
import { Badge } from '../ui/badge';
import { AlertTriangle, ShieldCheck } from 'lucide-react';

function parseScoreFields(raw: unknown): [string, number][] {
  let obj: unknown = raw;
  if (typeof raw === 'string') {
    try { obj = JSON.parse(raw); } catch { return []; }
  }
  if (obj !== null && typeof obj === 'object') {
    return Object.entries(obj as Record<string, unknown>)
      .filter((entry): entry is [string, number] => typeof entry[1] === 'number');
  }
  return [];
}

export function QaTab({
  qa, critique, autoReview,
}: {
  qa: VisualQAResult | null;
  critique: ClipCritique | null;
  autoReview: AutoReview | null;
}) {
  const failedScenes = qa?.issues?.filter(v => !v.ok) ?? [];
  const scoreFields = parseScoreFields(critique?.score);

  return (
    <div className="space-y-5">
      <div>
        <h3 className="text-sm font-semibold mb-2">ผลตรวจ Visual QA</h3>
        {!qa ? (
          <p className="text-sm text-muted-foreground">
            ไม่พบผลตรวจ QA ของคลิปนี้ (อาจถูกตั้งสถานะด้วยมือ)
          </p>
        ) : failedScenes.length === 0 ? (
          <div className="flex items-start gap-2 text-sm text-emerald-600">
            <ShieldCheck className="size-4 mt-0.5 shrink-0" />
            <span>ไม่พบ scene ที่มีปัญหาในผลตรวจล่าสุด</span>
          </div>
        ) : (
          <div className="space-y-2">
            <p className="text-xs text-muted-foreground">
              AI ตรวจเจอปัญหาใน {failedScenes.length} scene — อ่านแล้วดูวิดีโอประกอบก่อนตัดสินใจ
            </p>
            {failedScenes.map(v => (
              <div key={v.scene_number} className="rounded-lg border border-amber-200 bg-amber-50 p-3">
                <div className="flex items-center gap-1.5 text-sm font-medium text-amber-800">
                  <AlertTriangle className="size-3.5" />
                  Scene {v.scene_number}
                </div>
                <ul className="mt-1 ml-1 space-y-0.5">
                  {v.issues.map((issue, i) => (
                    <li key={i} className="text-xs text-amber-700 leading-snug">• {issue}</li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        )}
      </div>

      {critique && (
        <div className="border-t pt-3">
          <div className="flex items-center gap-2 mb-2">
            <h3 className="text-sm font-semibold">Content Critic</h3>
            {critique.applied && (
              <Badge className="bg-blue-100 text-blue-700 border-blue-200 text-xs">Applied</Badge>
            )}
          </div>
          {scoreFields.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {scoreFields.map(([k, v]) => (
                <span key={k} className="text-xs bg-muted rounded px-2 py-1">{k}: {v}</span>
              ))}
            </div>
          )}
        </div>
      )}

      {autoReview && (
        <div className="border-t pt-3">
          <div className="flex items-center gap-2 mb-2">
            <h3 className="text-sm font-semibold">Auto-review</h3>
            <Badge className="bg-blue-100 text-blue-700 border-blue-200 text-xs">
              {autoReview.decision}
            </Badge>
          </div>
          <div className="flex flex-wrap gap-2 mb-2">
            <span className="text-xs bg-muted rounded px-2 py-1">confidence: {autoReview.confidence}</span>
            {autoReview.defect_type && (
              <span className="text-xs bg-muted rounded px-2 py-1">defect: {autoReview.defect_type}</span>
            )}
          </div>
          {autoReview.reasons?.length > 0 && (
            <ul className="ml-1 space-y-0.5">
              {autoReview.reasons.map((reason, i) => (
                <li key={i} className="text-xs text-muted-foreground leading-snug">• {reason}</li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: สร้าง `StatsTab.tsx`**

หนึ่งการ์ดต่อหนึ่งแถวใน `clip_analytics` (แถวเรียงใหม่ก่อนเก่า) แสดงเฉพาะฟิลด์ที่ `AnalyticsRepo.ListByClip` เลือกมาจริง — ฟิลด์อย่าง `engagement_rate` / `subscribers_gained` มีใน struct แต่ query ไม่ได้ดึงมา จึงเป็น 0 เสมอ ห้ามแสดง

```tsx
import type { ClipAnalyticsRow } from '../../api';

function Metric({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-lg bg-muted/40 px-3 py-2">
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className="text-sm font-semibold tabular-nums">{value}</div>
    </div>
  );
}

export function StatsTab({ rows }: { rows: ClipAnalyticsRow[] }) {
  if (rows.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        ยังไม่มีตัวเลข — คลิปที่ยังไม่เผยแพร่ หรือเพิ่งเผยแพร่แล้วยังไม่ถึงรอบดึงข้อมูล
      </p>
    );
  }

  return (
    <div className="space-y-3">
      {rows.map(r => (
        <div key={r.id} className="rounded-lg border p-3">
          <div className="flex items-center gap-2 mb-2 text-sm">
            <span className="font-medium">{r.platform}</span>
            {r.post_type && <span className="text-xs text-muted-foreground">{r.post_type}</span>}
            <span className="text-xs text-muted-foreground ml-auto">
              ดึงเมื่อ {new Date(r.fetched_at).toLocaleString('th-TH', { dateStyle: 'short', timeStyle: 'short' })}
            </span>
          </div>
          <div className="grid grid-cols-3 sm:grid-cols-6 gap-2">
            <Metric label="วิว" value={r.views.toLocaleString()} />
            <Metric label="ไลก์" value={r.likes.toLocaleString()} />
            <Metric label="คอมเมนต์" value={r.comments.toLocaleString()} />
            <Metric label="แชร์" value={r.shares.toLocaleString()} />
            <Metric label="เวลาดูรวม" value={`${Math.round(r.watch_time_seconds).toLocaleString()} วิ`} />
            <Metric label="retention" value={`${(r.retention_rate * 100).toFixed(1)}%`} />
          </div>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 3: สร้าง `PublishTab.tsx`**

```tsx
import type { ClipFull, ClipMetadata } from '../../api';
import { ExternalLink } from 'lucide-react';

function Row({ label, value }: { label: string; value: string | null | undefined }) {
  if (!value) return null;
  return (
    <div className="flex gap-3 py-2 border-b last:border-0 text-sm">
      <span className="text-muted-foreground w-32 shrink-0">{label}</span>
      <span className="min-w-0 break-words whitespace-pre-wrap">{value}</span>
    </div>
  );
}

export function PublishTab({ clip, metadata }: { clip: ClipFull; metadata: ClipMetadata | null }) {
  if (!metadata) {
    return (
      <p className="text-sm text-muted-foreground">
        ยังไม่มีข้อมูลการเผยแพร่ — คลิปนี้ยังไม่ถูก publish
      </p>
    );
  }

  const tags = metadata.youtube_tags ?? [];

  return (
    <div>
      {metadata.youtube_video_id && (
        <a
          href={`https://www.youtube.com/watch?v=${metadata.youtube_video_id}`}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1.5 text-sm text-primary hover:underline mb-3"
        >
          <ExternalLink className="size-3.5" /> เปิดคลิปบน YouTube
        </a>
      )}

      <Row label="ชื่อบน YouTube" value={metadata.youtube_title} />
      <Row label="คำบรรยาย" value={metadata.youtube_description} />
      {tags.length > 0 && (
        <div className="flex gap-3 py-2 border-b text-sm">
          <span className="text-muted-foreground w-32 shrink-0">แท็ก</span>
          <div className="flex flex-wrap gap-1 min-w-0">
            {tags.map(t => (
              <span key={t} className="text-xs bg-muted rounded px-1.5 py-0.5">{t}</span>
            ))}
          </div>
        </div>
      )}
      <Row label="เผยแพร่เมื่อ" value={clip.publish_date} />
      <Row label="YouTube video id" value={metadata.youtube_video_id} />
      <Row label="TikTok post id" value={metadata.tiktok_post_id} />
      <Row label="Zernio post id" value={metadata.zernio_post_id} />
      <Row label="Zernio shorts id" value={metadata.zernio_shorts_post_id} />
      <Row label="Zernio TikTok id" value={metadata.zernio_tiktok_post_id} />
    </div>
  );
}
```

- [ ] **Step 4: เสียบทั้ง 3 แท็บเข้าหน้าหลัก**

```tsx
import { QaTab } from '../components/clip-detail/QaTab';
import { StatsTab } from '../components/clip-detail/StatsTab';
import { PublishTab } from '../components/clip-detail/PublishTab';
```
```tsx
          <TabsContent value="qa">
            <QaTab qa={data.visual_qa} critique={data.critique} autoReview={data.auto_review} />
          </TabsContent>
          <TabsContent value="stats"><StatsTab rows={data.analytics} /></TabsContent>
          <TabsContent value="publish"><PublishTab clip={clip} metadata={data.metadata} /></TabsContent>
```

- [ ] **Step 5: Build ผ่าน**

Run: `cd frontend && npm run build`
Expected: build สำเร็จ

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/clip-detail/ frontend/src/pages/ClipDetail.tsx
git commit -m "feat(ui): แท็บ QA/ตัวเลข/เผยแพร่ พร้อมลิงก์ไปดูคลิปจริงบน YouTube"
```

---

### Task 7: ปุ่มแอ็กชัน + ลบ ReviewDialog

ย้ายแอ็กชันทั้งหมดจาก `ReviewDialog` มาไว้ใต้วิดีโอในหน้าใหม่ แล้วลบ dialog ทิ้งพร้อม state ที่ค้างใน `Content.tsx`

**Files:**
- Modify: `frontend/src/pages/ClipDetail.tsx`
- Modify: `frontend/src/pages/Content.tsx` (ลบ state `reviewClip` + import `ReviewDialog` + บล็อกท้ายไฟล์)
- Delete: `frontend/src/components/ReviewDialog.tsx`

**Interfaces:**
- Consumes: `apiFetch` จาก `api.ts`, `useToast` จาก `components/ui/toaster`
- Produces: ไม่มี export ใหม่

- [ ] **Step 1: เพิ่มแอ็กชันใน `ClipDetail.tsx`**

เพิ่ม import 1 บรรทัด:

```tsx
import { useToast } from '../components/ui/toaster';
```

และ**แก้ 3 บรรทัด import ที่ Task 3 เขียนไว้แล้ว** ให้รวมของใหม่เข้าไปในบรรทัดเดิม
(อย่าเพิ่ม import ซ้ำจากโมดูลเดียวกัน — ESLint จะเตือน):

```tsx
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch, getClipDetail } from '../api';
import { ArrowLeft, CheckCircle2, Loader2, Lock, Trash2, VideoOff, X } from 'lucide-react';
```

เพิ่ม state และฟังก์ชันในตัว component (วางถัดจาก `const [videoError, setVideoError] = useState(false);`):

```tsx
  const queryClient = useQueryClient();
  const { success, error: showError } = useToast();
  const [acting, setActing] = useState<'approve' | 'reject' | 'delete' | null>(null);
```

วางฟังก์ชันเหล่านี้ก่อน `return` (หลังบล็อก early-return ของ loading/error — ต้องอยู่หลัง `const { clip } = data;` เพราะใช้ `clip`):

```tsx
  // แอ็กชันทั้งสามหมุนรอบเดียวกัน: ล็อกปุ่ม → ยิง API → รีเฟรชรายการคลิป →
  // toast → กลับหน้าตารางถ้าคลิปถูกลบไปแล้ว
  async function runAction(
    kind: 'approve' | 'reject' | 'delete',
    fn: () => Promise<unknown>,
    successMsg: string,
    leavePage: boolean,
  ): Promise<void> {
    setActing(kind);
    try {
      await fn();
      queryClient.invalidateQueries({ queryKey: ['clips'] });
      queryClient.invalidateQueries({ queryKey: ['clip-detail', id] });
      success(successMsg);
      if (leavePage) navigate('/');
    } catch (e) {
      showError(`ไม่สำเร็จ: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setActing(null);
    }
  }

  function handleApprove(): void {
    // คลิปที่ถูกกักเป็น 'ready' อยู่แล้ว — อนุมัติคือปลดล็อกให้ publisher หยิบไปได้
    // ส่วนคลิป needs_review ต้องเลื่อนสถานะเป็น ready
    runAction(
      'approve',
      () => held
        ? apiFetch(`/api/v1/clips/${clip.id}/unhold`, { method: 'POST' })
        : apiFetch(`/api/v1/clips/${clip.id}`, {
            method: 'PATCH',
            body: JSON.stringify({ status: 'ready' }),
          }),
      held ? 'Override แล้ว — คลิปพร้อม publish รอบถัดไป' : 'อนุมัติแล้ว — คลิปพร้อม publish',
      false,
    );
  }

  function handleDelete(kind: 'reject' | 'delete'): void {
    const label = kind === 'reject' ? 'ตีกลับและลบคลิปนี้?' : 'ลบคลิปนี้?';
    if (!window.confirm(`${label}\n\n"${clip.title}"`)) return;
    runAction(
      kind,
      () => apiFetch(`/api/v1/clips/${clip.id}`, { method: 'DELETE' }),
      kind === 'reject' ? 'ตีกลับและลบคลิปแล้ว' : 'ลบคลิปแล้ว',
      true,
    );
  }
```

ประกาศ `reviewable` ถัดจาก `held`:

```tsx
  const reviewable = clip.status === 'needs_review';
```

เพิ่มปุ่มลบที่ header — แทรกต่อจาก `<StatusBadge status={clip.status} />` และ badge "ถูกกัก QA":

```tsx
          <Button
            variant="ghost"
            size="icon"
            className="size-8 text-muted-foreground hover:text-destructive"
            onClick={() => handleDelete('delete')}
            disabled={acting !== null}
            title="ลบคลิป"
          >
            {acting === 'delete' ? <Loader2 className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
          </Button>
```

เพิ่มปุ่มอนุมัติ/ตีกลับใต้วิดีโอ — วางต่อจากบล็อก video/placeholder ภายใน `<div className="sm:sticky sm:top-4">`:

```tsx
          {(reviewable || held) && (
            <div className="flex flex-col gap-2 mt-3">
              <Button onClick={handleApprove} disabled={acting !== null} size="sm">
                {acting === 'approve' ? <Loader2 className="size-4 animate-spin" /> : <CheckCircle2 className="size-4" />}
                {held ? 'Override — publish ทั้งที่มีตำหนิ' : 'อนุมัติ — พร้อม publish'}
              </Button>
              <Button variant="destructive" size="sm" onClick={() => handleDelete('reject')} disabled={acting !== null}>
                {acting === 'reject' ? <Loader2 className="size-4 animate-spin" /> : <X className="size-4" />}
                ตีกลับ (ลบ)
              </Button>
            </div>
          )}
```

- [ ] **Step 2: ล้าง `Content.tsx`**

ลบทั้งสามจุด:

1. บรรทัด 17: `import { ReviewDialog } from '../components/ReviewDialog';`
2. บรรทัด 82: `const [reviewClip, setReviewClip] = useState<Clip | null>(null);`
3. บล็อกท้ายไฟล์ (บรรทัด 542-544):

```tsx
      {reviewClip && (
        <ReviewDialog clip={reviewClip} onClose={() => setReviewClip(null)} />
      )}
```

- [ ] **Step 3: ลบไฟล์ dialog**

```bash
git rm frontend/src/components/ReviewDialog.tsx
```

- [ ] **Step 4: ยืนยันว่าไม่มีใครอ้างถึงแล้ว**

Run: `grep -rn "ReviewDialog" frontend/src/`
Expected: ไม่มีผลลัพธ์

- [ ] **Step 5: Build ผ่าน**

Run: `cd frontend && npm run build`
Expected: build สำเร็จ ไม่มี TypeScript error และไม่มี warning เรื่อง import ที่ไม่ได้ใช้

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/ClipDetail.tsx frontend/src/pages/Content.tsx
git commit -m "feat(ui): ย้ายปุ่มอนุมัติ/ตีกลับ/override มาหน้ารายละเอียด แล้วลบ ReviewDialog"
```

---

### Task 8: ตรวจของจริงด้วยตา

ไม่มี test runner ฝั่ง frontend งานนี้จึงเป็นการยืนยันด้วยการเปิดหน้าจริงกับข้อมูลบน prod (frontend รันในเครื่อง ชี้ API ไปที่ prod ตามค่า default ใน `api.ts:1`)

**Files:** ไม่มีการแก้ไฟล์ — ถ้าเจอปัญหาให้แก้แล้ว commit เพิ่ม

- [ ] **Step 1: รันทั้งสองฝั่ง**

Run: `cd frontend && npm run dev`
เปิดเบราว์เซอร์ไปที่ URL ที่ vite พิมพ์ออกมา

- [ ] **Step 2: เช็ครายการต่อไปนี้ทีละข้อ**

- คลิปสถานะ `published` — กดเข้าไปแล้วเห็นครบ 6 แท็บ, แท็บ "เผยแพร่" มีลิงก์ YouTube ที่กดแล้วเปิดคลิปจริง, แท็บ "ตัวเลข" มีวิว/retention
- คลิปสถานะ `needs_review` — มีปุ่มอนุมัติกับตีกลับใต้วิดีโอ, แท็บ QA แสดง scene ที่มีปัญหา
- คลิปสถานะ `ready` ที่ถูกกัก QA — ปุ่มขึ้นว่า "Override — publish ทั้งที่มีตำหนิ"
- คลิปสถานะ `failed` — แท็บภาพรวมแสดง `สาเหตุที่ล้มเหลว` และไม่มีปุ่มอนุมัติ
- คลิปเก่าที่วิดีโอหมดอายุ — เห็นกล่อง "วิดีโอหมดอายุแล้ว" ไม่ใช่จอขาว
- แท็บฉาก — เห็นภาพรายฉาก, ความยาววินาที, บทพากย์
- กด "กลับ" แล้วได้หน้าตารางเหมือนเดิม
- กดปุ่มถังขยะในตาราง แล้วต้อง **ไม่** เด้งเข้าหน้ารายละเอียด (ต้องเด้ง confirm ลบเท่านั้น)
- เปิด DevTools console — ต้องไม่มี error สีแดง

- [ ] **Step 3: ถ้าทุกข้อผ่าน — รายงานผลให้ผู้ใช้พร้อมบอกว่าข้อไหนตรวจแล้ว**

ถ้าข้อไหนไม่ผ่าน ให้แก้แล้ว commit เพิ่ม ห้ามข้ามไปบอกว่าเสร็จ

---

## หมายเหตุสำหรับคนทำ

- **endpoint เดิม 4 ตัวไม่ถูกลบ** (`/visual-qa`, `/scenes`, `/critique`, `/auto-review`, `/analytics`) — ยังมีที่อื่นเรียกอยู่ และการลบไม่ได้ช่วยอะไร
- **`clipMode` ไม่ใช่ตัวกำหนดหน้าตาคลิป — preset ต่างหาก** (บทเรียนเก่าในโปรเจกต์) งานนี้ไม่แตะ pipeline การผลิตเลย จึงไม่กระทบ
- **ห้ามยิง `/orchestrator/produce*` เพื่อทดสอบ** — endpoint พวกนี้ผลิตคลิปจริงบน prod ทันที งานนี้ไม่ต้องใช้
