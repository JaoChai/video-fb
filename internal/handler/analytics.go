package handler

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type analyticsFetcher interface {
	FetchAnalytics(ctx context.Context) error
}

type AnalyticsHandler struct {
	repo      *repository.AnalyticsRepo
	publisher analyticsFetcher
	fetching  sync.Mutex
}

func NewAnalyticsHandler(repo *repository.AnalyticsRepo, publisher analyticsFetcher) *AnalyticsHandler {
	return &AnalyticsHandler{repo: repo, publisher: publisher}
}

func (h *AnalyticsHandler) ListByClip(w http.ResponseWriter, r *http.Request) {
	clipID := chi.URLParam(r, "clipId")
	analytics, err := h.repo.ListByClip(r.Context(), clipID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: analytics})
}

func (h *AnalyticsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	rangeParam := r.URL.Query().Get("range")
	days := 30
	switch rangeParam {
	case "7d":
		days = 7
	case "30d", "":
		days = 30
	case "all":
		days = 3650
	}

	ctx := r.Context()
	summary, err := h.repo.Summary(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	topClips, err := h.repo.TopClips(ctx, 10)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	byPostType, err := h.repo.SummaryByPostType(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	byPlatform, err := h.repo.SummaryByPlatform(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	trend, err := h.repo.Trend(ctx, days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	prev, err := h.repo.PreviousPeriodTotals(ctx, days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	lastFetched, _ := h.repo.LastFetchedAt(ctx)

	delta := computeDelta(summary, prev)

	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{
		"summary":         summary,
		"top_clips":       topClips,
		"by_post_type":    byPostType,
		"by_platform":     byPlatform,
		"trend":           trend,
		"delta":           delta,
		"range_days":      days,
		"last_fetched_at": lastFetched,
	}})
}

func (h *AnalyticsHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	if !h.fetching.TryLock() {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: "fetch already in progress"})
		return
	}
	go func() {
		defer h.fetching.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := h.publisher.FetchAnalytics(ctx); err != nil {
			log.Printf("Manual FetchAnalytics failed: %v", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, models.APIResponse{Data: map[string]string{"status": "triggered"}})
}

func computeDelta(cur, prev models.AnalyticsSummary) models.DeltaSummary {
	pct := func(c, p int) float64 {
		if p == 0 {
			if c == 0 {
				return 0
			}
			return 100
		}
		return (float64(c) - float64(p)) / float64(p) * 100
	}
	pctF := func(c, p float64) float64 {
		if p == 0 {
			if c == 0 {
				return 0
			}
			return 100
		}
		return (c - p) / p * 100
	}
	return models.DeltaSummary{
		Views:          pct(cur.TotalViews, prev.TotalViews),
		Likes:          pct(cur.TotalLikes, prev.TotalLikes),
		Comments:       pct(cur.TotalComments, prev.TotalComments),
		Shares:         pct(cur.TotalShares, prev.TotalShares),
		WatchTime:      pctF(cur.TotalWatchTime, prev.TotalWatchTime),
		RetentionPoint: (cur.AvgRetention - prev.AvgRetention) * 100,
	}
}
