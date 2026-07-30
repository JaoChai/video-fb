# สเปก: คลิป "จับความเชื่อผิด" (format `myth`) — ช่อง 09:00

วันที่: 2026-07-30
สถานะ: รออนุมัติสเปก → ทำแผน implement

## 1. ปัญหาที่แก้

ช่องมีคลิป 4 ชนิด/วันแล้ว (เคสจริง 12:00, สอนพื้นฐาน 15:00, ถาม-ตอบ 18:00, สอนขั้นลึก 21:00)
ทั้งสี่ชนิด **สอนสิ่งที่ถูก** แต่ไม่มีชนิดใดที่ **รื้อสิ่งที่คนเชื่อผิด** ทั้งที่ความเชื่อผิดคือสาเหตุที่ลูกค้า
กลุ่มเป้าหมาย (คนซื้อบัญชี/BM) ตัดสินใจพลาดซ้ำๆ — เช่น "เปิด BM แล้วบัญชีแข็งกว่าบัญชีส่วนตัว"

## 2. ขอบเขต

**ทำ:** content_format ใหม่ `myth` + คลังหัวข้อ `myth_beliefs` + preset/theme ใหม่ `factcheck`
+ ช่องผลิตใหม่ 09:00 ไทย + ตะแกรงกันคลิปสร้างความเชื่อผิดใหม่

**ไม่ทำ:** ไม่แตะ 4 ช่องเดิม, ไม่แตะ publish path, ไม่ทำ UI แอดมินสำหรับคลัง (เติมคลังผ่าน migration/SQL
เหมือน `tutorial_features` วันนี้), ไม่ดึงข้อเท็จจริงจากเว็บอัตโนมัติ

## 3. ทางเลือกที่พิจารณาแล้วและเหตุผลที่ตัดออก

| ทางเลือก | ตัดออกเพราะ |
|---|---|
| หมุน `myth` เข้า pool ช่อง 18:00 | ได้ myth ~1 ใน 3 วันและไปแย่งพื้นที่ qa/tips ที่ยังต้องรัน |
| แทนช่อง 12:00 (case_story/news) | เสียคอนเทนต์เดิมไปหนึ่งแบบทั้งที่ยังทำงานอยู่ |
| ให้ agent คิด myth + หักล้างเอง จาก knowledge base | เรื่องนี้ **ผิดไม่ได้** — ถ้า agent เดาข้อเท็จจริง คลิปจะกลายเป็นตัวสร้างความเชื่อผิดใหม่และเสียเครดิตช่อง (บทเรียนเดียวกับ hook-signal: ห้ามต่อท่อข้อมูลที่ยังแยกสัญญาณจริงไม่ได้เข้า agent เขียนบท) |
| theme แบบ split-screen เทียบบน/ล่าง | อ่านเร็วสุด แต่พื้นที่ต่อซีนเหลือครึ่งเดียว ใส่ตัวเลข/แหล่งอ้างไม่ได้ |
| theme ตาชั่งเอียงตามน้ำหนักหลักฐาน | motion แพงและตัวหนังสือเล็กลงเพราะตาชั่งกินที่ |

## 4. สถาปัตยกรรม — เดินตามรอย `tutorial`/`basic` (คลิปที่หัวข้อมาจากคลัง)

