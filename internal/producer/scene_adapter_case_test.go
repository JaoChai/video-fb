package producer

import (
	"encoding/json"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func caseScene(layout string, content string) agent.GeneratedScene {
	return agent.GeneratedScene{SceneNumber: 1, Layout: layout, OnScreenText: "fallback",
		Content: json.RawMessage(content)}
}

func TestBuildSceneContentCasefile(t *testing.T) {
	s := caseScene("casefile", `{"title":"คดีบัญชีฟาร์มปลิวใน 2 วัน",
		"rows":[{"t":"ผู้เสียหาย: มือใหม่ยิงครีม"},{"t":"ความเสียหาย: 30,000 บาท"}],
		"stamp":"ด่วนที่สุด"}`)
	c := buildSceneContent(s, sceneBound{Start: 0, End: 4})
	if c.Layout != "casefile" {
		t.Fatalf("layout = %q, want casefile", c.Layout)
	}
	if c.Title != "คดีบัญชีฟาร์มปลิวใน 2 วัน" || len(c.Rows) != 2 || c.Stamp != "ด่วนที่สุด" {
		t.Errorf("casefile content not parsed: %+v", c)
	}
	if c.Speed != "fast" {
		t.Errorf("casefile speed = %q, want fast", c.Speed)
	}
}

func TestBuildSceneContentComicPanels(t *testing.T) {
	s := caseScene("comic", `{"panels":[
		{"time":"วันที่ 1","t":"เปิดแอด งบ 15,000","quote":"ใบแรกต้องรุ่งแน่"},
		{"time":"คืนวันที่ 2","t":"บัญชีถูกปิด","dark":true}]}`)
	c := buildSceneContent(s, sceneBound{Start: 4, End: 9})
	if len(c.Panels) != 2 {
		t.Fatalf("panels = %d, want 2", len(c.Panels))
	}
	if c.Panels[0].Time != "วันที่ 1" || c.Panels[0].Quote != "ใบแรกต้องรุ่งแน่" {
		t.Errorf("panel[0] = %+v", c.Panels[0])
	}
	if !c.Panels[1].Dark {
		t.Errorf("panel[1].Dark must be true")
	}
}

func TestBuildSceneContentComicPanelCap(t *testing.T) {
	s := caseScene("comic", `{"panels":[{"t":"a"},{"t":"b"},{"t":"c"},{"t":"d"}]}`)
	c := buildSceneContent(s, sceneBound{Start: 0, End: 3})
	if len(c.Panels) != 3 {
		t.Errorf("panels must cap at 3, got %d", len(c.Panels))
	}
}

func TestBuildSceneContentEvidenceAndVerdict(t *testing.T) {
	ev := buildSceneContent(caseScene("evidence",
		`{"kicker":"หลักฐานชิ้นที่ 1","stamp":"REJECTED","sub":"ครีเอทีฟชุดเดิมที่เจนทับ"}`),
		sceneBound{Start: 9, End: 13})
	if ev.Layout != "evidence" || ev.Stamp != "REJECTED" || ev.Kicker == "" {
		t.Errorf("evidence content: %+v", ev)
	}
	vd := buildSceneContent(caseScene("verdict",
		`{"title":"ออกแบบใหม่ + วอร์มบัญชี","stamp":"ปิดคดี - รอดได้","cta":"ส่งเคสมาเลย","brand":"ADS VANCE"}`),
		sceneBound{Start: 13, End: 17})
	if vd.Layout != "verdict" || vd.Stamp == "" || vd.CTA != "ส่งเคสมาเลย" {
		t.Errorf("verdict content: %+v", vd)
	}
	// verdict มี content จริง ต้องไม่หลุดไป hero fallback
	if vd.Title != "ออกแบบใหม่ + วอร์มบัญชี" {
		t.Errorf("verdict title lost: %q", vd.Title)
	}
}

func TestBuildSceneContentStampOnlyNotHeroFallback(t *testing.T) {
	// ซีนที่มีแค่ stamp/panels ต้องไม่ถูกมองว่า "ว่าง" แล้ว degrade เป็น hero
	c := buildSceneContent(caseScene("comic", `{"panels":[{"t":"เหตุการณ์"}]}`),
		sceneBound{Start: 0, End: 3})
	if c.Layout != "comic" {
		t.Errorf("comic with panels degraded to %q", c.Layout)
	}
}
