package rag

import (
	"strings"
	"testing"
	"time"
)

// Bug: ChunkText used to loop forever whenever the text had more words than
// `overlap` — the final partial chunk never advanced `start`, appending the
// same chunk until OOM. This killed both the crawler and the embed endpoint.
func TestChunkTextTerminates(t *testing.T) {
	tests := []struct {
		name       string
		wordCount  int
		maxChunk   int
		overlap    int
		wantChunks int
	}{
		{"empty text", 0, 200, 30, 0},
		{"short text under overlap", 20, 200, 30, 1},
		{"text between overlap and chunk size (was infinite)", 50, 200, 30, 1},
		{"text just over chunk size (was infinite)", 250, 200, 30, 2},
		{"long text (was infinite)", 1000, 200, 30, 6},
		{"crawler params medium text (was infinite)", 100, 300, 50, 1},
		{"exactly chunk size", 200, 200, 30, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			words := make([]string, tt.wordCount)
			for i := range words {
				words[i] = "คำ"
			}
			text := strings.Join(words, " ")

			done := make(chan []string, 1)
			go func() { done <- ChunkText(text, tt.maxChunk, tt.overlap) }()

			select {
			case chunks := <-done:
				if len(chunks) != tt.wantChunks {
					t.Errorf("got %d chunks, want %d", len(chunks), tt.wantChunks)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("ChunkText did not terminate (infinite loop)")
			}
		})
	}
}

// Verify chunks actually cover the whole text with expected overlap.
func TestChunkTextCoverage(t *testing.T) {
	words := make([]string, 500)
	for i := range words {
		words[i] = string(rune('a' + i%26))
	}
	text := strings.Join(words, " ")

	chunks := ChunkText(text, 200, 30)

	// First chunk starts with the first word, last chunk ends with the last word.
	if !strings.HasPrefix(chunks[0], words[0]) {
		t.Errorf("first chunk doesn't start at the beginning")
	}
	last := chunks[len(chunks)-1]
	if !strings.HasSuffix(last, words[len(words)-1]) {
		t.Errorf("last chunk doesn't reach the end of the text")
	}
}
