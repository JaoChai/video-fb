# Search-intent Metadata + Title Bug Fix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ทำให้ YouTube title/tags/description ของคลิปดักการค้นหาของกลุ่มเป้าหมาย และแก้บั๊ก title ที่แบรนด์ซ้ำ/ตัดกลางคำ

**Architecture:** แตะเฉพาะชั้น metadata — (1) แก้ฟังก์ชัน `validateScript` ใน orchestrator ให้ normalize title ทนทุกรูปแบบแบรนด์ (โค้ด + unit test), (2) migration `028` อัปเดต `prompt_template` ของ agent `script` ใน DB ให้ gen metadata แบบ search-intent + สั่ง LLM ห้ามเติมแบรนด์เอง ไม่กระทบ render/วิดีโอ

**Tech Stack:** Go 1.25, pgx, Postgres (Neon), file-based SQL migrations (`internal/database/migrations.go`, track by filename)

**Spec:** `docs/superpowers/specs/2026-06-05-search-intent-metadata-design.md`

---

## File Structure

- `internal/orchestrator/orchestrator.go` — แก้ `validateScript()` (บรรทัด ~197-211)
- `internal/orchestrator/validate_script_test.go` — **สร้างใหม่** unit test ของ `validateScript`
- `migrations/028_search_intent_metadata.sql` — **สร้างใหม่** UPDATE `prompt_template` ของ agent `script`

หมายเหตุ: `validateScript` เป็น unexported ในแพ็กเกจ `orchestrator` → test อยู่ในแพ็กเกจเดียวกัน (`package orchestrator`) เข้าถึงได้ตรง

---

## Task 1: แก้บั๊ก title normalization (`validateScript`)

**Files:**
- Modify: `internal/orchestrator/orchestrator.go:197-211`
- Test: `internal/orchestrator/validate_script_test.go` (create)

- [ ] **Step 1: เขียน failing test**

สร้าง `internal/orchestrator/validate_script_test.go`:

```go
package orchestrator

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestValidateScriptTitle(t *testing.T) {
	const suffix = " | Ads Vance"
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "LLM added curly brand → no dup, no mid-cut",
			in:   "เพิ่มงบแอดแล้วโดนแบน แก้ยังไง? {Ads Vance}",
			want: "เพิ่มงบแอดแล้วโดนแบน แก้ยังไง?" + suffix,
		},
		{
			name: "LLM added pipe brand → single suffix",
			in:   "บัญชีโฆษณาโดนแบน แก้ยังไง? | Ads Vance",
			want: "บัญชีโฆษณาโดนแบน แก้ยังไง?" + suffix,
		},
		{
			name: "double brand → collapsed to one",
			in:   "เพิ่มงบแล้วโดนแบน {Ads Vance} | Ads Vance",
			want: "เพิ่มงบแล้วโดนแบน" + suffix,
		},
		{
			name: "no brand → suffix appended",
			in:   "Pixel ไม่นับ Lead แก้ด้วยวิธีนี้",
			want: "Pixel ไม่นับ Lead แก้ด้วยวิธีนี้" + suffix,
		},
		{
			name: "paren brand variant",
			in:   "BM โดนระงับ ทำไงดี (Ads Vance)",
			want: "BM โดนระงับ ทำไงดี" + suffix,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &agent.GeneratedScript{YoutubeTitle: tt.in}
			validateScript(s)
			if s.YoutubeTitle != tt.want {
				t.Errorf("got %q, want %q", s.YoutubeTitle, tt.want)
			}
		})
	}
}

func TestValidateScriptTitleLength(t *testing.T) {
	const suffix = " | Ads Vance"
	longTitle := strings.Repeat("ก", 100) + " {Ads Vance}"
	s := &agent.GeneratedScript{YoutubeTitle: longTitle}
	validateScript(s)

	if l := len([]rune(s.YoutubeTitle)); l > 70 {
		t.Errorf("title length = %d runes, want <= 70", l)
	}
	if !strings.HasSuffix(s.YoutubeTitle, suffix) {
		t.Errorf("title %q must end with exactly one brand suffix", s.YoutubeTitle)
	}
	if strings.Count(s.YoutubeTitle, "Ads Vance") != 1 {
		t.Errorf("title %q must contain brand exactly once", s.YoutubeTitle)
	}
	// no dangling separator/bracket right before the suffix
	core := strings.TrimSuffix(s.YoutubeTitle, suffix)
	if strings.HasSuffix(core, "{") || strings.HasSuffix(core, "(") || strings.HasSuffix(core, "|") {
		t.Errorf("title core %q has dangling separator", core)
	}
}
```

