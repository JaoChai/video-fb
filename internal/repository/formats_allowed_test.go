package repository

import (
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

// filterAllowed เป็นตัวกรองบริสุทธิ์ เทสต์ได้โดยไม่ต้องมี DB — กติกาที่ห้ามพลาดคือ
// "ห้ามคืน format นอกชุดที่ slot อนุญาต" เพราะ format คือตัวกำหนดโหมดของคลิป
func TestFilterAllowedKeepsOnlyListed(t *testing.T) {
	usages := []models.FormatUsage{
		{Format: models.ContentFormat{FormatName: "qa", Weight: 1}, UsedCount: 5},
		{Format: models.ContentFormat{FormatName: "tips", Weight: 1}, UsedCount: 1},
		{Format: models.ContentFormat{FormatName: "case_story", Weight: 1}, UsedCount: 0},
	}
	got := filterAllowed(usages, []string{"qa", "tips"})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for _, u := range got {
		if u.Format.FormatName == "case_story" {
			t.Error("case_story is not in the allowed list but survived the filter")
		}
	}
}

func TestFilterAllowedEmptyListMeansNoRestriction(t *testing.T) {
	usages := []models.FormatUsage{
		{Format: models.ContentFormat{FormatName: "qa", Weight: 1}},
	}
	if len(filterAllowed(usages, nil)) != 1 {
		t.Error("nil allowed list must mean no restriction (manual /produce keeps working)")
	}
}

// ถ้ากรองแล้วไม่เหลืออะไร ต้องไม่เงียบๆ คืนตัวแรกของคลังทั้งหมด
func TestFilterAllowedNoMatchReturnsEmpty(t *testing.T) {
	usages := []models.FormatUsage{
		{Format: models.ContentFormat{FormatName: "qa", Weight: 1}},
	}
	if len(filterAllowed(usages, []string{"news"})) != 0 {
		t.Error("no match must return empty so the caller can fail loudly")
	}
}
