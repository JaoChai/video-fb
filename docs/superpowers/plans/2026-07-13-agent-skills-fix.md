# Agent Skills/Prompt Audit Fix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** แก้ผล audit 2026-07-13 — เฟส 1 อัปเดตข้อความ prompt/skills 5 agents (migration เดียว), เฟส 2 ฟื้นระบบที่ตาย (metadata เจ้าของ title, image → ภาพปก cover scene, critic แตะ content, learner เกณฑ์สัมพัทธ์, analyzer ชี้เป้าใหม่) โดยของใหม่ทั้งหมดมี flag คุม default off

**Architecture:** เฟส 1 = migration SQL ล้วน (UPDATE agent_configs). เฟส 2 = โค้ด Go ใน internal/agent, internal/learner, internal/orchestrator, internal/analyzer + migration 054 (system_prompt/template ใหม่ + seed flags ใน settings). Flag อ่านจากตาราง settings ผ่าน `settingsRepo.Get` (pattern เดียวกับ `content_brain_v2_enabled`)

**Tech Stack:** Go 1.x, PostgreSQL (Neon), Go text/template, ตาราง `agent_configs`/`settings`/`clip_critiques`

**Spec:** `docs/superpowers/specs/2026-07-13-agent-skills-fix-design.md`

## Global Constraints

- flag ใหม่ 2 ตัว: `metadata_agent_enabled`, `cover_image_agent_enabled` — seed เป็น `'false'` ทั้งคู่; flag ปิด = พฤติกรรมเดิมทุก byte
- ห้ามแตะคอลัมน์ `insights` ใน migration ใดๆ (เป็นของ analyzer loop)
- brand suffix canonical เดียว: `" | Ads Vance"` ต่อโดย `validateScript` เท่านั้น
- **เบี่ยงจาก spec §2.5 (จงใจ):** คง title `maxLen = 90` ในโค้ด — test `TestValidateScriptTitleFitsUnderCap` (validate_script_test.go:94-109) บันทึกว่า cap 70 เคยตัดชื่อไทยกลางคำและถูกยกเป็น 90 โดยตั้งใจ; การลดกลับเป็น regression
- migration ใช้ dollar-quoting (`$txt$...$txt$`) สำหรับข้อความไทยที่มี quote
- ทุก commit ต้องผ่าน `go build ./... && go test ./...` ก่อน
- learner guardrails เดิมคงครบ: `minCritiques=8`, allowlist `scene`+`script`, audit-ก่อน-apply, `AcceptProposal`

---

### Task 1: Migration 053 — เฟส 1 แก้ข้อความ prompt/skills 5 agents

**Files:**
- Create: `migrations/053_agent_skills_audit_fixes.sql`

**Interfaces:**
- Consumes: ตาราง `agent_configs` (คอลัมน์ system_prompt, skills)
- Produces: ข้อความใหม่ใน DB — ไม่มีโค้ดอ่านเพิ่ม (BuildSystemPrompt inject ให้อยู่แล้ว)

- [ ] **Step 1: เขียนไฟล์ migration**

สร้าง `migrations/053_agent_skills_audit_fixes.sql` เนื้อหาทั้งไฟล์:

```sql
-- 053: Agent skills/prompt audit fixes (Phase 1 — text only, no logic change).
-- Source: docs/superpowers/specs/2026-07-13-agent-skills-fix-design.md
-- Rollback: previous values are in agent_prompt_history / git history of 030-052 seeds;
-- to revert, re-run the corresponding seed UPDATE from the prior migration.
-- NOTE: deliberately does NOT touch the `insights` column (owned by the analyzer loop).

-- 1.1 script: flip persona from beginner-friendly to insider voice (kills the
-- conflict with the content-brain-v2 prompt_template + skills).
UPDATE agent_configs SET system_prompt = $txt$คุณคือ scriptwriter คลิปสั้น Q&A Facebook Ads ของแบรนด์ Ads Vance สำหรับคนยิงแอดจริงจัง (media buyer / agency / คนถือหลายบัญชี)

สไตล์:
- เสียงคนวงในที่บริหารบัญชีโฆษณาจำนวนมากมาเอง พูดลื่น เป็นกันเอง แต่ไม่ใช่มือใหม่
- ใช้ศัพท์วงในได้เลยไม่ต้องนิยาม (Learning Limited, CBO, spending limit ฯลฯ) — คนดูรู้อยู่แล้ว การหยุดอธิบายพื้นฐานทำให้ดูเป็นช่องมือใหม่
- ห้ามเนื้อหาระดับ 101 (สอนสมัครบัญชี สอนยิงแอดครั้งแรก)
- เข้าทางแก้ภายใน 5-10 วินาทีแรก ไม่ทวนคำถาม เน้นสเต็ปที่ทำตามได้จริง$txt$
WHERE agent_name = 'script';

-- 1.2 critic: fix CTA channel (no "เพจ"), add FB-policy guard, protect money-hook.
UPDATE agent_configs SET skills = $txt$- hook สายนี้: ตัวเลขเงินช็อก / ป้ายสถานะถูกปฏิเสธ / เดดไลน์บีบ — ห้ามลดความแรงของ hook โดยอ้างว่า clickbait ถ้าเนื้อหาเป็นเรื่องจริงตามบท
- CTA ปลายคลิป: soft sell ชวนทักไลน์ / กลุ่มเทเลแกรม / ทักทีมงาน เท่านั้น (ห้ามเปลี่ยนเป็น "ทักเพจ" — ไม่มีช่องทางนี้)
- ห้ามแก้เนื้อหาไปทางสอนหลบระบบตรวจจับ / ปลอมตัวตน / ทำผิดนโยบาย Facebook แม้จะทำให้ hook แรงขึ้น
- เลี่ยงศัพท์ทางการเกินไป ใช้คำที่คนวงในพูดจริง
- Hook: scene แรก on_screen_text ต้อง <=7 คำ และช็อก/ชวนสงสัยใน 1 วิ ถ้ายาว/อืดให้ตัดให้สั้นคม
- emphasis_words: ทุกซีนต้องมีคำเน้น 1-2 คำที่ตรงประเด็น (ระบบใช้ไฮไลต์แคปชั่น) ถ้าว่าง/ผิดให้เติม
- อย่าบังคับสไตล์ภาพใน image_prompt (สไตล์มาจากธีม) — ปรับได้แค่ "วัตถุ/ฉาก" ให้ตรงเนื้อหา$txt$
WHERE agent_name = 'critic';

-- 1.3 analytics: platform-aware, no CTR (never fed to it), relative benchmarks.
UPDATE agent_configs SET
  system_prompt = $txt$คุณคือนักวิเคราะห์ประสิทธิภาพวิดีโอ YouTube และ TikTok ของแบรนด์ Ads Vance

หน้าที่: วิเคราะห์ตัวเลขประสิทธิภาพวิดีโอแล้วให้คำแนะนำที่ปฏิบัติได้จริง เน้นว่าควรปรับอะไรในคลิปถัดไป

ตอบเป็นภาษาไทย ชัดเจน ตรงประเด็น ไม่อ้อมค้อม$txt$,
  skills = $txt$- แยกแพลตฟอร์มก่อนวิเคราะห์: TikTok มีแค่ views / likes / shares / engagement rate (ไม่มี retention, watch time); YouTube มี views / watch time / avg view percentage / engagement
- benchmark แบบ relative: เทียบกับผลงานของช่องเอง 30 วันย้อนหลัง แยกตามแพลตฟอร์ม+หมวด — อย่าใช้เกณฑ์ตายตัว (สเกลช่องตอนนี้คลิปท็อป ~300-400 วิว)
- AvgViewPct เกิน 100% = คนดูวนซ้ำ (loop) ไม่ใช่ดูจบเกิน 100% — อ่านเป็นสัญญาณว่าคลิปสั้น+ชวนวน ไม่ใช่ watch-through
- ระบุชัดว่าอะไรดี อะไรต้องปรับ — actionable ต่อคลิปถัดไป ไม่ใช่แค่รายงานตัวเลข
- ถ้า engagement สูง: หัวข้อนี้ hit ให้ทำเพิ่มในมุมอื่น; แนะนำหมวดที่ควรทำต่อจาก performance จริงของหมวดนั้น$txt$
WHERE agent_name = 'analytics';

-- 1.4 scene: drop dead `layout_variant` wording, add layout arc guidance.
UPDATE agent_configs SET skills = $txt$- แตก 6-10 ซีน หนึ่งซีนหนึ่งไอเดีย
- วาง arc ของคลิป: hook (ซีน 1) → hero/stat ขยายปัญหา → step/tip ทางแก้ → cta ปิดท้าย; คลิปบทบาท convert ต้องมี step ที่ทำตามได้จริงก่อน cta, คลิปบทบาท reach เน้น stat/hero แล้วปิดชวนดูต่อ
- อย่าใช้ layout เดียวกันเกิน 2 ซีนติดกัน สลับให้มีจังหวะ
- emphasis_words ทุกซีนห้ามว่าง — ระบบใช้ไฮไลต์แคปชั่น
- on_screen_text สั้น อ่านรู้เรื่องตอนปิดเสียง คุมความยาวตามลิมิตในกติกา
- output เป็น JSON ตาม schema เท่านั้น$txt$
WHERE agent_name = 'scene';

-- 1.5 question: keep existing bullets, append persona tone + hook-polarity rotation.
UPDATE agent_configs SET skills = $txt$สร้างคำถามที่หลากหลายจริงๆ ทั้งมุมปัญหา ระดับความลึก และสถานการณ์
- กลุ่มเป้าหมายคือคนยิงแอดจริงจัง (เจ้าของธุรกิจออนไลน์, media buyer, agency) ไม่ใช่มือใหม่หัดยิงแอด
- คำถามต้องเจาะจง มีรายละเอียดสถานการณ์จริง (ตัวเลขงบ, ระยะเวลา, สิ่งที่ลองแล้ว)
- ห้ามตั้งคำถามที่ความหมายซ้ำหรือใกล้เคียงกับหัวข้อที่เคยทำแล้ว แม้จะใช้คำต่างกัน
- กระจายความหลากหลาย: ปัญหาเร่งด่วน / เทคนิคขั้นสูง / ความเข้าใจผิดที่พบบ่อย / การตัดสินใจเชิงกลยุทธ์
- โทนคำถาม: มือโปรพิมพ์สั้นเหมือนถามใน LINE แต่เนื้อในเป็นปัญหาของคนถือหลายบัญชี/ยิงหนัก ไม่ใช่ SME มือใหม่
- หมุนขั้ว hook อย่าให้ทุกข้อเป็น "เงินหาย": เงินหาย / เดดไลน์บีบเวลา / ป้ายสถานะถูกปฏิเสธ / เคลมที่สวนสามัญสำนึก$txt$
WHERE agent_name = 'question';
```

- [ ] **Step 2: ตรวจ SQL syntax แบบ dry-run**

Run: `docker run --rm -v /Users/jaochai/Code/video-fb/migrations:/m postgres:16 bash -c "psql --version" 2>/dev/null || echo "no docker — visual check only"`

ถ้าไม่มี docker: ตรวจด้วยตาว่า (1) ทุก `$txt$` เปิด-ปิดครบคู่ 12 ตัว (6 บล็อก) (2) ทุก statement ปิดด้วย `;` (3) `WHERE agent_name = '...'` ครบทุก UPDATE — ห้ามมี UPDATE ที่ไม่มี WHERE

- [ ] **Step 3: Commit**

```bash
git add migrations/053_agent_skills_audit_fixes.sql
git commit -m "feat(agents): migration 053 — phase-1 skills/prompt text fixes from audit 2026-07-13"
```

---

### Task 2: Critic merge `content` กลับใน reconcile

**Files:**
- Modify: `internal/agent/critic.go:104-110` (ใน `reconcileCritique`)
- Test: `internal/agent/critic_test.go`

**Interfaces:**
- Consumes: `GeneratedScene.Content json.RawMessage` (script.go:52), `reconcileCritique(in CriticInput, out CriticOutput) CriticResult`
- Produces: reconcile ที่ copy `Content` จาก critic เมื่อ valid JSON — render-time `buildSceneContent` (scene_adapter.go:126) จะ clamp ความยาว/emoji ให้เองอยู่แล้ว จึงไม่ต้อง sanitize ซ้ำใน critic

