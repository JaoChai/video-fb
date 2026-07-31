# YouTube First Comment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ให้ทุกคลิปที่ระบบโพสต์ขึ้น YouTube มีคอมเมนต์ของช่องเองใต้คลิป บอกช่องทางติดต่อ (LINE @adsvance) โดยไม่ต้องเชื่อม YouTube API เอง

**Architecture:** ระบบโพสต์ผ่าน Zernio อยู่แล้ว (`internal/publisher/publisher.go`) — งานนี้แค่แนบฟิลด์ `platformSpecificData.firstComment` ไปกับคำขอโพสต์เดิม Zernio จะโพสต์คอมเมนต์และปักหมุดให้เองหลังอัปโหลดเสร็จ ข้อความมาจากตาราง `settings` คีย์ `youtube_first_comment` ค่าว่าง = ไม่ส่งฟิลด์นี้เลย = พฤติกรรมเดิมทุกประการ

**Tech Stack:** Go 1.x, pgx/v5, Neon Postgres, Zernio REST API, `net/http/httptest` สำหรับเทสต์

สเปก: `docs/superpowers/specs/2026-07-31-youtube-first-comment-design.md`

## Global Constraints

- ข้อความคอมเมนต์ค่าเริ่มต้น (คัดลอกเป๊ะ ห้ามแก้ถ้อยคำ):
  ```
  สนใจบัญชีโฆษณา / สอบถามเพิ่มเติม
  ติดต่อทีมงานได้ที่ LINE id : @adsvance

  กลุ่มข่าวสาร Telegram : https://t.me/adsvancech
  ```
- **ค่าว่างต้องเงียบสนิท**: เมื่อ `youtube_first_comment` เป็นค่าว่าง JSON ที่ยิงไป Zernio ต้อง **ไม่มีคีย์ `platformSpecificData`** เลย (ไม่ใช่ส่งเป็น `null` หรือ object ว่าง)
- **ห้ามเพิ่ม auto-retry** ที่ยิงโพสต์ซ้ำโดยตัด `firstComment` ทิ้ง — Zernio อาจ error หลังอัปวิดีโอขึ้น YouTube สำเร็จแล้ว การยิงซ้ำ = คลิปซ้ำบนช่องซึ่งเอากลับไม่ได้ พฤติกรรมเมื่อ error คงเดิม: `log.Printf` แล้ว `continue` ปล่อยคลิปเป็น `ready`
- **ห้ามยิง `/orchestrator/produce*`** ไม่ว่ากรณีใด — endpoint พวกนี้ผลิตคลิปจริงบน prod ทันที
- **ห้ามรัน backend local ต่อ prod DB** — scheduler จะยิง cron จริง
- migration ต้องหุ้ม `BEGIN;` / `COMMIT;` เอง (RunMigrations ไม่หุ้มให้)
- migration ต้องไม่ทับค่าที่ user แก้เอง ⇒ ใช้ `ON CONFLICT (key) DO NOTHING`
- ไม่แตะ TikTok, ไม่แตะคลิปเก่า, ไม่แตะ `youtube_description`

## File Structure

| ไฟล์ | หน้าที่ | Task |
|------|--------|------|
| `internal/publisher/zernio.go` | โครงสร้าง request/response ของ Zernio + การยิง HTTP — เพิ่ม `YouTubeOptions` และฟิลด์ `PlatformSpecificData`, เปลี่ยน `Post` ให้ใช้ `z.baseURL` (เพื่อให้เทสต์ดักได้) | 1 |
| `internal/publisher/zernio_test.go` | เทสต์ระดับ wire format — ยืนยัน JSON ที่ยิงจริง | 1 |
| `internal/publisher/publisher.go` | ตรรกะรอบส่ง — อ่าน setting และประกอบ platform target ต่อโพสต์ | 2 |
| `internal/publisher/first_comment_test.go` | เทสต์ helper `youtubePlatforms` (ไฟล์ใหม่ แยกจากเทสต์เดิมเพื่อให้อ่านง่าย) | 2 |
| `migrations/077_youtube_first_comment.sql` | ใส่ค่าเริ่มต้นลง `settings` | 3 |
| `internal/handler/settings.go` | allowlist ของคีย์ที่แก้จากหน้าเว็บได้ | 3 |

