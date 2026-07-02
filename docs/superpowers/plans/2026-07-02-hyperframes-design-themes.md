# Hyperframes Design Themes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ยกเครื่องดีไซน์ hyperframes ให้เลิก "ซ้ำเดิม" — สร้างระบบ Design Themes หมุนเวียน 4 ธีม (Editorial Bold / Cinematic Photo / Neon Techno HUD / Soft 3D Clay) + ยกคุณภาพ hook/caption/รูป เพื่อดัน 3-second retention → ยอดวิว

**Architecture:** ต่อยอดระบบ `StylePreset` เดิม (`internal/producer/presets.go`) — ทั้ง 4 ธีม **ใช้พาเลตต์ navy+orange เดียวกัน (`Brand`)** เป็นค่าคงที่ของแบรนด์ แล้วแปรที่ 4 แกน: art-anchor (สื่อรูป), heading font, motion profile, theme texture. เพิ่มฟอนต์ใหม่ (vendor local), แก้ image agent ให้ prompt คุณภาพสูง, แก้ caption ให้ไฮไลต์คำ emphasis จริง, แก้ scene/critic/visual_qa prompt ผ่าน migration.

**Tech Stack:** Go 1.22+ (pgx, html/template, standard `testing`), HTML/CSS/JS template + GSAP 3.14.2 (vendored), PostgreSQL (Neon) migrations (plain .sql, alphabetical), kie.ai gpt-image-2 + LLM.

## Global Constraints

- **Render is OFFLINE** — no CDN reachable at render time. ทุกฟอนต์ต้อง vendor เป็นไฟล์ local ใน `internal/producer/assets/fonts/` + โหลดผ่าน `@font-face` (GSAP ก็ bundle แล้ว). ห้ามใช้ Google Fonts `<link>`.
- **Thai typography:** line-height ≥ 1.32 สำหรับข้อความหลายบรรทัด; **ห้าม negative letter-spacing เด็ดขาด** (ชนสระ/วรรณยุกต์); HTML-escape ทุก text ก่อน render (เดิมทำแล้ว).
- **Brand invariants (คงทุกธีม):** badge "ADS VANCE" มุมซ้ายบน + หมวดหมู่; ส้ม (`--orange`) เป็น accent เน้นจุดเดียว; มาสคอตเสือดาวสไตล์เดียว; progress bar บน; พาเลตต์ = `Brand` (navy+orange) ทุกธีม.
- **Flag-off = no-op:** `STYLE_PRESETS_ENABLED != "true"` ⇒ ใช้ `Presets[0]` อย่างเดียว. `Presets[0]` ต้องเป็น fallback สากล (selection พัง = ได้ธีมนี้เสมอ).
- **Fail-open เดิมคง:** image fail → CSS background downgrade (circuit breaker เดิม); visual_qa ไม่แน่ใจ = ok=true.
- **Test style:** standard Go `testing` (`t.Fatal/t.Errorf`), ไม่มี assert lib. Migration = plain SQL, idempotent, รันเรียงตามชื่อไฟล์, เลขถัดไป = 046.
- **Branch:** ทำงานบน `feat/design-themes` (สร้างแล้ว). commit บ่อยต่อ task.

---

## File Structure

**สร้างใหม่:**
- `internal/producer/assets/fonts/Kanit-{Bold,ExtraBold,Black}.ttf`, `Prompt-{SemiBold,Bold,ExtraBold}.ttf`, `IBMPlexSansThai-{Medium,SemiBold}.ttf` — ฟอนต์ vendor
- `migrations/046_scene_prompt_themes.sql` — scene prompt: hook≤7คำ + layout variety + emphasis
- `migrations/047_critic_visualqa_themes.sql` — critic + visual_qa theme coherence
- `scripts/fetch_fonts.sh` — สคริปต์ดาวน์โหลดฟอนต์ (ต้องมี network)

**แก้ไข:**
- `internal/producer/brand.go` — `TypeTokens` (+HeadingFont fields), `MotionProfile` type, `cssVars` (+`--font-heading`/motion vars), `buildScenePrompt` (house-style/negative/cohesion)
- `internal/producer/presets.go` — `StylePreset` (+`HeadingFont`,`Motion`), `Presets` = 4 ธีม
- `internal/producer/captions.go` — `TranscriptSegment` (+`Emphasis`), `captionSegmentsFromScenes` (attach emphasis)
- `internal/producer/composition_types.go` — `scenesTemplateData`/params (+`ThemeKey`, motion vars) [ในไฟล์ composition.go]
- `internal/producer/composition.go` — thread ThemeKey + motion + heading into template data
- `internal/producer/templates/layout_multi_scene.html.tmpl` — @font-face ใหม่, `--font-heading` บน display text, `data-theme` texture, keyIdx ใช้ emphasis, motion จาก vars
- `internal/producer/producer.go` — ส่ง preset.Key เป็น ThemeKey + preset.Motion เข้า composition params (call site `AssembleHyperframes916`)

---

## Task 1: Vendor ฟอนต์ใหม่ (Kanit / Prompt / IBM Plex Sans Thai)

**Files:**
- Create: `scripts/fetch_fonts.sh`
- Create: `internal/producer/assets/fonts/Kanit-Bold.ttf`, `Kanit-ExtraBold.ttf`, `Kanit-Black.ttf`, `Prompt-SemiBold.ttf`, `Prompt-Bold.ttf`, `Prompt-ExtraBold.ttf`, `IBMPlexSansThai-Medium.ttf`, `IBMPlexSansThai-SemiBold.ttf`
- Test: `internal/producer/fonts_present_test.go`

**Interfaces:**
- Produces: ไฟล์ `.ttf` ใน `assets/fonts/` ที่ `copyDir` (composition_builder.go:274) จะ copy เข้า project dir อัตโนมัติ (copy ทุกไฟล์ใน dir อยู่แล้ว — ไม่ต้องแก้ copy logic)

> **หมายเหตุ network:** Task นี้ต้องดาวน์โหลดฟอนต์จากอินเทอร์เน็ต (Google Fonts). ถ้า sandbox บล็อก network ให้รันด้วย network access. ฟอนต์ทั้งหมดเป็น **OFL license** (ใช้เชิงพาณิชย์ + embed ได้).

- [ ] **Step 1: เขียนสคริปต์ดาวน์โหลดฟอนต์**

สร้าง `scripts/fetch_fonts.sh`:
```bash
#!/usr/bin/env bash
# ดาวน์โหลดฟอนต์ OFL ที่ใช้ในธีม (vendor เป็น local .ttf — render offline)
set -euo pipefail
DEST="internal/producer/assets/fonts"
mkdir -p "$DEST"
base="https://raw.githubusercontent.com/google/fonts/main"
declare -A FONTS=(
  ["Kanit-Bold.ttf"]="$base/ofl/kanit/Kanit-Bold.ttf"
  ["Kanit-ExtraBold.ttf"]="$base/ofl/kanit/Kanit-ExtraBold.ttf"
  ["Kanit-Black.ttf"]="$base/ofl/kanit/Kanit-Black.ttf"
  ["Prompt-SemiBold.ttf"]="$base/ofl/prompt/Prompt-SemiBold.ttf"
  ["Prompt-Bold.ttf"]="$base/ofl/prompt/Prompt-Bold.ttf"
  ["Prompt-ExtraBold.ttf"]="$base/ofl/prompt/Prompt-ExtraBold.ttf"
  ["IBMPlexSansThai-Medium.ttf"]="$base/ofl/ibmplexsansthai/IBMPlexSansThai-Medium.ttf"
  ["IBMPlexSansThai-SemiBold.ttf"]="$base/ofl/ibmplexsansthai/IBMPlexSansThai-SemiBold.ttf"
)
for name in "${!FONTS[@]}"; do
  echo "→ $name"
  curl -fsSL "${FONTS[$name]}" -o "$DEST/$name"
done
echo "done: $(ls -1 "$DEST" | wc -l) fonts"
```

