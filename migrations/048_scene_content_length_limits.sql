-- Append advisory per-layout length caps to the scene agent prompt so the LLM
-- keeps on-screen text within the layout boxes. The code-side TruncateRunes is the
-- enforcement net; this reduces how often it has to fire.
UPDATE agent_configs
SET prompt_template = prompt_template || E'\n\nกฎความยาวข้อความบนจอ (อย่าเกิน เพื่อไม่ให้ล้นกรอบ): cta/ปุ่ม ≤ 14 ตัวอักษร, pill ≤ 16, statLabel ≤ 28, sub ≤ 50, แต่ละแถว(rows[].t) ≤ 36, title ≤ 40. เขียนให้กระชับพอดีกรอบ.'
WHERE agent_name = 'scene'
  AND prompt_template NOT LIKE '%กฎความยาวข้อความบนจอ%';
