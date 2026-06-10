# Scene + Metadata Agents (Plan 2b-2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the two new topic-pipeline agents — `SceneAgent` (Claude: script → 6-10 scene JSON) and `MetadataAgent` (Gemini: script → YouTube title/desc/tags) — as standalone, tested Go code that compiles into the `agent` package, without wiring them into the orchestrator yet.

**Architecture:** Pure additive change, mirroring the existing `ScriptAgent`/`ImageAgent` shape. Each new agent holds a `*KieLLMClient`, receives its `*models.AgentConfig` as a call parameter (the orchestrator fetches it via `GetByName` in Plan 2b-3), renders the DB-seeded `prompt_template` via the existing `renderTemplate`, and calls `GenerateJSON`. The DB rows (`scene` Claude, `metadata` Gemini) were already seeded by migration 030 (Plan 2b-1), and the `GeneratedScene` struct already carries the new fields. Nothing in `main.go`, the orchestrator, or the producer changes here, so master stays green.

**Tech Stack:** Go 1.25, `KieLLMClient` (kie.ai Claude/Gemini, prefix-routed), `renderTemplate` (struct-field text substitution), `GenerateJSON` (fence-stripped JSON unmarshal with temperature-retry).

**Scope:** Two new agent files + two test files inside `internal/agent/`. No orchestrator, no `main.go`, no migration (030 already seeded `scene` + `metadata`), no frontend. The `image`→`imageprompt` rename, `question` removal, topic-driven orchestrator flow, per-scene TTS/render, and frontend alignment are deliberately deferred to Plan 2b-3.

---

## File Structure

