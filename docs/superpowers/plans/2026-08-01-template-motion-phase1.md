# Template Motion เฟส 1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** แก้ motion ใน template render 4 จุดตามตำรา motion-doctrine/cut-the-curve: ทิศทางฉากคงที่, รอยต่อฉากแบบ cut-the-curve (แก้บั๊ก data-start overlap ไปในตัว), ลบ idle drift, และแทน `back.out` แรงเกินด้วย springEase

**Architecture:** แก้ไฟล์เดียวเป็นหลักคือ `internal/producer/templates/layout_multi_scene.html.tmpl` (GSAP timeline ใน Go template ที่ embed เข้าไบนารี) + เพิ่ม string-assertion tests ใน `internal/producer/composition_scenes_test.go` ตาม idiom เดิมของ repo · ไม่แตะ DB / prompt / CLI เวอร์ชัน · flag ทุกตัวคงเดิม

**Tech Stack:** Go html/template, GSAP 3 (vendored), hyperframes CLI 0.6.70 (lint/inspect/render ผ่าน test harness ที่มีอยู่)

## Global Constraints

- **ห้ามพิมพ์ `-->` ที่ใดก็ตามใน JS** — Go html/template จะตัดทั้ง script block ทิ้ง (บั๊กคลิปจอเปล่าขึ้น YouTube เคยมาจากตรงนี้)
- **ห้ามพิมพ์ Go template action (`{{...}}`) ในคอมเมนต์ JS** — template อ่านเป็นบล็อกจริง (คอมเมนต์เตือนอยู่ที่ tmpl:951-954)
- **ห้าม `Math.random`/`Date.now`/`new Date()` ในโค้ดที่เพิ่ม** — lint gate `non_deterministic_code` จะ flag
- **ห้าม CSS transition/@keyframes** — ทุก motion อยู่บน GSAP timeline เดียวที่ paused (`window.__timelines["main"]`)
- ทุก tween ต้อง seek-safe: `fromTo`/`set` ค่าสัมบูรณ์ ห้าม `+=`, ห้าม `repeat`
- **ห้าม tween property เดียวกันซ้อนเวลากันบน element เดียว** (กติกา "ONE entrance per element" ที่ tmpl:864)
- Thai-safe: ห้าม letter-spacing ติดลบ, ห้ามแตะ line-height ที่ตั้งไว้
- flag บน prod: `AUDIO_MOTION_ENABLED=true` (MOTION_UP branch คือ branch ที่รันจริง), `SCENE_MOTION_V2_ENABLED=false`, `COVER_SCENE_ENABLED=false` — โค้ดใน branch ที่ปิดต้องยังถูกต้อง
- ทุก commit ลงท้ายด้วย:
  ```
  Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01CL2MQzxMoJE6FZ7ejGR8J6
  ```
- เลขบรรทัดในแผนอ้างอิงสถานะไฟล์ ณ commit `b43f702` — งานแต่ละ Task จะเลื่อนบรรทัดของ Task ถัดไป ให้ยึด **ข้อความ old code** เป็นตัวหา ไม่ใช่เลขบรรทัด

---

### Task 0: สร้าง branch

**Files:** ไม่มีการแก้ไฟล์

- [ ] **Step 1: ตรวจว่า working tree สะอาดแล้วสร้าง branch**

```bash
cd /Users/jaochai/Code/video-fb
git status --short   # ต้องว่าง
git checkout -b feat/motion-seams-phase1
```

---

### Task 1: ทิศทาง slide คงที่ (เลิก ping-pong)

ตำรา motion-doctrine "The Current": ทั้งคลิปต้องมีทิศหลักทิศเดียว (เราเลือก **เลื่อนซ้าย** = เข้าจากขวา) · โค้ดเดิมสลับซ้าย/ขวาตามเลขคู่-คี่ของฉาก ซึ่งเป็น anti-pattern ที่ตำราห้ามตรงๆ · โค้ดนี้อยู่ใต้ flag MOTION_V2 (ปิดบน prod) แต่แก้ให้ถูกไว้ก่อนเปิด

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (~บรรทัด 819-826)
- Test: `internal/producer/composition_scenes_test.go`