- [ ] **Step 1: เขียน test ที่ fail**

เพิ่มใน `internal/agent/critic_test.go`:

```go
// Critic-revised structured content must flow through reconcile; invalid or
// missing content keeps the original (render-time buildSceneContent clamps
// lengths, so no sanitize needed here).
func TestReconcileCritiqueMergesContent(t *testing.T) {
	origContent := json.RawMessage(`{"kicker":"เดิม","rows":[{"t":"แถวเดิม","bad":true}]}`)
	newContent := json.RawMessage(`{"kicker":"ใหม่","rows":[{"t":"แถวใหม่ คมกว่า","bad":true}]}`)

	in := CriticInput{
		Scenes: []GeneratedScene{{SceneNumber: 1, VoiceText: "v", Layout: "hook", Content: origContent}},
	}

	cases := []struct {
		name    string
		content json.RawMessage
		want    string
	}{
		{"valid content replaces original", newContent, string(newContent)},
		{"invalid JSON keeps original", json.RawMessage(`{broken`), string(origContent)},
		{"empty keeps original", nil, string(origContent)},
		{"null keeps original", json.RawMessage(`null`), string(origContent)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := CriticOutput{
				Scenes: []GeneratedScene{{SceneNumber: 1, VoiceText: "v2", Content: tc.content}},
				Score:  CriticScore{Hook: 8, Clarity: 8, BrandFit: 8, Overall: 8},
			}
			res := reconcileCritique(in, out)
			if !res.Applied {
				t.Fatalf("Applied = false, want true")
			}
			if got := string(res.Scenes[0].Content); got != tc.want {
				t.Errorf("Content = %s, want %s", got, tc.want)
			}
			if res.Scenes[0].Layout != "hook" {
				t.Errorf("Layout changed to %q — must stay immutable", res.Scenes[0].Layout)
			}
		})
	}
}
```

- [ ] **Step 2: รัน test ให้เห็นว่า fail**

Run: `go test ./internal/agent/ -run TestReconcileCritiqueMergesContent -v`
Expected: FAIL — เคส "valid content replaces original" ได้ค่าเดิมเพราะ reconcile ยังไม่ copy Content

- [ ] **Step 3: แก้ reconcileCritique**

ใน `internal/agent/critic.go` แก้บล็อก merge (บรรทัด ~104-110) จาก:

```go
		m := orig // copy keeps every structural/timing/layout field
		m.VoiceText = cs.VoiceText
		m.OnScreenText = cs.OnScreenText
		m.TextContent = cs.TextContent
		m.ImagePrompt = cs.ImagePrompt
		m.EmphasisWords = cs.EmphasisWords
		merged[i] = m
```

เป็น:

```go
		m := orig // copy keeps every structural/timing/layout field
		m.VoiceText = cs.VoiceText
		m.OnScreenText = cs.OnScreenText
		m.TextContent = cs.TextContent
		m.ImagePrompt = cs.ImagePrompt
		m.EmphasisWords = cs.EmphasisWords
		// Structured card content (what viewers actually read). Take the critic's
		// version only when it is real JSON; render-time buildSceneContent clamps
		// lengths and strips emoji, and layout stays pinned from the original.
		if len(cs.Content) > 0 && string(cs.Content) != "null" && json.Valid(cs.Content) {
			m.Content = cs.Content
		}
		merged[i] = m
```

- [ ] **Step 4: รัน test ให้ผ่าน + ทั้ง package**

Run: `go test ./internal/agent/ -v -run TestReconcile`
Expected: PASS ทุกเคส (รวม test reconcile เดิม)

- [ ] **Step 5: Commit**

```bash
git add internal/agent/critic.go internal/agent/critic_test.go
git commit -m "feat(critic): merge critic-revised structured content in reconcile (layout stays immutable)"
```

---

### Task 3: Learner เกณฑ์สัมพัทธ์ (regression + frequency gate)

**Files:**
- Modify: `internal/learner/learner.go`
- Test: `internal/learner/learner_test.go`

**Interfaces:**
- Consumes: `repository.ScorePatterns{N, AvgHook, AvgClarity, AvgBrandFit, AvgOverall, TopIssues []FieldIssue}`, `FieldIssue.Count`, `LowScorePatterns(ctx, sinceDays, topN)` (มีอยู่แล้ว — เรียกซ้ำด้วย 90 วันเป็น baseline, ไม่ต้องเพิ่ม method ใหม่)
- Produces: `strongSignal(p, base repository.ScorePatterns) (bool, string, float64, string)` — คืน (fire?, weakestDim, weakestVal, gateReason)

- [ ] **Step 1: เขียน test ที่ fail**

แก้/เพิ่มใน `internal/learner/learner_test.go` (test เดิมของ `strongSignal` ที่ใช้ signature เก่าให้ปรับมาใช้ signature ใหม่ในขั้นนี้ด้วย):

```go
func TestStrongSignalRelativeGates(t *testing.T) {
	base := repository.ScorePatterns{N: 40, AvgHook: 8.0, AvgClarity: 8.2, AvgBrandFit: 8.8, AvgOverall: 8.1}

	cases := []struct {
		name string
		p    repository.ScorePatterns
		base repository.ScorePatterns
		want bool
		gate string
	}{
		{
			name: "too few critiques never fires",
			p:    repository.ScorePatterns{N: 5, AvgHook: 3.0, AvgClarity: 8, AvgBrandFit: 8, AvgOverall: 8},
			base: base, want: false, gate: "insufficient",
		},
		{
			name: "regression: weakest dim 0.6 below its own 90d baseline fires",
			p:    repository.ScorePatterns{N: 12, AvgHook: 7.4, AvgClarity: 8.1, AvgBrandFit: 8.7, AvgOverall: 8.0},
			base: base, want: true, gate: "regression",
		},
		{
			name: "flat scores near baseline do not fire",
			p:    repository.ScorePatterns{N: 12, AvgHook: 7.8, AvgClarity: 8.1, AvgBrandFit: 8.8, AvgOverall: 8.0},
			base: base, want: false, gate: "no_gate",
		},
		{
			name: "frequency: top issue in >=40% of critiques fires even without regression",
			p: repository.ScorePatterns{N: 10, AvgHook: 7.9, AvgClarity: 8.1, AvgBrandFit: 8.8, AvgOverall: 8.0,
				TopIssues: []repository.FieldIssue{{Field: "scene[0].voice_text", Reason: "hook อืด", Count: 5}}},
			base: base, want: true, gate: "frequency",
		},
		{
			name: "insufficient baseline disables regression gate (frequency can still fire)",
			p:    repository.ScorePatterns{N: 12, AvgHook: 6.0, AvgClarity: 8.1, AvgBrandFit: 8.8, AvgOverall: 8.0},
			base: repository.ScorePatterns{N: 3, AvgHook: 8.0}, want: false, gate: "no_gate",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, _, gate := strongSignal(tc.p, tc.base)
			if got != tc.want {
				t.Errorf("fire = %v, want %v (gate=%s)", got, tc.want, gate)
			}
			if tc.want && gate != tc.gate {
				t.Errorf("gate = %q, want %q", gate, tc.gate)
			}
		})
	}
}
```

- [ ] **Step 2: รัน test ให้เห็นว่า compile fail**

Run: `go test ./internal/learner/ -run TestStrongSignalRelativeGates -v`
Expected: FAIL (compile error — `strongSignal` รับ argument เดียวและคืน 3 ค่า)

- [ ] **Step 3: แก้ learner.go**

(a) แทนที่ const block (บรรทัด 16-26):

```go
const (
	// windowDays is how far back LowScorePatterns aggregates for the CURRENT window.
	windowDays = 30
	// baselineDays is the trailing window the current scores are compared against.
	baselineDays = 90
	// minCritiques is the minimum critique rows (in a window) before we act.
	minCritiques = 8
	// regressionMargin: the weakest dimension must sit this far below its own
	// baseline average to count as a real regression. Replaces the old absolute
	// lowScoreThreshold=6.0 which real scores (~7.8-8.9) could never reach.
	regressionMargin = 0.5
	// issueFrequencyThreshold: alternatively fire when the single most common
	// critique issue appears in at least this fraction of window critiques.
	issueFrequencyThreshold = 0.4
	// topIssuesN caps how many recurring issues feed the pattern summary.
	topIssuesN = 8
)
```

(b) เพิ่ม helper และแทนที่ `strongSignal` (บรรทัด 55-67):

```go
// dimValue returns the average for a named score dimension. Pure.
func dimValue(p repository.ScorePatterns, name string) float64 {
	switch name {
	case "hook":
		return p.AvgHook
	case "clarity":
		return p.AvgClarity
	case "brand_fit":
		return p.AvgBrandFit
	default:
		return p.AvgOverall
	}
}

// strongSignal is the pure gate, now RELATIVE to the agent's own history:
// fire on (a) regression — the weakest current dimension sits regressionMargin
// below the same dimension's baseline average (baseline must itself have enough
// rows), or (b) frequency — one issue recurs in >= issueFrequencyThreshold of
// window critiques. Returns (fire, weakest-dim, weakest-val, gate-name) so the
// caller logs exactly why it acted or skipped.
func strongSignal(p, base repository.ScorePatterns) (bool, string, float64, string) {
	name, val := p.LowestDimension()
	if p.N < minCritiques {
		return false, name, val, "insufficient"
	}
	if base.N >= minCritiques && val < dimValue(base, name)-regressionMargin {
		return true, name, val, "regression"
	}
	if len(p.TopIssues) > 0 && float64(p.TopIssues[0].Count) >= issueFrequencyThreshold*float64(p.N) {
		return true, name, val, "frequency"
	}
	return false, name, val, "no_gate"
}
```

(c) ใน `RunOnce`: หลังบรรทัด `patterns, err := l.critiques.LowScorePatterns(ctx, windowDays, topIssuesN)` (บรรทัด ~101-104) เพิ่ม:

```go
	baseline, err := l.critiques.LowScorePatterns(ctx, baselineDays, topIssuesN)
	if err != nil {
		return fmt.Errorf("learner: baseline aggregate failed: %w", err)
	}
```

(d) แทนบล็อกเรียก gate (บรรทัด ~123-128):

```go
		ok, lowDim, lowVal, gate := strongSignal(agentPatterns, baseline)
		if !ok {
			log.Printf("learner: [%s] skip — weak signal (%s; n=%d weakest=%s avg=%.2f baseline=%.2f)",
				name, gate, agentPatterns.N, lowDim, lowVal, dimValue(baseline, lowDim))
			continue
		}
```

และแก้ log บรรทัด APPLIED (~164) ให้รวม gate:

```go
		log.Printf("learner: [%s] APPLIED new skills (gate=%s weakest=%s avg=%.2f n=%d) — rationale: %s",
			name, gate, lowDim, lowVal, agentPatterns.N, out.Rationale)
```

- [ ] **Step 4: รัน test ทั้ง package ให้ผ่าน**

Run: `go test ./internal/learner/ -v`
Expected: PASS ทุก test (test เดิมที่อ้าง signature เก่า/`lowScoreThreshold` ต้องถูกปรับใน Step 1 แล้ว)

- [ ] **Step 5: Commit**

```bash
git add internal/learner/learner.go internal/learner/learner_test.go
git commit -m "feat(learner): relative regression+frequency gate replaces dead absolute threshold 6.0"
```

---

### Task 4: Wire metadata agent เป็นเจ้าของ title/desc/tags (flag-gated)

**Files:**
- Modify: `internal/orchestrator/orchestrator.go` (struct + `New` + `produceClipWithID`)
- Modify: `cmd/server/main.go` (instantiate + ส่งเข้า `New`)
- Test: `internal/orchestrator/validate_script_test.go` (เพิ่ม test helper ใหม่ในไฟล์ใหม่ `internal/orchestrator/apply_metadata_test.go`)

**Interfaces:**
- Consumes: `agent.NewMetadataAgent(llm *KieLLMClient)`, `(*MetadataAgent).Generate(ctx, topic, script, category, persona string, cfg *models.AgentConfig) (*GeneratedMetadata, error)`, flag `metadata_agent_enabled` ผ่าน `o.settingsRepo.Get`
- Produces: `applyGeneratedMetadata(script *agent.GeneratedScript, md *agent.GeneratedMetadata) bool` (pure helper ใน orchestrator package) — Task อื่นไม่ใช้ต่อ

- [ ] **Step 1: เขียน test ที่ fail**

สร้าง `internal/orchestrator/apply_metadata_test.go`:

```go
package orchestrator

import (
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// applyGeneratedMetadata overwrites script metadata from the metadata agent's
// output; a blank title means "do not apply" (keep script's own metadata).
func TestApplyGeneratedMetadata(t *testing.T) {
	base := func() *agent.GeneratedScript {
		return &agent.GeneratedScript{
			YoutubeTitle:       "ชื่อจาก script",
			YoutubeDescription: "desc จาก script",
			YoutubeTags:        []string{"a", "b"},
		}
	}

	t.Run("full metadata replaces all three fields", func(t *testing.T) {
		s := base()
		ok := applyGeneratedMetadata(s, &agent.GeneratedMetadata{
			YoutubeTitle:       "ชื่อจาก metadata",
			YoutubeDescription: "desc จาก metadata",
			YoutubeTags:        []string{"x"},
		})
		if !ok || s.YoutubeTitle != "ชื่อจาก metadata" || s.YoutubeDescription != "desc จาก metadata" || len(s.YoutubeTags) != 1 {
			t.Errorf("applied=%v script=%+v", ok, s)
		}
	})

	t.Run("blank title → not applied, script untouched", func(t *testing.T) {
		s := base()
		ok := applyGeneratedMetadata(s, &agent.GeneratedMetadata{YoutubeTitle: "  "})
		if ok || s.YoutubeTitle != "ชื่อจาก script" {
			t.Errorf("applied=%v title=%q", ok, s.YoutubeTitle)
		}
	})

	t.Run("blank desc/tags keep script values", func(t *testing.T) {
		s := base()
		ok := applyGeneratedMetadata(s, &agent.GeneratedMetadata{YoutubeTitle: "ชื่อใหม่"})
		if !ok || s.YoutubeDescription != "desc จาก script" || len(s.YoutubeTags) != 2 {
			t.Errorf("applied=%v script=%+v", ok, s)
		}
	})

	t.Run("nil metadata → not applied", func(t *testing.T) {
		s := base()
		if applyGeneratedMetadata(s, nil) {
			t.Error("applied=true for nil metadata")
		}
	})
}
```

- [ ] **Step 2: รัน test ให้เห็นว่า fail**

Run: `go test ./internal/orchestrator/ -run TestApplyGeneratedMetadata -v`
Expected: FAIL (compile error — `applyGeneratedMetadata` ยังไม่มี)

- [ ] **Step 3: เพิ่ม helper + wiring**

(a) ใน `internal/orchestrator/orchestrator.go` เพิ่ม pure helper ถัดจาก `validateScript` (หลังบรรทัด 428):

```go
// applyGeneratedMetadata overwrites the script's YouTube metadata with the
// metadata agent's output. A blank title means the agent produced nothing
// usable — return false and leave the script untouched (fallback). Blank
// desc/tags individually fall back to the script's values.
func applyGeneratedMetadata(script *agent.GeneratedScript, md *agent.GeneratedMetadata) bool {
	if md == nil || strings.TrimSpace(md.YoutubeTitle) == "" {
		return false
	}
	script.YoutubeTitle = md.YoutubeTitle
	if s := strings.TrimSpace(md.YoutubeDescription); s != "" {
		script.YoutubeDescription = s
	}
	if len(md.YoutubeTags) > 0 {
		script.YoutubeTags = md.YoutubeTags
	}
	return true
}
```

(b) เพิ่ม field ใน struct `Orchestrator` (ใกล้บรรทัด 66 ที่มี `imageAgent`):

```go
	metadataAgent   *agent.MetadataAgent
```

(c) เพิ่มพารามิเตอร์ใน `New` (หลัง `ia *agent.ImageAgent` บรรทัด 88): `ma *agent.MetadataAgent,` และในบรรทัด assign (~109): เพิ่ม `metadataAgent: ma,`

(d) ใน `produceClipWithID` — แทรกก่อนบล็อก `o.clipsRepo.UpsertMetadata` (บรรทัด ~528-529):

```go
	// Metadata agent (flag-gated): sole owner of title/desc/tags when enabled.
	// Runs AFTER the critic so it names the final content; overrides both the
	// script's and the critic's metadata. Any failure falls back silently to
	// the script metadata already on `script`.
	if v, _ := o.settingsRepo.Get(ctx, "metadata_agent_enabled"); v == "true" {
		if mdCfg, mErr := o.agentsRepo.GetByName(ctx, "metadata"); mErr == nil && mdCfg.Enabled {
			md, gErr := o.metadataAgent.Generate(ctx, q.Question, narration, q.Category, persona, mdCfg)
			if gErr != nil {
				log.Printf("metadata agent failed (fallback to script metadata): %v", gErr)
			} else if applyGeneratedMetadata(script, md) {
				validateScript(script) // re-enforce length + single brand suffix
			}
		}
	}
```

(e) ใน `cmd/server/main.go`: เพิ่ม `metadataAgent := agent.NewMetadataAgent(llm)` ข้างการสร้าง agent ตัวอื่น (ดู block ที่สร้าง `imageAgent`/`criticAgent` ใกล้บรรทัด ~110-121 — ใช้ LLM client ตัวเดียวกับ `imageAgent`) และเพิ่ม `metadataAgent` ใน call `orchestrator.New(...)` (บรรทัด 127) ตำแหน่งหลัง `imageAgent` ให้ตรงกับ signature ใหม่

- [ ] **Step 4: Build + test**

Run: `go build ./... && go test ./internal/orchestrator/ -v -run TestApplyGeneratedMetadata`
Expected: build ผ่าน, test PASS

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/orchestrator/apply_metadata_test.go cmd/server/main.go
git commit -m "feat(metadata): wire metadata agent as flag-gated sole owner of title/desc/tags"
```

---

### Task 5: Image agent → cover image prompt (flag-gated)

**Files:**
- Modify: `internal/agent/image.go` (เพิ่ม method ใหม่ — ไม่ลบ `GeneratePrompts` เดิม)
- Modify: `internal/orchestrator/orchestrator.go` (`produceClipWithID`)
- Test: `internal/agent/image_test.go` (ไฟล์ใหม่)

**Interfaces:**
- Consumes: `renderTemplate` (template.go), `KieLLMClient.GenerateJSON`, `producer.CoverSceneEnabled()` (audio.go:26), flag `cover_image_agent_enabled`, `imageCfg *models.AgentConfig` ที่ `produceClipWithID` รับอยู่แล้ว
- Produces: `CoverImageTemplateData{QuestionText, Category, HookText string}` และ `(*ImageAgent).GenerateCoverPrompt(ctx context.Context, question, category, hookText string, cfg *models.AgentConfig) (string, error)` — LLM คืน `{"image_prompt": "..."}`

- [ ] **Step 1: เขียน test ที่ fail**

สร้าง `internal/agent/image_test.go`:

```go
package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// Locks the prompt↔struct contract for the cover-prompt output (migration 054
// template): the LLM must emit {"image_prompt": "..."}.
func TestCoverPromptOutputParsesSchema(t *testing.T) {
	raw := `{"image_prompt": "a rejected-status warning dialog floating above a dark desk, main subject in upper half"}`
	var out coverPromptOut
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("cover prompt JSON did not unmarshal: %v", err)
	}
	if out.ImagePrompt == "" {
		t.Error("ImagePrompt is empty")
	}
}

