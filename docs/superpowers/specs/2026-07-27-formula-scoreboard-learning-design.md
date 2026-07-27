# กระดานคะแนนสูตร + การเรียนรู้จากยอดวิวจริง (Formula Scoreboard Learning)

วันที่: 2026-07-27
สถานะ: design approved (ส่วน 1-3 ผู้ใช้อนุมัติในการสนทนา, ส่วน 4-6 รอรีวิวพร้อมเอกสารนี้)

## 1. ปัญหา

ระบบมีลูปเรียนรู้ 2 ลูป ลูปหนึ่งเดินอยู่ อีกลูปตายสนิท:

**ลูป A — สถิติ → `agent_configs.insights`** (`internal/analyzer`) ทำงานจริง เขียน insights ครบ 4 agent ทุกสัปดาห์ (ล่าสุด 2026-07-26) แต่ป้อนข้อมูลให้ LLM เป็นตารางคลิปดิบสูงสุด 200 บรรทัดที่มีแค่ `title / category / hook` แล้วให้ LLM เดา correlation เอง ผลคือ insights วนซ้ำข้อสรุปเดิม ("hook เลขเงินชนะ") ทุกสัปดาห์

**ลูป B — คะแนน critic → `agent_configs.skills`** (`internal/learner`) **ไม่เคยยิงสักครั้ง**: `SELECT count(*) FROM skill_revisions` = 0 ตั้งแต่วันแรก สาเหตุเชิงโครงสร้าง 2 อย่าง

1. `LowScorePatterns(90)` (baseline) ครอบ `LowScorePatterns(30)` (window) ไว้ในตัวเอง — คะแนนล่าสุดถูกนับรวมใน baseline ด้วย ประตู regression จึงแทบไม่มีทางเปิด (ของจริง: hook 30 วัน = 7.90, baseline = 7.74, ต้องต่ำกว่า 7.24 ถึงจะยิง)
2. ประตู frequency นับปัญหาซ้ำโดย `GROUP BY field, reason` ที่ field มี index ซีนติดมา (`scene[5].image_prompt`) และ reason เป็นข้อความอิสระที่ LLM เขียนใหม่ทุกครั้ง — ปัญหาเดียวกันแตกเป็นสิบแถว ของจริงสูงสุด 3 ครั้ง ขณะที่เกณฑ์ต้องการ 33 ครั้ง (40% ของ 82)

**ช่องว่างที่ใหญ่กว่านั้น:** ทั้งสองลูปไม่มีลูปไหนเชื่อม "สูตรที่ใช้ผลิตคลิป" เข้ากับ "ผลลัพธ์จริง" ทั้งที่ข้อมูลมีอยู่ครบใน DB แล้ว

## 2. เป้าหมาย

ให้ยอดวิว/retention จริงป้อนกลับเข้าทั้งสองทาง: **ข้อความ** (`insights` + `skills`) และ **น้ำหนักการสุ่มสูตร** (`weight`)

ตัวชี้วัดที่ผู้ใช้ต้องการเห็นขยับใน 1-2 เดือน: median ยอดวิวต่อคลิปโต, retention ดีขึ้น, สัดส่วนคลิป flop ลด, และทุกการเปลี่ยนแปลงต้องตรวจสอบย้อนหลังและ rollback ได้

ระดับอิสระที่ตกลง: **ระบบแก้เองได้ ไม่ต้องขออนุมัติ** แต่ต้อง rollback ง่ายและมี audit ครบ

## 3. ข้อเท็จจริงจาก prod (วัดเมื่อ 2026-07-27)

จาก 83 คลิป `published` ใน 30 วัน:

