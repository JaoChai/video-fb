package producer

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"testing"
)

func sampleScenesParams(aspect string) ScenesParams {
	return ScenesParams{
		AspectRatio: aspect, BrandName: "ADS VANCE", CategoryLabel: "PIXEL",
		QuestionerName: "คุณป๊อบ", Kicker: "CAPI & PIXEL", VoiceSrc: "assets/voice.wav",
		DurationSeconds: 45,
		IntroMascot:     "assets/mascot/rocket.png",
		OutroMascot:     "assets/mascot/wave.png",
		CTAText:         "กดติดตาม ไม่พลาดทุกอัปเดตแอด",
		Scenes: []SceneSpec{
			{SceneNumber: 1, LayoutVariant: "hook_big", AccentColor: "#F0A030", AnimationSpeed: "normal",
				StartSec: 0, EndSec: 15, BackgroundMode: "css", CaptionStyle: "word_pop",
				Slots:   []SlotSpec{{Role: "headline", HTML: template.HTML("บัญชีโดนแบน")}},
				Content: SceneContent{SceneNumber: 1, Start: 0, End: 15, Layout: "hero", CaptionStyle: "word_pop", Title: "บัญชีโดนแบน"}},
			{SceneNumber: 2, LayoutVariant: "list_steps", AccentColor: "#F0A030", AnimationSpeed: "normal",
				StartSec: 15, EndSec: 32, BackgroundMode: "css",
				MascotPose: "assets/mascot/thumbs_up.png", CaptionStyle: "phrase_block",
				Slots:   []SlotSpec{{Role: "step", HTML: template.HTML("เช็คเวลา UTC"), StepNum: 1}},
				Content: SceneContent{SceneNumber: 2, Start: 15, End: 32, Layout: "step", CaptionStyle: "phrase_block", Num: "1", Of: "ขั้นที่ 1", Title: "เช็คเวลา UTC"}},
			{SceneNumber: 3, LayoutVariant: "quote_cta", AccentColor: "#2fd17a", AnimationSpeed: "normal",
				StartSec: 32, EndSec: 45, BackgroundMode: "css",
				Slots:   []SlotSpec{{Role: "headline", HTML: template.HTML("ทักแอดส์แวนซ์")}},
				Content: SceneContent{SceneNumber: 3, Start: 32, End: 45, Layout: "cta", CaptionStyle: "phrase_block", Title: "ทักแอดส์แวนซ์", CTA: "ทักเลย", Brand: "ADS VANCE"}},
		},
		Segments: []TranscriptSegment{{Text: "บัญชีโดนแบนเพราะอะไร", Start: 0, End: 4}},
	}
}

func TestRenderCompositionScenes_9x16(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	// Thai on-screen text now flows in via the structured ScenesJSON (Content),
	// not the legacy Slots markup; SEGMENTS/SCENES are the in-page builders.
	for _, m := range []string{`data-width="1080"`, `data-height="1920"`, "บัญชีโดนแบน", "เช็คเวลา UTC", "ทักแอดส์แวนซ์", "const SEGMENTS", "const SCENES"} {
		if !strings.Contains(s, m) {
			t.Errorf("output missing %q", m)
		}
	}
	// Go template actions must all be rendered. (The Style-B JS legitimately
	// contains literal "}}" in its arrow-function bodies, so a bare "}}" scan
	// would false-positive; check for an unrendered ".Field" action instead.)
	if strings.Contains(s, "{{.") || strings.Contains(s, "{{if") || strings.Contains(s, "{{end") {
		t.Errorf("unrendered template action")
	}
}

func TestRenderCompositionScenes_16x9(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("16:9"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `data-width="1920"`) || !strings.Contains(s, `data-height="1080"`) {
		t.Errorf("16:9 dimensions wrong")
	}
}

// Style-B has no mascot bumpers; the persistent chrome is the brand/category
// badge row and the in-page scene-wrapper builder. This asserts that chrome.
func TestRenderCompositionScenes_BadgesAndScaffold(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	for _, m := range []string{
		`class="badge-brand">ADS VANCE`, `class="badge-cat">PIXEL`,
		`id="progress"`, `id="capStage"`, `id="badges"`,
		`w.id = "scene-" + sc.scene`, `data-i="`,
	} {
		if !strings.Contains(s, m) {
			t.Errorf("output missing %q", m)
		}
	}
}