- **Create:** `internal/agent/scene.go` — `SceneAgent` (Claude). Input: script text + target scene count/duration + brand theme + its `AgentConfig`. Output: `[]GeneratedScene` (the existing struct, new fields populated). Single responsibility: script → constrained scene array.
- **Create:** `internal/agent/scene_test.go` — guards the prompt↔struct contract (the seeded `scene` prompt's JSON schema must unmarshal into `GeneratedScene`) and the theme-description helper.
- **Create:** `internal/agent/metadata.go` — `MetadataAgent` (Gemini). Input: topic + script + category + persona + its `AgentConfig`. Output: `*GeneratedMetadata`. Single responsibility: script → YouTube metadata.
- **Create:** `internal/agent/metadata_test.go` — guards the metadata prompt↔struct contract and that all four template vars render.

Both agents reuse package-level helpers already present in `internal/agent/`: `renderTemplate` (`template.go`), `safeStr` (`image.go`), and `KieLLMClient.GenerateJSON` (`kiellm.go`). The `GeneratedScene` struct lives in `script.go` and already has `LayoutVariant`, `OnScreenText`, `EmphasisWords []string`, `Beat`, `CaptionStyle`, `ImagePrompt` (added in Plan 2b-1).

---

## Reference: what migration 030 seeded (do not re-create — read-only context)

The `scene` agent row (`agent_configs`): `model = 'claude-sonnet-4-6'`, prompt template asks for a **JSON array** where each object has `scene_number, beat, voice_text, on_screen_text, emphasis_words, layout_variant, caption_style, duration_seconds, image_prompt`, and uses template vars `{{.TargetDurationSec}}`, `{{.Script}}`, `{{.ThemeDescription}}`.

The `metadata` agent row: `model = 'gemini-3-5-flash'`, prompt template asks for a **JSON object** with `youtube_title, youtube_description, youtube_tags`, and uses template vars `{{.Topic}}`, `{{.Category}}`, `{{.Script}}`, `{{.AudiencePersona}}`.

Per design §4.6 the canonical scene template vars are `Script, TargetSceneCount, TargetDurationSec, ThemeDescription` and metadata vars are `Topic, Script, Category, AudiencePersona`. The template-data structs below expose all of these (so a user can later add `{{.TargetSceneCount}}` to the seeded template without a code change), even though the seed text doesn't reference every one.

---

## Task 1: SceneAgent — script → scene JSON (Claude)

**Files:**
- Create: `internal/agent/scene.go`
- Test: `internal/agent/scene_test.go`

The agent is a thin wrapper: build a Thai brand-theme description from `*models.BrandTheme`, render the seeded template, call `GenerateJSON` into `[]GeneratedScene`. The pure, testable seams are (a) the theme-description helper and (b) the contract that the seeded prompt's JSON schema unmarshals into `GeneratedScene`.

- [ ] **Step 1: Write the failing tests**

Create `internal/agent/scene_test.go`:

```go
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

// The seeded `scene` prompt (migration 030) asks for a JSON array of objects with
// these exact fields. This test locks the prompt↔struct contract: if the JSON the
// LLM is told to emit ever stops unmarshalling into GeneratedScene, the field tags
// drifted and the pipeline would silently lose scene data.
func TestSceneOutputParsesSeededSchema(t *testing.T) {
	raw := `[
	  {
	    "scene_number": 1,
	    "beat": "hook",
	    "voice_text": "คุณรู้ไหมว่าบัญชีโฆษณาโดนแบนได้ใน 3 วินาที",
	    "on_screen_text": "โดนแบนใน 3 วิ?",
	    "emphasis_words": ["แบน", "3 วินาที"],
	    "layout_variant": "hook_big",
	    "caption_style": "word_pop",
	    "duration_seconds": 4.5,
	    "image_prompt": "dark navy gradient, orange accent"
	  },
	  {
	    "scene_number": 2,
	    "beat": "payoff",
	    "voice_text": "วิธีกันไว้ก่อนคือแยกบัญชีสำรอง",
	    "on_screen_text": "แยกบัญชีสำรอง",
	    "emphasis_words": ["สำรอง"],
	    "layout_variant": "phrase_block",
	    "caption_style": "phrase_block",
	    "duration_seconds": 6,
	    "image_prompt": ""
	  }
	]`

	var scenes []GeneratedScene
	if err := json.Unmarshal([]byte(raw), &scenes); err != nil {
		t.Fatalf("seeded scene JSON did not unmarshal into []GeneratedScene: %v", err)
	}
	if len(scenes) != 2 {
		t.Fatalf("len(scenes) = %d, want 2", len(scenes))
	}
	s0 := scenes[0]
	if s0.SceneNumber != 1 {
		t.Errorf("SceneNumber = %d, want 1", s0.SceneNumber)
	}
	if s0.Beat != "hook" {
		t.Errorf("Beat = %q, want hook", s0.Beat)
	}
	if s0.LayoutVariant != "hook_big" {
		t.Errorf("LayoutVariant = %q, want hook_big", s0.LayoutVariant)
	}
	if s0.CaptionStyle != "word_pop" {
		t.Errorf("CaptionStyle = %q, want word_pop", s0.CaptionStyle)
	}
	if s0.OnScreenText != "โดนแบนใน 3 วิ?" {
		t.Errorf("OnScreenText = %q", s0.OnScreenText)
	}
	if len(s0.EmphasisWords) != 2 || s0.EmphasisWords[0] != "แบน" {
		t.Errorf("EmphasisWords = %v, want [แบน 3 วินาที]", s0.EmphasisWords)
	}
	if s0.DurationSeconds != 4.5 {
		t.Errorf("DurationSeconds = %v, want 4.5", s0.DurationSeconds)
	}
	if s0.VoiceText == "" {
		t.Errorf("VoiceText is empty")
	}
}

func TestBuildSceneThemeDescription(t *testing.T) {
	style := "flat illustration"
	theme := &models.BrandTheme{
		PrimaryColor: "#0f1d35",
		AccentColor:  "#ff6b2b",
		ImageStyle:   &style,
	}
	got := buildSceneThemeDescription(theme)
	if !strings.Contains(got, "#0f1d35") || !strings.Contains(got, "#ff6b2b") {
		t.Errorf("description missing brand colors: %q", got)
	}
	if !strings.Contains(got, "flat illustration") {
		t.Errorf("description missing image style: %q", got)
	}
}

// Renders a stand-in template with the four registry vars and confirms each is
// substituted — guards SceneTemplateData field names against the §4.6 registry.
func TestSceneTemplateRendersRegistryVars(t *testing.T) {
	tmpl := "dur={{.TargetDurationSec}} count={{.TargetSceneCount}} script={{.Script}} theme={{.ThemeDescription}}"
	out, err := renderTemplate(tmpl, SceneTemplateData{
		Script:            "SCRIPT",
		TargetSceneCount:  8,
		TargetDurationSec: 75,
		ThemeDescription:  "THEME",
	})
	if err != nil {
		t.Fatalf("renderTemplate err: %v", err)
	}
	for _, want := range []string{"dur=75", "count=8", "script=SCRIPT", "theme=THEME"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q: %s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/agent/ -run 'TestSceneOutputParsesSeededSchema|TestBuildSceneThemeDescription|TestSceneTemplateRendersRegistryVars' -v`
Expected: FAIL — compile error `undefined: buildSceneThemeDescription` and `undefined: SceneTemplateData` (the parse test alone would pass, but the package won't compile until the helper + struct exist).

(If the go-build cache errors with "operation not permitted", that is a sandbox restriction — rerun with sandbox disabled.)

- [ ] **Step 3: Write the SceneAgent**

Create `internal/agent/scene.go`:

```go
package agent

import (
	"context"
	"fmt"

	"github.com/jaochai/video-fb/internal/models"
)

// SceneTemplateData fills the seeded `scene` prompt template. Field names match
// the design §4.6 registry exactly (renderTemplate substitutes {{.FieldName}}).
type SceneTemplateData struct {
	Script            string
	TargetSceneCount  int
	TargetDurationSec int
	ThemeDescription  string
}

// SceneAgent is the Director: it breaks a finished script into 6-10 constrained
// scenes for the 9:16 hyperframes template. Runs on Claude (cfg.Model is
// claude-sonnet-4-6, routed by KieLLMClient prefix). Output fields map onto the
// new scenes columns added in Plan 2b-1.
type SceneAgent struct {
	llm *KieLLMClient
}

func NewSceneAgent(llm *KieLLMClient) *SceneAgent {
	return &SceneAgent{llm: llm}
}

// Generate turns a script into an ordered scene array. cfg is the `scene`
// AgentConfig (fetched by the caller via GetByName). targetSceneCount and
// targetDurationSec steer length; theme supplies brand styling for image_prompt.
func (a *SceneAgent) Generate(ctx context.Context, script string, targetSceneCount, targetDurationSec int, theme *models.BrandTheme, cfg *models.AgentConfig) ([]GeneratedScene, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, SceneTemplateData{
		Script:            script,
		TargetSceneCount:  targetSceneCount,
		TargetDurationSec: targetDurationSec,
		ThemeDescription:  buildSceneThemeDescription(theme),
	})
	if err != nil {
		return nil, fmt.Errorf("render scene template: %w", err)
	}

	var scenes []GeneratedScene
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &scenes); err != nil {
		return nil, fmt.Errorf("generate scenes: %w", err)
	}
	return scenes, nil
}

// buildSceneThemeDescription renders a short Thai brand summary for the Director,
// so its draft image_prompt and tone stay on-brand. Pure — testable.
func buildSceneThemeDescription(theme *models.BrandTheme) string {
	return fmt.Sprintf("แบรนด์ Ads Vance: navy %s + ส้ม %s, มาสคอตเสือดาว. สไตล์ภาพ: %s",
		theme.PrimaryColor, theme.AccentColor, safeStr(theme.ImageStyle))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/agent/ -run 'TestSceneOutputParsesSeededSchema|TestBuildSceneThemeDescription|TestSceneTemplateRendersRegistryVars' -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/agent/scene.go internal/agent/scene_test.go
git commit -m "feat(agent): add SceneAgent — script to scene JSON (Claude)"
```

---

## Task 2: MetadataAgent — script → YouTube metadata (Gemini)

**Files:**
- Create: `internal/agent/metadata.go`
- Test: `internal/agent/metadata_test.go`

Same thin-wrapper shape, Gemini side. The seeded `metadata` prompt returns a JSON **object** (not array), so the output type is a single struct.

- [ ] **Step 1: Write the failing tests**

Create `internal/agent/metadata_test.go`:

```go
package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// Locks the prompt↔struct contract for the seeded `metadata` prompt (migration
// 030): it must emit youtube_title / youtube_description / youtube_tags, which
// must unmarshal into GeneratedMetadata.
func TestMetadataOutputParsesSeededSchema(t *testing.T) {
	raw := `{
	  "youtube_title": "บัญชีโฆษณาโดนแบน แก้ยังไง",
	  "youtube_description": "วิธีกันบัญชีโฆษณาโดนแบน และสิ่งที่ต้องทำก่อนสาย",
	  "youtube_tags": ["บัญชีโฆษณา", "โดนแบน", "facebook ads", "ยิงแอด"]
	}`

	var md GeneratedMetadata
	if err := json.Unmarshal([]byte(raw), &md); err != nil {
		t.Fatalf("seeded metadata JSON did not unmarshal into GeneratedMetadata: %v", err)
	}
	if md.YoutubeTitle != "บัญชีโฆษณาโดนแบน แก้ยังไง" {
		t.Errorf("YoutubeTitle = %q", md.YoutubeTitle)
	}
	if md.YoutubeDescription == "" {
		t.Errorf("YoutubeDescription is empty")
	}
	if len(md.YoutubeTags) != 4 || md.YoutubeTags[0] != "บัญชีโฆษณา" {
		t.Errorf("YoutubeTags = %v", md.YoutubeTags)
	}
}

// Guards MetadataTemplateData field names against the §4.6 registry
// (Topic, Script, Category, AudiencePersona).
func TestMetadataTemplateRendersRegistryVars(t *testing.T) {
	tmpl := "topic={{.Topic}} cat={{.Category}} script={{.Script}} persona={{.AudiencePersona}}"
	out, err := renderTemplate(tmpl, MetadataTemplateData{
		Topic:           "TOPIC",
		Script:          "SCRIPT",
		Category:        "CAT",
		AudiencePersona: "PERSONA",
	})
	if err != nil {
		t.Fatalf("renderTemplate err: %v", err)
	}
	for _, want := range []string{"topic=TOPIC", "cat=CAT", "script=SCRIPT", "persona=PERSONA"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q: %s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/agent/ -run 'TestMetadataOutputParsesSeededSchema|TestMetadataTemplateRendersRegistryVars' -v`
Expected: FAIL — compile error `undefined: GeneratedMetadata` and `undefined: MetadataTemplateData`.

- [ ] **Step 3: Write the MetadataAgent**

Create `internal/agent/metadata.go`:

```go
package agent

import (
	"context"
	"fmt"

	"github.com/jaochai/video-fb/internal/models"
)

// MetadataTemplateData fills the seeded `metadata` prompt template. Field names
// match the design §4.6 registry exactly.
type MetadataTemplateData struct {
	Topic           string
	Script          string
	Category        string
	AudiencePersona string
}

// GeneratedMetadata is the search-intent YouTube metadata the MetadataAgent
// produces. The brand suffix (" | Ads Vance") is appended later by the
// orchestrator's validateScript, not requested in the prompt.
type GeneratedMetadata struct {
	YoutubeTitle       string   `json:"youtube_title"`
	YoutubeDescription string   `json:"youtube_description"`
	YoutubeTags        []string `json:"youtube_tags"`
}

// MetadataAgent generates Thai search-intent YouTube metadata from a finished
// script. Runs on Gemini Flash (cfg.Model is gemini-3-5-flash, routed by prefix).
type MetadataAgent struct {
	llm *KieLLMClient
}

func NewMetadataAgent(llm *KieLLMClient) *MetadataAgent {
	return &MetadataAgent{llm: llm}
}

// Generate produces title/description/tags for one clip. cfg is the `metadata`
// AgentConfig (fetched by the caller via GetByName).
func (a *MetadataAgent) Generate(ctx context.Context, topic, script, category, persona string, cfg *models.AgentConfig) (*GeneratedMetadata, error) {
	userPrompt, err := renderTemplate(cfg.PromptTemplate, MetadataTemplateData{
		Topic:           topic,
		Script:          script,
		Category:        category,
		AudiencePersona: persona,
	})
	if err != nil {
		return nil, fmt.Errorf("render metadata template: %w", err)
	}

	var md GeneratedMetadata
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &md); err != nil {
		return nil, fmt.Errorf("generate metadata: %w", err)
	}
	return &md, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/agent/ -run 'TestMetadataOutputParsesSeededSchema|TestMetadataTemplateRendersRegistryVars' -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/agent/metadata.go internal/agent/metadata_test.go
git commit -m "feat(agent): add MetadataAgent — script to YouTube metadata (Gemini)"
```

---

## Task 3: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Build, vet, and test the whole module**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS. The new agents are additive package-level code; no existing call site, test, or struct changed. The new exported constructors (`NewSceneAgent`, `NewMetadataAgent`) being unused outside tests is fine for package-level funcs — they get wired into the orchestrator in Plan 2b-3.

(Sandbox go-build cache error → rerun with sandbox disabled.)

- [ ] **Step 2: Confirm the additive contract held**

Run: `git diff <base>..HEAD --stat` where `<base>` is the commit before Task 1.
Expected: only these files appear — `internal/agent/scene.go`, `internal/agent/scene_test.go`, `internal/agent/metadata.go`, `internal/agent/metadata_test.go`, and this plan doc. No orchestrator, `main.go`, producer, migration, or frontend file should appear.

---

## Self-Review Notes

- **Spec coverage:** Implements the `scene.go` (design §4.2, §4.6 `scene` row) and `metadata.go` (§4.2, §4.6 `metadata` row) agents — the two agents Plan 2b-1 explicitly deferred ("the new SceneAgent/MetadataAgent ... are Plans 2b-2 and 2b-3"). The `image`→`imageprompt` rename, `question` removal, research/script prompt rewrites, the topic-driven orchestrator (§4.5), producer/hyperframes (§4.3/§4.4), and frontend alignment (§4.8) remain in Plan 2b-3 — this increment stays additive and green.
- **No placeholders:** every struct, method, helper, and test is written in full.
- **Type consistency:** `SceneAgent.Generate` returns `[]GeneratedScene` (the struct from `script.go`, whose 2b-1 fields the parse test exercises). `MetadataAgent.Generate` returns `*GeneratedMetadata`, a new struct defined in `metadata.go`. Template-data field names (`SceneTemplateData`, `MetadataTemplateData`) match the §4.6 registry and the migration-030 seed vars. Both agents take `cfg *models.AgentConfig` as a parameter — consistent with `ScriptAgent`/`ImageAgent` (orchestrator-fetched), not the `agentsRepo`-internal pattern of `ResearchAgent`.
- **Reused helpers:** `renderTemplate` (`template.go`), `safeStr` (`image.go`), `KieLLMClient.GenerateJSON` (`kiellm.go`) — no duplication. `GenerateJSON` unmarshals both a top-level array (scene) and object (metadata), as `ImageAgent` already proves for arrays.
- **Honesty flag:** the parse tests assert the prompt↔struct contract against representative JSON, not a live kie.ai call — no LLM is invoked in `go test`. A real end-to-end scene/metadata generation happens when Plan 2b-3 wires these into the orchestrator. The seeded prompts may need real-call tuning then; that is out of scope here.
```