| ชั้น | ของใหม่ |
|---|---|
| `content_formats` | แถว `myth` / display "จับความเชื่อผิด" · `enabled=false` · `weight=1` — ปิดไว้เพื่อไม่ให้สุ่มเข้า pool ของช่อง 12:00/18:00 (เหมือน `tutorial`/`basic`) |
| `schedules` | `Myth Produce & Publish` · `0 9 * * *` (เวลาไทย — cron ในตารางนี้เป็นเวลาไทย ยืนยันจาก `last_run_at` ของ 4 ช่องเดิม) · action `produce_myth` · **`enabled=false` ตอน migrate** · INSERT ต้องใช้ `WHERE NOT EXISTS` (ไม่มี unique index บน action) · cron ที่จะ **เปิดใช้จริง** ครั้งแรกอาจเป็น `0 9 * * 1,3,5` ตามเงื่อนไขคลังใน §9 |
| `internal/scheduler/scheduler.go` | `produceMyth` + case `"produce_myth"` ใน `handlerFor` |
| `internal/orchestrator` | `ProduceMyth` + `clipMode("myth") → ModeMyth` + `presetFor → MythPreset` + `resolveFormatInfo → FormatInfo{Mode: ModeMyth}` |
| `internal/orchestrator/tutorial.go` | `needsCatalogFeature()` และเส้นทาง `retryFull` ต้องรู้จัก `myth` — ไม่งั้นซ้ำบั๊กของ `basic` ที่ตะแกรงคลังปิดสนิทตอน retry แล้ว auto-publish |
| `agent_configs` | `script_myth`, `scene_myth`, `critic_myth` (ตามแบบ `*_chat` / `*_warroom`) |
| `internal/producer/myth_format.go` | `MythPreset` + `buildMythCoverPrompt` |
| template | บล็อก CSS `[data-format='myth']` + layout `belief`, `proof` |
| endpoint | `POST /api/v1/orchestrator/produce-myth` (ยิงมือ ตามแบบ produce-basic) |

`ProduceMyth` ไม่เรียก question agent — หัวข้อคือแถวคลัง เหมือน `produceCatalogClip` วันนี้
แต่ **คลังคนละตาราง** (`myth_beliefs` ไม่มี `steps`/`ui_vocab`) จึงเขียนฟังก์ชันผลิตแยก
ที่ยืมวินัยเดียวกัน: production gate, tracker, kie credit pre-check, cooldown/parking

## 5. คลัง `myth_beliefs`

โครงเลียน `tutorial_features` ในส่วนที่พิสูจน์แล้วว่าจำเป็น (`used_count`, `last_used_at`,
`parked_until`, `weight`, `enabled`, `audience`, `needs_verify`, `verify_reason`, `last_verified_at`)
บวกคอลัมน์เนื้อหา:

| คอลัมน์ | ชนิด | ใช้ที่ซีน | หมายเหตุ |
|---|---|---|---|
| `belief_key` | text NOT NULL UNIQUE | — | คีย์เสถียรสำหรับ log/dedup |
| `belief_th` | text NOT NULL | belief | คำเชื่อที่ยกมา |
| `why_believed_th` | text NOT NULL | belief | ทำไมคนถึงเชื่อ (ห้ามดูถูกคนดู) |
| `verdict` | text NOT NULL CHECK IN ('false','half_true','outdated') | verdict | → `Meter`/`Stamp` **Go-injected** |
| `fact_th` | text NOT NULL | proof | ข้อเท็จจริงที่หักล้าง |
| `source_label` | text NOT NULL | proof | ชื่อแหล่งที่แสดงบนจอ |
| `source_url` | text NOT NULL DEFAULT '' | — | ว่าง = ยังไม่มีแหล่งยืนยัน (มีผลกับ denylist ข้อ 7.3) |
| `nuance_th` | text NOT NULL | tip | ส่วนที่ **จริง** ของความเชื่อ |
| `cost_th` | text NOT NULL | hook | เชื่อผิดแล้วเสียอะไร |

การหยิบแถว: `enabled AND NOT needs_verify AND (parked_until IS NULL OR parked_until < now())`
เรียงตาม `weight` สุ่มถ่วงน้ำหนัก แล้ว `parked_until = now() + 14 วัน` หลังใช้ —
**และต้องมีพื้นกันคลังยุบใน UPDATE เดียวกัน** (ถ้าไม่มีแถวที่ยังไม่ park ให้หยิบแถวที่ park นานสุด)
เพราะประตูทางเดียวทำให้ produce คืน 0 คลิปเงียบๆ มาแล้วสองครั้ง

### 5.1 คลังเริ่มต้น (ต้องให้เจ้าของช่องตรวจก่อนทุกแถว)

ตรวจแล้วจากแหล่งจริง — พร้อมใช้ (`needs_verify=false`):