func TestRenderCompositionScenes_NoLegacyLiterals(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	legacy := []string{"#0a1428", "#0f1d35", "#16284a", "#ff6b2b",
		"rgba(15, 29, 53", "rgba(15,29,53", "rgba(255, 107, 43", "rgba(255,107,43",
		"rgba(10, 20, 40", "rgba(8, 16, 32"}
	for _, bad := range legacy {
		if strings.Contains(s, bad) {
			t.Errorf("rendered multi-scene HTML still contains legacy literal %q", bad)
		}
	}
}

func TestRenderCompositionScenes_RejectsEmpty(t *testing.T) {
	if _, err := RenderCompositionScenes(ScenesParams{AspectRatio: "9:16", DurationSeconds: 0}); err == nil {
		t.Error("expected error for DurationSeconds<=0")
	}
	if _, err := RenderCompositionScenes(ScenesParams{AspectRatio: "9:16", DurationSeconds: 10}); err == nil {
		t.Error("expected error for empty Scenes")
	}
}

func TestRenderCompositionScenes_CaptionStyleInJSON(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	for _, m := range []string{`"caption_style":"word_pop"`, `"caption_style":"phrase_block"`} {
		if !strings.Contains(s, m) {
			t.Errorf("ScenesJSON missing %q", m)
		}
	}
}

// assertRenderContains renders params and fails for each marker absent from the
// output — shared by the wiring tests below to avoid repeating the scaffold.
func assertRenderContains(t *testing.T, params ScenesParams, markers ...string) string {
	t.Helper()
	out, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	for _, m := range markers {
		if !strings.Contains(s, m) {
			t.Errorf("rendered template missing %q", m)
		}
	}
	return s
}

// assertRenderNotContains renders params and fails if any substring IS present
// in the output — the negative counterpart to assertRenderContains.
func assertRenderNotContains(t *testing.T, params ScenesParams, subs ...string) {
	t.Helper()
	out, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, sub := range subs {
		if strings.Contains(string(out), sub) {
			t.Errorf("expected output NOT to contain %q", sub)
		}
	}
}

// A1 (per-scene entrance speed) + A4 (emphasis word pop/glow): the derived
// Speed must reach the template's SCENES JSON, and the template must carry the
// GSAP wiring that consumes it — SPEED_FACTOR for entrance pacing and a
// palette-driven key-word textShadow tween for the glow (GSAP-only, no keyframes).
func TestRenderCompositionScenes_SpeedAndGlowWiring(t *testing.T) {
	params := sampleScenesParams("9:16")
	params.Scenes[0].Content.Speed = "fast"
	params.Scenes[1].Content.Speed = "slow"
	s := assertRenderContains(t, params,
		`"speed":"fast"`, `"speed":"slow"`, // A1: derived speed serialized into SCENES
		"SPEED_FACTOR", // A1: template consumes it for entrance duration
		"keyEl=s",      // A4: key word captured at creation (no re-query)
		"KEY_ACCENT",   // A4: glow color read from the palette, not hardcoded
		"textShadow",   // A4: glow tween (GSAP, seek-safe)
	)
	// Guard the seek-safety rule the plan committed to: no CSS keyframe animation.
	if strings.Contains(s, "@keyframes") {
		t.Errorf("template contains @keyframes — animations must be GSAP-driven for frame capture")
	}
}

// A2 (per-layout composition): the scene wrapper must expose its layout as a
// data-layout attribute, and the template must carry the per-layout CSS hooks
// that reposition each layout so scenes aren't all centered.
func TestRenderCompositionScenes_LayoutComposition(t *testing.T) {
	assertRenderContains(t, sampleScenesParams("9:16"),
		`w.setAttribute("data-layout", sc.type`, // scene exposes its layout to CSS
		`.scene[data-layout="hook"] .scene-content`, // opener raised
		`.scene[data-layout="step"] .scene-content`, // step left-aligned
		`.scene[data-layout="cta"]  .scene-content`, // cta centered
	)
}

// MOTION_V2: the JS const must reflect the params flag, and the contentEntrance
// helper must always be emitted (it's runtime-gated on MOTION_V2, not compiled
// out), so both on and off renders carry the helper function.
func TestRenderCompositionScenes_MotionV2(t *testing.T) {
	on := sampleScenesParams("9:16")
	on.MotionV2 = true
	assertRenderContains(t, on, "const MOTION_V2 = true", "function contentEntrance")

	off := sampleScenesParams("9:16")
	off.MotionV2 = false
	s := assertRenderContains(t, off, "const MOTION_V2 = false")
	// helper JS is always present (runtime-gated), only the const value differs:
	if !strings.Contains(s, "function contentEntrance") {
		t.Error("contentEntrance helper should always be emitted (runtime-gated by MOTION_V2)")
	}
}

