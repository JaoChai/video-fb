# คอมเมนต์ปักหมุดอัตโนมัติใต้คลิป YouTube (first comment)

วันที่: 2026-07-31
สถานะ: อนุมัติดีไซน์แล้ว รอเขียนแผนลงมือ

## ปัญหา

ข้อความติดต่อ (`ติดต่อทีมงาน line id : @adsvance` + ลิงก์ Telegram) อยู่ใน `youtube_description`
อยู่แล้ว — บังคับไว้ใน prompt ของ script agent ตั้งแต่ migration 011 และย้ำใน 028

แต่บน YouTube Shorts คำอธิบายถูกซ่อน ผู้ชมต้องกดถึงจะเห็น ส่วนคอมเมนต์แสดงอยู่บนหน้าจอ
คลิปเกือบทั้งหมดของช่องเป็น 9:16 (hyperframes ผลิต 9:16 อย่างเดียว) ⇒ ช่องทางติดต่อที่มีอยู่
แทบไม่ถูกเห็น

ต้องการ: หลังโพสต์คลิปขึ้น YouTube ให้มีคอมเมนต์ของช่องเองใต้คลิป บอกช่องทางติดต่อ

## ข้อเท็จจริงที่ตรวจแล้ว

- ระบบไม่ได้คุยกับ YouTube โดยตรง — โพสต์ผ่าน Zernio API (`internal/publisher/publisher.go:148`
  สำหรับ 16:9 และ `:173` สำหรับ 9:16) ไม่มี OAuth ของ YouTube ในระบบเลย
- `youtube_video_id` ในตาราง `clip_metadata` ว่างทุกแถว — โค้ดอ่านอย่างเดียว ไม่เคยเขียน
- video ID จริงหาได้จาก `GetAnalytics(postID, "youtube")` → `platformAnalytics[].platformPostId`
  (ใช้อยู่แล้วใน `fetchYouTubeDetail`) แต่ได้ก็ต่อเมื่อ Zernio sync แล้ว = มีดีเลย์
- `settings` แก้จากหน้าเว็บได้เฉพาะคีย์ที่อยู่ใน allowlist ที่ `internal/handler/settings.go:55`

## ข้อเท็จจริงจากเอกสาร (อ่านจากเว็บ ยังไม่ได้ยิงจริง)

