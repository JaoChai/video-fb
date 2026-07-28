package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jaochai/video-fb/internal/models"
	"github.com/jaochai/video-fb/internal/repository"
)

// clipSource คือสองอย่างที่ต้องได้จาก ClipsRepo — แยกเป็น interface เพื่อให้
// เทสต์ 404 รันได้โดยไม่ต้องมี Postgres
type clipSource interface {
	GetByID(ctx context.Context, id string) (*models.Clip, error)
	GetMetadata(ctx context.Context, clipID string) (*models.ClipMetadata, error)
}

type ClipDetailHandler struct {
	clips       clipSource
	scenes      *repository.ScenesRepo
	qa          *repository.VisualQARepo
	critiques   *repository.CritiquesRepo
	autoReviews *repository.AutoReviewsRepo
	analytics   *repository.AnalyticsRepo
	debates     *repository.ScriptDebatesRepo
}

func NewClipDetailHandler(
	clips clipSource,
	scenes *repository.ScenesRepo,
	qa *repository.VisualQARepo,
	critiques *repository.CritiquesRepo,
	autoReviews *repository.AutoReviewsRepo,
	analytics *repository.AnalyticsRepo,
	debates *repository.ScriptDebatesRepo,
) *ClipDetailHandler {
	return &ClipDetailHandler{clips, scenes, qa, critiques, autoReviews, analytics, debates}
}

// Get คืนทุกอย่างที่ระบบรู้เกี่ยวกับคลิปหนึ่งตัวในก้อนเดียว หน้า /clips/:id
// ใช้ครบทุกก้อนเสมอ การแยกเป็น 7 request จึงได้แค่ loading state 7 อัน
//
// คลิปไม่มีจริง = 404 ส่วนก้อนอื่นที่ query พัง = null/[] ไม่ลาก response ทั้งก้อนตายไปด้วย
// เพราะคลิปเก่าจำนวนมากไม่มี critique/debate/analytics อยู่แล้ว
func (h *ClipDetailHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "clipId")

	clip, err := h.clips.GetByID(ctx, id)
	if err != nil || clip == nil {
		writeJSON(w, http.StatusNotFound, models.APIResponse{Error: "clip not found"})
		return
	}

	metadata, err := h.clips.GetMetadata(ctx, id)
	logPartial(id, "metadata", err)
	scenes, err := h.scenes.ListByClip(ctx, id)
	logPartial(id, "scenes", err)
	qa, err := h.qa.GetLatestByClipID(ctx, id)
	logPartial(id, "visual qa", err)
	critique, err := h.critiques.GetByClip(ctx, id)
	logPartial(id, "critique", err)
	autoReview, err := h.autoReviews.GetByClip(ctx, id)
	logPartial(id, "auto review", err)
	analytics, err := h.analytics.LatestByClip(ctx, id)
	logPartial(id, "analytics", err)
	debate, err := h.debates.GetByClip(ctx, id)
	logPartial(id, "script debate", err)

	writeJSON(w, http.StatusOK, models.APIResponse{
		Data: normalizeClipDetail(models.ClipDetail{
			Clip:         clip,
			Metadata:     metadata,
			Scenes:       scenes,
			VisualQA:     qa,
			Critique:     critique,
			AutoReview:   autoReview,
			Analytics:    analytics,
			ScriptDebate: debate,
		}),
	})
}

// logPartial ทิ้งร่องรอยของก้อนที่ดึงไม่สำเร็จ — หน้ารายละเอียดยอมแสดงไม่ครบ
// ดีกว่าพังทั้งหน้า แต่ถ้าไม่ log ไว้ ก้อนที่หายเพราะ query พังจะดูเหมือน
// "คลิปนี้ไม่มีข้อมูลส่วนนั้น" ทั้งบนหน้าเว็บและใน log
func logPartial(clipID, part string, err error) {
	if err != nil {
		log.Printf("clip detail %s: %s unavailable: %v", clipID, part, err)
	}
}

// normalizeClipDetail บังคับให้ list ที่ว่างเป็น [] ไม่ใช่ null — repository คืน
// []T{} อยู่แล้ว แต่ก้อนที่ query พัง (fail-soft) ยังไหลมาเป็น nil ได้ และ nil
// slice ของ Go serialize เป็น null แล้ว .map() ฝั่ง React พัง
func normalizeClipDetail(d models.ClipDetail) models.ClipDetail {
	if d.Scenes == nil {
		d.Scenes = []models.Scene{}
	}
	if d.Analytics == nil {
		d.Analytics = []models.ClipAnalytics{}
	}
	return d
}
