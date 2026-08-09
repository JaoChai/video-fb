# Telegram Channel Post Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** คลิปขึ้น YouTube สำเร็จเมื่อไหร่ ให้ระบบโพสต์ "ชื่อคลิป + ลิงก์" เข้าช่อง Telegram `@adsvancech` อัตโนมัติผ่าน Zernio

**Architecture:** ต่อท้ายเส้นทาง "โพสต์สำเร็จ" ของ `PublishReady()` — หลังบันทึกสถานะ `published` แล้ว ยิง `GET /posts/{id}` เอา `platformPostUrl` ของ YouTube มาประกอบข้อความ แล้วส่งเป็นโพสต์ Telegram อีกใบผ่าน Zernio · มีรอบเก็บตกในกรอบ 24 ชั่วโมงกันคลิปตกหล่นตอน Zernio ล่ม · ทุกอย่างปิดสนิทเมื่อ setting `zernio_telegram_account_id` ว่าง

**Tech Stack:** Go 1.x (stdlib + pgx/v5), PostgreSQL (Neon), Zernio API v1, เทสต์ด้วย `testing` + `net/http/httptest`

## Global Constraints

- **สเปกอ้างอิง:** `docs/superpowers/specs/2026-08-09-telegram-channel-post-design.md` — อ่านก่อนเริ่ม
- **ห้ามแตะ `clips.status` และ `clips.fail_reason` ในเส้นทาง Telegram** — คลิปขึ้น YouTube สำเร็จไปแล้ว ความล้มเหลวของ Telegram ต้องเป็นแค่ `log.Printf`
- **`zernio_telegram_account_id` ค่าว่าง = ปิดสนิท** ต้องไม่มี HTTP request ออกไปแม้แต่ครั้งเดียว
- **รอบเก็บตกต้องจำกัด `updated_at > NOW() - INTERVAL '24 hours'` เสมอ** — กรอบนี้คือสิ่งเดียวที่กันคลังคลิปเก่าไม่ให้ถูกยิงเข้าช่องรวดเดียว
- ภาษาในคอมเมนต์โค้ด: ไทย (ตามที่ไฟล์ `publisher.go` ใช้อยู่) · อธิบาย **ทำไม** ไม่ใช่ **อะไร**
- ห้ามรัน backend local ต่อ prod DB (scheduler จะยิง cron จริง) — ทดสอบ SQL บน Neon branch เท่านั้น
- คอมมิตทุก task · ข้อความคอมมิตภาษาไทย ขึ้นต้นด้วย `feat(publisher):` / `test(publisher):` / `chore:`
- ค่าคงที่ที่ต้องใช้ตรงกันทุก task: `const platformTelegram = "telegram"`, format ของ request id คือ `postRequestID(clipID, "telegram")`

## หมายเหตุการตัดสินใจที่ต่างจากสเปกเล็กน้อย (ทำให้เล็กลง ไม่ใช่ใหญ่ขึ้น)

1. **ไม่ส่ง `platformSpecificData` ในโพสต์ Telegram เลย** — เอกสาร Zernio ระบุว่า `parseMode`
   ค่า default คือ `"HTML"` อยู่แล้ว จึงไม่ต้องคลาย type ของ `PlatformTarget.PlatformSpecificData`
   (ปัจจุบันผูกกับ `*YouTubeOptions`) ⇒ เส้นทาง YouTube ไม่ถูกแตะเลยแม้แต่บรรทัดเดียว
   แต่ยัง escape `& < >` ในชื่อคลิปเหมือนเดิม เพราะปลายทาง parse HTML
2. **ไม่ต้องตัด ` #Shorts` ออกจากชื่อคลิป** — ตรวจแล้วว่าชื่อที่ไหลเข้าเส้นทาง Telegram ไม่เคยมี
   `#Shorts`: ใน `PublishReady` ตัวแปร `shortsTitle` เป็นคนละตัวกับ `title` (`publisher.go:233`)
   และรอบเก็บตกอ่าน `clip_metadata.youtube_title` ซึ่งเป็นค่าดิบ ⇒ เขียนโค้ดตัดไว้ = โค้ดตายตั้งแต่วันแรก

---

## File Structure

| ไฟล์ | ความรับผิดชอบ |
|---|---|
| `migrations/081_telegram_channel_post.sql` (สร้าง) | คอลัมน์ `zernio_telegram_post_id`, setting ค่าว่าง, ปั๊ม `skipped-backfill` ให้คลิปเก่า |
| `internal/publisher/zernio.go` (แก้) | `GetPost()` อ่าน `platforms[]` เพิ่ม — ทุกอย่างที่เกี่ยวกับรูปร่าง response ของ Zernio อยู่ไฟล์นี้ |
| `internal/publisher/telegram.go` (สร้าง) | ตรรกะ Telegram ทั้งหมด: ประกอบข้อความ, เลือกลิงก์, ส่ง, บันทึก, รอบเก็บตก |
| `internal/publisher/telegram_test.go` (สร้าง) | เทสต์ของไฟล์ข้างบน |
| `internal/publisher/publisher.go` (แก้) | เรียกใช้ 2 จุด + rename `cleanTikTokHook` → `stripBrandTag` |
| `internal/handler/settings.go` (แก้) | allowlist |
| `frontend/src/pages/Settings.tsx` (แก้) | ช่องกรอก account id |

---

## Task 1: migration 081 — คอลัมน์ + setting + กันคลิปเก่า

**Files:**
- Create: `migrations/081_telegram_channel_post.sql`

**Interfaces:**
- Consumes: ตาราง `clip_metadata`, `clips`, `settings` ที่มีอยู่
- Produces: คอลัมน์ `clip_metadata.zernio_telegram_post_id TEXT` (NULL = ยังไม่ส่ง) และ setting key `zernio_telegram_account_id`

- [ ] **Step 1: เขียนไฟล์ migration**