- [ ] **Step 2: รัน test ให้แน่ใจว่า fail**

Run: `go test ./internal/orchestrator/ -run TestValidateScript -v`
Expected: FAIL (เคส curly/paren/double-brand จะได้ผลแบรนด์ซ้ำหรือตัดกลาง)

- [ ] **Step 3: แก้ `validateScript` ให้ normalize ทนทาน**

แทนที่ฟังก์ชัน `validateScript` (orchestrator.go:197-211) ด้วย:

```go
// brandTailRe matches a trailing brand mention in any form the LLM might add:
// " | Ads Vance", "| Ads Vance", "{Ads Vance}", "(Ads Vance)", " Ads Vance".
var brandTailRe = regexp.MustCompile(`(?i)\s*[|({\[]?\s*ads\s*vance\s*[)}\]]?\s*$`)

func validateScript(script *agent.GeneratedScript) {
	const suffix = " | Ads Vance"
	const maxLen = 70

	// Strip any brand variant the LLM appended — repeat to catch doubled brands.
	title := script.YoutubeTitle
	for {
		stripped := strings.TrimRight(brandTailRe.ReplaceAllString(title, ""), " |-({[")
		if stripped == title {
			break
		}
		title = stripped
	}
	title = strings.TrimSpace(title)

	maxContent := maxLen - len([]rune(suffix))
	if titleRunes := []rune(title); len(titleRunes) > maxContent {
		title = strings.TrimRight(strings.TrimSpace(string(titleRunes[:maxContent])), " |-({[")
		log.Printf("Warning: youtube_title truncated to fit %d chars", maxLen)
	}

	script.YoutubeTitle = title + suffix
}
```

เพิ่ม `"regexp"` ใน import block ของ orchestrator.go (ถ้ายังไม่มี). `strings` และ `log` มีอยู่แล้ว (ใช้ใน sanitizeVoiceText/เดิม).

- [ ] **Step 4: รัน test ให้ผ่าน + ทั้งแพ็กเกจ**

Run: `go test ./internal/orchestrator/ -v`
Expected: PASS ทั้งหมด (รวม `TestSanitizeVoiceText` เดิม)

- [ ] **Step 5: Build ทั้งโปรเจกต์**

Run: `go build ./... && go vet ./internal/orchestrator/`
Expected: ไม่มี error

- [ ] **Step 6: /simplify ก่อน commit**

รัน `/simplify` บน diff ของ Task นี้ (ตามที่ผู้ใช้กำหนด) — เกลาให้กระชับ/ตรงแนว codebase แล้วรัน `go test ./internal/orchestrator/` ซ้ำให้เขียวก่อนไปต่อ

- [ ] **Step 7: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/orchestrator/validate_script_test.go
git commit -m "fix(metadata): normalize youtube_title — strip any brand variant, no mid-cut/dup"
```

---

## Task 2: Migration 028 — prompt search-intent ของ agent `script`

**Files:**
- Create: `migrations/028_search_intent_metadata.sql`

- [ ] **Step 1: สร้างไฟล์ migration**

สร้าง `migrations/028_search_intent_metadata.sql` (อัปเดตเฉพาะ `prompt_template` ของ agent `script` — ส่วนโครงสร้าง scene คงเดิมจาก migration 017, เปลี่ยน 3 bullet metadata + เอาคำสั่งเติมแบรนด์ออก):

```sql
-- Migration 028: retune script agent metadata for SEARCH intent.
-- Target audience searches for fixes when their ad accounts break, so:
--  - youtube_title: lead with the searched problem phrase; do NOT add brand
--    (orchestrator.validateScript appends " | Ads Vance" exactly once).
--  - youtube_description: first line = searchable problem+fix summary, THEN contact lines.
--  - youtube_tags: real search phrases.
-- Scene structure unchanged from 017 (single-scene).

