package producer

// MythPreset คือหน้าตาของโหมดจับความเชื่อผิด: โต๊ะตรวจเอกสารกลางวัน ไม่ใช่โต๊ะ
// นักสืบกลางคืนของแฟ้มคดีและไม่ใช่จอมอนิเตอร์ของห้องควบคุม — สามโหมดนี้ต้องแยกออก
// จากกันด้วยสายตาภายในวินาทีแรก palette ยังเป็น Brand (navy+orange) เหมือนทุกโหมด
// เพราะช่องต้องอ่านเป็นแบรนด์เดียว
var MythPreset = StylePreset{
	Key:         "factcheck",
	DisplayName: "Fact-Check Lab",
	Palette:     Brand,
	ImageAnchor: "Cinematic editorial PHOTOGRAPHY, shot on 50mm, a clean review desk in DAYLIGHT: " +
		"documents and a printed report under a bright desk lamp, a magnifier and a pen resting on top, " +
		"deep-navy #0047AF accents in the room, one warm amber #F0A030 highlight. " +
		"Photorealistic, calm, methodical, premium. NO illustration, NO 3D render, NO cartoon, " +
		"no text, no logos, no readable document content. " +
		"Atmosphere: someone is about to check whether what everyone repeats is actually true.",
	Font:        TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	HeadingFont: TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	// ตรายางกระแทกลงการ์ด = entrance สั้นและแข็ง ไม่ใช่การไถลแบบภาพยนตร์
	Motion: MotionProfile{EntranceDur: 0.24, EntranceEase: "power4.out", BGZoomTo: 1.02},
}

// buildMythCoverPrompt เขียน prompt ของภาพ AI ใบเดียวที่คลิป myth ได้: ซีนเปิด
// วัตถุหลักอยู่ครึ่งบน เพราะการ์ดคำเชื่อกินครึ่งล่างของเฟรม
func buildMythCoverPrompt(concept string, preset StylePreset, clipToken string) string {
	return buildImagePromptCore(concept,
		"cinematic high-angle desk scene in daylight, the key subject placed in the UPPER half of the frame, lower half clean and uncluttered",
		preset, clipToken)
}