// COVER_SCENE: the JS const must reflect the ScenesParams.Cover flag. (The
// frame-0 cover entrance guard "COVER && idx===0" is added in Task 2 — this
// test only asserts the Task 1 plumbing: the const itself.)
func TestRenderCompositionScenes_Cover(t *testing.T) {
	on := sampleScenesParams("9:16")
	on.Cover = true
	assertRenderContains(t, on, "const COVER = true")

	off := sampleScenesParams("9:16")
	off.Cover = false
	// Cover off: no COVER trace at all — the plan's invariant is byte-identical
	// output, so even the inert `const COVER = false;` must not be emitted.
	assertRenderNotContains(t, off, "const COVER")

	// Cover on: scene 0 is pinned visible at t=0 (no opacity:0 fade on the poster frame).
	assertRenderContains(t, on, "tl.set(sceneEl,{opacity:1},0)")
	assertRenderNotContains(t, off, "tl.set(sceneEl,{opacity:1},0)")

	// Cover on: the scene-set gate AND the per-child entrance skip for scene 0
	// must both ship — without the skip, the hook text stays opacity:0 at
	// frame 0 even though the container is pinned visible.
	assertRenderContains(t, on, "if(COVER && idx===0){", "!(COVER && idx===0)")

	// Cover off: the child-skip guard must not leak in, and the children
	// loop condition must remain byte-identical to the pre-cover template.
	assertRenderNotContains(t, off, "!(COVER && idx===0)")
	assertRenderContains(t, off, "if(content){")

	// Code-review fix #2: cover scene 0's background must still ken-burns —
	// without this the opener's bg sits dead-still while every other scene's
	// bg tweens.
	assertRenderContains(t, on, `tl.fromTo(bg,{scale:1.04},{scale:BG_ZOOM_TO,duration:span,ease:"none"},0)`)

	// Cover on: the ADS VANCE badge is pinned visible at t=0 so the poster
	// frame carries the brand; off keeps the 0.3s slide-in exactly as before.
	// BOTH layers must move: the hyperframes clip gate (data-start) hides the
	// element until its start time no matter what GSAP does, so cover needs
	// data-start="0" AND the GSAP pin.
	assertRenderContains(t, on, `tl.set("#badges",{opacity:1,x:0},0)`)
	assertRenderContains(t, on, `id="badges" class="clip badge-row" data-start="0"`)
	assertRenderNotContains(t, on, `tl.fromTo("#badges"`)
	assertRenderContains(t, off, `tl.fromTo("#badges",{opacity:0,x:-40},{opacity:1,x:0,duration:.6,ease:"power3.out"},0.3)`)
	assertRenderContains(t, off, `id="badges" class="clip badge-row" data-start="0.3"`)
	assertRenderNotContains(t, off, `tl.set("#badges"`)
	assertRenderNotContains(t, off, `tl.fromTo(bg,{scale:1.04},{scale:BG_ZOOM_TO,duration:span,ease:"none"},0)`)

	// Code-review fix #1: cover scene-0 stat must pin its final value at t=0
	// with no 0→final count-up tween (poster-frame flash guard).
	assertRenderContains(t, on, `COVER && idx===0 && sc.type==="stat"`)
	assertRenderNotContains(t, off, `COVER && idx===0 && sc.type==="stat"`)
}

func TestRenderCompositionScenes_CountUp(t *testing.T) {
	p := sampleScenesParams("9:16")
	p.MotionV2 = true
	assertRenderContains(t, p, "function parseStatNumber", "stat-num")
}

// TestRenderCompositionScenes_CountUpSkipsSuffixedStat guards the fix for a
// review finding: parseStatNumber used to match the LEADING number even when
// the stat string had a non-numeric suffix (e.g. "80%"), so the count-up span
// silently dropped the suffix. The regex must now be anchored to match only
// when the ENTIRE trimmed string is numeric, so a suffixed stat like "80%"
// falls back to the static render (full string, suffix intact) instead of
// being torn apart into a counting "80" + a vanished "%".
func TestRenderCompositionScenes_CountUpSkipsSuffixedStat(t *testing.T) {
	p := sampleScenesParams("9:16")
	p.MotionV2 = true
	p.Scenes[0].Content.Layout = "stat"
	p.Scenes[0].Content.Stat = "80%"
	s := assertRenderContains(t, p,
		`"stat":"80%"`, // suffixed value passed through to SCENES JSON verbatim
	)
	// The anchored regex literal must ship in the template's JS source — this
	// is what actually prevents "80%" from matching as a leading-number stat.
	if !strings.Contains(s, `/^\s*([\d.,]+)\s*$/`) {
		t.Errorf("parseStatNumber regex is not anchored to the full string — suffixed stats like %q would have their suffix dropped by the count-up span", "80%")
	}
}

