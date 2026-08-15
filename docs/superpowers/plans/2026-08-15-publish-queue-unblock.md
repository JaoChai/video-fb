# Publish Queue Unblock Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** คลิปใบเดียวที่ส่งไม่ออกต้องไม่สามารถยึดหัวคิวรอบส่งได้อีก และคลิปที่ตะแกรงเนื้อหาตีตกต้องมีทางกู้ที่คนกดได้ พร้อมเหตุผลที่มองเห็นบนหน้าเว็บ

**Architecture:** สามชั้น (1) **คิวส่ง** — `PublishReady` ดึงผู้สมัคร 5 ใบแทน 1 ใบ กรองคลิปไร้ไฟล์วิดีโอออกตั้งแต่ SQL ลองส่งทีละใบจนสำเร็จ 1 ใบแล้วหยุด (drip 1 คลิป/รอบเท่าเดิม) และทุกเส้นทางที่ส่งไม่ออกต้องเขียน `fail_reason` เสมอ (2) **ประตูเข้าคิว** — ปุ่มอนุมัติ/unhold และ `PATCH status=ready` ปฏิเสธคลิปที่ยังไม่มีไฟล์วิดีโอ (3) **ทางกู้** — endpoint `POST /clips/{id}/rerender` สั่ง resume ที่ขั้น render ให้คลิปที่ตะแกรงเนื้อหาตีตก (มีสคริปต์/ฉาก/metadata ครบแล้ว) พร้อมเหตุผลของตะแกรงที่เขียนลง `fail_reason` ให้เห็นบนหน้าคลิป

**Tech Stack:** Go 1.x (stdlib + pgx/v5), PostgreSQL (Neon), Zernio API v1, React 19 + TypeScript + Vite (frontend), เทสต์ฝั่ง Go ด้วย `testing` + `net/http/httptest` · ฝั่ง frontend ไม่มี test runner — ตรวจด้วย `npm run build` (tsc) เท่านั้น

## Global Constraints

- **เหตุที่มาของแผนนี้ (อ่านก่อนเริ่ม):** 14 ส.ค. 2026 คลิป `afa5b69c` ถูก tutorial gate ตีตกก่อน render (ไม่มีไฟล์วิดีโอ) → auto-review กัก → คนกดปุ่ม "อนุมัติ" (unhold) ทำให้สถานะเป็น `ready` → รอบส่งซึ่งดึงคลิปเดียว (`LIMIT 1`) หยิบคลิปนี้ทุกรอบแล้ว log `No video published ... leaving as ready` โดยไม่เขียน `fail_reason` ⇒ กลไกดันท้ายคิว (`ORDER BY starts_with(fail_reason,'publish: ')`) มองไม่เห็นมัน ⇒ **คิวตัน 21 ชั่วโมง / 13 รอบ** จนคนลบคลิปทิ้งเอง
- **ห้ามเปลี่ยนพฤติกรรม drip:** รอบส่งยังต้องขึ้น YouTube **ไม่เกิน 1 คลิปต่อรอบ** เหมือนเดิม การดึงผู้สมัครมาหลายใบคือ "ตัวสำรองเมื่อใบแรกส่งไม่ออก" ไม่ใช่การส่งทีละหลายใบ
- **ห้ามให้ระบบ re-render คลิปที่ตะแกรงเนื้อหาตีตกโดยอัตโนมัติ:** `resumeHyperframesProduction` **ข้ามตะแกรง** (`tutorialGateFailure`/`mythGateFailure` อยู่ก่อน `renderAndFinalize`) — การกู้อัตโนมัติจะทำให้คลิปเนื้อหาผิดขึ้น YouTube จริง ทางกู้ต้องเป็น **คนกดเท่านั้น**
- **`advisory lock` เดิมของ `PublishReady` ต้องอยู่ครบทุกบรรทัด** (`publishLockKey`, `defer` unlock ด้วย `context.Background()`, log CRITICAL) — ห้าม refactor ทิ้ง
- **ห้ามแตะเส้นทาง Telegram:** `postTelegramForClip` / `sweepTelegramBacklog` ต้องถูกเรียกที่เดิม ด้วยเงื่อนไขเดิม
- ภาษาในคอมเมนต์โค้ด: **ไทย** ตามไฟล์เดิม · อธิบาย **ทำไม** ไม่ใช่ **อะไร**
- ห้ามรัน backend local ต่อ prod DB (scheduler จะยิง cron จริง) · ห้ามยิง `/orchestrator/produce*` เพื่อทดสอบ (มันผลิตคลิปจริงเสียเงินจริง)
- โปรเจกต์นี้ **ไม่มี integration test ที่ต่อ Postgres** ทุก assertion ต้องทำผ่าน pure function หรือ `httptest` — ห้ามเพิ่ม dependency ใหม่เพื่อทำ DB test
- คอมมิตทุก task · ข้อความคอมมิตภาษาไทย ขึ้นต้นด้วย `fix(publisher):` / `feat(orchestrator):` / `test(...)` / `feat(ui):`
- รัน `go build ./... && go test ./...` ให้เขียวก่อนคอมมิตทุกครั้ง
- ก่อนคอมมิตงานทั้งชุด (หลัง Task 7) ให้รัน `/simplify` กับ diff ตามธรรมเนียมของเจ้าของโปรเจกต์

## ค่าคงที่ / ชื่อที่ทุก task ต้องใช้ให้ตรงกัน

| ชื่อ | ค่า/ลายเซ็น | เกิดใน task |
|---|---|---|
| `publishFailPrefix` | `"publish: "` (มีอยู่แล้ว `publisher.go:25`) | — |
| `readyCandidateLimit` | `const readyCandidateLimit = 5` | 1 |
| `publishCandidate` | `struct{ ClipID, Title, Desc, Video169, Video916 string }` | 1 |
| `publishOpts` | `struct{ ytAccountID, firstComment, tgAccountID string }` | 1 |
| `publishFirst` | `func publishFirst(cands []publishCandidate, send func(publishCandidate) bool) string` | 1 |
| `noVideoErr` | `var noVideoErr = errors.New("ไม่มีไฟล์วิดีโอให้ส่ง")` | 2 |
| `publishFailure` | `func publishFailure(postErr error) error` | 2 |
| `stuckQueueMessage` | `func stuckQueueMessage(total, noMetadata, noVideo int) string` | 3 |
| `readyBlockedReason` | `func readyBlockedReason(c *models.Clip) string` | 4 |
| `ClearHeldFlag` | `func (r *ClipsRepo) ClearHeldFlag(ctx context.Context, id string) error` | 5 |
| `rerenderBlockedReason` | `func rerenderBlockedReason(c *models.Clip) string` | 5 |
| `ErrRerenderNotAllowed` | `var ErrRerenderNotAllowed = errors.New("rerender not allowed")` | 5 |
| `RerenderClip` | `func (o *Orchestrator) RerenderClip(ctx context.Context, id string) error` | 5 |
| `CanRerender` | `func (o *Orchestrator) CanRerender(ctx context.Context, id string) error` | 5 |
| `gateFailReason` | `func gateFailReason(gate, msg string) string` | 6 |
| route ใหม่ | `POST /api/v1/clips/{id}/rerender` | 5 |

---

## File Structure

| ไฟล์ | ความรับผิดชอบ | task |
|---|---|---|
| `internal/publisher/publisher.go` (แก้) | คิวส่ง: query ผู้สมัคร, `publishFirst`, `publishOne`, บันทึกเหตุผล, log คิวตัน | 1, 2, 3 |
| `internal/publisher/queue_test.go` (สร้าง) | เทสต์ pure ของคิวส่งทั้งหมด (`publishFirst`, `publishFailure`, `stuckQueueMessage`, รูปร่าง SQL) | 1, 2, 3 |
| `internal/handler/clips.go` (แก้) | ประตูเข้าคิว: guard ของ `Unhold` และ `Update(status=ready)` | 4 |
| `internal/handler/clips_ready_guard_test.go` (สร้าง) | เทสต์ `readyBlockedReason` | 4 |
| `internal/repository/clips.go` (แก้) | `ClearHeldFlag` (เคลียร์ธงอย่างเดียว ไม่แตะสถานะ) | 5 |
| `internal/orchestrator/orchestrator.go` (แก้) | `RerenderClip`, `rerenderBlockedReason`, `gateFailReason` ใน `blockForReview` | 5, 6 |
| `internal/orchestrator/rerender_test.go` (สร้าง) | เทสต์ `rerenderBlockedReason` + `gateFailReason` | 5, 6 |
| `internal/handler/orchestrator.go` (แก้) | handler `RerenderClip` | 5 |
| `internal/router/router.go` (แก้) | route `POST /api/v1/clips/{id}/rerender` | 5 |
| `frontend/src/pages/ClipDetail.tsx` (แก้) | ปุ่มแยกสองกรณี: มีวิดีโอ → อนุมัติ · ไม่มีวิดีโอ → สั่งเรนเดอร์ | 7 |

**ลำดับที่ต้องทำ:** 1 → 2 → 3 (แตะไฟล์เดียวกัน ต่อยอดกัน) → 4 → 5 → 6 → 7 (5 ต้องมาก่อน 7 เพราะ UI เรียก endpoint ของ 5)

---

## Task 1: คิวส่งดึงผู้สมัครหลายใบ แล้วข้ามใบที่ส่งไม่ออก

**Files:**
- Modify: `internal/publisher/publisher.go:119-310` (ทั้ง `PublishReady`)
- Test: `internal/publisher/queue_test.go` (สร้าง)