| ช่องใน `clips` | เติมจริง | ปุ่มที่มี | ใช้ในสโคปนี้ |
|---|---|---|---|
| `content_format` (5 ค่า, enabled 4) | 83/83 | `content_formats.weight` | ✅ ปุ่มใหม่หลัก |
| `category` (enabled 3 หมวด) | 83/83 | `topic_categories.weight` | ✅ |
| `style_preset` (10 ค่า) | 79/83 | epsilon-greedy ใน Go (ไม่ทำงาน ดูหัวข้อ 7) | ✅ วัดผลอย่างเดียว ไม่หมุน weight |
| `title_archetype` | 42/83 | `title_archetypes.weight` | ❌ ตัดออก (fill ครึ่งเดียว) |
| `clip_role` | 43/83 | ไม่มี | ❌ ตัดออก (fill ครึ่งเดียว) |
| `audience_persona` | 43/83 | ไม่มี | ❌ free text ที่ LLM เขียนเอง 11 ค่าไม่ซ้ำกัน นับสถิติไม่ได้ |
| `composition_style`, `narrative_angle` | 0/83 | ไม่มี | ❌ คอลัมน์ตาย ไม่มีโค้ดเขียน |

**บั๊กที่พบระหว่างออกแบบ (อยู่ใน prod ตอนนี้):**

1. `AnalyticsRepo.PresetRetention` ใช้ `AVG(retention_rate)` → outlier ตัวเดียวลากทั้งกลุ่ม ของจริง `cinematic-photo` avg 0.130 (ชนะ ถูก exploit ทุกวัน) แต่ median 0.041 **ต่ำที่สุดในกลุ่ม** ขณะที่ `editorial-bold` median 0.058 กลับแพ้ → ระบบกำลังเลือกธีมที่แย่กว่าอยู่
2. `case-file` (ธีมแฟ้มคดีที่เพิ่ง go-live) มี n=6 แต่ `retention_rate` = 0 ทุกแถว → ตกเกณฑ์ `N >= minClips` ไม่มีวันถูก exploit
3. `clip_analytics` มีคอลัมน์ retention สองตัวคนละสเกล: `avg_view_percentage` (0.57-0.75, analyzer ใช้) กับ `retention_rate` (0.04-0.06, preset bandit ใช้) — spec นี้เลือก **`avg_view_percentage` เป็นแหล่งเดียว**

## 4. สถาปัตยกรรม

```
clip_analytics (รายวัน 04:00)
        │
        ▼
[A] formula_scores  ← snapshot รายสัปดาห์ (SQL ล้วน ไม่มี LLM)
        │
        ├──────────────► [B] weight tuner (deterministic) ──► content_formats.weight
        │                                                     topic_categories.weight
        │                                                     └─ audit: weight_revisions
        │
        ├──────────────► [C] preset selection (epsilon-greedy เดิม แก้ให้ใช้ median)
        │
        └──────────────► [D] analytics agent → insights
                             learner agent   → skills
                             └─ audit: agent_prompt_history / skill_revisions (ของเดิม)
```

หน่วยงานแยกกันชัด: [A] ผลิตตัวเลขอย่างเดียว, [B]/[C] แปลงตัวเลขเป็นการตัดสินใจแบบ deterministic, [D] แปลงตัวเลขเป็นข้อความผ่าน LLM แต่ละส่วนทดสอบแยกได้

## 5. ส่วน A — กระดานคะแนนสูตร (`formula_scores`)

เก็บเป็น **snapshot** ไม่ใช่ view เพื่อให้ตอบได้ว่า "weight สัปดาห์นี้มาจากคะแนนชุดไหน"

หนึ่งแถว = (`computed_at`, `dimension`, `value`, `platform`) เก็บ:

| ช่อง | นิยาม | เหตุผล |
|---|---|---|
| `n` | จำนวนคลิปที่มี analytics ในหน้าต่าง **60 วัน** | 30 วันให้ category แค่ ~5 คลิป/หมวด ไม่พอ |
| `median_pct` | median ของ views percentile **ภายในแพลตฟอร์มเดียวกัน** | ค่าเฉลี่ยโดนคลิปไวรัลลาก |
| `median_retention` | median `avg_view_percentage` นับเฉพาะค่า > 0 | ค่า 0 = ยังไม่มีรายงาน ไม่ใช่ retention แย่ |
| `flop_rate` | สัดส่วนคลิปที่ percentile < 0.25 | ตัวชี้วัดที่ผู้ใช้เลือก |
| `score_raw`, `score_final` | ตามสูตรข้างล่าง | เก็บทั้งคู่เพื่อ debug |

