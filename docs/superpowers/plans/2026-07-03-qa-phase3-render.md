# QA Reliability Phase 3 (render defects) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`).

**Goal:** Reduce the genuine render defects a minority of clips hit: (3c) Thai text corruption from mid-cluster caption splitting; (3a) text overflow / CTA overlap from missing CSS wrap bounds; (3b) unbounded on-screen text length.

**Architecture:** Three independent changes — `internal/producer/captions.go` (pure Go), `internal/producer/templates/layout_multi_scene.html.tmpl` (CSS), and generation length limits (`internal/agent/scene_content.go` helper + `internal/producer/scene_adapter.go` application + a new prompt migration). Builds on the visual-qa-reliability spec.

**Tech Stack:** Go, HTML/CSS template, SQL migration. Tests: `go test ./internal/producer/ ./internal/agent/`.

## Global Constraints

- `go build`/`go test`/`go vet` FAIL in the default sandbox ("operation not permitted" on go-build cache) — run with sandbox DISABLED.
- **Appearance risk:** these change how clips render. Per the render-audit, 3c and the 3a title/stat-label/row rules only activate on already-broken content (safe); `.cta`/`.pill` `max-width` and code truncation need a human eyeball on a real render. Do NOT touch `data-layout-allow-overflow` (line 153/182 — consumed by the external hyperframes CLI, unverifiable here). Do NOT revert the `stat` layout width narrowing (line 57 — deliberate design).
- Grounding source caps (chars): cta≤14, statLabel≤28, pill≤16, sub≤50, rows≤36.

---

### Task 1: Combining-mark-safe caption split (3c)

**Files:**
- Modify: `internal/producer/captions.go` — add `safeCut`, use it in the hard-split loop; add `"unicode"` import.
- Test: `internal/producer/captions_test.go`

**Interfaces:**
- Produces: `func safeCut(tr []rune, max int) int` — largest index ≤ max whose rune is not a Unicode Mn (combining mark), so a hard split never orphans a Thai vowel/tone mark.

- [ ] **Step 1: Write failing test** — append to `internal/producer/captions_test.go`:

