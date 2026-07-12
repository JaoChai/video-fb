package orchestrator

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func TestPickCategoryWeighted(t *testing.T) {
	categories := []string{"account", "payment", "campaign"}
	scores := []models.CategoryScore{
		{Category: "payment", AvgPercentile: 0.8, N: 4},
		{Category: "account", AvgPercentile: 0.3, N: 5},
		{Category: "retired-category", AvgPercentile: 0.99, N: 9}, // not configured → ignored
	}

	exploit := func(int) int { return 0 }  // rng(100)=0 < 50 → exploit
	explore := func(int) int { return 99 } // rng(100)=99 ≥ 50 → round-robin

	if got := PickCategoryWeighted(categories, scores, 7, exploit); got != "payment" {
		t.Errorf("exploit pick = %q, want payment (best configured category)", got)
	}
	if got := PickCategoryWeighted(categories, scores, 7, explore); got != categories[7%3] {
		t.Errorf("explore pick = %q, want round-robin %q", got, categories[7%3])
	}
	if got := PickCategoryWeighted(categories, nil, 7, exploit); got != categories[7%3] {
		t.Errorf("no-scores pick = %q, want round-robin fallback", got)
	}
}

func TestFormatTopicStats(t *testing.T) {
	if got := FormatTopicStats(nil); got != "" {
		t.Errorf("empty scores should render empty string, got %q", got)
	}
	out := FormatTopicStats([]models.CategoryScore{
		{Category: "payment", AvgPercentile: 0.8, AvgViews: 95, N: 4},
	})
	for _, want := range []string{"payment", "95", "80", "4", "ครึ่งหนึ่ง"} {
		if !strings.Contains(out, want) {
			t.Errorf("stats missing %q:\n%s", want, out)
		}
	}
}

func TestPickClipRole_RatioDistribution(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	convert := 0
	n := 1000
	for i := 0; i < n; i++ {
		if PickClipRole(0.30, rng) == "convert" {
			convert++
		}
	}
	// ~30% ± 5%
	if convert < n*25/100 || convert > n*35/100 {
		t.Errorf("convert ratio out of range: %d/%d", convert, n)
	}
}

func TestPickClipRole_Boundaries(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	if got := PickClipRole(0, rng); got != "reach" {
		t.Errorf("ratio=0 → want reach, got %s", got)
	}
	if got := PickClipRole(1, rng); got != "convert" {
		t.Errorf("ratio=1 → want convert, got %s", got)
	}
}

func TestPickPersona_NonEmpty(t *testing.T) {
	personas := []string{"media buyer", "owner", "agency", "banned"}
	rng := rand.New(rand.NewSource(4))
	got := PickPersona(personas, rng)
	if got == "" {
		t.Fatal("empty persona")
	}
	// ต้องเป็นหนึ่งใน list
	found := false
	for _, p := range personas {
		if p == got {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("persona %q not in list", got)
	}
}

func TestPickPersona_EmptyList(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	if got := PickPersona(nil, rng); got != "" {
		t.Errorf("empty list → want empty string, got %q", got)
	}
}

func TestRoleInstruction(t *testing.T) {
	reach := RoleInstruction("reach")
	convert := RoleInstruction("convert")

	if reach == "" {
		t.Fatal("reach instruction should be non-empty")
	}
	if !strings.Contains(reach, "ทุกระดับ") {
		t.Errorf("reach instruction missing distinctive keyword ทุกระดับ:\n%s", reach)
	}

	if convert == "" {
		t.Fatal("convert instruction should be non-empty")
	}
	if !strings.Contains(convert, "ทักแชท") {
		t.Errorf("convert instruction missing distinctive keyword ทักแชท:\n%s", convert)
	}

	if reach == convert {
		t.Error("reach and convert instructions must be distinct")
	}

	if got := RoleInstruction(""); got != "" {
		t.Errorf("empty role → want empty string, got %q", got)
	}
	if got := RoleInstruction("garbage"); got != "" {
		t.Errorf("unknown role → want empty string, got %q", got)
	}
}