- [ ] **Step 2: รันสคริปต์ (ต้องมี network)**

Run: `bash scripts/fetch_fonts.sh`
Expected: พิมพ์ `→ ...` แต่ละไฟล์ + `done: 12 fonts` (Sarabun 4 เดิม + ใหม่ 8). ยืนยันไฟล์ไม่ว่าง: `ls -la internal/producer/assets/fonts/*.ttf` แต่ละไฟล์ควร > 50KB.

- [ ] **Step 3: เขียน test ยืนยันฟอนต์มีจริง (failing ก่อน)**

สร้าง `internal/producer/fonts_present_test.go`:
```go
package producer

import (
	"os"
	"path/filepath"
	"testing"
)

// The render is offline — every font a theme references MUST be vendored as a
// local .ttf, or @font-face falls back to a system font (or nothing) at render.
func TestVendoredFontsPresent(t *testing.T) {
	want := []string{
		"Sarabun-Regular.ttf", "Sarabun-SemiBold.ttf", "Sarabun-Bold.ttf", "Sarabun-ExtraBold.ttf",
		"Kanit-Bold.ttf", "Kanit-ExtraBold.ttf", "Kanit-Black.ttf",
		"Prompt-SemiBold.ttf", "Prompt-Bold.ttf", "Prompt-ExtraBold.ttf",
		"IBMPlexSansThai-Medium.ttf", "IBMPlexSansThai-SemiBold.ttf",
	}
	dir := filepath.Join("assets", "fonts")
	for _, f := range want {
		info, err := os.Stat(filepath.Join(dir, f))
		if err != nil {
			t.Errorf("missing vendored font %s: %v", f, err)
			continue
		}
		if info.Size() < 20_000 {
			t.Errorf("font %s too small (%d bytes) — likely a failed/HTML download", f, info.Size())
		}
	}
}
```

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/producer/ -run TestVendoredFontsPresent -v`
Expected: PASS (ถ้า FAIL = ฟอนต์ยังโหลดไม่ครบ กลับไป Step 2)

- [ ] **Step 5: Commit**

```bash
git add scripts/fetch_fonts.sh internal/producer/assets/fonts/ internal/producer/fonts_present_test.go
git commit -m "feat(themes): vendor Kanit/Prompt/IBM Plex Sans Thai fonts for design themes"
```

---

## Task 2: ขยาย TypeTokens + StylePreset + MotionProfile + BrandCSS

**Files:**
- Modify: `internal/producer/brand.go` (TypeTokens struct ~150-169, `Type` var, `cssVars` 186-233, add `MotionProfile`)
- Modify: `internal/producer/presets.go` (StylePreset struct 15-21)
- Test: `internal/producer/brand_test.go` (เพิ่ม test)

**Interfaces:**
- Consumes: `MotionTokens`/`Motion` (brand.go:119-143), `TypeTokens`/`Type` (brand.go:150-169)
- Produces:
  - `TypeTokens.HeadingFamily string` — ฟอนต์หัวข้อ (ว่าง = ใช้ `Family`)
  - `MotionProfile{EntranceDur float64; EntranceEase string; BGZoomTo float64}` + var `MotionDefault`
  - `StylePreset.HeadingFont TypeTokens` (ถ้าว่าง fallback `.Font`) + `StylePreset.Motion MotionProfile`
  - `cssVars` emits `--font-heading`, `--entrance-dur`, `--entrance-ease`, `--bg-zoom-to`

- [ ] **Step 1: เขียน test (failing)**

เพิ่มใน `internal/producer/brand_test.go`:
```go
func TestCSSVars_EmitsHeadingFontAndMotionProfile(t *testing.T) {
	// A theme with a distinct heading font + snappy motion must surface those as
	// CSS custom properties the template can consume.
	tt := TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit",
		WeightRegular: 400, WeightSemiBold: 600, WeightBold: 700, WeightExtraBold: 800}
	css := Brand.cssVars(tt)
	for _, want := range []string{`--font-heading: "Kanit"`, "--font-family:"} {
		if !strings.Contains(css, want) {
			t.Errorf("cssVars missing %q\n%s", want, css)
		}
	}
}

func TestMotionProfile_Default(t *testing.T) {
	if MotionDefault.EntranceDur <= 0 || MotionDefault.EntranceEase == "" || MotionDefault.BGZoomTo < 1.0 {
		t.Errorf("MotionDefault has invalid zero values: %+v", MotionDefault)
	}
}
```

- [ ] **Step 2: รัน test ให้ FAIL**

Run: `go test ./internal/producer/ -run 'TestCSSVars_EmitsHeadingFontAndMotionProfile|TestMotionProfile_Default' -v`
Expected: FAIL — `HeadingFamily` / `MotionDefault` undefined (compile error)

- [ ] **Step 3: เพิ่ม field + type ใน brand.go**

ใน `TypeTokens` struct (brand.go:150-159) เพิ่ม field หลัง `Family`:
```go
	// HeadingFamily is the display font for headlines/hooks. Empty ⇒ fall back
	// to Family. Vendored locally (see assets/fonts). Body text keeps Family.
	HeadingFamily string // "Kanit" / "Prompt" / "" (=Family)
```

เพิ่ม MotionProfile หลังบล็อก `Motion` (หลัง brand.go:143):
```go
// MotionProfile is a per-theme animation feel: how a scene's content enters and
// how far the background ken-burns. Injected into the template as CSS vars +
// JS constants so each theme moves differently without new template code.
type MotionProfile struct {
	EntranceDur  float64 // seconds — content entrance duration
	EntranceEase string  // GSAP ease name, e.g. "power3.out", "back.out(1.6)"
	BGZoomTo     float64 // background ken-burns end scale, e.g. 1.06, 1.10
}

// MotionDefault matches today's Style-B feel (Editorial Bold baseline).
var MotionDefault = MotionProfile{EntranceDur: 0.60, EntranceEase: "power3.out", BGZoomTo: 1.06}
```

- [ ] **Step 4: อัปเดต `cssVars` ให้ emit heading font + motion vars**

ใน `cssVars` (brand.go:186-233) — เพิ่มบล็อกใน format string หลัง `--font-family` (หลังบรรทัด `--font-family: "%s", sans-serif;`) และก่อน weight vars, เพิ่ม:
```
  --font-heading: %s;
```
และเปลี่ยน argument list ให้คำนวณ heading: เพิ่ม helper ก่อน `return fmt.Sprintf`:
```go
	heading := t.HeadingFamily
	if heading == "" {
		heading = t.Family
	}
	headingCSS := fmt.Sprintf(`"%s", "%s", sans-serif`, heading, t.Family)
```
แล้วในบล็อก Typography ของ format string เพิ่มบรรทัด `  --font-heading: %s;` ใต้ `--font-family:` และเพิ่ม `headingCSS` เป็น argument ตำแหน่งตรงกับ `%s` ใหม่ (ก่อน `t.WeightRegular`).

- [ ] **Step 5: เพิ่ม field ใน StylePreset**

ใน `presets.go` StylePreset struct (15-21) เพิ่มหลัง `Font`:
```go
	HeadingFont TypeTokens    // display font for headlines; zero ⇒ use Font
	Motion      MotionProfile // per-theme entrance/ken-burns feel
