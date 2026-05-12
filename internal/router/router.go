package router

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jaochai/video-fb/internal/handler"
	"github.com/jaochai/video-fb/internal/progress"
	"github.com/jaochai/video-fb/internal/publisher"
	"github.com/jaochai/video-fb/internal/rag"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(pool *pgxpool.Pool, apiKey string, ragEngine *rag.Engine, tracker *progress.Tracker, pub *publisher.Publisher) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		MaxAge:           300,
	}))
	r.Use(handler.APIKeyAuth(apiKey))

	r.Get("/health", handler.HealthCheck)

	clips := handler.NewClipsHandler(repository.NewClipsRepo(pool))
	r.Route("/api/v1/clips", func(r chi.Router) {
		r.Get("/", clips.List)
		r.Post("/", clips.Create)
		r.Get("/{id}", clips.Get)
		r.Patch("/{id}", clips.Update)
		r.Delete("/{id}", clips.Delete)
	})

	scenes := handler.NewScenesHandler(repository.NewScenesRepo(pool))
	r.Route("/api/v1/clips/{clipId}/scenes", func(r chi.Router) {
		r.Get("/", scenes.ListByClip)
		r.Post("/", scenes.Create)
	})
	r.Delete("/api/v1/scenes/{id}", scenes.Delete)

	knowledge := handler.NewKnowledgeHandler(repository.NewKnowledgeRepo(pool), ragEngine)
	r.Route("/api/v1/knowledge/sources", func(r chi.Router) {
		r.Get("/", knowledge.ListSources)
		r.Post("/", knowledge.CreateSource)
		r.Get("/{id}", knowledge.GetSource)
		r.Put("/{id}", knowledge.UpdateSource)
		r.Patch("/{id}", knowledge.ToggleSource)
		r.Delete("/{id}", knowledge.DeleteSource)
		r.Post("/{id}/embed", knowledge.EmbedSource)
	})

	agentsRepo := repository.NewAgentsRepo(pool)
	agents := handler.NewAgentsHandler(agentsRepo)
	promptHistory := handler.NewPromptHistoryHandler(agentsRepo)
	r.Route("/api/v1/agents", func(r chi.Router) {
		r.Get("/", agents.List)
		r.Get("/prompt-history", promptHistory.List)
		r.Patch("/{id}", agents.Update)
	})

	schedules := handler.NewSchedulesHandler(repository.NewSchedulesRepo(pool))
	r.Route("/api/v1/schedules", func(r chi.Router) {
		r.Get("/", schedules.List)
		r.Patch("/{id}", schedules.Update)
	})

	themes := handler.NewThemesHandler(repository.NewThemesRepo(pool))
	r.Route("/api/v1/themes", func(r chi.Router) {
		r.Get("/", themes.List)
		r.Get("/active", themes.GetActive)
		r.Patch("/{id}", themes.Update)
	})

	analytics := handler.NewAnalyticsHandler(repository.NewAnalyticsRepo(pool), pub)
	r.Get("/api/v1/analytics/summary", analytics.Summary)
	r.Get("/api/v1/clips/{clipId}/analytics", analytics.ListByClip)
	r.Post("/api/v1/analytics/fetch", analytics.Trigger)

	settings := handler.NewSettingsHandler(repository.NewSettingsRepo(pool))
	r.Route("/api/v1/settings", func(r chi.Router) {
		r.Get("/", settings.Get)
		r.Put("/", settings.Update)
		r.Post("/test-key", settings.TestKey)
		r.Get("/test-zernio", settings.TestZernio)
	})

	prod := handler.NewProductionHandler(tracker)
	r.Get("/api/v1/production/status", prod.GetStatus)

	return r
}

func SetOrchestrator(r *chi.Mux, h *handler.OrchestratorHandler) {
	r.Post("/api/v1/orchestrator/produce", h.TriggerWeekly)
	r.Post("/api/v1/orchestrator/stop", h.StopProduction)
	r.Post("/api/v1/orchestrator/publish", h.TriggerPublish)
	r.Post("/api/v1/orchestrator/retry", h.RetryFailed)
}
