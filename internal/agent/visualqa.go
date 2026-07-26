package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
	"golang.org/x/sync/errgroup"
)

// QAFrame is one extracted scene frame plus the metadata the model needs to
// judge it (the on-screen text it SHOULD show, and the voice line for context).
type QAFrame struct {
	SceneNumber  int
	PNG          []byte
	OnScreenText string
	VoiceText    string
}

// VisualQAInput is the per-clip QA request: the question (topic context) plus
// one frame per scene.
type VisualQAInput struct {
	Question string
	Frames   []QAFrame
	// Fast judges frames concurrently (cap 4). Set by the orchestrator from
	// producer.PipelineFastEnabled() — this package must stay env-free and
	// cannot import producer (import direction is producer → agent).
	Fast bool
}

// SceneVerdict คือคำตัดสินของโมเดลสำหรับหนึ่งซีน. OK=false คือตำหนิที่มั่นใจว่าจริง
// และต้องบล็อกการเผยแพร่อัตโนมัติ. Issues คือเหตุผลภาษาไทย (เขียนลง visual_qa ด้วย).
// Codes คือรหัสประเภทตำหนิจากชุดปิดที่ prompt กำหนด — ใช้แยกว่าตำหนิไหน "ขึ้นกับ
// จังหวะเวลา" (รอบยืนยันตัดสินซ้ำได้) กับไหน "ผูกกับตัวข้อความ" (รอบยืนยันตัดสินซ้ำไม่ได้).
type SceneVerdict struct {
	SceneNumber int      `json:"scene_number"`
	OK          bool     `json:"ok"`
	Issues      []string `json:"issues"`
	Codes       []string `json:"codes,omitempty"`
}

// visionVerdict คือ JSON ดิบที่โมเดลคืนต่อหนึ่งเฟรม
type visionVerdict struct {
	OK     bool     `json:"ok"`
	Issues []string `json:"issues"`
	Codes  []string `json:"codes"`
}

// VisualQAResult is what Review hands back to the orchestrator: per-scene
// verdicts plus the clip-level Passed decision.
type VisualQAResult struct {
	Verdicts []SceneVerdict
	Passed   bool
}

// summarizeVerdicts is the pure clip-level decision: the clip passes iff every
// scene verdict is OK. An empty verdict slice passes (nothing to block on —
// fail-open, consistent with the infra-error policy).
func summarizeVerdicts(verdicts []SceneVerdict) bool {
	for _, v := range verdicts {
		if !v.OK {
			return false
		}
	}
	return true
}

// VisualQATemplateData fills the seeded `visual_qa` prompt_template for one
// scene/frame.
type VisualQATemplateData struct {
	Question     string
	SceneNumber  int
	OnScreenText string
	VoiceText    string
}

// VisualQAAgent looks at one rendered frame per scene and flags visual defects.
// Runs on a vision-capable Claude model (cfg.Model = claude-sonnet-5).
type VisualQAAgent struct {
	llm *KieLLMClient
}

func NewVisualQAAgent(llm *KieLLMClient) *VisualQAAgent {
	return &VisualQAAgent{llm: llm}
}

// Review judges every frame and returns per-scene verdicts + the clip decision.
// It NEVER blocks on infrastructure: a template/vision/decode error for a scene
// is logged and recorded as an OK verdict (with the error in Issues), so only a
// confident visual defect (model says ok=false) can fail the clip. cfg is the
// `visual_qa` AgentConfig fetched by the caller via GetByName.
func (a *VisualQAAgent) Review(ctx context.Context, in VisualQAInput, cfg *models.AgentConfig) VisualQAResult {
	verdicts := make([]SceneVerdict, len(in.Frames))
	if in.Fast {
		// Judge frames concurrently (cap 4) — ~3.4 min → ~1 min for 10 scenes.
		// Each slot is index-assigned so verdict order stays stable, and
		// reviewFrame is already fail-open per scene.
		var g errgroup.Group
		g.SetLimit(4)
		for i, f := range in.Frames {
			i, f := i, f
			g.Go(func() error {
				verdicts[i] = a.reviewFrame(ctx, in.Question, f, cfg)
				return nil
			})
		}
		g.Wait() // reviewFrame never returns an error — Wait is just the barrier
	} else {
		for i, f := range in.Frames {
			verdicts[i] = a.reviewFrame(ctx, in.Question, f, cfg)
		}
	}
	return VisualQAResult{Verdicts: verdicts, Passed: summarizeVerdicts(verdicts)}
}

// reviewFrame judges a single frame. On ANY error it returns an OK verdict
// (fail-open) annotated with the error, never blocking publish on infra.
func (a *VisualQAAgent) reviewFrame(ctx context.Context, question string, f QAFrame, cfg *models.AgentConfig) SceneVerdict {
	ok := func(note string) SceneVerdict {
		var issues []string
		if note != "" {
			issues = []string{note}
		}
		return SceneVerdict{SceneNumber: f.SceneNumber, OK: true, Issues: issues}
	}

	if len(f.PNG) == 0 {
		log.Printf("visualqa: scene %d has no frame bytes — treating as OK (fail-open)", f.SceneNumber)
		return ok("no frame extracted (skipped)")
	}

	userPrompt, err := renderTemplate(cfg.PromptTemplate, VisualQATemplateData{
		Question:     question,
		SceneNumber:  f.SceneNumber,
		OnScreenText: f.OnScreenText,
		VoiceText:    f.VoiceText,
	})
	if err != nil {
		log.Printf("visualqa: scene %d template error (fail-open): %v", f.SceneNumber, err)
		return ok(fmt.Sprintf("template error: %v", err))
	}

	var out visionVerdict
	if err := a.llm.GenerateVisionJSON(ctx, cfg.Model, cfg.BuildSystemPrompt(), userPrompt, cfg.Temperature, [][]byte{f.PNG}, &out); err != nil {
		log.Printf("visualqa: scene %d vision error (fail-open): %v", f.SceneNumber, err)
		return ok(fmt.Sprintf("vision error: %v", err))
	}
	return SceneVerdict{SceneNumber: f.SceneNumber, OK: out.OK, Issues: out.Issues, Codes: out.Codes}
}

