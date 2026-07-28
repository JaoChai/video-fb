package producer

// ChatPreset is the visual identity of the chat format: a customer's message
// thread. Palette stays Brand (navy + orange) — only the mood changes. Motion is
// quick and light: a chat bubble should pop in, not glide cinematically.
var ChatPreset = StylePreset{
	Key:         "chat",
	DisplayName: "Customer Chat",
	Palette:     Brand,
	ImageAnchor: "Cinematic editorial PHOTOGRAPHY, shot on 50mm, a real person at night lit mainly by " +
		"the blue glow of a phone screen against a deep-navy #0047AF ambience, one warm amber #F0A030 " +
		"practical light in the background. Candid, worried, human. Photorealistic, premium, restrained. " +
		"NO illustration, NO 3D render, NO cartoon, no text, no user interface, no readable screen content. " +
		"Atmosphere: the message you send when the money already stopped moving.",
	Font:        TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	HeadingFont: TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	Motion:      MotionProfile{EntranceDur: 0.32, EntranceEase: "power2.out", BGZoomTo: 1.03},
}

// buildChatCoverPrompt renders the image prompt for the ONE AI image a chat clip
// gets: the opening portrait. Composition reserves the lower half for the hook
// copy; every later scene is a CSS message thread and gets no image.
func buildChatCoverPrompt(concept string, preset StylePreset, clipToken string) string {
	return buildImagePromptCore(concept,
		"cinematic portrait, the subject placed in the UPPER half of the frame, lower half dark and uncluttered",
		preset, clipToken)
}
