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

// buildCoverPrompt renders the poster image for the casefile opening scene: a
// moody detective-desk establishing shot. Subject sits in the UPPER half —
// the case folder card overlays the lower half (visual full-frame spec 2026-07-24).
func buildCoverPrompt(concept string, preset StylePreset, clipToken string) string {
	return buildImagePromptCore(concept,
		"cinematic high-angle desk scene at night, the key subject placed in the UPPER half of the frame, lower half dark and uncluttered",
		preset, clipToken)
}

// promptForScene picks the image prompt for one scene. Single owner of this
// choice so the fast (parallel) and sequential image paths can never drift:
// case format routes casefile → cover shot, evidence → forensic photo;
// classic mode keeps the original scene prompt.
func promptForScene(s agent.GeneratedScene, preset StylePreset, clipToken, mode string) string {
	switch mode {
	case ModeCase:
		if agent.ClampLayout(s.Layout) == "casefile" {
			return buildCoverPrompt(s.ImagePrompt, preset, clipToken)
		}
		return buildEvidencePrompt(s.ImagePrompt, preset, clipToken)
	case ModeTutorial:
		return buildTutorialCoverPrompt(s.ImagePrompt, preset, clipToken)
	default:
		return buildScenePrompt(s.ImagePrompt, "9:16", preset, clipToken)
	}
}

// buildTutorialCoverPrompt renders the single cover image of a tutorial clip.
// Replaced with the real art direction in the tutorial-format task.
func buildTutorialCoverPrompt(concept string, preset StylePreset, clipToken string) string {
	return buildScenePrompt(concept, "9:16", preset, clipToken)
}

// Content mode of a clip. Derived per-clip (from clips.content_format), never
// from a process-wide flag alone — so a tutorial clip and a case clip can be
// produced by the same running server.
const (
	ModeClassic  = ""
	ModeCase     = "case"
	ModeTutorial = "tutorial"
)

// FormatInfo carries the per-clip content mode down the producer path.
// Zero value = classic format (byte-identical to today's output).
type FormatInfo struct {
	Mode       string // "" | "case" | "tutorial"
	CaseNumber int    // case mode only; 0 = unknown, template omits the number
}

func (f FormatInfo) IsCase() bool     { return f.Mode == ModeCase }
func (f FormatInfo) IsTutorial() bool { return f.Mode == ModeTutorial }

// imageScenesForMode returns the scene numbers eligible for AI image generation.
// nil = no restriction (classic). Case format: casefile cover + evidence, cap 2.
// Tutorial format: the first scene carrying an image_prompt only, cap 1 — every
// other tutorial scene is an HTML UI mock and must not compete with a photo.
func imageScenesForMode(scenes []agent.GeneratedScene, mode string) map[int]bool {
	switch mode {
	case ModeCase:
		allowed := map[int]bool{}
		for _, s := range scenes {
			if len(allowed) >= 2 {
				break
			}
			layout := agent.ClampLayout(s.Layout)
			if (layout == "casefile" || layout == "evidence") && strings.TrimSpace(s.ImagePrompt) != "" {
				allowed[s.SceneNumber] = true
			}
		}
		return allowed
	case ModeTutorial:
		for _, s := range scenes {
			if strings.TrimSpace(s.ImagePrompt) != "" {
				return map[int]bool{s.SceneNumber: true}
			}
		}
		return map[int]bool{}
	default:
		return nil
	}
}
