package producer

// TutorialPreset is the visual identity of the tutorial format: a clean manual,
// not the case-file investigation. Like CaseFilePreset it is deliberately NOT in
// Presets — the random/weighted pickers must never select it; the orchestrator
// chooses it explicitly for tutorial-mode clips.
//
// Palette stays Brand (navy + orange): the channel must still read as one brand,
// only the mood changes. Motion is fast and crisp — a walkthrough should feel
// efficient, never cinematic.
var TutorialPreset = StylePreset{
	Key:         "tutorial",
	DisplayName: "Tutorial Manual",
	Palette:     Brand,
	ImageAnchor: "Cinematic editorial PHOTOGRAPHY, shot on 35mm, moody low-key lighting with a deep-navy #0047AF wash " +
		"and a single warm amber #F0A030 practical light. Real-world scene of a marketer's workspace at night " +
		"(a phone face-up showing an alert, a laptop glow, an empty desk chair). Photorealistic, premium, restrained. " +
		"NO illustration, NO 3D render, NO cartoon, no text, no user interface, no screens with readable content. " +
		"Atmosphere: the quiet moment before something expensive goes wrong.",
	Font:        TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	HeadingFont: TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	Motion:      MotionProfile{EntranceDur: 0.28, EntranceEase: "power2.out", BGZoomTo: 1.04},
}

// buildTutorialCoverPrompt renders the image prompt for the ONE AI image a
// tutorial clip gets: the opening cover shot. Composition reserves the lower
// half for the hook copy. Every later scene is an HTML UI mock and gets no image
// — a photo there would compete with the menu the viewer must actually read.
func buildTutorialCoverPrompt(concept string, preset StylePreset, clipToken string) string {
	return buildImagePromptCore(concept,
		"cinematic wide shot, the key subject placed in the UPPER half of the frame, lower half dark and uncluttered",
		preset, clipToken)
}
