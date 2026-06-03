package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaochai/video-fb/internal/rag"
)

// similarityThreshold: questions with >= this cosine similarity to any past
// topic are considered semantic duplicates and rejected.
// Calibrated against real data: known duplicate pairs in production scored
// 0.81-0.82, while genuinely different angles scored 0.69-0.75.
const similarityThreshold = 0.78

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
	pool *pgxpool.Pool
	rag  *rag.Engine
}

func NewDeduper(pool *pgxpool.Pool, ragEngine *rag.Engine) *Deduper {
	return &Deduper{pool: pool, rag: ragEngine}
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
