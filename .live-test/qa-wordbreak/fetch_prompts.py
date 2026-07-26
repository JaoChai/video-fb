#!/usr/bin/env python3
"""ดึง prompt ตัวจริงของ visual_qa จาก DB มาเป็น prompts.json ให้ eval.py ใช้

  NEON_URL='postgresql://...' python3 fetch_prompts.py

ต้องดึงจาก DB ไม่ใช่ก๊อปจาก migration เพราะ prompt จริงคือ prompt เดิมที่ต่อกับส่วนใหม่
และ BuildSystemPrompt ยังเอา skills + insights มาต่อท้ายอีก (internal/models/agent.go:18-27)

ระวัง: ชี้ไปที่ branch ที่รัน migration 066 แล้วเท่านั้น — default branch ของ prod ยังไม่มี 066
ถ้าดึงผิด branch จะได้ prompt เก่าแล้ววัดไปคนละตัวโดยไม่มีอะไรเตือน

NEON_URL มีรหัสผ่านอยู่ข้างใน — ส่งผ่าน env เท่านั้น ห้ามเขียนลงไฟล์หรือ commit
(สคริปต์นี้ไม่พิมพ์ค่านั้นออกมาไม่ว่ากรณีใด)
"""
import json
import os

import psycopg2  # ไม่ใช่ stdlib แต่เครื่องมีอยู่แล้ว และดีกว่าก๊อป prompt 6KB ด้วยมือ

# SQL นี้ re-derive ตรรกะของ AgentConfig.BuildSystemPrompt() (internal/models/agent.go:18-27)
# — ถ้าโค้ด Go เปลี่ยนวิธีต่อ system_prompt+skills+insights ต้องมาแก้ query นี้ตาม ไม่งั้น
# prompts.json จะไม่ตรงกับ prompt จริงที่ยิงบน prod เงียบๆ (agent.go คือ source of truth)
#
# raw string เพื่อให้ \n ตกถึง Postgres เป็น escape ของ E'' ตามเดิม ไม่ใช่ให้ Python แปลงก่อน
SQL = r"""
SELECT json_build_object(
  'system', system_prompt
    || CASE WHEN skills   <> '' THEN E'\n\n## Skills & Guidelines\n'  || skills   ELSE '' END
    || CASE WHEN insights <> '' THEN E'\n\n## Performance Insights\n' || insights ELSE '' END,
  'template', prompt_template,
  'model', model,
  'temperature', temperature
)::text AS payload
FROM agent_configs WHERE agent_name = 'visual_qa';
"""

HERE = os.path.dirname(os.path.abspath(__file__))
url = os.environ.get("NEON_URL") or ""
if not url:
    raise SystemExit("ตั้ง NEON_URL ก่อนรัน (connection string ของ branch ที่มี migration 066)")

with psycopg2.connect(url) as conn, conn.cursor() as cur:
    cur.execute(SQL)
    row = cur.fetchone()
if not row:
    raise SystemExit("ไม่พบ agent_configs แถว visual_qa")

cfg = json.loads(row[0])
out = os.path.join(HERE, "prompts.json")
with open(out, "w", encoding="utf-8") as f:
    json.dump(cfg, f, ensure_ascii=False, indent=1)

# กันพลาดแบบเงียบๆ: ถ้า branch ไม่มี 066 จะไม่มีคำว่า wordbreak ใน prompt เลย
has_criterion = "wordbreak" in cfg["system"] and "wordbreak" in cfg["template"]
print(f"เขียน {out} แล้ว — model={cfg['model']} temp={cfg['temperature']} "
      f"system={len(cfg['system'])} ตัวอักษร template={len(cfg['template'])} ตัวอักษร")
print("เกณฑ์ wordbreak อยู่ใน prompt:", "ใช่" if has_criterion else "ไม่ — ดึงผิด branch แน่นอน")
