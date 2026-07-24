package producer

import (
	"os"
	"strings"

	"github.com/jaochai/video-fb/internal/agent"
)

// CaseFormatEnabled reports whether the case-file investigation format is on.
// Off => every code path behaves exactly as before (spec 2026-07-24 §4).
func CaseFormatEnabled() bool { return os.Getenv("CASE_FORMAT_ENABLED") == "true" }

// CaseFilePreset is the visual identity of the case-file format. It is
// deliberately NOT in Presets: the random/weighted pickers must never select
// it — the orchestrator chooses it explicitly when CaseFormatEnabled().
var CaseFilePreset = StylePreset{
	Key:         "case-file",
	DisplayName: "Case File",
	Palette:     Brand,
	ImageAnchor: "Evidence photograph, harsh direct camera flash, slightly desaturated muted tones, " +
		"plain neutral background, single centered subject, shallow shadows, " +
		"documentary forensic feel, photorealistic. No illustration, no 3D render, no text.",
	Font:        TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	HeadingFont: TypeTokens{Family: "Sarabun", HeadingFamily: "Kanit"},
	Motion:      MotionProfile{EntranceDur: 0.42, EntranceEase: "power4.out", BGZoomTo: 1.05},
}

// buildEvidencePrompt renders the image prompt for a case-format "evidence"
// scene: one centered subject shot like a forensic photo. Unlike
// buildScenePrompt it does NOT reserve the lower 45% of the frame — the image
// sits inside a polaroid card, not under a text card.
func buildEvidencePrompt(concept string, preset StylePreset, clipToken string) string {
	return buildImagePromptCore(concept,
		"single subject centered, plain background, generous margins on all sides",
		preset, clipToken)
}

// promptForScene picks the image prompt for one scene: the classic scene
// prompt, or the evidence-photo prompt in case format. Single owner of this
// choice so the fast (parallel) and sequential image paths can never drift.
func promptForScene(s agent.GeneratedScene, preset StylePreset, clipToken string, caseEnabled bool) string {
	if caseEnabled {
		return buildEvidencePrompt(s.ImagePrompt, preset, clipToken)
	}
	return buildScenePrompt(s.ImagePrompt, "9:16", preset, clipToken)
}

// CaseInfo carries the case-file production context down the producer path.
// Zero value = classic format (byte-identical to today's output).
type CaseInfo struct {
	Enabled    bool
	CaseNumber int // 0 = unknown; the template then omits the case number
}

// evidenceImageScenes returns the scene numbers eligible for AI image
// generation in case format: evidence-layout scenes only, capped at 2
// (spec §6). Returns nil in classic mode = no restriction.
func evidenceImageScenes(scenes []agent.GeneratedScene, caseEnabled bool) map[int]bool {
	if !caseEnabled {
		return nil
	}
	allowed := map[int]bool{}
	for _, s := range scenes {
		if len(allowed) >= 2 {
			break
		}
		if agent.ClampLayout(s.Layout) == "evidence" && strings.TrimSpace(s.ImagePrompt) != "" {
			allowed[s.SceneNumber] = true
		}
	}
	return allowed
}
