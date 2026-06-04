# Runbook: Apply migrations 026/027 + run 1 real test clip

วันที่: 2026-06-04 — สำหรับทดสอบ video redesign (Phase 1 + 2-4) กับ pipeline จริง

## สรุปกลไก (ตรวจจากโค้ดจริง)
- **Migrations apply อัตโนมัติตอน server boot** — `database.RunMigrations` (`cmd/server/main.go:51`) อ่านโฟลเดอร์ `migrations/`, ข้ามอันที่ apply แล้ว (ตาราง `schema_migrations`). **Deploy master ที่ push ไปแล้ว = 026 + 027 วิ่งเอง** ไม่ต้องรันมือ
- **Path redesign (multi-scene) เปิดด้วย env 2 ตัว**: `HYPERFRAMES_ENABLED=true` **และ** `HYPERFRAMES_MULTI_SCENE=true` (`config.go:55,57`) — ถ้าไม่ครบจะตกไป single-scene (ของเก่า)
- **Key ที่ใช้**: `openrouter_api_key` ในตาราง `settings` (ใช้ทำ TTS + รูป — `openrouter.go:38`). multi-scene **ไม่ต้องใช้** `openai_api_key`/Whisper แล้ว (แคปชั่นมาจากบทพูดต้นฉบับ). LLM agents ใช้ env `CLAUDE_API_KEY`
- **Trigger 1 คลิป**: `POST /api/v1/orchestrator/produce` body `{"count":1}` (auth ด้วย header `API_KEY`). สร้างคลิปอย่างเดียว **ไม่ auto-publish** (publish เป็น endpoint แยก)

---

## ขั้นตอน

### 1. Deploy master ขึ้น Railway
master ถูก push แล้ว (`feca51c`). สั่ง deploy service ตามปกติ → ตอน boot จะเห็น log:
```
Applied migration: 026_composition_scenes_content_driven_bg.sql
Applied migration: 027_composition_scenes_creative_design.sql
```
(ถ้าไม่เห็น = อาจ apply ไปแล้ว หรือ migrations dir ไม่อยู่ใน image — ตรวจ log)

### 2. ตั้ง env vars บน Railway service
```
HYPERFRAMES_ENABLED=true
HYPERFRAMES_MULTI_SCENE=true
```
(DATABASE_URL, CLAUDE_API_KEY, API_KEY ควรมีอยู่แล้ว)

### 3. ตรวจ settings table มี openrouter_api_key
ผ่านหน้า Settings ของแอป หรือ `GET /api/v1/settings`. ถ้ายังไม่มี ให้ใส่ผ่าน Settings page

### 4. Trigger สร้าง 1 คลิป
แทน `<URL>` ด้วย Railway domain และ `<API_KEY>` ด้วยค่าจริง:
```bash
curl -X POST "https://<URL>/api/v1/orchestrator/produce" \
  -H "API_KEY: <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"count":1}'
```

### 5. ดูว่า multi-scene ทำงานจริง (ไม่ตกไป fallback)
ดู log ต้องเจอ:
```
Multi-scene assembly for <clipID> (Hyperframes)
synthScenesVoice: N scenes ...
assembleMultiScene: ... (per-scene bg gen)
```
ถ้าเจอ `falling back to single-scene` หรือ `Multi-scene ... failed, falling back` = multi-scene ล้ม (ดู error ตามหลัง)

### 6. ดูผลลัพธ์
- `GET /api/v1/clips` → หา clip ล่าสุด, ดู status + วิดีโอ (9:16 + 16:9)
- ตรวจด้วยตา: ภาพพื้นหลังตรงเนื้อหาแต่ละฉากไหม / layout หลากหลาย (hook_punch ฯลฯ) / สีไล่ตามเนื้อหา / แคปชั่นตรงเสียง

---

## ⚠️ หมายเหตุ / ความเสี่ยง
- การรัน produce **เสียค่า API จริง** (TTS + รูป AI หลายใบ + LLM) — count=1 ก่อนเสมอ
- ครั้งแรกที่รันด้วย prompt ใหม่ (027) ผล AI ไม่แน่นอน — ดู 1 คลิปก่อนค่อยปรับ prompt ถ้าจำเป็น
- ถ้า bg image gen ล้มบางฉาก จะ downgrade เป็น CSS background (ไม่ fatal)
- migration ใช้ schema เดิม (แค่ UPDATE prompt) — ถ้าต้อง rollback: รัน SQL ของ 023/026 ทับ (runner ไม่ re-apply เอง)

## รันแบบ local แทนได้ไหม
ได้ถ้ามี `DATABASE_URL` (ชี้ Neon dev) + keys ครบ + ffmpeg/node/chromium: ตั้ง 2 env ข้างบน → `go run ./cmd/server` (migrations วิ่งตอน boot) → ยิง curl ที่ `localhost:<PORT>` → ดูไฟล์ใน `work/<clipID>/`