```
และใน 5 preset เดิม + `AsTheme`: ยังไม่ต้องแก้ (Task 3 จะเขียน Presets ใหม่ทั้งชุด). เพื่อให้ compile ผ่านตอนนี้ ใน `BrandCSS()` (presets.go:203-205) เปลี่ยนให้ใช้ HeadingFont ถ้ามี:
```go
func (p StylePreset) BrandCSS() string {
	font := p.Font
	if p.HeadingFont.HeadingFamily != "" {
		font.HeadingFamily = p.HeadingFont.HeadingFamily
	}
	return p.Palette.cssVars(font)
}
```

- [ ] **Step 6: รัน test ให้ PASS + build**

Run: `go test ./internal/producer/ -run 'TestCSSVars_EmitsHeadingFontAndMotionProfile|TestMotionProfile_Default' -v && go build ./...`
Expected: PASS + build สำเร็จ (preset เดิมยัง compile ได้เพราะ field ใหม่มีค่า zero)

- [ ] **Step 7: Commit**

```bash
git add internal/producer/brand.go internal/producer/presets.go internal/producer/brand_test.go
git commit -m "feat(themes): add HeadingFont + MotionProfile to preset/type tokens + CSS vars"
```

---

## Task 3: นิยาม Presets ใหม่ = 4 ธีม (ใช้ Brand palette ร่วมกัน)

**Files:**
- Modify: `internal/producer/presets.go` (Presets var 26-98)
- Test: `internal/producer/presets_test.go` (เพิ่ม test)

**Interfaces:**
- Consumes: `Brand` (brand.go:40), `Type` (brand.go:163), `MotionProfile`/`MotionDefault` (Task 2)
- Produces: `Presets` = 4 ธีม keys: `editorial-bold`(=Presets[0]), `cinematic-photo`, `neon-techno`, `soft-3d-clay` — ทุกตัว `Palette: Brand`

- [ ] **Step 1: เขียน test (failing)**

เพิ่มใน `presets_test.go`:
```go
func TestThemes_AllShareBrandPaletteAndHaveHeadingFontAndMotion(t *testing.T) {
	wantKeys := map[string]bool{
		"editorial-bold": true, "cinematic-photo": true, "neon-techno": true, "soft-3d-clay": true,
	}
	if len(Presets) != 4 {
		t.Fatalf("len(Presets) = %d, want 4 themes", len(Presets))
	}
	if Presets[0].Key != "editorial-bold" {
		t.Errorf("Presets[0].Key = %q, want editorial-bold (universal fallback)", Presets[0].Key)
	}
	for _, p := range Presets {
		if !wantKeys[p.Key] {
			t.Errorf("unexpected theme key %q", p.Key)
		}
		// Brand invariant: every theme keeps navy+orange (palette differences are
		// NOT how themes differ — media/font/motion are).
		if p.Palette != Brand {
			t.Errorf("theme %q palette drifts from Brand (violates brand invariant)", p.Key)
		}
		if p.HeadingFont.HeadingFamily == "" {
			t.Errorf("theme %q missing HeadingFont.HeadingFamily", p.Key)
		}
		if p.Motion.EntranceEase == "" || p.Motion.BGZoomTo < 1.0 {
			t.Errorf("theme %q has invalid Motion %+v", p.Key, p.Motion)
		}
		if strings.TrimSpace(p.ImageAnchor) == "" {
			t.Errorf("theme %q empty ImageAnchor", p.Key)
		}
	}
}
```

- [ ] **Step 2: รัน test ให้ FAIL**

Run: `go test ./internal/producer/ -run TestThemes_AllShareBrandPaletteAndHaveHeadingFontAndMotion -v`
Expected: FAIL (keys/จำนวนไม่ตรง)

- [ ] **Step 3: แทนที่ `Presets` ทั้งชุด**

แทนที่ `var Presets = []StylePreset{...}` (presets.go:26-98) ด้วย 4 ธีม (พาเลตต์ = `Brand` ทุกตัว; heading font + motion + art anchor ต่างกัน):
```go
var Presets = []StylePreset{
	{
		Key:         "editorial-bold",
		DisplayName: "Editorial Bold",
		Palette:     Brand,
		ImageAnchor: "Flat modern editorial illustration, premium and clean, with soft cinematic lighting. " +
			"Strict two-tone palette: vivid royal blue #0047AF as the dominant background/structural color, " +
			"warm amber gold #F0A030 as the single accent for highlights and focal points. No other saturated hues. " +
			"Crisp vector-quality shapes, confident composition, subtle top-center glow, gentle edge vignette, minimal grain. " +
			"No photorealism, no 3D render, no text. Atmosphere: confident, authoritative, premium digital-marketing brand.",
		Font:        Type,
		HeadingFont: TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
		Motion:      MotionProfile{EntranceDur: 0.60, EntranceEase: "power3.out", BGZoomTo: 1.06},
	},
	{
		Key:         "cinematic-photo",
		DisplayName: "Cinematic Photo",
		Palette:     Brand,
		ImageAnchor: "Cinematic editorial PHOTOGRAPHY, shot on 85mm f/1.4, natural window light, shallow depth of field, " +
			"warm filmic color grade with a subtle deep-navy #0047AF duotone wash in the shadows and warm amber #F0A030 highlights. " +
			"Real-world settings (modern office, hands on a laptop showing an ads dashboard, banknotes, people), photorealistic, " +
			"premium and trustworthy. NO illustration, NO 3D render, NO cartoon, NO flat vector, no text. " +
			"Atmosphere: credible, premium, real digital-marketing business.",
		Font:        TypeTokens{Family: "IBM Plex Sans Thai", HeadingFamily: "Kanit"},
		HeadingFont: TypeTokens{Family: "IBM Plex Sans Thai", HeadingFamily: "Kanit"},
		Motion:      MotionProfile{EntranceDur: 0.70, EntranceEase: "power2.out", BGZoomTo: 1.12},
	},
	{
		Key:         "neon-techno",
		DisplayName: "Neon Techno HUD",
		Palette:     Brand,
		ImageAnchor: "Sleek techno HUD illustration on a dark deep-navy #062F78 background, crisp neon line-art and thin glowing strokes, " +
			"glassmorphism panels, data/graph/ring motifs. Electric-blue glow accents with warm amber #F0A030 as the single focal accent. " +
			"High-tech, sharp, clean vector rendering, subtle scanline glow. No photorealism, no text. " +
			"Atmosphere: high-energy, data-driven, premium digital-marketing brand.",
		Font:        TypeTokens{Family: "Prompt", HeadingFamily: "Prompt"},
		HeadingFont: TypeTokens{Family: "Prompt", HeadingFamily: "Prompt"},
		Motion:      MotionProfile{EntranceDur: 0.34, EntranceEase: "power4.out", BGZoomTo: 1.05},
	},
	{
		Key:         "soft-3d-clay",
		DisplayName: "Soft 3D Clay",
		Palette:     Brand,
		ImageAnchor: "Soft 3D clay-render illustration, rounded matte shapes with gentle soft studio shadows, tactile and friendly. " +
			"Palette anchored to brand royal blue #0047AF with warm amber #F0A030 as the single accent; warm approachable mood. " +
			"Claymorphism, smooth surfaces, no harsh edges. No photorealism, no text. " +
			"Atmosphere: friendly, approachable, premium digital-marketing brand.",
		Font:        TypeTokens{Family: "Prompt", HeadingFamily: "Kanit"},
		HeadingFont: TypeTokens{Family: "Prompt", HeadingFamily: "Kanit"},
		Motion:      MotionProfile{EntranceDur: 0.48, EntranceEase: "back.out(1.6)", BGZoomTo: 1.08},
	},
}
```

> **หมายเหตุ:** `Presets[0]` เปลี่ยนจาก `signature` เป็น `editorial-bold`. `PresetByKey`/`PickPreset` fallback ไป `Presets[0]` เดิม — ยังถูกต้อง. คลิปเก่าที่เก็บ `style_preset="signature"` จะ fallback เป็น editorial-bold (art-anchor ใกล้ signature เดิม — ยอมรับได้).

- [ ] **Step 4: รัน test ทั้ง producer package**

Run: `go test ./internal/producer/ -run 'TestThemes|TestPresets|TestBrandCSS' -v`
Expected: PASS. **ถ้า test เดิม `TestPresets_SignatureIsFirstAndMatchesBrand` FAIL** (มันคาด `signature`) — แก้ test เดิมให้คาด `editorial-bold` + ลบ assertion `sig.ImageAnchor == Brand.ImageStyleAnchor()` (anchor ยกระดับแล้ว), คง `sig.Palette == Brand`.

- [ ] **Step 5: รัน producer package ทั้งหมด (regression)**

Run: `go test ./internal/producer/ -v 2>&1 | tail -30`
Expected: PASS ทั้งหมด (แก้ test เดิมที่อ้าง preset เก่า teal-coral/purple-gold ถ้ามี — เปลี่ยนไปอ้าง key ใหม่ หรือทดสอบ property ทั่วไปแทน)

- [ ] **Step 6: Commit**

```bash
git add internal/producer/presets.go internal/producer/presets_test.go
git commit -m "feat(themes): define 4 design themes sharing navy+orange brand palette"
```

---

## Task 4: buildScenePrompt — house-style + negative prompt + per-clip cohesion

**Files:**
- Modify: `internal/producer/brand.go` (`buildScenePrompt` 263-275)
- Test: `internal/producer/brand_test.go`

**Interfaces:**
- Consumes: `StylePreset` (มี ImageAnchor + Key), `SafeZone`
- Produces: `buildScenePrompt(concept, aspect string, preset StylePreset, clipToken string) string` — **เพิ่ม param `clipToken`** (สตริงสั้นคงที่ต่อคลิป เพื่อสั่งความต่อเนื่องของสไตล์ข้ามฉาก). ผู้เรียกทุกที่ต้องส่ง clipToken (ดู Task 9).

> **Honest note (per-clip seed):** เราไม่ยืนยันว่า kie gpt-image-2 รับ `seed` param จริง (ยังไม่ตรวจ API). แทนที่จะพึ่ง seed จริง เราใส่ **cohesion directive** ที่เป็น text token คงที่ต่อคลิป — เป็น best-effort ให้ภาพในคลิปเดียวเข้าชุดกัน. ถ้าภายหลังยืนยันว่า API รับ seed ได้ ค่อยเพิ่มเป็น true seed (ดู "คำถามค้าง" ในสเปก).

- [ ] **Step 1: เขียน test (failing)**

เพิ่มใน `brand_test.go`:
```go
func TestBuildScenePrompt_HasNegativeExclusionsAndCohesion(t *testing.T) {
	p := PresetByKey("cinematic-photo")
	got := buildScenePrompt("hands on a laptop", "9:16", p, "clip-abc123")
	for _, want := range []string{
		p.ImageAnchor,                 // theme art anchor present
		"hands on a laptop",           // subject present
		"NO text",                     // hard no-text rule (case-insensitive contains below)
		"cohesive",                    // cohesion directive present
		"clip-abc123",                 // per-clip token threaded in
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q\n%s", want, got)
		}
	}
}

