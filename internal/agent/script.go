package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/rag"
)

type ScriptTemplateData struct {
	Question          string
	QuestionerName    string
	Category          string
	RAGContext        string
	FormatInstruction string
	AudiencePersona   string
}

type ScriptAgent struct {
	llm      *LLMClient
	rag      *rag.Engine
	research *ResearchAgent
}

func NewScriptAgent(llm *LLMClient, ragEngine *rag.Engine, research *ResearchAgent) *ScriptAgent {
	return &ScriptAgent{llm: llm, rag: ragEngine, research: research}
}

type GeneratedScene struct {
	SceneNumber     int             `json:"scene_number"`
	SceneType       string          `json:"scene_type"`
	TextContent     string          `json:"text_content"`
	VoiceText       string          `json:"voice_text"`
	BgHint          string          `json:"bg_hint"`
	DurationSeconds float64         `json:"duration_seconds"`
	TextOverlays    json.RawMessage `json:"text_overlays"`
}

type GeneratedScript struct {
	Scenes             []GeneratedScene `json:"scenes"`
	TotalDuration      float64          `json:"total_duration_seconds"`
	YoutubeTitle       string           `json:"youtube_title"`
	YoutubeDescription string           `json:"youtube_description"`
	YoutubeTags        []string         `json:"youtube_tags"`
}

func (a *ScriptAgent) Generate(ctx context.Context, question, questionerName, category string, format *models.ContentFormat, persona string, cfg *models.AgentConfig) (*GeneratedScript, error) {
	var ragContext strings.Builder

	if format.FormatName == "news" {
		// News format: research the specific story for accurate, current facts
		researchContext, err := a.research.Research(ctx, question)
		if err != nil {
			log.Printf("ScriptAgent: research failed, continuing with KB only: %v", err)
		}
		if researchContext != "" {
			ragContext.WriteString(researchContext)
			ragContext.WriteString("\n---\n")
		}
	}

	// Business knowledge + brand context from the hand-written Thai KB (all formats)
	ragResults, err := a.rag.Search(ctx, question, 5)
	if err != nil {
		return nil, fmt.Errorf("RAG search: %w", err)
	}
	for _, r := range ragResults {
		ragContext.WriteString(r.Content)
		ragContext.WriteString("\n---\n")
	}

	userPrompt, err := renderTemplate(cfg.PromptTemplate, ScriptTemplateData{
		Question:          question,
		QuestionerName:    questionerName,
		Category:          category,
		RAGContext:        ragContext.String(),
		FormatInstruction: format.ScriptInstruction,
		AudiencePersona:   persona,
	})
	if err != nil {
		return nil, fmt.Errorf("render script template: %w", err)
	}

	var script GeneratedScript
	if err := a.llm.GenerateJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, &script); err != nil {
		return nil, fmt.Errorf("generate script: %w", err)
	}
	return &script, nil
}

// Scene types the composition agent maps to layout variants downstream.
const (
	SceneHook    = "hook"
	SceneProblem = "problem"
	SceneStep    = "step"
	SceneWin     = "win"
	SceneCTA     = "cta"
)

// Scene-count bounds. maxScenes caps Normalize(); minScenes is the floor the
// orchestrator guard enforces (wired in a later task).
const (
	minScenes = 1
	maxScenes = 6
)

var validSceneTypes = map[string]bool{
	SceneHook: true, SceneProblem: true, SceneStep: true, SceneWin: true, SceneCTA: true,
}

// Normalize keeps an LLM-produced script safe for the pipeline: caps the scene
// count, renumbers scenes 1..N in arrival order, and defaults any unrecognized
// scene_type to SceneStep. It does not fabricate scenes (0-scene is the caller's
// error to handle).
func (s *GeneratedScript) Normalize() {
	if len(s.Scenes) > maxScenes {
		s.Scenes = s.Scenes[:maxScenes]
	}
	for i := range s.Scenes {
		s.Scenes[i].SceneNumber = i + 1
		if !validSceneTypes[s.Scenes[i].SceneType] {
			s.Scenes[i].SceneType = SceneStep
		}
	}
}
