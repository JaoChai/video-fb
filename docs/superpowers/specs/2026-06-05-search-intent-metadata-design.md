# Search-intent Metadata + Title Bug Fix — Design

**Date:** 2026-06-05
**Status:** Approved (brainstorming) — pending implementation plan
**Topic:** ปรับ YouTube metadata (title/tags/description) ของคลิปให้ "ดักการเสิร์ช" ของกลุ่มเป้าหมาย + แก้บั๊ก title แบรนด์ซ้ำ/ตัดกลาง

---

## 1. Context & Problem

ADS VANCE ช่อง YouTube Shorts (เผยแพร่ผ่าน Zernio) เริ่ม 2026-04-27, ~49 คลิป published.

**หลักฐานจาก DB (prod, 2026-06-05):**
- max views ตลอดกาล = 86, avg ~16/คลิป; likes รวมทั้งประวัติ = 13, comments/shares = 0.
- retention คำนวณได้และแปรผัน (0–59.7%, avg ~9% เมื่อ >0) — pipeline **ไม่ได้พัง** ตัวเลขเป็นของจริง (ช่องเล็ก/ใหม่).
- เผยแพร่ **แพลตฟอร์มเดียว** (YouTube Shorts); ไม่ cross-post.

**กลุ่มเป้าหมายจริง (ยืนยันจาก user):** คนยิงโฆษณา "สายเทา" — ใช้บัญชีโฆษณาจำนวนมาก โดนแบนบ่อย จึงต้องซื้อบัญชีใหม่เรื่อยๆ. คนกลุ่มนี้ **เสิร์ชหาทางแก้ตอนบัญชีพัง** (ไม่ได้ค้นพบซัพพลายเออร์ผ่านการปัดฟีด Shorts).

**ข้อสรุป:** หัวข้อคอนเทนต์ตรงเป้าอยู่แล้ว (BM โดนแบน, บัตรโดนแบน, เพิ่มงบแล้วแบน) แต่ **metadata ไม่ได้ออกแบบมาให้ดักการเสิร์ช** และ **title มีบั๊กจริง**.

### บั๊ก title (root cause ยืนยันแล้ว)
`internal/orchestrator/orchestrator.go` → `validateScript()`:
```go
const suffix = " | Ads Vance"
title := strings.TrimSuffix(script.YoutubeTitle, suffix) // strip เฉพาะรูปแบบนี้เป๊ะ
...truncate ที่ maxLen=70...
script.YoutubeTitle = string(titleRunes) + suffix
```
LLM (ตาม prompt/skills) ใส่แบรนด์รูปแบบ `{Ads Vance}` มา → `TrimSuffix` แมตช์ไม่โดน → ตัดความยาวลงไปกลาง `{Ads Vance}` กลายเป็น `{Ads V` → แล้ว append ` | Ads Vance` → ได้ `...{Ads V | Ads Vance`.

**ตัวอย่างจริงจาก DB:**
- `BM โดนแบนถาวรเพราะบัตรเครดิต แก้ยังไงให้ยอดไม่สะดุด {Ads V | Ads Vance` (ตัดกลาง + ซ้ำ)
- `เพิ่มงบแอดแล้วโดนแบน แก้ยังไง? {Ads Vance} | Ads Vance` (แบรนด์ซ้ำ 2 รอบ)

---

## 2. Goal & Non-Goals

**Goal:**
1. แก้บั๊ก title ให้เหลือแบรนด์ครั้งเดียว ไม่ตัดกลางคำ (ทนทุกรูปแบบที่ LLM อาจเติม).
2. ปรับ prompt ของ `script` agent ให้ title/tags/description **ดักคำที่กลุ่มเป้าหมายเสิร์ช**ตอนบัญชีพัง.

**Non-Goals (รอบนี้ไม่ทำ):**
- Cross-post หลายแพลตฟอร์ม (ทิศทาง B — แยกรอบ).
- Auto-post เข้ากลุ่ม FB/Telegram ของคนอื่น (เสี่ยง ToS — ไม่ทำเป็นระบบ).
- ปรับ self-improve/auto-tune loop (พบว่าไม่ใช่แก่นปัญหา — ไม่มีสัญญาณพอให้เรียนรู้).
- การันตีว่า views จะเพิ่ม (วัดได้เฉพาะระยะยาว).

---

## 3. Changes

### 3.1 [CODE] แก้ `validateScript` ให้ normalize title ทนทาน
ไฟล์: `internal/orchestrator/orchestrator.go`

พฤติกรรมใหม่:
1. **Strip แบรนด์ทุกรูปแบบ** ที่ LLM อาจเติมท้าย — ครอบคลุมอย่างน้อย: ` | Ads Vance`, `| Ads Vance` (ไม่มีเว้นวรรค), `{Ads Vance}`, `(Ads Vance)`, `Ads Vance` ห้อยท้าย (case-insensitive, มี/ไม่มีช่องว่างรอบ separator). ใช้การ normalize/regex ไม่ใช่ `TrimSuffix` เป๊ะตัวเดียว.
2. ตัดความยาว title ที่สะอาดแล้วให้พอดี `maxLen - len(suffix)`.
3. **Trim ช่องว่าง/เครื่องหมายค้างท้าย** (`|`, `{`, `(`, `-`, space) หลังตัด เพื่อไม่ให้เหลือเศษ.
4. Append suffix มาตรฐานตัวเดียว ` | Ads Vance`.

