package producer

import (
	"math"
	"strings"

	"github.com/jaochai/video-fb/internal/agent"
)

// captionMaxRunes caps one on-screen caption phrase. Thai has no spaces between
// words (only between phrases), so we pack phrases on existing whitespace up to
// this length and only hard-split a single run when it exceeds it. The template
// fits text to width, so this is a readability target, not a hard layout limit.
const captionMaxRunes = 42

// captionSegmentsFromScenes builds caption segments from the GROUND-TRUTH scene
// VoiceText — the exact text we sent to TTS — instead of re-transcribing the
// audio with ASR (Whisper), which mangled Thai vowels (สระ) and words. Each
// scene's [start,end) audio window is divided across its phrases in proportion
// to rune count, so captions are correct by construction and roughly in sync.
//
// scenes and bounds are matched by index; the shorter slice wins so a mismatch
// never panics. Scenes with empty text or a zero-width window produce nothing.
func captionSegmentsFromScenes(scenes []agent.GeneratedScene, bounds []sceneBound) []TranscriptSegment {
	n := len(scenes)
	if nb := len(bounds); nb < n {
		n = nb
	}

	var segs []TranscriptSegment
	for i := 0; i < n; i++ {
		text := strings.TrimSpace(scenes[i].VoiceText)
		b := bounds[i]
		if text == "" || b.End <= b.Start {
			continue
		}

		phrases := splitCaptionPhrases(text)
		weights := make([]int, len(phrases))
		total := 0
		for j, ph := range phrases {
			weights[j] = len([]rune(ph))
			total += weights[j]
		}
		if total == 0 {
			continue
		}

		span := b.End - b.Start
		cursor := b.Start
		for j, ph := range phrases {
			start := cursor
			end := start + span*float64(weights[j])/float64(total)
			if j == len(phrases)-1 {
				end = b.End // pin the last phrase to the boundary; kills float drift
			}
			// หา emphasis จากข้อความดิบก่อน แล้วค่อยประกบคำทับศัพท์ด้วย ZWSP —
			// สลับลำดับแล้วคำเน้นที่เป็นคำทับศัพท์จะจับคู่ไม่เจอ
			emph := emphasisInPhrase(scenes[i].EmphasisWords, ph)
			segs = append(segs, TranscriptSegment{
				Text:     GuardLoanWords(ph),
				Start:    math.Round(start*100) / 100,
				End:      math.Round(end*100) / 100,
				Emphasis: emph,
			})
			cursor = end
		}
	}
	return segs
}

// splitCaptionPhrases breaks one scene's narration into caption-sized phrases.
// It packs whitespace-separated tokens (Thai uses spaces between phrases, not
// words) up to captionMaxRunes.
//
// A single token longer than captionMaxRunes stays WHOLE. เดิมโค้ดนี้หั่นมันตาม
// จำนวนตัวอักษร ซึ่งตัดกลางคำแล้วทำให้ผู้ชมเห็นคำขาดข้ามเฟรม ("…แอดมินอ" ค้าง
// จนจบวลี แล้ววลีถัดไปขึ้นต้นด้วย "ยู่…") — backtest 2026-07-26 พบว่าเกิดกับ
// 20% ของซีน. ปล่อยให้ Chromium ขึ้นบรรทัดเองดีกว่า เพราะมันรู้ขอบเขตคำไทย
// (ส่วนคำทับศัพท์ที่มันไม่รู้จัก GuardLoanWords ช่วยประกบไว้อีกชั้น)
func splitCaptionPhrases(text string) []string {
	var phrases []string
	var cur []rune

	flush := func() {
		if len(cur) > 0 {
			phrases = append(phrases, string(cur))
			cur = cur[:0]
		}
	}

	for _, tok := range strings.Fields(text) {
		tr := []rune(tok)

		// Flush the current line if adding this token would overflow it.
		if len(cur) > 0 && len(cur)+1+len(tr) > captionMaxRunes {
			flush()
		}
		if len(cur) > 0 {
			cur = append(cur, ' ')
		}
		cur = append(cur, tr...)
	}
	flush()
	return phrases
}

// emphasisInPhrase returns the subset of emphasis words that appear in phrase,
// so each caption segment only carries the emphasis it can actually highlight.
func emphasisInPhrase(words []string, phrase string) []string {
	var out []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w != "" && strings.Contains(phrase, w) {
			out = append(out, w)
		}
	}
	return out
}
