package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/rag"
	"github.com/jaochai/video-fb/internal/repository"
)

type KnowledgeHandler struct {
	repo   *repository.KnowledgeRepo
	engine *rag.Engine
}

func NewKnowledgeHandler(repo *repository.KnowledgeRepo, engine *rag.Engine) *KnowledgeHandler {
	return &KnowledgeHandler{repo: repo, engine: engine}
}

func (h *KnowledgeHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.repo.ListSources(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: sources})
}

func (h *KnowledgeHandler) CreateSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Content  string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	source, err := h.repo.CreateSource(r.Context(), req.Name, req.Category, req.Content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}

	if strings.TrimSpace(req.Content) != "" {
		go h.embedSource(source.ID, req.Content)
	}

	writeJSON(w, http.StatusCreated, models.APIResponse{Data: source})
}

func (h *KnowledgeHandler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Content  string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	if err := h.repo.UpdateSource(r.Context(), id, req.Name, req.Category, req.Content); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}

	go h.rebuildChunks(id, req.Content)

	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}

func (h *KnowledgeHandler) ToggleSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct{ Enabled bool `json:"enabled"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}
	if err := h.repo.ToggleSource(r.Context(), id, req.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "updated"})
}

func (h *KnowledgeHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteSource(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "deleted"})
}

func (h *KnowledgeHandler) RebuildAll(w http.ResponseWriter, r *http.Request) {
	sources, err := h.repo.ListSources(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}

	go func() {
		for _, s := range sources {
			if !s.Enabled || strings.TrimSpace(s.Content) == "" {
				continue
			}
			h.rebuildChunks(s.ID, s.Content)
		}
		log.Println("Rebuild all embeddings complete")
	}()

	writeJSON(w, http.StatusOK, models.APIResponse{Message: "rebuilding embeddings in background"})
}

func (h *KnowledgeHandler) embedSource(sourceID, content string) {
	h.rebuildChunks(sourceID, content)
}

func (h *KnowledgeHandler) rebuildChunks(sourceID, content string) {
	ctx := context.Background()

	if err := h.repo.DeleteChunksBySource(ctx, sourceID); err != nil {
		log.Printf("delete chunks for %s: %v", sourceID, err)
		return
	}

	if strings.TrimSpace(content) == "" {
		return
	}

	chunks := rag.ChunkText(content, 200, 30)
	stored := 0
	for _, chunk := range chunks {
		if len(strings.Fields(chunk)) < 10 {
			continue
		}
		embedding, err := h.engine.GenerateEmbedding(ctx, chunk)
		if err != nil {
			log.Printf("embedding failed: %v", err)
			continue
		}
		if err := h.engine.StoreChunk(ctx, sourceID, chunk, "", embedding); err != nil {
			log.Printf("store chunk failed: %v", err)
			continue
		}
		stored++
	}
	log.Printf("Embedded %d/%d chunks for source %s", stored, len(chunks), sourceID)
}