**สูตรคะแนน**

```
retention_norm = min-max normalize ของ median_retention ภายใน (dimension, platform) เดียวกัน
                 ถ้า max == min ให้ = 0.5
score_raw = 0.5×median_pct + 0.3×retention_norm + 0.2×(1 − flop_rate)
```

ถ้าไม่มี `median_retention` (TikTok ทั้งแพลตฟอร์ม / คลิปใหม่ที่ยังไม่มีรายงาน) → **ตัดพจน์นั้นออกแล้ว renormalize เหลือ 0.5/0.2 → 0.714/0.286** ห้ามแทนด้วย 0 (นี่คือบั๊กที่ทำให้ `case-file` ไม่มีสิทธิ์แข่ง)

**Shrinkage กัน n น้อย**

```
score_final = (n × score_raw + 5 × 0.5) / (n + 5)
```

สูตรที่มีข้อมูล 3 คลิปถูกดึงเข้าหากลาง (0.5); `qa` ที่มี n=75 แทบไม่ถูกดึง

**การรวมข้ามแพลตฟอร์ม** สำหรับ weight tuner: ค่าเฉลี่ยของ `score_final` ถ่วงด้วย `n` ของแต่ละแพลตฟอร์ม แพลตฟอร์มที่หยุด publish (TikTok paused ตั้งแต่ 24 ก.ค.) จะหลุดจากหน้าต่าง 60 วันไปเอง ไม่ต้องมีโค้ดพิเศษ

**ตัวเลขตรวจสอบแล้ว (60 วัน, TikTok, `content_format`):** tips median_pct 0.58 (n=17) · qa 0.54 (n=29) · case_story 0.44 (n=21) · news 0.31 (n=23)

## 6. ส่วน B — ตัวหมุนน้ำหนัก (deterministic ไม่ใช้ LLM)

รันเป็น scheduler action ใหม่ `tune_weights` ทุกจันทร์ 03:30 (หลัง `analyze_and_improve` 03:00) ควบคุมด้วย setting `weight_tuner_enabled` (เริ่มที่ `false`)

```
share_target  = score_final / Σ score_final
uniform       = 1 / จำนวนสูตรที่ enabled ในมิตินั้น
share_clamped = clamp(share_target, 0.5×uniform, 2×uniform)
share_new     = share_current + 0.25 × (share_clamped − share_current)
weight        = round(share_new × 100)     // สเกล Σ = 100
```

`share_current` อ่านจาก `weight` ปัจจุบันหารด้วยผลรวม weight ของแถวที่ enabled ในมิตินั้น

**เบรก 4 ชั้น**

1. ค่าใดมี `n < 8` → ตรึง weight เดิม ไม่คำนวณใหม่ ค่าที่เหลือคำนวณตามสูตรแล้วปรับสเกลให้ผลรวมเท่ากับ share ที่ยังว่างอยู่ (`1 − Σ share ของค่าที่ถูกตรึง`) โดย `uniform` ที่ใช้ clamp ยังคิดจากจำนวนสูตรที่ enabled **ทั้งมิติ** ไม่ใช่เฉพาะกลุ่มที่ขยับได้
2. เคลื่อน 25% ของระยะทางต่อสัปดาห์ → ถึงเป้าใน ~4 สัปดาห์ เป็น hysteresis โดยธรรมชาติ กัน flip-flop
3. เพดาน `2×uniform` / พื้น `0.5×uniform` → กับจำนวนสูตรวันนี้คือพื้น 12.5% (4 format) และ 16.7% (3 หมวด) **สูงกว่าพื้น 10% ที่ผู้ใช้ขอ** และไม่พังทางคณิตศาสตร์เมื่อจำนวนสูตรเปลี่ยน (15 หมวด × พื้น 10% = 150% เป็นไปไม่ได้)
4. **ห้ามแตะคอลัมน์ `enabled`** — ระบบ retire สูตรเองไม่ได้ ต้องผู้ใช้สั่งเท่านั้น

