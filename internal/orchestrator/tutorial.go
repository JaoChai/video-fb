package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
	"github.com/jaochai/video-fb/internal/repository"
)

// tutorialFormatName is the content_formats row (seeded disabled so the normal
// weighted picker can never select it) that marks a clip as tutorial mode.
const tutorialFormatName = "tutorial"

// basicFormatName คือ content_formats row ของคลิปสอนพื้นฐาน (seed ไว้แบบ disabled
// เพื่อไม่ให้ตัวสุ่ม format ของรอบปกติหยิบไปใช้) — คู่กับ tutorialFormatName
const basicFormatName = "basic"

// basicAgentMode คือ suffix ของ agent row ที่เขียนบทคลิปพื้นฐาน (script_basic ฯลฯ)
// จงใจไม่เพิ่มค่านี้เป็นโหมดใน internal/producer เพราะ "ใครเขียนบท" กับ
// "หน้าตาเป็นแบบไหน" เป็นคนละคำถาม และคลิป basic ต้องหน้าตาเหมือน tutorial เป๊ะ
const basicAgentMode = "basic"

// clipMode derives the RENDER mode from the clip's persisted content_format.
// Tutorial wins over the process-wide case flag so a tutorial clip renders as a
// manual even on a server where CASE_FORMAT_ENABLED is on. Basic clips render
// identically to tutorial clips — same preset, same image policy, same gate.
func clipMode(contentFormat string) string {
	if contentFormat == tutorialFormatName || contentFormat == basicFormatName {
		return producer.ModeTutorial
	}
	if producer.CaseFormatEnabled() {
		return producer.ModeCase
	}
	return producer.ModeClassic
}

// agentModeFor answers a different question from clipMode: which prompt rows
// write this clip. A basic clip looks exactly like a tutorial clip but is
// written for someone who has never run an ad, so it needs its own
// script/scene/critic rows while sharing every pixel of the tutorial's visuals.
func agentModeFor(contentFormat string) string {
	if contentFormat == basicFormatName {
		return basicAgentMode
	}
	return clipMode(contentFormat)
}

// needsCatalogFeature reports whether rebuilding this clip from scratch must
// reload its tutorial_features row first.
//
// คลิปที่หน้าตาเป็นคู่มือทุกตัวสอน UI จริงของ Facebook และ auto-publish โดยไม่มี
// คนตรวจ ถ้า retry รอบใหม่ไม่โหลดแถวคลังมาด้วย feat จะเป็น nil แปลว่า
// ตะแกรง ui_vocab ปิดเงียบ + agent ไม่ได้ menu path/steps → คลิปสอนเมนูที่โมเดล
// แต่งเองแล้วขึ้น YouTube เลย จึงต้องถามผ่าน clipMode ชุดเดียวกับที่เลือก preset
func needsCatalogFeature(contentFormat string) bool {
	return clipMode(contentFormat) == producer.ModeTutorial
}

// tutorialSceneShape maps a catalog step count onto the clip's scene budget:
// hook + promise + N uisteps + trap + recap + cta. Duration grows ~9s per step.
func tutorialSceneShape(steps int) (sceneCount, durationSec int) {
	return steps + 5, 28 + steps*9
}

// tutorialGateFailure is the deterministic half of the tutorial safety net: it
// returns a non-empty reason when the generated scenes would teach something the
// catalog never authorized. Returns "" for a nil feature (non-tutorial clips).
//
// This runs INSTEAD of a human reviewer — the clip auto-publishes when it comes
// back empty, so it must be strict about the two things a wrong tutorial gets
// wrong: an invented menu name, and a missing or invented step.
func tutorialGateFailure(scenes []agent.GeneratedScene, feat *models.TutorialFeature) string {
	if feat == nil {
		return ""
	}
	if v := agent.UIVocabViolations(scenes, feat.UIVocab); len(v) > 0 {
		msg := "ui_vocab violation: " + v[0]
		if len(v) > 1 {
			msg += fmt.Sprintf(" (and %d more)", len(v)-1)
		}
		return msg
	}
	if got, want := agent.CountUIStepScenes(scenes), len(feat.Steps); got != want {
		return fmt.Sprintf("step count mismatch: %d uistep scenes, catalog has %d steps", got, want)
	}
	return ""
}

// consultQAArchetype is the one archetype name ProduceTutorial refuses to use.
const consultQAArchetype = "consult_qa"

// tutorialArchetype filters out consult_qa for tutorial clips: its instruction
// frames the title as a fictitious person asking for advice ("คุณXครับ รบกวน
// ปรึกษา..."), which contradicts tutorial mode's no-greeting/no-persona prompt
// prefix (063_tutorial_agents.sql: "ห้ามขึ้นต้นด้วยชื่อฟีเจอร์ ห้ามทักทาย") and
// has no asker to name — tutorial clips run with an empty questioner_name.
// Tutorial auto-publishes with no human review, so a wrong pick would ship.
// ProduceWeekly is untouched and keeps picking consult_qa normally.
func tutorialArchetype(a *models.TitleArchetype) models.TitleArchetype {
	if a == nil || a.ArchetypeName == consultQAArchetype {
		return models.TitleArchetype{}
	}
	return *a
}