UPDATE agent_configs
SET prompt_template = $tmpl$สร้าง voice script + ข้อมูล metadata สำหรับวิดีโอสั้น

โครงสร้างวิดีโอ: ใช้ "ภาพเดียว" คงที่ตลอดทั้งคลิป + "เสียงพากย์เดียว" เล่าจบในตัว (ไม่มีการตัดฉาก ไม่มี multi-scene)

หัวข้อ: "{{.Question}}"
โดย: {{.QuestionerName}}
หมวด: {{.Category}}

รูปแบบการเล่า: {{.FormatInstruction}}

กลุ่มเป้าหมาย: {{.AudiencePersona}}

ข้อมูลอ้างอิง:
{{.RAGContext}}

ตอบเป็น JSON object มี:
- "scenes": array ที่มี object **เพียง 1 ตัวเท่านั้น** (วิดีโอนี้ออกแบบเป็น single-scene):
  - "scene_number": 1
  - "scene_type": "main"
  - "text_content": ข้อความสั้นสำหรับแสดงบนภาพ (เน้นหัวข้อ)
  - "voice_text": บทพากย์ภาษาไทยแบบธรรมชาติ ไหลลื่น เล่าตามรูปแบบการเล่าที่กำหนดข้างบน
  - "duration_seconds": 30-55 (ให้พอดี YouTube Shorts)
  - "text_overlays": []
- "total_duration_seconds": 30-55

**กฎสำคัญสำหรับ voice_text** (ป้องกันเสียงตัด/อ่านผิด):
- **ห้ามมีอักขระ "@" และห้ามมี URL ใดๆ** ใน voice_text เด็ดขาด (TTS อ่านลิงก์ไม่ออก เสียงจะตัด)
- เรียกชื่อแบรนด์ว่า "**แอดส์แวนซ์**" สะกดเป็นเสียงไทย (ห้ามเขียน "Adsvance", "@adsvance", "Ads Vance" ใน voice_text)
- ใช้ "..." สำหรับจังหวะหายใจระหว่างประโยค

- "youtube_title": พาดหัวสำหรับ "การค้นหา" — **ขึ้นต้นด้วยคำปัญหาที่กลุ่มเป้าหมายพิมพ์ค้นจริง** ตอนเจอปัญหา (เช่น "บัญชีโฆษณาโดนแบน", "BM โดนระงับ", "เพิ่มงบแล้วแอดโดนแบน") วางคำสำคัญไว้ต้นประโยค ตามด้วยวิธีแก้สั้นๆ เป็นภาษาที่คนพิมพ์ค้นจริง ไม่เกิน 55 ตัวอักษร
  **ห้ามเติมชื่อแบรนด์เองเด็ดขาด** (ห้ามมี " | Ads Vance", "{Ads Vance}", "(Ads Vance)", "Ads Vance" ใน youtube_title) — ระบบจะเติมต่อท้ายให้อัตโนมัติ
  ห้ามใส่ URL, line id, @handle ใน youtube_title
- "youtube_description": บรรทัดแรกเป็น **สรุปปัญหา + บอกว่ามีวิธีแก้** ด้วยภาษาที่คนค้นหา (1-2 ประโยค ใส่คำค้นสำคัญของหัวข้อนั้น) จากนั้นเว้นบรรทัด แล้วต่อด้วย 2 บรรทัดติดต่อนี้เป๊ะๆ ห้ามแก้:
  "ติดต่อทีมงาน line id : @adsvance\n\nเข้ากลุ่มเทเรแกรมเพื่อรับข่าวสาร : https://t.me/adsvancech"
- "youtube_tags": array ของ **วลีที่คนพิมพ์ค้นหาจริง** (ภาษาพูดไทย + ศัพท์หลัก เช่น "เฟสแบน", "บัญชีโฆษณาโดนระงับ", "แก้บัญชีโดนแบน", "facebook ads ban", "BM โดนแบน") 5-10 tag

