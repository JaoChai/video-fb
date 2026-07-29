package producer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func warroomParams() ScenesParams {
	mk := func(n int, layout string, c SceneContent) SceneSpec {
		c.SceneNumber, c.Layout = n, layout
		c.Start, c.End = float64(n-1)*5, float64(n)*5
		return SceneSpec{SceneNumber: n, StartSec: c.Start, EndSec: c.End,
			LayoutVariant: "hook_big", CaptionStyle: "phrase_block", Content: c}
	}
	return ScenesParams{
		AspectRatio: "9:16", BrandName: "ADS VANCE", VoiceSrc: "assets/voice.wav",
		// prod รัน SCENE_MOTION_V2_ENABLED=true — fixture ที่ปิดไว้ทำให้ด่านสายตา
		// มองไม่เห็นตัวเลข KPI ที่นับขึ้น (เกือบปล่อยโค้ดที่ไม่เคยถูกรันขึ้น prod)
		MotionV2:        true,
		DurationSeconds: 30, Format: "warroom", ThemeKey: "warroom",
		Scenes: []SceneSpec{
			mk(1, "dashboard", SceneContent{
				StatLabel:       "CPM 7 วันล่าสุด",
				Chips:           []ContentChip{{N: "+38%", T: "CPM", Bad: true}, {N: "1.4x", T: "ROAS"}},
				Callout:         "ค่าโฆษณาแพงขึ้น 38% ใน 3 วัน",
				BackgroundImage: "assets/bg-scene1.png"}),
			mk(2, "uistep", SceneContent{
				Num: "1", Of: "ขั้นที่ 1 / 2", StepTotal: 2, Title: "เปิดกฎอัตโนมัติ",
				Panel: &ContentUIPanel{
					Chrome: "Ads Manager", Breadcrumb: "Rules › Create new rule",
					Items: []ContentUIItem{
						{Label: "Campaigns", State: "normal"},
						{Label: "Automated Rules", State: "target"},
					},
					Field: &ContentUIField{Label: "Greater than", Value: "400 THB"},
				},
				Callout: "เลือก Automated Rules"}),
			mk(3, "alarm", SceneContent{Title: "ตั้งเพดานไว้ก่อนนอน",
				Rows: []ContentRow{{Text: "CPM ทะลุเพดาน = หยุดแอดอัตโนมัติ"},
					{Text: "ไม่ตั้ง = จ่ายทั้งคืน", Bad: true}}}),
		},
		Segments: []TranscriptSegment{{Text: "ค่าโฆษณาแพงขึ้น", Start: 0, End: 2}},
	}
}