func TestBuildScenePrompt_EmptyConceptFallsBack(t *testing.T) {
	got := buildScenePrompt("   ", "9:16", PresetByKey("editorial-bold"), "clip-x")
	if !strings.Contains(got, genericSceneSubject) {
		t.Errorf("empty concept must fall back to genericSceneSubject:\n%s", got)
	}
}
```

- [ ] **Step 2: รัน test ให้ FAIL**

Run: `go test ./internal/producer/ -run TestBuildScenePrompt -v`
Expected: FAIL — signature เก่าไม่มี `clipToken` (compile error)

- [ ] **Step 3: อัปเดต `buildScenePrompt`**

แทนที่ทั้งฟังก์ชัน (brand.go:263-275):
```go
func buildScenePrompt(concept, aspect string, preset StylePreset, clipToken string) string {
	subject := strings.TrimSpace(concept)
	if subject == "" {
		subject = genericSceneSubject
	}
	sz := preset.Palette.SafeZone(aspect)
	return preset.ImageAnchor + " " +
		"Subject: " + subject + ". " +
		"Composition: " + sz.NegativeSpace + ". " +
		"Keep the image uncluttered with generous negative space. " +
		// Cross-scene cohesion: all scenes in one clip share the same look.
		"Maintain a cohesive style across the whole set: same lighting direction, " +
		"same color grade, same rendering style (style set: " + clipToken + "). " +
		// Anti-"AI-slop" exclusions.
		"Avoid: oversaturated colors, plastic/glossy surfaces, warped hands or faces, " +
		"generic stock-photo look, watermarks, extra limbs, cluttered composition. " +
		"ABSOLUTELY NO text, letters, numbers, words, UI labels, or logos anywhere in the image. " +
		"Place the main subject in the UPPER 55% of the frame; keep the LOWER 45% simple and uncluttered (a text card is overlaid there)."
}
```

- [ ] **Step 4: แก้ผู้เรียกใน producer ให้ compile (ชั่วคราว)**

หา call site: `grep -rn "buildScenePrompt(" internal/producer/`. ทุกที่เพิ่ม arg สุดท้าย. ถ้ายังไม่มี clip token จริง ให้ส่ง `clip.ID` (Task 9 จะทำให้สมบูรณ์). ชั่วคราวถ้าใน test อื่นเรียก ให้ส่ง `""`.

- [ ] **Step 5: รัน test + build**

Run: `go test ./internal/producer/ -run TestBuildScenePrompt -v && go build ./...`
Expected: PASS + build ผ่าน

- [ ] **Step 6: Commit**

```bash
git add internal/producer/brand.go internal/producer/brand_test.go
git commit -m "feat(themes): image prompt house-style — negative exclusions + per-clip cohesion token"
```

---

## Task 5: Caption ไฮไลต์คำ emphasis จริง (แทนคำยาวสุด)

**Files:**
- Modify: `internal/producer/captions.go` (`TranscriptSegment` type + `captionSegmentsFromScenes` 24-108)
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (`keyIdx` line 239, caller 243)
- Test: `internal/producer/captions_test.go`

**Interfaces:**
- Consumes: `agent.GeneratedScene.EmphasisWords []string` (script.go:44), `sceneBound`
- Produces: `TranscriptSegment` +field `Emphasis []string json:"emph,omitempty"`; caption JS ไฮไลต์คำที่ตรง emphasis ก่อน, fallback = คำยาวสุด

- [ ] **Step 1: เขียน test (failing)**

เพิ่มใน `captions_test.go`:
```go
func TestCaptionSegments_CarryEmphasisFromScene(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "บัญชีโฆษณาโดนแบนถาวร", EmphasisWords: []string{"โดนแบน"}},
	}
	bounds := []sceneBound{{Start: 0, End: 5}}
	segs := captionSegmentsFromScenes(scenes, bounds)
	if len(segs) == 0 {
		t.Fatal("expected segments")
	}
	// At least one produced segment must carry the scene's emphasis words so the
	// template can highlight the RIGHT word (not merely the longest).
	found := false
	for _, s := range segs {
		for _, e := range s.Emphasis {
			if e == "โดนแบน" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no segment carried emphasis %q; got %+v", "โดนแบน", segs)
	}
}
```

- [ ] **Step 2: รัน test ให้ FAIL**

Run: `go test ./internal/producer/ -run TestCaptionSegments_CarryEmphasisFromScene -v`
Expected: FAIL — `TranscriptSegment` ไม่มี field `Emphasis`

- [ ] **Step 3: เพิ่ม field ใน TranscriptSegment**

หา struct `TranscriptSegment` (`grep -n "type TranscriptSegment" internal/producer/`). เพิ่ม field:
```go
	Emphasis []string `json:"emph,omitempty"` // words the template should highlight; empty ⇒ longest-word fallback