```sql
-- 081: โพสต์คลิปเข้าช่อง Telegram อัตโนมัติหลังขึ้น YouTube
--
-- ระบบโพสต์ผ่าน Zernio เหมือน YouTube/TikTok (แผน Accelerate ที่ใช้อยู่รองรับ platform
-- telegram แล้ว: uploads ไม่จำกัด, profiles 7/50 เมื่อ 2026-08-09) โพสต์ในช่องจะขึ้นเป็น
-- ชื่อและโลโก้ของช่องเอง ไม่ใช่ชื่อบอท
--
-- zernio_telegram_post_id: id ของโพสต์ Telegram ฝั่ง Zernio · NULL = ยังไม่ได้ส่ง
-- ใช้เป็นตัวกันส่งซ้ำและเป็นตัวชี้เป้าของรอบเก็บตก
--
-- ทำไมต้องปั๊ม 'skipped-backfill': รอบเก็บตกไล่คลิปที่ published ภายใน 24 ชม.ที่ยัง
-- ไม่มี id — ถ้าไม่ปั๊มค่าไว้ก่อน คลิป 2-4 ตัวที่เพิ่งขึ้นก่อน deploy จะถูกยิงเข้าช่อง
-- ทันทีที่ deploy ทั้งที่เจ้าของสั่งไว้ว่า "เริ่มนับจากคลิปใหม่เท่านั้น"
--
-- setting ค่าว่าง = ฟีเจอร์ปิดสนิท (โค้ดข้ามทั้งบล็อก ไม่ยิง HTTP เลย) เปิดใช้โดยกรอก
-- account id ที่ได้จาก Zernio หลังผูกช่อง ไม่ต้อง deploy ใหม่
-- ON CONFLICT DO NOTHING เพื่อไม่ให้การรัน migration ซ้ำทับค่าที่กรอกไว้แล้ว

BEGIN;

ALTER TABLE clip_metadata ADD COLUMN IF NOT EXISTS zernio_telegram_post_id TEXT;

UPDATE clip_metadata cm
SET zernio_telegram_post_id = 'skipped-backfill'
FROM clips c
WHERE c.id = cm.clip_id
  AND c.status = 'published'
  AND cm.zernio_telegram_post_id IS NULL;

INSERT INTO settings (key, value) VALUES ('zernio_telegram_account_id', '')
ON CONFLICT (key) DO NOTHING;

COMMIT;
```

- [ ] **Step 2: ทดสอบบน Neon branch (ห้ามรันบน production branch)**

สร้าง branch ทดสอบด้วย Neon MCP (`create_branch`) จาก project `snowy-grass-75448787` ชื่อ `test-mig-081`
แล้วรัน SQL ข้างบนบน branch นั้น

- [ ] **Step 3: ยืนยันผลบน branch ทดสอบ**

```sql
SELECT
  (SELECT COUNT(*) FROM clip_metadata WHERE zernio_telegram_post_id = 'skipped-backfill') AS backfilled,
  (SELECT COUNT(*) FROM clips WHERE status = 'published') AS published_clips,
  (SELECT value = '' FROM settings WHERE key = 'zernio_telegram_account_id') AS setting_empty;
```

Expected: `backfilled` = `published_clips` (เท่ากันเป๊ะ) และ `setting_empty` = `true`

- [ ] **Step 4: รันซ้ำอีกรอบบน branch เดิม แล้วรัน query ข้อ 3 ใหม่ (พิสูจน์ว่า idempotent)**

Expected: ตัวเลขเท่าเดิมทุกช่อง ไม่มี error

- [ ] **Step 5: ลบ branch ทดสอบ (`delete_branch`) แล้วคอมมิต**

```bash
git add migrations/081_telegram_channel_post.sql
git commit -m "feat(publisher): migration 081 คอลัมน์ zernio_telegram_post_id + setting ช่อง Telegram"
```

---

## Task 2: `GetPost()` อ่านลิงก์โพสต์จริงของ YouTube

**Files:**
- Modify: `internal/publisher/zernio.go:126-129` (struct `PostStatus`) และ `:305-337` (ฟังก์ชัน `GetPost`)
- Test: `internal/publisher/zernio_parse_test.go` (เพิ่มเทสต์ในไฟล์เดิม)

**Interfaces:**
- Consumes: `ZernioClient.GetPost(ctx, id) (*PostStatus, error)` เดิม
- Produces:
  ```go
  type PostPlatform struct {
      Platform        string `json:"platform"`
      Status          string `json:"status"`
      PlatformPostID  string `json:"platformPostId"`
      PlatformPostURL string `json:"platformPostUrl"`
  }
  type PostStatus struct {
      ID        string
      Status    string
      Platforms []PostPlatform
  }
  ```
  Task 4 และ Task 5 ใช้ `PostStatus.Platforms`

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

เพิ่มท้ายไฟล์ `internal/publisher/zernio_parse_test.go` (ไฟล์นี้ใช้ package `publisher` และ import
`context`, `net/http`, `net/http/httptest`, `testing` อยู่แล้ว — เพิ่ม import ที่ขาดตามที่คอมไพเลอร์ฟ้อง):

```go
// โครงสร้าง response ตัดมาจากของจริง (GET /posts/6a7889575925ec7a11ae2273 เมื่อ 2026-08-09)
func TestGetPost_ParsesPlatformPostURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"post":{
			"_id":"P1",
			"status":"published",
			"platforms":[{
				"platform":"youtube",
				"status":"published",
				"platformPostId":"iJq2W3-vRbs",
				"platformPostUrl":"https://www.youtube.com/watch?v=iJq2W3-vRbs"
			}]
		}}`))
	}))
	defer srv.Close()

	z := newTestZernioClient(srv.URL, "k")
	ps, err := z.GetPost(context.Background(), "P1")
	if err != nil {
		t.Fatalf("GetPost err: %v", err)
	}
	if ps.ID != "P1" || ps.Status != "published" {
		t.Fatalf("expected P1/published, got %+v", ps)
	}
	if len(ps.Platforms) != 1 {
		t.Fatalf("expected 1 platform entry, got %d", len(ps.Platforms))
	}
	if ps.Platforms[0].Platform != "youtube" {
		t.Fatalf("expected platform youtube, got %q", ps.Platforms[0].Platform)
	}
	if ps.Platforms[0].PlatformPostURL != "https://www.youtube.com/watch?v=iJq2W3-vRbs" {
		t.Fatalf("unexpected url %q", ps.Platforms[0].PlatformPostURL)
	}
	if ps.Platforms[0].PlatformPostID != "iJq2W3-vRbs" {
		t.Fatalf("unexpected video id %q", ps.Platforms[0].PlatformPostID)
	}
}