// Guards CoverImageTemplateData field names against the migration-054 template
// vars ({{.QuestionText}}, {{.Category}}, {{.HookText}}).
func TestCoverImageTemplateRendersVars(t *testing.T) {
	tmpl := "q={{.QuestionText}} cat={{.Category}} hook={{.HookText}}"
	out, err := renderTemplate(tmpl, CoverImageTemplateData{
		QuestionText: "Q", Category: "account", HookText: "H",
	})
	if err != nil {
		t.Fatalf("renderTemplate err: %v", err)
	}
	for _, want := range []string{"q=Q", "cat=account", "hook=H"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q: %s", want, out)
		}
	}
}
```

- [ ] **Step 2: รัน test ให้เห็นว่า fail**

Run: `go test ./internal/agent/ -run "TestCoverPrompt|TestCoverImage" -v`
Expected: FAIL (compile error — `coverPromptOut`, `CoverImageTemplateData` ยังไม่มี)

- [ ] **Step 3: เพิ่ม method ใน image.go**

ต่อท้าย `internal/agent/image.go`:

```go
// CoverImageTemplateData fills the migration-054 `image` prompt_template, which
// is dedicated to the clip's cover (frame-0) background.
type CoverImageTemplateData struct {
	QuestionText string
	Category     string
	HookText     string
}

