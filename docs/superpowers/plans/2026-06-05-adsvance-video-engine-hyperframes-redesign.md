# ADS VANCE Video Engine — Hyperframes-only Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** รื้อระบบผลิตวิดีโอให้ใช้ Hyperframes ทางเดียว, รีธีมเป็น CI ของ ADS VANCE (royal blue + amber จากโลโก้จริง), เพิ่มมาสคอต bumper + pop, ต่อ gpt-image-2 ของ OpenAI ตรงสำหรับ hero art, และตัด FFmpeg/single-scene fallback ออก

**Architecture:** เก็บเครื่องยนต์ render เดิม (single `index.html` + GSAP timeline เดียวต่อคลิป — deterministic, inspect-safe) แล้วรีแฟกเตอร์ฝั่ง Go: รีธีม brand tokens (เปลี่ยน "ค่า" สี คงชื่อ CSS var เดิมเพื่อลดความเสี่ยง), เพิ่ม OpenAI image client, เพิ่ม scene `role` + ขยาย composition agent (caption_style / bg_mode / mascot_cue), เพิ่ม bumper + mascot overlay ใน multi-scene template, แล้วลบ path สำรองทิ้ง

**Tech Stack:** Go 1.x, Hyperframes CLI 0.6.70 (pinned), GSAP 3.14.2 (local asset), OpenAI Images API (gpt-image-2), Postgres (pgx), Sarabun font, html/template

**อ้างอิง spec:** `docs/superpowers/specs/2026-06-05-adsvance-video-engine-hyperframes-redesign-design.md`

---

## ค่าสีจริงจากโลโก้ (ใช้ตลอดแผน)

| บทบาท | ค่า |
|-------|-----|
| royal blue (พื้นหลัก) | `#0047AF` |
| blue-hi (glow บน) | `#1A5FD0` |
| blue-deep (vignette/ขอบล่าง) | `#062F78` |
| navy-ink (panel/การ์ด/ขอบ) | `#0A2358` |
| amber (accent) | `#F0A030` |
| amber-soft | `#E8A030` |
| amber-deep | `#C07028` |
| ink (ตัวอักษร) | `#F6F9FF` |
| muted | `#BCD2FF` |

**กลยุทธ์รีธีมที่เลือก (สำคัญ):** ไม่ rename CSS var ทั่ว template (เสี่ยงสูง) — แต่ map "ค่า" ใหม่ลงชื่อ var เดิม:
- `--navy-deep` ← `#062F78` · `--navy` ← `#0047AF` · `--navy-hi` ← `#1A5FD0`
- `--orange` ← `#F0A030` · `--orange-soft` ← `#E8A030` · `--orange-bright` ← `#F0A030`
- `--ink` ← `#F6F9FF` · `--muted` ← `#BCD2FF`
- semantic `--green`/`--red` คงเดิม

ผลคือ gradient พื้นฉาก `radial(--navy-hi → --navy → --navy-deep)` กลายเป็น `#1A5FD0 → #0047AF → #062F78` = ลุค royal blue ตามที่อนุมัติ

---

## File Structure

**Phase 1 — Brand retheme**
- Modify: `internal/producer/brand.go` (ค่าสี + ImageStyleAnchor)
- Modify: `internal/producer/brand_test.go` (expectations ใหม่)
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (literal hex/rgba)

**Phase 2 — gpt-image-2 (OpenAI ตรง)**
- Create: `internal/producer/openai_image.go` (client)
- Create: `internal/producer/openai_image_test.go`
- Modify: `internal/producer/producer.go` (ใช้ OpenAI client สำหรับรูป)

**Phase 3 — Mascot + bumper**
- Create: `cmd/mascot-gen/main.go` (เครื่องมือออฟไลน์สร้างท่ามาสคอต)
- Create: `internal/producer/mascot.go` (pose registry + request builder) + `mascot_test.go`
- Create: `assets/mascot/*.png` (ผลลัพธ์จากเครื่องมือ — commit เป็น asset)
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (bumper + mascot overlay)
- Modify: `Dockerfile` (COPY assets/mascot)

**Phase 4 — Script role + Composition agent**
- Modify: `internal/agent/script.go` (`SceneRole`)
- Modify: `internal/agent/composition.go` (`SceneDesign` + Normalize)
- Modify: `internal/producer/composition_types.go` (`SceneSpec` fields)
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (caption_style switch)
- Modify: tests ที่เกี่ยว

**Phase 5 — Remove fallback**
- Delete: `internal/producer/ffmpeg.go` + `ffmpeg`-related fields/calls
- Modify: `internal/producer/producer.go` (Hyperframes-only + retry)
- Create: `internal/producer/retry_render_test.go`

**Phase 6 — Verify**
- ตรวจ render จริง + build/test

---

## Phase 1 — Brand Retheme

### Task 1: รีธีมค่าสี Brand เป็น royal blue + amber

**Files:**
- Modify: `internal/producer/brand_test.go:103-128` (TestBrandColors), `:246-258` (colorVars ใน TestCSSVars)
- Modify: `internal/producer/brand.go:35-50` (var Brand)

- [ ] **Step 1: แก้ test ให้ expect ค่าใหม่ (test-first)**

ใน `brand_test.go` แทนค่า `want` ใน `TestBrandColors`:

```go
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"NavyDeep", Brand.NavyDeep, "#062F78"},
		{"Navy", Brand.Navy, "#0047AF"},
		{"NavyHi", Brand.NavyHi, "#1A5FD0"},
		{"Orange", Brand.Orange, "#F0A030"},
		{"OrangeSoft", Brand.OrangeSoft, "#E8A030"},
		{"OrangeBright", Brand.OrangeBright, "#F0A030"},
		{"Ink", Brand.Ink, "#F6F9FF"},
		{"Muted", Brand.Muted, "#BCD2FF"},
		{"Warn", Brand.Warn, "#ff5a52"},
		{"Win", Brand.Win, "#2fd17a"},
		{"Info", Brand.Info, "#3b82f6"},
	}
```

และใน `TestCSSVars` แก้ `colorVars` ให้ตรง:

```go
	colorVars := []struct{ varName, hex string }{
		{"--navy-deep", "#062F78"},
		{"--navy", "#0047AF"},
		{"--navy-hi", "#1A5FD0"},
		{"--orange", "#F0A030"},
		{"--orange-soft", "#E8A030"},
		{"--orange-bright", "#F0A030"},
		{"--ink", "#F6F9FF"},
		{"--muted", "#BCD2FF"},
		{"--green", "#2fd17a"},
		{"--red", "#ff5a52"},
	}
```

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/producer/ -run 'TestBrandColors|TestCSSVars' -v`
Expected: FAIL (ค่าจริงยังเป็น navy/orange เดิม)

- [ ] **Step 3: แก้ค่าใน brand.go**

แทน block `var Brand = BrandColors{...}` ใน `brand.go`:

```go
var Brand = BrandColors{
	NavyDeep: "#062F78",
	Navy:     "#0047AF",
	NavyHi:   "#1A5FD0",

	Orange:       "#F0A030",
	OrangeSoft:   "#E8A030",
	OrangeBright: "#F0A030",

	Ink:   "#F6F9FF",
	Muted: "#BCD2FF",

	Warn: "#ff5a52",
	Win:  "#2fd17a",
	Info: "#3b82f6",
}
```

อัปเดต doc-comment ค่าฮกซ์ข้างฟิลด์ใน struct `BrandColors` (บรรทัด 12-31) ให้ตรงค่าใหม่ด้วย

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/producer/ -run 'TestBrandColors|TestCSSVars' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/producer/brand.go internal/producer/brand_test.go
git commit -m "feat(brand): retheme tokens to royal-blue + amber CI"
```