**ทำไมไม่ต้องแก้ logic การเลือก:** `FormatsRepo.PickNext` และ `TopicCategoriesRepo.PickNextExclude` เลือกด้วย least-used/weight อยู่แล้ว การเปลี่ยนสเกล weight เป็น Σ=100 ไม่กระทบเพราะอัตราส่วนเทียบกันเองภายในตาราง

**ขอบเขตที่ไม่แตะ:** `title_archetypes.weight` คงสเกลเดิม 1-2 ไม่เข้าสโคปนี้

## 7. ส่วน C — preset selection (เลื่อนออก ไม่อยู่ในรอบนี้)

**ข้อเท็จจริงที่พบตอนทำแผน:** `PickPresetWeighted` เป็น dead code บน prod ตอนนี้ `orchestrator.go:355-374` ตรวจ `CaseFormatEnabled()` **ก่อน** `StylePresetsEnabled()` และ prod ตั้ง `CASE_FORMAT_ENABLED=true` → ทุกคลิปได้ `CaseFilePreset` ตายตัว ยืนยันจากข้อมูลจริง: คลิปที่ไม่ใช่ tutorial ตั้งแต่ 2026-07-24 เป็น `case-file` ล้วน 100%

ดังนั้นการแก้ `PresetRetention`/`PickPresetWeighted` จะไม่มีผลกับ prod จนกว่าจะปิด `CASE_FORMAT_ENABLED` — **เลื่อนออกจากรอบนี้**

สิ่งที่ยังทำในรอบนี้: **เก็บมิติ `style_preset` ไว้ในกระดานคะแนน** เพราะมันตอบคำถามที่ค้างอยู่ว่า "case-file ดีกว่าธีมเก่า 4 ตัวจริงไหม" ซึ่งเป็นเงื่อนไขที่ตั้งไว้ก่อนจะลบธีมเก่าทิ้ง (ครบกำหนดตรวจ ~31 ก.ค.)

เมื่อใดที่จะกลับมาทำ: ถ้าตัดสินใจปิด `CASE_FORMAT_ENABLED` แล้วกลับไปหมุนหลายธีม ให้แก้ `PresetRetention` เป็น median + `avg_view_percentage` และป้อน `score_final` จาก `formula_scores` แทน `AvgRetention` ดิบ (บั๊ก outlier และบั๊ก retention=0 ในหัวข้อ 3 ยังรออยู่ตรงนั้น)

## 8. ส่วน D — ฝั่งข้อความ (insights + skills)

**analytics agent:** เปลี่ยนอาหารจาก "ตารางคลิปดิบ ≤200 บรรทัด" เป็น **กระดานคะแนน + คลิป top 10 / bottom 10 ต่อแพลตฟอร์ม** เพื่อให้ LLM อธิบาย *ทำไม* สูตรที่ชนะถึงชนะ แทนที่จะเดาว่าสูตรไหนชนะ (ตัวเลขนั้นคำนวณมาให้แล้ว) — `ValidateInsights` guardrail คงไว้ทั้งหมด เพราะการชี้นำหัวข้อทำผ่าน weight แล้ว ไม่ต้องให้ insights ทำซ้ำ

**learner (ปลุก skills loop):** แก้ 3 จุด