ผล: title ลงท้าย ` | Ads Vance` ครั้งเดียวเสมอ ไม่มีเศษ ไม่ตัดกลางคำ.

### 3.2 [DB migration] จูน prompt ของ `script` agent (search-intent)
ไฟล์: `migrations/028_search_intent_metadata.sql` → `UPDATE agent_configs ... WHERE agent_name='script'` (เฉพาะ `prompt_template`).

ปรับส่วน metadata ใน prompt:
- **youtube_title**: เอา **คำปัญหาที่คนเสิร์ช** ขึ้นต้น (เช่น "บัญชีโฆษณาโดนแบน", "BM โดนระงับ", "บัตรตัดเงินไม่ได้") — YouTube ให้น้ำหนักคำต้นๆ. กระชับ เป็นภาษาที่คนพิมพ์ค้นจริง. **สั่ง LLM ห้ามเติมแบรนด์/`{Ads Vance}`/` | Ads Vance` เอง** (โค้ดเติมให้ — แก้ root cause จากฝั่ง prompt ด้วย).
- **youtube_tags**: วลีที่คนพิมพ์เสิร์ชจริง (ภาษาพูดไทย + ศัพท์หลัก เช่น "เฟสแบน", "บัญชีโฆษณาโดนระงับ", "BM ban") ไม่ใช่ tag กว้างลอยๆ.
- **youtube_description**: 1–2 บรรทัดแรก = สรุปปัญหา+บอกว่ามีทางแก้ (ภาษาเสิร์ช, YouTube index ส่วนนี้เพื่อค้นหา) → **แล้วตามด้วย** 2 บรรทัดติดต่อเดิม (line id + telegram) ครบถ้วน. ปัจจุบันมีแต่บรรทัดติดต่อ = ทิ้ง search signal.

> หมายเหตุ migration numbering: runner (`internal/database/migrations.go`) track ด้วย **filename** (PK) และ apply ทุกไฟล์ที่ชื่อยังไม่อยู่ใน `schema_migrations` เรียงตามชื่อ. แถว orphan `021_…`–`027_…` (hyperframes ที่ rollback ไปแล้ว) ไม่มีไฟล์จริง → ถูก skip ไม่ error. ตั้งชื่อไฟล์ใหม่ `028_…` เพื่ออยู่เหนือ orphan rows และคง ordering. (verified จากโค้ด runner)

---

## 4. Data Flow

```
noon cron → Orchestrator.produceClip
  → ScriptAgent.Generate (ใช้ prompt_template จาก agent_configs[script])  ← [3.2 จูนตรงนี้]
      → GeneratedScript{ YoutubeTitle, YoutubeDescription, YoutubeTags[] }
  → validateScript(script)  ← [3.1 แก้ normalize ตรงนี้]
  → clipsRepo save → clip_metadata (youtube_title/description/tags)
  → publish ผ่าน Zernio
```

ไม่กระทบ render/วิดีโอ — แตะเฉพาะชั้น metadata.

---

## 5. Testing / Verification

| สิ่งที่แก้ | วิธีพิสูจน์ | ความมั่นใจ |
|-----------|-------------|------------|
| Title normalization (3.1) | **Unit test (TDD)** — ป้อน title หลายรูปแบบ (LLM ใส่ `{Ads Vance}` / ` \| Ads Vance` / ไม่ใส่ / ยาวเกิน 70 / มีเศษ separator) → assert: แบรนด์เดียว, ≤70, ไม่ตัดกลางคำ, ไม่มีเศษค้างท้าย | พิสูจน์ได้ 100% |
| Search-intent prompt (3.2) | gen metadata จากคำถามจริง 3–5 ตัวอย่าง (local/manual) → ตรวจตา: คำปัญหาขึ้นต้น title, tags เป็นวลีเสิร์ช, description มีสรุป+บรรทัดติดต่อครบ, ไม่มีแบรนด์ซ้ำ | ตรวจคุณภาพได้ทันที |
| reach เพิ่มจริง | YouTube search traffic — วัดได้ **เฉพาะระยะยาว (สัปดาห์+)** และ noisy | พิสูจน์ระยะสั้นไม่ได้ — จะไม่เคลม |

**จุดยืน:** งานนี้ "ลด friction" (title พัง + metadata ไม่ดักเสิร์ช) ซึ่งเป็นการแก้สิ่งที่ผิดอยู่แน่ๆ — แต่ไม่สัญญาว่า views พุ่ง.

---

## 6. Risks

1. **Prompt กระทบทุกคลิปอนาคต** (noon cron) → ตรวจ sample ที่ gen ก่อนปล่อยยาว.
2. **Migration numbering** → ใช้ 028+ (resolved ใน §3.2).
3. **กลุ่มเป้าหมายสายเทา** → ช่วยเฉพาะ metadata/SEO ที่ถูกกติกา; ไม่สร้างระบบละเมิด ToS (auto-spam / ปั่น engagement ปลอม).
4. **description ยาวขึ้น** → คุมให้บรรทัดติดต่อ (line/telegram) ยังอยู่และอ่านง่าย ไม่ถูกดันหลุด.

---

## 7. Out of scope / future
- ทิศทาง B: cross-post TikTok + auto-share Telegram ของแบรนด์เอง (ต้อง verify Zernio รองรับ).
- จุดเล็ก: `youtube_video_id` ไม่ถูกบันทึก (analytics มีข้อมูล); 14 คลิป views>0 แต่ watchtime=0 (YouTube Analytics fetch เร็วไป/scope) — re-fetch ภายหลัง.