---

### Task 2: รีไรต์ ImageStyleAnchor เป็นโทน royal blue + amber

**Files:**
- Modify: `internal/producer/brand_test.go:130-148` (TestImageStyleAnchor)
- Modify: `internal/producer/brand.go:100-107` (ImageStyleAnchor)

- [ ] **Step 1: แก้ test ให้ check hex ใหม่**

แทน body ของ `TestImageStyleAnchor` สองบรรทัดสุดท้าย:

```go
	if !strings.Contains(first, "#0047AF") {
		t.Errorf("ImageStyleAnchor() missing royal-blue hex #0047AF: %q", first)
	}
	if !strings.Contains(first, "#F0A030") {
		t.Errorf("ImageStyleAnchor() missing amber hex #F0A030: %q", first)
	}
```

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/producer/ -run TestImageStyleAnchor -v`
Expected: FAIL (ยังมี #0a1428/#ff6b2b)

- [ ] **Step 3: รีไรต์ ImageStyleAnchor()**

แทน return ใน `brand.go`:

```go
func (b BrandColors) ImageStyleAnchor() string {
	return "Flat modern editorial illustration style with soft cinematic lighting. " +
		"Strict two-tone palette: vivid royal blue #0047AF as the dominant background and structural color, " +
		"warm amber gold #F0A030 as the single accent for highlights, glows, and focal points. " +
		"No other saturated hues. Clean vector-quality rendering, minimal grain, no photorealism. " +
		"Subtle radial glow from the top-center, gentle vignette at the edges. " +
		"Atmosphere: confident, modern, premium digital-marketing brand identity."
}
```

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/producer/ -run TestImageStyleAnchor -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/producer/brand.go internal/producer/brand_test.go
git commit -m "feat(brand): retheme image style anchor to royal-blue + amber"
```

---

### Task 3: รีธีม literal hex/rgba ในเทมเพลต multi-scene

ในเทมเพลตมี literal สีที่ไม่ได้มาจาก CSS var ต้องเปลี่ยนให้เข้าโทนใหม่ (panel = navy-ink, พื้น = royal blue)

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl`
- Test: `internal/producer/composition_scenes_render_test.go` (มี render test อยู่แล้ว — เพิ่ม assertion no-legacy-hex)

- [ ] **Step 1: เพิ่ม test ว่า output ไม่มี legacy hex**

เปิด `internal/producer/composition_scenes_render_test.go` หา test ที่ render multi-scene เป็น HTML string (ตัวแปรผลลัพธ์ เช่น `html`/`out`). เพิ่ม subtest:

```go
	t.Run("no legacy navy/orange literals", func(t *testing.T) {
		for _, bad := range []string{"#0a1428", "#0f1d35", "#16284a", "#ff6b2b", "rgba(15, 29, 53", "rgba(15,29,53"} {
			if strings.Contains(html, bad) {
				t.Errorf("rendered multi-scene HTML still contains legacy literal %q", bad)
			}
		}
	})
```

(ถ้าชื่อตัวแปร HTML ต่างจาก `html` ให้ใช้ชื่อจริงในไฟล์; เพิ่ม `"strings"` ใน import ถ้ายังไม่มี)

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/producer/ -run 'Scenes' -v`
Expected: FAIL (ยังมี literal เดิม)

- [ ] **Step 3: แก้ literal ในเทมเพลต**

ใน `layout_multi_scene.html.tmpl` แก้ตามตารางนี้ (ใช้ replace-all ต่อค่า):

| เดิม | ใหม่ | ที่ |
|------|------|-----|
| `background: #0a1428;` | `background: var(--navy-deep);` | body (บรรทัด ~45) |
| `rgba(15, 29, 53, 0.82)` | `rgba(10, 35, 88, 0.82)` | slot-step |
| `rgba(15, 29, 53, 0.86)` | `rgba(10, 35, 88, 0.86)` | slot-callout / compare |
| `rgba(15, 29, 53, 0.55)` | `rgba(10, 35, 88, 0.55)` | compare last-side |
| `rgba(10, 20, 40, 0.82)` | `rgba(8, 24, 64, 0.86)` | cap-phrase (เข้มขึ้นเพื่ออ่านง่ายบนพื้นน้ำเงินสด) |
| `rgba(10, 20, 40, 0.60)` | `rgba(6, 24, 64, 0.62)` | scene-scrim top |
| `rgba(10, 20, 40, 0.15)` | `rgba(6, 24, 64, 0.15)` | scene-scrim mid (สองจุด) |
| `rgba(10, 20, 40, 0.85)` | `rgba(6, 24, 64, 0.88)` | scene-scrim bottom |
| `rgba(10, 20, 40, 0)` | `rgba(6, 24, 64, 0)` | scene-vignette inner |
| `rgba(8, 16, 32, 0.55)` | `rgba(4, 18, 52, 0.58)` | scene-vignette outer |

> หมายเหตุ: ค่า `rgba(255, 107, 43, ...)` (เงา badge/progress glow) เปลี่ยนเป็น amber glow `rgba(240, 160, 48, ...)` ทุกจุด (badge-brand box-shadow, badge-cat border/bg, progress-fill box-shadow)

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/producer/ -run 'Scenes' -v`
Expected: PASS

- [ ] **Step 5: รีวิวภาพจริง (manual)**

Run: `go test ./internal/producer/ -run 'Scenes' -v` แล้วเปิดไฟล์ HTML/MP4 ที่ test เขียนออก (golden/demo render test เขียนลง `/tmp` หรือ testdata) ตรวจด้วยตาว่าพื้นเป็น royal blue, accent เป็นทอง, caption อ่านออก
Expected: ลุค royal blue + amber ตรง mockup A

- [ ] **Step 6: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_scenes_render_test.go
git commit -m "feat(template): retheme multi-scene literals to royal-blue + amber"
```

---

## Phase 2 — gpt-image-2 (OpenAI ตรง)

### Task 4: helper แปลง aspect → ขนาด gpt-image-2

**Files:**
- Create: `internal/producer/openai_image.go`
- Create: `internal/producer/openai_image_test.go`