**Interfaces:**
- Consumes: `publishFailPrefix` (`publisher.go:25`), `p.pool`, `p.zernio`, `p.recordPublished`, `p.recordPublishFailure`, `p.clearPublishFailure`, `p.adoptDuplicate`, `p.postTelegramForClip`, `p.sweepTelegramBacklog`, `isContactInfo`, `sanitizeYouTubeText`, `youtubePlatforms`, `postRequestID`
- Produces: `readyCandidateLimit`, `readyClipsQuery`, `publishCandidate`, `publishOpts`, `publishFirst(cands, send) string`, `(p *Publisher) readyCandidates(ctx) ([]publishCandidate, error)`, `(p *Publisher) publishOne(ctx, c, o) bool`

**บริบทสำหรับคนที่ไม่เคยเห็นไฟล์นี้:** `PublishReady` คือรอบส่งคลิปขึ้น YouTube ผ่าน Zernio ถูกเรียกจาก 3 ทาง — schedule `Publish Ready` (ทุกชั่วโมงนาที :30), ท้ายรอบผลิตทุกรอบ, และปุ่ม publish บนหน้าเว็บ (`POST /api/v1/orchestrator/publish`) โครงเดิมคือ `SELECT ... LIMIT 1` แล้ว `for rows.Next()` — ลูปที่วนได้ครั้งเดียวเสมอ Task นี้เปลี่ยนเป็นดึง 5 ใบแล้วลองทีละใบ **หยุดทันทีที่ใบหนึ่งขึ้นสำเร็จ**

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

สร้าง `internal/publisher/queue_test.go`:

```go
package publisher

import (
	"strings"
	"testing"
)

// หัวใจของเหตุ 2026-08-14: คลิปที่ส่งไม่ออกต้องไม่ขวางใบถัดไป
func TestPublishFirst_SkipsFailingCandidateAndPublishesNext(t *testing.T) {
	cands := []publishCandidate{{ClipID: "เสีย"}, {ClipID: "ดี"}, {ClipID: "ไม่ควรถูกแตะ"}}
	var tried []string
	got := publishFirst(cands, func(c publishCandidate) bool {
		tried = append(tried, c.ClipID)
		return c.ClipID == "ดี"
	})
	if got != "ดี" {
		t.Errorf("publishFirst() = %q, want %q", got, "ดี")
	}
	want := []string{"เสีย", "ดี"}
	if strings.Join(tried, ",") != strings.Join(want, ",") {
		t.Errorf("ลองส่ง %v, want %v (ต้องหยุดทันทีที่สำเร็จ)", tried, want)
	}
}

func TestPublishFirst_NoCandidateSucceeds(t *testing.T) {
	cands := []publishCandidate{{ClipID: "a"}, {ClipID: "b"}}
	calls := 0
	got := publishFirst(cands, func(publishCandidate) bool { calls++; return false })
	if got != "" {
		t.Errorf("publishFirst() = %q, want \"\"", got)
	}
	if calls != 2 {
		t.Errorf("เรียก send %d ครั้ง, want 2 (ต้องลองครบทุกใบก่อนยอมแพ้)", calls)
	}
}

func TestPublishFirst_EmptyList(t *testing.T) {
	if got := publishFirst(nil, func(publishCandidate) bool { return true }); got != "" {
		t.Errorf("publishFirst(nil) = %q, want \"\"", got)
	}
}

// ล็อกเจตนาของ SQL ไว้ด้วยการอ่านสตริง เพราะโปรเจกต์นี้ไม่มี integration test ต่อ Postgres
// สามอย่างนี้คือสิ่งที่ถ้าหลุดไปแล้วคิวจะกลับไปตันแบบเดิมโดยไม่มีเทสต์ไหนร้อง
func TestReadyClipsQuery_KeepsAntiBlockingGuards(t *testing.T) {
	for _, want := range []string{
		"starts_with(c.fail_reason",              // ดันคลิปที่ส่งพลาดไปท้ายคิว
		"COALESCE(c.video_9_16_url, '') <> ''",   // คลิปไร้ไฟล์วิดีโอไม่เข้าคิวตั้งแต่ต้น
		"LIMIT $2",                                // ผู้สมัครหลายใบ ไม่ใช่ LIMIT 1
	} {
		if !strings.Contains(readyClipsQuery, want) {
			t.Errorf("readyClipsQuery ขาด %q", want)
		}
	}
	if strings.Contains(readyClipsQuery, "LIMIT 1") {
		t.Error("readyClipsQuery ยังเป็น LIMIT 1 — คลิปใบเดียวจะยึดหัวคิวได้อีก")
	}
}

func TestPublishCandidate_HasVideo(t *testing.T) {
	cases := []struct {
		name string
		c    publishCandidate
		want bool
	}{
		{"ไม่มีทั้งคู่", publishCandidate{}, false},
		{"มี 9:16", publishCandidate{Video916: "https://cdn/x.mp4"}, true},
		{"มี 16:9", publishCandidate{Video169: "https://cdn/x.mp4"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.c.hasVideo(); got != c.want {
				t.Errorf("hasVideo() = %v, want %v", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

รัน: `go test ./internal/publisher/ -run 'TestPublishFirst|TestReadyClipsQuery|TestPublishCandidate' -v`
คาดหวัง: คอมไพล์ไม่ผ่าน — `undefined: publishCandidate`, `undefined: publishFirst`, `undefined: readyClipsQuery`

- [ ] **Step 3: เพิ่มชนิดข้อมูล + query + `publishFirst`**

แทรกเหนือ `func (p *Publisher) PublishReady` ใน `internal/publisher/publisher.go` (ใต้ `const publishLockKey`):

```go
// รอบส่งดึงผู้สมัครมาหลายใบแทนที่จะดึงใบเดียว · ยังขึ้น YouTube ไม่เกิน 1 คลิปต่อรอบเหมือนเดิม
// (drip) — ตัวสำรองมีไว้เพื่อให้คลิปที่ส่งไม่ออกใบหนึ่ง "ข้ามได้" ไม่ใช่เพื่อส่งทีละหลายใบ
// เหตุ 2026-08-14: คลิปไร้ไฟล์วิดีโอใบเดียวยึดหัวคิวทุกรอบนาน 21 ชั่วโมง เพราะ LIMIT 1
// บวกกับเส้นทางที่ไม่เขียน fail_reason ทำให้กลไกดันท้ายคิวมองไม่เห็นมัน
const readyCandidateLimit = 5

// เงื่อนไขวิดีโอใน WHERE คือด่านที่สอง (ด่านแรกคือปุ่มอนุมัติที่ไม่ปล่อยคลิปไร้ไฟล์เข้าสถานะ
// ready) · คลิปที่ไม่มีไฟล์ให้ส่งไม่ควรกินคิวตั้งแต่แรก ไม่ใช่แค่ถูกดันไปท้ายคิว
const readyClipsQuery = `
	SELECT c.id, cm.youtube_title, cm.youtube_description, c.video_16_9_url, c.video_9_16_url
	FROM clips c
	JOIN clip_metadata cm ON c.id = cm.clip_id
	WHERE c.status = 'ready' AND c.auto_review_held = FALSE AND c.publish_date <= CURRENT_DATE
	  AND (COALESCE(c.video_16_9_url, '') <> '' OR COALESCE(c.video_9_16_url, '') <> '')
	ORDER BY (c.fail_reason IS NOT NULL AND starts_with(c.fail_reason, $1)), c.publish_date ASC
	LIMIT $2`

// publishCandidate คือคลิปหนึ่งใบที่รอบส่งจะลองส่ง — อ่านจาก DB ให้ครบก่อนแล้วปิด rows
// เพื่อไม่ถือ connection ไว้ตลอดการอัปโหลดวิดีโอ (โพสต์หนึ่งใบใช้เวลาได้ถึงหลักนาที)
type publishCandidate struct {
	ClipID   string
	Title    string
	Desc     string
	Video169 string
	Video916 string
}

func (c publishCandidate) hasVideo() bool { return c.Video169 != "" || c.Video916 != "" }

// publishOpts คือค่าที่อ่านครั้งเดียวต่อรอบแล้วใช้ร่วมกันทุกใบ (ไม่ query ซ้ำต่อผู้สมัคร)
type publishOpts struct {
	ytAccountID  string
	firstComment string
	tgAccountID  string
}

// publishFirst ลองส่งทีละใบตามลำดับคิว หยุดทันทีที่ใบหนึ่งขึ้นสำเร็จ แล้วคืน id ของใบนั้น
// ("" = รอบนี้ไม่มีใบไหนขึ้นเลย) · แยกออกจาก I/O เพื่อให้ทดสอบพฤติกรรม "ใบที่ส่งไม่ออก
// ต้องไม่ขวางใบถัดไป" ได้จริงในโปรเจกต์ที่ไม่มี integration test ต่อ Postgres
func publishFirst(cands []publishCandidate, send func(publishCandidate) bool) string {
	for _, c := range cands {
		if send(c) {
			return c.ClipID
		}
	}
	return ""
}

func textOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// readyCandidates อ่านผู้สมัครของรอบนี้ให้ครบแล้วปิด rows ทันที
func (p *Publisher) readyCandidates(ctx context.Context) ([]publishCandidate, error) {
	rows, err := p.pool.Query(ctx, readyClipsQuery, publishFailPrefix, readyCandidateLimit)
	if err != nil {
		return nil, fmt.Errorf("query ready clips: %w", err)
	}
	defer rows.Close()

	var out []publishCandidate
	for rows.Next() {
		var c publishCandidate
		var desc, video169, video916 *string
		if err := rows.Scan(&c.ClipID, &c.Title, &desc, &video169, &video916); err != nil {
			return nil, fmt.Errorf("scan clip: %w", err)
		}
		c.Desc = textOrEmpty(desc)
		c.Video169 = textOrEmpty(video169)
		c.Video916 = textOrEmpty(video916)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ready clips: %w", err)
	}
	return out, nil
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

รัน: `go test ./internal/publisher/ -run 'TestPublishFirst|TestReadyClipsQuery|TestPublishCandidate' -v`
คาดหวัง: PASS ทั้ง 5 เทสต์

- [ ] **Step 5: ย้ายเนื้อลูปเดิมมาเป็น `publishOne`**

เพิ่มเมธอดนี้ใต้ `readyCandidates` — เนื้อในคือโค้ดเดิมจาก `publisher.go:180-302` ทุกบรรทัด เปลี่ยนแค่ `continue` เป็น `return false` และ `clipID` เป็น `c.ClipID`:

```go
// publishOne ส่งคลิปหนึ่งใบขึ้น YouTube · คืน true เมื่อคลิปขึ้นจริงและบันทึก published สำเร็จ
// false = คลิปยังคาสถานะ ready ให้รอบหน้าลองใหม่ และผู้เรียกจะไปลองใบถัดไปในคิวทันที
func (p *Publisher) publishOne(ctx context.Context, c publishCandidate, o publishOpts) bool {
	title := c.Title
	if isContactInfo(title) {
		var clipTitle string
		if err := p.pool.QueryRow(ctx, `SELECT title FROM clips WHERE id = $1`, c.ClipID).Scan(&clipTitle); err == nil && clipTitle != "" {
			log.Printf("Title validation: '%s' looks like contact info, using clip question instead", title)
			title = clipTitle
		}
	}

	// แปลงครั้งเดียวตรงนี้ให้ครอบทั้งโพสต์ 16:9 และ 9:16 (shortsTitle ต่อยอดจาก title)
	title = sanitizeYouTubeText(title)
	desc := sanitizeYouTubeText(c.Desc)

	if o.ytAccountID == "" {
		log.Printf("No Zernio accounts configured, skipping clip %s", c.ClipID)
		return false
	}

	// Post whatever formats this clip actually has. The hyperframes pipeline
	// produces 9:16 only; the legacy pipeline produced 16:9 (+ a 9:16 Short).
	var mainPostID, shortsPostID string
	var postErr error

	if c.Video169 != "" {
		result169, err := p.zernio.Post(ctx, PostRequest{
			Title:      title,
			Content:    title + "\n\n" + desc,
			Platforms:  youtubePlatforms(o.ytAccountID, title, o.firstComment, VisibilityPublic),
			MediaItems: []MediaItem{{Type: "video", URL: c.Video169}},
			Visibility: VisibilityPublic,
			PublishNow: true,
			RequestID:  postRequestID(c.ClipID, "169"),
		})
		if err != nil {
			if id, ok := p.adoptDuplicate(ctx, err); ok {
				mainPostID = id
			} else {
				log.Printf("Failed to post 16:9 for clip %s: %v", c.ClipID, err)
				p.recordPublishFailure(ctx, c.ClipID, err)
				return false
			}
		} else {
			log.Printf("Posted 16:9 public for clip %s → %s", c.ClipID, result169.Post.ID)
			mainPostID = result169.Post.ID
		}
	}

	if c.Video916 != "" {
		shortsTitle := title
		if utf8.RuneCountInString(shortsTitle) > 60 {
			runes := []rune(shortsTitle)
			shortsTitle = string(runes[:60])
		}
		shortsTitle += " #Shorts"

		result916, err := p.zernio.Post(ctx, PostRequest{
			Title:      shortsTitle,
			Content:    shortsTitle + "\n\n" + desc,
			Platforms:  youtubePlatforms(o.ytAccountID, shortsTitle, o.firstComment, VisibilityPublic),
			MediaItems: []MediaItem{{Type: "video", URL: c.Video916}},
			Visibility: VisibilityPublic,
			PublishNow: true,
			RequestID:  postRequestID(c.ClipID, "916"),
		})
		if err != nil {
			if id, ok := p.adoptDuplicate(ctx, err); ok {
				shortsPostID = id
			} else {
				log.Printf("Failed to post 9:16 for clip %s: %v", c.ClipID, err)
				postErr = err
			}
		} else {
			log.Printf("Posted 9:16 Shorts public for clip %s → %s", c.ClipID, result916.Post.ID)
			shortsPostID = result916.Post.ID
		}
	}

	// Nothing posted (no usable video, or every post failed) → leave the clip
	// 'ready' so a later run retries it instead of marking an empty publish.
	if mainPostID == "" && shortsPostID == "" {
		log.Printf("No video published for clip %s, leaving as ready", c.ClipID)
		if postErr != nil {
			p.recordPublishFailure(ctx, c.ClipID, postErr)
		}
		return false
	}

	// Persist status + post ids atomically. The YouTube post already happened,
	// so a silent DB failure here would leave the clip 'ready' and re-post it
	// (duplicate upload) on the next run — log loudly if the commit fails.
	if err := p.recordPublished(ctx, c.ClipID, mainPostID, shortsPostID); err != nil {
		log.Printf("CRITICAL clip %s posted to YouTube (main=%q shorts=%q) but DB commit FAILED: %v — will be re-published next run", c.ClipID, mainPostID, shortsPostID, err)
		return false
	}

	// เงื่อนไข postErr == nil: คลิปที่มีทั้ง 16:9 และ 9:16 อาจขึ้นได้แค่รูปแบบเดียว
	// อีกตัวเพิ่งล้มไป — เหตุผลนั้นยังใหม่อยู่ ห้ามล้างทิ้ง
	if postErr == nil {
		p.clearPublishFailure(ctx, c.ClipID)
	}

	log.Printf("Published clip %s via Zernio (main=%q shorts=%q)", c.ClipID, mainPostID, shortsPostID)

	// ส่งเข้าช่อง Telegram ท้ายสุดเสมอ — คลิปขึ้น YouTube และ DB commit ไปแล้ว
	// อะไรที่พังหลังจากนี้จึงกระทบแค่ช่อง Telegram ช่องเดียว
	p.postTelegramForClip(ctx, c.ClipID, o.tgAccountID, title, mainPostID, shortsPostID)
	return true
}
```

- [ ] **Step 6: เขียน `PublishReady` ใหม่ให้เรียกของสามชิ้นข้างบน**

แทนที่เนื้อของ `PublishReady` ตั้งแต่บรรทัดหลัง `defer func(){...advisory unlock...}()` จนจบฟังก์ชัน (เดิมคือบรรทัด 144-309) ด้วย:

```go
	cands, err := p.readyCandidates(ctx)
	if err != nil {
		return err
	}

	var o publishOpts
	p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'zernio_youtube_account_id'`).Scan(&o.ytAccountID)

	// คอมเมนต์ติดต่อทีมงานใต้คลิป (ไม่ปักหมุด ดูเหตุผลที่ youtubePlatforms) · อ่านครั้งเดียวต่อรอบ
	// ค่าว่าง (หรือไม่มีแถวนี้) = ปิดฟีเจอร์ ไม่ต้อง deploy — Scan ที่ error จะทิ้งค่าไว้เป็น ""
	// ซึ่งคือสถานะปิดพอดี จึงไม่ต้องจัดการ error แยก
	_ = p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'youtube_first_comment'`).Scan(&o.firstComment)
	o.firstComment = strings.TrimSpace(o.firstComment)

	// ช่อง Telegram ที่จะส่งคลิปเข้าไปหลังขึ้น YouTube · ค่าว่าง = ปิดฟีเจอร์ (เหตุผลเดียวกับข้างบน)
	_ = p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'zernio_telegram_account_id'`).Scan(&o.tgAccountID)
	o.tgAccountID = strings.TrimSpace(o.tgAccountID)

	// ส่งจริงไม่เกินใบเดียวต่อรอบ · ใบที่ส่งไม่ออกจะถูกข้ามไปใบถัดไปทันที ไม่ยึดคิวอีกต่อไป
	processedClipID := publishFirst(cands, func(c publishCandidate) bool {
		return p.publishOne(ctx, c, o)
	})

	// เก็บตกท้ายรอบ: คลิปที่ขึ้น YouTube แล้วแต่ยังไม่เข้าช่อง Telegram (จำกัด 24 ชม.)
	// ส่ง processedClipID ไปกันไม่ให้ sweep หยิบคลิปที่เพิ่งส่งในทิคเดียวกันมาลองซ้ำ
	p.sweepTelegramBacklog(ctx, o.tgAccountID, processedClipID)
	return nil
}
```

ลบตัวแปร/คอมเมนต์เดิมที่ถูกแทนแล้วให้หมด (`rows`, `ytAccountID`, `firstComment`, `tgAccountID`, `var processedClipID string`, ทั้งลูป `for rows.Next()`, `rows.Err()`)

**หมายเหตุที่ต้องรู้:** SELECT เดิมดึง `c.thumbnail_url` มา scan ลงตัวแปร `thumb` ที่ไม่มีใครใช้ — query ใหม่ตัดคอลัมน์นั้นออก เพราะเรากำลังเขียน SELECT นี้ใหม่ทั้งอัน ไม่ใช่การไปลบโค้ดข้างเคียงที่ไม่เกี่ยว

- [ ] **Step 7: ยืนยันว่าไม่มีอะไรพัง**

รัน: `go build ./... && go test ./... 2>&1 | tail -20`
คาดหวัง: build ผ่าน, เทสต์ทุกแพ็กเกจ PASS (ไม่มี `ok` ตัวไหนขึ้น FAIL)

- [ ] **Step 8: คอมมิต**

```bash
git add internal/publisher/publisher.go internal/publisher/queue_test.go
git commit -m "fix(publisher): รอบส่งข้ามคลิปที่ส่งไม่ออกแทนการวนใบเดิมจนคิวตัน"
```

---

## Task 2: ทุกเส้นทางที่ส่งไม่ออกต้องเขียน fail_reason

**Files:**
- Modify: `internal/publisher/publisher.go` (ใน `publishOne` — บล็อก `if mainPostID == "" && shortsPostID == ""`)
- Test: `internal/publisher/queue_test.go` (เพิ่มเทสต์)

**Interfaces:**
- Consumes: `p.recordPublishFailure` (`publisher.go:340`), `publishFailPrefix`
- Produces: `var noVideoErr = errors.New("ไม่มีไฟล์วิดีโอให้ส่ง")`, `func publishFailure(postErr error) error`

**ทำไมต้องมี task นี้แม้ Task 1 แก้อาการไปแล้ว:** การดันท้ายคิว (`ORDER BY starts_with(fail_reason, 'publish: ')`) ยังเป็นด่านที่ทำให้คลิปดีได้คิวก่อนเสมอ และ `fail_reason` คือสิ่งเดียวที่ทำให้คนเปิดหน้าคลิปแล้วรู้ว่า "ทำไมกด publish แล้วไม่มีอะไรเกิดขึ้น" โดยไม่ต้องไปไล่ log บน Railway

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

เพิ่มใน `internal/publisher/queue_test.go`:

```go
// เส้นทาง "ไม่มีอะไรขึ้นเลย" ต้องมีเหตุผลเสมอ — เดิมกรณี postErr == nil เงียบสนิท
// ทำให้คลิปไม่ถูกดันท้ายคิวและคนเปิดหน้าคลิปไม่เห็นสาเหตุ (เหตุ 2026-08-14)
func TestPublishFailure_NilBecomesNoVideoReason(t *testing.T) {
	err := publishFailure(nil)
	if err == nil {
		t.Fatal("publishFailure(nil) = nil, want error")
	}
	if !errors.Is(err, noVideoErr) {
		t.Errorf("publishFailure(nil) = %v, want noVideoErr", err)
	}
}