เอกสาร Zernio (https://docs.zernio.com/platforms/youtube) ระบุว่าคำขอโพสต์ YouTube รับ
`platforms[].platformSpecificData` ที่มีฟิลด์:

```json
{
  "title": "string",                 // Video title. Maximum 100 characters.
  "visibility": "public|private|unlisted",
  "madeForKids": false,              // true = ปิดคอมเมนต์ถาวร
  "categoryId": "22",
  "firstComment": "string"           // "Auto-posted and pinned comment. Maximum 10,000 characters"
}
```

⇒ Zernio โพสต์คอมเมนต์แรกให้ **และปักหมุดให้** ตามเอกสาร ระดับความมั่นใจ: "น่าจะถูก"
(อ่านเอกสาร ยังไม่ยิงจริง) — แผนจึงมีขั้นพิสูจน์ก่อนเปิดใช้

## ทางเลือกที่พิจารณาแล้วไม่เอา

**ยิง YouTube Data API เอง** — ต้องตั้ง Google Cloud project, OAuth consent, เก็บ refresh token,
หา video ID จาก Zernio (มีดีเลย์) แล้วต้องมีรอบ retry แยกเพราะ video ID ยังไม่มาตอนโพสต์เสร็จ
และที่สำคัญ **YouTube Data API ปักหมุดคอมเมนต์ไม่ได้** เก็บไว้เป็นทางสำรองเฉพาะกรณีที่พิสูจน์
แล้วพบว่า Zernio ไม่รองรับจริง

## ดีไซน์

### ข้อความคอมเมนต์

ข้อความเดียว ตายตัวทุกคลิป เก็บใน `settings` คีย์ `youtube_first_comment` ค่าเริ่มต้น:

```
สนใจบัญชีโฆษณา / สอบถามเพิ่มเติม
ติดต่อทีมงานได้ที่ LINE id : @adsvance

กลุ่มข่าวสาร Telegram : https://t.me/adsvancech
```

เหตุผลที่ไม่ให้ AI เขียน: คุมได้ 100% ไม่มีทางที่ agent จะแต่งช่องทางติดต่อผิด (เคยเกิดกับ
`source_label` ในฟีเจอร์ myth มาแล้ว) และคนอ่านคอมเมนต์ซ้ำๆ ไม่ได้เสียหายเท่าบทที่ซ้ำ

### จุดที่แก้

**1. `internal/publisher/zernio.go`**

เพิ่ม struct และฟิลด์ใหม่:

```go
// YouTubeOptions คือ platformSpecificData ของ Zernio สำหรับ YouTube
// FirstComment: Zernio โพสต์คอมเมนต์นี้ใต้คลิปและปักหมุดให้หลังอัปโหลดเสร็จ
type YouTubeOptions struct {
	Title        string `json:"title,omitempty"`
	Visibility   string `json:"visibility,omitempty"`
	FirstComment string `json:"firstComment,omitempty"`
}

type PlatformTarget struct {
	Platform             string          `json:"platform"`
	AccountID            string          `json:"accountId"`
	PlatformSpecificData *YouTubeOptions `json:"platformSpecificData,omitempty"`
}
```

ส่ง `Title` + `Visibility` ซ้ำเข้าไปใน `platformSpecificData` ด้วย (ทั้งที่ยังส่งระดับบนอยู่)
เพราะไม่รู้ว่า Zernio รวมค่าสองระดับยังไง ถ้ามันให้ `platformSpecificData` ชนะแล้วเราส่งแค่
`firstComment` ค่าที่หายไปคือชื่อคลิปและ visibility = คลิปขึ้นแบบไม่มีชื่อหรือเป็น private

**2. `internal/publisher/publisher.go` — `PublishReady`**

- อ่าน `settings.youtube_first_comment` ครั้งเดียวต่อรอบ (นอกลูปคลิป เหมือน `ytAccountID`)
- สร้าง `platforms` แยกต่อโพสต์ เพราะ 16:9 กับ 9:16 มี title ต่างกัน (9:16 ตัดที่ 60 ตัวอักษร
  แล้วต่อท้ายด้วย ` #Shorts`) — ปัจจุบันโค้ดใช้ slice `platforms` ตัวเดียวร่วมกันทั้งสองโพสต์
- **ถ้าค่าว่าง ⇒ ไม่ใส่ `platformSpecificData` เลย** คำขอที่ยิงต้องเหมือนของเดิมทุก byte

**3. migration `077_youtube_first_comment.sql`**

- `INSERT INTO settings (key, value) ... WHERE NOT EXISTS` ด้วยข้อความค่าเริ่มต้นข้างบน
  (ใช้ `WHERE NOT EXISTS` ไม่ใช่ `ON CONFLICT DO UPDATE` เพื่อไม่ทับค่าที่ user แก้เอง)

**4. `internal/handler/settings.go`**

เพิ่ม `"youtube_first_comment": true` ใน allowlist เพื่อให้แก้ข้อความจากหน้าเว็บได้โดยไม่ต้อง deploy

### สวิตช์และ rollback

ตั้ง `youtube_first_comment` เป็นค่าว่าง = ปิดฟีเจอร์ทันที ไม่ต้อง deploy ไม่ต้อง revert
(rollback แบบเต็ม = revert commit ได้ เพราะ migration แค่เพิ่มแถว settings ไม่ได้เปลี่ยน schema)

### ความปลอดภัยของรอบส่ง

**จะไม่ทำ auto-retry แบบตัด `firstComment` ทิ้งเมื่อยิงไม่ผ่าน** — ถ้า Zernio ตอบ error หลังจาก
อัปวิดีโอขึ้น YouTube สำเร็จแล้ว การยิงซ้ำ = คลิปซ้ำบนช่อง คงพฤติกรรมเดิมไว้: log แล้วปล่อยคลิป
เป็น `ready` ให้รอบหน้าลองใหม่ (`publisher.go:157`) ตัวป้องกันความเสี่ยงจริงคือขั้นทดสอบก่อนเปิด

## แผนทดสอบ

**ชั้นที่ 1 — unit test (`internal/publisher/zernio_test.go` แนวเดียวกับที่มีอยู่)**

ใช้ `httptest.Server` ดัก body ที่ยิงจริง แล้วยืนยัน:

| กรณี | ต้องได้ |
|------|---------|
| setting มีค่า | body มี `platformSpecificData.firstComment` = ข้อความนั้น และมี `title`/`visibility` ครบ |
| setting ว่าง | body **ไม่มีคีย์ `platformSpecificData` เลย** (ตรวจที่ JSON ดิบ ไม่ใช่ struct) |
| โพสต์ 9:16 | `firstComment` เหมือน 16:9 แต่ `title` เป็นตัวที่ต่อท้าย `#Shorts` |

**ชั้นที่ 2 — live test ก่อนเปิดใช้จริง (บังคับ ทำก่อน merge)**

สคริปต์ยิง Zernio ตรงๆ 1 โพสต์ (ไม่ผ่าน `/orchestrator/*` — ห้ามยิง endpoint ผลิตคลิป):
- ใช้ `video_9_16_url` ของคลิปเก่าที่ขึ้น R2 แล้ว
- `visibility: "unlisted"` — คนทั่วไปหาไม่เจอ
- ใส่ `firstComment` ข้อความทดสอบที่แยกออกจากของจริงได้

เช็คในช่อง (YouTube Studio) ว่า:
1. คอมเมนต์ขึ้นจริงใต้คลิป
2. ปักหมุดจริงหรือไม่ (ถ้าไม่ปัก = ยังใช้ได้ ช่องนี้ยัง 0 คอมเมนต์ คอมเมนต์เราเป็นอันเดียว)
3. ชื่อคลิปถูกต้อง ไม่หาย
4. visibility เป็น unlisted จริง (พิสูจน์ว่า `platformSpecificData` ไม่ไปทับค่าอื่นเพี้ยน)

จากนั้นลบวิดีโอทดสอบทิ้ง

**ถ้าข้อ 1 ไม่ผ่าน** = Zernio ไม่รองรับจริง ⇒ หยุด กลับมาคุยเรื่องทางสำรอง (YouTube Data API
ซึ่งต้อง OAuth และปักหมุดไม่ได้) ห้าม merge

**ชั้นที่ 3 — หลัง deploy**

ดูคลิปจริงตัวแรกที่ขึ้นหลังใส่ค่า setting ว่ามีคอมเมนต์ และ log รอบส่งไม่มี error ใหม่

## ขอบเขตที่ไม่ทำ

- **TikTok** — เอกสาร Zernio ระบุ first comment รองรับ YouTube/IG/FB/LinkedIn ไม่มี TikTok
  และ schedule ของ TikTok ปิดอยู่ตั้งแต่ 24 ก.ค. อยู่แล้ว
- **คลิปเก่า ~159 ตัว** — `firstComment` ทำงานเฉพาะตอนโพสต์ใหม่ ย้อนหลังต้องใช้ Comments API
  แยก (ยังไม่ยืนยันว่าสร้างคอมเมนต์ top-level ได้) + ปักหมุดอัตโนมัติไม่ได้ ⇒ ไม่คุ้ม
- **ตารางเก็บสถานะคอมเมนต์ / รอบ retry แยก** — ไม่จำเป็นเมื่อคอมเมนต์ไปพร้อมคำขอโพสต์
- **`youtube_description`** — ไม่แตะ ข้อความติดต่อยังอยู่ที่เดิม
- **`containsSyntheticMedia`** (ป้ายบอกว่าเป็นเนื้อหาที่ AI สร้าง) — เอกสาร Zernio มีฟิลด์นี้และ
  คลิปของเราเข้าข่าย แต่คนละเรื่องกับงานนี้ บันทึกไว้เป็นงานแยก

## ผลการพิสูจน์ (live test) — 2026-07-31

ยิงโพสต์ทดสอบ 1 ตัวด้วยโค้ดจริงของโปรเจกต์ (`ZernioClient.Post` + `platformSpecificData`)
ผ่าน `railway run` เพื่อไม่ต้องดึง API key ออกจากฐานข้อมูล · Zernio post id
`6a6c213e61722238006c7932` → YouTube video id `3k1ywoixAMI`

| # | ตรวจอะไร | ผล |
|---|----------|-----|
| 1 | คอมเมนต์ขึ้นใต้คลิป | ✅ ขึ้นจริง โพสต์โดยบัญชีช่องเอง (@adsvance) ข้อความตรงกับที่ส่งไปทุกตัวอักษร |
| 2 | ปักหมุด | ❌ **ไม่ได้ปักหมุด** — `#pinned-comment-badge` เป็น div ว่างและ `display:none` |
| 3 | ชื่อคลิป | ✅ "ทดสอบระบบ ห้ามเผยแพร่ 20260731" ไม่หายไม่เพี้ยน |
| 4 | visibility | ✅ ขึ้นป้าย "Unlisted" ตามที่ส่งไป |

**สรุป: ผ่านประตู** — เกณฑ์บังคับ (ข้อ 1, 3, 4) ผ่านครบ · ข้อ 2 ไม่ผ่านแต่ไม่บล็อกตามที่
ระบุไว้ในแผนทดสอบ เพราะช่องนี้ยังไม่มีคอมเมนต์จากใครเลย คอมเมนต์ของเราจึงเป็นอันเดียว
ใต้คลิปอยู่ดี · ข้อสำคัญที่พิสูจน์ได้เพิ่ม: `platformSpecificData` **ไม่ได้ทับ**ค่า title กับ
visibility ที่ส่งระดับบน (ข้อ 3 กับ 4 ยืนยันเรื่องนี้) ซึ่งเป็นความเสี่ยงหลักที่กลัวไว้

ต้องทำต่อด้วยมือ: ลบวิดีโอทดสอบ `3k1ywoixAMI` ออกจากช่อง (ระบบไม่มีคำสั่งลบวิดีโอ)

เอกสาร Zernio เขียนว่า firstComment เป็น "auto-posted and pinned comment" — ส่วน "pinned"
ไม่เป็นจริงในการวัดครั้งนี้ ถ้าอยากได้หมุดจริงต้องปักเองใน YouTube Studio (YouTube Data API
ก็ปักไม่ได้เช่นกัน)

## เกณฑ์ว่าสำเร็จ

1. `go test ./internal/publisher/...` ผ่าน รวม test ใหม่ทั้ง 3 กรณี
2. live test ชั้นที่ 2 ยืนยันด้วยตาว่าคอมเมนต์ขึ้นใต้คลิปทดสอบ และชื่อ/visibility ไม่เพี้ยน
3. คลิปจริงตัวแรกหลัง deploy มีคอมเมนต์ติดต่อใต้คลิป
4. ตั้ง setting เป็นค่าว่างแล้วรอบส่งถัดไปยิงคำขอเหมือนเดิม (พิสูจน์ว่าปิดได้จริง)