func TestRenderWarRoomFormat(t *testing.T) {
	out, err := RenderCompositionScenes(warroomParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	for _, want := range []string{
		`data-format="warroom"`,
		"const FORMAT_WARROOM = true",
		// เดิมเทสต์นี้หา `data-layout="dashboard"` ซึ่ง "ผ่าน" เพราะไปแมตช์ CSS selector
		// ไม่ใช่ markup — data-layout ถูกเซ็ตตอน runtime ด้วย JS (setAttribute) ไม่เคย
		// อยู่ใน HTML ที่ render ออกมา · ลบกฎ CSS ที่ไม่ได้ใช้เมื่อไรเทสต์ก็แดงทั้งที่
		// ตัวซีนไม่ได้พัง จึงย้ายมาตรวจ payload ที่ผู้เล่นจริงอ่าน
		`"type":"dashboard"`,
		`"type":"alarm"`,
		// assert บน payload ของ SCENES ไม่ใช่ชื่อ class — CSS ถูกฝังในทุกคลิป
		// ทุกโหมดอยู่แล้ว การหา "wr-kpi" จึงผ่านแม้ renderer จะถูกลบทิ้ง
		`"statLabel":"CPM 7 วันล่าสุด"`,
		`"callout":"ค่าโฆษณาแพงขึ้น 38% ใน 3 วัน"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered warroom HTML missing %q", want)
		}
	}
	for _, branch := range []string{`sc.type==="dashboard"`, `sc.type==="alarm"`,
		`el("div","wr-kpi"+(ch.bad?" bad":""))`} {
		if !strings.Contains(html, branch) {
			t.Errorf("JS renderer branch missing: %s — war-room scenes would render empty", branch)
		}
	}
}

// หัวใจของสเปก: ห้องควบคุมต้องยังเรนเดอร์ uistep ด้วย markup เดิม เพราะตะแกรง
// ui_vocab กับตัวนับขั้นตอนนับเฉพาะซีน uistep — ถ้าหายไปคลิปสอนผิดจะหลุดขึ้นช่อง
func TestWarRoomStillRendersUIStep(t *testing.T) {
	out, _ := RenderCompositionScenes(warroomParams())
	html := string(out)
	for _, want := range []string{`data-layout="uistep"`, `"label":"Automated Rules"`,
		`"state":"target"`, `"stepTotal":2`} {
		if !strings.Contains(html, want) {
			t.Errorf("warroom must keep the uistep walkthrough intact — missing %q", want)
		}
	}
	// โค้ดที่วาดแผงจำลองกับป้าย "กดตรงนี้" ต้องยังอยู่ — ถ้าใครลบ branch uistep ทิ้ง
	// payload ข้างบนยังผ่านหมดเพราะมันมาจาก params ของเทสต์เอง
	for _, branch := range []string{`sc.type==="uistep"`, `el("div","ui-point"`, `el("div","ui-tap")`} {
		if !strings.Contains(html, branch) {
			t.Errorf("uistep renderer gone: %s — teaching steps would render blank", branch)
		}
	}
}

// KPI ที่แย่ต้องมีธง bad ติดไปถึง JSON ไม่งั้นเลขแดงหายทั้งโหมด
func TestWarRoomKPICarriesBadFlag(t *testing.T) {
	out, _ := RenderCompositionScenes(warroomParams())
	// ต้องเจาะจงถึงชิป ไม่ใช่หา "bad":true ลอยๆ — ซีน alarm ก็มี row ที่ bad:true
	// เหมือนกัน เทสต์แบบกว้างจึงผ่านแม้ ContentChip.Bad จะถูกลบทิ้งทั้งฟิลด์
	if !strings.Contains(string(out), `"n":"+38%","t":"CPM","bad":true`) {
		t.Error("SCENES JSON must carry the bad flag on the failing KPI chip itself")
	}
}

// กราฟในการ์ด gauge ต้องเป็นเส้นที่คำนวณจากข้อความของซีน ไม่ใช่รูปทรงคงที่
// เดิมมันคือ clip-path polygon ชุดเดียว ทุกคลิปที่เคยผลิตจึงได้ยอดเขาเหมือนกันเป๊ะ
// จนดูเหมือนภาพประกอบที่ยังโหลดไม่เสร็จ · seed ต้องมาจาก statLabel+callout เพื่อให้
// เรนเดอร์ซ้ำคลิปเดิม (retry/resume) ได้เส้นเดิม ไม่ใช่สุ่มใหม่ทุกครั้ง
func TestWarRoomSparkIsARealSeededChart(t *testing.T) {
	html := assertRenderContains(t, warroomParams())
	for _, want := range []string{
		"function sparkChart(",
		`g.appendChild(sparkChart((sc.statLabel||"")+"|"+(sc.callout||"")+"|"+i))`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("war-room spark must be the seeded SVG chart — missing %q", want)
		}
	}
	if strings.Contains(html, "clip-path:polygon(0 76%") {
		t.Error("the fixed clip-path silhouette is back — every clip would draw the same graph")
	}
}

// เลข KPI ต้องนับขึ้น และต้องนับได้ทั้งที่มีเครื่องหมายติดหัว/ท้าย ("+38%", "1.4x")
// parseStatNumber เดิมรับเฉพาะตัวเลขล้วน ถ้าใครเอา kpiCountUp ออกแล้วไปเรียกมันตรงๆ
// KPI ของห้องควบคุมจะไม่ขยับสักตัว โดยที่ไม่มีเทสต์ไหนฟ้อง
func TestWarRoomKPICountsUp(t *testing.T) {
	html := assertRenderContains(t, warroomParams())
	for _, want := range []string{
		"function kpiCountUp(",
		`kpiCountUp(sceneEl, sc.start+0.3, span)`,
		`match(/^(\D*)([\d.,]+)(\D*)$/)`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("war-room KPI count-up not wired — missing %q", want)
		}
	}
	// คลิปที่ไม่มีเฟรมปกต้องไม่มีคำว่า COVER ใน output เลย (invariant ของ COVER_SCENE)
	// การ์ดนี้จึงต้องเชื่อมด้วย {{if .Cover}} ไม่ใช่เช็คตอน runtime
	noCover := warroomParams()
	noCover.Cover = false
	assertRenderNotContains(t, noCover, "COVER")
}

// TestDumpWarRoomHTML — ด่านสายตาเดียวกับฝั่งแชท ดูว่ากรอบจอมอนิเตอร์ไม่ทับ uistep
func TestDumpWarRoomHTML(t *testing.T) {
	dir := os.Getenv("HF_KEEP_DIR")
	if dir == "" {
		t.Skip("set HF_KEEP_DIR=<dir> to dump the war-room HTML for eyeballing")
	}
	out, err := RenderCompositionScenes(warroomParams())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "warroom.html"), out, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Logf("wrote %s/warroom.html", dir)
}
