package router

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jaochai/video-fb/internal/handler"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(pool *pgxpool.Pool, apiKey string) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
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

	knowledge := handler.NewKnowledgeHandler(repository.NewKnowledgeRepo(pool))
	r.Route("/api/v1/knowledge/sources", func(r chi.Router) {
		r.Get("/", knowledge.ListSources)
		r.Post("/", knowledge.CreateSource)
		r.Patch("/{id}", knowledge.ToggleSource)
		r.Delete("/{id}", knowledge.DeleteSource)
	})

	agents := handler.NewAgentsHandler(repository.NewAgentsRepo(pool))
	r.Route("/api/v1/agents", func(r chi.Router) {
		r.Get("/", agents.List)
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

	analytics := handler.NewAnalyticsHandler(repository.NewAnalyticsRepo(pool))
	r.Get("/api/v1/clips/{clipId}/analytics", analytics.ListByClip)

	return r
}

func SetOrchestrator(r *chi.Mux, h *handler.OrchestratorHandler) {
	r.Post("/api/v1/orchestrator/produce", h.TriggerWeekly)
}