// MarshalVerdicts is a small helper for the orchestrator to JSON-encode verdicts
// for the visual_qa.issues column without importing encoding/json there twice.
func MarshalVerdicts(verdicts []SceneVerdict) []byte {
	b, err := json.Marshal(verdicts)
	if err != nil {
		return []byte("[]")
	}
	return b
}

// stickyCodes คือรหัสตำหนิที่ "ผูกกับตัวข้อความ ไม่ใช่จังหวะเวลา" — รอบยืนยันเคลียร์
// ไม่ได้. เหตุผล: เฟรมยืนยันถูกสุ่มที่ 85% ของซีน ซึ่งเป็นคนละวลีคาราโอเกะกับเฟรมรอบแรก
// ที่ 60% (ดู qaRecheckSceneFrac ใน orchestrator). คำที่ถูกผ่ากลางในวลีหนึ่งย่อมไม่โผล่
// ในอีกวลีหนึ่ง ถ้าปล่อยให้เคลียร์ได้ ตำหนิจริงที่คนดูเห็นเต็มตาตอนวินาทีนั้นจะหลุดขึ้น
// YouTube เงียบๆ ทุกครั้ง. two-strike มีไว้ฆ่า false positive จากอนิเมชันที่ยังไม่นิ่ง
// เท่านั้น — ตำหนิคลาสนี้ไม่ได้เกิดจากอนิเมชัน จึงไม่อยู่ในขอบเขตของมัน.
var stickyCodes = map[string]bool{
	"wordbreak": true,
}

// HasStickyCode บอกว่า verdict นี้มีรหัสที่รอบยืนยันแตะไม่ได้ไหม. ทน whitespace และ
// ตัวพิมพ์ใหญ่เพราะค่านี้มาจาก LLM ไม่ใช่จาก enum ในโค้ด.
func HasStickyCode(codes []string) bool {
	for _, c := range codes {
		if stickyCodes[strings.ToLower(strings.TrimSpace(c))] {
			return true
		}
	}
	return false
}

// ScenesNeedingConfirm คือ scene number ของ verdict ที่ตก (OK=false) และคุ้มจะยิง
// รอบยืนยัน — ไม่รวมซีนที่พก sticky code เพราะรอบยืนยันตัดสินซ้ำไม่ได้อยู่แล้ว
// (เฟรมยืนยันเป็นคนละวลีคาราโอเกะ ดูเหตุผลที่ stickyCodes) ยิงไปก็เสีย credit ฟรี
// โดยที่ผลลัพธ์จาก ConfirmMerge เหมือนเดิมทุกกรณี. ตัวเรียก (orchestrator) ใช้ผลนี้
// ตัดสินใจว่าจะขอเฟรมยืนยันซีนไหนบ้าง โดยไม่ต้อง re-derive ตรรกะ sticky เอง.
func ScenesNeedingConfirm(verdicts []SceneVerdict) map[int]bool {
	out := make(map[int]bool)
	for _, v := range verdicts {
		if !v.OK && !HasStickyCode(v.Codes) {
			out[v.SceneNumber] = true
		}
	}
	return out
}

// ConfirmMerge resolves a two-strike QA: a scene flagged by the first pass stays
// failed only if the confirm pass (a frame sampled later in the same scene) also
// flagged it. This kills timing false positives — karaoke captions mid-reveal,
// entrance animations still settling — while a baked-in defect (an overflowing
// headline is wrong at every timestamp) survives both passes. A flagged scene
// the confirm pass has no verdict for (frame extraction failed) is cleared:
// fail-open, matching reviewFrame's infra policy. ข้อยกเว้นเดียวคือ verdict ที่พก
// รหัสใน stickyCodes — รอบยืนยันแตะไม่ได้ (ดูเหตุผลที่ stickyCodes).
func ConfirmMerge(first, confirm VisualQAResult) VisualQAResult {
	if first.Passed {
		return first
	}
	confirmFailed := make(map[int][]string)
	for _, v := range confirm.Verdicts {
		if !v.OK {
			confirmFailed[v.SceneNumber] = v.Issues
		}
	}
	out := make([]SceneVerdict, len(first.Verdicts))
	for i, v := range first.Verdicts {
		if v.OK {
			out[i] = v
			continue
		}
		if HasStickyCode(v.Codes) {
			// ตำหนิผูกกับตัวข้อความ — เฟรมยืนยันเป็นคนละวลี ไม่มีสิทธิ์ล้างผลรอบแรก
			out[i] = v
			continue
		}
		if issues, still := confirmFailed[v.SceneNumber]; still {
			merged := append(append([]string{}, v.Issues...), issues...)
			out[i] = SceneVerdict{SceneNumber: v.SceneNumber, OK: false, Issues: merged, Codes: v.Codes}
			continue
		}
		note := append([]string{"เฟรมยืนยัน (recheck) ไม่พบปัญหา — เคลียร์ผลรอบแรก"}, v.Issues...)
		out[i] = SceneVerdict{SceneNumber: v.SceneNumber, OK: true, Issues: note}
	}
	return VisualQAResult{Verdicts: out, Passed: summarizeVerdicts(out)}
}