- [ ] **Step 1: เขียน test**

`openai_image_test.go`:

```go
package producer

import "testing"

func TestGptImageSize(t *testing.T) {
	cases := map[string]string{
		"9:16": "864x1536",
		"16:9": "1536x864",
		"1:1":  "1024x1024", // fallback
		"":     "1024x1024",
	}
	for aspect, want := range cases {
		if got := gptImageSize(aspect); got != want {
			t.Errorf("gptImageSize(%q) = %q, want %q", aspect, got, want)
		}
	}
}
```

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/producer/ -run TestGptImageSize -v`
Expected: FAIL ("undefined: gptImageSize")

- [ ] **Step 3: เขียน helper + โครง client ใน openai_image.go**

```go
package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const openAIImageAPI = "https://api.openai.com/v1/images/generations"

// gptImageSize maps a video aspect ratio to a gpt-image-2 size string. Sizes
// must have width/height divisible by 16 and aspect within 1:3..3:1.
func gptImageSize(aspect string) string {
	switch aspect {
	case "9:16":
		return "864x1536"
	case "16:9":
		return "1536x864"
	default:
		return "1024x1024"
	}
}

type OpenAIImageClient struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewOpenAIImageClient(pool *pgxpool.Pool) *OpenAIImageClient {
	return &OpenAIImageClient{pool: pool, client: &http.Client{Timeout: 5 * time.Minute}}
}

func (o *OpenAIImageClient) getAPIKey(ctx context.Context) (string, error) {
	var key string
	if err := o.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'openai_api_key'`).Scan(&key); err != nil {
		return "", fmt.Errorf("get openai_api_key from settings: %w", err)
	}
	if key == "" {
		return "", fmt.Errorf("openai_api_key is empty — set it in Settings")
	}
	return key, nil
}

type oaiImageReq struct {
	Model        string `json:"model"`
	Prompt       string `json:"prompt"`
	Size         string `json:"size"`
	Quality      string `json:"quality"`
	OutputFormat string `json:"output_format"`
}

type oaiImageResp struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GenerateImage matches OpenRouterClient.GenerateImage's signature so producer
// call sites swap with no other change.
func (o *OpenAIImageClient) GenerateImage(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	return retryableCall(ctx, "openai-image", func() error {
		return o.generateImageOnce(ctx, prompt, aspectRatio, outputPath)
	})
}

func (o *OpenAIImageClient) generateImageOnce(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	apiKey, err := o.getAPIKey(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(oaiImageReq{
		Model:        "gpt-image-2",
		Prompt:       prompt,
		Size:         gptImageSize(aspectRatio),
		Quality:      "high",
		OutputFormat: "png",
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", openAIImageAPI, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}
	var result oaiImageResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if result.Error != nil {
		return fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Data) == 0 || result.Data[0].B64JSON == "" {
		return fmt.Errorf("no image data in response")
	}
	// gpt-image-2 returns raw base64 (no data: prefix). saveBase64Image expects a
	// data URL "x,<b64>" — prepend a comma-delimited prefix to reuse it.
	return saveBase64Image("png;base64,"+result.Data[0].B64JSON, outputPath)
}
```

> `retryableCall`, `saveBase64Image`, `min` มีอยู่แล้วใน package producer (ใช้ซ้ำได้)

- [ ] **Step 4: รัน test ให้ผ่าน + build**

Run: `go test ./internal/producer/ -run TestGptImageSize -v && go build ./...`
Expected: PASS + build สำเร็จ

- [ ] **Step 5: Commit**

```bash
git add internal/producer/openai_image.go internal/producer/openai_image_test.go
git commit -m "feat(image): add OpenAI gpt-image-2 client"
```

---

### Task 5: ต่อ producer ให้ใช้ OpenAI client สำหรับรูป (hero art)

**Files:**
- Modify: `internal/producer/producer.go:28-52` (struct + constructor), `:128,:137,:339,:503` (call sites)
- Modify: จุดที่ construct Producer (หา `NewProducer(` ใน `cmd/` หรือ `internal/`)

- [ ] **Step 1: เพิ่ม field openAIImage ใน Producer**

ใน struct `Producer` เพิ่มหลัง `openRouter`:

```go
	openRouter   *OpenRouterClient
	openAIImage  *OpenAIImageClient
```

และใน `NewProducer` เพิ่มพารามิเตอร์ + assign:

```go
func NewProducer(pool *pgxpool.Pool, kie *KieClient, openRouter *OpenRouterClient, openAIImage *OpenAIImageClient, ffmpeg *FFmpegAssembler, voice, workDir string, tracker *progress.Tracker) *Producer {
	os.MkdirAll(workDir, 0755)
	return &Producer{pool: pool, kie: kie, openRouter: openRouter, openAIImage: openAIImage, ffmpeg: ffmpeg, defaultVoice: voice, workDir: workDir, tracker: tracker}
}
```

- [ ] **Step 2: เปลี่ยน call site รูปทั้งหมด `p.openRouter.GenerateImage` → `p.openAIImage.GenerateImage`**

Run (หา): `grep -n "openRouter.GenerateImage" internal/producer/producer.go`
แก้ทั้ง 4 จุด (บรรทัด ~128, ~137, ~339, ~503) เปลี่ยนเฉพาะ receiver จาก `p.openRouter` เป็น `p.openAIImage` (อาร์กิวเมนต์เหมือนเดิม)
> `GenerateVoice` (บรรทัด ~109) คง `p.openRouter` ไว้ (TTS ยังใช้ OpenRouter)

- [ ] **Step 3: อัปเดตจุด construct Producer**

Run: `grep -rn "NewProducer(" cmd/ internal/ --include=*.go`
ที่ call site นั้น สร้าง client ใหม่แล้วส่งเข้า:

```go
openAIImage := producer.NewOpenAIImageClient(pool)
prod := producer.NewProducer(pool, kie, openRouter, openAIImage, ffmpeg, voice, workDir, tracker)
```

- [ ] **Step 4: build + test**

Run: `go build ./... && go test ./internal/producer/ ./internal/orchestrator/ 2>&1 | tail -20`
Expected: build สำเร็จ, test ผ่าน (หรือ test เดิมที่ไม่เกี่ยวยังเขียว)

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(producer): generate hero art via OpenAI gpt-image-2"
```

---

## Phase 3 — Mascot Pose Library + Bumper

### Task 6: mascot pose registry + request builder

**Files:**
- Create: `internal/producer/mascot.go`
- Create: `internal/producer/mascot_test.go`

- [ ] **Step 1: เขียน test**

`mascot_test.go`:

```go
package producer

import (
	"strings"
	"testing"
)

func TestMascotPoses(t *testing.T) {
	want := []string{"rocket", "point_left", "point_right", "thumbs_up", "think", "wave"}
	got := MascotPoseNames()
	if len(got) != len(want) {
		t.Fatalf("MascotPoseNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pose[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMascotEditPrompt(t *testing.T) {
	p := mascotEditPrompt("thumbs_up")
	if p == "" {
		t.Fatal("empty prompt")
	}
	// must keep mascot identity + transparent bg + on-brand
	for _, must := range []string{"cheetah", "astronaut", "transparent", "thumbs"} {
		if !strings.Contains(strings.ToLower(p), must) {
			t.Errorf("prompt missing %q: %q", must, p)
		}
	}
}

func TestMascotCueToPose(t *testing.T) {
	cases := map[string]string{"point": "point_right", "thumbs": "thumbs_up", "think": "think", "none": "", "": ""}
	for cue, want := range cases {
		if got := MascotCueToPose(cue); got != want {
			t.Errorf("MascotCueToPose(%q) = %q, want %q", cue, got, want)
		}
	}
}
```

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/producer/ -run 'Mascot' -v`
Expected: FAIL (undefined)

- [ ] **Step 3: เขียน mascot.go**

```go
package producer

// mascotPoses is the fixed pose set baked once (offline) via cmd/mascot-gen and
// committed under assets/mascot/<name>.png as transparent PNGs.
var mascotPoses = []string{"rocket", "point_left", "point_right", "thumbs_up", "think", "wave"}

func MascotPoseNames() []string {
	out := make([]string, len(mascotPoses))
	copy(out, mascotPoses)
	return out
}

// poseDirective is the per-pose body/gesture line appended to the shared identity
// prompt when generating the pose via gpt-image-2 image edits.
var poseDirective = map[string]string{
	"rocket":      "riding a small white rocket, leaning forward with energy, thumbs forward",
	"point_left":  "pointing clearly to the left with one paw, confident smile",
	"point_right": "pointing clearly to the right with one paw, confident smile",
	"thumbs_up":   "giving a big thumbs up with one paw, cheerful",
	"think":       "one paw on chin in a thinking pose, curious expression",
	"wave":        "waving hello with one paw, friendly",
}

// mascotEditPrompt builds the gpt-image-2 /edits prompt for one pose. The
// reference image (the brand logo mascot) is sent alongside; this text directs
// the new pose while preserving identity, palette, and a transparent background.
func mascotEditPrompt(pose string) string {
	return "Keep the exact same character: a friendly orange-and-amber cheetah mascot " +
		"wearing a white astronaut suit and a blue ADS VANCE cap, bold outline cartoon style. " +
		"Redraw the same mascot " + poseDirective[pose] + ". " +
		"Royal blue #0047AF and amber #F0A030 brand palette, thick navy outlines. " +
		"Transparent background. Full body, centered, no text, no logo wordmark."
}

// MascotCueToPose maps a composition agent mascot_cue to a baked pose name.
// "point" resolves to point_right by default; "" / "none" mean no mascot.
func MascotCueToPose(cue string) string {
	switch cue {
	case "point":
		return "point_right"
	case "thumbs":
		return "thumbs_up"
	case "think":
		return "think"
	default:
		return ""
	}
}
```

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/producer/ -run 'Mascot' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/producer/mascot.go internal/producer/mascot_test.go
git commit -m "feat(mascot): pose registry + gpt-image-2 edit prompt builder"
```

---

### Task 7: เครื่องมือออฟไลน์สร้างท่ามาสคอต (cmd/mascot-gen)

**Files:**
- Create: `cmd/mascot-gen/main.go`
- Create: `assets/mascot/.gitkeep` (โฟลเดอร์)

- [ ] **Step 1: เขียน cmd/mascot-gen/main.go**

เครื่องมือนี้เรียก OpenAI `/v1/images/edits` (multipart: `image`=reference, `prompt`=mascotEditPrompt, `model`=gpt-image-2, `background`=transparent, `size`=1024x1024) บันทึก `assets/mascot/<pose>.png` ทุกท่า รับ API key จาก env `OPENAI_API_KEY` และ path รูปต้นแบบจาก arg

```go
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jaochai/video-fb/internal/producer"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: mascot-gen <reference.png|jpg>  (needs OPENAI_API_KEY env)")
		os.Exit(1)
	}
	ref := os.Args[1]
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		fmt.Println("OPENAI_API_KEY not set")
		os.Exit(1)
	}
	outDir := "assets/mascot"
	os.MkdirAll(outDir, 0755)
	for _, pose := range producer.MascotPoseNames() {
		fmt.Printf("generating %s ...\n", pose)
		if err := genPose(key, ref, pose, filepath.Join(outDir, pose+".png")); err != nil {
			fmt.Printf("  FAILED %s: %v\n", pose, err)
			os.Exit(1)
		}
	}
	fmt.Println("done — review assets/mascot/*.png then commit")
}