| # | ความเชื่อ | verdict | ข้อเท็จจริง / แหล่ง |
|---|---|---|---|
| 1 | เปิด BM แล้วบัญชีแข็งกว่าใช้บัญชีส่วนตัว | half_true | BM ให้ความเป็นเจ้าของ asset, สิทธิ์ทีม/พาร์ตเนอร์ (ไม่ต้องแชร์รหัส), หลาย payment — ไม่ได้แจก "ภูมิคุ้มกัน" · Jon Loomer, BM myths |
| 2 | ซอยหลาย ad set กันแอดตัวเองแย่งประมูล | false | Meta คัดเฉพาะแอดที่ค่าสูงสุดเข้าประมูลอยู่แล้ว (auction overlap) การซอยยิ่งทำให้ออกจาก learning ยาก · Jon Loomer, auction overlap |
| 3 | เพจไลก์เยอะ = แอดถูกลง | false | ไลก์ราคาถูกไม่ช่วย ที่มีผลคือผู้ติดตามที่มีส่วนร่วมจริง |
| 4 | โพสต์ขอไลก์/แชร์/แท็กเพื่อน ช่วยดัน reach | false | engagement bait ถูกลดการมองเห็น · เอกสาร Meta Business Help |
| 5 | Boost post ก็คือยิงแอดเหมือนกัน | false | boost ทำได้แค่ awareness/engagement พื้นๆ ไม่มี objective/โครงสร้าง/รายงานระดับ Ads Manager |
| 6 | ต้องมี 50 conversions/สัปดาห์ ไม่งั้นแอดใช้ไม่ได้ | half_true | ~50 optimized events/สัปดาห์ คือเกณฑ์ **ออกจาก learning phase** ไม่ใช่เกณฑ์ที่ทำให้แอดใช้ไม่ได้ |

ยังยืนยันไม่ได้ — ลงคลังด้วย `needs_verify=true` (คลิปหยิบไปเล่าไม่ได้จนมีคนยืนยัน):

| # | ความเชื่อ | ต้องยืนยันอะไร |
|---|---|---|
| 7 | บัญชีใหม่ต้อง warm up ค่อยๆ เพิ่มงบ ไม่งั้นถูกแบน | ไม่พบเอกสาร Meta; ที่มีจริงคือการเพิ่มงบแรงๆ ดีดกลับเข้า learning — ต้องหาแหล่ง Meta ก่อน |
| 8 | มี trust score / tier ที่กำหนดเพดานงบ | ตัวเลข tier ทั้งหมดที่วงการอ้างมาจาก reseller/agency ไม่ใช่ Meta |
| 9 | โดนแบนบัญชีเดียว = ทั้ง BM ตาย | ยังไม่ตรวจ |
| 10 | แก้แอดที่กำลังรัน = learning reset ทุกครั้ง | ต้องดูเอกสาร Meta ว่าการแก้ชนิดไหน reset |
| 11 | งบยิ่งเยอะ CPM ยิ่งถูก | ยังไม่ตรวจ |
| 12 | เพจใหม่ยิงแอดไม่ได้ ต้องโพสต์สะสมก่อน | ยังไม่ตรวจ |

**ผลที่ตามมา:** คลังพร้อมใช้จริง **6 แถว** ไม่ใช่ 12 → ดู §9 เงื่อนไขก่อนเปิด schedule

## 6. Theme "Fact-Check Lab" (`MythPreset`, key `factcheck`)

