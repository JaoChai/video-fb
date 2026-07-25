package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
)

// tutorialFormatName is the content_formats row (seeded disabled so the normal
// weighted picker can never select it) that marks a clip as tutorial mode.
const tutorialFormatName = "tutorial"

// clipMode derives the render mode from the clip's persisted content_format.
// Tutorial wins over the process-wide case flag so a tutorial clip renders as a
// manual even on a server where CASE_FORMAT_ENABLED is on. Deriving from the
// stored column (not a flag) is what makes resume and retry pick the right mode.
func clipMode(contentFormat string) string {
	if contentFormat == tutorialFormatName {
		return producer.ModeTutorial
	}
	if producer.CaseFormatEnabled() {
		return producer.ModeCase
	}
	return producer.ModeClassic
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

// ProduceTutorial produces exactly one tutorial clip from the catalog. It mirrors
// ProduceWeekly's gate/tracker discipline but skips the question agent entirely —
// the topic IS the catalog row, so there is nothing to invent.
func (o *Orchestrator) ProduceTutorial(ctx context.Context) error {
	if credits, err := o.producer.KieCredits(ctx); err != nil {
		log.Printf("kie credit pre-check skipped (non-fatal): %v", err)
	} else if credits <= 0 {
		return fmt.Errorf("kie เครดิตหมด (เหลือ %d) — เติมเครดิตที่ kie.ai ก่อนผลิต", credits)
	}

	feat, err := o.pickVerifiedFeature(ctx)
	if err != nil {
		return err
	}
	if feat == nil {
		return fmt.Errorf("tutorial catalog is empty or every feature is parked for re-verification")
	}
	if len(feat.Steps) == 0 {
		return fmt.Errorf("tutorial feature %s has no steps — fix the catalog row", feat.FeatureKey)
	}
	log.Printf("Producing tutorial clip — feature: %s (%d steps)", feat.FeatureKey, len(feat.Steps))

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
	scriptCfg, err := o.modeAgentConfig(ctx, "script", producer.ModeTutorial)
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
	format, err := o.formatsRepo.GetByName(ctx, tutorialFormatName)
	if err != nil {
		return fmt.Errorf("get tutorial content format: %w", err)
	}
	persona, _ := o.settingsRepo.Get(ctx, "audience_persona")

	q := agent.GeneratedQuestion{
		Question:  feat.DisplayNameTH,
		Category:  "grey-operator",
		PainPoint: feat.PainPoint,
	}

	o.tracker.SetTotalClips(1)
	o.tracker.StartClip(1, feat.DisplayNameTH)
	if err := o.produceClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format,
		persona, models.TitleArchetype{}, "reach", feat); err != nil {
		o.tracker.AddErrorLog(fmt.Sprintf("tutorial clip failed: %v", err))
		o.nudgeRetry()
		return err
	}
	if mErr := o.tutorialFeaturesRepo.MarkUsed(ctx, feat.ID); mErr != nil {
		log.Printf("tutorial: MarkUsed failed (non-fatal): %v", mErr)
	}
	o.tracker.CompleteStep("complete")
	log.Println("Tutorial production complete")
	return nil
}

// pickVerifiedFeature picks the least-used feature and asks research whether its
// menu path still matches Meta's current UI. A feature research says has moved
// gets parked (needs_verify) and the next one is tried — up to 2 skips, then the
// last pick is produced anyway. Never returning a feature would mean 0 clips,
// which is worse than one clip on a possibly-moved menu (the ui_vocab gate still
// guards the labels themselves).
func (o *Orchestrator) pickVerifiedFeature(ctx context.Context) (*models.TutorialFeature, error) {
	const maxSkips = 2
	var skipped []string
	var last *models.TutorialFeature

	for attempt := 0; attempt <= maxSkips; attempt++ {
		feat, err := o.tutorialFeaturesRepo.PickNext(ctx, skipped)
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
		log.Printf("tutorial: feature %s parked for re-verification: %s", feat.FeatureKey, reason)
		if mErr := o.tutorialFeaturesRepo.MarkNeedsVerify(ctx, feat.ID, reason); mErr != nil {
			log.Printf("tutorial: MarkNeedsVerify failed (non-fatal): %v", mErr)
		}
		skipped = append(skipped, feat.FeatureKey)
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
