# Design: Analytics Viral Feedback Loop (YouTube Shorts + TikTok)

วันที่: 2026-07-04
สถานะ: อนุมัติ design แล้ว (รอ review spec)

## บริบทและปัญหา (ตรวจสอบจาก prod จริงแล้ว 2026-07-04)

ตรวจ raw response จาก Zernio API บน prod (โพสต์จริง 3 แบบ: TikTok สำเร็จ, TikTok ล้มเหลว, YouTube Shorts + daily-views) พบว่า:

1. **TikTok**: Zernio ให้เฉพาะ `views, likes, comments, shares, engagementRate` — ฟิลด์ `impressions, reach, saves, clicks` เป็น 0 เสมอ และไม่มี watch time / retention (ข้อจำกัดของ Zernio ไม่ใช่บั๊กของเรา) ปัจจุบันเราเก็บครบยกเว้น `engagementRate`
2. **YouTube Shorts**: endpoint `analytics/youtube/daily-views` ส่งข้อมูลที่เรา parse แล้วทิ้ง:
   - `averageViewPercentage` — % retention ที่ YouTube คำนวณให้ตรง ๆ (แม่นกว่าที่เราคำนวณเองจาก `averageViewDuration / duration`)
   - `subscribersGained / subscribersLost` รายวัน
   - time series รายวันทั้งชุด (views, minutes watched, likes/comments/shares รายวัน) — ปัจจุบันบีบเหลือ aggregate ก้อนเดียว ทำให้มองไม่เห็น "รูปทรงแนวโน้ม" (พีคแล้วดับ vs ค่อย ๆ ไต่) ซึ่งเป็นสัญญาณการเข้า feed แนะนำ
3. **โพสต์ล้มเหลวเงียบ**: Zernio ส่ง `status` + `platformAnalytics[].errorMessage` มาแต่เราไม่อ่าน — เจอตัวอย่างจริง: TikTok โพสต์ไม่สำเร็จเพราะ "TikTok could not download the video" (ไฟล์วิดีโอเป็น temp URL ของ kie.ai ที่หมดอายุก่อน TikTok มาดึง) คลิปพวกนี้ยอดเป็น 0 ตลอดกาล และปนเปื้อนข้อมูลที่ agent ใช้เรียนรู้
4. **ฝั่ง learning loop**: Analyzer (รายสัปดาห์) อ่านเฉพาะ `platform='youtube'` — TikTok ถูกเมินทั้งหมด, prompt สั่งห้ามวิเคราะห์หัวข้อ ("STORYTELLING STYLES — NOT topics") ระบบจึงไม่เคยเรียนรู้ว่าหัวข้อไหนคนดูเยอะ, และ AI ไม่เคยเห็น hook (ประโยคเปิด) จริงของคลิปที่วิเคราะห์

## เป้าหมาย

ให้ Content agent ปรับปรุงวีดีโอโดยใช้ข้อมูลจริงจากทั้ง YouTube Shorts + TikTok โดย **ตัวชี้วัดหลักคือยอดวิว + การกระจาย** (views, shares, รูปทรงแนวโน้มรายวัน) ตามที่ user เลือก

การตัดสินใจสำคัญที่ user อนุมัติแล้ว:
- Optimize ที่ **ยอดวิว + การกระจาย** เป็นหลัก
- **เปิดการเรียนรู้ระดับหัวข้อ แบบคุมความหลากหลาย** (~ครึ่งหนึ่งเอนไปหมวดยอดดี อีกครึ่งกระจายเหมือนเดิม)
- **ตรวจจับโพสต์ล้มเหลว + แสดงใน UI** (ยังไม่ทำโพสต์ซ้ำอัตโนมัติ)
- แนวทาง A: ต่อยอด Analyzer เดิม ไม่สร้างระบบใหม่ขนาน

## Non-goals (ไม่ทำในงานนี้)

- โพสต์ซ้ำอัตโนมัติเมื่อล้มเหลว (ต้องแก้เรื่อง permanent storage ของไฟล์วิดีโอก่อน — เป็นงานแยก)
- Bandit เลือกหัวข้อแบบตัวเลขล้วน (ข้อมูลปัจจุบัน ~100-120 วิว/คลิป น้อยเกินกว่าจะมีนัยสถิติ)
- แก้ปัญหา kie.ai temp URL expiry ที่ต้นเหตุ
- ดึง metric ที่ Zernio ไม่มีให้ (TikTok watch time ฯลฯ)

