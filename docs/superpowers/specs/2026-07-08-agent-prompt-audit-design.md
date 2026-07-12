# Agent Prompt Audit & Upgrade — Design

วันที่: 2026-07-08 · สถานะ: อนุมัติแล้ว (รอ apply หลังพิสูจน์ PR #17)

## เป้าหมาย

Audit prompt/skills/template ของ agent ทั้ง 11 ตัวใน `agent_configs` (prod) ว่าออกแบบดีพอจะให้ผลงานคุณภาพไหม ทั้งสายเนื้อหาและสายคุมคุณภาพ แล้วร่างฉบับปรับปรุงโดยอ้างอิงแหล่งที่น่าเชื่อถือ ส่งมอบเป็นรายงาน + ร่าง migration 052 ให้ user อนุมัติก่อน apply

## ขอบเขต

- Audit ครบทั้ง 11 agents: question, research, script, scene, image, metadata, visual_qa, auto_review, critic, learner, analytics
- ตัดสินจาก **ผลงานจริง** (บทคลิป, ตำหนิ QA, ผล auto_review, ยอดวิว) ไม่ใช่แค่อ่าน prompt
- งานวันนี้เป็น read-only ทั้งหมด — ไม่แตะ prod จนกว่าจะอนุมัติ

## สถาปัตยกรรม: ทีม Opus 3 ระลอก (Workflow)

1. **Audit (4 agents ขนาน, opus)** — สายเนื้อหา (question/research/script), สายภาพ (scene/image/metadata), สายคุมคุณภาพ (visual_qa/auto_review/critic), สายเรียนรู้ (learner/analytics) แต่ละตัวได้ prompt ตัวจริง + สิทธิ์ query prod DB แบบ SELECT-only เพื่อดูผลงานจริง ให้คะแนน 4 มิติ (บทบาทชัด / กติกาครบ / หลักฐานคุณภาพผลงาน / ความเสี่ยง) + จุดอ่อนพร้อมหลักฐาน
2. **Research (2 agents ขนาน, opus)** — (ก) prompt engineering checklist จากเอกสารทางการ Anthropic (ข) หลักคอนเทนต์วิดีโอสั้น (hook/retention/CTA) พร้อมแหล่งอ้างอิงทุกข้อ
3. **Synthesis (1 agent, opus)** — รวมทุกผลเป็นร่าง prompt ใหม่ต่อ agent พร้อมเหตุผล ภายใต้กติกาเหล็ก:
   - ห้ามทำลาย output contract (placeholder / รูปแบบ JSON ที่โค้ด parse)
   - ห้ามลบความรู้ที่เพิ่งใส่ (เช่น กติกาคาราโอเกะ migration 051 ใน visual_qa)
   - ส่วนที่ output ต้องเป็นภาษาไทย คงภาษาไทย

## ผลส่งมอบ

1. รายงาน audit (คะแนน + before/after ต่อ agent) — user อ่านอนุมัติ
2. ร่าง `migrations/052_agent_prompt_upgrade.sql` — ยังไม่ merge/apply
3. Apply หลัง (ก) คลิป 3 ตัววันนี้พิสูจน์ PR #17 แล้ว (ข) user อนุมัติ → จากนั้นเฝ้าดูคลิป 2-3 ตัวแรกเทียบก่อน/หลัง

## Error handling / ความเสี่ยง

- แยกจังหวะจาก PR #17 เพื่อไม่ให้ผลวัดปนกัน
- apply ผ่าน migration (idempotent, revert ได้ commit เดียว) ไม่แก้ DB มือ
- ถ้า agent วิจัยเข้าเว็บไม่ได้/ข้อมูลไม่พอ ให้รายงานว่าขาดอะไร ห้ามแต่งแหล่งอ้างอิง

## เกณฑ์สำเร็จ

- รายงานครบ 11 agents ทุกตัวมีหลักฐานประกอบ
- ร่าง prompt ใหม่ผ่านกติกาเหล็ก 3 ข้อ
- หลัง apply: คุณภาพคลิป (ผ่าน QA + eyeball) ไม่แย่ลง และจุดอ่อนที่ audit ชี้ถูกแก้
