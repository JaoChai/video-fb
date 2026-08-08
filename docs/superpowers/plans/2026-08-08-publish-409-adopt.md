# Publish 409 Adopt & Timeout Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** กันเหตุ publish deadlock 08-08 — Zernio POST ช้ากว่า timeout → คลิปขึ้น YouTube แล้วแต่ DB ไม่รู้ → 409 ขวางหัวคิวทั้งระบบ

**Architecture:** แก้ 4 จุดใน `internal/publisher` — (1) client แยกสำหรับ Post timeout 5 นาที (2) idempotency ผ่าน header `x-request-id` + รองรับ `existingPost` replay (3) 409 เป็น typed error + `GetPost` เช็คสถานะแล้ว adopt โพสต์เดิมเป็นความสำเร็จ (4) เรียงคิวให้คลิปที่เคยส่งพลาดไปท้าย กัน head-of-line blocking

**Tech Stack:** Go 1.x, httptest (แพทเทิร์นเดียวกับ `zernio_test.go` เดิม), ไม่เพิ่ม dependency ใหม่ (ห้ามเพิ่ม uuid lib — สร้าง request id จาก sha1 เอง)

## Global Constraints

- ห้ามเพิ่ม dependency ใหม่ใน `go.mod`
- ห้ามแตะ logic การ sanitize / firstComment / TikTok ที่มีอยู่
- ทุก struct field ใหม่ที่ optional ต้อง `omitempty` หรือ `json:"-"` ตามแบบแผนไฟล์เดิม
- comment ภาษาไทยตามสไตล์ไฟล์เดิม — เขียนเฉพาะ constraint ที่โค้ดบอกเองไม่ได้
- verify ทั้งชุด: `go build ./... && go vet ./internal/publisher/ && go test ./internal/publisher/`
- อ้างอิงพฤติกรรม Zernio (วัดจริง + docs.zernio.com/guides/idempotency):
  - POST `publishNow:true` block ~70-90 วิ (เกิน 60 วิที่เคยตั้ง)
  - retry ภายใน ~5 นาทีด้วย `x-request-id` เดิม → 200 พร้อม `existingPost` แทนการสร้างซ้ำ
  - content-hash ซ้ำใน 24 ชม. → 409 body: `{"error":"...","details":{"accountId":"...","platform":"youtube","existingPostId":"..."}}` (docs แสดง `existingPostId` ที่ top-level ด้วย — parse ทั้งสองที่)
  - `GET /posts/{id}` → `{"post":{"_id":"...","status":"scheduled|publishing|published|failed|partial",...}}`

---

### Task 1: แยก http client ของ Post — timeout 5 นาที

**Files:**
- Modify: `internal/publisher/zernio.go` (struct `ZernioClient`, `NewZernioClient`, `Post`)
- Test: `internal/publisher/zernio_test.go`

**Interfaces:**
- Consumes: —
- Produces: field `postClient *http.Client` ใน `ZernioClient` (Task 2-3 ไม่แตะ field นี้ แต่ struct literal ในเทสต์ที่สร้างเองต้องยังใช้ได้เพราะมี nil-fallback)

- [ ] **Step 1: เขียนเทสต์ fail**

เพิ่มใน `internal/publisher/zernio_test.go`:

```go
func TestNewZernioClient_PostClientTimeout(t *testing.T) {
	z := NewZernioClient("k", nil)
	if z.postClient == nil {
		t.Fatal("postClient must be initialized")
	}
	if z.postClient.Timeout != 5*time.Minute {
		t.Fatalf("postClient timeout = %v, want 5m", z.postClient.Timeout)
	}
}

// struct literal ที่ไม่ตั้ง postClient (แพทเทิร์นเทสต์เดิมทั้งไฟล์) ต้องยังใช้ Post ได้
func TestPost_NilPostClientFallsBackToClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"post":{"_id":"p1"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	resp, err := z.Post(context.Background(), PostRequest{Content: "x"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if resp.Post.ID != "p1" {
		t.Fatalf("got %q", resp.Post.ID)
	}
}
```

- [ ] **Step 2: รันให้เห็นว่า fail**

Run: `go test ./internal/publisher/ -run 'TestNewZernioClient_PostClientTimeout|TestPost_NilPostClientFallsBackToClient' -v`
Expected: FAIL (undefined field `postClient`)

- [ ] **Step 3: implement**

ใน `zernio.go`:

