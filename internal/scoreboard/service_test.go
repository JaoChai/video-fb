package scoreboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jaochai/video-fb/internal/models"
)

type fakeRepo struct {
	stats       []Stat
	saved       []Score
	weights     map[string]map[string]int
	applied     map[string]map[string]int
	revisions   []models.WeightRevision
	failRecord  bool
	lastBatch   []models.WeightRevision
	latestScore []Score
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		weights: map[string]map[string]int{},
		applied: map[string]map[string]int{},
	}
}

func (f *fakeRepo) RawStats(ctx context.Context, windowDays int) ([]Stat, error) { return f.stats, nil }
func (f *fakeRepo) SaveSnapshot(ctx context.Context, at time.Time, scores []Score) error {
	f.saved = scores
	f.latestScore = scores
	return nil
}
func (f *fakeRepo) Latest(ctx context.Context) (time.Time, []Score, error) {
	return time.Unix(1000, 0), f.latestScore, nil
}
func (f *fakeRepo) CurrentWeights(ctx context.Context, dim string) (map[string]int, error) {
	if w, ok := f.weights[dim]; ok {
		return w, nil
	}
	return map[string]int{}, nil
}
func (f *fakeRepo) ApplyWeights(ctx context.Context, dim string, w map[string]int) error {
	f.applied[dim] = w
	return nil
}
func (f *fakeRepo) RecordRevisions(ctx context.Context, revs []models.WeightRevision) error {
	if f.failRecord {
		return errors.New("audit ล่ม")
	}
	f.revisions = append(f.revisions, revs...)
	return nil
}
func (f *fakeRepo) LastRevisionBatch(ctx context.Context) ([]models.WeightRevision, error) {
	return f.lastBatch, nil
}

type fakeSettings struct{ v string }

func (f fakeSettings) Get(ctx context.Context, key string) (string, error) { return f.v, nil }

func fixedNow() time.Time { return time.Unix(1000, 0).UTC() }

func TestComputeSnapshotScoresAndSaves(t *testing.T) {
	repo := newFakeRepo()
	repo.stats = []Stat{
		{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30, MedianPct: 0.53, MedianRetention: 0.7, FlopRate: 0.3},
		{Dimension: "content_format", Value: "news", Platform: "youtube", N: 25, MedianPct: 0.53, MedianRetention: 0.75, FlopRate: 0.16},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	n, err := svc.ComputeSnapshot(context.Background())
	if err != nil {
		t.Fatalf("ComputeSnapshot error: %v", err)
	}
	if n != 2 || len(repo.saved) != 2 {
		t.Fatalf("บันทึก %d แถว (คืน %d), want 2", len(repo.saved), n)
	}
	if repo.saved[0].ScoreFinal == 0 {
		t.Error("ScoreFinal ต้องถูกคำนวณก่อนบันทึก")
	}
}

func TestTuneOnceDisabledIsNoOp(t *testing.T) {
	repo := newFakeRepo()
	repo.weights["content_format"] = map[string]int{"qa": 50, "news": 50}
	repo.latestScore = []Score{
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
		{Stat: Stat{Dimension: "content_format", Value: "news", Platform: "youtube", N: 30}, ScoreFinal: 0.2},
	}
	svc := NewService(repo, fakeSettings{v: "false"}, fixedNow)
	if err := svc.TuneOnce(context.Background()); err != nil {
		t.Fatalf("TuneOnce error: %v", err)
	}
	if len(repo.applied) != 0 {
		t.Errorf("flag ปิดแล้วยังเขียน weight: %v", repo.applied)
	}
}

func TestTuneOnceWritesAuditBeforeApplying(t *testing.T) {
	repo := newFakeRepo()
	repo.failRecord = true
	repo.weights["content_format"] = map[string]int{"qa": 50, "news": 50}
	repo.latestScore = []Score{
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
		{Stat: Stat{Dimension: "content_format", Value: "news", Platform: "youtube", N: 30}, ScoreFinal: 0.2},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	_ = svc.TuneOnce(context.Background())
	if len(repo.applied) != 0 {
		t.Error("audit ล้มแล้วห้าม apply weight เด็ดขาด")
	}
}

func TestTuneOnceAppliesAndAudits(t *testing.T) {
	repo := newFakeRepo()
	repo.weights["content_format"] = map[string]int{"qa": 50, "news": 50}
	repo.latestScore = []Score{
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
		{Stat: Stat{Dimension: "content_format", Value: "news", Platform: "youtube", N: 30}, ScoreFinal: 0.2},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	if err := svc.TuneOnce(context.Background()); err != nil {
		t.Fatalf("TuneOnce error: %v", err)
	}
	applied := repo.applied["content_format"]
	if applied["qa"] <= 50 {
		t.Errorf("qa = %d ควรเพิ่มขึ้น", applied["qa"])
	}
	if len(repo.revisions) == 0 {
		t.Error("ต้องมี audit อย่างน้อยหนึ่งแถว")
	}
	for _, r := range repo.revisions {
		if r.Dimension == "style_preset" {
			t.Error("style_preset ห้ามถูกหมุนน้ำหนัก")
		}
	}
}

func TestTuneOnceSkipsStylePreset(t *testing.T) {
	repo := newFakeRepo()
	repo.latestScore = []Score{
		{Stat: Stat{Dimension: "style_preset", Value: "case-file", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	if err := svc.TuneOnce(context.Background()); err != nil {
		t.Fatalf("TuneOnce error: %v", err)
	}
	if _, ok := repo.applied["style_preset"]; ok {
		t.Error("style_preset ต้องไม่ถูกเขียน weight")
	}
}

func TestRollbackRestoresOldWeights(t *testing.T) {
	repo := newFakeRepo()
	repo.lastBatch = []models.WeightRevision{
		{Dimension: "content_format", Value: "qa", OldWeight: 25, NewWeight: 31},
		{Dimension: "content_format", Value: "news", OldWeight: 25, NewWeight: 19},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	n, err := svc.Rollback(context.Background())
	if err != nil {
		t.Fatalf("Rollback error: %v", err)
	}
	if n != 2 {
		t.Errorf("คืนค่า %d แถว, want 2", n)
	}
	if repo.applied["content_format"]["qa"] != 25 {
		t.Errorf("qa = %d, want 25", repo.applied["content_format"]["qa"])
	}
}