// freshnessDecision turns a verifier outcome into the (stale, reason) pair the
// picker acts on. It is the single place the fail-open contract lives: ANY error
// — LLM down, bad JSON, timeout — means "not stale", because parking a feature
// on a failed check would silently shrink the catalog until production stops.
// Only an explicit moved=true parks a feature. Pure, so the contract is testable
// without an LLM.
func freshnessDecision(v agent.FreshnessVerdict, err error) (stale bool, reason string) {
	if err != nil || !v.Moved {
		return false, ""
	}
	r := strings.TrimSpace(v.Reason)
	if r == "" {
		r = "research indicates the menu path changed"
	}
	return true, r
}

// ProduceTutorial produces the daily advanced tutorial clip (21:00 slot).
func (o *Orchestrator) ProduceTutorial(ctx context.Context) error {
	return o.produceCatalogClip(ctx, repository.TutorialLevelAdvanced,
		tutorialFormatName, producer.ModeTutorial, "tutorial")
}

// ProduceBasic produces the daily beginner clip (15:00 slot). Same machinery,
// same visuals, different catalog level and a script agent that explains every
// term instead of assuming the viewer already runs ads.
func (o *Orchestrator) ProduceBasic(ctx context.Context) error {
	return o.produceCatalogClip(ctx, repository.TutorialLevelBasic,
		basicFormatName, basicAgentMode, "basic")
}

// produceCatalogClip produces exactly one clip whose topic comes from the
// tutorial catalog. It mirrors ProduceWeekly's gate/tracker discipline but skips
// the question agent entirely — the topic IS the catalog row, so there is
// nothing to invent. label appears in logs and error messages only.
func (o *Orchestrator) produceCatalogClip(ctx context.Context, level, formatName, agentMode, label string) error {
	if credits, err := o.producer.KieCredits(ctx); err != nil {
		log.Printf("kie credit pre-check skipped (non-fatal): %v", err)
	} else if credits <= 0 {
		return fmt.Errorf("kie เครดิตหมด (เหลือ %d) — เติมเครดิตที่ kie.ai ก่อนผลิต", credits)
	}

	feat, err := o.pickVerifiedFeature(ctx, level)
	if err != nil {
		return err
	}
	if feat == nil {
		return fmt.Errorf("%s catalog is empty or every feature is parked for re-verification", label)
	}
	if len(feat.Steps) == 0 {
		return fmt.Errorf("%s feature %s has no steps — fix the catalog row", label, feat.FeatureKey)
	}
	log.Printf("Producing %s clip — feature: %s (%d steps)", label, feat.FeatureKey, len(feat.Steps))

	if !o.tracker.StartProduction(1) {
		return ErrProductionRunning
	}
	defer o.tracker.FinishProduction()

	ctx, cancel := context.WithCancel(ctx)
	o.tracker.SetCancelFunc(cancel)
	defer cancel()

	theme, err := o.themesRepo.GetActive(ctx)
	if err != nil {
		return fmt.Errorf("get active theme: %w", err)
	}
	scriptCfg, err := o.modeAgentConfig(ctx, "script", agentMode)
	if err != nil {
		return fmt.Errorf("get script agent config: %w", err)
	}
	imageCfg, err := o.agentsRepo.GetByName(ctx, "image")
	if err != nil {
		return fmt.Errorf("get image agent config: %w", err)
	}
	brandAliases, err := o.settingsRepo.GetBrandAliases(ctx)
	if err != nil {
		return fmt.Errorf("read brand aliases: %w", err)
	}
	format, err := o.formatsRepo.GetByName(ctx, formatName)
	if err != nil {
		return fmt.Errorf("get %s content format: %w", label, err)
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var archetype models.TitleArchetype
	if a, aerr := o.titleArchetypesRepo.PickNext(ctx); aerr != nil || a == nil {
		log.Printf("%s: archetype pick failed, using empty: %v", label, aerr)
	} else {
		archetype = tutorialArchetype(a)
	}

	var persona string
	personasJSON, _ := o.settingsRepo.Get(ctx, "audience_personas")
	var personas []string
	if json.Unmarshal([]byte(personasJSON), &personas) == nil && len(personas) > 0 {
		persona = PickPersona(personas, rng)
	} else {
		persona, _ = o.settingsRepo.Get(ctx, "audience_persona")
	}

	q := agent.GeneratedQuestion{
		Question:  feat.DisplayNameTH,
		Category:  feat.Audience,
		PainPoint: feat.PainPoint,
	}

	o.tracker.SetTotalClips(1)
	o.tracker.StartClip(1, feat.DisplayNameTH)
	if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format,
		persona, archetype, "reach", feat); err != nil {
		o.tracker.AddErrorLog(fmt.Sprintf("%s clip failed: %v", label, err))
		o.nudgeRetry()
		return err
	}
	if mErr := o.tutorialFeaturesRepo.MarkUsed(ctx, feat.ID); mErr != nil {
		log.Printf("%s: MarkUsed failed (non-fatal): %v", label, mErr)
	}
	o.tracker.CompleteStep("complete")
	log.Printf("%s production complete", label)
	return nil
}