func TestGetPost_NoPlatformsKeepsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"post":{"_id":"P2","status":"scheduled"}}`))
	}))
	defer srv.Close()

	z := newTestZernioClient(srv.URL, "k")
	ps, err := z.GetPost(context.Background(), "P2")
	if err != nil {
		t.Fatalf("GetPost err: %v", err)
	}
	if ps.Status != "scheduled" || len(ps.Platforms) != 0 {
		t.Fatalf("expected scheduled with no platforms, got %+v", ps)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

Run: `go test ./internal/publisher/ -run TestGetPost -v`
Expected: FAIL — คอมไพล์ไม่ผ่าน `ps.Platforms undefined`

- [ ] **Step 3: แก้ `zernio.go` ให้น้อยที่สุดเท่าที่ทำให้ผ่าน**

แทนที่ struct `PostStatus` (บรรทัด 126-129) ด้วย:

```go
// PostPlatform คือรายการต่อแพลตฟอร์มใน GET /posts/{id} · PlatformPostURL คือลิงก์จริงบน
// แพลตฟอร์มปลายทาง (เช่น https://www.youtube.com/watch?v=xxxx) ซึ่งมาพร้อมทันทีที่โพสต์
// publish เสร็จ — ไม่ต้องรอ FetchAnalytics ที่รันวันละครั้ง
type PostPlatform struct {
	Platform        string `json:"platform"`
	Status          string `json:"status"`
	PlatformPostID  string `json:"platformPostId"`
	PlatformPostURL string `json:"platformPostUrl"`
}