func TestPublishFailure_KeepsOriginalError(t *testing.T) {
	orig := errors.New("zernio 500")
	if got := publishFailure(orig); got != orig {
		t.Errorf("publishFailure(orig) = %v, want %v (ห้ามกลืนเหตุผลจริง)", got, orig)
	}
}
```

เพิ่ม `"errors"` ใน import ของไฟล์เทสต์

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

รัน: `go test ./internal/publisher/ -run TestPublishFailure -v`
คาดหวัง: คอมไพล์ไม่ผ่าน — `undefined: publishFailure`, `undefined: noVideoErr`

- [ ] **Step 3: เพิ่มฟังก์ชันและเรียกใช้**

เพิ่มใต้ `func (c publishCandidate) hasVideo()` ใน `publisher.go`:

```go
// noVideoErr คือเหตุผลของกรณีที่ไม่มีอะไรถูกยิงออกไปเลย (ไม่มีไฟล์วิดีโอให้ส่ง จึงไม่มี
// error จาก Zernio ให้บันทึก) · ต้องมีข้อความเสมอ ไม่งั้นคลิปจะไม่ถูกดันท้ายคิวและคนเปิด
// หน้าคลิปจะไม่เห็นสาเหตุ — นี่คือรูที่ทำให้คิวตัน 21 ชั่วโมงเมื่อ 2026-08-14
var noVideoErr = errors.New("ไม่มีไฟล์วิดีโอให้ส่ง")

// publishFailure เลือกเหตุผลที่จะบันทึก: ถ้ามี error จริงจากการโพสต์ให้ใช้ตัวนั้น
// (คนอ่านต้องได้เหตุผลจริง) ถ้าไม่มีแปลว่าไม่มีอะไรให้ส่งตั้งแต่แรก
func publishFailure(postErr error) error {
	if postErr != nil {
		return postErr
	}
	return noVideoErr
}
```

แล้วแก้บล็อกใน `publishOne`:

```go
	if mainPostID == "" && shortsPostID == "" {
		log.Printf("No video published for clip %s, leaving as ready", c.ClipID)
		p.recordPublishFailure(ctx, c.ClipID, publishFailure(postErr))
		return false
	}
```

(`errors` ถูก import ใน `publisher.go` อยู่แล้ว)

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

รัน: `go test ./internal/publisher/ -v 2>&1 | tail -20`
คาดหวัง: PASS ทั้งแพ็กเกจ

- [ ] **Step 5: คอมมิต**

```bash
git add internal/publisher/publisher.go internal/publisher/queue_test.go
git commit -m "fix(publisher): คลิปที่ไม่มีไฟล์วิดีโอต้องบันทึกเหตุผล ไม่ใช่เงียบแล้ววนซ้ำ"
```

---

## Task 3: log เตือนเมื่อรอบส่งไม่ได้ส่งอะไรทั้งที่มีคลิปค้าง

**Files:**
- Modify: `internal/publisher/publisher.go` (ท้าย `PublishReady`)
- Test: `internal/publisher/queue_test.go` (เพิ่มเทสต์)

**Interfaces:**
- Consumes: `p.pool`
- Produces: `func stuckQueueMessage(total, noMetadata, noVideo int) string`, `func (p *Publisher) logStuckQueue(ctx context.Context)`

**ทำไม:** เหตุ 2026-08-14 ไม่เคยสร้าง error สักตัว รอบส่งขึ้น `completed` ทุกครั้ง อาการเดียวคือคลิปสะสมบนหน้าเว็บ · และยังมีรูที่เงียบยิ่งกว่า: `JOIN clip_metadata` เป็น INNER — คลิป `ready` ที่ไม่มีแถว metadata จะหายจากคิวโดยไม่มี log แม้แต่บรรทัดเดียว (ตรวจ prod 15 ส.ค. 2026 แล้ว: ยังไม่มีเคสนี้ แต่เป็นรูที่เปิดอยู่)

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

เพิ่มใน `internal/publisher/queue_test.go`:

```go
func TestStuckQueueMessage_SilentWhenNothingWaiting(t *testing.T) {
	if got := stuckQueueMessage(0, 0, 0); got != "" {
		t.Errorf("stuckQueueMessage(0,0,0) = %q, want \"\" (ไม่มีคลิปค้าง = ไม่ต้องเตือน)", got)
	}
}

func TestStuckQueueMessage_ReportsBlockers(t *testing.T) {
	got := stuckQueueMessage(3, 1, 2)
	for _, want := range []string{"3", "metadata=1", "ไม่มีไฟล์วิดีโอ=2"} {
		if !strings.Contains(got, want) {
			t.Errorf("stuckQueueMessage(3,1,2) = %q, ต้องมี %q", got, want)
		}
	}
}

func TestStuckQueueMessage_WaitingWithoutKnownBlocker(t *testing.T) {
	got := stuckQueueMessage(2, 0, 0)
	if got == "" {
		t.Fatal("มีคลิปค้าง 2 ใบแต่เงียบ — ต้องเตือนเสมอ")
	}
	if !strings.Contains(got, "2") {
		t.Errorf("ข้อความ %q ต้องบอกจำนวนคลิปที่ค้าง", got)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

รัน: `go test ./internal/publisher/ -run TestStuckQueue -v`
คาดหวัง: คอมไพล์ไม่ผ่าน — `undefined: stuckQueueMessage`

- [ ] **Step 3: เพิ่มฟังก์ชัน**

เพิ่มใต้ `publishFailure` ใน `publisher.go`:

```go
// stuckQueueMessage ประกอบข้อความเตือนเมื่อรอบส่งจบโดยไม่ได้ส่งอะไรเลยทั้งที่ยังมีคลิป
// สถานะ ready ค้างอยู่ · คืน "" เมื่อไม่มีคลิปค้าง (กรณีปกติ ไม่ต้องเตือน)
// noMetadata คือคลิปที่หายจากคิวเพราะไม่มีแถว clip_metadata ให้ JOIN — เงียบที่สุดในบรรดา
// เส้นทางทั้งหมด เพราะมันไม่เคยเข้าถึงลูปส่งเลยจึงไม่มี log ของตัวเอง
func stuckQueueMessage(total, noMetadata, noVideo int) string {
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("PublishReady: จบรอบโดยไม่ได้ส่งคลิปใดเลย ทั้งที่มีคลิป ready ค้าง %d ใบ (ไม่มี metadata=%d, ไม่มีไฟล์วิดีโอ=%d)",
		total, noMetadata, noVideo)
}

// logStuckQueue นับคลิปที่ค้างในคิวแล้วเตือน · เรียกเฉพาะรอบที่ไม่ได้ส่งอะไรเลย
// ความล้มเหลวของการนับต้องไม่กระทบรอบส่ง — log แล้วจบ
func (p *Publisher) logStuckQueue(ctx context.Context) {
	var total, noMetadata, noVideo int
	err := p.pool.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE cm.clip_id IS NULL),
		       COUNT(*) FILTER (WHERE COALESCE(c.video_16_9_url, '') = '' AND COALESCE(c.video_9_16_url, '') = '')
		FROM clips c
		LEFT JOIN clip_metadata cm ON cm.clip_id = c.id
		WHERE c.status = 'ready' AND c.auto_review_held = FALSE AND c.publish_date <= CURRENT_DATE`,
	).Scan(&total, &noMetadata, &noVideo)
	if err != nil {
		log.Printf("PublishReady: นับคลิปค้างในคิวไม่สำเร็จ: %v", err)
		return
	}
	if msg := stuckQueueMessage(total, noMetadata, noVideo); msg != "" {
		log.Print(msg)
	}
}
```

แล้วเรียกใน `PublishReady` ก่อน `sweepTelegramBacklog`:

```go
	if processedClipID == "" {
		p.logStuckQueue(ctx)
	}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