---

### Task 1: ทำให้ `Post` ยิงผ่าน `baseURL` และรับ `platformSpecificData`

**Files:**
- Modify: `internal/publisher/zernio.go:49-52` (struct `PlatformTarget`), `internal/publisher/zernio.go:192` (URL ที่ยิง)
- Test: `internal/publisher/zernio_test.go` (เพิ่มท้ายไฟล์)

**Interfaces:**
- Consumes: `newTestZernioClient(baseURL, apiKey string) *ZernioClient` — helper ที่มีอยู่แล้วใน `zernio_test.go:13`
- Produces:
  - `type YouTubeOptions struct { Title, Visibility, FirstComment string }` (json: `title`, `visibility`, `firstComment` ทุกตัว `omitempty`)
  - `PlatformTarget.PlatformSpecificData *YouTubeOptions` (json: `platformSpecificData,omitempty`)

**หมายเหตุสำคัญ:** ตอนนี้ `Post` ยิงไป `zernioAPI+"/posts"` (ค่าคงที่) ไม่ใช่ `z.baseURL` ⇒ เทสต์ดักคำขอไม่ได้เลย ต้องเปลี่ยนเป็น `z.baseURL` ซึ่ง `NewZernioClient` เซ็ตเป็น `zernioAPI` อยู่แล้ว (`zernio.go:36`) พฤติกรรมบน prod จึงไม่เปลี่ยน

- [ ] **Step 1: เขียนเทสต์ที่ต้อง fail ก่อน**

เพิ่มท้าย `internal/publisher/zernio_test.go`:

```go
// captureBody รัน Post กับ httptest server แล้วคืน JSON ดิบที่ยิงออกไป
func captureBody(t *testing.T, req PostRequest) string {
	t.Helper()
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"post":{"_id":"P1"}}`))
	}))
	defer srv.Close()

	z := newTestZernioClient(srv.URL, "test_key")
	if _, err := z.Post(context.Background(), req); err != nil {
		t.Fatalf("Post err: %v", err)
	}
	return body
}

