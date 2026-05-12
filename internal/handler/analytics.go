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
	summary, err := h.repo.Summary(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	topClips, err := h.repo.TopClips(r.Context(), 10)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	lastFetched, _ := h.repo.LastFetchedAt(r.Context())
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{
		"summary":         summary,
		"top_clips":       topClips,
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
