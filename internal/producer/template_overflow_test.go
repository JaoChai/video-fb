package producer

import (
	"strings"
	"testing"
)

// overflowTestParams is the minimal valid input for RenderCompositionScenes.
func overflowTestParams() ScenesParams {
	return ScenesParams{
		AspectRatio:     "9:16",
		DurationSeconds: 10,
		VoiceSrc:        "assets/voice.wav",
		Scenes:          []SceneSpec{{SceneNumber: 1, StartSec: 0, EndSec: 10}},
	}
}

// Kanit/Prompt heading fonts (Design Themes, PR #14) render wider Thai glyphs
// than the Sarabun these px sizes were tuned for. The template must (a) never
// use `overflow-wrap:anywhere` — it cuts Thai mid-word instead of letting
// Chromium's ICU dictionary break at word boundaries — and (b) not narrow the
// stat box below the default 56px gutters.
func TestTemplateThaiWrapRules(t *testing.T) {
	out, err := RenderCompositionScenes(overflowTestParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)

	if strings.Contains(html, "overflow-wrap:anywhere") {
		t.Error("template still uses overflow-wrap:anywhere (cuts Thai mid-word)")
	}
	if strings.Contains(html, `left:110px`) {
		t.Error("stat box is still narrowed to 110px gutters (overflows 230px digits)")
	}
	if !strings.Contains(html, "overflow-wrap:break-word") {
		t.Error("missing overflow-wrap:break-word last-resort rule")
	}
	// Unit must scale with the stat digits (auto-fit shrinks the parent font-size).
	if !strings.Contains(html, ".stat .unit{font-size:.37em") {
		t.Error(".stat .unit is not em-based (won't shrink with auto-fit)")
	}
	// Stat is a number + unit: it must not wrap (auto-fit shrinks it instead).
	if !strings.Contains(html, "white-space:nowrap;font-variant-numeric:tabular-nums") {
		t.Error(".stat must be white-space:nowrap so auto-fit (not wrapping) handles overflow")
	}

	// Every selector newly allowed to wrap must carry a Thai-safe line-height
	// (>=1.25) — wrapped Thai tone marks collide on tight leading.
	for _, rule := range []string{
		".kicker{font-weight:800;font-size:30px;line-height:1.3",
		".step-of{font-weight:700;font-size:30px;line-height:1.3",
		".brandbig{font-weight:800;font-size:88px;line-height:1.3",
	} {
		if !strings.Contains(html, rule) {
			t.Errorf("missing Thai-safe line-height rule: %s", rule)
		}
	}
	if strings.Contains(html, "line-height:1.22") {
		t.Error(".chip .t line-height must be >=1.25 (Thai tone-mark collision)")
	}
}

// The auto-fit pass shrinks nowrap text (.stat, .chip .n) that overflows its
// box — Kanit/Prompt digits+unit run wider than Sarabun. The MOTION_V2 stat
// counts up from "0", so it must be measured at its widest (final) value.
func TestTemplateHasAutoFit(t *testing.T) {
	out, err := RenderCompositionScenes(overflowTestParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if !strings.Contains(html, "function fitText") {
		t.Error("template is missing the fitText auto-fit pass")
	}
	if !strings.Contains(html, `data-final=`) {
		t.Error("stat-num span is missing data-final (auto-fit would measure the count-up '0')")
	}
	if !strings.Contains(html, "document.fonts.ready") {
		t.Error("auto-fit must run after fonts load (fallback-font metrics mis-measure)")
	}
	// The fitText while-condition must survive template rendering intact.
	// html/template treats "-->" inside <script> as an HTML comment close and
	// silently truncates the rest of the line ("guard-->0" did exactly this),
	// producing a JS syntax error that kills the whole inline script — every
	// scene then stays at opacity:0 and the rendered video is blank.
	if !strings.Contains(html, "while(guard>0&&size>min&&node.scrollWidth>node.clientWidth+1)") {
		t.Error("fitText while-condition was mangled by html/template (script would be broken)")
	}
	if strings.Contains(html, "-->0") {
		t.Error(`template contains "-->0" — html/template truncates the line inside <script>`)
	}
}

// The whole inline <script> must be valid JS after template rendering — a
// single syntax error leaves every scene at opacity:0 (blank video). Guard
// against truncation artifacts like a while-condition losing its closing brace.
func TestTemplateScriptNotTruncated(t *testing.T) {
	out, err := RenderCompositionScenes(overflowTestParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, frag := range []string{"while(guard\n", "while(guard\r"} {
		if strings.Contains(html, frag) {
			t.Fatal("inline script is truncated mid-statement (html/template comment handling)")
		}
	}
}
