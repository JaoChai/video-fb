-- 058 Script newsroom debate: 3 lens writers + judge (flag-gated, fail-open)
-- Spec: docs/superpowers/specs/2026-07-18-script-newsroom-debate-design.md
-- renderTemplate = string-replace (custom) NOT text/template — ใช้ได้แค่ {{.Field}} ห้าม {{if}}
-- Re-runnable ในไฟล์ (IF NOT EXISTS / ON CONFLICT / DELETE ก่อน INSERT / NOT LIKE guard)
-- หุ้ม BEGIN/COMMIT ให้ atomic: RunMigrations ทำ pool.Exec ไฟล์เดียวไม่หุ้ม transaction
BEGIN;

-- 1) Audit table: หนึ่งแถวต่อการ debate หนึ่งครั้ง
CREATE TABLE IF NOT EXISTS script_debates (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id    UUID NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    candidates JSONB NOT NULL DEFAULT '[]'::jsonb,
    verdict    JSONB,                      -- NULL เมื่อข้าม judge (เหลือ candidate เดียว) หรือ judge พัง
    source     TEXT NOT NULL,              -- judge | single_candidate | judge_failed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_script_debates_clip_id ON script_debates (clip_id);

-- 2) Flag + lenses (แก้จากหน้า Settings ได้โดยไม่ต้อง deploy)
INSERT INTO settings (key, value) VALUES
('script_debate_enabled', 'false'),
('script_debate_lenses', '[{"key":"hook_maximalist","name":"Hook Maximalist","instruction":"โฟกัสพลังหยุดนิ้วสูงสุด: 3 วินาทีแรกต้องแรงจนคนยิงแอดต้องหยุดดู เปิดด้วยตัวเลข/ความเสียหาย/คำถามที่แทงใจกลุ่มเป้าหมาย กล้าตัดเนื้อหาที่ไม่เสริม hook ทิ้ง ทุกประโยคต้องสร้างเหตุผลให้ดูต่ออีก 3 วินาที ห้ามเปิดด้วยการอธิบายพื้นหลังหรือทักทาย"},{"key":"skeptic_editor","name":"Skeptic Editor","instruction":"โฟกัสความแม่นและความน่าเชื่อถือ: ทุก claim ต้องเป็นสิ่งที่คนวงในยืนยันได้จริง ห้าม oversell ห้ามตัวเลขมั่ว ห้ามสัญญาผลลัพธ์ ถ้าไม่ชัวร์ให้พูดแบบมีเงื่อนไข เนื้อหาต้องลึกระดับที่มือใหม่เขียนไม่ได้ และห้ามแนะนำอะไรที่ขัดนโยบาย Meta ตรงๆ"},{"key":"target_viewer","name":"Target Viewer","instruction":"เขียนจากมุมคนดูเป้าหมายของรอบนี้ตรงๆ: ใช้ภาษาและศัพท์ที่คนกลุ่มนี้พิมพ์คุยกันจริง เล่าสถานการณ์ที่เขากำลังเจออยู่ให้รู้สึกว่านี่คือเรื่องของเขาเอง ตอบ pain ให้จบในคลิปเดียว ไม่อวดฉลาดเกินคนดู"}]')
ON CONFLICT (key) DO NOTHING;

-- 3) Judge agent (enabled=TRUE — การเปิดใช้จริงคุมด้วย script_debate_enabled)
DELETE FROM agent_configs WHERE agent_name = 'script_judge';
INSERT INTO agent_configs (agent_name, system_prompt, prompt_template, model, temperature, enabled, skills)
VALUES (
  'script_judge',
  'คุณคือหัวหน้ากองบรรณาธิการ (Editor-in-Chief) ของ Ads Vance ช่องความรู้เรื่องบัญชีโฆษณา Facebook สำหรับคนยิงแอดตัวจริง หน้าที่คุณคือตัดสินสคริปต์วิดีโอสั้นที่นักเขียน 3 คนเขียนแข่งกันคนละมุมมอง แล้วประกอบร่างฉบับที่ดีที่สุดฉบับเดียว คุณให้น้ำหนัก retention เหนือความครบถ้วน: hook 3 วินาทีแรกสำคัญที่สุด รองลงมาคือความแม่นของข้อมูล (เลขมั่ว/ขัดนโยบาย = ตกทันที) และความตรง pain ของผู้ชม',
  'คำถามของคลิป: {{.Question}}
ผู้ชมเป้าหมายรอบนี้: {{.AudiencePersona}}

สคริปต์จากนักเขียนแต่ละมุมมอง (JSON array, field lens บอกมุมมอง):
{{.CandidatesJSON}}

ขั้นตอน:
1) ให้คะแนนแต่ละฉบับ 1-10 สามด้าน: hook (พลังหยุดนิ้ว 3 วิแรก), accuracy (ความแม่น ไม่ oversell), audience_fit (ตรง pain และภาษาผู้ชม)
2) เลือกผู้ชนะ 1 ฉบับ — winner_lens ต้องเป็นค่า lens ของฉบับนั้นเป๊ะๆ
3) สร้างฉบับสุดท้าย (final): ใช้ฉบับผู้ชนะเป็นฐาน ยก hook หรือประโยคเด็ดจากฉบับอื่นมาเสริมได้เฉพาะจุดที่ทำให้ดีขึ้นจริง ห้ามเขียนเนื้อหาใหม่เอง ห้ามเพิ่ม claim ที่ไม่ปรากฏในฉบับใดเลย ความยาวใกล้เคียงฉบับผู้ชนะ

ตอบเป็น JSON เท่านั้น ห้ามมีข้อความอื่นนอก JSON:
{"scores":[{"lens":"...","hook":1,"accuracy":1,"audience_fit":1}],"winner_lens":"...","rationale":"เหตุผลสั้นๆ","final":{"answer_script":"...","voice_script":"...","youtube_title":"...","youtube_description":"...","youtube_tags":["..."]}}',
  'claude-sonnet-5',
  0.3,
  TRUE,
  '- final.voice_script ต้องพร้อมพากย์: ไม่มี URL, ไม่มี emoji, ไม่มี markdown
- ถ้าทุกฉบับ accuracy แย่ ให้เลือกฉบับที่เสี่ยงน้อยสุดเป็นผู้ชนะ ห้ามคืนค่าว่าง'
);

-- 4) ต่อท้าย placeholder ใน prompt ของ script (flag ปิด/ไม่มีเลนส์ → แทนด้วยสตริงว่าง ไม่กระทบ)
UPDATE agent_configs
SET prompt_template = prompt_template || E'\n\n{{.DebateLens}}'
WHERE agent_name = 'script'
  AND prompt_template NOT LIKE '%{{.DebateLens}}%';

COMMIT;