type PostStatus struct {
	ID        string
	Status    string
	Platforms []PostPlatform
}
```

แล้วในฟังก์ชัน `GetPost` เปลี่ยน struct ที่ใช้ unmarshal และ return:

```go
	var parsed struct {
		Post struct {
			ID        string         `json:"_id"`
			Status    string         `json:"status"`
			Platforms []PostPlatform `json:"platforms"`
		} `json:"post"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse post: %w", err)
	}
	return &PostStatus{ID: parsed.Post.ID, Status: parsed.Post.Status, Platforms: parsed.Post.Platforms}, nil
```

- [ ] **Step 4: รันเทสต์ทั้ง package**

Run: `go test ./internal/publisher/ -v`
Expected: PASS ทั้งหมด (เทสต์ `adoptDuplicate` เดิมยังผ่าน เพราะ `PostStatus.Status` ยังอยู่ที่เดิม)

- [ ] **Step 5: คอมมิต**

```bash
git add internal/publisher/zernio.go internal/publisher/zernio_parse_test.go
git commit -m "feat(publisher): GetPost อ่าน platformPostUrl ของ YouTube เพิ่ม"
```

---

## Task 3: ประกอบข้อความ Telegram (ฟังก์ชันบริสุทธิ์)

**Files:**
- Create: `internal/publisher/telegram.go`
- Create: `internal/publisher/telegram_test.go`
- Modify: `internal/publisher/publisher.go:48-59` (rename `cleanTikTokHook` → `stripBrandTag`), `:41`, `:413`
- Modify: `internal/publisher/caption_test.go:15,16,27` (ชื่อฟังก์ชันในเทสต์เดิม)

**Interfaces:**
- Consumes: `stripBrandTag(title string) string` (ฟังก์ชันเดิม เปลี่ยนแค่ชื่อ — ตัด `| Ads Vance` ท้ายชื่อ และจำกัด 120 รูน)
- Produces:
  - `const platformTelegram = "telegram"`
  - `telegramMessage(title, videoURL string) string`
  - `telegramPlatforms(accountID string) []PlatformTarget`
  - `youtubePostURL(ps *PostStatus) string`

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

สร้าง `internal/publisher/telegram_test.go`:

```go
package publisher

import (
	"strings"
	"testing"
)

func TestTelegramMessage(t *testing.T) {
	cases := []struct {
		name  string
		title string
		url   string
		want  string
	}{
		{
			name:  "ตัดแบรนด์แท็กท้ายชื่อ",
			title: "แยกผลรายตำแหน่งใน Ads Manager แล้วตัดตัวเผางบ | Ads Vance",
			url:   "https://www.youtube.com/watch?v=iJq2W3-vRbs",
			want:  "แยกผลรายตำแหน่งใน Ads Manager แล้วตัดตัวเผางบ\n\nhttps://www.youtube.com/watch?v=iJq2W3-vRbs",
		},
		{
			name:  "ไม่มีแบรนด์แท็กก็ส่งตามเดิม",
			title: "ตั้งงบยังไงไม่ให้บาน",
			url:   "https://www.youtube.com/watch?v=abc",
			want:  "ตั้งงบยังไงไม่ให้บาน\n\nhttps://www.youtube.com/watch?v=abc",
		},
		{
			name:  "escape อักขระที่ Telegram ตีเป็น HTML",
			title: "A&B <คู่แข่ง> ใครแพงกว่า",
			url:   "https://www.youtube.com/watch?v=abc",
			want:  "A&amp;B &lt;คู่แข่ง&gt; ใครแพงกว่า\n\nhttps://www.youtube.com/watch?v=abc",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := telegramMessage(c.title, c.url); got != c.want {
				t.Errorf("telegramMessage() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestTelegramPlatforms(t *testing.T) {
	got := telegramPlatforms("tg_acc")
	if len(got) != 1 {
		t.Fatalf("expected 1 target, got %d", len(got))
	}
	if got[0].Platform != platformTelegram || got[0].AccountID != "tg_acc" {
		t.Fatalf("unexpected target %+v", got[0])
	}
	// ไม่แนบ platformSpecificData: Zernio ใช้ parseMode HTML เป็นค่า default อยู่แล้ว
	// และฟิลด์นี้ผูก type กับ YouTubeOptions — แนบไปจะทำให้ต้องแก้เส้นทาง YouTube โดยไม่จำเป็น
	if got[0].PlatformSpecificData != nil {
		t.Fatalf("expected nil PlatformSpecificData, got %+v", got[0].PlatformSpecificData)
	}
}

func TestYoutubePostURL(t *testing.T) {
	ps := &PostStatus{Platforms: []PostPlatform{
		{Platform: "telegram", PlatformPostURL: "https://t.me/x/1"},
		{Platform: "youtube", PlatformPostURL: "https://www.youtube.com/watch?v=abc"},
	}}
	if got := youtubePostURL(ps); got != "https://www.youtube.com/watch?v=abc" {
		t.Fatalf("expected youtube url, got %q", got)
	}
	if got := youtubePostURL(&PostStatus{}); got != "" {
		t.Fatalf("expected empty string when no platforms, got %q", got)
	}
	// ยังไม่มีลิงก์ = ยังไม่พร้อมส่ง ต้องคืนค่าว่าง ไม่ใช่เดา URL เอง
	noURL := &PostStatus{Platforms: []PostPlatform{{Platform: "youtube", PlatformPostID: "abc"}}}
	if got := youtubePostURL(noURL); got != "" {
		t.Fatalf("expected empty string when url missing, got %q", got)
	}
}

func TestStripBrandTagStillUsedByTikTok(t *testing.T) {
	if got := stripBrandTag("ตั้งงบยังไง | Ads Vance"); !strings.HasSuffix(got, "ตั้งงบยังไง") {
		t.Fatalf("stripBrandTag ไม่ได้ตัดแบรนด์แท็ก: %q", got)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

Run: `go test ./internal/publisher/ -run "TestTelegram|TestYoutubePostURL|TestStripBrandTag" -v`
Expected: FAIL — `undefined: telegramMessage`, `undefined: stripBrandTag`

- [ ] **Step 3: เปลี่ยนชื่อฟังก์ชันเดิม (พฤติกรรมไม่เปลี่ยนแม้แต่นิดเดียว)**

ใน `internal/publisher/publisher.go` เปลี่ยน `cleanTikTokHook` เป็น `stripBrandTag` ทั้ง 3 จุด
(นิยามที่บรรทัด 50, ที่เรียกในบรรทัด 41 และ 413) และแก้คอมเมนต์เหนือฟังก์ชันเป็น:

```go
// stripBrandTag ตัดแบรนด์แท็ก "| Ads Vance" ท้ายชื่อคลิปและจำกัดความยาว เพื่อให้ข้อความ
// อ่านเป็นพาดหัวไม่ใช่บรรทัด SEO · ใช้ทั้งแคปชัน TikTok และข้อความที่ส่งเข้าช่อง Telegram
```

แล้วแก้ชื่อในเทสต์เดิม `internal/publisher/caption_test.go` (3 จุด: บรรทัด 15, 16, 27)

- [ ] **Step 4: เขียน `internal/publisher/telegram.go`**

```go
package publisher

import "strings"

const platformTelegram = "telegram"

// Zernio ส่งข้อความ Telegram ด้วย parseMode "HTML" เป็นค่า default ⇒ ตัวอักษรสามตัวนี้ใน
// ชื่อคลิปจะถูกตีความเป็นแท็ก ต้องแปลงก่อนเสมอ · ต้องแปลง & ก่อนตัวอื่นเสมอ ไม่งั้นจะไป
// แปลง & ที่ตัวเองเพิ่งสร้างซ้ำ (strings.NewReplacer สแกนรอบเดียวจึงปลอดภัยอยู่แล้ว)
var telegramHTMLEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// telegramMessage ประกอบข้อความที่จะขึ้นช่อง: พาดหัวบรรทัดแรก เว้นบรรทัด แล้วลิงก์เปล่า
// ปล่อยลิงก์เปล่าโดยตั้งใจ เพราะ Telegram จะดึงการ์ดพรีวิว (ภาพปก + ชื่อคลิป + ชื่อช่อง)
// จาก YouTube มาแสดงให้เอง — ใส่ <a href> ครอบแล้วการ์ดจะหาย
func telegramMessage(title, videoURL string) string {
	return telegramHTMLEscaper.Replace(stripBrandTag(title)) + "\n\n" + videoURL
}

// telegramPlatforms ประกอบเป้าหมายโพสต์ฝั่ง Telegram · ไม่แนบ platformSpecificData เพราะ
// ค่า default ของ Zernio (parseMode HTML) คือสิ่งที่เราต้องการอยู่แล้ว และฟิลด์นั้นผูก type
// ไว้กับ YouTubeOptions — เลี่ยงการแก้เส้นทาง YouTube เพื่อฟีเจอร์ที่ไม่ต้องใช้
func telegramPlatforms(accountID string) []PlatformTarget {
	return []PlatformTarget{{Platform: platformTelegram, AccountID: accountID}}
}

// youtubePostURL ดึงลิงก์คลิปจริงจากผลของ GetPost · คืนค่าว่างเมื่อยังไม่มี แปลว่า "ยังไม่พร้อม
// ส่ง" ไม่ใช่ "ประกอบ URL เองจาก video id" — เดา URL ผิดรูป (watch vs shorts) แล้วช่องจะได้
// ลิงก์เสียโดยไม่มีใครรู้
func youtubePostURL(ps *PostStatus) string {
	for _, pl := range ps.Platforms {
		if pl.Platform == platformYouTube && pl.PlatformPostURL != "" {
			return pl.PlatformPostURL
		}
	}
	return ""
}
```

- [ ] **Step 5: รันเทสต์ทั้ง package**

Run: `go test ./internal/publisher/ -v`
Expected: PASS ทั้งหมด รวมเทสต์ TikTok เดิมที่เพิ่งเปลี่ยนชื่อฟังก์ชัน

- [ ] **Step 6: คอมมิต**

```bash
git add internal/publisher/telegram.go internal/publisher/telegram_test.go internal/publisher/publisher.go internal/publisher/caption_test.go
git commit -m "feat(publisher): ตัวประกอบข้อความ Telegram + rename cleanTikTokHook เป็น stripBrandTag"
```

---

## Task 4: ส่งโพสต์เข้า Telegram และบันทึกผล

**Files:**
- Modify: `internal/publisher/telegram.go`
- Modify: `internal/publisher/telegram_test.go`

**Interfaces:**
- Consumes: `telegramMessage`, `telegramPlatforms` (Task 3), `postRequestID(clipID, format)` (`publisher.go:94`), `DuplicatePostError` (`zernio.go:118`)
- Produces: `func (p *Publisher) sendTelegram(ctx context.Context, clipID, accountID, title, videoURL string)` — ไม่คืน error โดยตั้งใจ (ทุกความล้มเหลวจบที่ log)

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

เพิ่มใน `internal/publisher/telegram_test.go` (เพิ่ม import `context`, `encoding/json`, `io`, `net/http`, `net/http/httptest`):

```go
func TestSendTelegram_PostsExpectedPayload(t *testing.T) {
	var body map[string]any
	var reqID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		reqID = r.Header.Get("x-request-id")
		_, _ = w.Write([]byte(`{"post":{"_id":"TG1"}}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	p.sendTelegram(context.Background(), "clip-1", "tg_acc",
		"ตั้งงบยังไงไม่ให้บาน | Ads Vance", "https://www.youtube.com/watch?v=abc")

	if body["content"] != "ตั้งงบยังไงไม่ให้บาน\n\nhttps://www.youtube.com/watch?v=abc" {
		t.Fatalf("unexpected content: %v", body["content"])
	}
	if body["publishNow"] != true {
		t.Fatalf("expected publishNow=true, got %v", body["publishNow"])
	}
	platforms, ok := body["platforms"].([]any)
	if !ok || len(platforms) != 1 {
		t.Fatalf("expected 1 platform, got %v", body["platforms"])
	}
	first := platforms[0].(map[string]any)
	if first["platform"] != "telegram" || first["accountId"] != "tg_acc" {
		t.Fatalf("unexpected platform target: %v", first)
	}
	// Telegram ไม่มีชื่อโพสต์ ส่ง title ไปก็ไม่มีที่ลง — ต้องไม่ติดไปกับคำขอ
	if _, has := body["title"]; has {
		t.Fatalf("expected no title field, got %v", body["title"])
	}
	if reqID != postRequestID("clip-1", "telegram") {
		t.Fatalf("expected deterministic x-request-id, got %q", reqID)
	}
}

func TestSendTelegram_ErrorDoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	// ต้องกลับมาเงียบๆ ไม่ panic ไม่ค้าง — คลิปขึ้น YouTube ไปแล้ว ห้ามให้ Telegram ทำพัง
	p.sendTelegram(context.Background(), "clip-1", "tg_acc", "หัวข้อ", "https://youtu.be/x")
}
```

หมายเหตุสำหรับผู้ลงมือ: เทสต์สองตัวนี้สร้าง `Publisher` โดยไม่ใส่ `pool` ⇒ ขั้นบันทึกลง DB
ต้องข้ามเมื่อ `p.pool == nil` (ดู Step 3) — นี่คือเหตุผลเดียวที่มี guard นั้น เขียนคอมเมนต์กำกับไว้

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

Run: `go test ./internal/publisher/ -run TestSendTelegram -v`
Expected: FAIL — `p.sendTelegram undefined`

- [ ] **Step 3: เพิ่มโค้ดใน `internal/publisher/telegram.go`**

เพิ่ม import `context`, `errors`, `log` แล้วต่อท้ายไฟล์:

```go
// sendTelegram โพสต์ข้อความเข้าช่องแล้วบันทึก post id · ไม่คืน error โดยตั้งใจ: คลิปขึ้น
// YouTube สำเร็จไปแล้ว ความล้มเหลวตรงนี้ต้องไม่ไหลย้อนไปเปลี่ยนสถานะคลิปหรือเขียน fail_reason
func (p *Publisher) sendTelegram(ctx context.Context, clipID, accountID, title, videoURL string) {
	res, err := p.zernio.Post(ctx, PostRequest{
		Content:    telegramMessage(title, videoURL),
		Platforms:  telegramPlatforms(accountID),
		PublishNow: true,
		RequestID:  postRequestID(clipID, "telegram"),
	})

	postID := ""
	switch {
	case err == nil:
		postID = res.Post.ID
	default:
		// 409 = Zernio เห็นเนื้อหาซ้ำใน 24 ชม. แปลว่าข้อความนี้เคยส่งเข้าช่องไปแล้ว
		// บันทึก id เดิมเพื่อไม่ให้รอบเก็บตกวนมาลองใหม่ทุกชั่วโมง
		var dup *DuplicatePostError
		if errors.As(err, &dup) {
			postID = dup.ExistingPostID
			log.Printf("Telegram: คลิป %s ซ้ำกับโพสต์ %s ที่เคยส่งแล้ว บันทึกเป็นส่งแล้ว", clipID, postID)
		} else {
			log.Printf("Telegram: ส่งคลิป %s เข้าช่องไม่สำเร็จ: %v", clipID, err)
			return
		}
	}
	if postID == "" {
		log.Printf("Telegram: คลิป %s ไม่ได้ post id กลับมา ข้ามการบันทึก", clipID)
		return
	}

	// pool เป็น nil เฉพาะในเทสต์ที่ตรวจรูปคำขอ HTTP เท่านั้น — เส้นทางจริงมี pool เสมอ
	if p.pool == nil {
		return
	}
	if _, err := p.pool.Exec(ctx,
		`UPDATE clip_metadata SET zernio_telegram_post_id = $2 WHERE clip_id = $1`,
		clipID, postID); err != nil {
		// บันทึกพลาด = รอบเก็บตกจะลองใหม่ แล้ว x-request-id เดิมทำให้ Zernio คืนโพสต์เดิม
		// แทนการโพสต์ซ้ำ จึงแค่ log พอ
		log.Printf("Telegram: บันทึก telegram_post_id ของคลิป %s ไม่สำเร็จ: %v", clipID, err)
		return
	}
	log.Printf("Telegram: ส่งคลิป %s เข้าช่องแล้ว → %s", clipID, postID)
}
```

- [ ] **Step 4: รันเทสต์**

Run: `go test ./internal/publisher/ -v`
Expected: PASS ทั้งหมด

- [ ] **Step 5: คอมมิต**

```bash
git add internal/publisher/telegram.go internal/publisher/telegram_test.go
git commit -m "feat(publisher): ส่งโพสต์เข้าช่อง Telegram + บันทึก post id + จับ 409 เป็นส่งแล้ว"
```

---

## Task 5: ต่อเข้ารอบส่งจริง (เส้นทางคลิปใหม่)

**Files:**
- Modify: `internal/publisher/telegram.go` (เพิ่ม `postTelegramForClip`)
- Modify: `internal/publisher/publisher.go:160-169` (อ่าน setting ต้นรอบ) และ `:280-286` (เรียกหลังบันทึก published)
- Modify: `internal/publisher/telegram_test.go`

**Interfaces:**
- Consumes: `sendTelegram` (Task 4), `youtubePostURL` (Task 3), `ZernioClient.GetPost` (Task 2)
- Produces: `func (p *Publisher) postTelegramForClip(ctx context.Context, clipID, accountID, title, mainPostID, shortsPostID string)`

- [ ] **Step 1: เขียนเทสต์ที่ยังไม่ผ่าน**

เพิ่มใน `internal/publisher/telegram_test.go`:

```go
func TestPostTelegramForClip_DisabledWhenNoAccount(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"post":{"_id":"X"}}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	p.postTelegramForClip(context.Background(), "clip-1", "", "หัวข้อ", "MAIN", "SHORTS")

	if calls != 0 {
		t.Fatalf("account id ว่าง = ปิดสนิท ต้องไม่ยิง HTTP เลย แต่ยิงไป %d ครั้ง", calls)
	}
}

func TestPostTelegramForClip_PrefersMainPostAndPostsOnce(t *testing.T) {
	var gotPaths []string
	var content string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"post":{"_id":"MAIN","status":"published","platforms":[
				{"platform":"youtube","status":"published","platformPostId":"vid1","platformPostUrl":"https://www.youtube.com/watch?v=vid1"}]}}`))
			return
		}
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		content, _ = body["content"].(string)
		_, _ = w.Write([]byte(`{"post":{"_id":"TG1"}}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	p.postTelegramForClip(context.Background(), "clip-1", "tg_acc", "หัวข้อ", "MAIN", "SHORTS")

	if len(gotPaths) != 2 {
		t.Fatalf("คาด GET /posts/MAIN แล้ว POST /posts อย่างละครั้ง ได้ %v", gotPaths)
	}
	if gotPaths[0] != "GET /posts/MAIN" {
		t.Fatalf("ต้องใช้โพสต์ 16:9 ก่อนเสมอ ได้ %q", gotPaths[0])
	}
	if !strings.Contains(content, "https://www.youtube.com/watch?v=vid1") {
		t.Fatalf("ข้อความไม่มีลิงก์ที่ได้จาก GetPost: %q", content)
	}
}

func TestPostTelegramForClip_FallsBackToShorts(t *testing.T) {
	var firstPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if firstPath == "" {
			firstPath = r.URL.Path
		}
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"post":{"_id":"SHORTS","status":"published","platforms":[
				{"platform":"youtube","platformPostUrl":"https://www.youtube.com/watch?v=vid2"}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"post":{"_id":"TG1"}}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	p.postTelegramForClip(context.Background(), "clip-1", "tg_acc", "หัวข้อ", "", "SHORTS")

	if firstPath != "/posts/SHORTS" {
		t.Fatalf("ไม่มีโพสต์ 16:9 ต้องใช้ Shorts แทน ได้ %q", firstPath)
	}
}

func TestPostTelegramForClip_NoURLSkipsSending(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// ยังไม่มี platformPostUrl (Zernio ยังไม่อัปเดต) = ยังไม่พร้อมส่ง
			_, _ = w.Write([]byte(`{"post":{"_id":"MAIN","status":"publishing","platforms":[{"platform":"youtube"}]}}`))
			return
		}
		posts++
		_, _ = w.Write([]byte(`{"post":{"_id":"TG1"}}`))
	}))
	defer srv.Close()

	p := &Publisher{zernio: newTestZernioClient(srv.URL, "k")}
	p.postTelegramForClip(context.Background(), "clip-1", "tg_acc", "หัวข้อ", "MAIN", "")

	if posts != 0 {
		t.Fatalf("ไม่มีลิงก์ต้องไม่ส่ง ปล่อยให้รอบเก็บตกจัดการ แต่ส่งไป %d ครั้ง", posts)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าไม่ผ่าน**

Run: `go test ./internal/publisher/ -run TestPostTelegramForClip -v`
Expected: FAIL — `p.postTelegramForClip undefined`

- [ ] **Step 3: เพิ่มฟังก์ชันใน `internal/publisher/telegram.go`**

```go
// postTelegramForClip หาลิงก์คลิปจริงจากโพสต์ที่เพิ่งขึ้น แล้วส่งเข้าช่อง · เรียกครั้งเดียว
// ต่อคลิปแม้จะมีทั้ง 16:9 และ 9:16 — ช่องไม่ควรได้ลิงก์คลิปเดียวกันสองใบ
func (p *Publisher) postTelegramForClip(ctx context.Context, clipID, accountID, title, mainPostID, shortsPostID string) {
	if accountID == "" {
		return // ฟีเจอร์ปิด: ไม่ยิงอะไรออกไปเลย
	}
	// 16:9 ก่อนเสมอ เพราะเป็นหน้าคลิปเต็มที่ดูได้ทุกอุปกรณ์ · ตอนนี้ pipeline ผลิต 9:16
	// อย่างเดียว ทางนี้จึงตกไป Shorts เป็นปกติ
	postID := mainPostID
	if postID == "" {
		postID = shortsPostID
	}
	if postID == "" {
		return
	}

	ps, err := p.zernio.GetPost(ctx, postID)
	if err != nil {
		log.Printf("Telegram: อ่านโพสต์ %s ของคลิป %s ไม่สำเร็จ: %v", postID, clipID, err)
		return
	}
	videoURL := youtubePostURL(ps)
	if videoURL == "" {
		log.Printf("Telegram: โพสต์ %s ของคลิป %s ยังไม่มีลิงก์ YouTube — ปล่อยให้รอบเก็บตกจัดการ", postID, clipID)
		return
	}
	p.sendTelegram(ctx, clipID, accountID, title, videoURL)
}
```

- [ ] **Step 4: ต่อเข้า `PublishReady` ใน `internal/publisher/publisher.go`**

หลังบล็อกที่อ่าน `firstComment` (บรรทัด ~166-168) เพิ่มการอ่าน setting ครั้งเดียวต่อรอบ:

```go
	// ช่อง Telegram ที่จะส่งคลิปเข้าไปหลังขึ้น YouTube · ค่าว่าง (หรือไม่มีแถวนี้) = ปิดฟีเจอร์
	// อ่านครั้งเดียวต่อรอบเหมือน ytAccountID — Scan ที่ error ทิ้งค่าไว้เป็น "" ซึ่งคือสถานะปิดพอดี
	var tgAccountID string
	_ = p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'zernio_telegram_account_id'`).Scan(&tgAccountID)
	tgAccountID = strings.TrimSpace(tgAccountID)
```

แล้วต่อท้ายบล็อกสุดท้ายของ loop (หลังบรรทัด `log.Printf("Published clip %s via Zernio ...")` บรรทัด ~286):

```go
		// ส่งเข้าช่อง Telegram ท้ายสุดเสมอ — คลิปขึ้น YouTube และ DB commit ไปแล้ว
		// อะไรที่พังหลังจากนี้จึงกระทบแค่ช่อง Telegram ช่องเดียว
		p.postTelegramForClip(ctx, clipID, tgAccountID, title, mainPostID, shortsPostID)
```

- [ ] **Step 5: รันเทสต์ทั้งชุด + build**

Run: `go build ./... && go test ./internal/... `
Expected: build ผ่าน เทสต์ PASS ทั้งหมด

- [ ] **Step 6: คอมมิต**

```bash
git add internal/publisher/telegram.go internal/publisher/telegram_test.go internal/publisher/publisher.go
git commit -m "feat(publisher): รอบส่งคลิปยิงเข้าช่อง Telegram ต่อท้ายเมื่อขึ้น YouTube สำเร็จ"
```

---

## Task 6: รอบเก็บตกในกรอบ 24 ชั่วโมง

**Files:**
- Modify: `internal/publisher/telegram.go`
- Modify: `internal/publisher/publisher.go` (เรียกท้าย `PublishReady` ก่อน `return nil` บรรทัด ~288-291)

**Interfaces:**
- Consumes: `postTelegramForClip` (Task 5), คอลัมน์ `zernio_telegram_post_id` (Task 1)
- Produces: `func (p *Publisher) sweepTelegramBacklog(ctx context.Context, accountID string)`

- [ ] **Step 1: เพิ่มฟังก์ชันใน `internal/publisher/telegram.go`**

(ตรรกะนี้ทดสอบด้วย unit test ไม่ได้เพราะเป็น SQL ล้วน — ขั้นพิสูจน์อยู่ใน Step 2 และ Task 8
ซึ่งรัน query จริงบน Neon branch)

```go
// sweepTelegramBacklog เก็บตกคลิปที่ขึ้น YouTube แล้วแต่ยังไม่ได้เข้าช่อง (เช่นรอบที่ Zernio
// ล่มพอดี หรือรอบที่ยังไม่มี platformPostUrl) · ทีละ 1 คลิปต่อรอบเหมือน PublishTikTok
//
// กรอบ 24 ชั่วโมงคือสิ่งเดียวที่กันไม่ให้คลังคลิปเก่าทั้งหมดถูกยิงเข้าช่องรวดเดียว ห้ามถอด
// (migration 081 ปั๊ม 'skipped-backfill' ให้คลิปที่ published อยู่ก่อนแล้ว จึงไม่มีคลิปเก่า
// ค้างในกรอบนี้ตอน deploy ครั้งแรก)
func (p *Publisher) sweepTelegramBacklog(ctx context.Context, accountID string) {
	if accountID == "" {
		return
	}
	var clipID, title string
	var mainPostID, shortsPostID *string
	err := p.pool.QueryRow(ctx, `
		SELECT c.id, cm.youtube_title, cm.zernio_post_id, cm.zernio_shorts_post_id
		FROM clips c
		JOIN clip_metadata cm ON cm.clip_id = c.id
		WHERE c.status = 'published'
		  AND c.updated_at > NOW() - INTERVAL '24 hours'
		  AND (cm.zernio_telegram_post_id IS NULL OR cm.zernio_telegram_post_id = '')
		  AND (COALESCE(cm.zernio_post_id, '') <> '' OR COALESCE(cm.zernio_shorts_post_id, '') <> '')
		ORDER BY c.updated_at ASC LIMIT 1`).
		Scan(&clipID, &title, &mainPostID, &shortsPostID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Telegram: หาคลิปเก็บตกไม่สำเร็จ: %v", err)
		}
		return
	}

	main, shorts := "", ""
	if mainPostID != nil {
		main = *mainPostID
	}
	if shortsPostID != nil {
		shorts = *shortsPostID
	}
	log.Printf("Telegram: เก็บตกคลิป %s ที่ยังไม่ได้เข้าช่อง", clipID)
	p.postTelegramForClip(ctx, clipID, accountID, sanitizeYouTubeText(title), main, shorts)
}
```

เพิ่ม import `github.com/jackc/pgx/v5` ในไฟล์

- [ ] **Step 2: ทดสอบ query บน Neon branch**

สร้าง branch `test-tg-sweep` จาก production แล้วรัน migration 081 (Task 1) ตามด้วย:

```sql
-- จำลองคลิปที่ยังไม่ได้เข้าช่อง: ล้าง marker ของคลิป published ตัวล่าสุด
UPDATE clip_metadata SET zernio_telegram_post_id = NULL
WHERE clip_id = (SELECT id FROM clips WHERE status='published' ORDER BY updated_at DESC LIMIT 1);

-- query เดียวกับในโค้ด
SELECT c.id, c.updated_at, cm.zernio_shorts_post_id
FROM clips c JOIN clip_metadata cm ON cm.clip_id = c.id
WHERE c.status = 'published'
  AND c.updated_at > NOW() - INTERVAL '24 hours'
  AND (cm.zernio_telegram_post_id IS NULL OR cm.zernio_telegram_post_id = '')
  AND (COALESCE(cm.zernio_post_id,'') <> '' OR COALESCE(cm.zernio_shorts_post_id,'') <> '')
ORDER BY c.updated_at ASC LIMIT 1;
```

Expected: คืน **1 แถว** คือคลิปที่เพิ่งล้าง marker · แล้วรันซ้ำโดยเปลี่ยนเงื่อนไขเป็น
`INTERVAL '1 minute'` ต้องคืน **0 แถว** (พิสูจน์ว่ากรอบเวลาทำงานจริง)

- [ ] **Step 3: เรียกใช้ท้าย `PublishReady`**

ใน `internal/publisher/publisher.go` ก่อน `return nil` สุดท้ายของ `PublishReady` (หลังบล็อก
`if err := rows.Err(); ...`):

```go
	// เก็บตกท้ายรอบ: คลิปที่ขึ้น YouTube แล้วแต่ยังไม่เข้าช่อง Telegram (จำกัด 24 ชม.)
	p.sweepTelegramBacklog(ctx, tgAccountID)
	return nil
```

**ข้อควรระวัง:** `rows` ถูก `defer rows.Close()` ไว้ แต่ `sweepTelegramBacklog` ใช้ `p.pool`
คนละ connection จึงไม่ชนกัน · โค้ดนี้ต้องอยู่ **หลัง** loop จบแล้วเท่านั้น

- [ ] **Step 4: build + รันเทสต์ทั้งชุด**

Run: `go build ./... && go test ./internal/...`
Expected: PASS ทั้งหมด

- [ ] **Step 5: ลบ Neon branch ทดสอบ แล้วคอมมิต**

```bash
git add internal/publisher/telegram.go internal/publisher/publisher.go
git commit -m "feat(publisher): รอบเก็บตกส่งคลิปเข้าช่อง Telegram ในกรอบ 24 ชั่วโมง"
```

---

## Task 7: เปิดให้ตั้งค่าจากหน้าเว็บ

**Files:**
- Modify: `internal/handler/settings.go:55-69` (allowlist)
- Modify: `frontend/src/pages/Settings.tsx:119-128` (เพิ่มช่องกรอกถัดจาก TikTok Account ID)

**Interfaces:**
- Consumes: setting key `zernio_telegram_account_id` (Task 1)
- Produces: ไม่มี API ใหม่ — ใช้ `PUT /api/v1/settings` เดิม

- [ ] **Step 1: เพิ่มคีย์ใน allowlist**

ใน `internal/handler/settings.go` เพิ่มบรรทัดในแมป `allowed` (ต่อจาก `"zernio_tiktok_account_id": true,`):

```go
		"zernio_telegram_account_id": true,
```

- [ ] **Step 2: เพิ่มช่องกรอกในหน้า Settings**

ใน `frontend/src/pages/Settings.tsx` ต่อจากบล็อก TikTok Account ID (ปิดที่บรรทัด 128) แทรก:

```tsx
        <div>
          <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
            Telegram Account ID (Zernio)
          </label>
          <Input
            value={form['zernio_telegram_account_id'] ?? ''}
            onChange={e => onChange('zernio_telegram_account_id', e.target.value)}
            placeholder="Zernio Telegram account ID..."
          />
          <p className="text-xs text-muted-foreground mt-1">
            เว้นว่างไว้ = ไม่ส่งคลิปเข้าช่อง Telegram
          </p>
        </div>
```

- [ ] **Step 3: ยืนยันว่าคอมไพล์ผ่านทั้งสองฝั่ง**

Run: `go build ./... && cd frontend && npm run build`
Expected: ไม่มี error ทั้งสองคำสั่ง

- [ ] **Step 4: คอมมิต**

```bash
git add internal/handler/settings.go frontend/src/pages/Settings.tsx
git commit -m "feat(settings): ช่องกรอก Telegram account ID ในหน้าตั้งค่า"
```

---

## Task 8: ตรวจงานรวมและพิสูจน์กับของจริง

**Files:** ไม่มีไฟล์ใหม่ (เป็นขั้นตรวจ)

- [ ] **Step 1: รันเทสต์ทั้งโปรเจกต์**

Run: `go test ./... 2>&1 | tail -30`
Expected: ทุก package `ok` หรือ `no test files` — ไม่มี FAIL

- [ ] **Step 2: ตรวจว่าไม่มีอะไรหลุดไปแตะเส้นทาง YouTube**

Run: `git diff master --stat && git diff master -- internal/publisher/publisher.go`
Expected: การแก้ `publisher.go` มีแค่ 4 กลุ่ม — rename `cleanTikTokHook`→`stripBrandTag`,
อ่าน `tgAccountID`, เรียก `postTelegramForClip`, เรียก `sweepTelegramBacklog` · **ไม่มีการแก้
ลำดับหรือเงื่อนไขของการโพสต์ YouTube เลย**

- [ ] **Step 3: รัน /simplify กับ diff**

ตามคำสั่งเจ้าของ: ก่อนขั้นสรุปงานต้องรัน `/simplify` กับโค้ดที่เปลี่ยน แล้วแก้ตามที่มันชี้

- [ ] **Step 4: สรุปส่งเจ้าของ พร้อมสิ่งที่ต้องทำด้วยมือ**

รายงานต้องมี:
- ผลเทสต์จริง (ตัวเลข ไม่ใช่คำว่า "ผ่านหมด")
- ขั้นตอนที่เจ้าของต้องทำเอง: เพิ่ม `@ZernioScheduleBot` เป็นแอดมินช่อง `@adsvancech`
  (ต้องมีสิทธิ์โพสต์) → ผูกบัญชีในหน้า Zernio ด้วย access code → เอา account id มากรอกใน Settings
- ย้ำว่าจนกว่าจะกรอก account id ฟีเจอร์ปิดสนิท ระบบทำงานเหมือนเดิมทุกประการ
- **ยังไม่ merge / ยังไม่ push จนกว่าเจ้าของจะสั่ง**

- [ ] **Step 5: หลังเจ้าของเปิดใช้จริง — ตรวจของจริงรอบแรก**

เมื่อคลิปใหม่ขึ้นรอบถัดไป ให้ตรวจ 3 อย่าง:
1. `railway logs` เห็นบรรทัด `Telegram: ส่งคลิป <id> เข้าช่องแล้ว → <post id>`
2. Neon: `SELECT clip_id, zernio_telegram_post_id FROM clip_metadata WHERE zernio_telegram_post_id IS NOT NULL AND zernio_telegram_post_id <> 'skipped-backfill'` มีแถวใหม่
3. เปิดช่อง `@adsvancech` เห็นข้อความขึ้นในนามชื่อช่อง (ไม่ใช่ชื่อบอท) และการ์ดพรีวิวแสดงภาพปกคลิปถูก

---

## ผลตรวจสเปกกับแผน (self-review)

| ข้อกำหนดในสเปก | ครอบคลุมที่ |
|---|---|
| Zernio เป็นช่องทางส่ง | Task 4 |
| ข้อความล้วน title + ลิงก์ ให้ Telegram ทำการ์ดพรีวิว | Task 3 |
| เริ่มนับจากคลิปใหม่เท่านั้น | Task 1 (`skipped-backfill`) + Task 6 (กรอบ 24 ชม.) |
| ส่งทันทีในรอบเดียวกัน | Task 5 |
| รอบเก็บตก 24 ชั่วโมง | Task 6 |
| ลิงก์จาก `platformPostUrl` ไม่รอ FetchAnalytics | Task 2 |
| 16:9 ก่อน 9:16 ส่งครั้งเดียว | Task 5 (เทสต์ `PrefersMainPostAndPostsOnce`) |
| ล้มแล้วไม่แตะ status / fail_reason | Task 4 (`sendTelegram` ไม่คืน error) + Task 8 Step 2 |
| กันส่งซ้ำ 2 ชั้น (คอลัมน์ + x-request-id) | Task 1 + Task 4 |
| 409 = ถือว่าส่งแล้ว | Task 4 |
| สวิตช์ค่าว่าง = ปิดสนิท | Task 5 (เทสต์ `DisabledWhenNoAccount`) + Task 7 |
| escape `& < >` | Task 3 |
| งานที่ต้องทำด้วยมือ | Task 8 Step 4 |

ข้อกำหนดในสเปกที่ **จงใจไม่ทำ** พร้อมเหตุผล — ดูหัวข้อ "หมายเหตุการตัดสินใจที่ต่างจากสเปก"
ด้านบน (ไม่ส่ง `platformSpecificData`, ไม่ตัด `#Shorts`)