- **Palette:** `Brand` (navy #0047AF + amber #F0A030) เหมือนทุก format — ช่องต้องอ่านเป็นแบรนด์เดียว
- **ImageAnchor:** โต๊ะตรวจเอกสาร **กลางวัน** แสงจากหน้าต่าง + โคมส่อง + แว่นขยาย, photoreal,
  ห้ามมีตัวอักษร/โลโก้ — ตั้งใจให้ต่างจาก `case-file` (โต๊ะนักสืบกลางคืน) และ `warroom` (จอกลางคืน)
- **ภาพ AI 1 ใบ/คลิป** (ซีนปกเท่านั้น) เหมือน chat/warroom — ซีนที่เหลือเป็น CSS ล้วน
- **Font:** Sarabun (body) + Kanit (heading) · **Motion:** ตรายางกระแทก = entrance `punch` เร็ว (0.24s)

### 6.1 ฟิลด์ซีนที่เพิ่ม (ทั้งคู่ Go-injected เหมือน `CaseNo`/`StepTotal`)

```go
Meter  string `json:"meter,omitempty"`  // false|half_true|outdated — จาก myth_beliefs.verdict
Source string `json:"source,omitempty"` // จาก myth_beliefs.source_label
```

### 6.2 6 ซีน

| # | layout | หน้าตา | ข้อมูล |
|---|---|---|---|
| 1 | `hook` (เดิม) | ภาพปก + hook | `cost_th` |
| 2 | `belief` (**ใหม่**) | การ์ดกระดาษเอียงเล็กน้อยบนพื้นโต๊ะ + kicker "ที่เชื่อกัน" + `Sub` = ทำไมคนเชื่อ | `belief_th`, `why_believed_th` |
| 3 | `verdict` (เดิม + CSS ใหม่ scope ด้วย `[data-format='myth']`) | ตรายางกระแทกลงการ์ด + มิเตอร์ 3 ระดับ (เท็จ / จริงครึ่งเดียว / ล้าสมัย) | `Meter`, `Stamp` |
| 4 | `proof` (**ใหม่**) | การ์ดหลักฐาน + แถบแหล่งอ้างท้ายการ์ด | `fact_th`, `Source` |
| 5 | `tip` (เดิม) | "ส่วนที่จริงคือ…" | `nuance_th` |
| 6 | `cta` (เดิม) | ปิดแบรนด์ปกติ | — |

## 7. ตะแกรงกันคลิปสร้างความเชื่อผิดใหม่

1. **Go-injected wins** — `Meter`, `Source`, `Stamp` เขียนทับค่าที่ LLM ส่งมาเสมอ
   (LLM ตัดสินคำพิพากษาเองไม่ได้)
2. **ตะแกรงตัวเลข** (แบบเดียวกับ `ui_vocab`): ตัวเลขที่เป็น **ข้อเท็จจริง** ใน narration และ
   on-screen text ต้องปรากฏใน `fact_th`/`nuance_th`/`cost_th` ของแถวนั้น
   ขอบเขต: จำนวนเงิน, เปอร์เซ็นต์, จำนวนวัน/สัปดาห์, จำนวน event/คลิก/ครั้ง, ชื่อ tier/ระดับ
   **ไม่นับ:** เลขลำดับซีน/ขั้น ("ข้อ 2", "อย่างที่หนึ่ง") และเลขที่เป็นส่วนของชื่อเฉพาะ
   **strike 1 → regen · strike 2 → คลิปเข้า `needs_review` (ไม่ทิ้งคลิป ไม่ auto-publish)**
   — สองสไตรก์ก่อนตัดสินคือรูปแบบที่ลด false positive ลงครึ่งหนึ่งใน visual QA แล้ว
   และการไม่ทิ้งคลิปคือบทเรียนจากประตูทางเดียวของ `needs_verify`
3. **denylist คำอ้างลอย** — คำว่า trust score / tier / HiVA ใช้ได้เฉพาะแถวที่ `source_url <> ''`
   วันนี้ไม่มีแถวไหนมี จึงเท่ากับห้ามทั้งหมด
4. **`critic_myth`** ตรวจเพิ่ม 1 ข้อที่ critic ตัวอื่นไม่มี: สคริปต์ต้องมีทั้ง "ส่วนที่ผิด" และ
   "ส่วนที่จริง" — ห้ามเหวี่ยงเป็นขาว-ดำเมื่อ `verdict='half_true'`

## 8. การทดสอบ

```
1. render test (แบบ composition_chat_render_test.go)
   → verify: การ์ด belief + ตรายาง + มิเตอร์ + การ์ด proof ขึ้นครบ, ไม่มีแถบว่างท้ายเฟรม
2. Go-injected test: ป้อน scene ที่ LLM ใส่ Meter/Source/Stamp มั่ว
   → verify: ค่าที่เรนเดอร์มาจากแถวคลัง ไม่ใช่จาก LLM
3. fact-guard test: narration มีเลขที่ไม่อยู่ในแถวคลัง
   → verify: strike 1 regen, strike 2 → clip.status = needs_review (ไม่ใช่ ready/published)
4. denylist test: สคริปต์พูด "tier 2" ขณะ source_url ว่าง → verify: ถูกจับ
5. handler test: handlerFor("produce_myth") ไม่เป็น nil
6. catalog-floor test: ทุกแถว park อยู่ → verify: ยังได้ 1 แถว ไม่ใช่ 0 คลิป
7. retry test: retryFull ของคลิป myth → verify: โหลดแถวคลังกลับมา (ตะแกรงไม่ถูกข้าม)
8. Thai word-break: การ์ด belief ที่มีคำทับศัพท์ยาว → verify ด้วยตัวตรวจ deterministic
   ที่มี PoC อยู่ (.live-test/qa-wordbreak/repro.html) ไม่ใช่ visual_qa
```

## 9. Rollout / Rollback

**เงื่อนไขก่อนเปิด schedule** (คลังพร้อมใช้ 6 แถว, cooldown 14 วัน):
- ถ้าคลังยังไม่ถึง **14 แถวที่ `needs_verify=false`** → เปิดแบบ `0 9 * * 1,3,5` (จ/พ/ศ) ก่อน
- ครบ 14 แถว → PATCH เป็น `0 9 * * *`

**ลำดับ:**
```
1. migration 075: content_formats + myth_beliefs + agent rows + schedule (enabled=false)
   — RunMigrations ไม่หุ้ม transaction ต้องเขียน BEGIN/COMMIT เอง
2. deploy → ตรวจว่าแถวขึ้นจริงด้วย SQL
3. ยิงมือ POST /api/v1/orchestrator/produce-myth 1 คลิป
4. เจ้าของช่อง eyeball (ตรายาง/มิเตอร์/การ์ด/คำอ่านไม่ขาดกลางคำ)
5. PATCH /api/v1/schedules/{id} → enabled=true (ห้าม UPDATE ตรงใน DB — scheduler reload จาก API เท่านั้น)
```

**Rollback = PATCH ปิด schedule** ห้าม revert commit เฉยๆ เพราะจะดึงตะแกรง §7 ออกทั้งที่รอบผลิตยังเปิด
(รูปแบบเดียวกับ publish-queue fix)

## 10. ความเสี่ยงที่รู้ตัว

| ความเสี่ยง | รับมือ |
|---|---|
| คลังพร้อมใช้แค่ 6 แถว | เปิด 3 วัน/สัปดาห์ก่อน (§9) และเติมคลังเป็นงานต่อเนื่อง |
| ข้อเท็จจริงล้าสมัย (Meta เปลี่ยนกฎ) | `last_verified_at` + `verdict='outdated'` มีอยู่ในดีไซน์ ให้ทบทวนคลังทุกไตรมาส |
| การ์ดข้อความไทยยาว → หั่นกลางคำทับศัพท์ | บั๊กนี้ยังเปิดบน prod (ZWSP แก้ได้ครึ่งเดียว) → เทสต์ข้อ 8 เป็นตัวกัน ไม่ใช่ visual_qa |
| CSS `verdict` ของ myth ไปกวน case-file | ทุก selector ใหม่ต้อง scope `[data-format='myth']` + render test ของ case ต้องยังผ่าน |
| +1 คลิป/วัน = kie credit +25% | pre-check เครดิตมีอยู่แล้วในเส้นทางผลิต |

## 11. เอกสารอ้างอิงที่ใช้

- Jon Loomer — Facebook Business Manager: Common Myths <https://www.jonloomer.com/facebook-business-manager-myths/>
- Jon Loomer — Facebook Auction Overlap <https://www.jonloomer.com/facebook-auction-overlap/>
- Meta Business Help — How to Avoid Posting Engagement Bait <https://www.facebook.com/business/help/259911614709806>
- (อ้างเป็นตัวอย่างของแหล่งที่ **ไม่** ใช้เป็นข้อเท็จจริงในคลิป — reseller ไม่ใช่ Meta) <https://rentacc.agency/blog/facebook_trustscore>
