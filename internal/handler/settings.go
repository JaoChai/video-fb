package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/producer"
	"github.com/jaochai/video-fb/internal/repository"
)

type SettingsHandler struct {
	repo *repository.SettingsRepo
}

func NewSettingsHandler(repo *repository.SettingsRepo) *SettingsHandler {
	return &SettingsHandler{repo: repo}
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.repo.GetAll(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}

	masked := make(map[string]string)
	for k, v := range settings {
		if strings.Contains(k, "api_key") && v != "" {
			masked[k] = maskKey(v)
		} else {
			masked[k] = v
		}
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: masked})
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request"})
		return
	}

	allowed := map[string]bool{
		"openrouter_api_key":        true,
		"kie_api_key":               true,
		"elevenlabs_voice":          true,
		"zernio_api_key":            true,
		"zernio_youtube_account_id": true,
	}

	for k, v := range req {
		if !allowed[k] {
			continue
		}
		if k == "elevenlabs_voice" && v != "" && !producer.ValidVoices[strings.ToLower(v)] {
			writeJSON(w, http.StatusBadRequest, models.APIResponse{
				Error: fmt.Sprintf("Invalid voice: %s", v),
			})
			return
		}
		if err := h.repo.Set(r.Context(), k, v); err != nil {
			writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "settings updated"})
}

func (h *SettingsHandler) TestZernio(w http.ResponseWriter, r *http.Request) {
	key, err := h.repo.Get(r.Context(), "zernio_api_key")
	if err != nil || key == "" {
		writeJSON(w, http.StatusOK, models.APIResponse{Error: "zernio_api_key not set"})
		return
	}

	httpReq, err := http.NewRequestWithContext(r.Context(), "GET", "https://zernio.com/api/v1/accounts", nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: "failed to create request"})
		return
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, models.APIResponse{Error: "failed to connect to Zernio"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result any
	json.Unmarshal(body, &result)
	writeJSON(w, resp.StatusCode, models.APIResponse{Data: result})
}

func (h *SettingsHandler) TestKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "key is required"})
		return
	}

	httpReq, err := http.NewRequestWithContext(r.Context(), "GET", "https://openrouter.ai/api/v1/key", nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: "failed to create request"})
		return
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", req.Key))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, models.APIResponse{Error: "failed to connect to OpenRouter"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusOK, models.APIResponse{Error: "invalid API key", Data: map[string]any{"status": resp.StatusCode}})
		return
	}

	var result map[string]any
	json.Unmarshal(body, &result)
	writeJSON(w, http.StatusOK, models.APIResponse{Data: result})
}
