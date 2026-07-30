package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/jaochai/video-fb/internal/agent"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
)

// mythFormatName คือแถว content_formats ของคลิปจับความเชื่อผิด (seed ไว้แบบปิด
// เพื่อไม่ให้ตัวสุ่ม format ของรอบ 12:00/18:00 หยิบไปใช้)
const mythFormatName = "myth"

// ErrContentGateBlocked ห่ออยู่ในทุก error ที่มาจากตะแกรงเนื้อหา (ui_vocab ของคลิปสอน,
// ข้อเท็จจริงของคลิป myth) เพื่อให้ผู้เรียกแยก "เนื้อหาผิด" ออกจาก "ระบบพัง" ได้
// โดยไม่ต้องอ่านข้อความ error — สองกรณีนี้ต้องจัดการคนละแบบกับคลังหัวข้อ
var ErrContentGateBlocked = errors.New("content gate blocked")

// mythSceneShape คือรูปร่างคลิปตามสเปก §6.2: hook + belief + verdict + proof +
// tip(ส่วนที่จริง) + cta = 6 ซีน คงที่ ไม่ผูกกับข้อมูลในคลังอย่างคลิปสอน
func mythSceneShape() (sceneCount, durationSec int) {
	return 6, 62
}

// needsMythBelief บอกว่าการสร้างคลิปนี้ใหม่ทั้งตัวต้องโหลดแถวคลัง myth ก่อนไหม
//
// คลิป myth auto-publish โดยไม่มีคนตรวจ ถ้า retry ไม่โหลดแถวคลังมาด้วย ตะแกรง
// ข้อเท็จจริงจะไม่มีอะไรเทียบ = ปิดเงียบ แล้วคลิปที่แต่งตัวเลขเองจะขึ้น YouTube
// (บั๊กแบบเดียวกับที่เคยเกิดกับคลิป basic ตอน retryFull)
func needsMythBelief(contentFormat string) bool {
	return clipMode(contentFormat) == producer.ModeMyth
}

// mythGateFailure คือครึ่ง deterministic ของตะแกรงข้อเท็จจริง คืนเหตุผลที่อ่านออก
// เมื่อซีนพูดสิ่งที่แถวคลังไม่ได้ให้มา และคืน "" เมื่อผ่าน (หรือ b == nil = โหมดอื่น)
//
// ตัวนี้ทำงานแทนคนตรวจ: คลิปเผยแพร่เองเมื่อผ่าน จึงเข้มกับสองสิ่งที่คลิปแนวนี้
// พลาดแล้วเสียหายที่สุด — ตัวเลขที่โมเดลแต่งขึ้น และการอ้างระบบคะแนนที่ไม่มีเอกสาร
func mythGateFailure(scenes []agent.GeneratedScene, b *models.MythBelief) string {
	if msg := mythTextGateFailure(scenes, b); msg != "" {
		return msg
	}
	if b == nil {
		return ""
	}
	// คลิปที่ขาดซีน verdict หรือ proof = คลิปที่ไม่มีคำตัดสินและไม่มีแหล่งอ้างบนจอ
	// ทั้งที่นั่นคือแกนทั้งหมดของรูปแบบนี้ — ตะแกรงข้อความจับไม่ได้เพราะ "ไม่มีอะไร"
	// ไม่ใช่ "มีของผิด" (คลิปสอนใช้การนับซีน uistep กันเรื่องเดียวกัน)
	//
	// กฎนี้อยู่แยกจาก mythTextGateFailure โดยตั้งใจ: ตัวตรวจ title/description ของ
	// YouTube ส่งข้อความเข้ามาในรูปซีนสมมุติหนึ่งตัว ถ้ารวมกฎนี้ไว้ด้วย คลิปทุกตัว
	// จะถูกบล็อกที่ขั้น metadata ทันที = ช่องตายสนิท
	need := map[string]bool{"verdict": true, "proof": true}
	for _, sc := range scenes {
		delete(need, agent.ClampLayout(sc.Layout))
	}
	if len(need) > 0 {
		missing := make([]string, 0, len(need))
		for layout := range need {
			missing = append(missing, layout)
		}
		sort.Strings(missing)
		return "missing required scene layout: " + strings.Join(missing, ", ")
	}
	return ""
}

// mythTextGateFailure ตรวจเฉพาะ "ข้อความพูดสิ่งที่คลังไม่ได้ให้มา" — ใช้ได้กับทั้งชุดซีน
// และกับ title/description ของ YouTube ที่ห่อมาในรูปซีนสมมุติ
func mythTextGateFailure(scenes []agent.GeneratedScene, b *models.MythBelief) string {
	if b == nil {
		return ""
	}
	if v := agent.FactNumberViolations(scenes, b); len(v) > 0 {
		msg := "fact number violation: " + v[0]
		if len(v) > 1 {
			msg += fmt.Sprintf(" (และอีก %d จุด)", len(v)-1)
		}
		return msg
	}
	if v := agent.DisallowedClaimViolations(scenes, b); len(v) > 0 {
		return "unsourced claim: " + v[0]
	}
	return ""
}