1. baseline ไม่ทับ window — `LowScorePatterns` รับช่วง (`fromDays`, `toDays`): window = 0-30 วัน, baseline = 30-90 วัน
2. normalize field ก่อนนับ — ตัด `[n]` ออกจาก field (`scene[5].image_prompt` → `scene.image_prompt`) และ **จัดกลุ่มด้วย field ที่ normalize แล้วเท่านั้น ไม่ group ด้วย reason** (เก็บ reason ตัวอย่าง 3 อันแรกไว้แสดง) — ของจริงจะได้ `scene.image_prompt` ~18 ครั้ง แทนที่จะกระจายเป็นสิบแถวๆ ละ 3
3. เพิ่มประตูที่สาม **outcome gate**: ถ้า `flop_rate` รวม 30 วันแย่กว่า baseline เกิน 0.10 → ยิง learner แม้คะแนน critic จะปกติ (คะแนน critic เป็น proxy ยอดวิวคือของจริง)

`LearnInput` เพิ่มฟิลด์กระดานคะแนนของ agent นั้น เพื่อให้ข้อเสนอแก้ skills อ้างผลจริงได้ guardrail `AcceptProposal` + allowlist (`scene`, `script`) + audit-ก่อน-apply คงไว้ทั้งหมด

## 9. Audit และ rollback

ตารางใหม่ `weight_revisions` (append-only): `id`, `dimension`, `value`, `old_weight`, `new_weight`, `score_final`, `n`, `computed_at` (โยงกลับไป `formula_scores`), `created_at`

- **rollback:** `POST /api/weights/rollback` (ไม่รับพารามิเตอร์ = ย้อน 1 สัปดาห์) เขียน `old_weight` ของ batch ล่าสุดกลับ แล้วบันทึกเป็น revision ใหม่ (ไม่ลบประวัติ)
- **kill switch:** setting `weight_tuner_enabled = false` หยุดการหมุนทันทีโดยไม่แตะค่าที่มีอยู่
- **อ่านสถานะ:** `GET /api/formula-scores` คืนกระดานล่าสุด + `GET /api/weight-revisions` คืนประวัติ 20 รายการล่าสุด แสดงในหน้า Analytics ที่มีอยู่ (endpoint ที่คืน list ต้อง init เป็น `[]T{}` ไม่ใช่ `var out []T` มิฉะนั้น JSON เป็น `null` แล้วหน้าเว็บพัง)
- **สั่งคำนวณเอง:** `POST /api/formula-scores/compute` คำนวณ snapshot ใหม่ทันทีโดยไม่แตะ weight ใช้ตอนตรวจก่อนเปิด flag

## 10. Error handling

ทุกจุดล้มแบบไม่ทำอันตราย ตามแบบที่ learner/analyzer ใช้อยู่:

- คำนวณกระดานคะแนนล้ม → log แล้วจบรอบ ไม่เขียน snapshot; tuner รอบนั้นไม่มีข้อมูลใหม่ → ไม่ขยับ weight
- tuner ล้มกลางทาง → เขียน weight ทีละมิติในทรานแซกชันเดียวต่อมิติ มิติที่สำเร็จอยู่ มิติที่ล้มคงค่าเดิม
- เขียน `weight_revisions` ล้ม → **ไม่ apply weight** (audit ต้องมาก่อนเสมอ เหมือน learner ปัจจุบัน)
- ไม่มีคลิปพอ (`n < 8` ทุกค่าในมิติ) → log ว่าข้ามเพราะอะไร ไม่ใช่เงียบ
- `formula_scores` ว่าง → tuner ข้ามรอบทั้งหมด ไม่เขียน weight ใดๆ (preset selection ไม่ได้อ่านกระดานในรอบนี้ ดูหัวข้อ 7)

## 11. Testing

ฟังก์ชัน pure ทดสอบด้วย unit test ตามแบบที่ repo ใช้อยู่ (`pickLeastUsed`, `strongSignal`, `TrendLabel` เป็นแบบอย่าง):