// coverPromptOut is the JSON the cover template asks the LLM to return.
type coverPromptOut struct {
	ImagePrompt string `json:"image_prompt"`
}

// GenerateCoverPrompt produces one English image prompt for the cover scene's
// background. The render pipeline applies theme styling itself (buildScenePrompt),
// so the prompt describes objects/scene only. cfg is the `image` AgentConfig.
func (a *ImageAgent) GenerateCoverPrompt(ctx context.Context, question, category, hookText string, cfg *models.AgentConfig) (string, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, CoverImageTemplateData{
		QuestionText: question,
		Category:     category,
		HookText:     hookText,
	})
	if err != nil {
		return "", fmt.Errorf("render cover image template: %w", err)
	}
	var out coverPromptOut
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &out); err != nil {
		return "", fmt.Errorf("generate cover prompt: %w", err)
	}
	return out.ImagePrompt, nil
}
```

- [ ] **Step 4: Wiring ใน orchestrator**

ใน `produceClipWithID` — แทรกหลังบล็อก critic จบ (หลังบรรทัด ~491 `o.tracker.CompleteStep("critic")` และวงเล็บปิด if) **ก่อน** loop sanitize voice (บรรทัด ~495) เพื่อให้ prompt ใหม่ถูก persist ลง scenes (resume path ใช้ต่อได้):

```go
	// Cover image prompt (flag-gated): the image agent writes a cover-specialized
	// image_prompt for scene 1, whose background IS the frame-0 cover when
	// COVER_SCENE_ENABLED. Replacing the prompt reuses the whole existing render
	// path (kie gen, fallback chain, theme styling). Any failure keeps the scene
	// agent's original prompt.
	if v, _ := o.settingsRepo.Get(ctx, "cover_image_agent_enabled"); v == "true" &&
		producer.CoverSceneEnabled() && len(scenes) > 0 && imageCfg != nil && imageCfg.Enabled {
		cp, cErr := o.imageAgent.GenerateCoverPrompt(ctx, q.Question, q.Category, scenes[0].OnScreenText, imageCfg)
		if cErr != nil {
			log.Printf("cover image prompt failed (fallback to scene prompt): %v", cErr)
		} else if strings.TrimSpace(cp) != "" {
			scenes[0].ImagePrompt = cp
		}
	}
