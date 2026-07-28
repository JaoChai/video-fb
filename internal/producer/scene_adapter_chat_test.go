package producer

import (
	"encoding/json"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// เทสต์ชุดนี้วิ่งผ่าน buildSceneContent ซึ่งเป็นชั้นที่แปลง JSON ของโมเดลเข้าเป็น
// SceneContent — ชั้นที่เทสต์เรนเดอร์มองไม่เห็นเพราะมันป้อน SceneContent ตรงๆ
//
// บั๊กจริงบน prod 2026-07-28: โมเดลส่ง msgs/asker/verdict มาครบทุกซีน แต่ struct
// ที่รับ JSON ไม่มีสามช่องนี้ ข้อมูลจึงถูกทิ้งทั้งหมดตั้งแต่ต้นทาง ซีน chat_out
// กลายเป็นซีนว่างแล้วถูก fallback เป็น hero ส่วน chat_in รอดจาก fallback เพราะ
// บังเอิญมี stamp ที่ struct เดิมรู้จัก แต่ก็เรนเดอร์ออกมาเป็นซีนเปล่า
func chatScene(layout, contentJSON string) agent.GeneratedScene {
	return agent.GeneratedScene{
		SceneNumber:  2,
		Layout:       layout,
		OnScreenText: "ข้อความสำรอง",
		Content:      json.RawMessage(contentJSON),
	}
}

func TestBuildSceneContent_ChatOutKeepsMessages(t *testing.T) {
	c := buildSceneContent(chatScene("chat_out",
		`{"msgs":[{"from":"me","t":"อย่าเพิ่งยื่นอุทธรณ์ครับ"},{"from":"them","t":"แล้วทำไงดี"}],`+
			`"verdict":"รอ 72 ชั่วโมงแล้วยื่นครั้งเดียว"}`), sceneBound{Start: 0, End: 5})

	if c.Layout != "chat_out" {
		t.Fatalf("Layout = %q, want chat_out — ซีนถูก fallback เป็น hero ทั้งที่มีข้อมูลครบ", c.Layout)
	}
	if len(c.Msgs) != 2 {
		t.Fatalf("len(Msgs) = %d, want 2 — ฟองข้อความหายระหว่างแปลง JSON", len(c.Msgs))
	}
	if c.Msgs[0].From != "me" || c.Msgs[1].From != "them" {
		t.Errorf("ฝั่งของฟองผิด: %+v", c.Msgs)
	}
	if c.Verdict == "" {
		t.Error("Verdict หาย — การ์ดสรุปเขียวจะไม่ขึ้น")
	}
}

func TestBuildSceneContent_ChatInKeepsHeader(t *testing.T) {
	c := buildSceneContent(chatScene("chat_in",
		`{"asker":"คุณเก่ง","stamp":"21:47 น.","msgs":[{"from":"them","t":"บัญชีโดนแบน","alert":true}]}`),
		sceneBound{Start: 0, End: 5})

	if c.Asker != "คุณเก่ง" {
		t.Errorf("Asker = %q, want คุณเก่ง — หัวแชทจะไม่ขึ้น", c.Asker)
	}
	if len(c.Msgs) != 1 || !c.Msgs[0].Alert {
		t.Errorf("ธง alert หาย: %+v", c.Msgs)
	}
}

// from ที่โมเดลเขียนไม่ตรงสเปกต้องตกฝั่งลูกค้า ไม่ใช่หายไปทั้งฟอง
func TestBuildSceneContent_UnknownSenderFallsToThem(t *testing.T) {
	c := buildSceneContent(chatScene("chat_out",
		`{"msgs":[{"from":"customer","t":"ข้อความจากคนถาม"},{"from":"me","t":"คำตอบ"}]}`),
		sceneBound{Start: 0, End: 5})

	if len(c.Msgs) != 2 {
		t.Fatalf("len(Msgs) = %d, want 2", len(c.Msgs))
	}
	if c.Msgs[0].From != "them" {
		t.Errorf("from=%q ควรตกเป็น them", c.Msgs[0].From)
	}
}

// KPI ที่วิ่งผิดทางต้องได้ธง bad ติดมาถึง SceneContent ไม่งั้นเลขแดงหายทั้งโหมด
func TestBuildSceneContent_DashboardKeepsBadChip(t *testing.T) {
	c := buildSceneContent(chatScene("dashboard",
		`{"statLabel":"CPM 7 วัน","chips":[{"n":"+38%","t":"CPM","bad":true},{"n":"1.4x","t":"ROAS"}],`+
			`"callout":"แพงขึ้น 38%"}`), sceneBound{Start: 0, End: 5})

	if c.Layout != "dashboard" {
		t.Fatalf("Layout = %q, want dashboard", c.Layout)
	}
	if len(c.Chips) != 2 {
		t.Fatalf("len(Chips) = %d, want 2", len(c.Chips))
	}
	if !c.Chips[0].Bad || c.Chips[1].Bad {
		t.Errorf("ธง bad ของชิปผิด: %+v", c.Chips)
	}
}

// ตาข่ายกันลืม: ทุก layout ของทุกโหมดต้องผ่าน adapter โดยไม่ถูกตัดสินว่าว่าง
// ถ้ามีใครเพิ่มโหมดใหม่แล้วลืมอัปเดต empty check เทสต์นี้จะจับได้ทันที
func TestBuildSceneContent_NoLayoutSilentlyDegradesToHero(t *testing.T) {
	for _, tc := range []struct{ layout, content string }{
		{"hook", `{"rows":[{"t":"บรรทัดเดียว"}]}`},
		{"chat_in", `{"asker":"คุณเก่ง","msgs":[{"from":"them","t":"ถาม"}]}`},
		{"chat_out", `{"msgs":[{"from":"me","t":"ตอบ"}]}`},
		{"recap", `{"title":"สรุป","chips":[{"n":"3","t":"ข้อ"}]}`},
		{"dashboard", `{"statLabel":"CPM","chips":[{"n":"+38%","t":"CPM"}]}`},
		{"alarm", `{"title":"เตือน","rows":[{"t":"ทำอะไร"}]}`},
		{"verdict", `{"title":"สรุปคดี","stamp":"ปิดคดี"}`},
		{"uistep", `{"num":"1","of":"ขั้นที่ 1 / 2","panel":{"chrome":"Ads Manager"}}`},
	} {
		c := buildSceneContent(chatScene(tc.layout, tc.content), sceneBound{Start: 0, End: 5})
		if c.Layout != tc.layout {
			t.Errorf("layout %q ถูกลดเป็น %q — adapter มองไม่เห็นช่องข้อมูลของมัน", tc.layout, c.Layout)
		}
	}
}
