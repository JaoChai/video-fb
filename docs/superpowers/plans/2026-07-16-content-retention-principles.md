# Content Retention Principles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Encode 3 source-backed retention principles (3-second hook, open loop / write-for-completion, tight scenes + mid-clip re-hook) into the `script`, `question`, `scene`, and `critic` agent prompts via a single idempotent migration.

**Architecture:** One SQL migration (`056_content_retention_principles.sql`) with four `UPDATE agent_configs SET ... = REPLACE(...)` statements. Each appends content rules onto an exact existing anchor substring. No Go code changes. No JSON output-shape changes (avoids the 052 blank-narration regression). Verification is a read-only dry-run of each REPLACE on prod, then a post-deploy produce-1-clip check.

**Tech Stack:** PostgreSQL (Neon), Go migration runner (auto-applies on Railway deploy), Neon MCP `run_sql` for verification.

## Global Constraints

- Migration file: `migrations/056_content_retention_principles.sql` — next in sequence after 055.
- Every `UPDATE` uses `REPLACE(field, '<exact anchor>', '<anchor>' || '<addition>')` — idempotent (no-op once applied); dollar-quote both args with `$old$…$old$` / `$new$…$new$`.
- Do NOT add, rename, or remove any JSON output field in any prompt. Only append content rules to existing prompt text.
- Do NOT touch the comment CTA text (owned by migration 055) or Thai caption logic (`internal/producer/captions.go`, already done).
- Prod Neon projectId: `snowy-grass-75448787`. Run all verification SELECTs there (read-only).
- Rollback = revert migration 056 (or a paired down-migration that REPLACEs the new text back to the anchor).

---

### Task 1: `script` agent — 3s hook + open loop rules

**Files:**
- Create: `migrations/056_content_retention_principles.sql`

**Interfaces:**
- Produces: migration file containing the script-agent `REPLACE` block. Later tasks append their blocks to the same file.

**Anchor (exact current text of the `voice_script` bullet in `script.prompt_template`):**
```
- "voice_script": สคริปต์สำหรับ voiceover ภาษาไทย สั้นกว่า answer_script 150-300 คำ จบด้วย CTA ชวนคอมเมนต์ใต้คลิปเหมือน answer_script
```

- [ ] **Step 1: Create the migration file with the script block**

```sql
-- 056 Content retention principles
-- Deep research (2026-07-16, primary sources TikTok/YouTube) established that
-- hook (first ~3s) + watch-to-completion is the strongest ranking lever.
-- Encode 3 principles into script/question/scene/critic prompts. Prompt-only,
-- no JSON output-shape change (avoids the 052 blank-narration regression).
-- Idempotent: REPLACE is a no-op once the anchor is gone.

-- 1. script: 3-second spoken hook + open loop + clip arc
UPDATE agent_configs
SET prompt_template = REPLACE(
	prompt_template,
	$old$- "voice_script": สคริปต์สำหรับ voiceover ภาษาไทย สั้นกว่า answer_script 150-300 คำ จบด้วย CTA ชวนคอมเมนต์ใต้คลิปเหมือน answer_script$old$,
	$new$- "voice_script": สคริปต์สำหรับ voiceover ภาษาไทย สั้นกว่า answer_script 150-300 คำ จบด้วย CTA ชวนคอมเมนต์ใต้คลิปเหมือน answer_script

กติกา HOOK & ดูจนจบ (สำคัญสุดต่อการกระจายบน TikTok/Shorts):
- ประโยคแรกของ voice_script และ answer_script ต้องเป็น HOOK ที่ตรึงคนดูภายใน 3 วินาที (สั้น ไม่เกิน 15 คำ) เลือก 1 ใน 3 แบบ: (ก) ตั้งคำถามที่คลิปจะเฉลยทันที (ข) ตัวเลข/สถานะช็อก (ค) โยนผลลัพธ์หรือบทสรุปตอนจบขึ้นมาก่อน
- ห้ามเปิดด้วยการทวนคำถาม ทักทาย (เช่น "สวัสดีครับ") หรือเกริ่นยาว
- ใส่ OPEN LOOP ช่วงต้น (สัญญาว่าจะบอกวิธี/คำตอบท้ายคลิป) เพื่อดึงให้ดูจนจบ
- โครงคลิป: hook (ซีน 1 ราว 3 วิ) → ขยายเดิมพัน/ปัญหา → สเต็ปที่ทำตามได้จริง → payoff + CTA$new$)
WHERE agent_name = 'script';
```

- [ ] **Step 2: Dry-run the REPLACE on prod (read-only) to verify the anchor matches and the addition lands**

