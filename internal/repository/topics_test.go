package repository

import (
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

func TestPickTopicCategoryLeastUsed(t *testing.T) {
	usages := []topicUsage{
		{Cat: models.TopicCategory{CategoryName: "a", Weight: 2}, UsedCount: 4}, // ratio 2.0
		{Cat: models.TopicCategory{CategoryName: "b", Weight: 1}, UsedCount: 1}, // ratio 1.0
		{Cat: models.TopicCategory{CategoryName: "c", Weight: 1}, UsedCount: 0}, // ratio 0.0 (lowest)
	}
	got := pickTopicCategoryLeastUsed(usages, nil)
	if got.CategoryName != "c" {
		t.Errorf("expected c, got %s", got.CategoryName)
	}
}

func TestPickTopicCategoryLeastUsedRespectsWeight(t *testing.T) {
	usages := []topicUsage{
		{Cat: models.TopicCategory{CategoryName: "a", Weight: 2}, UsedCount: 2}, // ratio 1.0
		{Cat: models.TopicCategory{CategoryName: "b", Weight: 1}, UsedCount: 2}, // ratio 2.0
	}
	got := pickTopicCategoryLeastUsed(usages, nil)
	if got.CategoryName != "a" {
		t.Errorf("expected a (lower used/weight ratio), got %s", got.CategoryName)
	}
}

func TestPickTopicCategoryLeastUsedExcludesGivenNames(t *testing.T) {
	usages := []topicUsage{
		{Cat: models.TopicCategory{CategoryName: "a", Weight: 1}, UsedCount: 0}, // lowest ratio but excluded
		{Cat: models.TopicCategory{CategoryName: "b", Weight: 1}, UsedCount: 1},
	}
	got := pickTopicCategoryLeastUsed(usages, []string{"a"})
	if got.CategoryName != "b" {
		t.Errorf("expected b (a excluded), got %s", got.CategoryName)
	}
}

func TestPickTopicCategoryLeastUsedAllExcludedFallsBackToFullSet(t *testing.T) {
	usages := []topicUsage{
		{Cat: models.TopicCategory{CategoryName: "a", Weight: 1}, UsedCount: 0},
		{Cat: models.TopicCategory{CategoryName: "b", Weight: 1}, UsedCount: 5},
	}
	got := pickTopicCategoryLeastUsed(usages, []string{"a", "b"})
	// everything excluded -> fallback to full set -> lowest ratio ("a") wins
	if got.CategoryName != "a" {
		t.Errorf("expected fallback to full set picking a, got %s", got.CategoryName)
	}
}

func TestPickTopicCategoryLeastUsedEmpty(t *testing.T) {
	got := pickTopicCategoryLeastUsed(nil, nil)
	if got.CategoryName != "" {
		t.Errorf("expected zero-value category for empty input, got %+v", got)
	}
}

func TestPickTitleArchetypeLeastUsed(t *testing.T) {
	usages := []archetypeUsage{
		{Arch: models.TitleArchetype{ArchetypeName: "shock_number", Weight: 2}, UsedCount: 4}, // ratio 2.0
		{Arch: models.TitleArchetype{ArchetypeName: "warning", Weight: 1}, UsedCount: 0},       // ratio 0.0 (lowest)
	}
	got := pickTitleArchetypeLeastUsed(usages)
	if got.ArchetypeName != "warning" {
		t.Errorf("expected warning, got %s", got.ArchetypeName)
	}
}

func TestPickTitleArchetypeLeastUsedRespectsWeight(t *testing.T) {
	usages := []archetypeUsage{
		{Arch: models.TitleArchetype{ArchetypeName: "shock_number", Weight: 2}, UsedCount: 2}, // ratio 1.0
		{Arch: models.TitleArchetype{ArchetypeName: "warning", Weight: 1}, UsedCount: 2},       // ratio 2.0
	}
	got := pickTitleArchetypeLeastUsed(usages)
	if got.ArchetypeName != "shock_number" {
		t.Errorf("expected shock_number (lower used/weight ratio), got %s", got.ArchetypeName)
	}
}

func TestPickTitleArchetypeLeastUsedEmpty(t *testing.T) {
	got := pickTitleArchetypeLeastUsed(nil)
	if got.ArchetypeName != "" {
		t.Errorf("expected zero-value archetype for empty input, got %+v", got)
	}
}