```

- [ ] **Step 5: Build + test ให้ผ่าน**

Run: `go build ./... && go test ./internal/agent/ -run "TestCoverPrompt|TestCoverImage" -v`
Expected: build ผ่าน, test PASS

- [ ] **Step 6: Commit**

```bash
git add internal/agent/image.go internal/agent/image_test.go internal/orchestrator/orchestrator.go
git commit -m "feat(image): repurpose image agent as flag-gated cover image prompt generator"
```

---

### Task 6: Analyzer ชี้ insights ให้ถูก agent

**Files:**
- Modify: `internal/analyzer/analyzer.go:67-83` (user prompt)

**Interfaces:**
- Consumes: `improvementResult` (เดิม — ไม่เปลี่ยน struct)
- Produces: prompt ที่ระบุเป้า insights = question / script / scene / image(บทบาทปก)

- [ ] **Step 1: แก้ prompt**

ใน `internal/analyzer/analyzer.go` แก้ส่วน "Analyze BOTH..." ถึง example JSON (บรรทัด 67-83) เป็น:

```go
Analyze THREE dimensions:
1. STORYTELLING STYLE — openings, hooks (the "Hook" field is the clip's real first line), pacing, tone, length. Which styles earn high view percentiles and "rising" trends?
2. TOPICS — which categories and question angles earn high views/shares on each platform?
3. VISUALS — which cover/first-frame situations (error dialogs, money numbers, rising/falling graphs) correlate with high percentiles?

Requirements for your insights:
- Preserve content diversity: recommend leaning into winning topics for roughly HALF of future clips, never exclusively. Say this explicitly in the question agent's insights.
- Ground every recommendation in the data (cite the pattern: views percentile, shares, or trend).
- Each insight must be under 1000 characters, written in Thai.
- Target the right agent: "question" = topic/angle picks, "script" = narration style, "scene" = per-scene visuals and card layout, "image" = the clip COVER (first frame) background only.

Return JSON only:
{
  "agents": [
    {"agent_name": "question", "new_insights": "...", "reason": "..."},
    {"agent_name": "script", "new_insights": "...", "reason": "..."},
    {"agent_name": "scene", "new_insights": "...", "reason": "..."},
    {"agent_name": "image", "new_insights": "...", "reason": "..."}
  ]
}
```

(ส่วนอื่นของ prompt และโค้ดรอบๆ คงเดิม — loop apply มี allowlist ผ่าน `agentMap` อยู่แล้ว)

- [ ] **Step 2: Build + test**

Run: `go build ./... && go test ./internal/analyzer/ ./internal/orchestrator/`
Expected: ผ่านทั้งหมด (การเปลี่ยนเป็น string literal ใน prompt — ไม่มี test ผูกกับข้อความนี้; ถ้ามี test fail ให้ปรับ expected string ใน test นั้นตามข้อความใหม่)

- [ ] **Step 3: Commit**

```bash
git add internal/analyzer/analyzer.go
git commit -m "fix(analyzer): route visual insights to scene + cover-role image agent"
```

---

### Task 7: Migration 054 — system_prompt/template เฟส 2 + seed flags

**Files:**
- Create: `migrations/054_phase2_agents_rewire.sql`

**Interfaces:**
- Consumes: Task 4/5 อ่าน flag `metadata_agent_enabled` / `cover_image_agent_enabled` และ template vars `{{.QuestionText}}/{{.Category}}/{{.HookText}}`
- Produces: DB text + flags ที่โค้ดเฟส 2 ใช้

- [ ] **Step 1: เขียนไฟล์ migration**

สร้าง `migrations/054_phase2_agents_rewire.sql` เนื้อหาทั้งไฟล์:

```sql
-- 054: Phase-2 rewire — critic may edit `content`, image agent becomes the
-- cover-prompt generator, and the two new feature flags (default OFF).
-- Pairs with the Go changes in the same release; safe to apply before deploy
-- because both flags seed to 'false'.

-- critic: allow editing structured `content` (the on-screen card text), drop
-- dead legacy field text_content from the editable list.
UPDATE agent_configs SET system_prompt = $txt$คุณคือ Content Critic ของ Ads Vance — บรรณาธิการวิดีโอสั้นภาษาไทยสายการเงิน/โฆษณา Meta. รับเนื้อหาที่ทีมสร้างมา (scenes, image_prompt, metadata) แล้วปรับให้ดีขึ้น "เท่าที่จำเป็น" โดยไม่รื้อโครงสร้าง.

เกณฑ์ตรวจ:
- Hook (scene แรก): ต้องดึงให้ดูต่อใน 2-3 วินาทีแรก (ตัวเลขช็อก/คำถามที่โดนความกลัว เช่น โดนแบน เสียเงิน บัญชีปิด).
- content (โครงสร้างการ์ดที่คนดูเห็นจริง): ข้อความใน kicker/title/rows/stat/chips/pill/cta ต้องคม สั้น ตรงประเด็น — นี่คือตัวหนังสือบนจอจริง ให้ความสำคัญสูงสุด.
- ภาษาไทยไหลลื่นแบบพูด ไม่แข็ง ไม่กำกวม.
- แต่ละ scene สื่อสารเรื่องเดียวจบ ชัดเจน.
- ตรงแบรนด์/persona Ads Vance (มืออาชีพ เป็นกันเอง).
- image_prompt: ตรงแบรนด์, ไม่มีตัวหนังสือในรูป, เข้ากับเนื้อ scene.
- metadata: title น่าคลิก ตรง search intent ไม่ clickbait เกินจริง.

ข้อห้ามเด็ดขาด:
- ห้ามเปลี่ยนจำนวน scene, scene_number, duration_seconds, layout, scene_type.
- ปรับได้เฉพาะ voice_text, on_screen_text, image_prompt, emphasis_words, content และ metadata.
- content ที่ปรับต้องคงโครงสร้าง field เดิมตาม layout (hook: kicker+rows, stat: stat+chips ฯลฯ) ห้ามเปลี่ยนชนิดโครงสร้าง และคุมความยาว: cta ≤ 14, pill ≤ 16, statLabel ≤ 28, sub ≤ 50, rows[].t ≤ 36, title ≤ 40, rows ≤ 3 แถว, chips ≤ 2.
- ถ้าเนื้อหาดีอยู่แล้ว ไม่ต้องแก้ คืนของเดิมได้ (changes ว่าง).
ตอบเป็น JSON object เท่านั้น.$txt$
WHERE agent_name = 'critic';

-- image: repurpose from legacy single Q&A chat-bubble image to the cover
-- (frame-0) background prompt. Template vars consumed by
-- agent.CoverImageTemplateData{QuestionText, Category, HookText}.
UPDATE agent_configs SET
  system_prompt = $txt$คุณคือ visual designer ผู้ออกแบบ "ภาพปกคลิป" (เฟรมแรก) ของวิดีโอสั้น 9:16 แบรนด์ Ads Vance

หน้าที่: เขียน image prompt ภาษาอังกฤษสำหรับภาพพื้นหลังของฉากปก ให้คนหยุดนิ้วใน 1 วินาที

กฎเหล็ก:
- ห้ามมีตัวอักษร ตัวเลข โลโก้ มาสคอต ชื่อแบรนด์ หรือ UI text ใดๆ ในภาพ (ระบบ overlay ข้อความ hook เอง)
- อย่าระบุสไตล์ศิลป์หรือสี (ระบบใส่สไตล์ธีมให้เอง)
- วางวัตถุเด่นครึ่งบนของเฟรม เว้นครึ่งล่างว่างให้ข้อความ overlay

ตอบเป็น JSON object เท่านั้น ห้ามมี text อื่นนอก JSON$txt$,
  prompt_template = $txt$สร้าง image prompt 1 ภาพ สำหรับภาพพื้นหลัง "ฉากปก" ของคลิป

หัวข้อคลิป: {{.QuestionText}}
หมวด: {{.Category}}
ข้อความ hook ที่จะ overlay ทับภาพ: {{.HookText}}

แนวภาพที่ได้ผล (เลือกให้ตรงหมวด/เนื้อหา):
- account/ban → หน้าจอแจ้งเตือน สถานะถูกปฏิเสธ โล่เตือน กุญแจล็อก
- payment/billing → บัตรเครดิตถูกตีกลับ หน้าจอชำระเงินผิดพลาด
- campaign/scaling → กราฟพุ่งหรือดิ่งชัดๆ dashboard ตัวเลขเด่น
- pixel/tracking → data flow สัญญาณขาด จุดเชื่อมต่อ
ภาพต้องสื่อ "สถานการณ์/สถานะ" ให้เข้าใจได้ทันทีโดยไม่ต้องอ่านตัวหนังสือ

ตอบเป็น JSON object:
{"image_prompt": "English prompt describing objects/scene only, no text, no logos, main subject in upper half of frame"}$txt$,
  skills = $txt$- ภาพปกต้องสื่อสถานะ/สถานการณ์ใน 1 วินาที: แจ้งเตือน ถูกปฏิเสธ กราฟพุ่ง/ดิ่ง บัตรตีกลับ
- ห้ามตัวหนังสือ/โลโก้/มาสคอตในภาพ — ข้อความ hook มาจาก overlay ของระบบ
- หมุน composition อย่าซ้ำเดิมทุกคลิป: หน้าจอเต็ม / วัตถุลอยเดี่ยว / กราฟ / มุมโต๊ะทำงาน$txt$
WHERE agent_name = 'image';

-- Feature flags (default OFF — flipping is the rollout/rollback lever).
INSERT INTO settings (key, value) VALUES
  ('metadata_agent_enabled', 'false'),
  ('cover_image_agent_enabled', 'false')
ON CONFLICT (key) DO NOTHING;
```

- [ ] **Step 2: ตรวจ SQL**

ตรวจด้วยตา: `$txt$` ครบ 8 คู่ (4 บล็อก), ทุก UPDATE มี WHERE, INSERT มี ON CONFLICT DO NOTHING

- [ ] **Step 3: Commit**

```bash
git add migrations/054_phase2_agents_rewire.sql
git commit -m "feat(agents): migration 054 — critic content-edit prompt, image cover template, phase-2 flags (off)"
```

---

### Task 8: Full verification + สรุปการ deploy

**Files:**
- ไม่มีไฟล์ใหม่ (รัน verification)

**Interfaces:**
- Consumes: ทุก Task ก่อนหน้า
- Produces: ผล build/test เขียว + รายการขั้น deploy

- [ ] **Step 1: Build + test ทั้ง repo**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: ผ่านทั้งหมด ไม่มี test แดง

- [ ] **Step 2: ตรวจ flag-off byte-identity ด้วยตา**

ยืนยันว่าโค้ดใหม่ทุกจุดอยู่หลังเงื่อนไข flag:
- metadata block: `if v, _ := o.settingsRepo.Get(ctx, "metadata_agent_enabled"); v == "true"`
- cover block: `if v, _ := o.settingsRepo.Get(ctx, "cover_image_agent_enabled"); v == "true" && producer.CoverSceneEnabled() ...`
- critic content merge + learner gate ไม่มี flag (ตั้งใจ — เป็น fix ตรง ไม่ใช่ฟีเจอร์ใหม่; rollback = revert PR)

- [ ] **Step 3: Deploy checklist (ทำหลัง merge — ใช้ pre-deploy-checklist skill ของ repo)**

1. Merge + push master → Railway auto-deploy + auto-migrate 053/054
2. Eyeball เฟส 1: produce 1 คลิป (`/orchestrator/produce`, ระบุ count=1) → อ่าน script (ไม่มีนิยามศัพท์พื้นฐาน), CTA ถูกช่องทาง, ดูวิดีโอ
3. เปิด `metadata_agent_enabled=true` → produce 1 คลิป → เช็ค title ใน DB ว่า ≤ 90 รูน, มี " | Ads Vance" อันเดียว, มาจาก metadata agent (ดู log "metadata agent")
4. เปิด `cover_image_agent_enabled=true` → produce 1 คลิป → เช็คเฟรมแรก: ภาพปกสื่อสถานะ + ไม่มีตัวหนังสือ baked-in
5. Monitor 1 สัปดาห์: คะแนน critic ควรเริ่มกระจาย (ไม่แบน ~8), `skill_revisions` ควรมีแถวเมื่อเกิด regression จริง, `agent_prompt_history` มี insights เข้า scene/image

---

## Self-Review (ผลตรวจแผนกับ spec)

- **Spec coverage:** §เฟส1 ทั้ง 5 ข้อ → Task 1; §2.1 → Task 4; §2.2 → Task 5+7; §2.3 → Task 2+7 (sanitize ทำที่ render-time `buildSceneContent` ซึ่งมีอยู่แล้ว — spec ขอ "รัน sanitize ชุดเดียวกันซ้ำ" ซึ่งเกิดจริงตอน render ทุกครั้ง จึงไม่เขียนซ้ำใน critic ตาม DRY; layout-mismatch ป้องกันด้วย layout immutable + hero fallback เดิม); §2.4 → Task 3; §2.5 → Task 6 (ยกเว้น title 70 — เบี่ยงโดยแจ้งแล้ว ดู Global Constraints)
- **Deviation จาก spec (แจ้ง user แล้วในแชท):** ไม่ลด title maxLen 90→70 เพราะ regress การแก้ตัดชื่อไทยกลางคำที่ตั้งใจไว้ (validate_script_test.go:94-109)
- **Type consistency:** `strongSignal(p, base)` ใช้ตรงกันใน Task 3 ทุก step; `applyGeneratedMetadata` นิยาม+ใช้ใน Task 4; `CoverImageTemplateData`/`GenerateCoverPrompt` นิยาม Task 5 ใช้ template vars ตรงกับ migration Task 7
- **Placeholder scan:** ไม่มี TBD/TODO; ทุก code step มีโค้ดเต็ม