**Interfaces:**
- Consumes: `RenderCompositionScenes(params ScenesParams) ([]byte, error)` + helper `sampleScenesParams(aspect string) ScenesParams` (มีอยู่แล้วในไฟล์เทสต์)
- Produces: HTML ที่มี `from:{x:  80` และไม่มี `sc.scene % 2` (Task 5 จะ render ของจริงตรวจซ้ำ)

- [ ] **Step 1: เขียนเทสต์ที่ fail ก่อน** — ต่อท้าย `internal/producer/composition_scenes_test.go`:

```go
// TestRenderScenes_SlideDirectionIsConstant pins the motion-doctrine "current":
// the slide entrance must always come from the right (+x) — never alternate
// per scene parity (ping-pong reads as amateur per the motion doctrine).
func TestRenderScenes_SlideDirectionIsConstant(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if strings.Contains(html, "sc.scene % 2") {
		t.Errorf("slide entrance still alternates direction by scene parity (ping-pong)")
	}
	slideRe := regexp.MustCompile(`v==="slide".*x:\s*80`)
	if !slideRe.MatchString(html) {
		t.Errorf("slide entrance is not the constant +80 (enter from the right)")
	}
}
```

- [ ] **Step 2: รันให้เห็นว่า fail**

Run: `go test ./internal/producer/ -run TestRenderScenes_SlideDirectionIsConstant -v`
Expected: FAIL ("still alternates direction")

- [ ] **Step 3: แก้ template** — ใน `layout_multi_scene.html.tmpl` แก้ 2 จุด:

old (คอมเมนต์เหนือ `contentEntrance`):
```js
      // contentEntrance returns the from/to vars for a scene's content entrance.
      // MOTION_V2 off → the legacy rise (y-based). MOTION_V2 on → per-scene
      // geometry from sc.entrance (punch/rise/slide); slide alternates direction.
```
new:
```js
      // contentEntrance returns the from/to vars for a scene's content entrance.
      // MOTION_V2 off → the legacy rise (y-based). MOTION_V2 on → per-scene
      // geometry from sc.entrance (punch/rise/slide); slide always enters from
      // the right — one directional current per clip, never ping-pong.
```

old:
```js
        if(v==="slide"){ const dx = (sc.scene % 2 === 0) ? -80 : 80; return {from:{x:dx,opacity:0}, to:{x:0,opacity:1}}; }
```
new:
```js
        if(v==="slide"){ return {from:{x:80,opacity:0}, to:{x:0,opacity:1}}; }
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/producer/ -run TestRenderScenes_SlideDirectionIsConstant -v`
Expected: PASS · แล้วรันทั้งแพ็กเกจกันพัง: `go test ./internal/producer/ -count=1` Expected: PASS (ถ้าเทสต์เดิมตัวไหน fail เพราะ string ที่เราเปลี่ยน ให้แก้เทสต์นั้นตามพฤติกรรมใหม่)