รัน: `go test ./internal/publisher/ -v 2>&1 | tail -20`
คาดหวัง: PASS ทั้งแพ็กเกจ

- [ ] **Step 5: คอมมิต**

```bash
git add internal/publisher/publisher.go internal/publisher/queue_test.go
git commit -m "feat(publisher): เตือนใน log เมื่อรอบส่งจบโดยไม่ส่งอะไรทั้งที่มีคลิปค้าง"
```

---

## Task 4: ปุ่มอนุมัติ/unhold ไม่ปล่อยคลิปที่ยังไม่มีไฟล์วิดีโอเข้าคิว

**Files:**
- Modify: `internal/handler/clips.go` (`Update` บรรทัด 52-74, `Unhold` บรรทัด 87-94)
- Test: `internal/handler/clips_ready_guard_test.go` (สร้าง)

**Interfaces:**
- Consumes: `h.repo.GetByID(ctx, id) (*models.Clip, error)` (`repository/clips.go:59`), `models.Clip.Video916URL/Video169URL *string`
- Produces: `func readyBlockedReason(c *models.Clip) string` ("" = ผ่าน)

**ทำไม:** ปุ่มบนหน้าคลิปเขียนว่า "Override — publish ทั้งที่มีตำหนิ" แต่คลิปที่ตะแกรงเนื้อหาตีตกไม่ได้ "มีตำหนิ" — มัน**ยังไม่เคยถูก render** การกดปุ่มจึงส่งคลิปเปล่าเข้าคิว (นี่คือคลิกที่ทำให้คิวตันจริงเมื่อ 14 ส.ค. 10:27) · Task 5 จะเป็นคนให้ทางออกที่ถูกต้องแทน

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

สร้าง `internal/handler/clips_ready_guard_test.go`:

```go
package handler

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func strp(s string) *string { return &s }

// คลิปที่ยังไม่มีไฟล์วิดีโอต้องเข้าสถานะ ready ไม่ได้ — รอบส่งจะหยิบมันไปวนซ้ำโดยส่งอะไร
// ไม่ได้เลย (เหตุ 2026-08-14 คิวตัน 21 ชั่วโมง)
func TestReadyBlockedReason(t *testing.T) {
	cases := []struct {
		name    string
		clip    *models.Clip
		blocked bool
	}{
		{"ไม่มีไฟล์เลย", &models.Clip{}, true},
		{"url ว่าง", &models.Clip{Video916URL: strp("")}, true},
		{"มี 9:16", &models.Clip{Video916URL: strp("https://cdn/x.mp4")}, false},
		{"มี 16:9", &models.Clip{Video169URL: strp("https://cdn/x.mp4")}, false},
		{"คลิปหาย (nil) ปล่อยให้ชั้นบนจัดการ", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := readyBlockedReason(c.clip)
			if (got != "") != c.blocked {
				t.Errorf("readyBlockedReason() = %q, blocked=%v want %v", got, got != "", c.blocked)
			}
			if c.blocked && !strings.Contains(got, "เรนเดอร์") {
				t.Errorf("ข้อความ %q ต้องบอกทางออก (สั่งเรนเดอร์)", got)
			}
		})
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

รัน: `go test ./internal/handler/ -run TestReadyBlockedReason -v`
คาดหวัง: คอมไพล์ไม่ผ่าน — `undefined: readyBlockedReason`

- [ ] **Step 3: เพิ่ม guard + เรียกใช้ในสองจุด**

เพิ่มใน `internal/handler/clips.go` (เหนือ `func (h *ClipsHandler) Update`):

```go
// readyBlockedReason บอกว่าทำไมคลิปนี้ยังเข้าสถานะ ready ไม่ได้ ("" = เข้าได้)
// สถานะ ready แปลว่า "พร้อมให้รอบส่งหยิบไปอัปขึ้น YouTube" — คลิปที่ยังไม่มีไฟล์วิดีโอ
// เข้าสถานะนี้แล้วจะถูกหยิบไปวนซ้ำทุกรอบโดยส่งอะไรไม่ได้เลย (เหตุ 2026-08-14: คลิปเดียว
// ยึดหัวคิว 21 ชั่วโมง) · ทางออกที่ถูกต้องของคลิปแบบนั้นคือสั่งเรนเดอร์ ไม่ใช่อนุมัติ
func readyBlockedReason(c *models.Clip) string {
	if c == nil {
		return ""
	}
	has := func(u *string) bool { return u != nil && *u != "" }
	if has(c.Video916URL) || has(c.Video169URL) {
		return ""
	}
	return "คลิปนี้ยังไม่มีไฟล์วิดีโอ — สั่งเรนเดอร์ก่อน แล้วคลิปจะเข้าคิวเผยแพร่เอง"
}
```

ใน `Update` เพิ่มหลังบล็อก guard `published`/`producing` ที่มีอยู่:

```go
	if req.Status != nil && *req.Status == "ready" {
		clip, err := h.repo.GetByID(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "clip not found"})
			return
		}
		if reason := readyBlockedReason(clip); reason != "" {
			writeJSON(w, http.StatusConflict, models.APIResponse{Error: reason})
			return
		}
	}
```

ใน `Unhold` เพิ่มก่อนเรียก `ClearAutoReviewHeld`:

```go
	clip, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "clip not found"})
		return
	}
	// ปลดกัก = ดันคลิปเป็น ready (ดู ClearAutoReviewHeld) จึงต้องผ่านด่านเดียวกับ PATCH
	if reason := readyBlockedReason(clip); reason != "" {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: reason})
		return
	}
```

แล้วเปลี่ยนบรรทัดถัดไปเป็น `if err := h.repo.ClearAutoReviewHeld(r.Context(), id); err != nil {` (ตัวแปร `err` ถูกประกาศไปแล้วด้านบน — ใช้ `=` แทน `:=` ถ้าคอมไพเลอร์บ่นเรื่อง shadow)

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

รัน: `go test ./internal/handler/ -v 2>&1 | tail -20`
คาดหวัง: PASS ทั้งแพ็กเกจ (รวม `TestUpdateRejectsPipelineOnlyStatus` เดิม — guard ใหม่อยู่หลัง guard เดิม จึงไม่แตะ path ที่เทสต์นั้นเดิน และ repo ยังเป็น `nil` ไม่ถูกเรียก)

- [ ] **Step 5: คอมมิต**

```bash
git add internal/handler/clips.go internal/handler/clips_ready_guard_test.go
git commit -m "fix(handler): กันคลิปที่ยังไม่มีไฟล์วิดีโอไม่ให้ถูกอนุมัติเข้าคิวเผยแพร่"
```

---

## Task 5: ทางกู้คลิปที่ตะแกรงเนื้อหาตีตก — สั่งเรนเดอร์ใหม่

**Files:**
- Modify: `internal/repository/clips.go` (เพิ่ม `ClearHeldFlag` ใต้ `ClearAutoReviewHeld` บรรทัด 262)
- Modify: `internal/orchestrator/orchestrator.go` (เพิ่ม `ErrRerenderNotAllowed`, `rerenderBlockedReason`, `RerenderClip` ใต้ `RetryClip` บรรทัด 886)
- Modify: `internal/handler/orchestrator.go` (เพิ่ม handler)
- Modify: `internal/router/router.go` (เพิ่ม route ใน `SetOrchestrator`)
- Test: `internal/orchestrator/rerender_test.go` (สร้าง)

**Interfaces:**
- Consumes: `resumeAtRender(stage string) bool` (`orchestrator.go:877`), `o.clipsRepo.GetByID`, `o.tracker.StartProduction/FinishProduction/SetCancelFunc`, `o.resumeHyperframesProduction(ctx, clip)`, `ErrProductionRunning` (`orchestrator.go:147`)
- Produces: `ErrRerenderNotAllowed`, `rerenderBlockedReason(c *models.Clip) string`, `(o *Orchestrator) RerenderClip(ctx, id string) error`, `(r *ClipsRepo) ClearHeldFlag(ctx, id string) error`, route `POST /api/v1/clips/{id}/rerender`

**บริบท:** คลิปที่ `tutorialGateFailure`/`mythGateFailure` ตีตกจะถูกตั้งเป็น `needs_review` **หลังจาก** สคริปต์ ฉาก และ metadata ถูกบันทึกลง DB ครบแล้ว (`orchestrator.go:669-694`) และ `production_stage` เป็น `content_ready` ⇒ `resumeAtRender()` คืน `true` ⇒ **แค่สั่ง render ต่อก็ได้คลิปสมบูรณ์ โดยไม่ต้องจ่ายค่า LLM ใหม่** แต่วันนี้ไม่มีใครเรียกให้ เพราะ `RetryAllFailed` หยิบเฉพาะ `status='failed'` (`repository/clips.go:164`) และ auto-review ทำได้แค่กักซ้ำ

**ข้อควรระวังที่สำคัญที่สุดของ task นี้:** `resumeHyperframesProduction` เข้า `renderAndFinalize` ตรงๆ — **ตะแกรงเนื้อหาไม่ถูกเรียกซ้ำ** ดังนั้นการกดปุ่มนี้ = คนรับผิดชอบเนื้อหาที่ตะแกรงเคยตีตก จึงต้องเป็น endpoint ที่คนกดเท่านั้น ห้ามผูกเข้ากับ schedule หรือ tick ใดๆ

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

สร้าง `internal/orchestrator/rerender_test.go`:

```go
package orchestrator

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func strp(s string) *string { return &s }

func TestRerenderBlockedReason(t *testing.T) {
	cases := []struct {
		name    string
		clip    *models.Clip
		blocked bool
	}{
		{
			name:    "คลิปที่ตะแกรงตีตก — สั่งได้",
			clip:    &models.Clip{Status: "needs_review", ProductionStage: "content_ready"},
			blocked: false,
		},
		{
			name:    "ready ที่ไฟล์หาย — สั่งได้",
			clip:    &models.Clip{Status: "ready", ProductionStage: "rendered"},
			blocked: false,
		},
		{
			name:    "มีไฟล์วิดีโออยู่แล้ว — ห้าม (เผาเงินซ้ำโดยไม่จำเป็น)",
			clip:    &models.Clip{Status: "needs_review", ProductionStage: "content_ready", Video916URL: strp("https://cdn/x.mp4")},
			blocked: true,
		},
		{
			name:    "กำลังผลิตอยู่ — ห้าม",
			clip:    &models.Clip{Status: "producing", ProductionStage: "content_ready"},
			blocked: true,
		},
		{
			name:    "เผยแพร่ไปแล้ว — ห้าม",
			clip:    &models.Clip{Status: "published", ProductionStage: "rendered"},
			blocked: true,
		},
		{
			name:    "ยังไม่มีฉากที่บันทึกไว้ — ห้าม (ไม่มีอะไรให้ resume)",
			clip:    &models.Clip{Status: "needs_review", ProductionStage: ""},
			blocked: true,
		},
		{
			name:    "ไม่มีคลิป",
			clip:    nil,
			blocked: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rerenderBlockedReason(c.clip)
			if (got != "") != c.blocked {
				t.Errorf("rerenderBlockedReason() = %q, blocked=%v want %v", got, got != "", c.blocked)
			}
		})
	}
}