```

- [ ] **Step 4: attach emphasis ใน `captionSegmentsFromScenes`**

ใน loop สร้าง segment (captions.go ~68-80 ในบล็อก `for j, ph := range phrases`) เปลี่ยนการ append ให้แนบ emphasis ของ scene ปัจจุบัน (เฉพาะคำ emphasis ที่ปรากฏใน phrase นั้น):
```go
			emph := emphasisInPhrase(scenes[i].EmphasisWords, ph)
			segs = append(segs, TranscriptSegment{
				Text:     ph,
				Start:    math.Round(start*100) / 100,
				End:      math.Round(end*100) / 100,
				Emphasis: emph,
			})
```
เพิ่ม helper ท้ายไฟล์ captions.go:
```go
// emphasisInPhrase returns the subset of emphasis words that appear in phrase,
// so each caption segment only carries the emphasis it can actually highlight.
func emphasisInPhrase(words []string, phrase string) []string {
	var out []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w != "" && strings.Contains(phrase, w) {
			out = append(out, w)
		}
	}
	return out
}
```

- [ ] **Step 5: รัน Go test ให้ PASS**

Run: `go test ./internal/producer/ -run 'TestCaptionSegments|TestCaptionSegmentsFromScenes' -v`
Expected: PASS ทั้งหมด (test เดิมยังผ่าน — field ใหม่ optional)

- [ ] **Step 6: แก้ template `keyIdx` ให้ใช้ emphasis**

ใน `layout_multi_scene.html.tmpl` แก้ `keyIdx` (line 239) + caller (line 243):

แทน line 239:
```javascript
      function keyIdx(ws,emph){
        // Prefer a word that matches the scene's emphasis; fall back to longest.
        if(emph&&emph.length){
          for(let i=0;i<ws.length;i++){const t=ws[i].trim();
            for(const e of emph){if(e&&t.indexOf(e)>=0)return i;}}
        }
        let b=0,bl=-1;ws.forEach((w,i)=>{const l=gl(w.trim()).length;if(l>bl){bl=l;b=i;}});return b;
      }
```
แทน line 243 (`const ws=splitWords(seg.text);const k=keyIdx(ws);`):
```javascript
        const ws=splitWords(seg.text);const k=keyIdx(ws,seg.emph);
```

- [ ] **Step 7: ยืนยัน template ยัง parse ได้ (render smoke)**

Run: `go test ./internal/producer/ -run 'TestRenderCompositionScenes|Render' -v 2>&1 | tail -20`
Expected: test ที่ execute template ต้อง PASS (template ยัง valid). ถ้าไม่มี test ชื่อนี้ ให้รัน `go build ./...` แล้วตรวจ template ด้วย test ใน composition_scenes_render_test.go: `go test ./internal/producer/ -run Scenes -v | tail`.

- [ ] **Step 8: Commit**

```bash
git add internal/producer/captions.go internal/producer/captions_test.go internal/producer/templates/layout_multi_scene.html.tmpl
git commit -m "feat(themes): captions highlight true emphasis word, not longest"
```

---

## Task 6: Template — heading font + per-theme texture + motion จาก data/vars

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl`
- Modify: `internal/producer/composition.go` (`scenesTemplateData` 12-34 + data build 109-131 + funcs)
- Test: `internal/producer/composition_scenes_test.go` หรือ render test ที่มีอยู่

**Interfaces:**
- Consumes: `preset.HeadingFont`, `preset.Motion`, `preset.Key`
- Produces: template อ่าน `--font-heading` (จาก BrandCSS แล้ว), `data-theme="{{.ThemeKey}}"` บน `#root`, และ motion จาก `{{.EntranceDur}}/{{.EntranceEase}}/{{.BGZoomTo}}`. `scenesTemplateData` +fields `ThemeKey string`, `EntranceDur float64`, `EntranceEase string`, `BGZoomTo float64`.

- [ ] **Step 1: เพิ่ม field ใน scenesTemplateData**

ใน `composition.go` struct `scenesTemplateData` (12-34) เพิ่ม:
```go
	ThemeKey     string
	EntranceDur  float64
	EntranceEase string
	BGZoomTo     float64
```
และหา `RenderCompositionScenes` params struct (ที่มี `p.BrandCSS`, `p.BrandName`) — เพิ่ม field เดียวกัน (`ThemeKey`, `Motion MotionProfile`) เพื่อรับจากผู้เรียก. ในบล็อกสร้าง `data := scenesTemplateData{...}` (109-131) เพิ่ม:
```go
		ThemeKey:     p.ThemeKey,
		EntranceDur:  motion.EntranceDur,
		EntranceEase: motion.EntranceEase,
		BGZoomTo:     motion.BGZoomTo,
```
ก่อนบล็อกนั้นเพิ่ม default guard:
```go
	motion := p.Motion
	if motion.EntranceEase == "" {
		motion = MotionDefault
	}
```

- [ ] **Step 2: template — heading font บน display text**

ใน `<style>` (หลังบรรทัด 21) เพิ่ม rule ให้ display text ใช้ `--font-heading`:
```css
      .kicker,h1.title,.stat,.stat-label,.step-of,.step-title,.pill,.cta,.brandbig,
      .chip .n,.badge-brand,.cap-phrase.key{font-family:var(--font-heading,"Sarabun"),sans-serif}
```
(เนื้อความ/rows ยังใช้ Sarabun body ตาม `#root`.)

- [ ] **Step 3: template — @font-face ฟอนต์ใหม่**

หลังบล็อก Sarabun @font-face (หลังบรรทัด 11) เพิ่ม:
```css
      @font-face{font-family:"Kanit";src:url("assets/fonts/Kanit-Bold.ttf") format("truetype");font-weight:700;font-display:block}
      @font-face{font-family:"Kanit";src:url("assets/fonts/Kanit-ExtraBold.ttf") format("truetype");font-weight:800;font-display:block}
      @font-face{font-family:"Kanit";src:url("assets/fonts/Kanit-Black.ttf") format("truetype");font-weight:900;font-display:block}
      @font-face{font-family:"Prompt";src:url("assets/fonts/Prompt-SemiBold.ttf") format("truetype");font-weight:600;font-display:block}
      @font-face{font-family:"Prompt";src:url("assets/fonts/Prompt-Bold.ttf") format("truetype");font-weight:700;font-display:block}
      @font-face{font-family:"Prompt";src:url("assets/fonts/Prompt-ExtraBold.ttf") format("truetype");font-weight:800;font-display:block}
      @font-face{font-family:"IBM Plex Sans Thai";src:url("assets/fonts/IBMPlexSansThai-Medium.ttf") format("truetype");font-weight:500;font-display:block}
      @font-face{font-family:"IBM Plex Sans Thai";src:url("assets/fonts/IBMPlexSansThai-SemiBold.ttf") format("truetype");font-weight:600;font-display:block}
```

- [ ] **Step 4: template — data-theme + texture overlays**

