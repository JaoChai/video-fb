package scoreboard

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jaochai/video-fb/internal/models"
)

const (
	// WindowDays คือหน้าต่างวัดผล 60 วัน — 30 วันให้ category แค่ ~5 คลิป/หมวด ไม่พอตัดสิน
	WindowDays = 60
	// MinN คือจำนวนคลิปขั้นต่ำก่อนยอมให้สูตรหนึ่งขยับน้ำหนัก
	MinN = 8
	// Alpha คือสัดส่วนของระยะทางที่ยอมให้เคลื่อนต่อรอบ (สัปดาห์ละครั้ง)
	Alpha = 0.25
	// SettingKey คือสวิตช์ปิดฉุกเฉิน ปิดแล้วหยุดหมุนทันทีโดยไม่แตะค่าที่มีอยู่
	SettingKey = "weight_tuner_enabled"
)

// TunableDimensions คือมิติที่หมุนน้ำหนักได้จริง style_preset ไม่อยู่ในนี้โดยตั้งใจ:
// การเลือก preset บน prod ถูก CaseFormatEnabled คร่อมไว้ก่อน จึงวัดผลอย่างเดียว
var TunableDimensions = []string{"content_format", "category"}

// Repo คือสิ่งที่ service ต้องการจากชั้นข้อมูล — แคบไว้เพื่อให้ทดสอบด้วย fake ได้
type Repo interface {
	RawStats(ctx context.Context, windowDays int) ([]Stat, error)
	SaveSnapshot(ctx context.Context, computedAt time.Time, scores []Score) error
	Latest(ctx context.Context) (time.Time, []Score, error)
	CurrentWeights(ctx context.Context, dimension string) (map[string]int, error)
	ApplyWeights(ctx context.Context, dimension string, weights map[string]int) error
	RecordRevisions(ctx context.Context, revs []models.WeightRevision) error
	LastRevisionBatch(ctx context.Context) ([]models.WeightRevision, error)
}

// Settings อ่านสวิตช์เปิด/ปิด
type Settings interface {
	Get(ctx context.Context, key string) (string, error)
}

type Service struct {
	repo     Repo
	settings Settings
	now      func() time.Time
}

func NewService(repo Repo, settings Settings, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, settings: settings, now: now}
}

// ComputeSnapshot คำนวณกระดานคะแนนใหม่หนึ่งใบ ไม่แตะ weight ใดๆ
// คืนจำนวนแถวที่บันทึก
func (s *Service) ComputeSnapshot(ctx context.Context) (int, error) {
	stats, err := s.repo.RawStats(ctx, WindowDays)
	if err != nil {
		return 0, fmt.Errorf("scoreboard: raw stats: %w", err)
	}
	if len(stats) == 0 {
		log.Printf("scoreboard: ไม่มีสถิติในหน้าต่าง %d วัน — ไม่บันทึก snapshot", WindowDays)
		return 0, nil
	}
	scores := ScoreAll(stats)
	if err := s.repo.SaveSnapshot(ctx, s.now(), scores); err != nil {
		return 0, fmt.Errorf("scoreboard: save snapshot: %w", err)
	}
	log.Printf("scoreboard: บันทึก %d แถว (หน้าต่าง %d วัน)", len(scores), WindowDays)
	return len(scores), nil
}

// TuneOnce หมุนน้ำหนักหนึ่งรอบจาก snapshot ล่าสุด เขียน audit ให้สำเร็จก่อนเสมอ
// ความล้มเหลวของมิติหนึ่งไม่หยุดมิติอื่น
func (s *Service) TuneOnce(ctx context.Context) error {
	enabled, err := s.settings.Get(ctx, SettingKey)
	if err != nil {
		return fmt.Errorf("scoreboard: อ่าน %s: %w", SettingKey, err)
	}
	if enabled != "true" {
		log.Printf("scoreboard: %s != true — ข้ามรอบนี้", SettingKey)
		return nil
	}

	computedAt, scores, err := s.repo.Latest(ctx)
	if err != nil {
		return fmt.Errorf("scoreboard: อ่าน snapshot ล่าสุด: %w", err)
	}
	if len(scores) == 0 {
		log.Printf("scoreboard: ยังไม่มี snapshot — ไม่หมุนน้ำหนัก")
		return nil
	}

	for _, dim := range TunableDimensions {
		current, err := s.repo.CurrentWeights(ctx, dim)
		if err != nil {
			log.Printf("scoreboard: [%s] อ่าน weight ไม่ได้ (ข้าม): %v", dim, err)
			continue
		}
		if len(current) == 0 {
			log.Printf("scoreboard: [%s] ไม่มีสูตรที่ enabled — ข้าม", dim)
			continue
		}

		combined := CombinePlatforms(scores, dim)
		next := TuneWeights(current, combined, MinN, Alpha)

		scoreByValue := map[string]Combined{}
		for _, c := range combined {
			scoreByValue[c.Value] = c
		}
		var revs []models.WeightRevision
		for k, newW := range next {
			if newW == current[k] {
				continue
			}
			revs = append(revs, models.WeightRevision{
				Dimension: dim, Value: k,
				OldWeight: current[k], NewWeight: newW,
				ScoreFinal: scoreByValue[k].ScoreFinal, N: scoreByValue[k].N,
				ComputedAt: computedAt,
			})
		}
		if len(revs) == 0 {
			log.Printf("scoreboard: [%s] น้ำหนักไม่เปลี่ยน — ข้าม", dim)
			continue
		}

		// audit ก่อน apply เสมอ: ถ้าเขียนประวัติไม่ได้ ห้ามเปลี่ยนของจริง
		if err := s.repo.RecordRevisions(ctx, revs); err != nil {
			log.Printf("scoreboard: [%s] เขียน audit ไม่สำเร็จ — ไม่ apply: %v", dim, err)
			continue
		}
		if err := s.repo.ApplyWeights(ctx, dim, next); err != nil {
			log.Printf("scoreboard: [%s] apply ล้มหลังเขียน audit แล้ว (กู้จาก weight_revisions ได้): %v", dim, err)
			continue
		}
		log.Printf("scoreboard: [%s] หมุนน้ำหนัก %d ค่า: %v", dim, len(revs), next)
	}
	return nil
}

// Rollback เขียน old_weight ของ batch ล่าสุดกลับ คืนจำนวนแถวที่คืนค่า
// ไม่ลบประวัติ — ประวัติเป็น append-only เสมอ
func (s *Service) Rollback(ctx context.Context) (int, error) {
	batch, err := s.repo.LastRevisionBatch(ctx)
	if err != nil {
		return 0, fmt.Errorf("scoreboard: อ่าน batch ล่าสุด: %w", err)
	}
	if len(batch) == 0 {
		return 0, nil
	}
	byDim := map[string]map[string]int{}
	for _, v := range batch {
		if byDim[v.Dimension] == nil {
			byDim[v.Dimension] = map[string]int{}
		}
		byDim[v.Dimension][v.Value] = v.OldWeight
	}
	for dim, weights := range byDim {
		if err := s.repo.ApplyWeights(ctx, dim, weights); err != nil {
			return 0, fmt.Errorf("scoreboard: rollback %s: %w", dim, err)
		}
	}
	log.Printf("scoreboard: rollback คืนค่า %d แถว", len(batch))
	return len(batch), nil
}
