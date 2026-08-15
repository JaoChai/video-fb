package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

type ClipsHandler struct {
	repo *repository.ClipsRepo
}

func NewClipsHandler(repo *repository.ClipsRepo) *ClipsHandler {
	return &ClipsHandler{repo: repo}
}

func (h *ClipsHandler) List(w http.ResponseWriter, r *http.Request) {
	clips, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: clips})
}

func (h *ClipsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	clip, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "clip not found"})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: clip})
}

func (h *ClipsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateClipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request body"})
		return
	}
	clip, err := h.repo.Create(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, models.APIResponse{Data: clip})
}

// readyBlockedReason บอกว่าทำไมคลิปนี้ยังเข้าสถานะ ready ไม่ได้ ("" = เข้าได้)
// สถานะ ready แปลว่า "พร้อมให้รอบส่งหยิบไปอัปขึ้น YouTube" — คลิปที่ยังไม่มีไฟล์วิดีโอ
// เข้าสถานะนี้แล้วจะถูกหยิบไปวนซ้ำทุกรอบโดยส่งอะไรไม่ได้เลย (เหตุ 2026-08-14: คลิปเดียว
// ยึดหัวคิว 21 ชั่วโมง) · ทางออกที่ถูกต้องของคลิปแบบนั้นคือสั่งเรนเดอร์ ไม่ใช่อนุมัติ
func readyBlockedReason(c *models.Clip) string {
	if c == nil {
		return ""
	}
	has := func(u *string) bool { return u != nil && *u != "" }
	if has(c.Video916URL) || has(c.Video169URL) {
		return ""
	}
	return "คลิปนี้ยังไม่มีไฟล์วิดีโอ — สั่งเรนเดอร์ก่อน แล้วคลิปจะเข้าคิวเผยแพร่เอง"
}

func (h *ClipsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req models.UpdateClipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Error: "invalid request body"})
		return
	}
	// 'published' is written only by the publisher (it records a real Zernio
	// upload) and 'producing' only by the orchestrator — a raw PATCH to either
	// would bypass the publish gate / production tracker.
	if req.Status != nil && (*req.Status == "published" || *req.Status == "producing") {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{
			Error: "status '" + *req.Status + "' ตั้งได้โดย pipeline เท่านั้น"})
		return
	}
	if req.Status != nil && *req.Status == "ready" {
		clip, err := h.repo.GetByID(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "clip not found"})
			return
		}
		if reason := readyBlockedReason(clip); reason != "" {
			writeJSON(w, http.StatusConflict, models.APIResponse{Error: reason})
			return
		}
	}
	clip, err := h.repo.Update(r.Context(), id, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: clip})
}

func (h *ClipsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "deleted"})
}

// Unhold lifts an auto-review hold so the clip becomes publishable again. It's the
// manual "override & publish anyway" escape hatch for a clip the auto-reviewer gated.
func (h *ClipsHandler) Unhold(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	clip, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "clip not found"})
		return
	}
	// ปลดกัก = ดันคลิปเป็น ready (ดู ClearAutoReviewHeld) จึงต้องผ่านด่านเดียวกับ PATCH
	if reason := readyBlockedReason(clip); reason != "" {
		writeJSON(w, http.StatusConflict, models.APIResponse{Error: reason})
		return
	}
	if err := h.repo.ClearAutoReviewHeld(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Message: "hold lifted"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1MB limit
	return json.NewDecoder(r.Body).Decode(v)
}