Run via Neon MCP `run_sql` (projectId `snowy-grass-75448787`):
```sql
WITH u AS (
  SELECT prompt_template AS orig,
    REPLACE(prompt_template,
      '- "voice_script": สคริปต์สำหรับ voiceover ภาษาไทย สั้นกว่า answer_script 150-300 คำ จบด้วย CTA ชวนคอมเมนต์ใต้คลิปเหมือน answer_script',
      'REPLACED_MARKER') AS repl
  FROM agent_configs WHERE agent_name='script'
)
SELECT orig LIKE '%150-300 คำ จบด้วย CTA ชวนคอมเมนต์ใต้คลิปเหมือน answer_script%' AS anchor_present,
       repl LIKE '%REPLACED_MARKER%' AS replace_fires
FROM u;
```
Expected: `anchor_present = true`, `replace_fires = true`. If either is false, STOP — the anchor text drifted; re-fetch the live bullet and fix the anchor.

- [ ] **Step 3: Commit**

```bash
git add migrations/056_content_retention_principles.sql
git commit -m "feat(script): 3s hook + open loop retention rules (migration 056)"
```

---

### Task 2: `question` agent — payoff / curiosity-gap rule

**Files:**
- Modify: `migrations/056_content_retention_principles.sql`

**Anchor (exact last line of `question.skills`):**
```
- หมุนขั้ว hook อย่าให้ทุกข้อเป็น "เงินหาย": เงินหาย / เดดไลน์บีบเวลา / ป้ายสถานะถูกปฏิเสธ / เคลมที่สวนสามัญสำนึก
```

- [ ] **Step 1: Append the question block to the migration**

```sql
-- 2. question: pick questions with a single-clip payoff + curiosity gap
UPDATE agent_configs
SET skills = REPLACE(
	skills,
	$old$- หมุนขั้ว hook อย่าให้ทุกข้อเป็น "เงินหาย": เงินหาย / เดดไลน์บีบเวลา / ป้ายสถานะถูกปฏิเสธ / เคลมที่สวนสามัญสำนึก$old$,
	$new$- หมุนขั้ว hook อย่าให้ทุกข้อเป็น "เงินหาย": เงินหาย / เดดไลน์บีบเวลา / ป้ายสถานะถูกปฏิเสธ / เคลมที่สวนสามัญสำนึก
- เลือกคำถามที่มี payoff ชัดเจน ตอบจบได้ใน 1 คลิป และเปิด curiosity gap (คนดูอยากรู้คำตอบ) — ไม่ใช่คำถามกว้างจนไม่มีบทสรุปในคลิปเดียว$new$)
WHERE agent_name = 'question';
```

- [ ] **Step 2: Dry-run verify on prod (read-only)**

```sql
WITH u AS (
  SELECT skills AS orig,
    REPLACE(skills,
      '- หมุนขั้ว hook อย่าให้ทุกข้อเป็น "เงินหาย": เงินหาย / เดดไลน์บีบเวลา / ป้ายสถานะถูกปฏิเสธ / เคลมที่สวนสามัญสำนึก',
      'REPLACED_MARKER') AS repl
  FROM agent_configs WHERE agent_name='question'
)
SELECT orig LIKE '%เคลมที่สวนสามัญสำนึก%' AS anchor_present,
       repl LIKE '%REPLACED_MARKER%' AS replace_fires FROM u;
```
Expected: both `true`. If false, STOP and re-fetch the live `question.skills`.

- [ ] **Step 3: Commit**

```bash
git add migrations/056_content_retention_principles.sql
git commit -m "feat(question): single-clip payoff + curiosity-gap rule (migration 056)"
```

---

### Task 3: `scene` agent — tight fast-cut scenes + mid-clip re-hook

**Files:**
- Modify: `migrations/056_content_retention_principles.sql`

**Anchor (exact line in `scene.skills`):**
```
- on_screen_text สั้น อ่านรู้เรื่องตอนปิดเสียง คุมความยาวตามลิมิตในกติกา
```

- [ ] **Step 1: Append the scene block to the migration**

```sql
-- 3. scene: tight fast-cut scenes + mid-clip re-hook (anti-skip)
UPDATE agent_configs
SET skills = REPLACE(
	skills,
	$old$- on_screen_text สั้น อ่านรู้เรื่องตอนปิดเสียง คุมความยาวตามลิมิตในกติกา$old$,
	$new$- on_screen_text สั้น อ่านรู้เรื่องตอนปิดเสียง คุมความยาวตามลิมิตในกติกา
- ทุกซีนสั้น ตัดไว หนึ่งซีนหนึ่งไอเดีย ห้ามซีนยาวหรือข้อมูลแน่นจนคนเบื่อ — ทุกซีนต้องทำให้คนดูอยากดูซีนถัดไป
- แทรก re-hook หรือจุดชวนสงสัยช่วงกลางคลิป (ราวซีนกลาง) เพื่อกันคนปัดหนีช่วงกลาง$new$)
WHERE agent_name = 'scene';
```