- [ ] **Step 5: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_scenes_test.go
git commit -m "fix(motion): slide entrance ทิศเดียวคงที่ตาม doctrine เลิก ping-pong คู่-คี่"
```

---

### Task 2: ลบ MOTION_V2 idle drift

motion-doctrine ห้าม drift/float ฆ่าเวลา ("motion must PERFORM, not breathe") · drift นี้อยู่ใต้ MOTION_V2 (ปิดบน prod) — ลบทิ้งพร้อมตัวแปร `entranceEnd` ที่มีไว้ป้อน drift อย่างเดียว

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (~บรรทัด 874-876, 891, 898, 908, 929-941)
- Test: `internal/producer/composition_scenes_test.go`

**Interfaces:**
- Consumes: เหมือน Task 1
- Produces: HTML ที่ไม่มี `driftDur` / `entranceEnd` — Task 3 จะแก้บรรทัดข้างเคียงต่อ (แผนเขียน old code ของ Task 3 แบบ "หลัง Task 2 ลบแล้ว")

- [ ] **Step 1: เขียนเทสต์ที่ fail ก่อน** — ต่อท้ายไฟล์เทสต์เดิม:

```go
// TestRenderScenes_NoIdleDrift: the motion doctrine bans slow ambient drift
// ("video is waiting"). The MOTION_V2 post-entrance content drift and its
// entranceEnd bookkeeping must be gone from the emitted JS.
func TestRenderScenes_NoIdleDrift(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, banned := range []string{"driftDur", "driftStart", "entranceEnd"} {
		if strings.Contains(html, banned) {
			t.Errorf("emitted JS still contains %q (idle drift should be removed)", banned)
		}
	}
}
```

- [ ] **Step 2: รันให้เห็นว่า fail**

Run: `go test ./internal/producer/ -run TestRenderScenes_NoIdleDrift -v`
Expected: FAIL (เจอทั้ง 3 คำ)

- [ ] **Step 3: แก้ template** — ลบ 5 จุด:

3.1 ลบทั้งบล็อก (คอมเมนต์+โค้ด):
```js
        // MOTION_V2: after the entrance settles, drift the whole content block
        // slowly upward (opposite the bg ken-burns) so the focal content is never
        // dead-still. Starts after the entrance so it never fights the entrance's
        // own y/scale tween.
        if(MOTION_V2 && content){
          // Start the drift 0.1s AFTER the content entrance finishes so it never
          // fights the entrance's own y tween (the "rise" variant animates y too).
          const driftStart = entranceEnd + 0.1;
          const driftDur = sc.end - driftStart;
          if(driftDur > 0.3){
            tl.to(content,{y:-12,duration:driftDur,ease:"none"},driftStart);
          }
        }
```

3.2 ลบประกาศ:
```js
        // entranceEnd = when the content entrance finishes; the MOTION_V2 drift
        // reuses it so its start stays in sync with whichever entrance branch ran.
        let entranceEnd = sc.start + 0.7;
```

3.3 ใน cover branch ลบบรรทัด: `          entranceEnd = sc.start + Math.max(0.5,durIn);`

3.4 ใน MOTION_UP branch ลบบรรทัด: `          entranceEnd = sc.start + 0.08 + durIn;`

3.5 ใน legacy branch ลบบรรทัด: `          entranceEnd = sc.start + 0.1 + contentDurIn;`

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/producer/ -run TestRenderScenes_NoIdleDrift -v` → PASS
แล้ว `go test ./internal/producer/ -count=1` → PASS

- [ ] **Step 5: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_scenes_test.go
git commit -m "refactor(motion): ลบ MOTION_V2 idle drift — doctrine ห้าม motion ฆ่าเวลา"
```

---

### Task 3: รอยต่อฉากแบบ cut-the-curve (แก้บั๊ก data-start overlap ในตัว)

ปัญหาเดิม 2 เรื่องที่แก้พร้อมกัน:
1. **บั๊ก overlap**: entrance เริ่มที่ `inAt = sc.start - 0.35` แต่ framework ซ่อน `.clip` จนถึง `data-start = sc.start` → 0.35 วิแรกของ tween วิ่งตอนมองไม่เห็น ฉากโผล่กลางคันที่ opacity ~0.5
2. **crossfade ไม่มี carrier**: ฉากเดิมแค่ fade เข้า + `set opacity 0` ตอนจบ — ตำราเรียกว่ารอยต่อที่แย่ที่สุด

ดีไซน์ใหม่ (worker version จาก cut-the-curve §3 ปรับใช้กับ slab ทั้งฉาก):
- **เลิก pre-start ทั้งหมด**: `inAt = sc.start` ตรงกับ `data-start` เป๊ะ → บั๊ก 1 หายไปโดยโครงสร้าง
- **ขาออก** (ทุกฉากยกเว้นฉากสุดท้าย): ทั้ง wrapper เร่งเลื่อนซ้าย 12% ของความกว้างเฟรม (`power4.in` 0.33s) พร้อม fade จบพอดีที่รอยตัด (`power2.in` 0.30s)
- **ขาเข้า** (idx>0, MOTION_UP branch): hard cut — wrapper รับไม้ที่ x=+12% / opacity 0.35 แล้วไถลเข้าที่ (`power4.out` 0.40s ≥ ขาออก ตามกฎ velocity-match)
- ไม่ใส่ blur (ตำราบอก optional และ blur เต็มเฟรมแพงกับ CPU render 3 workers)
- `#root` มีพื้นทึบ `var(--navy-deep)` แล้ว (tmpl:31) → ผ่านกติกา seam-craft เรื่องแฟลชขาว

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (~บรรทัด 815-817, 868, 896-899, 967)
- Test: `internal/producer/composition_scenes_test.go`