func TestRerenderBlockedReason_MessageNamesTheProblem(t *testing.T) {
	got := rerenderBlockedReason(&models.Clip{Status: "published", ProductionStage: "rendered"})
	if !strings.Contains(got, "published") {
		t.Errorf("ข้อความ %q ต้องบอกสถานะที่เป็นเหตุ", got)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

รัน: `go test ./internal/orchestrator/ -run TestRerenderBlockedReason -v`
คาดหวัง: คอมไพล์ไม่ผ่าน — `undefined: rerenderBlockedReason`

- [ ] **Step 3: เพิ่ม `rerenderBlockedReason` + `RerenderClip`**

เพิ่มใน `internal/orchestrator/orchestrator.go` ใต้ `RetryClip` (บรรทัด 886):

```go
// ErrRerenderNotAllowed แยกจากความล้มเหลวอื่นเพื่อให้ handler ตอบ 409 ได้ตรง — คนกดปุ่ม
// ต้องเห็นว่า "สั่งไม่ได้เพราะอะไร" ไม่ใช่ 500 ลอยๆ
var ErrRerenderNotAllowed = errors.New("rerender not allowed")

// rerenderBlockedReason บอกว่าทำไมคลิปนี้สั่งเรนเดอร์ใหม่ไม่ได้ ("" = สั่งได้)
//
// เป้าหมายของทางนี้คือคลิปที่ตะแกรงเนื้อหา (tutorial/myth) ตีตกก่อนถึงขั้นเรนเดอร์:
// สคริปต์ ฉาก และ metadata ถูกบันทึกครบแล้ว (stage=content_ready) ขาดแค่ไฟล์วิดีโอ —
// การ resume จึงไม่ต้องจ่ายค่า LLM ใหม่ · คลิปที่มีไฟล์อยู่แล้วถูกห้ามไว้ เพราะการเรนเดอร์
// ซ้ำมีต้นทุนจริง (ภาพ + เสียง + CPU) และของเดิมก็ใช้ได้อยู่
func rerenderBlockedReason(c *models.Clip) string {
	if c == nil {
		return "ไม่พบคลิปนี้"
	}
	if c.Status != "needs_review" && c.Status != "ready" {
		return fmt.Sprintf("คลิปสถานะ %s สั่งเรนเดอร์ใหม่ไม่ได้ (รับเฉพาะ needs_review และ ready)", c.Status)
	}
	has := func(u *string) bool { return u != nil && *u != "" }
	if has(c.Video916URL) || has(c.Video169URL) {
		return "คลิปนี้มีไฟล์วิดีโออยู่แล้ว — ถ้าต้องการของใหม่ให้ลบคลิปแล้วผลิตใหม่"
	}
	if !resumeAtRender(c.ProductionStage) {
		return fmt.Sprintf("คลิปยังไม่มีฉาก/สคริปต์ที่บันทึกไว้ (stage=%q) จึงเรนเดอร์ต่อไม่ได้", c.ProductionStage)
	}
	return ""
}

// RerenderClip สั่งเรนเดอร์คลิปที่มีเนื้อหาครบแล้วแต่ยังไม่มีไฟล์วิดีโอ — ทางกู้ของคลิปที่
// ตะแกรงเนื้อหาตีตกก่อนถึงขั้นเรนเดอร์ · เรียกจาก endpoint ที่ "คนกด" เท่านั้น ห้ามผูกกับ
// schedule หรือ tick ใดๆ เพราะเส้นทาง resume ข้ามตะแกรงเนื้อหา (มันอยู่ก่อน renderAndFinalize)
// การกดปุ่มนี้จึงเท่ากับคนรับผิดชอบเนื้อหาที่ตะแกรงเคยตีตกด้วยตัวเอง
func (o *Orchestrator) RerenderClip(ctx context.Context, id string) error {
	clip, err := o.clipsRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("%w: ไม่พบคลิป %s", ErrRerenderNotAllowed, id)
	}
	if reason := rerenderBlockedReason(clip); reason != "" {
		return fmt.Errorf("%w: %s", ErrRerenderNotAllowed, reason)
	}

	if !o.tracker.StartProduction(1) {
		return ErrProductionRunning
	}
	defer o.tracker.FinishProduction()

	ctx, cancel := context.WithCancel(ctx)
	o.tracker.SetCancelFunc(cancel)
	defer cancel()

	// ปลดธงกักก่อนเรนเดอร์: renderAndFinalize จะตั้งสถานะเป็น ready เมื่อผ่านทุกด่าน
	// แต่รอบส่งกรอง auto_review_held = FALSE ด้วย ถ้าไม่ปลดตรงนี้คลิปจะเรนเดอร์เสร็จ
	// แล้วค้างอยู่นอกคิวเงียบๆ · ปลดแค่ธง ไม่แตะสถานะ (resume เป็นคนตั้ง producing เอง)
	if err := o.clipsRepo.ClearHeldFlag(ctx, id); err != nil {
		log.Printf("rerender %s: ปลดธงกักไม่สำเร็จ (เรนเดอร์ต่อ แต่คลิปอาจไม่เข้าคิว): %v", id, err)
	}

	log.Printf("Rerender: สั่งเรนเดอร์คลิป %s ใหม่ (stage=%s)", id, clip.ProductionStage)
	return o.resumeHyperframesProduction(ctx, clip)
}
```

เพิ่ม `ClearHeldFlag` ใน `internal/repository/clips.go` ใต้ `ClearAutoReviewHeld`:

```go
// ClearHeldFlag ปลดธงกักอย่างเดียว ไม่แตะสถานะ — ต่างจาก ClearAutoReviewHeld ที่ดันคลิป
// เป็น ready ด้วย · ใช้ตอนสั่งเรนเดอร์ใหม่ ซึ่งสถานะจะถูกตั้งเป็น producing แล้วจบที่
// ready/needs_review ตามผลของด่านตรวจ การดันเป็น ready ระหว่างทางจึงผิดและอันตราย
func (r *ClipsRepo) ClearHeldFlag(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE clips SET auto_review_held = FALSE, updated_at = NOW() WHERE id = $1`, id)
	return err
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

รัน: `go test ./internal/orchestrator/ -run TestRerender -v`
คาดหวัง: PASS ทั้ง 2 เทสต์

- [ ] **Step 5: เพิ่ม handler + route**

ใน `internal/handler/orchestrator.go` เพิ่มใต้ `RetryFailed`:

```go
// RerenderClip สั่งเรนเดอร์คลิปที่เนื้อหาครบแล้วแต่ยังไม่มีไฟล์วิดีโอ · ทำงานเบื้องหลัง
// (เรนเดอร์ใช้เวลาหลักนาที) จึงตอบ 202 ทันทีเหมือน RetryFailed แล้วให้หน้าเว็บติดตามจาก
// สถานะคลิป · ตรวจเงื่อนไขแบบ synchronous ก่อนเสมอ เพื่อให้คนกดปุ่มได้เหตุผลกลับทันที
func (h *OrchestratorHandler) RerenderClip(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if s := h.tracker.GetStatus(); s.Active {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: "Production already in progress"})
		return
	}
	if err := h.orch.CanRerender(r.Context(), id); err != nil {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, models.APIResponse{Message: "กำลังเรนเดอร์คลิปใหม่"})
	go func() {
		if err := h.orch.RerenderClip(context.Background(), id); err != nil {
			log.Printf("Rerender %s failed: %v", id, err)
		}
	}()
}
```

เพิ่มใน `internal/orchestrator/orchestrator.go` (ตัวตรวจแบบ synchronous ที่ handler ใช้ — ใช้ตรรกะเดียวกับ `RerenderClip` เป๊ะ ไม่ก๊อปเงื่อนไข):

```go
// CanRerender ตรวจว่าคลิปนี้สั่งเรนเดอร์ใหม่ได้ไหม โดยไม่เริ่มงานจริง — handler เรียก
// ก่อนตอบ 202 เพื่อให้คนกดปุ่มได้เหตุผลกลับทันทีแทนที่จะต้องไปไล่ log
// (RerenderClip ตรวจซ้ำเองอีกรอบ เพราะงานจริงวิ่งใน goroutine คนละจังหวะกับการตรวจนี้)
func (o *Orchestrator) CanRerender(ctx context.Context, id string) error {
	clip, err := o.clipsRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("%w: ไม่พบคลิป %s", ErrRerenderNotAllowed, id)
	}
	if reason := rerenderBlockedReason(clip); reason != "" {
		return fmt.Errorf("%w: %s", ErrRerenderNotAllowed, reason)
	}
	return nil
}
```

เพิ่ม import `"github.com/go-chi/chi/v5"` ใน `internal/handler/orchestrator.go` ถ้ายังไม่มี

ใน `internal/router/router.go` เพิ่มบรรทัดท้าย `SetOrchestrator`:

```go
	r.Post("/api/v1/clips/{id}/rerender", h.RerenderClip)
```

- [ ] **Step 6: ยืนยันว่า build + เทสต์ทั้งโปรเจกต์ผ่าน**

รัน: `go build ./... && go test ./... 2>&1 | tail -20`
คาดหวัง: build ผ่าน, ไม่มี FAIL

- [ ] **Step 7: ตรวจว่า route ขึ้นจริง (ไม่ยิงงานจริง)**

รัน (เครื่องตัวเอง ไม่ต่อ prod DB):
```bash
go vet ./internal/router/ ./internal/handler/
grep -n "rerender" internal/router/router.go
```
คาดหวัง: `go vet` เงียบ และ grep เจอบรรทัด route หนึ่งบรรทัด

- [ ] **Step 8: คอมมิต**

```bash
git add internal/orchestrator/orchestrator.go internal/orchestrator/rerender_test.go internal/repository/clips.go internal/handler/orchestrator.go internal/router/router.go
git commit -m "feat(orchestrator): สั่งเรนเดอร์ใหม่ให้คลิปที่ตะแกรงเนื้อหาตีตกก่อนถึงขั้นเรนเดอร์"
```

---

## Task 6: ตะแกรงเนื้อหาเขียนเหตุผลลง fail_reason

**Files:**
- Modify: `internal/orchestrator/orchestrator.go:1123-1128` (`blockForReview`)
- Test: `internal/orchestrator/rerender_test.go` (เพิ่มเทสต์)

**Interfaces:**
- Consumes: `o.clipsRepo.SetFailReason(ctx, id, reason) error` (`repository/clips.go:196`)
- Produces: `func gateFailReason(gate, msg string) string`

**ทำไม:** วันนี้ `blockForReview` เขียนแค่ `log.Printf` + เปลี่ยนสถานะ ⇒ คนเปิดหน้าคลิปเห็นแค่ป้าย "ถูกกัก QA" ไม่รู้ว่าตะแกรงตัวไหนตีตกเพราะอะไร (log บน Railway หมุนหายภายในไม่กี่วัน) · เส้นทาง layout inspector เขียนไว้แล้วที่ `orchestrator.go:812` — task นี้ทำให้ตะแกรงเนื้อหาทำเหมือนกัน · หน้าเว็บมีที่แสดงอยู่แล้ว (`OverviewTab.tsx:18` แถว "สาเหตุที่ล้มเหลว") จึงไม่ต้องแก้ UI เพิ่ม

**เกร็ดที่ต้องรู้:** เมื่อคลิปเรนเดอร์สำเร็จภายหลัง `renderAndFinalize` จะเรียก `ClearFailReason` ให้เอง (`orchestrator.go:829-831`) เหตุผลเก่าจึงไม่ค้างบนคลิปที่กู้สำเร็จแล้ว

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

เพิ่มใน `internal/orchestrator/rerender_test.go`:

```go
func TestGateFailReason(t *testing.T) {
	got := gateFailReason("tutorial", `ui_vocab violation: scene 2 breadcrumb: "A" not in ui_vocab`)
	if !strings.HasPrefix(got, "tutorial gate: ") {
		t.Errorf("gateFailReason() = %q, ต้องขึ้นต้นด้วยชื่อตะแกรง", got)
	}
	if !strings.Contains(got, "scene 2") {
		t.Errorf("gateFailReason() = %q, ต้องพกรายละเอียดเดิมไว้ครบ", got)
	}
	// ห้ามชนกับ prefix ของรอบส่ง ไม่งั้นเหตุผลของตะแกรงจะถูก clearPublishFailure ล้างทิ้ง
	if strings.HasPrefix(got, "publish: ") {
		t.Error("เหตุผลของตะแกรงต้องไม่ขึ้นต้นด้วย publish: — รอบส่งจะล้างทิ้ง")
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

รัน: `go test ./internal/orchestrator/ -run TestGateFailReason -v`
คาดหวัง: คอมไพล์ไม่ผ่าน — `undefined: gateFailReason`

- [ ] **Step 3: เพิ่มฟังก์ชันและเรียกใน `blockForReview`**

แก้ `internal/orchestrator/orchestrator.go` (แทนที่ `blockForReview` เดิมทั้งฟังก์ชัน):

```go
// gateFailReason ประกอบข้อความเหตุผลของตะแกรงให้อยู่รูปเดียวกันทุกตัว · จงใจไม่ขึ้นต้นด้วย
// "publish: " เพราะ prefix นั้นเป็นของรอบส่ง (clearPublishFailure จะล้างข้อความที่ขึ้นต้น
// แบบนั้นทิ้งเมื่อคลิปขึ้นสำเร็จ) — เหตุผลของตะแกรงต้องอยู่จนกว่าคลิปจะถูกเรนเดอร์ใหม่จริง
func gateFailReason(gate, msg string) string {
	return gate + " gate: " + msg
}

// blockForReview ส่งคลิปเข้าคิวคนตรวจแทนการทิ้ง ใช้ร่วมกันโดยทุกตะแกรง deterministic
// ที่ทำงานแทนคนตรวจก่อนเรนเดอร์ (ui_vocab ของคลิปสอน, ข้อเท็จจริงของคลิป myth)
//
// จงใจไม่ใช่ failClip: เนื้อหาผิด ไม่ใช่ระบบพัง การ retry แบบไม่มีคนดูจะเผา LLM
// รอบใหม่กับ output เดิม · เขียนที่เดียวเพื่อให้ตะแกรงที่เพิ่มมาทีหลังไม่หลุดสถานะ
// หรือรูปแบบข้อความ error ไปจากกันเงียบๆ
//
// เหตุผลต้องลง fail_reason ด้วยเสมอ: log บน Railway หมุนหายภายในไม่กี่วัน คนที่เปิดหน้า
// คลิปทีหลังจะเห็นแค่ป้าย "ถูกกัก QA" แล้วเดาไม่ออกว่าตะแกรงตัวไหนตีตกเพราะอะไร
// (เกิดจริง 2026-08-14: คนกดอนุมัติคลิปที่ยังไม่ได้เรนเดอร์ แล้วคิวส่งตัน 21 ชั่วโมง)
func (o *Orchestrator) blockForReview(ctx context.Context, clipID, gate, msg string) error {
	log.Printf("%s gate blocked clip %s: %s", gate, clipID, msg)
	reviewStatus := "needs_review"
	o.clipsRepo.Update(ctx, clipID, models.UpdateClipRequest{Status: &reviewStatus})
	if err := o.clipsRepo.SetFailReason(ctx, clipID, gateFailReason(gate, msg)); err != nil {
		log.Printf("%s gate: บันทึกเหตุผลของคลิป %s ไม่สำเร็จ: %v", gate, clipID, err)
	}
	return fmt.Errorf("%s gate: %s: %w", gate, msg, ErrContentGateBlocked)
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

รัน: `go test ./internal/orchestrator/ -v 2>&1 | tail -20`
คาดหวัง: PASS ทั้งแพ็กเกจ

- [ ] **Step 5: คอมมิต**

```bash
git add internal/orchestrator/orchestrator.go internal/orchestrator/rerender_test.go
git commit -m "feat(orchestrator): ตะแกรงเนื้อหาบันทึกเหตุผลลง fail_reason ให้เห็นบนหน้าคลิป"
```

---

## Task 7: หน้าคลิปเสนอปุ่มที่ถูกต้องตามสภาพคลิป

**Files:**
- Modify: `frontend/src/pages/ClipDetail.tsx` (state `acting` บรรทัด 25, `handleApprove` บรรทัด 101-115, บล็อกปุ่มบรรทัด 181-193)
- Test: ไม่มี test runner ฝั่ง frontend — ตรวจด้วย `npm run build` (tsc) + ดูด้วยตาบน dev server

**Interfaces:**
- Consumes: `apiFetch` (`frontend/src/api.ts:13`), `ClipFull.video_9_16_url` / `video_16_9_url` (`api.ts:120-122`), endpoint `POST /api/v1/clips/{id}/rerender` (Task 5)
- Produces: —

**ทำไม:** ปุ่มเดียวที่มีวันนี้ตั้ง `ready` เสมอ ซึ่งตอนนี้ backend จะปฏิเสธด้วย 409 (Task 4) ⇒ ถ้าไม่แก้ UI คนจะกดแล้วเจอ error ที่ไม่มีทางไปต่อ · ปุ่มต้องแยกสองกรณีให้ตรงกับสิ่งที่คลิปต้องการจริง

- [ ] **Step 1: เพิ่มสถานะปุ่มใหม่**

แก้บรรทัด 25:

```tsx
  const [acting, setActing] = useState<'approve' | 'reject' | 'delete' | 'rerender' | null>(null);
```

และแก้ signature ของ `runAction` (บรรทัด 76):

```tsx
    kind: 'approve' | 'reject' | 'delete' | 'rerender',
```

- [ ] **Step 2: เพิ่มตัวแปรและ handler**

เพิ่มใต้บรรทัด `const reviewable = clip.status === 'needs_review';` (บรรทัด 71):

```tsx
  // คลิปที่ตะแกรงเนื้อหาตีตกจะไม่มีไฟล์วิดีโอเลย (มันถูกกันไว้ก่อนถึงขั้นเรนเดอร์) — สิ่งที่
  // คลิปแบบนี้ต้องการคือ "สั่งเรนเดอร์" ไม่ใช่ "อนุมัติ" · การอนุมัติมันเข้าคิวเผยแพร่คือสิ่งที่
  // ทำให้คิวส่งตัน 21 ชั่วโมงเมื่อ 2026-08-14 และตอนนี้ backend ปฏิเสธด้วย 409
  const hasVideo = Boolean(clip.video_9_16_url) || Boolean(clip.video_16_9_url);
```

เพิ่ม handler ใต้ `handleApprove`:

```tsx
  function handleRerender(): void {
    runAction(
      'rerender',
      () => apiFetch(`/api/v1/clips/${clip.id}/rerender`, { method: 'POST' }),
      'สั่งเรนเดอร์แล้ว — ใช้เวลาราว 10 นาที คลิปจะเข้าคิวเผยแพร่เองเมื่อผ่านด่านตรวจ',
      false,
    );
  }
```

- [ ] **Step 3: แยกปุ่มตามสภาพคลิป**

แทนที่บล็อกปุ่ม (บรรทัด 181-193) ด้วย:

```tsx
          {(reviewable || held) && (
            <div className="flex flex-col gap-2 mt-3">
              {hasVideo ? (
                <Button onClick={handleApprove} disabled={acting !== null} size="sm">
                  {acting === 'approve' ? <Loader2 className="size-4 animate-spin" /> : <CheckCircle2 className="size-4" />}
                  {held ? 'Override — publish ทั้งที่มีตำหนิ' : 'อนุมัติ — พร้อม publish'}
                </Button>
              ) : (
                <Button onClick={handleRerender} disabled={acting !== null} size="sm">
                  {acting === 'rerender' ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
                  อนุมัติเนื้อหา + สั่งเรนเดอร์
                </Button>
              )}
              <Button variant="destructive" size="sm" onClick={() => handleDelete('reject')} disabled={acting !== null}>
                {acting === 'reject' ? <Loader2 className="size-4 animate-spin" /> : <X className="size-4" />}
                ตีกลับ (ลบ)
              </Button>
            </div>
          )}
```

เพิ่ม `RefreshCw` ใน import ของ `lucide-react` (บรรทัด 17):

```tsx
import { ArrowLeft, CheckCircle2, Loader2, Lock, RefreshCw, Trash2, VideoOff, X } from 'lucide-react';
```

- [ ] **Step 4: ตรวจว่า type + build ผ่าน**

รัน: `cd frontend && npm run build`
คาดหวัง: `tsc` ไม่มี error และ vite build สำเร็จ

- [ ] **Step 5: ตรวจด้วยตา (ไม่ต่อ prod DB)**

รัน: `cd frontend && npm run dev` แล้วเปิดหน้าคลิปที่ยังไม่มีไฟล์วิดีโอ (ถ้าไม่มีข้อมูลในเครื่อง ให้ตรวจจากโค้ดว่าเงื่อนไข `hasVideo` ครอบทั้ง `video_9_16_url` และ `video_16_9_url` แล้วข้ามขั้นนี้ พร้อมระบุในรายงานว่าไม่ได้ตรวจด้วยตา)
คาดหวัง: คลิปไม่มีวิดีโอ → เห็นปุ่ม "อนุมัติเนื้อหา + สั่งเรนเดอร์" · คลิปมีวิดีโอ → เห็นปุ่มเดิม

- [ ] **Step 6: คอมมิต**

```bash
git add frontend/src/pages/ClipDetail.tsx
git commit -m "feat(ui): คลิปที่ยังไม่มีไฟล์วิดีโอเสนอปุ่มสั่งเรนเดอร์แทนปุ่มอนุมัติ"
```

---

## ตรวจรับทั้งชุด (หลัง Task 7)

- [ ] **1. เทสต์ทั้งโปรเจกต์**

รัน: `go build ./... && go test ./... 2>&1 | grep -v "no test files" | tail -25`
คาดหวัง: ไม่มี FAIL

- [ ] **2. ทวนว่าเหตุเดิมเกิดซ้ำไม่ได้แล้ว — ไล่ตามโค้ด ไม่ใช่ความรู้สึก**

| ด่าน | ยืนยันด้วย |
|---|---|
| คลิปไร้วิดีโอเข้าสถานะ ready ไม่ได้ | `readyBlockedReason` ถูกเรียกทั้งใน `Unhold` และ `Update` |
| ถ้าเข้าไปได้ ก็ไม่ติดคิว | `readyClipsQuery` มีเงื่อนไข `COALESCE(...) <> ''` |
| ถ้าติดคิวได้ ก็ไม่ยึดหัวคิว | `publishFirst` ลองใบถัดไป + `LIMIT $2` = 5 |
| ถ้ายึดได้ ก็ถูกดันท้ายคิวรอบหน้า | `publishFailure(nil)` เขียน `fail_reason` เสมอ |
| ถ้ายังตันอยู่ ก็มีคนรู้ | `logStuckQueue` เตือนใน log ทุกรอบที่ส่งไม่ออก |
| คลิปที่ตะแกรงตีตกมีทางกู้ | `POST /clips/{id}/rerender` + ปุ่มบนหน้าคลิป |
| คนเห็นเหตุผล | `gateFailReason` → `fail_reason` → `OverviewTab` |

- [ ] **3. รัน `/simplify` กับ diff ทั้งชุดก่อนส่งรีวิว** (ธรรมเนียมของเจ้าของโปรเจกต์: ทำก่อนขั้น commit สุดท้าย/PR)

- [ ] **4. deploy**

Railway auto-deploy จาก push เข้า `master` (backend `adsvance-v2` และ frontend `adsvance-frontend` deploy จาก push เดียวกัน) · **ห้าม deploy ระหว่างที่มีคลิปกำลังผลิต** — เช็คก่อนด้วย `GET /api/v1/production/status` หรือดูหน้า war-room

- [ ] **5. หลัง deploy: ยืนยันจากของจริง**

รัน (อ่านอย่างเดียว ไม่ยิง produce):
```bash
railway logs -s adsvance-v2 | grep -E "PublishReady|No video published|Published clip"
```
คาดหวัง: รอบส่งถัดไปยังขึ้นคลิปตามปกติ และถ้ามีคลิปที่ส่งไม่ออกจะเห็นทั้งบรรทัด `No video published ...` และคลิปถัดไปถูกส่งต่อในรอบเดียวกัน

---

## Self-Review

**ครอบคลุมข้อเสนอทั้ง A/B/C ที่เจ้าของอนุมัติ:**

| ข้อเสนอเดิม | task |
|---|---|
| A1 เขียน `fail_reason` เมื่อไม่มีวิดีโอ | 2 |
| A2 กรองคลิปไร้วิดีโอออกจาก query | 1 |
| A3 guard ปุ่มอนุมัติ/unhold | 4 |
| B4 ทางกู้คลิปที่ gate ตีตก | 5 |
| B5 `blockForReview` เขียนเหตุผล | 6 |
| C6 log เตือนคิวตัน + กรณี metadata หาย | 3 |
| C7 คิวไม่ตันจากคลิปเดียว (ลองใบถัดไป) | 1 |
| (ผลพลอยได้) UI ต้องตามหลัง guard ใหม่ | 7 |

**ความสอดคล้องของชื่อ/ชนิด:** `publishCandidate`/`publishOpts`/`publishFirst` ใช้ตรงกันใน Task 1-3 · `readyBlockedReason` (handler) กับ `rerenderBlockedReason` (orchestrator) เป็นคนละฟังก์ชันคนละแพ็กเกจโดยตั้งใจ — ตัวแรกตอบ "เข้าคิวได้ไหม" ตัวหลังตอบ "สั่งเรนเดอร์ได้ไหม" · `strp` ถูกประกาศในไฟล์เทสต์ของ 2 แพ็กเกจต่างกัน (handler, orchestrator) จึงไม่ชนกัน — แต่ถ้าแพ็กเกจนั้นมี helper ชื่อเดียวกันอยู่แล้วให้ใช้ของเดิมแทนการประกาศซ้ำ

**ข้อจำกัดที่ยอมรับไว้อย่างรู้ตัว:** `TestReadyClipsQuery_KeepsAntiBlockingGuards` ตรวจสตริง SQL ไม่ใช่พฤติกรรมจริงกับฐานข้อมูล — เป็นสิ่งที่ทำได้ดีที่สุดในโปรเจกต์ที่ไม่มี integration test ต่อ Postgres และมันล็อกไว้เฉพาะสามอย่างที่ถ้าหลุดแล้วคิวจะกลับไปตันเงียบๆ แบบเดิม
