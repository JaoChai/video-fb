# หน้ารายละเอียดคลิป (Clip Detail Page)

วันที่: 2026-07-28

## ปัญหา

ตารางคลิปในหน้า Content กดเข้าไปดูรายละเอียดไม่ได้ ยกเว้น 2 กรณี
(`frontend/src/pages/Content.tsx:406-417`):

- `status = needs_review`
- `status = ready` และ `auto_review_held = true`

สองกรณีนี้เปิด `ReviewDialog` ซึ่งแสดงแค่ วิดีโอ / Visual QA / คะแนน Content Critic /
Auto-review — พอสำหรับตัดสินว่าจะปล่อยคลิปไหม แต่ไม่พอสำหรับดูว่าคลิปนั้นประกอบด้วยอะไร

คลิปที่ `published` / `failed` / `producing` / `ready` (ไม่ถูกกัก) กดไม่ได้เลย ทั้งที่เป็น
คลิปส่วนใหญ่ในระบบ ข้อมูลที่ระบบเก็บไว้แล้วแต่ไม่เคยแสดงที่ไหนเลย:

| ข้อมูล | ที่เก็บ | สถานะ API |
|---|---|---|
| สคริปต์เต็ม (`answer_script`, `voice_script`), `production_stage`, `case_number`, `tutorial_feature`, retry counts | `clips` | มี `GET /clips/{id}` — frontend ไม่เคยเรียก |
| ฉากรายฉาก (ภาพ 9:16, `voice_text`, `duration_seconds`, `layout`, `on_screen_text`, `beat`) | `scenes` | มี `GET /clips/{id}/scenes` — ไม่เคยเรียก |
| วิว/ไลก์/คอมเมนต์/แชร์/watch time/retention แยก platform | `clip_analytics` | มี `GET /clips/{id}/analytics` — ไม่เคยเรียก |
| YouTube title/description/tags + post id ของแต่ละแพลตฟอร์ม | `clip_metadata` | **ไม่มี endpoint** |
| ผลดีเบตสคริปต์ 3 มุมมอง + คำตัดสินของ judge | `script_debates` | **ไม่มี endpoint** |

## เป้าหมาย

กดคลิปไหนก็ได้ในตาราง แล้วเห็นทุกอย่างที่ระบบรู้เกี่ยวกับคลิปนั้นในหน้าเดียว
พร้อมทำแอ็กชันที่เคยทำได้ใน `ReviewDialog` ต่อจากหน้านั้นได้เลย

## ขอบเขต

**อยู่ในขอบเขต:** อ่านและแสดงผล + แอ็กชันที่มีอยู่แล้ว (อนุมัติ / ตีกลับ+ลบ / override hold / ลบ)

**ไม่อยู่ในขอบเขต:** แก้สคริปต์แล้วสั่ง re-render, แก้ prompt รายฉาก, สั่ง retry รายคลิป —
เป็นความสามารถใหม่ ไม่ใช่การแสดงผล

## สถาปัตยกรรม

### Backend: endpoint รวมตัวเดียว

```
GET /api/v1/clips/{id}/detail
```

คืน object เดียวที่ประกอบร่างจาก 7 แหล่ง แทนที่จะให้หน้าเว็บยิง 7 request เรียงกัน:

```json
{
  "clip":          { ...models.Clip },
  "metadata":      { ...models.ClipMetadata } | null,
  "scenes":        [ ...models.Scene ],
  "visual_qa":     { ...models.VisualQA } | null,
  "critique":      { ...models.ClipCritique } | null,
  "auto_review":   { ...models.AutoReview } | null,
  "analytics":     [ ...models.ClipAnalytics ],
  "script_debate": { ...models.ScriptDebate } | null
}
```

**เหตุผลที่รวมเป็นตัวเดียว:** หน้าเว็บต้องใช้ครบทุกก้อนเสมอ การยิง 7 request ทำให้เกิด
loading state 7 อัน และหน้ากระตุกตอนโหลด ส่วน endpoint เดิมทั้ง 4 ตัวยังคงอยู่ ไม่แตะ
(มีผู้ใช้อื่นอยู่ และการลบไม่ได้ช่วยอะไร)

