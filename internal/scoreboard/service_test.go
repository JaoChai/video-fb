package scoreboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jaochai/video-fb/internal/models"
)

type fakeRepo struct {
	stats         []Stat
	saved         []Score
	weights       map[string]map[string]int
	applied       map[string]map[string]int
	revisions     []models.WeightRevision
	failRecord    bool
	failRecordDim string // ถ้าไม่ว่าง: RecordRevisions ล้มเฉพาะ batch ของมิตินี้ มิติอื่นผ่านปกติ
	lastBatch     []models.WeightRevision
	latestScore   []Score
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		weights: map[string]map[string]int{},
		applied: map[string]map[string]int{},
	}
}

// TunableDimensions สะท้อนของจริงใน repository: content_format กับ category เท่านั้น
// style_preset วัดผลได้แต่ไม่มีตารางน้ำหนักให้หมุน
func (f *fakeRepo) TunableDimensions() []string { return []string{"category", "content_format"} }

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
	// service เรียก RecordRevisions แยกทีละมิติ (revs ทั้งชุดเป็นมิติเดียวกันเสมอ)
	// จึงเช็ค revs[0].Dimension พอเพื่อจำลอง "audit ล่มแค่มิติเดียว"
	if f.failRecordDim != "" && len(revs) > 0 && revs[0].Dimension == f.failRecordDim {
		return errors.New("audit ล่มเฉพาะมิตินี้")
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

// TestTuneOnceSkipsStylePreset ต้องพิสูจน์การ "กันไว้" ไม่ใช่แค่บังเอิญไม่มีอะไรให้ทำ
// จึงต้องมี weight + คะแนนที่ต่างกันพอให้ขยับจริงทั้งสองมิติ (style_preset และ
// content_format ที่หมุนได้) ถ้า service เผลอเอา style_preset เข้า TunableDimensions
// test นี้ต้อง fail เพราะ repo.applied จะมี key "style_preset" โผล่มา
func TestTuneOnceSkipsStylePreset(t *testing.T) {
	repo := newFakeRepo()
	repo.weights["style_preset"] = map[string]int{"case-file": 50, "detective-board": 50}
	repo.weights["content_format"] = map[string]int{"qa": 50, "news": 50}
	repo.latestScore = []Score{
		{Stat: Stat{Dimension: "style_preset", Value: "case-file", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
		{Stat: Stat{Dimension: "style_preset", Value: "detective-board", Platform: "youtube", N: 30}, ScoreFinal: 0.2},
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
		{Stat: Stat{Dimension: "content_format", Value: "news", Platform: "youtube", N: 30}, ScoreFinal: 0.2},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	if err := svc.TuneOnce(context.Background()); err != nil {
		t.Fatalf("TuneOnce error: %v", err)
	}
	// พิสูจน์ว่า loop หมุนน้ำหนักจริง (ไม่ใช่ no-op ทั้งก้อน) ก่อนเช็คการกัน style_preset
	if applied := repo.applied["content_format"]; applied["qa"] <= 50 {
		t.Fatalf("content_format ควรถูกหมุนน้ำหนักจริงในเทสต์นี้ (qa=%d)", applied["qa"])
	}
	if _, ok := repo.applied["style_preset"]; ok {
		t.Error("style_preset ต้องไม่ถูกเขียน weight")
	}
}

// TestTuneOnceContinuesAfterOneDimensionAuditFails ตรึงว่า error ของมิติหนึ่ง
// (RecordRevisions ล่ม) ต้องไม่ทำให้มิติอื่นที่เหลือถูกข้ามไปด้วย — ถ้า implementation
// ใช้ `return err` แทน `continue` ตอนเจอ error รายมิติ มิติที่มาทีหลังใน
// TunableDimensions (category) จะไม่ถูกประมวลผลเลย และ repo.applied["category"]
// จะว่าง ทำให้เทสต์นี้ fail
func TestTuneOnceContinuesAfterOneDimensionAuditFails(t *testing.T) {
	repo := newFakeRepo()
	repo.failRecordDim = "content_format" // มิติแรกใน TunableDimensions ล้มเจตนา
	repo.weights["content_format"] = map[string]int{"qa": 50, "news": 50}
	repo.weights["category"] = map[string]int{"education": 50, "entertainment": 50}
	repo.latestScore = []Score{
		{Stat: Stat{Dimension: "content_format", Value: "qa", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
		{Stat: Stat{Dimension: "content_format", Value: "news", Platform: "youtube", N: 30}, ScoreFinal: 0.2},
		{Stat: Stat{Dimension: "category", Value: "education", Platform: "youtube", N: 30}, ScoreFinal: 0.9},
		{Stat: Stat{Dimension: "category", Value: "entertainment", Platform: "youtube", N: 30}, ScoreFinal: 0.2},
	}
	svc := NewService(repo, fakeSettings{v: "true"}, fixedNow)
	if err := svc.TuneOnce(context.Background()); err != nil {
		t.Fatalf("TuneOnce error: %v — มิติหนึ่งล้มต้องไม่ทำให้ทั้งฟังก์ชัน error", err)
	}
	if _, ok := repo.applied["content_format"]; ok {
		t.Error("content_format audit ล้มแล้วห้าม apply")
	}
	if applied := repo.applied["category"]; applied["education"] <= 50 {
		t.Errorf("category ควรถูกหมุนน้ำหนักตามปกติแม้ content_format จะล้ม, ได้ %v", applied)
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