**Interfaces:**
- Consumes: เหมือน Task 1 · **สมมติว่า Task 2 ลบ `entranceEnd` ไปแล้ว** (old code ข้างล่างไม่มีบรรทัดนั้น)
- Produces: ค่าคงที่ JS ใหม่ `SEAM_DX` + HTML ที่ไม่มี `sc.start-0.35`

- [ ] **Step 1: เขียนเทสต์ที่ fail ก่อน**:

```go
// TestRenderScenes_CutTheCurveSeam pins the new scene seam: no pre-start
// animation window (data-start must equal the first visible tween time — the
// old sc.start-0.35 ran while the framework still hid the clip), and the
// velocity-matched leftward exit/entry pair must be present.
func TestRenderScenes_CutTheCurveSeam(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if strings.Contains(html, "sc.start-0.35") {
		t.Errorf("pre-start entrance window still present (runs while clip is hidden)")
	}
	for _, want := range []string{"SEAM_DX", `"power4.in"`, `"power4.out"`} {
		if !strings.Contains(html, want) {
			t.Errorf("emitted JS missing %q (cut-the-curve seam not wired)", want)
		}
	}
}
```

- [ ] **Step 2: รันให้เห็นว่า fail**

Run: `go test ./internal/producer/ -run TestRenderScenes_CutTheCurveSeam -v`
Expected: FAIL

- [ ] **Step 3: แก้ template** — 4 จุด:

3.1 เพิ่มค่าคงที่ใต้ `SPEED_FACTORS`:

old:
```js
      const SPEED_FACTORS={fast:0.7,slow:1.35};
```
new:
```js
      const SPEED_FACTORS={fast:0.7,slow:1.35};
      // cut-the-curve partial travel: ~12% of frame width, leftward current.
      const SEAM_DX=Math.round({{.Width}}*0.12);
```

3.2 เลิก pre-start:

old:
```js
        const inAt=idx===0?0:Math.max(0,sc.start-0.35);
```
new:
```js
        const inAt=idx===0?0:sc.start;
```

3.3 ขาเข้าใน MOTION_UP branch:

old:
```js
          // Upgraded: incoming scene slides up + scales in while fading; bg ken-burns.
          const durIn=ENTRANCE_DUR*SPEED_FACTOR;
          tl.fromTo(sceneEl,{opacity:0,scale:1.04},{opacity:1,scale:1,duration:idx===0?0.5:durIn,ease:ENTRANCE_EASE},inAt);
```
new:
```js
          // Scene 0 fades up from black; later scenes take a cut-the-curve
          // hand-off: the slab picks up mid-fade (opacity .35) already moving
          // leftward, decelerating into place — velocity-matched to the
          // previous scene's power4.in exit.
          const durIn=ENTRANCE_DUR*SPEED_FACTOR;
          if(idx===0){
            tl.fromTo(sceneEl,{opacity:0,scale:1.04},{opacity:1,scale:1,duration:0.5,ease:ENTRANCE_EASE},0);
          }else{
            tl.fromTo(sceneEl,{x:SEAM_DX,opacity:0.35},{x:0,opacity:1,duration:0.4,ease:"power4.out"},inAt);
          }
```

3.4 ขาออก:

old:
```js
        tl.set(sceneEl,{opacity:0},sc.end);
```
new:
```js
        // cut-the-curve exit: the slab accelerates leftward and the fade dies
        // exactly at the cut, so something is always moving across the seam.
        if(idx<SCENES.length-1){
          tl.to(sceneEl,{x:-SEAM_DX,duration:0.33,ease:"power4.in"},sc.end-0.33);
          tl.to(sceneEl,{opacity:0,duration:0.3,ease:"power2.in"},sc.end-0.3);
        }
        tl.set(sceneEl,{opacity:0,x:0},sc.end);
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/producer/ -run TestRenderScenes_CutTheCurveSeam -v` → PASS
แล้ว `go test ./internal/producer/ -count=1` → PASS (เทสต์ scene_timing / cover เดิมต้องยังเขียว — ถ้าตัวไหนล็อก `sc.start-0.35` ไว้ ให้ปรับตามพฤติกรรมใหม่)

- [ ] **Step 5: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_scenes_test.go
git commit -m "feat(motion): รอยต่อฉาก cut-the-curve + เลิก pre-start ที่วิ่งตอน clip ถูกซ่อน"
```

---

### Task 4: springEase แทน back.out ที่แรงเกิน

- การ์ด stat ใช้ `back.out(1.7)` (เพดาน doctrine พอดีแต่ตำรา easing แนะนำ spring จริงแทน — อ่านเป็นฟิสิกส์ ไม่ใช่การ์ตูน) → springEase ζ=0.85 (iOS register)
- คำเน้น caption ใช้ `back.out(2)` ซึ่ง**เกินเพดาน 1.7** → ลดเหลือ `back.out(1.7)` (จังหวะ caption ผูกเวลาแน่นกับ tween ตาม-หลัง จึงไม่สลับเป็น spring ที่คำนวณ duration เอง)
- กฎจากตำรา: ease ที่ overshoot ห้ามใช้กับ opacity → แยก opacity เป็น tween `power2.out` ต่างหากที่เวลาเดียวกัน

**Files:**
- Modify: `internal/producer/templates/layout_multi_scene.html.tmpl` (~บรรทัด 852-863 บริเวณหลัง `kpiCountUp`, 922, 1001)
- Test: `internal/producer/composition_scenes_test.go`

**Interfaces:**
- Consumes: เหมือน Task 1
- Produces: ฟังก์ชัน JS `springEase(opts)` + ค่าคงที่ `STAT_SETTLE` ในหน้า (deterministic ไม่มี random)

- [ ] **Step 1: เขียนเทสต์ที่ fail ก่อน**:

```go
// TestRenderScenes_SpringEaseReplacesHardBack: back.out(2) exceeds the
// doctrine ceiling (1.7); stat cards move to a baked damped-spring ease
// (deterministic, seek-safe) with opacity split onto its own tween.
func TestRenderScenes_SpringEaseReplacesHardBack(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if strings.Contains(html, "back.out(2)") {
		t.Errorf("caption key-word pop still uses back.out(2) — over the 1.7 doctrine ceiling")
	}
	for _, want := range []string{"function springEase", "STAT_SETTLE"} {
		if !strings.Contains(html, want) {
			t.Errorf("emitted JS missing %q", want)
		}
	}
}
```

- [ ] **Step 2: รันให้เห็นว่า fail**

Run: `go test ./internal/producer/ -run TestRenderScenes_SpringEaseReplacesHardBack -v`
Expected: FAIL

- [ ] **Step 3: แก้ template** — 3 จุด:

3.1 เพิ่ม helper ถัดจากฟังก์ชัน `kpiCountUp` (ก่อนคอมเมนต์ `// ── per-scene: ONE entrance per element`):