**กติกาการพัง (fail-soft):**

- คลิปไม่มีในระบบ → `404` พร้อม `{"error": "clip not found"}`
- ส่วนย่อยชิ้นไหน query ไม่สำเร็จ หรือไม่มีข้อมูล → ชิ้นนั้นเป็น `null` (object) หรือ `[]`
  (array) ส่วนอื่นยังคืนตามปกติ เหตุผล: คลิปเก่าจำนวนมากไม่มี critique / debate /
  analytics อยู่แล้ว การทำให้ทั้ง response พังเพราะชิ้นเดียวไม่มีข้อมูล = หน้าเปิดไม่ได้เลย
- `scenes` และ `analytics` ต้อง initialize เป็น `[]T{}` เสมอ ห้าม `var xs []T` เพราะ
  nil slice serialize เป็น `null` แล้วหน้าเว็บ `.map` พัง (เคยเกิดกับ `/prompt-history`)

**สิ่งที่ต้องเขียนเพิ่มใน repository:**

- `ClipsRepo.GetMetadata(ctx, clipID) (*models.ClipMetadata, error)` — ปัจจุบันมีแค่
  `UpsertMetadata`
- `ScriptDebatesRepo.GetByClip(ctx, clipID) (*models.ScriptDebate, error)` — ปัจจุบัน
  `internal/repository/scriptdebates.go` มีแต่ `INSERT` และยังไม่มี struct
  `models.ScriptDebate` ต้องสร้างใหม่ (`id`, `clip_id`, `candidates` JSONB,
  `verdict` JSONB, `source`, `created_at`)

ทั้งสองตัวคืน `(nil, nil)` เมื่อไม่มีแถว ไม่ใช่ error

**ไฟล์ backend:**

| ไฟล์ | การเปลี่ยนแปลง |
|---|---|
| `internal/models/clip.go` | เพิ่ม `ClipDetail`, `ScriptDebate` |
| `internal/repository/clips.go` | เพิ่ม `GetMetadata` |
| `internal/repository/scriptdebates.go` | เพิ่ม `GetByClip` |
| `internal/handler/clip_detail.go` (ใหม่) | handler ที่เรียก repo ทั้ง 7 ตัวแล้วประกอบร่าง |
| `internal/router/router.go` | ผูก route ใหม่ 1 บรรทัด |

Handler ใหม่รับ repo ทั้ง 7 ตัวผ่าน constructor เหมือน handler ตัวอื่นในโปรเจกต์

### Frontend: หน้าเต็ม `/clips/:id`

**การเข้าถึง:** ทุกแถวในตาราง `Content.tsx` กดได้ → `navigate('/clips/' + id)`
ป้าย "คลิกเพื่อรีวิว →" และ "ถูกกักโดย Visual QA — คลิกดูเหตุผล/override →" ยังอยู่
เพราะยังเป็นสัญญาณว่าแถวไหนต้องจัดการ ปุ่มถังขยะในแถวยังทำงานเหมือนเดิม
(`stopPropagation` กันไม่ให้เด้งเข้าหน้ารายละเอียด)

**โครงหน้า:**

```
┌──────────────────────────────────────┐
│ ← กลับ   [Ready] [ถูกกัก QA]  🗑 ลบ │
│ <title>                              │
│ category · content_format · preset   │
├───────────┬──────────────────────────┤
│  [วิดีโอ]  │ ภาพรวม | สคริปต์ | ฉาก   │
│   9:16    │ QA & รีวิว | ตัวเลข | เผยแพร่│
│  (sticky) │ ──────────────────────── │
│           │  <เนื้อหาแท็บ>            │
│ [อนุมัติ]  │                          │
│ [ตีกลับ]   │                          │
└───────────┴──────────────────────────┘
```

