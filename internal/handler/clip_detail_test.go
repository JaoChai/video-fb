package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jaochai/video-fb/internal/models"
)

// fakeClipSource ยืนแทน ClipsRepo — เทสต์ทั้งหมดในไฟล์นี้ไม่แตะ Postgres
type fakeClipSource struct {
	clip *models.Clip
	err  error
}

func (f fakeClipSource) GetByID(ctx context.Context, id string) (*models.Clip, error) {
	return f.clip, f.err
}

func (f fakeClipSource) GetMetadata(ctx context.Context, clipID string) (*models.ClipMetadata, error) {
	return nil, nil
}

// คลิปที่ไม่มีจริงต้องได้ 404 ส่วน DB ล่มต้องได้ 500 — หน้าเว็บใช้ status นี้
// เลือกข้อความ ถ้ายุบเป็น 404 หมดผู้ใช้จะอ่านว่า "คลิปถูกลบไปแล้ว" ทุกครั้งที่
// backend มีปัญหา ทั้งสองเส้นต้องคืนก่อนแตะ repo ตัวอื่น (ตัวอื่นเป็น nil ในเทสต์นี้
// ถ้าโค้ดเผลอเรียกจะ panic ให้เห็นทันที)
func TestClipDetailErrorStatuses(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"ไม่มีคลิปนี้", pgx.ErrNoRows, http.StatusNotFound},
		{"DB ล่ม", errors.New("connection refused"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewClipDetailHandler(fakeClipSource{nil, tc.err}, nil, nil, nil, nil, nil, nil)
			r := chi.NewRouter()
			r.Get("/clips/{clipId}/detail", h.Get)

			req := httptest.NewRequest(http.MethodGet, "/clips/abc/detail", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.want {
				t.Errorf("got %d, want %d", w.Code, tc.want)
			}
		})
	}
}

// repo จริง wrap error ด้วย %w — errors.Is ต้องยังเห็น ErrNoRows ผ่านชั้น wrap
// ไม่งั้นคลิปที่ไม่มีจริงจะกลายเป็น 500
func TestClipDetailWrappedNoRowsIs404(t *testing.T) {
	h := NewClipDetailHandler(
		fakeClipSource{nil, fmt.Errorf("get clip abc: %w", pgx.ErrNoRows)},
		nil, nil, nil, nil, nil, nil,
	)
	r := chi.NewRouter()
	r.Get("/clips/{clipId}/detail", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/clips/abc/detail", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}

// คลิปที่ไม่มีข้อมูลส่วนย่อยเลยต้อง serialize ให้ list ว่างเป็น [] (ไม่งั้น
// .map() ฝั่ง React พังทั้งหน้า — เคยเกิดกับ /prompt-history มาแล้ว) ส่วน
// object ที่ไม่มีต้องเป็น null เพื่อให้หน้าเว็บแยกออกว่า "ไม่มีข้อมูล"
// ต่างจาก "มีแต่ว่าง"
func TestNormalizeClipDetailEmptyShape(t *testing.T) {
	d := normalizeClipDetail(models.ClipDetail{Clip: &models.Clip{ID: "c1"}})

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(b)
	for _, want := range []string{
		`"scenes":[]`, `"analytics":[]`,
		`"metadata":null`, `"critique":null`, `"auto_review":null`,
		`"script_debate":null`, `"visual_qa":null`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ขาด %s ใน %s", want, body)
		}
	}
}