```js
      // springEase — damped-spring position curve baked into a seek-safe GSAP
      // ease. Deterministic: fixed-step scan, no Math.random / Date.now.
      // response ≈ seconds per oscillation; dampingFraction 1 = no overshoot,
      // 0.85 ≈ iOS register (~1% overshoot, felt not seen). Overshooting eases
      // go on transforms only — never opacity (it would push past 1).
      function springEase(opts){
        const response=(opts&&opts.response)||0.5;
        const z=(opts&&opts.dampingFraction!=null)?opts.dampingFraction:1;
        const w=(2*Math.PI)/response;
        let pos;
        if(z<1){
          const wd=w*Math.sqrt(1-z*z);
          pos=function(t){return 1-Math.exp(-z*w*t)*(Math.cos(wd*t)+((z*w)/wd)*Math.sin(wd*t));};
        }else if(z>1){
          const wo=w*Math.sqrt(z*z-1);
          pos=function(t){return 1-Math.exp(-z*w*t)*(Math.cosh(wo*t)+((z*w)/wo)*Math.sinh(wo*t));};
        }else{
          pos=function(t){return 1-Math.exp(-w*t)*(1+w*t);};
        }
        const EPS=0.001;
        const rate=z<=1?z*w:(z-Math.sqrt(z*z-1))*w;
        const SCAN=12/rate, N=4800;
        let T=SCAN;
        for(let i=N;i>=0;i--){
          const t=(i/N)*SCAN;
          if(Math.abs(1-pos(t))>EPS){T=((i+1)/N)*SCAN;break;}
        }
        const xT=pos(T);
        return {duration:T,ease:function(p){return pos(p*T)+p*(1-xT);}};
      }
      const STAT_SETTLE=springEase({response:0.35,dampingFraction:0.85});
```

3.2 การ์ด stat:

old:
```js
              tl.fromTo(kids[j],{scale:0.78,opacity:0},{scale:1,opacity:1,duration:0.5,ease:"back.out(1.7)"},sc.start+0.28+j*0.08);
```
new:
```js
              tl.fromTo(kids[j],{scale:0.78},{scale:1,duration:STAT_SETTLE.duration,ease:STAT_SETTLE.ease},sc.start+0.28+j*0.08);
              tl.fromTo(kids[j],{opacity:0},{opacity:1,duration:0.3,ease:"power2.out"},sc.start+0.28+j*0.08);
```

3.3 คำเน้น caption:

old:
```js
            {scale:1.14,textShadow:"0 0 20px "+KEY_ACCENT+"E6",duration:0.24,ease:"back.out(2)"},inAt+0.10);
```
new:
```js
            {scale:1.14,textShadow:"0 0 20px "+KEY_ACCENT+"E6",duration:0.24,ease:"back.out(1.7)"},inAt+0.10);
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/producer/ -run TestRenderScenes_SpringEaseReplacesHardBack -v` → PASS
แล้ว `go test ./internal/producer/ -count=1` → PASS

- [ ] **Step 5: Commit**

```bash
git add internal/producer/templates/layout_multi_scene.html.tmpl internal/producer/composition_scenes_test.go
git commit -m "feat(motion): springEase แทน back.out การ์ด stat + ลด back.out(2) ของ caption เหลือ 1.7"
```

---

### Task 5: ตรวจของจริง (lint + inspect + render MP4) + simplify + ส่งให้เจ้าของดู

**Files:** ไม่แก้โค้ดใหม่ (ยกเว้นผลจาก /simplify)

**Interfaces:**
- Consumes: test harness ที่มีอยู่ `TestManualRenderMultiScene` (HF_RENDER=1, รัน lint+inspect) และ `TestRenderSampleA1A4` (HF_SAMPLE=1 + HF_OUT, render MP4 จริง)

- [ ] **Step 1: รันเทสต์ทั้ง repo**

Run: `go test ./... -count=1`
Expected: PASS ทั้งหมด

- [ ] **Step 2: รันด่าน lint+inspect กับ template ใหม่**

Run: `HF_RENDER=1 HF_ASPECT=9:16 go test ./internal/producer/ -run TestManualRenderMultiScene -v -timeout 25m`
Expected: PASS, lint ไม่มี error ใหม่, inspect ไม่ flag เพิ่มจากเดิม
Gotcha: ถ้า `npx` ตาย EPERM (เคยเจอในเครื่องนี้) ให้ `npm install -g hyperframes@0.6.70` แล้วรันใหม่ — CLI global จะถูกเรียกก่อน npx