บนจอมือถือคอลัมน์ซ้ายไปอยู่ด้านบน (โครงเดียวกับ `ReviewDialog` ที่ใช้
`sm:grid-cols-[auto_1fr]` อยู่แล้ว)

**แท็บทั้ง 6:**

1. **ภาพรวม** — `question`, `questioner_name`, `category`, `content_format`,
   `style_preset`, `production_stage`, `case_number`, `tutorial_feature`,
   `retry_count`, `review_retry_count`, `fail_reason`, `created_at`, `updated_at`,
   `publish_date` (ฟิลด์ที่ว่าง/`null` ไม่ต้องแสดงแถว)
2. **สคริปต์** — `answer_script` และ `voice_script` เต็ม แสดงเป็นบล็อกข้อความอ่านง่าย
   ถ้ามี `script_debate` แสดงต่อท้าย: candidate ทั้ง 3 มุมมอง (lens + สคริปต์),
   คะแนนจาก `verdict.scores`, `winner_lens`, `rationale`, และ `source`
   (`judge` / `single_candidate` / `judge_failed`)
3. **ฉาก (N)** — การ์ดต่อฉาก เรียงตาม `scene_number`: ภาพ `image_9_16_url`,
   `scene_type`, `beat`, `layout`, `on_screen_text`, `voice_text`,
   `duration_seconds` ภาพที่โหลดไม่ขึ้น (URL หมดอายุ) แสดง placeholder เหมือนที่
   `ReviewDialog` ทำกับวิดีโอ
4. **QA & รีวิว** — Visual QA รายฉาก (ยกโค้ดจาก `ReviewDialog` มาทั้งชุด) +
   คะแนน Content Critic + คำตัดสิน Auto-review (`decision`, `confidence`,
   `defect_type`, `reasons[]`)
5. **ตัวเลข** — `clip_analytics` แยกตาม `platform`: `views`, `likes`, `comments`,
   `shares`, `watch_time_seconds`, `retention_rate`, `fetched_at`
   ถ้าไม่มีแถวเลย แสดงข้อความว่ายังไม่มีข้อมูล (คลิปที่ยังไม่ published เป็นเรื่องปกติ)
6. **เผยแพร่** — `youtube_title`, `youtube_description`, `youtube_tags[]`,
   ลิงก์ `https://www.youtube.com/watch?v={youtube_video_id}` (เปิดแท็บใหม่),
   `tiktok_post_id`, `zernio_post_id`, `zernio_shorts_post_id`,
   `zernio_tiktok_post_id`, `publish_date`
   (หมายเหตุ: `zernio_shorts_post_id` และ `zernio_tiktok_post_id` เป็นคอลัมน์ที่
   migration 013/032 เพิ่ม แต่ struct `models.ClipMetadata` ปัจจุบันยังไม่มี 2 ฟิลด์นี้ —
   ต้องเพิ่มเข้า struct ด้วยจึงจะแสดงได้)

**แอ็กชัน** (แสดงตามสถานะ ตรรกะเดียวกับ `ReviewDialog` ทุกประการ):

| สถานะ | ปุ่ม | ผลลัพธ์ |
|---|---|---|
| `needs_review` | อนุมัติ | `PATCH /clips/{id}` → `status: ready` |
| `ready` + `auto_review_held` | Override | `POST /clips/{id}/unhold` |
| `needs_review` หรือถูกกัก | ตีกลับ (ลบ) | ยืนยันก่อน → `DELETE /clips/{id}` → กลับไปหน้าตาราง |
| ทุกสถานะ | ลบ | ยืนยันก่อน → `DELETE /clips/{id}` → กลับไปหน้าตาราง |

หลังทำแอ็กชันสำเร็จ: `invalidateQueries(['clips'])` และ `['clip-detail', id]`
แอ็กชันที่ลบคลิปให้ `navigate` กลับหน้าตาราง แอ็กชันที่ไม่ลบให้อยู่หน้าเดิม

