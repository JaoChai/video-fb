package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/rag"
)

type SimilarityMatch struct {
	Similarity   float64
	MatchedTitle string
}

type rejectedQuestion struct {
	Question GeneratedQuestion
	Match    SimilarityMatch
}

// filterBySimilarity splits questions into passed/rejected based on their
// highest similarity to past topics. Pure function — testable without DB.
func filterBySimilarity(questions []GeneratedQuestion, similarities map[string]SimilarityMatch, threshold float64) (passed []GeneratedQuestion, rejected []rejectedQuestion) {
	for _, q := range questions {
		match, ok := similarities[q.Question]
		if ok && match.Similarity >= threshold {
			rejected = append(rejected, rejectedQuestion{Question: q, Match: match})
			continue
		}
		passed = append(passed, q)
	}
	return passed, rejected
}

// Deduper checks generated questions against past topics using pgvector.
type Deduper struct {
	pool      *pgxpool.Pool
	rag       *rag.Engine
	threshold float64 // default 0.78 (legacy); orchestrator set 0.72 เมื่อ flag on
}

func NewDeduper(pool *pgxpool.Pool, ragEngine *rag.Engine) *Deduper {
	return &Deduper{pool: pool, rag: ragEngine, threshold: 0.78}
}

// SetThreshold — orchestrator เรียกเมื่อ flag on (ค่าจาก setting dedup_threshold).
// ค่า <= 0 ไม่มีผล (กันเขียนทับด้วยค่าไร้สาระ).
func (d *Deduper) SetThreshold(t float64) {
	if t > 0 {
		d.threshold = t
	}
}

// CheckQuestions returns the highest-similarity past topic for each question,
// along with the question's embedding (so callers can store it).
func (d *Deduper) CheckQuestions(ctx context.Context, questions []GeneratedQuestion) (map[string]SimilarityMatch, map[string][]float64, error) {
	similarities := make(map[string]SimilarityMatch, len(questions))
	embeddings := make(map[string][]float64, len(questions))

	for _, q := range questions {
		emb, err := d.rag.GenerateEmbedding(ctx, q.Question)
		if err != nil {
			return nil, nil, fmt.Errorf("embed question: %w", err)
		}
		embeddings[q.Question] = emb

		var title string
		var similarity float64
		err = d.pool.QueryRow(ctx,
			`SELECT title, 1 - (embedding <=> $1::vector) AS similarity
			 FROM topic_history
			 WHERE embedding IS NOT NULL
			 ORDER BY embedding <=> $1::vector
			 LIMIT 1`,
			rag.FormatVector(emb)).Scan(&title, &similarity)
		if err != nil {
			// No past topics with embeddings yet — nothing to compare against.
			log.Printf("Dedup: no comparable past topics for %q: %v", q.Question, err)
			continue
		}
		similarities[q.Question] = SimilarityMatch{Similarity: similarity, MatchedTitle: title}
	}
	return similarities, embeddings, nil
}

// PainPointInCooldown — true ถ้า pain_point นี้เคยปรากฏใน topic_history ใน N วันล่าสุด.
// กัน "หัวข้อเดิมเปลี่ยนมุม" ที่ embedding จับไม่ได้. painPoint ว่าง/days<=0 → false.
func (d *Deduper) PainPointInCooldown(ctx context.Context, painPoint string, days int) (bool, error) {
	if painPoint == "" || days <= 0 {
		return false, nil
	}
	var n int
	err := d.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM topic_history
		 WHERE pain_point = $1 AND created_at > NOW() - make_interval(days => $2)`,
		painPoint, days).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("pain_point cooldown query: %w", err)
	}
	return n > 0, nil
}

// LexicalCheck — fallback เมื่อ embedding ล่ม. ใช้ pg_trgm similarity() เทียบ 30 title ล่าสุด.
// คืน map[question]true สำหรับ question ที่มี similarity > 0.5 กับ title เก่าอย่างน้อย 1 ตัว.
func (d *Deduper) LexicalCheck(ctx context.Context, questions []GeneratedQuestion) (map[string]bool, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT title FROM topic_history ORDER BY created_at DESC LIMIT 30`)
	if err != nil {
		return nil, fmt.Errorf("query recent titles for lexical check: %w", err)
	}
	defer rows.Close()
	past := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		past = append(past, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, q := range questions {
		for _, p := range past {
			var sim float64
			if err := d.pool.QueryRow(ctx, `SELECT similarity($1, $2)`, q.Question, p).Scan(&sim); err != nil {
				continue // ข้าม pair ที่ error ไม่ block มั่ว
			}
			if sim > 0.5 {
				out[q.Question] = true
				break
			}
		}
	}
	return out, nil
}
