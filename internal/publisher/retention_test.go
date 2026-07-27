package publisher

import (
	"testing"

	"github.com/jaochai/video-fb/internal/models"
)

// รอบนี้วัดได้จริง → ต้องใช้ของรอบนี้ทั้งชุด
func TestMergeRetention_FreshWins(t *testing.T) {
	fresh := models.RetentionSnapshot{WatchTimeSeconds: 180, RetentionRate: 0.04, AvgViewPercentage: 0.75}
	prev := models.RetentionSnapshot{WatchTimeSeconds: 60, RetentionRate: 0.01, AvgViewPercentage: 0.30}
	got := mergeRetention(fresh, prev)
	if got != fresh {
		t.Errorf("รอบนี้วัดได้ต้องใช้ของรอบนี้: got %+v, want %+v", got, fresh)
	}
}

// นี่คือบั๊กที่กำลังแก้: รอบนี้ว่าง ต้องไม่เขียนศูนย์ทับของเดิม
func TestMergeRetention_EmptyFetchKeepsPrevious(t *testing.T) {
	fresh := models.RetentionSnapshot{}
	prev := models.RetentionSnapshot{WatchTimeSeconds: 120, RetentionRate: 0.02, AvgViewPercentage: 0.536}
	got := mergeRetention(fresh, prev)
	if got != prev {
		t.Errorf("รอบว่างต้องไม่ทับค่าเดิม: got %+v, want %+v", got, prev)
	}
}

// คลิปใหม่หรือ TikTok ที่ไม่เคยมี retention เลย → ศูนย์ ไม่ใช่ error
func TestMergeRetention_NeverMeasuredReturnsZero(t *testing.T) {
	got := mergeRetention(models.RetentionSnapshot{}, models.RetentionSnapshot{})
	if got != (models.RetentionSnapshot{}) {
		t.Errorf("got %+v, want zero", got)
	}
}

// เคสขอบ: viewSum เป็น 0 แต่มีนาทีที่ดู (แทบเป็นไปไม่ได้ แต่ถ้าเกิด ต้องยกชุดเดิมมาทั้งชุด
// ไม่ผสมกับของใหม่ ไม่งั้นจะได้ watch_time ของรอบนี้คู่กับ retention ของอีกรอบ)
func TestMergeRetention_SwapsWholeSetNeverMixes(t *testing.T) {
	fresh := models.RetentionSnapshot{WatchTimeSeconds: 300, AvgViewPercentage: 0}
	prev := models.RetentionSnapshot{WatchTimeSeconds: 120, RetentionRate: 0.02, AvgViewPercentage: 0.536}
	got := mergeRetention(fresh, prev)
	if got != prev {
		t.Errorf("ต้องยกชุด prev ทั้งชุด: got %+v, want %+v", got, prev)
	}
}
