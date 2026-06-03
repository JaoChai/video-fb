# Research Agent — แทนที่ระบบ KB Crawl ด้วยการค้นเว็บสดแบบควบคุม

**วันที่:** 2026-06-03
**สถานะ:** อนุมัติแล้ว (วิเคราะห์โดย agent team 3 ตัว: cost / architecture / quality)

## ปัญหา

ระบบ KB crawl (ดึงเว็บ → chunk → embed → เก็บ) ไม่เคยทำงานสำเร็จเลยตลอดอายุโปรเจค:
- Scheduler ไม่โหลด schedule ที่เพิ่มทีหลัง (แก้แล้ว) → crawler ไม่เคยรัน
- พอรันได้ → OOM 16GB จากการอ่านข้อมูลไม่จำกัดขนาด → container ตาย (แก้แล้ว)
- เว็บเป้าหมายหลายแห่งบล็อกการดึงข้อมูล (Reddit, Facebook Help)
- ผลผลิตจริงจนถึงวันนี้: **0 chunks จากเว็บ**
- ภาระดูแล: โค้ด ~1,131 บรรทัด + failure modes ที่เกิดซ้ำ

## การตัดสินใจ (จากการวิเคราะห์ agent team)

**ใช้สถาปัตยกรรมลูกผสม:**

| ส่วน | การตัดสินใจ | เหตุผล |
|------|-------------|--------|
| ความรู้ธุรกิจภาษาไทย 20 ก้อน | **เก็บ** (KB + rag.Search เหมือนเดิม) | Proprietary หาจากเว็บไม่ได้ ควบคุม brand positioning |
| ระบบ crawl + URL sources | **ลบทั้งหมด** | ไม่เคยทำงาน เป็นภาระ |
| ข่าว/ข้อมูลสด (news format) | **ResearchAgent ค้นเว็บตอนผลิต** | สดกว่า ง่ายกว่า เจอข่าวไทยที่ crawl ไม่เจอ |
| ระบบกันเนื้อหาซ้ำ (dedup) | **เก็บ** | คนละระบบ ทำงานดีอยู่ |

## ความเสี่ยงที่ต้องควบคุม (จาก quality analysis)

1. **Web search อาจคืนบล็อก agency คู่แข่ง** → research prompt ต้องมี whitelist/blacklist แหล่งข้อมูล
2. **ข้อมูลผิด/เก่าหลุดขึ้นคลิป** → prompt บังคับ "ถ้าหาข้อมูลเชื่อถือได้ไม่เจอ ให้ตอบค่าว่าง ห้ามแต่งเอง" + ถ้า research ว่าง คลิป news จะ fallback เป็นเนื้อหาจาก KB
3. **Brand positioning หาย** → CTA/brand มาจาก skills + KB เสมอ (ไม่เปลี่ยน)

## สถาปัตยกรรมใหม่

```
QuestionAgent (news format)  → ResearchAgent.Research("ข่าว FB Ads ล่าสุดที่กระทบคนยิงแอดไทย")
                                → OpenRouter model + ":online" (Exa web search)
                                → ได้ summary + key facts + sources
                                → สร้างหัวข้อข่าวจาก research

ScriptAgent (news format)    → ResearchAgent.Research(หัวข้อข่าวที่เลือก) เพื่อหาข้อเท็จจริงละเอียด
                                + rag.Search(KB) เพื่อ brand context
                                → เขียนสคริปต์

QuestionAgent/ScriptAgent (qa/tips/case_story) → rag.Search(KB) เหมือนเดิม ไม่เปลี่ยน
```

**ResearchAgent เป็น agent ลำดับที่ 5 ในระบบ** (ต่อจาก question, script, image, analytics)
- มี config ใน agent_configs (แก้ prompt/model ได้จากหน้า Agents เหมือน agent อื่น)
- ใช้ `:online` suffix ของ OpenRouter → ไม่ต้องแก้โครงสร้าง LLM client

## สิ่งที่ลบ

| ไฟล์/ข้อมูล | การจัดการ |
|------------|-----------|
| `internal/crawler/crawler.go` | ลบไฟล์ทั้งหมด |
| `rag.SearchRecent()` | ลบ function |
| `-crawl` flag ใน main.go | ลบ |
| crawler wiring ใน scheduler | ลบ (field + param + case "crawl_knowledge") |
| URL sources ใน DB (11 แถว) | ลบ (chunks cascade อัตโนมัติ) |
| Schedule "Daily Knowledge Crawl" | ลบ |

## สิ่งที่เพิ่ม

| ไฟล์ | เนื้อหา |
|------|---------|
| `internal/agent/research.go` | ResearchAgent: Research(topic) → ค้นเว็บ + กรองแหล่ง + สรุปภาษาไทย |
| `migrations/018_research_agent.sql` | seed research agent config + ลบ URL sources + ลบ crawl schedule |
| แก้ `question.go`, `script.go` | news format ใช้ research แทน SearchRecent |
| แก้ `main.go` | ลบ crawler, เพิ่ม research agent wiring |

## เกณฑ์ความสำเร็จ

1. `go build ./...` + `go test ./...` + frontend build ผ่าน
2. ResearchAgent มี unit test (การประกอบ context + การ handle ผลว่าง)
3. ระบบ crawl หายไปหมด ไม่มี reference ค้าง
4. Research agent โผล่ในหน้า Agents (อัตโนมัติจาก API)
5. Deploy แล้ว server start ปกติ, migration 018 ผ่าน