```go
type ZernioClient struct {
	fallbackKey string
	pool        *pgxpool.Pool
	client      *http.Client
	// postClient ใช้เฉพาะ Post — Zernio publishNow ตอบหลัง publish เสร็จ (~70-90 วิ วัดจริง
	// 08-08) 60 วิของ client ปกติจึงตัดก่อนได้ post ID กลับมา = คลิปขึ้นแล้วแต่ DB ไม่รู้
	postClient *http.Client
	baseURL    string
}

func NewZernioClient(fallbackKey string, pool *pgxpool.Pool) *ZernioClient {
	return &ZernioClient{
		fallbackKey: fallbackKey,
		pool:        pool,
		client:      &http.Client{Timeout: 60 * time.Second},
		postClient:  &http.Client{Timeout: 5 * time.Minute},
		baseURL:     zernioAPI,
	}
}
```

ใน `Post` เปลี่ยนบรรทัด `resp, err := z.client.Do(httpReq)` เป็น:

```go
	postClient := z.postClient
	if postClient == nil {
		postClient = z.client
	}
	resp, err := postClient.Do(httpReq)
```

- [ ] **Step 4: รันเทสต์ผ่าน**

Run: `go test ./internal/publisher/ -run 'TestNewZernioClient_PostClientTimeout|TestPost_NilPostClientFallsBackToClient' -v` แล้วตามด้วย `go build ./... && go vet ./internal/publisher/ && go test ./internal/publisher/`
Expected: PASS ทั้งหมด

- [ ] **Step 5: หยุด — ห้าม commit (Claude commit เอง)** รายงานตามสัญญาท้าย brief

### Task 2: x-request-id deterministic + รองรับ existingPost replay

**Files:**
- Modify: `internal/publisher/zernio.go` (`PostRequest`, `PostResponse`, `Post`)
- Modify: `internal/publisher/publisher.go` (จุดสร้าง `PostRequest` ทั้ง 2 จุดใน `PublishReady`)
- Test: `internal/publisher/zernio_test.go`

**Interfaces:**
- Consumes: —
- Produces: `PostRequest.RequestID string` (ส่งเป็น header ไม่ใช่ body) · ฟังก์ชัน `postRequestID(clipID, format string) string` ใน `publisher.go`

- [ ] **Step 1: เขียนเทสต์ fail**

เพิ่มใน `zernio_test.go`:

```go
func TestPost_SendsXRequestIDHeader(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("x-request-id")
		w.Write([]byte(`{"post":{"_id":"p1"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	if _, err := z.Post(context.Background(), PostRequest{Content: "x", RequestID: "rid-1"}); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if got != "rid-1" {
		t.Fatalf("x-request-id = %q, want rid-1", got)
	}
}

func TestPost_AdoptsExistingPostOnReplay(t *testing.T) {
	// Zernio ตอบ 200 + existingPost เมื่อ x-request-id ซ้ำภายใน ~5 นาที (docs/guides/idempotency)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"existingPost":{"_id":"orig-1"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	resp, err := z.Post(context.Background(), PostRequest{Content: "x", RequestID: "rid-1"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if resp.Post.ID != "orig-1" {
		t.Fatalf("Post.ID = %q, want orig-1", resp.Post.ID)
	}
}

func TestPostRequestID_DeterministicAndDistinct(t *testing.T) {
	a := postRequestID("clip-1", "916")
	b := postRequestID("clip-1", "916")
	c := postRequestID("clip-1", "169")
	if a != b {
		t.Fatalf("not deterministic: %q vs %q", a, b)
	}
	if a == c {
		t.Fatal("formats must produce distinct ids")
	}
	if len(a) != 36 {
		t.Fatalf("want uuid-shaped id, got %q", a)
	}
}
```

- [ ] **Step 2: รันให้เห็นว่า fail**

Run: `go test ./internal/publisher/ -run 'TestPost_SendsXRequestIDHeader|TestPost_AdoptsExistingPostOnReplay|TestPostRequestID_DeterministicAndDistinct' -v`
Expected: FAIL (undefined `RequestID`, `postRequestID`)

- [ ] **Step 3: implement**

ใน `zernio.go`:

```go
type PostRequest struct {
	Title          string           `json:"title,omitempty"`
	Content        string           `json:"content"`
	Platforms      []PlatformTarget `json:"platforms"`
	MediaItems     []MediaItem      `json:"mediaItems,omitempty"`
	IsDraft        bool             `json:"isDraft,omitempty"`
	PublishNow     bool             `json:"publishNow,omitempty"`
	Visibility     string           `json:"visibility,omitempty"`
	TikTokSettings *TikTokSettings  `json:"tiktokSettings,omitempty"`
	// RequestID ส่งเป็น header x-request-id (idempotency ของ Zernio) ไม่ใช่ body
	RequestID string `json:"-"`
}

