package producer

import (
	"encoding/json"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func uistepScene(n int, content string) agent.GeneratedScene {
	return agent.GeneratedScene{SceneNumber: n, Layout: "uistep", OnScreenText: "fallback",
		Content: json.RawMessage(content)}
}

func TestBuildSceneContentUIStep(t *testing.T) {
	s := uistepScene(3, `{"num":"2","of":"ขั้นที่ 2 / 3","title":"ตั้งเงื่อนไขให้ตัดงบ",
		"panel":{"chrome":"Ads Manager","breadcrumb":"Rules › Create new rule",
			"items":[{"label":"Campaigns","state":"normal"},{"label":"Ad sets","state":"done"},
			         {"label":"Cost per result","state":"target"}],
			"field":{"label":"Greater than","value":"400 THB"}},
		"callout":"เลือก Cost per result แล้วใส่ 400 บาท"}`)
	c := buildSceneContent(s, sceneBound{Start: 9, End: 17})

	if c.Layout != "uistep" {
		t.Fatalf("layout = %q, want uistep (must NOT fall back to hero)", c.Layout)
	}
	if c.Panel == nil {
		t.Fatal("panel not parsed")
	}
	if c.Panel.Chrome != "Ads Manager" || c.Panel.Breadcrumb != "Rules › Create new rule" {
		t.Errorf("panel chrome/breadcrumb wrong: %+v", c.Panel)
	}
	if len(c.Panel.Items) != 3 || c.Panel.Items[1].State != "done" || c.Panel.Items[2].State != "target" {
		t.Errorf("panel items wrong: %+v", c.Panel.Items)
	}
	if c.Panel.Field == nil || c.Panel.Field.Value != "400 THB" {
		t.Errorf("panel field wrong: %+v", c.Panel.Field)
	}
	if c.Callout != "เลือก Cost per result แล้วใส่ 400 บาท" || c.Num != "2" || c.Of != "ขั้นที่ 2 / 3" {
		t.Errorf("rail/callout wrong: num=%q of=%q callout=%q", c.Num, c.Of, c.Callout)
	}
}

func TestBuildSceneContentUIStepClampsState(t *testing.T) {
	s := uistepScene(3, `{"panel":{"items":[{"label":"Rules","state":"highlighted"}]}}`)
	c := buildSceneContent(s, sceneBound{Start: 0, End: 4})
	if c.Panel == nil || len(c.Panel.Items) != 1 {
		t.Fatalf("panel not parsed: %+v", c.Panel)
	}
	if c.Panel.Items[0].State != "normal" {
		t.Errorf("unknown state = %q, want normal", c.Panel.Items[0].State)
	}
}

func TestBuildSceneContentUIStepCapsItemsAndLengths(t *testing.T) {
	s := uistepScene(3, `{"title":"ชื่อขั้นที่ยาวเกินสามสิบสี่ตัวอักษรแน่นอนเพราะพิมพ์ยาวมากจริงๆนะ",
		"callout":"คำอธิบายที่ยาวมากเกินหกสิบตัวอักษรแน่นอนเพราะเขียนบรรยายยืดยาวเกินกรอบที่ออกแบบไว้จริงๆ",
		"panel":{"items":[{"label":"a"},{"label":"b"},{"label":"c"},{"label":"d"},{"label":"e"},{"label":"f"},{"label":"g"}]}}`)
	c := buildSceneContent(s, sceneBound{Start: 0, End: 4})
	if c.Panel == nil {
		t.Fatal("panel not parsed")
	}
	if len(c.Panel.Items) != 5 {
		t.Errorf("items = %d, want capped at 5", len(c.Panel.Items))
	}
	if n := len([]rune(c.Title)); n > 34 {
		t.Errorf("title = %d runes, want <= 34", n)
	}
	if n := len([]rune(c.Callout)); n > 60 {
		t.Errorf("callout = %d runes, want <= 60", n)
	}
}

func TestBuildSceneSpecsFillsStepTotal(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "hook", Content: json.RawMessage(`{"rows":[{"t":"x"}]}`)},
		uistepScene(2, `{"num":"1","panel":{"items":[{"label":"Rules"}]}}`),
		uistepScene(3, `{"num":"2","panel":{"items":[{"label":"Rules"}]}}`),
		uistepScene(4, `{"num":"3","panel":{"items":[{"label":"Rules"}]}}`),
		{SceneNumber: 5, Layout: "cta", Content: json.RawMessage(`{"title":"จบ","cta":"ทักมา","brand":"ADS VANCE"}`)},
	}
	bounds := []sceneBound{{0, 3}, {3, 8}, {8, 13}, {13, 18}, {18, 24}}
	specs := buildSceneSpecs(scenes, bounds)
	for _, sp := range specs {
		if sp.Content.Layout == "uistep" && sp.Content.StepTotal != 3 {
			t.Errorf("scene %d StepTotal = %d, want 3", sp.SceneNumber, sp.Content.StepTotal)
		}
		if sp.Content.Layout != "uistep" && sp.Content.StepTotal != 0 {
			t.Errorf("non-uistep scene %d must not carry StepTotal", sp.SceneNumber)
		}
	}
}

// field ที่มี label แต่ไม่มี value = กล่องส้มเปล่าที่ซ้ำชื่อแถวด้านบน สอนอะไรไม่ได้
// (เจอจากคลิปจริงตัวแรกบน prod 2026-07-25 ขั้นที่ 1)
func TestBuildSceneContentUIStepDropsValuelessField(t *testing.T) {
	s := uistepScene(3, `{"panel":{"items":[{"label":"Selected audience","state":"target"}],
		"field":{"label":"Selected audience","value":""}}}`)
	c := buildSceneContent(s, sceneBound{Start: 0, End: 4})
	if c.Panel == nil {
		t.Fatal("panel not parsed")
	}
	if c.Panel.Field != nil {
		t.Errorf("field with an empty value must be dropped, got %+v", c.Panel.Field)
	}
}

func TestBuildSceneContentUIStepKeepsFieldWithValue(t *testing.T) {
	s := uistepScene(3, `{"panel":{"items":[{"label":"Overlap","state":"target"}],
		"field":{"label":"Overlap","value":"เกิน 30 เปอร์เซ็นต์"}}}`)
	c := buildSceneContent(s, sceneBound{Start: 0, End: 4})
	if c.Panel.Field == nil || c.Panel.Field.Value != "เกิน 30 เปอร์เซ็นต์" {
		t.Errorf("a field with a real value must survive, got %+v", c.Panel.Field)
	}
}