// All presets share Palette: Brand, so palette alone never varies by format
// (see presets.go). This asserts the wiring that must hold regardless: the
// rendered --font-heading value comes from the preset, not a hardcoded default.
func TestRenderCompositionScenes_UsesPresetPalette(t *testing.T) {
	preset := PresetByKey("case-file")
	params := ScenesParams{
		AspectRatio:     "9:16",
		BrandName:       BrandName,
		BrandCSS:        preset.BrandCSS(),
		Palette:         preset.Palette,
		VoiceSrc:        "assets/voice.wav",
		DurationSeconds: 6,
		Scenes:          []SceneSpec{{Content: SceneContent{}}},
	}
	html, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := fmt.Sprintf(`--font-heading: "%s"`, preset.HeadingFont.HeadingFamily)
	if !strings.Contains(string(html), want) {
		t.Errorf("rendered HTML missing preset heading font %q", want)
	}
}

func TestRenderCompositionScenes_StyleB(t *testing.T) {
	params := ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", CategoryLabel: "การเงิน",
		VoiceSrc: "assets/voice.wav", DurationSeconds: 10,
		Scenes: []SceneSpec{
			{SceneNumber: 1, StartSec: 0, EndSec: 5, BackgroundMode: "image", BackgroundImage: "assets/bg-scene1.png",
				Content: SceneContent{SceneNumber: 1, Start: 0, End: 5, Layout: "stat", Stat: "2026", StatLabel: "ปีบังคับใช้",
					Chips: []ContentChip{{N: "90%", T: "ยังไม่รองรับ"}}}},
			{SceneNumber: 2, StartSec: 5, EndSec: 10, BackgroundMode: "css",
				Content: SceneContent{SceneNumber: 2, Start: 5, End: 10, Layout: "step", Num: "1", Of: "ขั้นที่ 1",
					Title: "โทรธนาคาร", Rows: []ContentRow{{Text: "ขอเปิดต่างประเทศ"}}}},
		},
		Segments: []TranscriptSegment{{Text: "ทดสอบ", Start: 0, End: 10}},
	}
	out, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{`const SCENES =`, `"type":"stat"`, `"stat":"2026"`, `"type":"step"`, `letter-spacing:0`, `--amber`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, emo := range []string{"❌", "✅", "📞", "💳", "🛡️", "👇", "⏰"} {
		if strings.Contains(html, emo) {
			t.Errorf("emoji leaked: %q", emo)
		}
	}
}

// TestRenderScenes_InjectsThemeKeyAndHeadingFont confirms the template wires
// ThemeKey/Motion through to the rendered HTML: the data-theme attribute
// (drives per-theme texture CSS), the --font-heading var (drives display
// font), and the motion consts derived from the preset's MotionProfile.
func TestRenderScenes_InjectsThemeKeyAndHeadingFont(t *testing.T) {
	preset := PresetByKey("case-file")
	params := sampleScenesParams("9:16")
	params.ThemeKey = preset.Key
	params.Motion = preset.Motion
	params.BrandCSS = preset.BrandCSS()

	out, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{fmt.Sprintf(`data-theme="%s"`, preset.Key), "--font-heading", "ENTRANCE_EASE"} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
	if !strings.Contains(html, fmt.Sprintf(`"%s"`, preset.Motion.EntranceEase)) {
		t.Errorf("rendered HTML missing intact ease string %q", preset.Motion.EntranceEase)
	}
}

// TestRenderScenes_ParenthesizedEaseSurvivesEscaping guards against html/template
// mangling a GSAP ease string that itself contains parens — "back.out(1.6)" is
// the exact shape the review flagged as risky for JS-string escaping inside a Go
// template. No shipped preset uses it today, so the ease is fed in directly.
func TestRenderScenes_ParenthesizedEaseSurvivesEscaping(t *testing.T) {
	preset := PresetByKey("tutorial")
	params := sampleScenesParams("9:16")
	params.ThemeKey = preset.Key
	params.Motion = preset.Motion
	params.Motion.EntranceEase = "back.out(1.6)"
	params.BrandCSS = preset.BrandCSS()

	out, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if !strings.Contains(html, "back.out(1.6)") {
		t.Errorf("rendered HTML missing intact ease string %q (escaping regression?)", "back.out(1.6)")
	}
}