type PostResponse struct {
	Post struct {
		ID string `json:"_id"`
	} `json:"post"`
	// ExistingPost คือคำตอบ replay เมื่อ x-request-id ซ้ำภายใน ~5 นาที — โพสต์เดิมที่สร้างไปแล้ว
	ExistingPost struct {
		ID string `json:"_id"`
	} `json:"existingPost"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}
```

ใน `Post` — หลัง `httpReq.Header.Set("Authorization", ...)`:

```go
	if req.RequestID != "" {
		httpReq.Header.Set("x-request-id", req.RequestID)
	}
```

และก่อนเช็ค `result.Post.ID == ""`:

```go
	if result.Post.ID == "" && result.ExistingPost.ID != "" {
		log.Printf("[zernio] replay detected, adopting existing post %s", result.ExistingPost.ID)
		result.Post.ID = result.ExistingPost.ID
	}
```

ใน `publisher.go` (import เพิ่ม `crypto/sha1`, `encoding/hex`):

```go
// postRequestID สร้าง id รูป UUID แบบ deterministic ต่อ (clip, format) — retry รอบถัดไป
// ของคลิปเดิมจึงส่ง x-request-id เดิมเสมอ ให้ Zernio จับ replay ได้แทนที่จะสร้างโพสต์ซ้ำ
func postRequestID(clipID, format string) string {
	sum := sha1.Sum([]byte("adsvance-publish:" + clipID + ":" + format))
	h := hex.EncodeToString(sum[:16])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
```

แล้วเติม `RequestID` ในทั้งสองจุดของ `PublishReady`:
- จุด 16:9: `RequestID: postRequestID(clipID, "169"),`
- จุด 9:16: `RequestID: postRequestID(clipID, "916"),`

- [ ] **Step 4: รันเทสต์ผ่าน**

Run: `go test ./internal/publisher/ -run 'TestPost_SendsXRequestIDHeader|TestPost_AdoptsExistingPostOnReplay|TestPostRequestID_DeterministicAndDistinct' -v` แล้ว `go build ./... && go vet ./internal/publisher/ && go test ./internal/publisher/`
Expected: PASS ทั้งหมด

- [ ] **Step 5: หยุด — ห้าม commit (Claude commit เอง)** รายงานตามสัญญาท้าย brief

### Task 3: 409 → typed error + GetPost + adopt เมื่อ published

**Files:**
- Modify: `internal/publisher/zernio.go` (`Post` 409 branch, เพิ่ม `DuplicatePostError`, `GetPost`)
- Modify: `internal/publisher/publisher.go` (`PublishReady` error branch ทั้ง 16:9 และ 9:16, เพิ่ม `adoptDuplicate`)
- Test: `internal/publisher/zernio_test.go`

**Interfaces:**
- Consumes: `PostResponse` จาก Task 2
- Produces: `type DuplicatePostError struct{ ExistingPostID string }` · `func (z *ZernioClient) GetPost(ctx context.Context, id string) (*PostStatus, error)` โดย `type PostStatus struct{ ID, Status string }` · `func (p *Publisher) adoptDuplicate(ctx context.Context, err error) (string, bool)`

- [ ] **Step 1: เขียนเทสต์ fail**

เพิ่มใน `zernio_test.go`:

```go
func TestPost_409ReturnsDuplicatePostError(t *testing.T) {
	// body จริงที่วัดได้ 08-08: existingPostId อยู่ใน details (docs แสดง top-level ด้วย)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"Duplicate","details":{"accountId":"a","platform":"youtube","existingPostId":"dup-1"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	_, err := z.Post(context.Background(), PostRequest{Content: "x"})
	var dup *DuplicatePostError
	if !errors.As(err, &dup) {
		t.Fatalf("want DuplicatePostError, got %v", err)
	}
	if dup.ExistingPostID != "dup-1" {
		t.Fatalf("ExistingPostID = %q", dup.ExistingPostID)
	}
}

func TestPost_409TopLevelExistingPostID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"Duplicate","existingPostId":"dup-2"}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	_, err := z.Post(context.Background(), PostRequest{Content: "x"})
	var dup *DuplicatePostError
	if !errors.As(err, &dup) || dup.ExistingPostID != "dup-2" {
		t.Fatalf("got %v", err)
	}
}

func TestGetPost_ReturnsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/posts/p9" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`{"post":{"_id":"p9","status":"published"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	ps, err := z.GetPost(context.Background(), "p9")
	if err != nil {
		t.Fatalf("GetPost: %v", err)
	}
	if ps.ID != "p9" || ps.Status != "published" {
		t.Fatalf("got %+v", ps)
	}
}

func TestAdoptDuplicate_PublishedAdopts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"post":{"_id":"dup-1","status":"published"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	p := &Publisher{zernio: z}
	id, ok := p.adoptDuplicate(context.Background(), &DuplicatePostError{ExistingPostID: "dup-1"})
	if !ok || id != "dup-1" {
		t.Fatalf("got id=%q ok=%v", id, ok)
	}
}

func TestAdoptDuplicate_NotPublishedDoesNotAdopt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"post":{"_id":"dup-1","status":"scheduled"}}`))
	}))
	defer srv.Close()
	z := &ZernioClient{fallbackKey: "k", client: srv.Client(), baseURL: srv.URL}
	p := &Publisher{zernio: z}
	if _, ok := p.adoptDuplicate(context.Background(), &DuplicatePostError{ExistingPostID: "dup-1"}); ok {
		t.Fatal("must not adopt a non-published post")
	}
}

func TestAdoptDuplicate_NonDuplicateError(t *testing.T) {
	p := &Publisher{}
	if _, ok := p.adoptDuplicate(context.Background(), errors.New("boom")); ok {
		t.Fatal("plain errors must not adopt")
	}
}
```

- [ ] **Step 2: รันให้เห็นว่า fail**

Run: `go test ./internal/publisher/ -run 'TestPost_409|TestGetPost_|TestAdoptDuplicate_' -v`
Expected: FAIL (undefined `DuplicatePostError`, `GetPost`, `adoptDuplicate`)

- [ ] **Step 3: implement**

ใน `zernio.go`:

```go
// DuplicatePostError คือ 409 ของ Zernio (content-hash ซ้ำใน 24 ชม.) — ไม่ใช่ความล้มเหลวจริง
// เสมอไป: โพสต์เดิมอาจขึ้น YouTube สำเร็จไปแล้วแต่เราไม่ได้ id กลับมา (timeout) ฝั่งเรียกต้อง
// เช็คสถานะโพสต์เดิมก่อนตัดสิน (เหตุจริง 08-08: คลิปขวางหัวคิว 20 รอบ)
type DuplicatePostError struct {
	ExistingPostID string
}

func (e *DuplicatePostError) Error() string {
	return fmt.Sprintf("zernio 409: duplicate of existing post %s", e.ExistingPostID)
}

type PostStatus struct {
	ID     string
	Status string
}

func (z *ZernioClient) GetPost(ctx context.Context, id string) (*PostStatus, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", z.baseURL+"/posts/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	apiKey := z.getAPIKey(ctx)
	if apiKey == "" {
		return nil, fmt.Errorf("zernio API key not configured")
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := z.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("get post: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("zernio %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}
	var parsed struct {
		Post struct {
			ID     string `json:"_id"`
			Status string `json:"status"`
		} `json:"post"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse post: %w", err)
	}
	return &PostStatus{ID: parsed.Post.ID, Status: parsed.Post.Status}, nil
}
```

ใน `Post` — แทนที่ block `if resp.StatusCode < 200 || resp.StatusCode >= 300 { ... }` ด้วย:

```go
	if resp.StatusCode == http.StatusConflict {
		var dup struct {
			ExistingPostID string `json:"existingPostId"`
			Details        struct {
				ExistingPostID string `json:"existingPostId"`
			} `json:"details"`
		}
		_ = json.Unmarshal(respBody, &dup)
		id := dup.Details.ExistingPostID
		if id == "" {
			id = dup.ExistingPostID
		}
		if id != "" {
			return nil, &DuplicatePostError{ExistingPostID: id}
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("zernio %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}
```

ใน `publisher.go`:

```go
// adoptDuplicate ตัดสิน 409: โพสต์เดิมขึ้นจริงแล้ว → คืน id ให้บันทึกเป็นสำเร็จ · ยังไม่ขึ้น
// (scheduled/publishing/failed) → ไม่ adopt ปล่อยให้วน retry ตามเดิม เพราะ mark published
// ทั้งที่ของจริงยังไม่ขึ้นคือการโกหก DB แบบเดียวกับที่เพิ่งแก้มา
func (p *Publisher) adoptDuplicate(ctx context.Context, cause error) (string, bool) {
	var dup *DuplicatePostError
	if !errors.As(cause, &dup) {
		return "", false
	}
	ps, err := p.zernio.GetPost(ctx, dup.ExistingPostID)
	if err != nil {
		log.Printf("adoptDuplicate: เช็คสถานะโพสต์ %s ไม่สำเร็จ: %v", dup.ExistingPostID, err)
		return "", false
	}
	if ps.Status != "published" {
		log.Printf("adoptDuplicate: โพสต์ %s สถานะ %s ยังไม่ adopt", ps.ID, ps.Status)
		return "", false
	}
	log.Printf("adoptDuplicate: โพสต์ %s ขึ้นแล้วจริง — บันทึกเป็นสำเร็จ", ps.ID)
	return ps.ID, true
}
```

(import เพิ่ม `errors` ใน publisher.go ถ้ายังไม่มี)

ใน `PublishReady` — จุด 16:9 แทน block `if err != nil { log...; recordPublishFailure; continue }` ด้วย:

```go
			if err != nil {
				if id, ok := p.adoptDuplicate(ctx, err); ok {
					mainPostID = id
				} else {
					log.Printf("Failed to post 16:9 for clip %s: %v", clipID, err)
					p.recordPublishFailure(ctx, clipID, err)
					continue
				}
			} else {
				log.Printf("Posted 16:9 public for clip %s → %s", clipID, result169.Post.ID)
				mainPostID = result169.Post.ID
			}
```

จุด 9:16 แทน block `if err != nil { log...; postErr = err } else { ... }` ด้วย:

```go
			if err != nil {
				if id, ok := p.adoptDuplicate(ctx, err); ok {
					shortsPostID = id
				} else {
					log.Printf("Failed to post 9:16 for clip %s: %v", clipID, err)
					postErr = err
				}
			} else {
				log.Printf("Posted 9:16 Shorts public for clip %s → %s", clipID, result916.Post.ID)
				shortsPostID = result916.Post.ID
			}
```

- [ ] **Step 4: รันเทสต์ผ่าน**

Run: `go test ./internal/publisher/ -run 'TestPost_409|TestGetPost_|TestAdoptDuplicate_' -v` แล้ว `go build ./... && go vet ./internal/publisher/ && go test ./internal/publisher/`
Expected: PASS ทั้งหมด

- [ ] **Step 5: หยุด — ห้าม commit (Claude commit เอง)** รายงานตามสัญญาท้าย brief

### Task 4: คลิปที่เคยส่งพลาดไปท้ายคิว

**Files:**
- Modify: `internal/publisher/publisher.go` (query ใน `PublishReady`)

**Interfaces:**
- Consumes: —
- Produces: — (เปลี่ยนเฉพาะ ORDER BY)

- [ ] **Step 1: แก้ query**

ใน `PublishReady` เปลี่ยน `ORDER BY c.publish_date ASC LIMIT 1` เป็น:

```sql
ORDER BY (c.fail_reason IS NOT NULL), c.publish_date ASC LIMIT 1
```

พร้อม comment ภาษาไทยเหนือ query อธิบายว่า: คลิปที่ส่งพลาดค้าง fail_reason จะถูกดันท้ายคิว
ให้คลิปดีได้คิวก่อน — กัน head-of-line blocking แบบเหตุ 08-08 (คลิปเสีย 1 ตัวกินคิวทุกรอบ
เพราะ LIMIT 1 + เรียงตามวันเก่าสุด)

- [ ] **Step 2: verify**

Run: `go build ./... && go vet ./internal/publisher/ && go test ./internal/publisher/`
Expected: PASS ทั้งหมด (query นี้ไม่มี unit test infra ของ DB — Claude ตรวจ diff เอง)

- [ ] **Step 3: หยุด — ห้าม commit (Claude commit เอง)** รายงานตามสัญญาท้าย brief