func TestPost_SendsFirstCommentInPlatformSpecificData(t *testing.T) {
	body := captureBody(t, PostRequest{
		Title:   "หัวข้อคลิป",
		Content: "หัวข้อคลิป\n\nคำอธิบาย",
		Platforms: []PlatformTarget{{
			Platform:  "youtube",
			AccountID: "acc1",
			PlatformSpecificData: &YouTubeOptions{
				Title:        "หัวข้อคลิป",
				Visibility:   VisibilityPublic,
				FirstComment: "ติดต่อทีมงานได้ที่ LINE id : @adsvance",
			},
		}},
		Visibility: VisibilityPublic,
		PublishNow: true,
	})

	for _, want := range []string{
		`"platformSpecificData"`,
		`"firstComment"`,
		`LINE id : @adsvance`,
		`"visibility":"public"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %s, got %s", want, body)
		}
	}
}

func TestPost_OmitsPlatformSpecificDataWhenUnset(t *testing.T) {
	body := captureBody(t, PostRequest{
		Title:      "หัวข้อคลิป",
		Content:    "หัวข้อคลิป",
		Platforms:  []PlatformTarget{{Platform: "youtube", AccountID: "acc1"}},
		Visibility: VisibilityPublic,
		PublishNow: true,
	})

	if strings.Contains(body, "platformSpecificData") {
		t.Fatalf("expected no platformSpecificData key when unset, got %s", body)
	}
}
```

เพิ่ม `"io"` เข้า import block ของ `zernio_test.go` (ไฟล์นี้ยังไม่ได้ import `io`)

- [ ] **Step 2: รันเทสต์ให้เห็นว่า fail**

Run: `go test ./internal/publisher/ -run TestPost_ -v`
Expected: FAIL — คอมไพล์ไม่ผ่าน `undefined: YouTubeOptions` และ `unknown field PlatformSpecificData`

- [ ] **Step 3: เพิ่ม struct และฟิลด์ใน `zernio.go`**

แทนที่ `internal/publisher/zernio.go:49-52`:

```go
type PlatformTarget struct {
	Platform  string `json:"platform"`
	AccountID string `json:"accountId"`
	// PlatformSpecificData ต้องเป็น pointer + omitempty เพื่อให้ "ไม่ตั้งค่า" แปลว่า
	// คีย์นี้หายไปจาก JSON ทั้งอัน ไม่ใช่ส่ง object ว่างซึ่งอาจไปทับค่า default ของ Zernio
	PlatformSpecificData *YouTubeOptions `json:"platformSpecificData,omitempty"`
}

// YouTubeOptions คือ platformSpecificData ของ Zernio ฝั่ง YouTube
// FirstComment: Zernio โพสต์ข้อความนี้เป็นคอมเมนต์ใต้คลิปและปักหมุดให้หลังอัปโหลดเสร็จ
// (สูงสุด 10,000 ตัวอักษร) — เราจึงไม่ต้องมี OAuth ของ YouTube เอง
// Title/Visibility ส่งซ้ำกับระดับบนโดยตั้งใจ: ไม่รู้ว่า Zernio รวมค่าสองระดับยังไง
// ถ้ามันให้บล็อกนี้ชนะแล้วเราส่งมาแค่ firstComment คลิปอาจขึ้นแบบไม่มีชื่อหรือเป็น private
type YouTubeOptions struct {
	Title        string `json:"title,omitempty"`
	Visibility   string `json:"visibility,omitempty"`
	FirstComment string `json:"firstComment,omitempty"`
}
```

แก้ `internal/publisher/zernio.go:192` จาก:

```go
	httpReq, err := http.NewRequestWithContext(ctx, "POST", zernioAPI+"/posts", bytes.NewReader(body))
```

เป็น:

```go
	// ใช้ z.baseURL (ค่าเดียวกับ zernioAPI เมื่อสร้างผ่าน NewZernioClient) เพื่อให้เทสต์
	// ชี้ไป httptest server แล้วตรวจ JSON ที่ยิงจริงได้ — GetAnalytics ทำแบบนี้อยู่แล้ว
	httpReq, err := http.NewRequestWithContext(ctx, "POST", z.baseURL+"/posts", bytes.NewReader(body))
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/publisher/ -v`
Expected: PASS ทุกตัว รวมเทสต์เดิมที่มีอยู่ (`TestGetAnalytics_UsesQueryParamPostID`, `TestGetYouTubeDailyViews_AggregatesWatchTime`, ฯลฯ)

- [ ] **Step 5: Commit**

```bash
git add internal/publisher/zernio.go internal/publisher/zernio_test.go
git commit -m "feat(publish): รองรับ platformSpecificData.firstComment ในคำขอ Zernio"
```

---

### Task 2: อ่าน setting แล้วแนบคอมเมนต์ตอนโพสต์จริง

**Files:**
- Create: `internal/publisher/first_comment_test.go`
- Modify: `internal/publisher/publisher.go:108-109` (อ่าน setting), `:132-139` (สร้าง platform target), `:151` และ `:176` (ฟิลด์ `Platforms` ของสองโพสต์)

**Interfaces:**
- Consumes: `YouTubeOptions`, `PlatformTarget.PlatformSpecificData` จาก Task 1 · ค่าคงที่ `platformYouTube = "youtube"` (`publisher.go:18`) และ `VisibilityPublic = "public"` (`zernio.go:21`)
- Produces: `func youtubePlatforms(accountID, title, firstComment string) []PlatformTarget`

- [ ] **Step 1: เขียนเทสต์ที่ต้อง fail ก่อน**

สร้าง `internal/publisher/first_comment_test.go`:

```go
package publisher

import "testing"

func TestYoutubePlatforms(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		firstComment string
		wantOptions  bool
		wantTitle    string
	}{
		{
			name:         "มีข้อความ ⇒ แนบ platformSpecificData",
			title:        "แอดโดนแบนทำไง",
			firstComment: "ติดต่อทีมงานได้ที่ LINE id : @adsvance",
			wantOptions:  true,
			wantTitle:    "แอดโดนแบนทำไง",
		},
		{
			name:         "ค่าว่าง ⇒ ไม่แนบอะไรเลย",
			title:        "แอดโดนแบนทำไง",
			firstComment: "",
			wantOptions:  false,
		},
		{
			name:         "โพสต์ Shorts ใช้ title ของตัวเอง",
			title:        "แอดโดนแบนทำไง #Shorts",
			firstComment: "ติดต่อทีมงานได้ที่ LINE id : @adsvance",
			wantOptions:  true,
			wantTitle:    "แอดโดนแบนทำไง #Shorts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := youtubePlatforms("acc1", tt.title, tt.firstComment)
			if len(got) != 1 {
				t.Fatalf("expected 1 platform target, got %d", len(got))
			}
			if got[0].Platform != platformYouTube || got[0].AccountID != "acc1" {
				t.Fatalf("expected youtube/acc1, got %+v", got[0])
			}
			if !tt.wantOptions {
				if got[0].PlatformSpecificData != nil {
					t.Fatalf("expected nil PlatformSpecificData, got %+v", got[0].PlatformSpecificData)
				}
				return
			}
			opts := got[0].PlatformSpecificData
			if opts == nil {
				t.Fatalf("expected PlatformSpecificData, got nil")
			}
			if opts.FirstComment != tt.firstComment {
				t.Fatalf("expected firstComment %q, got %q", tt.firstComment, opts.FirstComment)
			}
			if opts.Title != tt.wantTitle {
				t.Fatalf("expected title %q, got %q", tt.wantTitle, opts.Title)
			}
			if opts.Visibility != VisibilityPublic {
				t.Fatalf("expected visibility %q, got %q", VisibilityPublic, opts.Visibility)
			}
		})
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่า fail**

Run: `go test ./internal/publisher/ -run TestYoutubePlatforms -v`
Expected: FAIL — คอมไพล์ไม่ผ่าน `undefined: youtubePlatforms`

- [ ] **Step 3: เขียน helper**

เพิ่มใน `internal/publisher/publisher.go` ต่อจากฟังก์ชัน `cleanTikTokHook` (ราวบรรทัด 53):

```go
// youtubePlatforms ประกอบเป้าหมายโพสต์ฝั่ง YouTube ของคำขอ Zernio
// เมื่อ firstComment ไม่ว่าง มันจะไปกับ platformSpecificData แล้ว Zernio โพสต์คอมเมนต์
// ใต้คลิปพร้อมปักหมุดให้เอง · ค่าว่าง = ไม่แนบบล็อกนี้เลย คำขอเหมือนก่อนมีฟีเจอร์นี้เป๊ะ
// รับ title เข้ามาเพราะโพสต์ 16:9 กับ 9:16 ใช้ชื่อคนละอัน (Shorts ตัดที่ 60 ตัวอักษร
// แล้วต่อท้าย #Shorts) — ส่ง target ตัวเดียวร่วมกันไม่ได้
func youtubePlatforms(accountID, title, firstComment string) []PlatformTarget {
	target := PlatformTarget{Platform: platformYouTube, AccountID: accountID}
	if firstComment != "" {
		target.PlatformSpecificData = &YouTubeOptions{
			Title:        title,
			Visibility:   VisibilityPublic,
			FirstComment: firstComment,
		}
	}
	return []PlatformTarget{target}
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/publisher/ -run TestYoutubePlatforms -v`
Expected: PASS ทั้ง 3 subtest

- [ ] **Step 5: ต่อสายเข้ารอบส่งจริง**

ที่ `internal/publisher/publisher.go:108-109` เดิม:

```go
	var ytAccountID string
	p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'zernio_youtube_account_id'`).Scan(&ytAccountID)
```

เปลี่ยนเป็น:

```go
	var ytAccountID string
	p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'zernio_youtube_account_id'`).Scan(&ytAccountID)

	// คอมเมนต์ปักหมุดใต้คลิป · อ่านครั้งเดียวต่อรอบเหมือน ytAccountID
	// ค่าว่าง (หรือไม่มีแถวนี้) = ปิดฟีเจอร์ ไม่ต้อง deploy — Scan ที่ error จะทิ้ง
	// firstComment ไว้เป็น "" ซึ่งคือสถานะปิดพอดี จึงไม่ต้องจัดการ error แยก
	var firstComment string
	_ = p.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'youtube_first_comment'`).Scan(&firstComment)
```

ที่ `internal/publisher/publisher.go:132-139` เดิม:

```go
		var platforms []PlatformTarget
		if ytAccountID != "" {
			platforms = append(platforms, PlatformTarget{Platform: "youtube", AccountID: ytAccountID})
		}
		if len(platforms) == 0 {
			log.Printf("No Zernio accounts configured, skipping clip %s", clipID)
			continue
		}
```

เปลี่ยนเป็น (สอง target สร้างแยกตอนโพสต์ เพราะชื่อคลิปคนละอัน):

```go
		if ytAccountID == "" {
			log.Printf("No Zernio accounts configured, skipping clip %s", clipID)
			continue
		}
```

ที่ `internal/publisher/publisher.go:151` (ในบล็อกโพสต์ 16:9) เปลี่ยน:

```go
				Platforms:  platforms,
```

เป็น:

```go
				Platforms:  youtubePlatforms(ytAccountID, title, firstComment),
```

ที่ `internal/publisher/publisher.go:176` (ในบล็อกโพสต์ 9:16) เปลี่ยน:

```go
				Platforms:  platforms,
```

เป็น:

```go
				Platforms:  youtubePlatforms(ytAccountID, shortsTitle, firstComment),
```

- [ ] **Step 6: รันเทสต์ทั้ง package + build**

Run: `go build ./... && go test ./internal/publisher/ -v`
Expected: build ผ่าน (ไม่มีตัวแปร `platforms` ค้างที่ไม่ถูกใช้) เทสต์ผ่านทุกตัว

- [ ] **Step 7: Commit**

```bash
git add internal/publisher/publisher.go internal/publisher/first_comment_test.go
git commit -m "feat(publish): แนบคอมเมนต์ติดต่อทีมงานไปกับโพสต์ YouTube ทั้ง 16:9 และ Shorts"
```

---

### Task 3: ค่าเริ่มต้นใน settings + แก้ได้จากหน้าเว็บ

**Files:**
- Create: `migrations/077_youtube_first_comment.sql`
- Modify: `internal/handler/settings.go:55-68` (allowlist)

**Interfaces:**
- Consumes: คีย์ `youtube_first_comment` ที่ `PublishReady` อ่านใน Task 2
- Produces: แถว `settings` คีย์ `youtube_first_comment` + สิทธิ์แก้ผ่าน `PATCH` settings

- [ ] **Step 1: เขียน migration**

สร้าง `migrations/077_youtube_first_comment.sql`:

```sql
-- 077: ข้อความคอมเมนต์ปักหมุดใต้คลิป YouTube
--
-- ระบบส่งคลิปผ่าน Zernio ซึ่งรับ platformSpecificData.firstComment แล้วโพสต์คอมเมนต์
-- ใต้คลิปพร้อมปักหมุดให้เอง (เอกสาร: docs.zernio.com/platforms/youtube) เราจึงไม่ต้อง
-- มี OAuth ของ YouTube เอง — และ YouTube Data API เองก็ปักหมุดคอมเมนต์ไม่ได้อยู่แล้ว
--
-- ทำไมต้องอยู่ใน settings ไม่ใช่ค่าคงที่ในโค้ด: ข้อความติดต่อเปลี่ยนบ่อยกว่าโค้ด และ
-- ตั้งค่าว่างคือสวิตช์ปิดฟีเจอร์ทันทีโดยไม่ต้อง deploy (publisher.go ไม่แนบ
-- platformSpecificData เลยเมื่อค่าว่าง = คำขอเหมือนก่อนมีฟีเจอร์นี้)
--
-- ON CONFLICT DO NOTHING เพราะถ้า user แก้ข้อความเองจากหน้าเว็บแล้ว การรัน migration
-- ซ้ำ (deploy ใหม่) ต้องไม่ทับค่าที่แก้ไว้
--
-- ข้อความนี้ต่างจากที่อยู่ใน youtube_description โดยตั้งใจ: บน Shorts คำอธิบายถูกซ่อน
-- ต้องกดถึงเห็น ส่วนคอมเมนต์อยู่บนหน้าจอ

BEGIN;

INSERT INTO settings (key, value) VALUES (
  'youtube_first_comment',
  $txt$สนใจบัญชีโฆษณา / สอบถามเพิ่มเติม
ติดต่อทีมงานได้ที่ LINE id : @adsvance

กลุ่มข่าวสาร Telegram : https://t.me/adsvancech$txt$
)
ON CONFLICT (key) DO NOTHING;

COMMIT;
```

- [ ] **Step 2: เพิ่มคีย์ใน allowlist**

ที่ `internal/handler/settings.go` ในแมป `allowed` (บรรทัด 55-68) เพิ่มบรรทัดต่อจาก `"topic_stats_enabled": true,`:

```go
		"youtube_first_comment":     true,
```

- [ ] **Step 3: ตรวจว่า migration ถูกไวยากรณ์ และ build ผ่าน**

Run:
```bash
go build ./... && go vet ./internal/handler/ ./internal/publisher/
```
Expected: ไม่มี output error

ตรวจไวยากรณ์ SQL ด้วยตาอีกชั้น: dollar-quote `$txt$ ... $txt$` ต้องเปิด-ปิดครบ และข้อความข้างในต้องมีบรรทัดว่างก่อนบรรทัด Telegram (คงรูปแบบ 3 บล็อกตามสเปก)

- [ ] **Step 4: รันเทสต์ทั้งโปรเจกต์**

Run: `go test ./...`
Expected: PASS (ถ้ามีเทสต์ที่ fail อยู่ก่อนหน้าโดยไม่เกี่ยวกับงานนี้ ให้บันทึกไว้ อย่าแก้)

- [ ] **Step 5: Commit**

```bash
git add migrations/077_youtube_first_comment.sql internal/handler/settings.go
git commit -m "feat(publish): migration 077 ข้อความคอมเมนต์ปักหมุด + เปิดให้แก้จากหน้าเว็บ"
```

---

### Task 4: พิสูจน์กับ Zernio จริงด้วยโพสต์ unlisted (ประตูก่อน merge)

**Files:** ไม่แก้ไฟล์ในโปรเจกต์ — สคริปต์ชั่วคราวเขียนลง scratchpad เท่านั้น

**Interfaces:**
- Consumes: `settings.zernio_api_key`, `settings.zernio_youtube_account_id`, `clips.video_9_16_url` จาก prod DB (อ่านอย่างเดียว)
- Produces: คำตอบว่า `firstComment` ทำงานจริงหรือไม่ ⇒ ตัดสินว่าจะ merge หรือหยุด

**ทำไมต้องมีขั้นนี้:** ฟิลด์ `firstComment` มาจากการอ่านเอกสาร ยังไม่เคยยิงจริงสักครั้ง ถ้า Zernio ตีความ `platformSpecificData` ต่างจากที่เข้าใจ ผลคือคลิปจริงขึ้นช่องแบบชื่อหายหรือเป็น private — ซึ่งเอากลับไม่ได้

- [ ] **Step 1: ดึงค่าที่ต้องใช้จาก prod (อ่านอย่างเดียว)**

ใช้ Neon MCP `run_sql` กับ project `snowy-grass-75448787`:

```sql
SELECT key, value FROM settings
WHERE key IN ('zernio_api_key', 'zernio_youtube_account_id', 'r2_public_base_url');
```

```sql
SELECT id, video_9_16_url FROM clips
WHERE video_9_16_url LIKE (SELECT value || '%' FROM settings WHERE key = 'r2_public_base_url')
ORDER BY created_at DESC LIMIT 1;
```

Expected: ได้ api key, accountId และ URL วิดีโอบน R2 (ต้องเป็นโดเมน `cdn.thinkclip.xyz` ไม่ใช่ `tempfile.redpandaai.co` ซึ่งหมดอายุแล้ว)

- [ ] **Step 2: ยิงโพสต์ทดสอบแบบ unlisted**

เขียนไฟล์ `$TMPDIR/zernio-test.json` (แทนค่า `<ACCOUNT_ID>` และ `<VIDEO_URL>` จาก Step 1):

```json
{
  "title": "ทดสอบระบบ ห้ามเผยแพร่ 20260731",
  "content": "ทดสอบระบบ ห้ามเผยแพร่ 20260731\n\nโพสต์ทดสอบ firstComment",
  "platforms": [{
    "platform": "youtube",
    "accountId": "<ACCOUNT_ID>",
    "platformSpecificData": {
      "title": "ทดสอบระบบ ห้ามเผยแพร่ 20260731",
      "visibility": "unlisted",
      "firstComment": "ทดสอบคอมเมนต์อัตโนมัติ 20260731 — ติดต่อทีมงานได้ที่ LINE id : @adsvance"
    }
  }],
  "mediaItems": [{"type": "video", "url": "<VIDEO_URL>"}],
  "visibility": "unlisted",
  "publishNow": true
}
```

รัน (ต้องใช้ `dangerouslyDisableSandbox: true` เพราะ sandbox บล็อกเน็ตเวิร์กทั้งหมด):

```bash
curl -sS -X POST https://zernio.com/api/v1/posts \
  -H "Authorization: Bearer <ZERNIO_API_KEY>" \
  -H "Content-Type: application/json" \
  --data-binary @"$TMPDIR/zernio-test.json"
```

Expected: HTTP 2xx และ JSON ที่มี `post._id` — จดค่านี้ไว้

**ห้ามยิงคำสั่งนี้ซ้ำ** ถ้าไม่แน่ใจว่ารอบแรกขึ้นไปหรือยัง ให้ไปเช็คในช่องก่อน (การยิงซ้ำ = วิดีโอทดสอบซ้ำบนช่อง)

- [ ] **Step 3: รอ Zernio อัปโหลด แล้วเช็คผลด้วยตา**

รอ ~3-5 นาที (คลิป 9-10MB) แล้วเปิด YouTube Studio → Content → หาวิดีโอชื่อ "ทดสอบระบบ ห้ามเผยแพร่ 20260731"

ต้องยืนยันครบ 4 ข้อ:

| # | ตรวจอะไร | ผ่านเมื่อ |
|---|----------|----------|
| 1 | คอมเมนต์ | มีคอมเมนต์ "ทดสอบคอมเมนต์อัตโนมัติ 20260731..." ใต้คลิป |
| 2 | ปักหมุด | คอมเมนต์นั้นมีป้าย "ปักหมุดโดยเจ้าของช่อง" (ถ้าไม่ปัก = ยังผ่านได้ ให้บันทึกไว้) |
| 3 | ชื่อคลิป | ตรงกับ "ทดสอบระบบ ห้ามเผยแพร่ 20260731" ไม่ว่างไม่เพี้ยน |
| 4 | visibility | เป็น "ไม่แสดงต่อสาธารณะ (unlisted)" ไม่ใช่ public |

**เกณฑ์ตัดสิน:**
- ข้อ 1, 3, 4 ผ่าน ⇒ ไปต่อ Step 4
- ข้อ 1 ไม่ผ่าน (ไม่มีคอมเมนต์เลย) ⇒ **หยุด อย่า merge** รายงานผลให้ user แล้วกลับไปคุยเรื่องทางสำรอง (YouTube Data API + OAuth ซึ่งปักหมุดไม่ได้)
- ข้อ 3 หรือ 4 ไม่ผ่าน ⇒ **หยุด อย่า merge** แปลว่า `platformSpecificData` ไปทับค่าระดับบนแบบที่คาดไม่ถึง ต้องออกแบบใหม่

- [ ] **Step 4: ลบวิดีโอทดสอบ**

ลบด้วยมือใน YouTube Studio (ระบบไม่มีคำสั่งลบวิดีโอ) แล้วลบไฟล์ชั่วคราว:

```bash
rm -f "$TMPDIR/zernio-test.json"
```

- [ ] **Step 5: บันทึกผลลงสเปก**

เติมท้าย `docs/superpowers/specs/2026-07-31-youtube-first-comment-design.md` หัวข้อ "ผลการพิสูจน์ (live test)" ระบุ: วันที่ทดสอบ, post id ที่ Zernio คืน, ผลข้อ 1-4 แต่ละข้อ, และสรุปว่าปักหมุดจริงหรือไม่ แล้ว commit:

```bash
git add docs/superpowers/specs/2026-07-31-youtube-first-comment-design.md
git commit -m "docs(publish): ผล live test firstComment กับ Zernio"
```

---

### Task 5: เก็บกวาด + เปิด PR

**Files:** ทั้ง diff ของ branch

- [ ] **Step 1: รัน /simplify กับ diff**

เรียก skill `simplify` ให้ตรวจ diff ทั้งก้อน แล้วแก้ตามที่มันเสนอ (เฉพาะที่สมเหตุสมผล)

- [ ] **Step 2: รันเทสต์ + build อีกรอบหลังแก้**

Run: `go build ./... && go test ./...`
Expected: PASS

- [ ] **Step 3: เปิด PR**

```bash
git push -u origin HEAD
gh pr create --title "feat(publish): คอมเมนต์ปักหมุดติดต่อทีมงานใต้คลิป YouTube" --body "$(cat <<'EOF'
## ทำอะไร
แนบ `platformSpecificData.firstComment` ไปกับคำขอโพสต์ YouTube ที่ยิงผ่าน Zernio
Zernio โพสต์คอมเมนต์ใต้คลิปและปักหมุดให้เอง — ไม่ต้องมี OAuth ของ YouTube ในระบบ

## ทำไม
ข้อความติดต่อ (LINE @adsvance) อยู่ใน youtube_description อยู่แล้ว แต่บน Shorts
คำอธิบายถูกซ่อนต้องกดถึงเห็น ส่วนคอมเมนต์อยู่บนหน้าจอ — คลิปเกือบทั้งหมดของช่องเป็น 9:16

## สวิตช์ / rollback
ตั้ง settings `youtube_first_comment` เป็นค่าว่าง = ปิดทันที ไม่ต้อง deploy
(ค่าว่าง ⇒ ไม่แนบ platformSpecificData เลย คำขอเหมือนก่อนมีฟีเจอร์นี้)
migration 077 แค่เพิ่มแถว settings ไม่เปลี่ยน schema ⇒ revert commit ได้ตรงๆ

## พิสูจน์แล้ว
- unit test: JSON ที่ยิงมี firstComment เมื่อ setting มีค่า และไม่มีคีย์ platformSpecificData เลยเมื่อว่าง
- live test: โพสต์ unlisted 1 ตัวบนช่องจริง (ผลอยู่ในสเปก) แล้วลบทิ้ง

สเปก: `docs/superpowers/specs/2026-07-31-youtube-first-comment-design.md`

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

- [ ] **Step 4: หลัง merge + deploy — ตรวจคลิปจริงตัวแรก**

- ดู log รอบส่งบน Railway ว่าไม่มี error ใหม่จาก Zernio
- เปิดคลิปจริงตัวแรกที่ขึ้นหลัง deploy ดูว่ามีคอมเมนต์ติดต่อใต้คลิป
- ถ้าคลิปไม่ขึ้นเลย (log ขึ้น `Failed to post`) ⇒ ตั้ง `youtube_first_comment` เป็นค่าว่างทันทีผ่านหน้าเว็บ แล้วรอบส่งถัดไปจะกลับไปทำงานเหมือนเดิม
