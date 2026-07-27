package handler

import (
	"net/http"

	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
	"github.com/jaochai/video-fb/internal/scoreboard"
)

type ScoreboardHandler struct {
	repo *repository.FormulaScoresRepo
	svc  *scoreboard.Service
}

func NewScoreboardHandler(repo *repository.FormulaScoresRepo, svc *scoreboard.Service) *ScoreboardHandler {
	return &ScoreboardHandler{repo: repo, svc: svc}
}

// Latest คืนกระดานคะแนนใบล่าสุด
func (h *ScoreboardHandler) Latest(w http.ResponseWriter, r *http.Request) {
	computedAt, scores, err := h.repo.Latest(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	rows := []models.FormulaScoreRow{} // non-nil so an empty result marshals to [] not null
	for _, s := range scores {
		rows = append(rows, models.FormulaScoreRow{
			ComputedAt: computedAt, Dimension: s.Dimension, Value: s.Value,
			Platform: s.Platform, N: s.N, MedianPct: s.MedianPct,
			MedianRetention: s.MedianRetention, FlopRate: s.FlopRate,
			ScoreFinal: s.ScoreFinal,
		})
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{
		"computed_at": computedAt,
		"scores":      rows,
	}})
}

// Compute คำนวณ snapshot ใหม่ทันทีโดยไม่แตะ weight — ใช้ตรวจก่อนเปิดสวิตช์
func (h *ScoreboardHandler) Compute(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.ComputeSnapshot(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{"rows": n}})
}

// Revisions คืนประวัติการหมุนน้ำหนัก 20 รายการล่าสุด
func (h *ScoreboardHandler) Revisions(w http.ResponseWriter, r *http.Request) {
	revs, err := h.repo.ListRevisions(r.Context(), 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: revs})
}

// Rollback คืน weight ของรอบล่าสุดกลับเป็นค่าเดิม
func (h *ScoreboardHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.Rollback(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.APIResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, models.APIResponse{Data: map[string]any{"restored": n}})
}
