package producer

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

// preset ของ myth ต้องหาได้จาก key ที่เก็บในคลิป ไม่งั้นคลิปเก่าที่ retry จะกลับมา
// เป็นแฟ้มคดี (ค่า fallback) แล้ว CSS ทั้งชุดไม่ทำงาน
func TestPresetByKeyResolvesMyth(t *testing.T) {
	if got := PresetByKey(MythPreset.Key); got.Key != "factcheck" {
		t.Errorf("PresetByKey(%q).Key = %q", MythPreset.Key, got.Key)
	}
}

// ภาพปกของ myth ต้องเป็นโต๊ะตรวจเอกสารกลางวัน ไม่ใช่โต๊ะนักสืบกลางคืนของแฟ้มคดี —
// ถ้าซ้ำกัน ช่องจะมีคลิปสองแบบที่หน้าตาเหมือนกันคนละชื่อ
func TestMythImageAnchorIsDaylightDesk(t *testing.T) {
	a := strings.ToLower(MythPreset.ImageAnchor)
	for _, want := range []string{"daylight", "magnifier"} {
		if !strings.Contains(a, want) {
			t.Errorf("MythPreset.ImageAnchor ขาดคำ %q", want)
		}
	}
	if strings.Contains(a, "at night") {
		t.Error("MythPreset.ImageAnchor ไม่ควรเป็นภาพกลางคืน (ชนกับ case-file/warroom)")
	}
}

// โหมด myth ได้ภาพ AI ใบเดียวเท่านั้น (ซีนปก) ซีนที่เหลือเป็น CSS — ภาพในซีน
// การ์ดจะแย่งความสนใจจากข้อความที่คนดูต้องอ่าน
func TestMythAllowsOneImageScene(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, Layout: "hook", ImagePrompt: "a desk"},
		{SceneNumber: 2, Layout: "belief", ImagePrompt: "another"},
		{SceneNumber: 3, Layout: "proof", ImagePrompt: "third"},
	}
	allowed := imageScenesForMode(scenes, ModeMyth)
	if len(allowed) != 1 || !allowed[1] {
		t.Errorf("imageScenesForMode(myth) = %v ต้องอนุญาตแค่ซีน 1", allowed)
	}
}

// prompt ของซีนปก myth ต้องกันพื้นที่ครึ่งล่างให้การ์ด ไม่งั้นการ์ดทับหน้าคนในภาพ
func TestMythCoverPromptReservesLowerHalf(t *testing.T) {
	got := buildMythCoverPrompt("a desk with documents", MythPreset, "tok")
	if !strings.Contains(strings.ToLower(got), "upper half") {
		t.Errorf("buildMythCoverPrompt ไม่ได้สั่งวางวัตถุครึ่งบน: %q", got)
	}
}

// promptForScene ต้องรู้จักโหมด myth ไม่งั้นซีนปกจะได้ prompt แบบ classic
// ที่ไม่กันพื้นที่ให้การ์ดเลย
func TestPromptForSceneRoutesMyth(t *testing.T) {
	s := agent.GeneratedScene{SceneNumber: 1, Layout: "hook", ImagePrompt: "a desk"}
	got := promptForScene(s, MythPreset, "tok", ModeMyth)
	if !strings.Contains(strings.ToLower(got), "upper half") {
		t.Errorf("promptForScene(myth) ไม่ได้ใช้ cover prompt: %q", got)
	}
}