**ไฟล์ frontend:**

| ไฟล์ | การเปลี่ยนแปลง |
|---|---|
| `frontend/src/lib/routes.ts` | เพิ่ม `CLIP_DETAIL: '/clips/:id'` |
| `frontend/src/App.tsx` | เพิ่ม `<Route>` |
| `frontend/src/api.ts` | เพิ่ม type `ClipDetail` + `getClipDetail(id)` |
| `frontend/src/pages/ClipDetail.tsx` (ใหม่) | หน้าหลัก: โหลดข้อมูล, header, วิดีโอ, ปุ่มแอ็กชัน, สลับแท็บ |
| `frontend/src/components/clip-detail/` (ใหม่) | 1 ไฟล์ต่อแท็บ — `OverviewTab.tsx`, `ScriptTab.tsx`, `ScenesTab.tsx`, `QaTab.tsx`, `StatsTab.tsx`, `PublishTab.tsx` |
| `frontend/src/pages/Content.tsx` | ทุกแถวกดได้ → navigate; ลบ state `reviewClip` และการ import `ReviewDialog` |
| `frontend/src/components/ReviewDialog.tsx` | **ลบทิ้ง** (ทุกอย่างย้ายไปหน้าใหม่) |

แยกไฟล์ต่อแท็บเพื่อไม่ให้ `ClipDetail.tsx` บวมเป็นพันบรรทัด แต่ละแท็บรับ prop เป็น
ก้อนข้อมูลที่ตัวเองต้องใช้เท่านั้น

`ui/tabs.tsx` มีอยู่แล้วในโปรเจกต์ — ใช้ตัวนั้น ไม่เขียนใหม่

## การพิสูจน์ว่าใช้ได้

**Backend (Go test, รูปแบบเดียวกับ `internal/handler/clips_update_guard_test.go`):**

- ขอ detail ของ clip id ที่ไม่มี → `404`
- คลิปที่ไม่มี critique / auto_review / debate / analytics → `200` และฟิลด์เหล่านั้น
  เป็น `null` / `[]` ไม่ใช่ error
- `scenes` และ `analytics` ที่ว่าง serialize เป็น `[]` ไม่ใช่ `null`

**Frontend:** โปรเจกต์ยังไม่มี test runner (ไม่มี vitest ใน `frontend/package.json`)
จึงพิสูจน์ด้วยการรันหน้าเว็บจริงแล้วดูตา:

- `npm run build` ผ่าน (TypeScript ไม่มี error)
- เปิดคลิปที่ `published` แล้วเห็นครบทั้ง 6 แท็บ รวมถึงลิงก์ YouTube ที่กดไปดูได้จริง
- เปิดคลิปที่ `needs_review` แล้วปุ่มอนุมัติ/ตีกลับทำงานเหมือนที่ `ReviewDialog` เคยทำ
- เปิดคลิปที่ `failed` แล้วเห็น `fail_reason` และไม่มีปุ่มอนุมัติ

## ทางเลือกที่พิจารณาแล้วไม่เลือก

- **ขยาย `ReviewDialog` เดิม** — งานน้อยที่สุด แต่ฉาก 6 การ์ด + สคริปต์ยาว + ผลดีเบต
  ต้อง scroll ในกล่อง และแชร์ลิงก์ให้คนอื่นดูคลิปใดคลิปหนึ่งไม่ได้
- **Drawer เลื่อนจากขวา** — เห็นตารางค้างไว้ กดคลิปถัดไปได้เร็ว แต่พื้นที่แคบกว่าหน้าเต็ม
  ซึ่งเป็นข้อจำกัดเดียวกับ dialog
- **ให้ frontend ยิง 7 endpoint เอง** — ไม่ต้องแตะ backend เลยก็เกือบทำได้ แต่ยังขาด
  `clip_metadata` กับ `script_debates` อยู่ดี (ต้องเขียน endpoint ใหม่ 2 ตัว) และได้
  loading state 7 อันเป็นของแถม
