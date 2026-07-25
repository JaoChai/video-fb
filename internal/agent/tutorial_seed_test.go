package agent

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// seedRe จับคู่ (ui_vocab, steps json) ของแต่ละแถวใน migration 062.
var seedRe = regexp.MustCompile(`(?s)\$vocab\$(.*?)\$vocab\$.*?\$steps\$(.*?)\$steps\$`)

// TestSeedStepsCoveredByUIVocab กันความพังที่เงียบที่สุดของฟีเจอร์นี้: ถ้า ui_target
// ของขั้นตอนไหนไม่อยู่ใน ui_vocab ของแถวเดียวกัน gate ตอนรันจริงจะตีคลิปตกทุกใบ
// โดยไม่มีใครรู้ว่าสาเหตุอยู่ที่ seed ไม่ใช่ที่โมเดล
func TestSeedStepsCoveredByUIVocab(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/062_tutorial_catalog_seed.sql")
	if err != nil {
		t.Fatalf("read seed migration: %v", err)
	}
	matches := seedRe.FindAllStringSubmatch(string(raw), -1)
	if len(matches) != 8 {
		t.Fatalf("found %d seeded features, want 8", len(matches))
	}
	for i, m := range matches {
		vocab := map[string]bool{}
		for _, v := range strings.Split(m[1], "|") {
			if v = strings.TrimSpace(v); v != "" {
				vocab[strings.ToLower(v)] = true
			}
		}
		var steps []struct {
			N        int    `json:"n"`
			UITarget string `json:"ui_target"`
			TitleTH  string `json:"title_th"`
		}
		if err := json.Unmarshal([]byte(m[2]), &steps); err != nil {
			t.Fatalf("feature %d: steps is not valid JSON: %v", i, err)
		}
		if len(steps) < 3 || len(steps) > 5 {
			t.Errorf("feature %d: %d steps, want 3-5 (see spec §6)", i, len(steps))
		}
		for _, s := range steps {
			if s.TitleTH == "" {
				t.Errorf("feature %d step %d: empty title_th", i, s.N)
			}
			if !vocab[strings.ToLower(strings.TrimSpace(s.UITarget))] {
				t.Errorf("feature %d step %d: ui_target %q missing from ui_vocab", i, s.N, s.UITarget)
			}
		}
	}
}
