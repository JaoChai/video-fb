package handler

import (
	"context"
	"encoding/json"
	"fmt"
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
	summaries, err := h.repo.ListSourceSummaries(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: summaries})
}

func (h *KnowledgeHandler) GetSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	source, err := h.repo.GetSourceByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "source not found"})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: source})
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


func (h *KnowledgeHandler) EmbedSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	source, err := h.repo.GetSourceByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "source not found"})
		return
	}

	log.Printf("Starting embed for source %s (%d chars)", id, len(source.Content))
	content := source.Content
	go func() {
		n, err := h.rebuildChunks(id, content)
		if err != nil {
			log.Printf("Embed source %s failed: %v", id, err)
		} else {
			log.Printf("Embedded source %s: %d chunks", id, n)
		}
	}()
	writeJSON(w, http.StatusAccepted, models.APIResponse{Message: "Embedding started in background"})
}

func (h *KnowledgeHandler) rebuildChunks(sourceID, content string) (int, error) {
	ctx := context.Background()

	if err := h.repo.DeleteChunksBySource(ctx, sourceID); err != nil {
		return 0, fmt.Errorf("delete chunks: %w", err)
	}

	if strings.TrimSpace(content) == "" {
		return 0, nil
	}

	chunks := rag.ChunkText(content, 200, 30)
	stored := 0
	for _, chunk := range chunks {
		if len(strings.Fields(chunk)) < 10 {
			continue
		}
		embedding, err := h.engine.GenerateEmbedding(ctx, chunk)
		if err != nil {
			return stored, fmt.Errorf("embedding chunk %d: %w", stored+1, err)
		}
		if err := h.engine.StoreChunk(ctx, sourceID, chunk, "", embedding); err != nil {
			return stored, fmt.Errorf("store chunk %d: %w", stored+1, err)
		}
		stored++
	}
	log.Printf("Embedded %d/%d chunks for source %s", stored, len(chunks), sourceID)
	return stored, nil
}