- [ ] **Step 2: Dry-run verify on prod (read-only)**

```sql
WITH u AS (
  SELECT skills AS orig,
    REPLACE(skills,
      '- on_screen_text สั้น อ่านรู้เรื่องตอนปิดเสียง คุมความยาวตามลิมิตในกติกา',
      'REPLACED_MARKER') AS repl
  FROM agent_configs WHERE agent_name='scene'
)
SELECT orig LIKE '%คุมความยาวตามลิมิตในกติกา%' AS anchor_present,
       repl LIKE '%REPLACED_MARKER%' AS replace_fires FROM u;
```
Expected: both `true`. If false, STOP and re-fetch the live `scene.skills`.

- [ ] **Step 3: Commit**

```bash
git add migrations/056_content_retention_principles.sql
git commit -m "feat(scene): tight scenes + mid-clip re-hook (migration 056)"
```

---

### Task 4: `critic` agent — spoken-hook + open-loop gate

**Files:**
- Modify: `migrations/056_content_retention_principles.sql`

**Anchor (exact visual-hook line already in `critic.skills`):**
```
- Hook: scene แรก on_screen_text ต้อง <=7 คำ และช็อก/ชวนสงสัยใน 1 วิ ถ้ายาว/อืดให้ตัดให้สั้นคม
```
Note: critic already gates the VISUAL hook (`on_screen_text`). This task adds the SPOKEN hook (scene-1 `VoiceText`) and open-loop checks. Critic edits scenes, so it references `VoiceText` of the first scene, not the `voice_script` field name.

- [ ] **Step 1: Append the critic block to the migration**

```sql
-- 4. critic: gate the spoken hook (scene-1 VoiceText) + open loop; fix, don't block
UPDATE agent_configs
SET skills = REPLACE(
	skills,
	$old$- Hook: scene แรก on_screen_text ต้อง <=7 คำ และช็อก/ชวนสงสัยใน 1 วิ ถ้ายาว/อืดให้ตัดให้สั้นคม$old$,
	$new$- Hook: scene แรก on_screen_text ต้อง <=7 คำ และช็อก/ชวนสงสัยใน 1 วิ ถ้ายาว/อืดให้ตัดให้สั้นคม
- Hook เสียงพูด: บทพูด (VoiceText) ของซีนแรกต้องเปิดด้วย hook ภายใน 3 วิ (คำถามที่จะเฉลย / ตัวเลข-สถานะช็อก / โยนผลลัพธ์ก่อน) ถ้าเปิดด้วยการทวนคำถาม ทักทาย หรือเกริ่นยาว ให้เขียนบทพูดซีนแรกใหม่ให้สั้นคม (แก้เฉพาะจุด ไม่ต้องรื้อทั้งบท)
- Open loop: ถ้าทั้งคลิปไม่มีจุดสัญญาว่าจะเฉลย/บอกวิธีท้ายคลิป ให้เติมประโยค open loop ในซีนต้นๆ$new$)
WHERE agent_name = 'critic';
```

- [ ] **Step 2: Dry-run verify on prod (read-only)**

```sql
WITH u AS (
  SELECT skills AS orig,
    REPLACE(skills,
      '- Hook: scene แรก on_screen_text ต้อง <=7 คำ และช็อก/ชวนสงสัยใน 1 วิ ถ้ายาว/อืดให้ตัดให้สั้นคม',
      'REPLACED_MARKER') AS repl
  FROM agent_configs WHERE agent_name='critic'
)
SELECT orig LIKE '%ช็อก/ชวนสงสัยใน 1 วิ ถ้ายาว/อืดให้ตัดให้สั้นคม%' AS anchor_present,
       repl LIKE '%REPLACED_MARKER%' AS replace_fires FROM u;
```
Expected: both `true`. If false, STOP and re-fetch the live `critic.skills`.

- [ ] **Step 3: Commit**

```bash
git add migrations/056_content_retention_principles.sql
git commit -m "feat(critic): spoken-hook + open-loop gate (migration 056)"
```

---

### Task 5: Whole-migration verification + all four anchors in one pass

**Files:**
- Modify: `migrations/056_content_retention_principles.sql` (final review only)

- [ ] **Step 1: Confirm all four anchors match on prod in a single read-only query**