---

## ส่วนที่ 1: ชั้นข้อมูล — เก็บจาก Zernio ให้ครบ

### 1a. ขยายตาราง `clip_analytics` (migration เพิ่มคอลัมน์อย่างเดียว)

| คอลัมน์ใหม่ | ที่มา | แพลตฟอร์ม |
|---|---|---|
| `engagement_rate FLOAT DEFAULT 0` | `PostMetrics.EngagementRate` | ทั้งคู่ |
| `avg_view_percentage FLOAT DEFAULT 0` | daily-views `averageViewPercentage` (เฉลี่ยถ่วงน้ำหนักด้วย views รายวัน) | YouTube |
| `subscribers_gained INT DEFAULT 0` | ผลรวม `subscribersGained` ในช่วงที่ดึง | YouTube |
| `subscribers_lost INT DEFAULT 0` | ผลรวม `subscribersLost` | YouTube |

`retention_rate` เดิมคงไว้ (backward compatible) แต่ frontend/Analyzer ใช้ `avg_view_percentage` แทนเมื่อมีค่า

### 1b. ตารางใหม่ `clip_analytics_daily` (YouTube เท่านั้น)

```sql
CREATE TABLE clip_analytics_daily (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,           -- 'youtube' (เผื่อโครงไว้ ไม่ hardcode)
    post_type TEXT NOT NULL,          -- 'regular' | 'shorts'
    date DATE NOT NULL,
    views INT NOT NULL DEFAULT 0,
    estimated_minutes_watched FLOAT NOT NULL DEFAULT 0,
    average_view_duration FLOAT NOT NULL DEFAULT 0,
    average_view_percentage FLOAT NOT NULL DEFAULT 0,
    subscribers_gained INT NOT NULL DEFAULT 0,
    subscribers_lost INT NOT NULL DEFAULT 0,
    likes INT NOT NULL DEFAULT 0,
    comments INT NOT NULL DEFAULT 0,
    shares INT NOT NULL DEFAULT 0,
    UNIQUE (clip_id, platform, post_type, date)
);
```

- Upsert (`ON CONFLICT ... DO UPDATE`) ทุกครั้งที่ fetch — ข้อมูลวันล่าสุดจาก YouTube มักขยับย้อนหลัง
- Parse struct `DailyViewEntry` ต้องเพิ่มฟิลด์ `AverageViewPercentage float64` ที่ตอนนี้ไม่มี
- **TikTok ไม่มี endpoint รายวัน** → แนวโน้ม TikTok คำนวณจากผลต่างระหว่าง snapshot ใน `clip_analytics` ที่ cron เก็บสะสมทุกวันตี 4 อยู่แล้ว (ไม่ต้องเก็บตารางเพิ่ม เขียนเป็น query diff ตอนอ่าน)

### 1c. ตารางใหม่ `clip_publish_status`

```sql
CREATE TABLE clip_publish_status (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,           -- 'youtube' | 'tiktok'
    post_type TEXT NOT NULL DEFAULT 'regular',
    zernio_post_id TEXT NOT NULL,
    status TEXT NOT NULL,             -- 'published' | 'failed' | 'scheduled' | 'unknown'
    error_message TEXT,               -- errorMessage ดิบจาก Zernio
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (clip_id, platform, post_type)
);
```

- เขียนตอน `FetchAnalytics` โดยอ่าน `resp.Status` + `resp.PlatformAnalytics[].Status/ErrorMessage` (ต้องเพิ่มฟิลด์ `Status`, `ErrorMessage` ใน struct `PlatformAnalyticsEntry` — ตอนนี้ไม่ parse)
- คลิปที่ `status='failed'` ถูก**กันออก**จาก: query ของ Analyzer, สถิติหัวข้อที่ป้อน Question agent, และ `PresetRetention` (กันข้อมูลเพี้ยน)

### การแก้โค้ด parse (`internal/publisher/zernio.go`, `publisher.go`)

- เพิ่มฟิลด์ใน struct ตามข้างบน — **fail-open ทั้งหมด**: ฟิลด์ไม่มา = 0/null, ไม่ทำให้ flow เดิมพัง
- `FetchAnalytics` เก็บ engagement_rate, avg_view_percentage, subscribers, daily rows, publish status เพิ่มจากเดิม — โครง loop เดิมไม่เปลี่ยน

