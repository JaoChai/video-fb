package producer

// MascotAssetDir is the project-relative directory holding baked mascot PNGs.
const MascotAssetDir = "assets/mascot"

// MascotAssetPath returns the project-relative path to a baked mascot pose PNG.
func MascotAssetPath(pose string) string {
	return MascotAssetDir + "/" + pose + ".png"
}

// mascotPoses is the fixed pose set baked once (offline) via cmd/mascot-gen and
// committed under assets/mascot/<name>.png as transparent PNGs.
var mascotPoses = []string{"rocket", "point_left", "point_right", "thumbs_up", "think", "wave"}

// MascotPoseNames returns a copy of the baked pose name list.
func MascotPoseNames() []string {
	out := make([]string, len(mascotPoses))
	copy(out, mascotPoses)
	return out
}

// poseDirective is the per-pose body/gesture line appended to the shared identity
// prompt when generating the pose via gpt-image-2 image edits.
var poseDirective = map[string]string{
	"rocket":      "riding a small white rocket, leaning forward with energy, thumbs forward",
	"point_left":  "pointing clearly to the left with one paw, confident smile",
	"point_right": "pointing clearly to the right with one paw, confident smile",
	"thumbs_up":   "giving a big thumbs up with one paw, cheerful",
	"think":       "one paw on chin in a thinking pose, curious expression",
	"wave":        "waving hello with one paw, friendly",
}

// MascotEditPrompt builds the gpt-image-2 /edits prompt for one pose. The
// reference image (the brand logo mascot) is sent alongside; this text directs
// the new pose while preserving identity, palette, and a transparent background.
func MascotEditPrompt(pose string) string {
	return "Keep the exact same character: a friendly orange-and-amber cheetah mascot " +
		"wearing a white astronaut suit and a blue ADS VANCE cap, bold outline cartoon style. " +
		"Redraw the same mascot " + poseDirective[pose] + ". " +
		"Royal blue #0047AF and amber #F0A030 brand palette, thick navy outlines. " +
		"Transparent background. Full body, centered, no text, no logo wordmark."
}

// MascotCueToPose maps a composition agent mascot_cue to a baked pose name.
// "point" resolves to point_right by default; "" / "none" mean no mascot.
func MascotCueToPose(cue string) string {
	switch cue {
	case "point":
		return "point_right"
	case "thumbs":
		return "thumbs_up"
	case "think":
		return "think"
	default:
		return ""
	}
}
