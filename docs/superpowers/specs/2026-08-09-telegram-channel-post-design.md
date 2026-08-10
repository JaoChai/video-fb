# โพสต์คลิปเข้าช่อง Telegram อัตโนมัติหลังขึ้น YouTube

วันที่: 2026-08-09
สถานะ: อนุมัติดีไซน์แล้ว รอเขียนแผนลงมือ

## ปัญหา

ช่อง Telegram `@adsvancech` (https://t.me/adsvancech) ถูกโฆษณาไว้ทุกที่ — ใน `youtube_description`
ตั้งแต่ migration 011 และในคอมเมนต์แรกใต้คลิปตั้งแต่ migration 077 — แต่ตัวช่องเองไม่มีคอนเทนต์
ป้อนเข้าไปเลย คนกดเข้าไปแล้วเจอช่องเงียบ

ต้องการ: คลิปขึ้น YouTube เมื่อไหร่ ให้ช่อง Telegram ได้ชื่อคลิป + ลิงก์อัตโนมัติ

## ข้อเท็จจริงที่ตรวจแล้ว (ยิงของจริง 2026-08-09)

- **Zernio รองรับ Telegram** — `platform: "telegram"` · เชื่อมต่อแบบ bot ไม่ใช่ OAuth
  (`@LateScheduleBot` ต้องเป็นแอดมินในช่องและมีสิทธิ์โพสต์ — ชื่อ display "Social Media
  Connector", username จริงคือ `LateScheduleBot`; ระวังบอทปลอมที่ตั้ง display name
  เลียนแบบแต่ username จริงเป็นคนละตัว)
- **โพสต์ในช่องขึ้นเป็นชื่อ + โลโก้ของช่องเรา** ไม่ใช่ชื่อบอท (ที่ขึ้นชื่อบอทคือกรณี group)
- **แผนที่จ่ายอยู่รองรับแล้ว ไม่ต้องจ่ายเพิ่ม** — `GET /usage` คืน `planName: "Accelerate"`,
  `limits.uploads: -1` (ไม่จำกัด), `usage.profiles: 7 / 50`
- **ยังไม่มีบัญชี Telegram เชื่อมไว้** — `GET /accounts` คืน youtube ×3 + tiktok ×1 เท่านั้น
  ⇒ ต้องเชื่อมด้วยมือครั้งเดียวก่อนเปิดใช้ (ดู "งานที่ต้องทำด้วยมือ")
- **ลิงก์ YouTube จริงหาได้ทันทีหลังโพสต์** — `GET /posts/{id}` คืน
  `platforms[].platformPostUrl` = `https://www.youtube.com/watch?v=iJq2W3-vRbs`
  และ `platforms[].platformPostId` = video id
  (ทดสอบกับโพสต์จริงของคลิป `fc949adf` ที่ขึ้นเมื่อ 14:06 วันเดียวกัน — `publishedAt` กับเวลาที่
  `Post()` ตอบกลับห่างกันไม่ถึงวินาที)
  ⇒ ไม่ต้องรอ `FetchAnalytics` (รันวันละครั้ง 04:00) เหมือนที่ first-comment เคยติดปัญหา
- โค้ดปัจจุบัน `GetPost()` (`internal/publisher/zernio.go:305`) parse แค่ `post.status` — ทิ้ง
  `platforms[]` ทั้งก้อน
- `PlatformTarget.PlatformSpecificData` ผูก type เป็น `*YouTubeOptions` ตายตัว
  (`internal/publisher/zernio.go:58`)
- คลิปที่ผลิตตอนนี้เป็น 9:16 อย่างเดียว (`zernio_post_id` เป็น NULL ทุกแถวใน 3 คลิปล่าสุด)

## ข้อจำกัดฝั่ง Telegram (จากเอกสาร Zernio)

| หัวข้อ | ค่า |
|---|---|
| ข้อความล้วน | 4,096 ตัวอักษร |
| caption เมื่อแนบสื่อ | 1,024 ตัวอักษร |
| analytics | **ไม่มี** (ข้อจำกัดของ Telegram เอง) |
| `platformSpecificData` | `parseMode` (HTML/Markdown/MarkdownV2), `disableWebPagePreview`, `disableNotification`, `protectContent` |

## การตัดสินใจของเจ้าของ

1. ใช้ Zernio (ไม่ทำ Telegram Bot API เอง) — เพราะจ่ายค่า Zernio อยู่แล้วและแผนรองรับ
2. โพสต์เป็น **ข้อความล้วน: ชื่อคลิป + ลิงก์** ปล่อยให้ Telegram สร้างการ์ดพรีวิว
   (ภาพปก + ชื่อคลิป + ชื่อช่อง) เอง — ไม่อัปวิดีโอซ้ำเข้า Telegram
3. **เริ่มนับจากคลิปใหม่เท่านั้น** — คลิปที่ `published` ไปแล้วก่อนฟีเจอร์ขึ้น prod ไม่ถูกส่งย้อนหลัง
4. **ส่งทันทีในรอบเดียวกัน** ที่โพสต์ YouTube สำเร็จ ไม่หน่วง
5. เอา **รอบเก็บตกในกรอบ 24 ชั่วโมง** ด้วย (กันคลิปตกหล่นตอน Zernio ล่ม)

## สถาปัตยกรรม

ต่อท้ายเส้นทาง "โพสต์สำเร็จ" ของ `PublishReady()` โดยไม่แตะลำดับเดิม:

```
โพสต์ YouTube สำเร็จ
  → recordPublished()            (เดิม)
  → clearPublishFailure()        (เดิม)
  → postTelegram(clip)           ← ใหม่ ท้ายสุด ล้มแล้วไม่กระทบใคร
        1. GetPost(postID)  → platformPostUrl
        2. POST /posts platform=telegram (ข้อความล้วน + x-request-id)
        3. UPDATE clip_metadata SET zernio_telegram_post_id
```

และก่อนจบรอบ (หลัง loop คลิป ready):

```
  → sweepTelegramBacklog()       ← ใหม่ เก็บตกคลิปที่ published ใน 24 ชม.
                                   และ zernio_telegram_post_id IS NULL
```

### ไฟล์ที่แตะ

| ไฟล์ | สิ่งที่ทำ |
|---|---|
| `migrations/081_telegram_channel_post.sql` | `ALTER TABLE clip_metadata ADD COLUMN IF NOT EXISTS zernio_telegram_post_id TEXT` + `INSERT` setting `zernio_telegram_account_id` ค่าว่าง + **ปั๊มค่า `'skipped-backfill'` ให้คลิป `published` ทุกตัวที่มีอยู่ ณ เวลารัน migration** |
| `internal/publisher/zernio.go` | `GetPost()` อ่าน `platforms[].platformPostUrl`/`platformPostId` เพิ่ม (ตอนนี้อ่านแค่ `status`) |
| `internal/publisher/telegram.go` (ใหม่) | `postTelegram()`, `sweepTelegramBacklog()`, ตัวประกอบข้อความ — แยกไฟล์เพราะ `publisher.go` 748 บรรทัดแล้ว |
| `internal/publisher/publisher.go` | เรียกสองฟังก์ชันข้างบน + เปลี่ยนชื่อ `cleanTikTokHook` → `stripBrandTag` (ใช้ร่วมสองที่) |
| `internal/handler/settings.go` | เพิ่ม `zernio_telegram_account_id` ใน allowlist ของ `Update` |
| frontend หน้า Settings | ช่องกรอก account id (ตามแพตเทิร์นของ tiktok account id ที่มีอยู่) |

### ข้อความที่ส่ง

```
แยกผลรายตำแหน่งใน Ads Manager แล้วตัดตัวเผางบ

https://www.youtube.com/watch?v=iJq2W3-vRbs
```

- ต้นทาง: `clip_metadata.youtube_title` ผ่าน `stripBrandTag` (ตัด `| Ads Vance` ท้ายชื่อ)
  ไม่ต้องตัด ` #Shorts` เพราะชื่อที่ไหลเข้าเส้นทางนี้ไม่เคยมี — `PublishReady` เก็บชื่อ Shorts
  ไว้ในตัวแปรแยก (`shortsTitle`, `publisher.go:233`) และรอบเก็บตกอ่านค่าดิบจาก DB
- **ไม่ส่ง `platformSpecificData` เลย** — `parseMode` ค่า default ของ Zernio คือ `"HTML"`
  อยู่แล้ว จึงไม่ต้องแตะ `PlatformTarget.PlatformSpecificData` ที่ผูก type กับ `*YouTubeOptions`
  ⇒ เส้นทาง YouTube ไม่ถูกแก้แม้แต่บรรทัดเดียว · แต่ยังต้อง escape `&` `<` `>` ในชื่อคลิป
  เพราะปลายทาง parse เป็น HTML
- คลิปที่มีทั้ง 16:9 และ 9:16 → ส่ง **ครั้งเดียว** ใช้ลิงก์ของ 16:9 ก่อน

### ความล้มเหลว

- Telegram ล้มทุกกรณี = `log.Printf` แล้วไปต่อ · **ห้ามแตะ `clips.status` และ `fail_reason`**
  (`fail_reason` สงวนไว้ให้เรื่องที่บล็อกการเผยแพร่จริง คลิปขึ้น YouTube สำเร็จไปแล้ว)
- `GetPost` ไม่คืน `platformPostUrl` (เช่น Zernio ยังไม่อัปเดต) → ไม่ส่ง ปล่อยให้รอบเก็บตกจัดการ
- รอบเก็บตกจำกัดที่ **`updated_at > NOW() - INTERVAL '24 hours'`** — กรอบนี้คือสิ่งเดียวที่กัน
  ไม่ให้คลังคลิปเก่าทั้งหมดถูกยิงเข้าช่องรวดเดียว ห้ามถอดออกโดยไม่คิด
- กรอบ 24 ชม. เพียงอย่างเดียวยังไม่พอตอน deploy ครั้งแรก — คลิปที่เพิ่ง `published` ไปใน
  24 ชม.ก่อนหน้า (ปกติ 2-4 คลิป) จะเข้าข่ายเก็บตกทันที ซึ่งขัดกับข้อตกลง "เริ่มนับจากคลิปใหม่
  เท่านั้น" · migration 081 จึงปั๊ม `zernio_telegram_post_id = 'skipped-backfill'` ให้คลิป
  `published` ทุกตัวที่มีอยู่แล้ว ⇒ หลัง deploy ช่องจะเงียบจนกว่าจะมีคลิปใหม่ขึ้นจริง
- เก็บตกทีละ 1 คลิปต่อรอบ (`LIMIT 1`) เหมือน `PublishTikTok` — ช่องไม่โดนรัว

### กันส่งซ้ำ

1. `zernio_telegram_post_id IS NOT NULL` = ไม่ส่งอีก
2. `x-request-id` = `postRequestID(clipID, "telegram")` (ฟังก์ชันเดิมใน `publisher.go:94`) —
   ถ้า DB เขียนพลาดแล้วรอบถัดไปส่งซ้ำ Zernio จะจับ replay คืนโพสต์เดิม
3. 409 `DuplicatePostError` → ถือว่าเคยส่งแล้ว บันทึก `ExistingPostID` ลง DB ไม่ถือเป็น error

### สวิตช์เปิด/ปิด

setting `zernio_telegram_account_id` — **ค่าว่าง = ปิดสนิท** (ข้ามทั้งบล็อก ไม่ยิง HTTP เลย)
แพตเทิร์นเดียวกับ `zernio_tiktok_account_id` และ `youtube_first_comment` · เปิด ปิด หรือย้ายช่อง
ทำผ่านหน้า Settings ไม่ต้อง deploy

## เทสต์

เขียนก่อนโค้ด ใช้ `httptest` แบบเดียวกับ `zernio_test.go`:

1. ประกอบข้อความถูก — ตัด `| Ads Vance`, ลิงก์อยู่คนละบรรทัด, `&` `<` `>` ถูก escape
2. มีทั้ง 16:9 และ 9:16 → ใช้ลิงก์ 16:9 และยิงโพสต์ครั้งเดียว
3. `zernio_telegram_account_id` ว่าง → ไม่มี HTTP request ออกไปเลย
4. Telegram ล้ม → `clips.status` ยังเป็น `published` และ `fail_reason` ไม่ถูกแตะ
5. คลิปที่มี `zernio_telegram_post_id` แล้ว → ไม่ถูกหยิบโดยรอบเก็บตก
6. 409 duplicate → บันทึก `existingPostId` ลง DB และไม่คืน error
7. รอบเก็บตกไม่หยิบคลิปที่ `updated_at` เก่ากว่า 24 ชั่วโมง

**เทสต์อัตโนมัติพิสูจน์ไม่ได้ ต้องดูของจริงหลัง deploy:** ข้อความขึ้นช่อง `@adsvancech` ในนาม
ชื่อช่อง (ไม่ใช่ชื่อบอท) และการ์ดพรีวิวแสดงภาพปกคลิปถูกต้อง

## งานที่ต้องทำด้วยมือ (เจ้าของทำเอง ระบบทำแทนไม่ได้)

1. เพิ่ม `@LateScheduleBot` เป็นแอดมินของช่อง `@adsvancech` พร้อมสิทธิ์โพสต์ข้อความ
2. หน้า Zernio ออก access code อายุ 15 นาที (รูปแบบ `ZRN-XXXXXX`) → เปิดแชทส่วนตัวกับบอท →
   ส่งโค้ดพร้อม `@ชื่อช่อง` (ช่องมี public username) หรือส่งโค้ดแล้ว forward ข้อความจากช่องมา
   (ช่องส่วนตัวไม่มี username) → รอหน้า Zernio อัปเดตอัตโนมัติ
3. เอา `accountId` ที่ได้ (`GET /accounts` → รายการ platform `telegram`) ไปกรอกใน Settings

ถ้ายังไม่ทำข้อ 1-3 ฟีเจอร์จะอยู่ในสถานะปิดสนิท ระบบทำงานเหมือนเดิมทุกประการ

## ทางเลือกที่พิจารณาแล้วไม่เอา

**Telegram Bot API เอง** — คุมข้อความได้ 100% และไม่ผูกกับ Zernio แต่ต้องสร้างบอทเอง เก็บ
bot token เพิ่มอีกหนึ่งความลับ และเขียน HTTP client ใหม่ทั้งชุด ทั้งที่ Zernio ที่จ่ายเงินอยู่แล้ว
ทำได้ครบและมี idempotency (x-request-id / content-hash) ให้ฟรี · เก็บไว้เป็นทางสำรองถ้าภายหลัง
พบว่าโพสต์ผ่าน Zernio ขึ้นช่องไม่สวยหรือถูกจำกัด

**อัปวิดีโอ 9:16 เข้า Telegram ตรงๆ** — ดูในแอปได้เลยไม่ต้องออกไป YouTube แต่ Telegram จำกัด
วิดีโอ 50 MB (ไฟล์ใหญ่กว่าจะถูกบีบ) และที่สำคัญกว่าคือมันดึงวิวออกจาก YouTube ซึ่งเป็นช่องทางหลัก
ที่เราวัดผลอยู่

**รอ `FetchAnalytics` เพื่อเอา video id** — ช้าถึงวันละครั้ง (04:00) ทั้งที่ `GET /posts/{id}`
ให้ URL ได้ทันทีในวินาทีที่โพสต์เสร็จ
