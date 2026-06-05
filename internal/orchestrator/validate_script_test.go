package orchestrator

import (
	"strings"
	"testing"

	"github.com/jaochai/video-fb/internal/agent"
)

func TestValidateScriptTitle(t *testing.T) {
	const suffix = " | Ads Vance"
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "LLM added curly brand → no dup, no mid-cut",
			in:   "เพิ่มงบแอดแล้วโดนแบน แก้ยังไง? {Ads Vance}",
			want: "เพิ่มงบแอดแล้วโดนแบน แก้ยังไง?" + suffix,
		},
		{
			name: "LLM added pipe brand → single suffix",
			in:   "บัญชีโฆษณาโดนแบน แก้ยังไง? | Ads Vance",
			want: "บัญชีโฆษณาโดนแบน แก้ยังไง?" + suffix,
		},
		{
			name: "double brand → collapsed to one",
			in:   "เพิ่มงบแล้วโดนแบน {Ads Vance} | Ads Vance",
			want: "เพิ่มงบแล้วโดนแบน" + suffix,
		},
		{
			name: "no brand → suffix appended",
			in:   "Pixel ไม่นับ Lead แก้ด้วยวิธีนี้",
			want: "Pixel ไม่นับ Lead แก้ด้วยวิธีนี้" + suffix,
		},
		{
			name: "paren brand variant",
			in:   "BM โดนระงับ ทำไงดี (Ads Vance)",
			want: "BM โดนระงับ ทำไงดี" + suffix,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &agent.GeneratedScript{YoutubeTitle: tt.in}
			validateScript(s)
			if s.YoutubeTitle != tt.want {
				t.Errorf("got %q, want %q", s.YoutubeTitle, tt.want)
			}
		})
	}
}

func TestValidateScriptTitleLength(t *testing.T) {
	const suffix = " | Ads Vance"
	longTitle := strings.Repeat("ก", 100) + " {Ads Vance}"
	s := &agent.GeneratedScript{YoutubeTitle: longTitle}
	validateScript(s)

	if l := len([]rune(s.YoutubeTitle)); l > 70 {
		t.Errorf("title length = %d runes, want <= 70", l)
	}
	if !strings.HasSuffix(s.YoutubeTitle, suffix) {
		t.Errorf("title %q must end with exactly one brand suffix", s.YoutubeTitle)
	}
	if strings.Count(s.YoutubeTitle, "Ads Vance") != 1 {
		t.Errorf("title %q must contain brand exactly once", s.YoutubeTitle)
	}
	core := strings.TrimSuffix(s.YoutubeTitle, suffix)
	if strings.HasSuffix(core, "{") || strings.HasSuffix(core, "(") || strings.HasSuffix(core, "|") {
		t.Errorf("title core %q has dangling separator", core)
	}
}