```go
func TestSafeCut_doesNotSplitCombiningMark(t *testing.T) {
	// "ที่" = ท + ◌ี(U+0E35 Mn) ; a naive cut at an index landing on the mark
	// would orphan it. Build a >max run ending so index `max` is a combining mark.
	base := []rune("กกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกกก") // 42 base consonants
	tr := append(base, 'ี')                                        // index 42 = combining mark
	cut := safeCut(tr, 42)
	if unicodeIsMn(tr[cut]) {
		t.Errorf("safeCut returned index %d which is a combining mark", cut)
	}
	if cut < 1 {
		t.Errorf("safeCut backed off too far: %d", cut)
	}
}

func unicodeIsMn(r rune) bool { return unicode.Is(unicode.Mn, r) }
```
(Add `"unicode"` to the test file's imports if not present.)

- [ ] **Step 2: Run to verify fail**

Run (sandbox disabled): `go test ./internal/producer/ -run TestSafeCut -v`
Expected: FAIL — `undefined: safeCut`.

- [ ] **Step 3: Implement** — in `internal/producer/captions.go`, add `"unicode"` to imports, then add:

```go
// safeCut returns the largest index <= max where tr[idx] is not a combining mark
// (Unicode Mn), so a hard split never separates a Thai vowel/tone mark from its
// base consonant (which would render as a floating mark / "corrupted" text).
func safeCut(tr []rune, max int) int {
	cut := max
	for cut > 1 && unicode.Is(unicode.Mn, tr[cut]) {
		cut--
	}
	return cut
}
```
Then replace the hard-split loop (currently `for len(tr) > captionMaxRunes { phrases = append(phrases, string(tr[:captionMaxRunes])); tr = tr[captionMaxRunes:] }`) with:
```go
			for len(tr) > captionMaxRunes {
				cut := safeCut(tr, captionMaxRunes)
				phrases = append(phrases, string(tr[:cut]))
				tr = tr[cut:]
			}
```

- [ ] **Step 4: Run to verify pass**

Run (sandbox disabled): `go build ./... && go test ./internal/producer/ -run "TestSafeCut|Caption" -v`
Expected: build OK; new test + existing caption tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/producer/captions.go internal/producer/captions_test.go
git commit -m "fix(render): combining-mark-safe caption hard-split (no orphaned Thai marks)"
```

---

### Task 2: CSS overflow bounds (3a)

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl`

**Interfaces:** none (CSS only).

- [ ] **Step 1: Add wrap rules.** In the `<style>` block, add these rules (place them right after the existing `.cap-word` rule near line 141, so they live beside the other wrap rules):

```css
    h1.title,.stat-label,.row .rt{overflow-wrap:anywhere;word-break:break-word}
    .cta,.pill{max-width:100%;overflow-wrap:anywhere}
```

Do NOT modify `.scene[data-layout="stat"] .scene-content` (line 57), and do NOT touch `data-layout-allow-overflow` (lines 153/182).

- [ ] **Step 2: Verify the template still parses / renders in tests**

Run (sandbox disabled): `go test ./internal/producer/ -v 2>&1 | grep -iE "FAIL|ok|render|composition" | head`
Expected: no FAIL; the composition/render tests that execute this template still pass (they assert injected HTML/vars — a CSS-only addition must not break them).

- [ ] **Step 3: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl
git commit -m "fix(render): bound title/stat/row/cta/pill text so long Thai wraps instead of overflowing"
```

> **Eyeball note (for the controller/user, not a code step):** the `.cta`/`.pill` `max-width:100%` only changes rendering when a CTA label is long enough to wrap; a wrapped pill (border-radius:999px) may look unusual. Confirm on the next real render.

---

### Task 3: On-screen text length caps (3b)

**Files:**
- Modify: `internal/agent/scene_content.go` — add `TruncateRunes`.
- Modify: `internal/producer/scene_adapter.go` — apply caps to the plain (non-highlighted) fields.
- Create: `migrations/048_scene_content_length_limits.sql`
- Test: `internal/agent/scene_content_test.go` (create if absent)

**Interfaces:**
- Produces: `func TruncateRunes(s string, max int) string` — returns s unchanged if ≤ max runes; else cuts at a combining-mark-safe boundary and trims a trailing space. Never cuts mid-cluster.

- [ ] **Step 1: Write failing test** — in `internal/agent/scene_content_test.go` (create if needed, `package agent`):

```go
package agent

import "testing"

func TestTruncateRunes_underLimitUnchanged(t *testing.T) {
	if got := TruncateRunes("สั้น", 14); got != "สั้น" {
		t.Errorf("want unchanged, got %q", got)
	}
}

func TestTruncateRunes_cutsToLimit(t *testing.T) {
	in := "กกกกกกกกกกกกกกกกกกกก" // 20 runes
	got := TruncateRunes(in, 14)
	if r := []rune(got); len(r) > 14 {
		t.Errorf("want <=14 runes, got %d (%q)", len(r), got)
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run (sandbox disabled): `go test ./internal/agent/ -run TestTruncateRunes -v`
Expected: FAIL — `undefined: TruncateRunes`.

- [ ] **Step 3: Implement `TruncateRunes`** in `internal/agent/scene_content.go` (next to `StripEmoji`, reusing the `unicode` import already present):

```go
// TruncateRunes caps s at max runes, backing off any trailing combining mark so a
// Thai vowel/tone mark is never orphaned, and trimming a trailing space. Returns s
// unchanged when already within budget. Used as a safety net for on-screen text
// fields whose generation prompt caps are advisory.
func TruncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	cut := max
	for cut > 1 && unicode.Is(unicode.Mn, r[cut]) {
		cut--
	}
	return strings.TrimRight(string(r[:cut]), " ")
}
```

- [ ] **Step 4: Apply caps to plain fields** in `internal/producer/scene_adapter.go`. Find the cleaning block (lines 139-144). Leave `c.Title` (line 141) UNTOUCHED — it may contain `<span class="acc">` markup and is highlighted; truncating it could cut a tag. Wrap only the plain fields:

Change:
```go
	c.Stat, c.Unit, c.StatLabel = clean(raw.Stat), clean(raw.Unit), clean(raw.StatLabel)
	c.Num, c.Of, c.Pill = clean(raw.Num), clean(raw.Of), clean(raw.Pill)
	c.CTA, c.Brand = clean(raw.CTA), clean(raw.Brand)
```
to:
```go
	c.Stat, c.Unit = clean(raw.Stat), clean(raw.Unit)
	c.StatLabel = agent.TruncateRunes(clean(raw.StatLabel), 28)
	c.Num, c.Of = clean(raw.Num), clean(raw.Of)
	c.Pill = agent.TruncateRunes(clean(raw.Pill), 16)
	c.CTA = agent.TruncateRunes(clean(raw.CTA), 14)
	c.Brand = clean(raw.Brand)
```
Also cap `c.Sub` — change line 140 `c.Kicker, c.Sub = clean(raw.Kicker), clean(raw.Sub)` to:
```go
	c.Kicker = clean(raw.Kicker)
	c.Sub = agent.TruncateRunes(clean(raw.Sub), 50)
```
And the row text — change line 146 `if t := clean(r.T); t != "" {` to `if t := agent.TruncateRunes(clean(r.T), 36); t != "" {`.

- [ ] **Step 5: Create the prompt migration** `migrations/048_scene_content_length_limits.sql`:

```sql
-- Append advisory per-layout length caps to the scene agent prompt so the LLM
-- keeps on-screen text within the layout boxes. The code-side TruncateRunes is the
-- enforcement net; this reduces how often it has to fire.
UPDATE agent_configs
SET prompt_template = prompt_template || E'\n\nกฎความยาวข้อความบนจอ (อย่าเกิน เพื่อไม่ให้ล้นกรอบ): cta/ปุ่ม ≤ 14 ตัวอักษร, pill ≤ 16, statLabel ≤ 28, sub ≤ 50, แต่ละแถว(rows[].t) ≤ 36, title ≤ 40. เขียนให้กระชับพอดีกรอบ.'
WHERE agent_name = 'scene'
  AND prompt_template NOT LIKE '%กฎความยาวข้อความบนจอ%';
```

- [ ] **Step 6: Run tests + build**

Run (sandbox disabled): `go build ./... && go vet ./internal/agent/ ./internal/producer/ && go test ./internal/agent/ ./internal/producer/ -v 2>&1 | grep -iE "FAIL|^ok"`
Expected: build OK; vet clean; TruncateRunes tests + existing suites PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/agent/scene_content.go internal/producer/scene_adapter.go internal/agent/scene_content_test.go migrations/048_scene_content_length_limits.sql
git commit -m "fix(render): cap on-screen text length (TruncateRunes + prompt caps, migration 048)"
```

> **Eyeball note:** truncation only fires on over-cap fields; confirm truncated Thai still reads coherently on the next real render.

---

## Self-review (controller, after all tasks)
- 3c: safeCut used in the hard-split loop; `unicode` imported; existing caption tests still green.
- 3a: only the 5 selectors changed; stat width + data-layout-allow-overflow untouched.
- 3b: Title NOT code-truncated; migration idempotent (`NOT LIKE` guard); TruncateRunes never cuts mid-cluster.

## Out of scope
- `emphasisInPhrase` substring matching (no proven bug — grounding). JS text-fit (CSS suffices). Reverting stat width.