- `computeScore` — retention หาย → renormalize ถูกไหม, flop_rate สูงกดคะแนนไหม
- `shrink` — n=1 ต้องเข้าใกล้ 0.5, n=75 ต้องแทบไม่ขยับ
- `clampShare` — พื้น/เพดานถูกต้องที่ k=3, 4, 15 สูตร และผลรวมยังเป็น 1
- `smoothShare` — 25% ต่อรอบ, ลู่เข้าเป้าใน ~4 รอบ, ไม่ overshoot
- `tuneWeights` (pure, รับ scores คืน weights) — เคส `n < 8` ตรึงค่า, เคสทุกค่าคะแนนเท่ากันต้องได้ uniform
- `normalizeField` — `scene[5].image_prompt` → `scene.image_prompt`, `metadata.youtube_title` ไม่เปลี่ยน
- `strongSignal` — เคส baseline ไม่ทับ window, เคส outcome gate ยิงเมื่อ flop แย่ลง

ตรวจกับข้อมูลจริงก่อนเปิด flag: รัน tuner แบบ dry-run (คำนวณแต่ไม่เขียน) แล้วเทียบ weight ที่เสนอกับกระดานคะแนนด้วยตา

## 12. Migration และการปล่อย

Migration `066_formula_scoreboard.sql` (ล่าสุดในโปรเจกต์คือ `065_tutorial_field_rule.sql`):

1. `CREATE TABLE formula_scores`, `CREATE TABLE weight_revisions`
2. ปรับ `content_formats.weight` และ `topic_categories.weight` ของแถวที่ enabled ให้เป็นสเกล Σ=100 แบบเท่าๆ กัน (4 format = 25 ต่อตัว, 3 หมวด = 33/33/34)
3. `INSERT` setting `weight_tuner_enabled = 'false'`
4. `INSERT` schedule row `tune_weights` cron `30 3 * * 1` **enabled = false**

`RunMigrations` ไม่หุ้ม transaction ให้อัตโนมัติ ต้องเขียน `BEGIN`/`COMMIT` เอง

**ลำดับปล่อย**

1. Deploy พร้อม flag ปิดทั้งคู่ → สั่ง `POST /api/formula-scores/compute` ให้ได้ snapshot แรก
2. ดูกระดานคะแนน 1 สัปดาห์ เทียบกับสิ่งที่เห็นด้วยตา — ถ้าตัวเลขขัดสามัญสำนึกให้หยุดตรงนี้
3. เปิด schedule `tune_weights` + `weight_tuner_enabled = true` (เปิด schedule ต้อง PATCH ผ่าน API ไม่ใช่ UPDATE DB โดยตรง — scheduler reload จาก API เท่านั้น)
4. ตรวจ `weight_revisions` หลังรอบแรก ว่าขยับไม่เกิน 25% และไม่มีสูตรไหนหลุดพื้น
5. ส่วน D (insights/skills) ปล่อยตามหลังได้ ไม่ผูกกับ B/C

**Rollback ทั้งฟีเจอร์:** ปิด setting + `POST /api/weights/rollback` — โครงสร้างตารางทิ้งไว้ได้ ไม่มีผลข้างเคียง

## 13. หนี้ที่รู้ตัวและตั้งใจไม่แก้ในรอบนี้

- `audience_persona` เป็น free text → วัดผลไม่ได้ ถ้าจะเรียนรู้มิตินี้ต้องเปลี่ยนเป็น enum ก่อน
- `composition_style`, `narrative_angle` เป็นคอลัมน์ตาย (0/83) — ควรลบหรือทำให้เขียนจริง
- `title_archetype` / `clip_role` เติมแค่ ~52% เพราะเส้นทาง tutorial ไม่ตั้งค่า — ถ้าอุดรูนี้ได้ทั้งสองมิติจะเข้าสโคปได้ในรอบถัดไป
- ที่ ~21 คลิป/สัปดาห์ สัญญาณจะขยับช้า คาดว่าเห็นผลจริง 4-6 สัปดาห์ ไม่ใช่ 1-2 สัปดาห์