---

## ส่วนที่ 2: Analyzer v2 + ป้อนข้อมูลเข้า Content agent

### 2a. ขยาย query ของ Analyzer (`internal/analyzer/analyzer.go`)

- เลิก lock `platform='youtube'` → อ่านทั้ง youtube + tiktok, ยังคง window 14 วัน
- ข้อมูลต่อคลิปที่ส่งให้ LLM: หมวด, ชื่อคลิป, คำถาม, **hook = ประโยคเปิดของ narration จริง** (ดึงจาก scenes/script ที่เก็บใน DB), views/likes/comments/shares/engagement_rate แยกแพลตฟอร์ม, retention (`avg_view_percentage`), subscribers_gained, **รูปทรงแนวโน้ม** (จาก `clip_analytics_daily` สำหรับ YouTube / diff ของ snapshot สำหรับ TikTok — สรุปเป็น label: `rising | peaked | flat`)
- JOIN `clip_publish_status` กันคลิป failed ออก

### 2b. เปรียบเทียบข้ามแพลตฟอร์ม

- จัดอันดับเป็น **percentile ภายในแพลตฟอร์มตัวเอง** ก่อนส่งให้ LLM (เช่น "top 20% ของ TikTok, bottom 30% ของ Shorts") — ไม่เทียบยอดดิบข้ามแพลตฟอร์ม
- คำนวณใน Go ก่อน build prompt ไม่ให้ LLM คำนวณเอง

### 2c. ปลดล็อกการเรียนรู้หัวข้อ

- แก้ prompt ของ Analyzer: วิเคราะห์**ทั้ง**สไตล์การเล่าเรื่อง**และ**หัวข้อ/หมวด โดยระบุในคำสั่งว่าต้องรักษาความหลากหลายของเนื้อหา (ไม่สั่งให้ผลิตซ้ำหมวดเดียว)
- Guardrail เดิมคงทั้งหมด: `ValidateInsights` (ไทย, ≤1000 ตัวอักษร), บันทึก `agent_prompt_history` ทุกการเปลี่ยน, insights เขียนลง `agent_configs.insights` เหมือนเดิม
- Learner (อ่าน critiques ภายใน) **ไม่แตะ** ในงานนี้

### 2d. ตารางสถิติหัวข้อ → Question agent

- ตอน generate คำถาม/หัวข้อคลิปใหม่: query สดจาก DB — ต่อหมวด: median views (percentile ภายในแพลตฟอร์ม), จำนวนคลิป, เฉพาะหมวดที่มี **≥3 คลิปวัดผลได้** (โพสต์สำเร็จ + อายุโพสต์ ≥3 วัน)
- Format เป็นตารางข้อความสั้น ๆ แนบท้าย prompt ของ Question agent พร้อมกฎ: **"เลือกหัวข้อจากหมวดผลงานดีประมาณครึ่งหนึ่งของโอกาส อีกครึ่งกระจายหมวดอื่นตามปกติ"**
- Semantic dedup กันคำถามซ้ำที่มีอยู่ทำงานเหมือนเดิม
- **Kill switch**: key `topic_stats_enabled` ในตาราง `settings` (default `true`, เพิ่มใน allowlist ให้แก้จาก UI ได้) — ปิดแล้ว Question agent กลับพฤติกรรมเดิมทันทีโดยไม่ต้อง deploy

### 2e. เกณฑ์ข้อมูลขั้นต่ำ

- Analyzer v2 ทำงานเฉพาะเมื่อมีคลิปวัดผลได้ ≥8 คลิปใน window (ตามเกณฑ์เดิมของ Learner) — ต่ำกว่านั้น skip แล้ว log
- Prompt บอก LLM ตรง ๆ ว่า n เท่าไหร่ และให้ระบุความมั่นใจต่ำเมื่อ n น้อย
- สถิติหัวข้อ: หมวดที่มี <3 คลิปไม่แสดงในตาราง (แสดงเป็น "ยังไม่มีข้อมูลพอ")

---

## ส่วนที่ 3: หน้า Analytics (frontend)

ต่อยอดจาก redesign 2026-07-03 (ภาษาไทย + tooltip) — ไม่รื้อโครงเดิม:

1. **แถบเตือนโพสต์ล้มเหลว** (บนสุด, โผล่เฉพาะเมื่อมี): รายการคลิป + แพลตฟอร์ม + สาเหตุแปลเป็นภาษาคน (map error ที่รู้จัก เช่น "could not download the video" → "ไฟล์วิดีโอหมดอายุก่อน TikTok ดึงไปโพสต์"; error อื่นแสดงข้อความดิบ)
2. **ตารางคลิป**: sparkline แนวโน้มรายวันต่อคลิป + badge "โพสต์ไม่สำเร็จ" ต่อแพลตฟอร์ม
3. **การ์ดแพลตฟอร์ม**: TikTok เพิ่ม engagement rate; YouTube เพิ่ม retention จริง (`avg_view_percentage`) + subscribers ที่ได้ในช่วง
4. **ส่วนใหม่ "หัวข้อไหนทำยอดดี"**: อันดับหมวดตามข้อมูลชุดเดียวกับที่ป้อน Question agent (user เห็นสิ่งเดียวกับที่ AI เห็น) + ระบุ n ต่อหมวด

Backend เพิ่ม endpoint/ขยาย `/api/v1/analytics/summary`: daily trend ต่อคลิป, รายการ publish failures, topic performance (ทุกตัว return `[]T{}` ไม่ใช่ nil — กัน JSON null ตาม convention ของ repo)

---

## Error handling

- Parse ฟิลด์ใหม่ทั้งหมด fail-open: ขาด = 0/null, ไม่ block การเก็บ metric เดิม
- daily-views endpoint ล่ม → เก็บ metric หลักตามเดิม, ข้าม daily (log warn)
- เขียน `clip_publish_status` ล้มเหลว → ไม่ fatal (log แล้วไปต่อ) แต่คลิปที่ไม่มีสถานะจะถือเป็น 'unknown' และ**ไม่ถูกกันออก**จากการเรียนรู้ (นับรวมแบบเดิม)
- Analyzer: ข้อมูลไม่พอ → skip ทั้งรอบ ไม่เขียน insights ว่าง ๆ ทับของเดิม

## การทดสอบ

1. **Fixtures จาก response จริง** ที่ดึงจาก prod วันนี้ (TikTok published, TikTok failed, YouTube + daily-views) → unit test การ parse ทุกฟิลด์ใหม่
2. Unit test: percentile ranking, trend label (rising/peaked/flat), topic stats query (กรอง n<3, กรอง failed), prompt builder ของ Question agent (มี/ไม่มีสถิติ ตาม kill switch)
3. Migration test: รันบนข้อมูลเก่า — คลิปเดิมต้องแสดงผลเหมือนเดิม
4. **ตรวจ prod หลัง deploy**: สั่ง fetch analytics ด้วยมือ 1 รอบ → ตรวจใน Neon ว่า `clip_analytics_daily`, `clip_publish_status`, คอลัมน์ใหม่ มีข้อมูลจริง แล้วเปิดหน้า Analytics ดูของจริง

## แผนย้อนกลับ

| ชิ้น | วิธีย้อน |
|---|---|
| สถิติหัวข้อเข้า Question agent | settings `topic_stats_enabled=false` (ไม่ต้อง deploy) |
| Insights จาก Analyzer v2 | SQL restore จาก `agent_prompt_history` |
| ทั้งฟีเจอร์ | revert merge commit เดียว (migration เป็น additive — ตารางใหม่ทิ้งไว้ได้ ไม่กระทบโค้ดเก่า) |

## ข้อจำกัดที่รู้แล้วยอมรับ

- TikTok ไม่มี watch time/retention จาก Zernio — การเรียนรู้ฝั่ง TikTok อิง views/shares/engagement rate + แนวโน้ม เท่านั้น
- ยอดวิวปัจจุบันต่ำ (~100-120/คลิป) — สัญญาณหยาบ ระบบออกแบบให้ "ถ่อมตัว" (เกณฑ์ n ขั้นต่ำ + บอก LLM ว่าข้อมูลน้อย) และจะแม่นขึ้นเองเมื่อช่องโต
- โพสต์ TikTok ที่ล้มเหลวไปแล้วในอดีต กู้คืนไม่ได้ในงานนี้ (ไฟล์หมดอายุ) — ทำได้แค่มองเห็นและกันออกจากข้อมูลเรียนรู้