```sql
SELECT
  (SELECT prompt_template LIKE '%150-300 คำ จบด้วย CTA ชวนคอมเมนต์ใต้คลิปเหมือน answer_script%' FROM agent_configs WHERE agent_name='script')   AS script_ok,
  (SELECT skills LIKE '%เคลมที่สวนสามัญสำนึก%' FROM agent_configs WHERE agent_name='question') AS question_ok,
  (SELECT skills LIKE '%คุมความยาวตามลิมิตในกติกา%' FROM agent_configs WHERE agent_name='scene') AS scene_ok,
  (SELECT skills LIKE '%ช็อก/ชวนสงสัยใน 1 วิ ถ้ายาว/อืดให้ตัดให้สั้นคม%' FROM agent_configs WHERE agent_name='critic') AS critic_ok;
```
Expected: all four columns `true`. If any is `false`, that agent's prompt drifted since the plan was written — re-fetch and fix that block's anchor before deploying.

- [ ] **Step 2: Verify the migration file is well-formed (dollar-quoting balanced, 4 UPDATEs)**

Run:
```bash
grep -c "WHERE agent_name" migrations/056_content_retention_principles.sql
```
Expected: `4`.

- [ ] **Step 3: Final commit if any cleanup was made (otherwise skip)**

```bash
git add migrations/056_content_retention_principles.sql
git commit -m "chore: finalize migration 056 content retention principles"
```

---

### Task 6: Deploy-time regression verification (run at/after deploy)

This task runs when the user chooses to deploy (merge to master → Railway auto-applies migration 056). It is the 052-regression guard.

- [ ] **Step 1: After deploy, confirm the migration applied on prod**

```sql
SELECT
  (SELECT prompt_template LIKE '%OPEN LOOP ช่วงต้น%' FROM agent_configs WHERE agent_name='script') AS script_applied,
  (SELECT skills LIKE '%payoff ชัดเจน ตอบจบได้ใน 1 คลิป%' FROM agent_configs WHERE agent_name='question') AS question_applied,
  (SELECT skills LIKE '%แทรก re-hook%' FROM agent_configs WHERE agent_name='scene') AS scene_applied,
  (SELECT skills LIKE '%Hook เสียงพูด%' FROM agent_configs WHERE agent_name='critic') AS critic_applied;
```
Expected: all four `true`.

- [ ] **Step 2: Produce one clip and check the retention rules + narration integrity**

Trigger a single-clip produce (e.g. `/produce` with count 1, or the prod orchestrator produce endpoint). Then query the newest clip:
```sql
SELECT c.id, c.status,
  (SELECT s->>'voice_text' FROM jsonb_array_elements(sc.scenes) WITH ORDINALITY AS x(s, ord) WHERE ord=1) AS scene1_voice
FROM clips c
JOIN scenes sc ON sc.clip_id = c.id
ORDER BY c.created_at DESC LIMIT 1;
```
(Adjust the scenes accessor to the actual scene storage — the key check is manual.) Confirm, by reading the clip:
- `scene1_voice` opens with a hook (question / shock number / result-first), NOT a greeting or question-restate.
- narration/voice text is **NOT empty** across scenes (direct 052-regression check).
- the clip reaches `ready`/`published` and renders normally.

Expected: hook present, open loop somewhere early, narration non-empty, normal render. If narration is empty on any scene → STOP, this is the 052 regression; revert migration 056.

- [ ] **Step 3: Record monitoring baseline**

Note current avg watch time / completion and 0-view TikTok count (failed-filtered per 055) so the 5–7 day trend can be compared. No commit.

---

## Self-Review

**1. Spec coverage:**
- Spec §4.1 script hook+open-loop → Task 1 ✓
- Spec §4.2 question payoff/curiosity → Task 2 ✓
- Spec §4.3 scene tight/re-hook → Task 3 ✓
- Spec §4.4 critic spoken-hook+open-loop gate → Task 4 ✓
- Spec §5 verification (dry-run anchors + produce-1-clip + narration check) → Tasks 1-4 Step 2, Task 5, Task 6 ✓
- Spec §6 rollout (single migration 056, revert to rollback) → Global Constraints + Task 6 ✓
- Spec §7 R1 blank-narration → Task 6 Step 2 explicit check ✓; R2 clickbait guard → existing 053 critic guard kept (Task 4 anchor retains it) ✓
- Out-of-scope (captions, CTA) → not touched, enforced by Global Constraints ✓

**2. Placeholder scan:** No TBD/TODO. All SQL and verification queries are concrete. Task 6 Step 2 flags that the scenes JSON accessor may need adjusting to actual storage — that is an explicit "read the clip manually" instruction, not a hidden placeholder.

**3. Type consistency:** No code types introduced (prompt/SQL only). Anchor strings in each task's Step 1 match the verification LIKE-substrings in Step 2 and Task 5. New-text markers in Task 6 Step 1 (`OPEN LOOP ช่วงต้น`, `payoff ชัดเจน ตอบจบได้ใน 1 คลิป`, `แทรก re-hook`, `Hook เสียงพูด`) each appear verbatim in the corresponding task's `$new$` block.