// ProduceMyth ผลิตคลิปจับความเชื่อผิดหนึ่งตัวจากคลัง (ช่อง 09:00)
//
// ยืมวินัยทั้งหมดของ produceCatalogClip (เช็คเครดิต kie, production gate, tracker,
// ยกเลิกได้) แต่คลังเป็นตารางอื่นที่ไม่มี steps/ui_vocab จึงไม่ใช้ฟังก์ชันเดียวกัน
func (o *Orchestrator) ProduceMyth(ctx context.Context) error {
	if credits, err := o.producer.KieCredits(ctx); err != nil {
		log.Printf("kie credit pre-check skipped (non-fatal): %v", err)
	} else if credits <= 0 {
		return fmt.Errorf("kie เครดิตหมด (เหลือ %d) — เติมเครดิตที่ kie.ai ก่อนผลิต", credits)
	}

	belief, err := o.mythBeliefsRepo.PickNext(ctx)
	if err != nil {
		return fmt.Errorf("pick myth belief: %w", err)
	}
	if belief == nil {
		return fmt.Errorf("คลัง myth_beliefs ไม่มีแถวที่ยืนยันข้อเท็จจริงแล้ว (needs_verify=false) — เติมคลังก่อน")
	}
	log.Printf("Producing myth clip — belief: %s (verdict %s)", belief.BeliefKey, belief.Verdict)

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
	scriptCfg, err := o.modeAgentConfig(ctx, "script", producer.ModeMyth)
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
	format, err := o.formatsRepo.GetByName(ctx, mythFormatName)
	if err != nil {
		return fmt.Errorf("get myth content format: %w", err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var persona string
	personasJSON, _ := o.settingsRepo.Get(ctx, "audience_personas")
	var personas []string
	if json.Unmarshal([]byte(personasJSON), &personas) == nil && len(personas) > 0 {
		persona = PickPersona(personas, rng)
	} else {
		persona, _ = o.settingsRepo.Get(ctx, "audience_persona")
	}

	q := agent.GeneratedQuestion{
		Question: belief.BeliefTH,
		Category: belief.Audience,
	}

	o.tracker.SetTotalClips(1)
	o.tracker.StartClip(1, belief.BeliefTH)
	err = o.produceMythClip(ctx, q, theme, scriptCfg, imageCfg, brandAliases, format, persona, belief)

	// นับใช้+พักแถวนี้ทั้งกรณีสำเร็จ และกรณีตกตะแกรงเนื้อหา — ถ้าไม่นับกรณีตกตะแกรง
	// แถวเดิมจะยังเป็น "แถวที่ใช้น้อยสุด" พรุ่งนี้ แล้วช่อง 09:00 จะได้คลิปที่ค้าง
	// needs_review ทุกวันโดยที่แถวอื่นไม่ได้คิวเลย (ช่องตายเงียบแบบมีคลิปแต่ไม่เผยแพร่)
	// error อื่น (kie ล่ม, DB พัง) ไม่นับ เพราะหัวข้อยังไม่ได้ถูกใช้จริง
	if err == nil || errors.Is(err, ErrContentGateBlocked) {
		if mErr := o.mythBeliefsRepo.MarkUsed(ctx, belief.ID); mErr != nil {
			log.Printf("myth: MarkUsed failed (non-fatal): %v", mErr)
		}
	}
	if err != nil {
		o.tracker.AddErrorLog(fmt.Sprintf("myth clip failed: %v", err))
		o.nudgeRetry()
		return err
	}
	o.tracker.CompleteStep("complete")
	log.Println("myth production complete")
	return nil
}

// produceMythClip สร้างแถว clip แล้วส่งต่อไปเส้นทางผลิตร่วม — แยกออกมาเพื่อให้
// belief_key ถูกบันทึกลงคลิปก่อนเริ่มผลิต (ทั้ง retry และชั้นเรนเดอร์ต้องใช้)
func (o *Orchestrator) produceMythClip(ctx context.Context, q agent.GeneratedQuestion,
	theme *models.BrandTheme, scriptCfg, imageCfg *models.AgentConfig, brandAliases map[string]string,
	format *models.ContentFormat, persona string, belief *models.MythBelief) error {

	preset := presetFor(format.FormatName)
	today := time.Now().Format("2006-01-02")
	clip, err := o.clipsRepo.Create(ctx, models.CreateClipRequest{
		Title:           q.Question,
		Question:        q.Question,
		Category:        q.Category,
		PublishDate:     &today,
		ContentFormat:   format.FormatName,
		ClipRole:        "reach",
		AudiencePersona: persona,
		MythBelief:      belief.BeliefKey,
	})
	if err != nil {
		return fmt.Errorf("create clip: %w", err)
	}
	status := "producing"
	o.clipsRepo.Update(ctx, clip.ID, models.UpdateClipRequest{Status: &status, StylePreset: &preset.Key})

	return o.produceClipWithID(ctx, clip.ID, q, theme, preset, scriptCfg, imageCfg, brandAliases,
		format, persona, models.TitleArchetype{}, "reach", nil, belief)
}

// mythAvoidRepeatBlock แจ้ง agent ว่าเคยเปิดคลิปเรื่องนี้ด้วยมุมไหนแล้ว — คลังมี
// แถวน้อย หัวข้อเดิมจะกลับมาทุก 14 วัน ถ้าเปิดเหมือนเดิมทุกครั้งคนดูจะจำได้ว่าเคยเห็น
// ล้มเหลวแบบเงียบ (คืน "") ทุกทาง: การอ่านสถิติที่พลาดต้องไม่หยุดคลิป
func (o *Orchestrator) mythAvoidRepeatBlock(ctx context.Context, belief *models.MythBelief) string {
	if belief == nil {
		return ""
	}
	titles, err := o.clipsRepo.RecentTitlesByMythBelief(ctx, belief.BeliefKey, 3)
	if err != nil {
		log.Printf("myth: RecentTitlesByMythBelief failed (non-fatal): %v", err)
		return ""
	}
	if len(titles) == 0 {
		return ""
	}
	return "\n## มุมที่คลิปก่อนหน้าใช้กับความเชื่อนี้แล้ว (ห้ามเปิดซ้ำมุมเดิม ห้ามเปลี่ยนข้อเท็จจริง)\n- " +
		strings.Join(titles, "\n- ") + "\n"
}