// TestRenderScenes_FlagOffPinsDefaultBGZoom pins the flag-off/no-caller motion
// path to today's live bg ken-burns zoom (1.10). Without ThemeKey/Motion set,
// composition.go falls back to MotionDefault; this guards against future
// silent drift away from the live value.
func TestRenderScenes_FlagOffPinsDefaultBGZoom(t *testing.T) {
	params := sampleScenesParams("9:16")

	out, err := RenderCompositionScenes(params)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	// html/template's JS-context escaper pads numeric literals with extra
	// spaces (e.g. "=  1.1  ||"), so match loosely on the token rather than
	// an exact "= 1.1" substring.
	bgZoomRe := regexp.MustCompile(`BG_ZOOM_TO\s*=\s*1\.1\b`)
	if !bgZoomRe.MatchString(html) {
		t.Errorf("rendered HTML missing default BG_ZOOM_TO = 1.1 (want %v); got motion default drift", MotionDefault.BGZoomTo)
	}
}

// The badge-cat pill next to the ADS VANCE brand pill must not render as an
// empty capsule when CategoryLabel is unset — which is the case for every
// real production clip (producer.go only sets BrandName).
func TestRenderScenes_EmptyCategoryLabelHidesPill(t *testing.T) {
	p := sampleScenesParams("9:16")
	p.CategoryLabel = ""
	assertRenderNotContains(t, p, `<div class="badge-cat">`)

	withLabel := sampleScenesParams("9:16")
	assertRenderContains(t, withLabel, `<div class="badge-cat">PIXEL</div>`)
}

// The ken-burns zoom scales .scene-bg past the 1080x1920 frame, which grows the
// scene's scrollHeight by a few px. `hyperframes inspect` reads that as
// clipped_text on the scene itself whenever it samples the ~0.1s window where
// the scene has faded in but its content has not (the content is the only child
// with text, so an invisible content makes the SCENE the text candidate). That
// cost a real production clip a needs_review. Keeping the bg inside its own
// clipping wrapper contains the overflow so no ancestor's scrollHeight grows.
func TestRenderScenes_BackgroundIsWrappedInClippingLayer(t *testing.T) {
	out, err := RenderCompositionScenes(sampleScenesParams("9:16"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if !strings.Contains(html, ".scene-bg-wrap{position:absolute;inset:0;overflow:hidden}") {
		t.Error("missing .scene-bg-wrap clipping rule — ken-burns overflow will reach the scene")
	}
	// the scene shell is assembled by the in-page builder, so assert on the
	// builder's own source strings rather than on final markup.
	for _, want := range []string{
		`'<div class="scene-bg-wrap"><div class="scene-bg" data-layout-allow-overflow'`,
		`'></div></div>' +`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("scene background markup missing %q", want)
		}
	}
}

// TestRenderScenes_SlideDirectionIsConstant pins the motion-doctrine "current":
// the slide entrance must always come from the right (+x) — never alternate
// per scene parity (ping-pong reads as amateur per the motion doctrine).
func TestRenderScenes_SlideDirectionIsConstant(t *testing.T) {
	p := sampleScenesParams("9:16")
	assertRenderNotContains(t, p, "sc.scene % 2")
	assertRenderContains(t, p, `v==="slide"){ return {from:{x:80`)
}

// TestRenderScenes_NoIdleDrift: the motion doctrine bans slow ambient drift
// ("video is waiting") — the MOTION_V2 post-entrance content drift must be
// gone from the emitted JS.
func TestRenderScenes_NoIdleDrift(t *testing.T) {
	assertRenderNotContains(t, sampleScenesParams("9:16"), "driftDur", "driftStart")
}

// TestRenderScenes_CutTheCurveSeam pins the new scene seam: the first tween
// lines up with data-start exactly (no hidden pre-start window — the old
// sc.start-0.35 ran while the framework still hid the clip), and the
// velocity-matched leftward exit/entry pair must be present.
func TestRenderScenes_CutTheCurveSeam(t *testing.T) {
	assertRenderContains(t, sampleScenesParams("9:16"),
		"const inAt=sc.start", "SEAM_DX", "SEAM_DUR", `"power4.in"`, `"power4.out"`)
}

// TestRenderScenes_SpringEaseReplacesHardBack: back.out(2) exceeds the
// doctrine ceiling (1.7); stat cards move to a baked damped-spring ease
// (deterministic, seek-safe) with opacity split onto its own tween.
func TestRenderScenes_SpringEaseReplacesHardBack(t *testing.T) {
	p := sampleScenesParams("9:16")
	assertRenderNotContains(t, p, "back.out(2)")
	assertRenderContains(t, p, "function springEase", "STAT_SETTLE")
}
