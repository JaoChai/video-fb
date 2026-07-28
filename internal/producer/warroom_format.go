package producer

// WarRoomPreset is the visual identity of the war-room format: monitors, live
// numbers, warning lights. It is the advanced tutorial's skin — the uistep
// layout still carries every teaching step, only the frame around it changes,
// so the ui_vocab gate keeps working untouched.
var WarRoomPreset = StylePreset{
	Key:         "warroom",
	DisplayName: "War Room",
	Palette:     Brand,
	ImageAnchor: "Cinematic editorial PHOTOGRAPHY, shot on 35mm, a dim night workspace with several " +
		"monitors glowing deep-navy #0047AF blue, one warm amber #F0A030 desk lamp, cables and a " +
		"coffee cup. Photorealistic, technical, premium. NO illustration, NO 3D render, NO cartoon, " +
		"no text, no logos, no readable screen content — screens are pure abstract glow. " +
		"Atmosphere: someone is watching the numbers move at 2am.",
	Font:        TypeTokens{Family: "Prompt", HeadingFamily: "Kanit"},
	HeadingFont: TypeTokens{Family: "Prompt", HeadingFamily: "Kanit"},
	Motion:      MotionProfile{EntranceDur: 0.26, EntranceEase: "power4.out", BGZoomTo: 1.03},
}

// buildWarRoomCoverPrompt renders the image prompt for the ONE AI image a
// war-room clip gets: the opening establishing shot. Every later scene is a CSS
// monitor and gets no image — a photo there would fight the menu the viewer has
// to read.
func buildWarRoomCoverPrompt(concept string, preset StylePreset, clipToken string) string {
	return buildImagePromptCore(concept,
		"cinematic wide shot, the key subject placed in the UPPER half of the frame, lower half dark and uncluttered",
		preset, clipToken)
}