func genPose(key, ref, pose, out string) error {
	refBytes, err := os.ReadFile(ref)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("model", "gpt-image-2")
	w.WriteField("prompt", producer.MascotEditPrompt(pose))
	w.WriteField("background", "transparent")
	w.WriteField("size", "1024x1024")
	w.WriteField("style_intensity", "high")
	fw, _ := w.CreateFormFile("image", filepath.Base(ref))
	fw.Write(refBytes)
	w.Close()

	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/images/edits", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("%d: %s", resp.StatusCode, string(body))
	}
	var r struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return err
	}
	if len(r.Data) == 0 {
		return fmt.Errorf("no image data")
	}
	dec, err := base64.StdEncoding.DecodeString(r.Data[0].B64JSON)
	if err != nil {
		return err
	}
	return os.WriteFile(out, dec, 0644)
}
```

> ต้อง export `mascotEditPrompt` เป็น `MascotEditPrompt` ใน `mascot.go` (rename + อัปเดต test ใน Task 6 ให้เรียกชื่อใหม่)

- [ ] **Step 2: build เครื่องมือ**

Run: `go build ./cmd/mascot-gen/`
Expected: build สำเร็จ

- [ ] **Step 3: รันสร้าง asset จริง (manual, ต้องมี OPENAI_API_KEY)**

Run: `OPENAI_API_KEY=<key> ./mascot-gen "/Users/jaochai/Downloads/495062453_122116620752815042_1522405604509897747_n (1).jpg"`
ตรวจ `assets/mascot/*.png` ด้วยตา — เลือกท่าที่ใช้ได้ ถ้าท่าไหนเพี้ยน รันซ้ำเฉพาะท่านั้น (ปรับ poseDirective)
Expected: PNG พื้นใส 6 ท่า หน้าตาตรงต้นแบบ

- [ ] **Step 4: Commit (โค้ด + asset)**

```bash
git add cmd/mascot-gen/main.go internal/producer/mascot.go internal/producer/mascot_test.go assets/mascot/
git commit -m "feat(mascot): offline pose generator + baked pose assets"
```

---

### Task 8: bumper (intro/outro) + mascot overlay ในเทมเพลต

**Files:**
- Modify: `internal/producer/composition_types.go` (`ScenesParams` เพิ่ม flags)
- Modify: `internal/producer/composition.go` (ส่งค่า bumper/mascot เข้า template data)
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (markup + GSAP)
- Modify: `Dockerfile` (COPY assets/mascot)
- Modify: `internal/producer/composition_builder.go` (copy mascot assets เข้า project dir)
- Test: `internal/producer/composition_scenes_render_test.go`

- [ ] **Step 1: เพิ่ม field ใน ScenesParams**

ใน `composition_types.go` struct `ScenesParams` เพิ่ม:

```go
	// Bumper + mascot
	IntroMascot string // relative assets path, e.g. "assets/mascot/rocket.png" ("" = no intro)
	OutroMascot string // relative assets path ("" = no outro)
	CTAText     string // outro call-to-action, e.g. "กดติดตาม ไม่พลาดทุกอัปเดตแอด"
```

และใน `SceneSpec` เพิ่ม:

```go
	MascotPose string // relative assets path for a per-scene mascot pop ("" = none)
	CaptionStyle string // "word_pop" | "phrase_block"
```

- [ ] **Step 2: เพิ่ม render test ว่า bumper + mascot ขึ้น**

ใน `composition_scenes_render_test.go` เพิ่ม subtest ที่ set `IntroMascot: "assets/mascot/rocket.png"` แล้ว assert:

```go
	t.Run("intro bumper present", func(t *testing.T) {
		if !strings.Contains(html, "assets/mascot/rocket.png") {
			t.Error("intro mascot not rendered")
		}
		if !strings.Contains(html, `id="introBumper"`) {
			t.Error("intro bumper element missing")
		}
	})
```

- [ ] **Step 3: รัน test ให้ fail**

Run: `go test ./internal/producer/ -run 'Scenes' -v`
Expected: FAIL

- [ ] **Step 4: เพิ่ม markup + GSAP ในเทมเพลต**

ใน `layout_multi_scene.html.tmpl` ก่อนปิด `#root` เพิ่ม (ใช้ track index ว่างที่ยังไม่ชน เช่น 3 = intro, 4 = outro, 6 = scene mascot):

```html
      {{if .IntroMascot}}
      <div id="introBumper" class="clip" data-start="0" data-duration="1.4" data-track-index="3"
           style="position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;z-index:40;opacity:0;background:radial-gradient(120% 90% at 50% 40%,var(--navy-hi) 0%,var(--navy) 60%,var(--navy-deep) 100%)">
        <img src="{{.IntroMascot}}" style="width:62%;max-width:760px" data-layout-allow-overflow />
      </div>
      {{end}}
      {{if .OutroMascot}}
      <div id="outroBumper" class="clip" data-start="{{addFloat .DurationSeconds -1.6}}" data-duration="1.6" data-track-index="4"
           style="position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:24px;z-index:40;opacity:0;background:radial-gradient(120% 90% at 50% 40%,var(--navy-hi) 0%,var(--navy) 60%,var(--navy-deep) 100%)">
        <img src="{{.OutroMascot}}" style="width:54%;max-width:680px" data-layout-allow-overflow />
        <div style="color:#fff;font-weight:800;font-size:42px;text-align:center;letter-spacing:.02em">{{.CTAText}}</div>
      </div>
      {{end}}
```

ในแต่ละ scene loop (หลัง `.scene-content`) เพิ่ม per-scene mascot pop:

```html
        {{if $s.MascotPose}}
        <img class="scene-mascot" id="scene-mascot-{{$s.SceneNumber}}" src="{{$s.MascotPose}}"
             data-layout-allow-overflow
             style="position:absolute;right:4%;bottom:calc(var(--cap-bottom) + 80px);width:26%;max-width:340px;z-index:5;opacity:0" />
        {{end}}
```

ใน `<script>` เพิ่ม timeline สำหรับ bumper + mascot (transform/opacity เท่านั้น):

```javascript
      // Intro bumper: fade+scale in, hold, fade out before scene 1
      const intro = document.getElementById("introBumper");
      if (intro) {
        tl.fromTo(intro, { opacity: 0, scale: 0.92 }, { opacity: 1, scale: 1, duration: 0.4, ease: "var(--ease-out)" }, 0);
        tl.to(intro, { opacity: 0, scale: 1.04, duration: 0.4, ease: "var(--ease-in)" }, 1.0);
      }
      // Outro bumper
      const outro = document.getElementById("outroBumper");
      if (outro) {
        const oAt = Math.max(0, TOTAL - 1.6);
        tl.fromTo(outro, { opacity: 0, scale: 0.92 }, { opacity: 1, scale: 1, duration: 0.4, ease: "var(--ease-out)" }, oAt);
      }
      // Per-scene mascot pop (slides in from right, holds, out at scene end)
      SCENES.forEach(function (sc) {
        const m = document.getElementById("scene-mascot-" + sc.scene);
        if (!m) return;
        tl.fromTo(m, { opacity: 0, xPercent: 30 }, { opacity: 1, xPercent: 0, duration: 0.4, ease: "var(--ease-spring)" }, sc.start + 0.3);
        tl.to(m, { opacity: 0, xPercent: 20, duration: 0.3, ease: "var(--ease-in)" }, sc.end - 0.3);
      });
```

> ต้องมี template func `addFloat` ใน `composition.go` (ถ้ายังไม่มี — มี `addInt`, `durSec` อยู่แล้ว เพิ่มแบบเดียวกัน) และ SCENES JSON ต้องมี field ที่ JS ใช้ (มี `scene`/`start`/`end` อยู่แล้ว)

- [ ] **Step 5: copy mascot assets เข้า project dir ตอน build**

ใน `composition_builder.go` (ฟังก์ชัน BuildScenes) — จุดที่ copy fonts/gsap เข้า `assets/` — เพิ่ม copy `assets/mascot/<pose>.png` ทุกตัวที่ถูกอ้างถึง (intro/outro/scene poses) ไปยัง `<projectDir>/assets/mascot/`. ใช้ helper copy เดิมในไฟล์นั้น (หาด้วย `grep -n "func.*copy\|CopyFile\|io.Copy" internal/producer/composition_builder.go`)

- [ ] **Step 6: Dockerfile COPY mascot**

ใน `Dockerfile` หลังบรรทัด COPY fonts เพิ่ม:

```dockerfile
COPY assets/mascot/ /app/assets/mascot/
```

- [ ] **Step 7: รัน test ให้ผ่าน + build**

Run: `go test ./internal/producer/ -run 'Scenes' -v && go build ./...`
Expected: PASS + build

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(template): mascot intro/outro bumper + per-scene pop"
```

---

## Phase 4 — Script Role + Composition Agent

### Task 9: เพิ่ม SceneRole ใน Script

**Files:**
- Modify: `internal/agent/script.go:33-41` (GeneratedScene), `:95-129` (consts + Normalize)
- Test: `internal/agent/script_test.go`

- [ ] **Step 1: เขียน test**

ใน `script_test.go` เพิ่ม:

```go
func TestSceneRoleDefault(t *testing.T) {
	s := &GeneratedScript{Scenes: []GeneratedScene{
		{SceneType: "hook"},
		{SceneType: "step", SceneRole: "weird"},
	}}
	s.Normalize()
	if s.Scenes[0].SceneRole != "hook" {
		t.Errorf("scene0 role = %q, want hook (derived from type)", s.Scenes[0].SceneRole)
	}
	if s.Scenes[1].SceneRole != "content" {
		t.Errorf("scene1 invalid role should default to content, got %q", s.Scenes[1].SceneRole)
	}
}
```

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/agent/ -run TestSceneRole -v`
Expected: FAIL (ไม่มี field SceneRole)

- [ ] **Step 3: เพิ่ม field + Normalize logic**

ใน `GeneratedScene` เพิ่ม:

```go
	SceneRole       string          `json:"scene_role"` // hook|content|stat|compare|closing
```

เพิ่ม consts + map:

```go
var validSceneRoles = map[string]bool{
	"hook": true, "content": true, "stat": true, "compare": true, "closing": true,
}

// sceneTypeToRole derives a default role from the legacy scene_type when the
// LLM didn't supply a valid scene_role.
func sceneTypeToRole(t string) string {
	switch t {
	case SceneHook:
		return "hook"
	case SceneCTA:
		return "closing"
	default:
		return "content"
	}
}
```

ใน `Normalize()` ภายใน loop เพิ่ม:

```go
		if !validSceneRoles[s.Scenes[i].SceneRole] {
			s.Scenes[i].SceneRole = sceneTypeToRole(s.Scenes[i].SceneType)
		}
```

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/agent/ -run TestSceneRole -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/script.go internal/agent/script_test.go
git commit -m "feat(script): add scene_role with default derivation"
```

---

### Task 10: ขยาย SceneDesign (caption_style / bg_mode / mascot_cue) + Normalize

**Files:**
- Modify: `internal/agent/composition.go:80-144` (SceneDesign + Normalize)
- Test: `internal/agent/composition_test.go`

- [ ] **Step 1: เขียน test**

ใน `composition_test.go` เพิ่ม:

```go
func TestScenesNormalizeNewFields(t *testing.T) {
	d := &ScenesDecision{Scenes: []SceneDesign{
		{LayoutVariant: "hook_big", Slots: []Slot{{Role: "headline", Text: "x"}}},
		{LayoutVariant: "list_steps", Slots: []Slot{{Role: "body", Text: "y"}},
			CaptionStyle: "bogus", BgMode: "bogus", MascotCue: "bogus"},
	}}
	d.Normalize()
	if d.Scenes[1].CaptionStyle != "phrase_block" {
		t.Errorf("invalid caption_style should default phrase_block, got %q", d.Scenes[1].CaptionStyle)
	}
	if d.Scenes[1].BgMode != "flat" {
		t.Errorf("invalid bg_mode should default flat, got %q", d.Scenes[1].BgMode)
	}
	if d.Scenes[1].MascotCue != "none" {
		t.Errorf("invalid mascot_cue should default none, got %q", d.Scenes[1].MascotCue)
	}
}
```

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/agent/ -run TestScenesNormalizeNewFields -v`
Expected: FAIL

- [ ] **Step 3: เพิ่ม field + validation**

ใน `SceneDesign` เพิ่ม:

```go
	CaptionStyle string `json:"caption_style"` // word_pop|phrase_block
	BgMode       string `json:"bg_mode"`       // hero|flat
	MascotCue    string `json:"mascot_cue"`    // none|point|thumbs|think
```

เพิ่ม maps + default ใน `Normalize()` loop:

```go
var validCaptionStyles = map[string]bool{"word_pop": true, "phrase_block": true}
var validBgModes = map[string]bool{"hero": true, "flat": true}
var validMascotCues = map[string]bool{"none": true, "point": true, "thumbs": true, "think": true}
```

```go
		if !validCaptionStyles[d.Scenes[i].CaptionStyle] {
			d.Scenes[i].CaptionStyle = "phrase_block"
		}
		if !validBgModes[d.Scenes[i].BgMode] {
			d.Scenes[i].BgMode = "flat"
		}
		if !validMascotCues[d.Scenes[i].MascotCue] {
			d.Scenes[i].MascotCue = "none"
		}
```

- [ ] **Step 4: รัน test ให้ผ่าน**

Run: `go test ./internal/agent/ -run TestScenesNormalizeNewFields -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/composition.go internal/agent/composition_test.go
git commit -m "feat(composition-agent): caption_style + bg_mode + mascot_cue fields"
```

---

### Task 11: ต่อ producer แมป role/agent fields → SceneSpec + mascot + caption

**Files:**
- Modify: `internal/producer/producer.go:406-520` (assembleMultiScene — แมป DecideScenes → SceneSpec)
- Test: `internal/producer/composition_scenes_test.go`

- [ ] **Step 1: เขียน test ของ mapping (ระดับ pure func)**

ในจุดที่ producer แปลง `agent.SceneDesign` → `SceneSpec` ให้แยกเป็น pure func `buildSceneSpec(d agent.SceneDesign, scene agent.GeneratedScene, start, end float64, aspect string) SceneSpec` (ถ้ายังไม่แยก) เพื่อ test ได้ เพิ่ม `composition_scenes_test.go`:

```go
func TestBuildSceneSpecMapsNewFields(t *testing.T) {
	d := agent.SceneDesign{LayoutVariant: "stat_reveal", CaptionStyle: "word_pop", BgMode: "hero", MascotCue: "thumbs", AccentColor: "#F0A030"}
	sc := agent.GeneratedScene{SceneNumber: 2, SceneRole: "stat"}
	spec := buildSceneSpec(d, sc, 3.0, 7.0, "9:16")
	if spec.CaptionStyle != "word_pop" {
		t.Errorf("CaptionStyle = %q", spec.CaptionStyle)
	}
	if spec.MascotPose != "assets/mascot/thumbs_up.png" {
		t.Errorf("MascotPose = %q, want thumbs_up", spec.MascotPose)
	}
	if spec.BackgroundMode != "image" { // hero → image
		t.Errorf("BackgroundMode = %q, want image", spec.BackgroundMode)
	}
}
```

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/producer/ -run TestBuildSceneSpec -v`
Expected: FAIL

- [ ] **Step 3: เขียน/แก้ buildSceneSpec**

```go
func buildSceneSpec(d agent.SceneDesign, sc agent.GeneratedScene, start, end float64, aspect string) SceneSpec {
	bgMode := "css"
	if d.BgMode == "hero" {
		bgMode = "image"
	}
	pose := MascotCueToPose(d.MascotCue)
	mascotPath := ""
	if pose != "" {
		mascotPath = "assets/mascot/" + pose + ".png"
	}
	return SceneSpec{
		SceneNumber:    sc.SceneNumber,
		LayoutVariant:  d.LayoutVariant,
		AccentColor:    sanitizeHexColor(d.AccentColor, Brand.Orange),
		AnimationSpeed: d.AnimationSpeed,
		StartSec:       start,
		EndSec:         end,
		BackgroundMode: bgMode,
		CaptionStyle:   d.CaptionStyle,
		MascotPose:     mascotPath,
		// Slots filled by existing slot-mapping code
	}
}
```

> ใช้ `sanitizeHexColor` ที่มีอยู่แล้วใน producer (หาด้วย `grep -n "func sanitizeHexColor" internal/producer/`). เชื่อม `buildSceneSpec` เข้ากับ loop เดิมใน `assembleMultiScene`, แล้วเซ็ต `Slots` ตามโค้ดเดิม. ส่ง `IntroMascot/OutroMascot/CTAText` ใน `ScenesParams`: intro = `"assets/mascot/rocket.png"`, outro = `"assets/mascot/wave.png"`, CTA = ข้อความติดตาม

- [ ] **Step 4: รัน test ให้ผ่าน + build**

Run: `go test ./internal/producer/ -run 'Scene' -v && go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(producer): map agent design to scene spec (caption/bg/mascot)"
```

---

### Task 12: caption_style switch ในเทมเพลต (phrase_block path)

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (caption JS + CSS)
- Test: `internal/producer/caption_demo_render_test.go` (มี caption test อยู่แล้ว)

- [ ] **Step 1: ส่ง caption style ของแต่ละ scene เข้า SEGMENTS หรือ map ตามช่วงเวลา**

ใน SCENES JSON มี `start/end/variant` อยู่แล้ว — เพิ่ม `caption_style` ลงใน object ที่ marshal (จุดสร้าง ScenesJSON ใน `composition.go`). ใน template JS หา caption window ของ segment ว่าตกในฉากไหน แล้วเลือก path:

```javascript
      function captionStyleAt(t) {
        for (let i = 0; i < SCENES.length; i++) {
          if (t >= SCENES[i].start && t < SCENES[i].end) return SCENES[i].caption_style || "phrase_block";
        }
        return "phrase_block";
      }
```

- [ ] **Step 2: เพิ่ม phrase_block branch ในลูป SEGMENTS**

ในลูป `SEGMENTS.forEach` ครอบ logic เดิม (word-by-word) ด้วยเงื่อนไข:

```javascript
        const style = captionStyleAt(seg.start);
        if (style === "word_pop") {
          // ... existing per-word staggered reveal (เก็บโค้ดเดิมไว้) ...
        } else {
          // phrase_block: whole phrase fades in once, key word amber, no per-word stagger
          tl.fromTo(el, { opacity: 0, y: 18 }, { opacity: 1, y: 0, duration: 0.3, ease: "var(--ease-out)" }, inAt);
          tl.call(function () { el.classList.add("active"); }, null, inAt);
          tl.to(el, { opacity: 0, y: -14, duration: 0.24, ease: "var(--ease-in)" }, Math.max(inAt + 0.4, outAt - 0.22));
          tl.set(el, { opacity: 0 }, outAt);
        }
```

(คำคีย์สีทองยังใช้ `buildCaptionWords` + class `key` เหมือนเดิม — ต่างแค่ไม่ stagger ทีละคำ)

- [ ] **Step 3: เพิ่ม render test ว่าทั้งสอง style ใช้ได้**

ใน `caption_demo_render_test.go` (หรือ scenes render test) เพิ่ม assert ว่า เมื่อ scene เป็น hook → ScenesJSON มี `"caption_style":"word_pop"`, scene เนื้อหา → `"phrase_block"`

```go
	if !strings.Contains(html, `"caption_style":"word_pop"`) {
		t.Error("hook scene should emit word_pop caption style")
	}
```

- [ ] **Step 4: รัน test ให้ผ่าน + รีวิวภาพ**

Run: `go test ./internal/producer/ -run 'Caption|Scenes' -v`
Expected: PASS — เปิด MP4 demo ดู: hook = เด้งทีละคำ, เนื้อหา = วลีเต็ม

- [ ] **Step 5: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition.go internal/producer/caption_demo_render_test.go
git commit -m "feat(template): caption_style switch (word_pop hook / phrase_block content)"
```

---

## Phase 5 — Hyperframes-only (ลบ fallback)

### Task 13: ลบ FFmpeg + single-scene path, render fail = retry → error

**Files:**
- Delete: `internal/producer/ffmpeg.go`
- Modify: `internal/producer/producer.go` (struct, Produce, ลบ assembleHyperframes916)
- Delete: `internal/producer/templates/layout_dynamic_karaoke.html.tmpl` (ถ้าไม่ถูกอ้างที่อื่น)
- Modify: `internal/producer/composition.go` (ลบ RenderComposition single-scene ถ้าไม่ใช้)

- [ ] **Step 1: ตรวจ dependency ก่อนลบ**

Run: `grep -rn "ffmpeg\|FFmpeg\|AssembleSingleImage\|assembleHyperframes916\|layout_dynamic_karaoke\|RenderComposition\b" internal/ cmd/ --include=*.go`
รายการที่เจอคือจุดที่ต้องแก้/ลบให้หมด

- [ ] **Step 2: แก้ Produce ให้เป็น Hyperframes-only**

ใน `producer.go`:
- ลบ field `ffmpeg *FFmpegAssembler` จาก struct + พารามิเตอร์จาก `NewProducer` + จุด construct
- ในบล็อก multi-scene (บรรทัด ~169-181) แทน fallback `p.ffmpeg.AssembleSingleImage*` ด้วย return error:

```go
				if err := p.assembleMultiScene(ctx, clipID, clipDir, scenes, bounds, voicePath, "9:16", video916); err != nil {
					p.tracker.FailStep("assembly", err)
					return nil, fmt.Errorf("render 9:16 (hyperframes): %w", err)
				}
				if err := p.assembleMultiScene(ctx, clipID, clipDir, scenes, bounds, voicePath, "16:9", video169); err != nil {
					p.tracker.FailStep("assembly", err)
					return nil, fmt.Errorf("render 16:9 (hyperframes): %w", err)
				}
				goto assemblyDone
```

- ลบบล็อก single-scene ทั้งหมด (บรรทัด ~188-215) และ `assembleHyperframes916` (บรรทัด ~284-405)
- ถ้า multi-scene เป็น path เดียวแล้ว ปรับ guard `if p.hf != nil && p.hf.multiScene && p.hf.scenesAgentCfg != nil` ให้ถ้าไม่ครบ → return error ชัดเจน (ไม่เงียบ)

- [ ] **Step 3: ลบไฟล์**

```bash
git rm internal/producer/ffmpeg.go
# ลบเฉพาะถ้า grep ใน Step 1 ยืนยันว่าไม่ถูกอ้าง:
git rm internal/producer/templates/layout_dynamic_karaoke.html.tmpl
```

- [ ] **Step 4: build + test ทั้ง package**

Run: `go build ./... && go test ./internal/producer/ ./internal/orchestrator/ 2>&1 | tail -30`
Expected: build สำเร็จ; ลบ/ปรับ test ที่อ้าง ffmpeg/single-scene ที่พังตามจริง (เป็นการลบ dead test ของ path ที่ตัดทิ้ง)

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(producer): Hyperframes-only — remove FFmpeg + single-scene fallback"
```

---

### Task 14: retry + alert เมื่อ render ล้มเหลว

**Files:**
- Modify: `internal/producer/producer.go` (wrap assembleMultiScene ด้วย retry)
- Create: `internal/producer/retry_render_test.go`

- [ ] **Step 1: เขียน test ของ retry helper**

`retry_render_test.go`:

```go
package producer

import (
	"context"
	"errors"
	"testing"
)

func TestRetryRenderRetriesThenSucceeds(t *testing.T) {
	calls := 0
	err := retryRender(context.Background(), 3, func() error {
		calls++
		if calls < 2 {
			return errors.New("boom")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestRetryRenderExhausts(t *testing.T) {
	calls := 0
	err := retryRender(context.Background(), 2, func() error { calls++; return errors.New("nope") })
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}
```

- [ ] **Step 2: รัน test ให้ fail**

Run: `go test ./internal/producer/ -run TestRetryRender -v`
Expected: FAIL (undefined retryRender)

- [ ] **Step 3: เขียน retryRender**

```go
// retryRender runs fn up to attempts times, returning nil on first success or
// the last error after exhausting attempts. Respects ctx cancellation.
func retryRender(ctx context.Context, attempts int, fn func() error) error {
	var last error
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if last = fn(); last == nil {
			return nil
		}
		log.Printf("render attempt %d/%d failed: %v", i+1, attempts, last)
	}
	return last
}
```

- [ ] **Step 4: ใช้ retryRender ใน Produce + alert**

แทนการเรียก `assembleMultiScene` ตรงๆ ด้วย:

```go
				if err := retryRender(ctx, 2, func() error {
					return p.assembleMultiScene(ctx, clipID, clipDir, scenes, bounds, voicePath, "9:16", video916)
				}); err != nil {
					p.tracker.FailStep("assembly", fmt.Errorf("9:16 render failed after retries: %w", err))
					return nil, fmt.Errorf("render 9:16 after retries: %w", err)
				}
```

(ทำแบบเดียวกันกับ 16:9) — `p.tracker.FailStep` คือช่องทาง alert ที่ frontend เห็น (มีอยู่แล้ว)

- [ ] **Step 5: รัน test ให้ผ่าน + build**

Run: `go test ./internal/producer/ -run TestRetryRender -v && go build ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(producer): retry render + surface failure via tracker alert"
```

---

## Phase 6 — Verify

### Task 15: render จริง 1 คลิป + ตรวจ + build/test เต็ม

**Files:** ไม่มีการแก้ — เป็นการตรวจสอบ

- [ ] **Step 1: build + test ทั้ง repo**

Run: `go build ./... && go test ./... 2>&1 | tail -40`
Expected: build สำเร็จ, test เขียวทั้งหมด

- [ ] **Step 2: render demo จริง (มี render test ที่เขียน MP4 ออก)**

Run: `go test ./internal/producer/ -run 'phase24_demo|scrim_demo|caption_demo|Scenes' -v 2>&1 | tail -20`
หา path MP4/HTML ที่ test log ออกมา
Expected: lint + inspect ผ่าน, ไม่มี `[Browser:PAGEERROR]`/`is not defined`

- [ ] **Step 3: รีวิวด้วยตา (manual checklist)**

เปิด MP4 ทั้ง 9:16 และ 16:9 ตรวจตาม Success Criteria ใน spec:
- [ ] พื้น royal blue + accent ทอง (ไม่มี navy ดำ/ส้มแดงหลงเหลือ)
- [ ] hook = caption word-pop, เนื้อหา = phrase-block, อยู่ 2 บรรทัด ในเขต safe
- [ ] intro/outro bumper มาสคอตขึ้น + scene mascot pop ตาม cue
- [ ] hero art (ฉาก hero) มาจาก gpt-image-2, ฉากอื่น brand flat
- [ ] transition เนียน ไม่กระตุก ไม่มีของหลุดเฟรม

- [ ] **Step 4: ตรวจไม่มี path เก่าหลงเหลือ**

Run: `grep -rn "ffmpeg\|FFmpeg\|assembleHyperframes916\|#0a1428\|#ff6b2b" internal/ --include=*.go`
Expected: ไม่เจอ (นอกจาก comment ที่ตั้งใจ)

- [ ] **Step 5: เปิด cron (manual)**

หลังตรวจผ่าน ค่อยเปิด weekly cron ตามปกติ (ผ่าน UI/ตั้งค่าเดิม) — ดูคลิปแรกที่ระบบสร้างเองอีกครั้งก่อนปล่อยเต็ม

- [ ] **Step 6: Commit (ถ้ามีแก้เล็กน้อยจากรีวิว) + ขึ้น PR**

```bash
git add -A && git commit -m "chore: post-review polish" || true
git push -u origin redesign/hyperframes-video-engine
```

---

## Self-Review (เทียบ spec)

**Spec coverage:**
- ✅ Brand royal blue + amber → Task 1-3
- ✅ gpt-image-2 OpenAI ตรง → Task 4-5, mascot edits Task 7
- ✅ มาสคอต intro/outro + pop → Task 6-8, 11
- ✅ caption word_pop(hook)/phrase_block(content) → Task 9-12
- ✅ scene role ขับ caption/bg → Task 9-11
- ✅ hero art เฉพาะฉากคุ้ม (bg_mode) → Task 10-11
- ✅ ตัด FFmpeg/single-scene + retry/alert → Task 13-14
- ✅ verify render + inspect → Task 15
- ✅ Determinism (transform/opacity เท่านั้น) → ยึดในทุก task ที่แตะ GSAP (8, 12)

**Type consistency:** `MascotEditPrompt` (export ใน Task 7, ใช้ใน cmd), `MascotCueToPose`/`MascotPoseNames` (Task 6 → 11), `SceneSpec.CaptionStyle`/`MascotPose` (Task 8 def → 11 set → 12 use), `gptImageSize`/`GenerateImage` (Task 4 → 5), `buildSceneSpec` (Task 11), `retryRender` (Task 14) — ชื่อสอดคล้องกันข้าม task

**ความเสี่ยงที่ฝังการตรวจไว้:** contrast caption บนพื้นน้ำเงินสด (Task 3 step 5 + Task 15 manual), ตัด fallback (Task 14 retry/alert + Task 15 ตรวจก่อนเปิด cron), ขนาด gpt-image-2 หาร 16 (Task 4 gptImageSize)