- [ ] **Step 3: render MP4 ตัวอย่างจริง**

Run: `HF_SAMPLE=1 HF_OUT=$TMPDIR/motion_phase1.mp4 go test ./internal/producer/ -run TestRenderSampleA1A4 -v -timeout 30m`
Expected: ได้ไฟล์ MP4

- [ ] **Step 4: ดึงเฟรมรอบรอยต่อมาตรวจด้วยตา** (รอยต่อฉาก 1→2 อยู่ที่ t=5s ใน sample)

```bash
ffmpeg -y -i $TMPDIR/motion_phase1.mp4 -vf "select='eq(n\,114)+eq(n\,120)+eq(n\,126)'" -fps_mode passthrough $TMPDIR/seam_%d.png
```
(fps 24 → เฟรม 114≈4.75s, 120=5.0s, 126≈5.25s) แล้วเปิดภาพตรวจ:
- 4.75s: ฉาก 1 กำลังเลื่อนซ้าย + จางลง (ไม่ใช่นิ่งๆ จางเฉยๆ)
- 5.0s: จุดตัด — ฉาก 2 โผล่ค่อนไปทางขวา ยังไม่ทึบเต็ม
- 5.25s: ฉาก 2 เกือบเข้าที่ ไม่มีแฟลชขาว/เฟรมว่าง
ถ้าไม่เป็นตามนี้ → กลับไปไล่ Task 3 ก่อน commit อะไรเพิ่ม

- [ ] **Step 5: รัน /simplify กับ diff ทั้ง branch** (ข้อตกลงกับเจ้าของ: simplify ก่อนปิดงานเสมอ)

เรียก skill `simplify` ให้รีวิว `git diff master...HEAD` แล้ว apply การลดรูปที่ผ่านเกณฑ์ · รัน `go test ./internal/producer/ -count=1` ซ้ำหลัง apply

- [ ] **Step 6: Commit ปิดท้าย (ถ้า simplify มีแก้) + ส่งวิดีโอให้เจ้าของ**

```bash
git add -A
git commit -m "chore(motion): เก็บงานตาม /simplify"
```
ส่ง `$TMPDIR/motion_phase1.mp4` + ภาพเฟรมรอยต่อให้เจ้าของดูเป็นหลักฐานก่อนตัดสินใจ merge — **ห้าม merge เข้า master เองโดยเจ้าของยังไม่เห็นวิดีโอ** (คลิปนี้คือหน้าตาของทุกคลิปที่ระบบจะผลิตอัตโนมัติ)

---

## หมายเหตุการตัดสินใจ (สำหรับคนตรวจงาน)

- **ไม่ใส่ blur ที่ seam**: ตำราบอก optional; blur เต็มเฟรม 1080×1920 บน CPU render (3 workers, เคยเจอ protocolTimeout) เสี่ยงไม่คุ้ม
- **เลื่อนทั้ง wrapper ไม่ใช่เฉพาะ content**: ลูกๆ ของ content เริ่ม opacity 0 (stagger ทีหลัง) — เลื่อนเฉพาะ content จะได้ "carrier ล่องหน" ที่ไม่มีอะไรให้ตามอง เลื่อนทั้ง slab ให้ bg เป็น carrier แทน
- **legacy branch (AUDIO_MOTION ปิด) ไม่แตะขาเข้า**: ได้ inAt ใหม่ + ขาออกใหม่ฟรี แต่ยัง fade เข้าแบบเดิม — branch นี้ไม่ได้รันบน prod
- **บั๊ก data-start แก้ด้วยการเลิก pre-start** ไม่ใช่เลื่อน data-start เร็วขึ้น เพราะ cut-the-curve เป็น hard cut ที่รอยต่อพอดี ไม่ต้องการหน้าต่างซ้อนอีกต่อไป — ทางที่ง่ายกว่าและลบเงื่อนไขพิเศษ