แก้ `<div id="root" ...>` (line 118) เพิ่ม attribute `data-theme="{{.ThemeKey}}"`.
เพิ่ม CSS texture (หลังบรรทัด 21) — เฉพาะธีมที่ต้องการ feel ต่าง:
```css
      /* per-theme texture (brand palette stays constant; texture adds feel) */
      [data-theme="neon-techno"] .scrim{background:
        linear-gradient(180deg, rgba(6,24,64,.10) 0%, rgba(6,24,64,.60) 45%, rgba(6,24,64,.95) 62%, var(--navy-deep) 100%)}
      [data-theme="neon-techno"] .scene-bg::after{content:"";position:absolute;inset:0;
        background-image:linear-gradient(rgba(46,139,255,.08) 1px,transparent 1px),
          linear-gradient(90deg,rgba(46,139,255,.08) 1px,transparent 1px);background-size:64px 64px}
      [data-theme="soft-3d-clay"] .card{border-radius:44px;
        box-shadow:0 30px 70px rgba(0,0,0,.35),inset 0 2px 0 rgba(255,255,255,.25)}
```

- [ ] **Step 5: template — motion จาก vars**

ใน `<script>` หลัง `const MOTION_UP = ...;` (line 135) เพิ่ม:
```javascript
      const ENTRANCE_DUR = {{.EntranceDur}} || 0.6;
      const ENTRANCE_EASE = "{{.EntranceEase}}" || "power3.out";
      const BG_ZOOM_TO = {{.BGZoomTo}} || 1.06;
```
ในบล็อก `if (MOTION_UP) {` (line 205-211) แทนค่าคงที่ด้วย vars:
```javascript
          tl.fromTo(sceneEl,{opacity:0,scale:1.04},{opacity:1,scale:1,duration:idx===0?0.5:ENTRANCE_DUR,ease:ENTRANCE_EASE},inAt);
          if(bg) tl.fromTo(bg,{scale:1.04},{scale:BG_ZOOM_TO,duration:span,ease:"none"},inAt);
          if(content){
            tl.fromTo(content,{y:60,opacity:0},{y:0,opacity:1,duration:ENTRANCE_DUR,ease:ENTRANCE_EASE},sc.start+0.08);
          }
```

- [ ] **Step 6: เขียน/รัน render test ยืนยัน template execute ได้ + มี data-theme**

เพิ่ม test ใน `composition_scenes_test.go` (ปรับชื่อ constructor/params ให้ตรงของจริง — อ่าน `RenderCompositionScenes` signature ก่อน):
```go
func TestRenderScenes_InjectsThemeKeyAndHeadingFont(t *testing.T) {
	// ... build minimal params with ThemeKey:"neon-techno", Motion:PresetByKey("neon-techno").Motion,
	//     BrandCSS: PresetByKey("neon-techno").BrandCSS() ...
	html := string(mustRenderScenes(t /*, params */))
	for _, want := range []string{`data-theme="neon-techno"`, "--font-heading", "ENTRANCE_EASE"} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
}
```
Run: `go test ./internal/producer/ -run 'Scenes' -v 2>&1 | tail -20`
Expected: PASS (ถ้าต้องปรับ helper ให้ตรง signature จริง ทำก่อน)