ห้ามแนะนำการทำผิดนโยบาย Facebook$tmpl$
WHERE agent_name = 'script';
```

- [ ] **Step 2: ตรวจ syntax migration (dry compile)**

Run: `go build ./...`
Expected: ไม่มี error (migration เป็นไฟล์ embed/อ่าน runtime — ยืนยันว่า build ไม่พัง). หมายเหตุ: ไฟล์ 021-027 ที่ค้างใน `schema_migrations` ไม่มีไฟล์จริง → runner skip; ไฟล์ใหม่ 028 ชื่อไม่ซ้ำ → จะถูก apply ตอน boot

- [ ] **Step 3: /simplify ก่อน commit**

รัน `/simplify` บน diff (ตามที่ผู้ใช้กำหนด) — สำหรับ migration SQL ส่วนใหญ่จะไม่มีอะไรให้เกลา แต่ทำตามขั้นเพื่อความสม่ำเสมอ

- [ ] **Step 4: Commit**

```bash
git add migrations/028_search_intent_metadata.sql
git commit -m "feat(agent): retune script metadata prompt for search intent + no self-branding"
```

---

## Task 3: Deploy + verify บน prod (manual inspection)

**Files:** none (deploy + ตรวจสอบ)

- [ ] **Step 1: Deploy**

Deploy branch ขึ้น Railway (วิธีเดียวกับที่ทีมใช้: manual deploy ผ่าน Railway MCP/CLI จาก working tree). migration 028 จะถูก apply อัตโนมัติตอน server boot (`database.RunMigrations`)

- [ ] **Step 2: ยืนยัน migration ถูก apply**

รัน SQL (ผ่าน Neon):
```sql
SELECT left(prompt_template, 60) AS head,
       (prompt_template LIKE '%ห้ามเติมชื่อแบรนด์เอง%') AS has_no_brand_rule,
       (prompt_template LIKE '%วลีที่คนพิมพ์ค้นหาจริง%') AS has_search_tags
FROM agent_configs WHERE agent_name='script';
SELECT * FROM schema_migrations WHERE filename = '028_search_intent_metadata.sql';
```
Expected: `has_no_brand_rule = true`, `has_search_tags = true`, และมีแถว 028 ใน schema_migrations

- [ ] **Step 3: ตรวจ metadata ที่ gen จริง (manual)**

หลัง noon cron ผลิตคลิปถัดไป (หรือ trigger produce 1 คลิป) — ตรวจ `clip_metadata` ของคลิปใหม่:
```sql
SELECT cm.youtube_title, left(cm.youtube_description,80) AS desc_head
FROM clip_metadata cm JOIN clips c ON c.id=cm.clip_id
ORDER BY c.created_at DESC LIMIT 3;
```
Expected (ตรวจตา):
- title ขึ้นต้นด้วยคำปัญหา, ลงท้าย ` | Ads Vance` **ครั้งเดียว**, ไม่มี `{Ads V`/แบรนด์ซ้ำ, ≤70 ตัว
- description บรรทัดแรกเป็นสรุปปัญหา (มีคำค้น) แล้วตามด้วยบรรทัดติดต่อ line/telegram ครบ

> หมายเหตุความซื่อสัตย์: ผลต่อ "reach/views" วัดได้เฉพาะระยะยาว (สัปดาห์+) และ noisy — Task นี้ยืนยันได้แค่ว่า metadata "ถูกรูป" ตามตั้งใจ ไม่ใช่ว่า views เพิ่ม

---

## Self-Review (ทำแล้ว)

- **Spec coverage:** §3.1 → Task 1; §3.2 → Task 2; §5 testing → Task 1 (unit), Task 3 (manual); §3.2 migration numbering note → Task 2 Step 2. ครบ
- **Placeholder scan:** ไม่มี TBD/TODO — โค้ดและ SQL เต็มทุก step
- **Type consistency:** ใช้ `agent.GeneratedScript{YoutubeTitle}` (ตรงกับ struct จริง script.go:42-48), `validateScript(*agent.GeneratedScript)` (ตรง signature เดิม), suffix/maxLen ตรงกันทุก task