// tutorialAvoidRepeat looks up prior angles used on this catalog feature and
// renders the avoid-repeat prompt block for the script agent. Fails open on
// any lookup error (non-fatal log + "") — a flaky DB read must never block a
// clip, same contract as every other tutorial fallback in this file.
func (o *Orchestrator) tutorialAvoidRepeat(ctx context.Context, feat *models.TutorialFeature) string {
	if feat == nil {
		return ""
	}
	angles, err := o.tutorialFeaturesRepo.RecentAngles(ctx, feat.FeatureKey, 5)
	if err != nil {
		log.Printf("tutorial: RecentAngles failed (non-fatal): %v", err)
		return ""
	}
	return agent.TutorialAvoidRepeatBlock(angles)
}

// pickVerifiedFeature picks the least-used feature and asks research whether its
// menu path still matches Meta's current UI. A feature research says has moved
// gets parked and the next one is tried — up to 2 skips, then the last pick is
// produced anyway. Never returning a feature would mean 0 clips, which is worse
// than one clip on a possibly-moved menu (the ui_vocab gate still guards the
// labels themselves).
func (o *Orchestrator) pickVerifiedFeature(ctx context.Context, level string) (*models.TutorialFeature, error) {
	// maxSkips bounds latency, not catalog size: each skip costs a research call
	// plus a verify call. The floor that keeps the catalog usable lives in Park.
	const maxSkips = 2
	var skipped []string
	var last *models.TutorialFeature

	for attempt := 0; attempt <= maxSkips; attempt++ {
		feat, err := o.tutorialFeaturesRepo.PickNext(ctx, level, skipped)
		if err != nil {
			return nil, fmt.Errorf("pick tutorial feature: %w", err)
		}
		if feat == nil {
			break
		}
		last = feat
		if attempt == maxSkips {
			break // out of skips — produce this one
		}
		stale, reason := o.featureLooksStale(ctx, feat)
		if !stale {
			return feat, nil
		}
		outcome, pErr := o.tutorialFeaturesRepo.Park(ctx, feat.ID, reason)
		if pErr != nil {
			// error ระหว่าง park — ทำเหมือน ParkRefusedFloor (fail-open เดิม):
			// ตะแกรง ui_vocab ยังกันชื่อเมนูที่โมเดลแต่งเองอยู่
			log.Printf("tutorial: park failed (non-fatal): %v", pErr)
			return feat, nil
		}
		switch outcome {
		case repository.ParkFirstStrike:
			log.Printf("tutorial: feature %s got its first stale flag (not parked yet): %s",
				feat.FeatureKey, reason)
			skipped = append(skipped, feat.FeatureKey)
		case repository.ParkedForVerify:
			log.Printf("tutorial: feature %s parked for re-verification: %s", feat.FeatureKey, reason)
			skipped = append(skipped, feat.FeatureKey)
		case repository.ParkRefusedFloor:
			// คลังเหลือน้อยถึงพื้น — ผลิตตัวนี้ต่อดีกว่าไม่มีคลิป
			log.Printf("tutorial: feature %s looks stale (%s) but catalog is at the floor — producing it anyway",
				feat.FeatureKey, reason)
			return feat, nil
		}
	}
	return last, nil
}

// featureLooksStale asks the research agent whether the feature's menu path has
// moved. Fails OPEN in every direction: any error, empty research, or an
// unparseable answer means "not stale" — a flaky web search must never stop the
// daily clip.
func (o *Orchestrator) featureLooksStale(ctx context.Context, feat *models.TutorialFeature) (bool, string) {
	if o.researchAgent == nil || o.tutorialVerifier == nil {
		return false, ""
	}
	rctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	research, err := o.researchAgent.Research(rctx,
		fmt.Sprintf("Facebook Ads %s menu location %s changed moved renamed",
			feat.DisplayNameTH, strings.Join(feat.MenuPath, " > ")))
	if err != nil || strings.TrimSpace(research) == "" {
		return false, ""
	}

	cfg, cErr := o.agentsRepo.GetByName(ctx, "research")
	if cErr != nil || cfg == nil {
		log.Printf("tutorial: research agent config unavailable (fail-open): %v", cErr)
		return false, ""
	}
	verdict, vErr := o.tutorialVerifier.Verify(rctx,
		strings.Join(feat.MenuPath, " > "), feat.DisplayNameTH, research, cfg)
	if vErr != nil {
		log.Printf("tutorial: freshness verdict failed (fail-open): %v", vErr)
	}
	return freshnessDecision(verdict, vErr)
}
