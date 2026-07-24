# Case Visual Full-Frame Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans (inline execution ในเซสชันควบคุม — งานเล็ก 5 จุดผูกกัน ไฟล์ทั้งหมดอยู่ในบริบทผู้คุมแล้ว) + review agent อิสระบน diff ก่อน merge

**Goal:** ภาพปกโต๊ะนักสืบ + reuse ใน hero/verdict + จัดเต็มเฟรมซีนไร้ภาพ (ลายน้ำเลขคดี/texture/ยกกึ่งกลาง/หัวซีน comic) — งบภาพคง 2 ใบ/คลิป
**Spec:** docs/superpowers/specs/2026-07-24-case-visual-fullframe-design.md
**Branch:** feat/case-visual-fullframe (จาก master)

## Global Constraints
เหมือนแผน case-file-format เดิมทุกข้อ: ห้าม `-->` ใน script, Thai-safe CSS, GSAP-only,
classic byte-stable, migration BEGIN/COMMIT เอง, prompt ห้าม {{if}}, fail-open

### Task 1: Go — filter + cover prompt + adapter hook (TDD)
- `case_format.go`: `evidenceImageScenes` รับ layout casefile ด้วย (cap 2 เดิม, ชื่อฟังก์ชันเปลี่ยนเป็น `caseImageScenes` ให้ตรงความหมาย — อัปเดต call sites 2 จุดใน producer.go); `buildCoverPrompt(concept, preset, clipToken)` = `buildImagePromptCore(concept, "cinematic high-angle desk scene at night, the key subject placed in the UPPER half of the frame, lower half dark and uncluttered", ...)`; `promptForScene` แยกตาม `agent.ClampLayout(s.Layout)`: casefile→cover, evidence→evidence, classic→scene
- `composition_types.go`: `SceneContent.Hook string \`json:"hook,omitempty"\``
- `scene_adapter.go`: ใน buildSceneContent หลัง clamp — `if c.Layout=="casefile" { c.Hook = highlightTitleStr(clean(strings.TrimSpace(s.OnScreenText)), s.EmphasisWords) }`
- Tests: อัปเดต TestEvidenceImageScenes (casefile eligible → allowed={1,2} จาก fixture เดิม), เพิ่ม TestBuildCoverPrompt (มี "UPPER half", ไม่มี "centered"), TestPromptForSceneRouting, adapter casefile Hook test
- Commit: `feat(case-visual): cover image eligibility + cover prompt + hook headline field`

### Task 2: composition + template
- `composition.go`: `scenesTemplateData.CaseNumber int` + ส่ง `CaseNumber: p.CaseNumber`
- template: `const CASE_NO={{.CaseNumber}};` + `const FORMAT_CASE={{if eq .Format "case"}}true{{else}}false{{end}};` ใน script ก่อน SCENES
- CSS ใหม่: `.cf-hook` (88px, .acc amber, Thai-safe), `.cf-wm` (ลายน้ำ 190px Kanit rgba(188,210,255,.07) top:150px right:36px), texture `[data-format='case'] .scene-bg::after` (จุด halftone จาง), scrim อ่อนสำหรับ casefile, ตำแหน่ง: casefile `top:130px;bottom:430px;space-between`, comic/board/step/stat `top:150px;bottom:430px;center`, hero/verdict reuse-bg dim (`opacity:.45;filter:saturate(.7)` บน .scene-bg)
- JS: casefile branch prepend `if(sc.hook)c.appendChild(el("h1","title cf-hook",sc.hook));` — wrapper builder: reuse bg `const bgSrc=(!sc.bg&&(sc.type==="hero"||sc.type==="verdict")&&FORMAT_CASE&&SCENES[0]&&SCENES[0].bg)?SCENES[0].bg:sc.bg;` + ลายน้ำ `(FORMAT_CASE&&CASE_NO>0&&{comic:1,board:1,step:1,stat:1}[sc.type]?'<div class="cf-wm">คดี '+CASE_NO+'</div>':'')`
- Render tests: case มี cf-hook/cf-wm/CASE_NO=91; classic ไม่มี (test เดิม) + `-->` guard เดิม
- Commit: `feat(case-visual): template full-frame — hook headline, watermark, texture, bg reuse`

### Task 3: migration 060
- UPDATE scene_case แถวเดียว 2 จุดด้วย REPLACE (แบบ 056): (a) ย่อหน้ากฎภาพเดิม → กฎใหม่ (casefile ฉากโต๊ะ+ของกลาง วัตถุครึ่งบน / evidence 1 ซีน / อื่น ""), (b) บรรทัด schema comic → เพิ่ม `"kicker":"หัวเรื่องสั้น",` — BEGIN/COMMIT, guard NOT LIKE กัน apply ซ้ำ
- Commit: `migration: 060 case cover image rule + comic kicker`

### Task 4: verify + review + ship
- `go build ./... && go test ./... -count=1` เขียวหมด + flag purity grep
- Review agent อิสระบน diff ทั้ง branch → แก้ Critical/Important
- PR → merge → deploy → produce 1 คลิป → ส่ง user eyeball