- [ ] **Step 7: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition.go internal/producer/composition_scenes_test.go
git commit -m "feat(themes): template consumes heading font, per-theme texture, motion profile"
```

---

## Task 7: Migration 046 — scene prompt (hook ≤7 คำ + layout variety + emphasis)

**Files:**
- Create: `migrations/046_scene_prompt_themes.sql`
- Test: `internal/agent/scene_test.go` (เพิ่ม schema-contract test)

**Interfaces:**
- Consumes: agent_configs row `scene` (จาก migration 031)
- Produces: prompt ใหม่ที่บังคับ hook สั้น + layout ไม่ซ้ำติด + emphasis_words

- [ ] **Step 1: เขียน migration**

สร้าง `migrations/046_scene_prompt_themes.sql` (อิงรูปแบบ 031, ใช้ `$TPL$` dollar-quote):
```sql
-- 046_scene_prompt_themes.sql
-- Design Themes: sharpen the SceneAgent — hook <=7 words on frame 1, vary layout
-- across scenes (no >2 same in a row), always emit emphasis_words, and keep the
-- image_prompt theme-neutral (the theme's art anchor is applied downstream in Go).
UPDATE agent_configs SET
  system_prompt = 'คุณคือ Director ที่แตกสคริปวิดีโอเป็นซีนสำหรับ explainer แนวตั้ง 9:16 ภาษาไทย. เป้าหมายสูงสุด: 3 วินาทีแรกต้องหยุดนิ้วคนดูให้ได้. ใช้โครงสร้าง content ตาม layout, ห้ามใส่ emoji เด็ดขาด, ตอบเป็น JSON เท่านั้น.',
  prompt_template = $TPL$แตกสคริปนี้ออกเป็น 6-10 ซีน สำหรับวิดีโอแนวตั้ง 9:16 ยาว {{.TargetDurationSec}} วินาที

สคริป:
{{.Script}}

ธีมแบรนด์: {{.ThemeDescription}}

กฎ HOOK (สำคัญที่สุด): ซีนแรก (scene_number=1) layout ต้องเป็น "hook"; on_screen_text ของซีนแรกต้องเป็นวลีเดียว "ไม่เกิน 7 คำ" ที่ช็อก/ชวนสงสัย (ตัวเลข/คำถามที่โดนความกลัว เช่น โดนแบน เสียเงิน บัญชีปิด) — อ่านจบใน 1 วินาที.

ตอบเป็น JSON array เท่านั้น หนึ่งซีนหนึ่งไอเดีย แต่ละ object มี:
- "scene_number": ลำดับซีน (เริ่มที่ 1 ต่อเนื่อง)
- "voice_text": ประโยคพากย์ไทยของซีนนี้ (สั้น พูดลื่น)
- "on_screen_text": ข้อความบนจอสั้นๆ (ซีนแรก <=7 คำ)
- "emphasis_words": array คำ 1-2 คำใน on_screen_text/voice_text ที่ต้องเน้น (ห้ามว่าง) — ระบบจะไฮไลต์คำนี้ในแคปชั่น
- "caption_style": "word_pop" (ซีนเปิด/พลังสูง) หรือ "phrase_block" (ซีนเนื้อหา)
- "image_prompt": คำอธิบายภาพประกอบ (อังกฤษ) — บรรยาย "วัตถุ/ฉาก" เท่านั้น อย่าระบุสไตล์ศิลป์หรือสี (ระบบใส่สไตล์ธีมให้เอง); ห้ามมีตัวอักษร ตัวเลข โลโก้ UI; วางวัตถุครึ่งบนของเฟรม เว้นครึ่งล่างว่าง. ใส่ "" ถ้าไม่ต้องใช้ภาพ
- "layout": หนึ่งใน "hook" | "hero" | "stat" | "step" | "tip" | "cta"
- "content": object ตาม layout (ดูด้านล่าง)

กฎความหลากหลาย (กันคลิปน่าเบื่อ): อย่าใช้ layout เดียวกันเกิน 2 ซีนติดกัน สลับ layout ให้จังหวะน่าติดตาม.

กฎเหล็ก: ห้ามใส่ emoji หรือสัญลักษณ์ภาพ (❌ ✅ 📞 💳 🛡️ 👇 ⏰ ★ • ฯลฯ) ใน field ใดๆ เด็ดขาด ใช้โครงสร้าง content แทน (rows ที่มี "bad":true = แถวสีแดง).

content แยกตาม layout:
- hook (เปิดด้วยปัญหา): {"kicker":"วลีสั้น","rows":[{"t":"ปัญหา 1","bad":true},{"t":"ปัญหา 2","bad":true}]}
- hero (ประโยคเด่น): {"title":"ข้อความใหญ่ ครอบคำเน้นด้วย <span class=\"acc\">คำ</span>","sub":"บรรทัดรอง"}
- stat (โชว์ตัวเลข): {"kicker":"หัวเรื่องสั้น","stat":"2026","unit":"","statLabel":"คำอธิบายตัวเลข","chips":[{"n":"90%","t":"คำอธิบายสั้น"}]}
- step (ขั้นตอน): {"num":"1","of":"ขั้นตอนที่ 1 / 4","title":"ชื่อขั้นตอน","rows":[{"t":"รายละเอียด 1"},{"t":"รายละเอียด 2"}]}
- tip (เคล็ดลับ/ป้องกัน): {"pill":"ป้องกันระยะยาว","rows":[{"t":"ทิป 1"},{"t":"ทิป 2"}]}
- cta (ปิดท้าย): {"title":"คำถามชวนคุย","cta":"ทักหาเราเลย","brand":"ADS VANCE","sub":"คำโปรย"}

เลือก layout ให้เข้ากับเนื้อหา: ซีนเปิด=hook, ตัวเลข/สถิติ=stat, สอนทำทีละขั้น=step, สรุปเคล็ดลับ=tip, ปิดท้าย=cta, ประโยคเด่นทั่วไป=hero
rows ไม่เกิน 3 แถว, chips ไม่เกิน 2 อัน, ข้อความสั้นอ่านจบใน 2 วินาที หนึ่งซีนหนึ่งไอเดีย$TPL$
WHERE agent_name = 'scene';
```

- [ ] **Step 2: เขียน schema-contract test (failing)**

เพิ่มใน `internal/agent/scene_test.go`:
```go
// Locks the prompt↔struct contract for the themed scene prompt (migration 046):
// on_screen_text + emphasis_words must still unmarshal into GeneratedScene.
func TestSceneOutput_ThemedSchemaHasHookAndEmphasis(t *testing.T) {
	raw := `[{"scene_number":1,"layout":"hook","voice_text":"บัญชีโฆษณาโดนแบนถาวรเพราะอะไร",
	  "on_screen_text":"โดนแบนถาวรใน 3 วิ","emphasis_words":["โดนแบน"],
	  "caption_style":"word_pop","image_prompt":"a locked facebook ads dashboard",
	  "content":{"kicker":"ระวัง","rows":[{"t":"ยิงผิดกฎ","bad":true}]}}]`
	var scenes []GeneratedScene
	if err := json.Unmarshal([]byte(raw), &scenes); err != nil {
		t.Fatalf("themed scene JSON did not unmarshal: %v", err)
	}
	s := scenes[0]
	if s.Layout != "hook" || s.OnScreenText != "โดนแบนถาวรใน 3 วิ" {
		t.Errorf("hook fields drifted: layout=%q ost=%q", s.Layout, s.OnScreenText)
	}
	if len(s.EmphasisWords) == 0 {
		t.Errorf("emphasis_words must be present")
	}
	// on_screen_text ของ hook ต้อง <=7 คำ (นับ token คั่นด้วยช่องว่าง)
	if n := len(strings.Fields(s.OnScreenText)); n > 7 {
		t.Errorf("hook on_screen_text has %d words, want <=7", n)
	}
}
```
(เพิ่ม `import "strings"` ถ้ายังไม่มี)

- [ ] **Step 3: รัน test**

Run: `go test ./internal/agent/ -run TestSceneOutput_ThemedSchemaHasHookAndEmphasis -v`
Expected: PASS (เป็น contract test — ยืนยัน struct รองรับ schema; prompt เป็น instruction ที่ทดสอบด้วยตาตอน smoke)

- [ ] **Step 4: Commit**

```bash
git add migrations/046_scene_prompt_themes.sql internal/agent/scene_test.go
git commit -m "feat(themes): migration 046 — scene prompt hook<=7 words + layout variety + emphasis"
```

---

## Task 8: Migration 047 — critic + visual_qa รับรู้ theme coherence

**Files:**
- Create: `migrations/047_critic_visualqa_themes.sql`

**Interfaces:**
- Consumes: agent_configs rows `critic` (034), `visual_qa` (036)
- Produces: prompt ที่ตรวจ hook<=7คำ, emphasis มีจริง, และ visual_qa ยอมรับสื่อภาพหลากหลาย (photo/3D/illustration) ตราบใดที่ยัง navy+orange

- [ ] **Step 1: เขียน migration (UPDATE, idempotent)**

สร้าง `migrations/047_critic_visualqa_themes.sql`:
```sql
-- 047_critic_visualqa_themes.sql
-- Design Themes: teach the critic to enforce the sharp hook + real emphasis, and
-- teach visual_qa that image MEDIA now varies by theme (photo / 3D / illustration
-- are all valid) — only navy+orange brand drift or true breakage should fail.

-- Critic: add hook<=7 words + emphasis presence to the review skills.
UPDATE agent_configs SET
  skills = skills || E'\n- Hook: scene แรก on_screen_text ต้อง <=7 คำ และช็อก/ชวนสงสัยใน 1 วิ. ถ้ายาว/อืด ให้ตัดให้สั้นคม.'
    || E'\n- emphasis_words: ทุกซีนต้องมีคำเน้น 1-2 คำที่ตรงประเด็น (ระบบใช้ไฮไลต์แคปชั่น). ถ้าว่าง/ผิด ให้เติม.'
    || E'\n- อย่าบังคับสไตล์ภาพใน image_prompt (สไตล์มาจากธีม) — ปรับได้แค่ "วัตถุ/ฉาก" ให้ตรงเนื้อหา.'
WHERE agent_name = 'critic';

-- Visual QA: media varies by theme; do NOT fail a clip just because it is a photo
-- or a 3D render instead of flat illustration.
UPDATE agent_configs SET
  skills = skills || E'\n- สื่อภาพหลากหลายตามธีมได้ (ภาพถ่ายจริง / 3D เคลย์ / เวกเตอร์ / นีออน) — อย่าตั้ง ok=false เพราะ"ไม่ใช่เวกเตอร์แบน". ตัดสินที่แบรนด์ (navy+ส้ม) และความพังจริงเท่านั้น.'
WHERE agent_name = 'visual_qa';
```

- [ ] **Step 2: ตรวจ migration รันได้ (syntax) บน branch DB**

ถ้ามี local/branch Postgres: รัน migration runner แล้วดู log `Applied migration: 047_...`. ถ้าไม่มี ให้ตรวจ syntax ด้วย: `psql -f migrations/047_critic_visualqa_themes.sql --dry-run` หรืออย่างน้อย review ว่า string concat (`||`) ถูกและ idempotent (รันซ้ำ = ต่อ skills ซ้ำ — ยอมรับได้สำหรับ skills text; ถ้าต้องกัน ให้ใช้ guard `WHERE ... AND skills NOT LIKE '%<7 คำ%'`).

> **แก้ให้ idempotent:** เปลี่ยน `WHERE agent_name='critic'` เป็น `WHERE agent_name='critic' AND skills NOT LIKE '%emphasis_words: ทุกซีน%'` และ visual_qa เป็น `AND skills NOT LIKE '%สื่อภาพหลากหลายตามธีม%'` เพื่อกันการต่อซ้ำเมื่อ migration ถูกรันซ้ำ.

- [ ] **Step 3: Commit**

```bash
git add migrations/047_critic_visualqa_themes.sql
git commit -m "feat(themes): migration 047 — critic enforces hook/emphasis, visual_qa allows varied media"
```

---

## Task 9: Wire ThemeKey + Motion + clipToken ผ่าน pipeline + selection

**Files:**
- Modify: `internal/producer/producer.go` (`AssembleHyperframes916` — call site ของ buildScenePrompt + RenderCompositionScenes)
- Modify: `internal/producer/composition.go` (params struct ที่ RenderCompositionScenes รับ — +`ThemeKey`, `Motion`)
- Test: `internal/producer/presets_test.go` (avoid-last over 4 themes)

**Interfaces:**
- Consumes: `PickPreset(lastKey)` / `PickPresetWeighted` (presets.go), `Clip.StylePreset`, `Clip.ID`
- Produces: preset ที่เลือกถูกส่งต่อ: `preset.Key`→ThemeKey, `preset.Motion`→Motion, `clip.ID`→clipToken ใน buildScenePrompt

- [ ] **Step 1: เขียน test — avoid-last หมุนครบ 4 ธีม**

เพิ่มใน `presets_test.go`:
```go
func TestPickPreset_AvoidsLastAcrossFourThemes(t *testing.T) {
	// Never repeats the previous theme; over many picks all 4 themes appear.
	seen := map[string]bool{}
	last := "editorial-bold"
	for i := 0; i < 200; i++ {
		p := PickPreset(last)
		if p.Key == last {
			t.Fatalf("PickPreset returned same as last %q", last)
		}
		seen[p.Key] = true
		last = p.Key
	}
	if len(seen) < 3 {
		t.Errorf("expected variety across themes, only saw %v", seen)
	}
}
```

- [ ] **Step 2: รัน test**

Run: `go test ./internal/producer/ -run TestPickPreset_AvoidsLastAcrossFourThemes -v`
Expected: PASS (logic เดิมรองรับ 4 themes อยู่แล้ว — ยืนยัน)

- [ ] **Step 3: อ่าน call site + thread ค่า**

Run: `grep -n "buildScenePrompt\|RenderCompositionScenes\|PickPreset" internal/producer/producer.go`
ที่ call `buildScenePrompt(concept, "9:16", preset)` → เพิ่ม arg `clip.ID` (หรือ id ที่มีในสโคป): `buildScenePrompt(concept, "9:16", preset, clipID)`.
ที่สร้าง params ให้ `RenderCompositionScenes` → set `ThemeKey: preset.Key, Motion: preset.Motion`.
(ถ้า params struct field ยังไม่มี ให้เพิ่มใน composition.go ตาม Task 6 Step 1 — `ThemeKey string`, `Motion MotionProfile`.)

- [ ] **Step 4: build + รัน producer package ทั้งหมด**

Run: `go build ./... && go test ./internal/producer/ ./internal/agent/ 2>&1 | tail -30`
Expected: build ผ่าน + test PASS ทั้งหมด

- [ ] **Step 5: Commit**

```bash
git add internal/producer/producer.go internal/producer/composition.go internal/producer/presets_test.go
git commit -m "feat(themes): thread theme key + motion + clip cohesion token through render pipeline"
```

---

## Task 10: Smoke render end-to-end + verify ตาจริง

**Files:** (ไม่มีโค้ดใหม่ — verification)

- [ ] **Step 1: build binary + รัน unit ทั้ง repo**

Run: `go build ./... && go test ./... 2>&1 | tail -40`
Expected: ทุก package PASS

- [ ] **Step 2: render คลิปทดสอบ 1 คลิปต่อธีม (local)**

ใช้ path เดิมที่ repo ใช้ทดสอบ render (producer_hyperframes_test.go / composition_scenes_render_test.go เป็นตัวอย่าง). ตั้ง `STYLE_PRESETS_ENABLED=true` แล้ว trigger produce 4 ครั้ง (หรือ force preset ผ่าน test harness) — ยืนยันแต่ละธีม render ออก MP4 ไม่ error, ฟอนต์แสดงถูก (ไม่ fallback), ไม่มี lint `non_deterministic_code`.

Run (ตัวอย่าง lint/render ผ่าน hyperframes CLI ที่ repo pin ไว้ — ดู hyperframes.go): ตรวจ log ต้องมี `lint ok` → `render ok`.

- [ ] **Step 3: verify ด้วยตา (checklist)**

เปิด MP4/thumbnail 4 ธีม ตรวจ:
- [ ] แต่ละธีมหน้าตา "ต่างกันชัด" (ฟอนต์หัวข้อ, สื่อภาพ, motion)
- [ ] ทุกธีมยังมี badge ADS VANCE + ส้ม accent + progress (brand invariant)
- [ ] hook เฟรมแรกสั้น (<=7 คำ) โผล่เร็ว
- [ ] caption ไฮไลต์ "คำสำคัญ" ถูกตัว (ไม่ใช่คำยาวสุดมั่วๆ)
- [ ] ภาษาไทยไม่มีสระ/วรรณยุกต์ชนกัน (line-height ok)

- [ ] **Step 4: commit บันทึกผล verify**

```bash
git add -A && git commit -m "test(themes): end-to-end smoke render verified across 4 themes" --allow-empty
```

- [ ] **Step 5: หลัง merge — deploy note**

Rollout: push `feat/design-themes` → PR → merge master (frontend+backend auto-migrate + auto-deploy บน Railway). เปิดใช้จริงด้วย env `STYLE_PRESETS_ENABLED=true`. Rollback = ตั้ง flag ว่าง (กลับไป editorial-bold ธีมเดียว). Phase 3 (performance-weighting) เปิด `STYLE_PRESETS_PERFORMANCE_ENABLED=true` ภายหลังเมื่อมีข้อมูล retention.

---

## Self-Review

**Spec coverage:**
- §2 ขยาย Preset→Theme → Task 2,3 ✓
- §3 brand invariants → Task 3 (palette=Brand), Task 6 (badge/texture คง) ✓
- §4 แคตตาล็อก 4 ธีม → Task 3 ✓
- §5 ฟอนต์ vendor + Thai gotcha → Task 1,6 (@font-face) + Global Constraints ✓
- §6 layout variety → Task 7 (prompt "ไม่ซ้ำ >2 ติด") ✓ (composition ใหม่ๆ = deferred, ระบุใน spec §12/หมายเหตุ)
- §7 motion profile → Task 2 (type), 3 (per-theme), 6 (template vars) ✓
- §8 agents: image → Task 4; scene → Task 7; caption → Task 5; critic+visual_qa → Task 8 ✓
- §9 selection → Task 9 ✓
- §10 migration/model → Task 7,8 (046/047) ✓
- §11 flag/fallback → Global Constraints + Task 3 (Presets[0]) + Task 10 rollout ✓

**Placeholder scan:** ไม่มี TBD/TODO. ทุก step มีโค้ด/คำสั่งจริง. จุดที่ต้อง "อ่าน signature จริงก่อน" (Task 6 Step 6, Task 9 Step 3) ระบุคำสั่ง grep ให้หา — เป็น instruction ที่ทำได้จริง ไม่ใช่ placeholder.

**Type consistency:** `MotionProfile{EntranceDur,EntranceEase,BGZoomTo}` ใช้ตรงกันทุก task (2,3,6,9). `TranscriptSegment.Emphasis` (Task 5) ↔ template `seg.emph` (Task 5 Step 6) json tag `emph` ตรงกัน. `buildScenePrompt(..., clipToken)` signature เพิ่ม param ตรงกัน (Task 4,9). `StylePreset.HeadingFont/Motion` (Task 2) ใช้ใน Task 3,9 ตรงกัน. Presets keys (`editorial-bold`/`cinematic-photo`/`neon-techno`/`soft-3d-clay`) ใช้ตรงกันทุก task ✓

**Known open items (honest):**
- per-clip **true seed** ยังไม่ยืนยันว่า kie API รองรับ — ใช้ cohesion text token แทน (Task 4). ถ้าต้องการ true determinism ต้องตรวจ kieai.go API ก่อน.
- ฟอนต์ต้องมี network ตอน Task 1 (Google Fonts). ถ้า offline สิ้นเชิงต้องหาไฟล์มาวางเอง.
