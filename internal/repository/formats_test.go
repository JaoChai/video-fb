package repository

import (
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func TestPickLeastUsedFormat(t *testing.T) {
	usages := []models.FormatUsage{
		{Format: models.ContentFormat{FormatName: "qa", Weight: 2}, UsedCount: 4},
		{Format: models.ContentFormat{FormatName: "news", Weight: 1}, UsedCount: 1},
		{Format: models.ContentFormat{FormatName: "tips", Weight: 1}, UsedCount: 2},
		{Format: models.ContentFormat{FormatName: "case_story", Weight: 1}, UsedCount: 0},
	}
	// case_story: 0/1=0 (lowest) → should be picked
	got := pickLeastUsed(usages)
	if got.FormatName != "case_story" {
		t.Errorf("expected case_story, got %s", got.FormatName)
	}
}

func TestPickLeastUsedRespectsWeight(t *testing.T) {
	usages := []models.FormatUsage{
		{Format: models.ContentFormat{FormatName: "qa", Weight: 2}, UsedCount: 2},   // ratio 1.0
		{Format: models.ContentFormat{FormatName: "news", Weight: 1}, UsedCount: 2}, // ratio 2.0
	}
	got := pickLeastUsed(usages)
	if got.FormatName != "qa" {
		t.Errorf("expected qa (lower used/weight ratio), got %s", got.FormatName)
	}
}

func TestPickLeastUsedEmpty(t *testing.T) {
	got := pickLeastUsed(nil)
	if got.FormatName != "qa" {
		t.Errorf("expected fallback qa, got %s", got.FormatName)
	}
}
