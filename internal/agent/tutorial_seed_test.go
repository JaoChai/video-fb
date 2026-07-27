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
	// ทุกไฟล์ seed ต้องถูกตรวจ ไม่ใช่แค่ไฟล์แรก — แถวที่เติมทีหลังพังเงียบได้เท่ากัน
	for _, seed := range []struct {
		file string
		want int
	}{
		{"../../migrations/062_tutorial_catalog_seed.sql", 8},
		{"../../migrations/068_tutorial_catalog_seed_v2.sql", 10},
	} {
		raw, err := os.ReadFile(seed.file)
		if err != nil {
			t.Fatalf("read seed migration %s: %v", seed.file, err)
		}
		matches := seedRe.FindAllStringSubmatch(string(raw), -1)
		if len(matches) != seed.want {
			t.Fatalf("%s: found %d seeded features, want %d", seed.file, len(matches), seed.want)
		}
		checkSeedRows(t, seed.file, matches)
	}
}

func checkSeedRows(t *testing.T, file string, matches [][]string) {
	t.Helper()
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
			t.Fatalf("%s feature %d: steps is not valid JSON: %v", file, i, err)
		}
		if len(steps) < 3 || len(steps) > 5 {
			t.Errorf("%s feature %d: %d steps, want 3-5 (see spec §6)", file, i, len(steps))
		}
		for _, s := range steps {
			if s.TitleTH == "" {
				t.Errorf("%s feature %d step %d: empty title_th", file, i, s.N)
			}
			if !vocab[strings.ToLower(strings.TrimSpace(s.UITarget))] {
				t.Errorf("%s feature %d step %d: ui_target %q missing from ui_vocab", file, i, s.N, s.UITarget)
			}
		}
	}
}
