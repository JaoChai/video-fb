package publisher

import "testing"

func TestYoutubePlatforms(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		firstComment string
		wantOptions  bool
		wantTitle    string
	}{
		{
			name:         "มีข้อความ ⇒ แนบ platformSpecificData",
			title:        "แอดโดนแบนทำไง",
			firstComment: "ติดต่อทีมงานได้ที่ LINE id : @adsvance",
			wantOptions:  true,
			wantTitle:    "แอดโดนแบนทำไง",
		},
		{
			name:         "ค่าว่าง ⇒ ไม่แนบอะไรเลย",
			title:        "แอดโดนแบนทำไง",
			firstComment: "",
			wantOptions:  false,
		},
		{
			name:         "โพสต์ Shorts ใช้ title ของตัวเอง",
			title:        "แอดโดนแบนทำไง #Shorts",
			firstComment: "ติดต่อทีมงานได้ที่ LINE id : @adsvance",
			wantOptions:  true,
			wantTitle:    "แอดโดนแบนทำไง #Shorts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := youtubePlatforms("acc1", tt.title, tt.firstComment)
			if len(got) != 1 {
				t.Fatalf("expected 1 platform target, got %d", len(got))
			}
			if got[0].Platform != platformYouTube || got[0].AccountID != "acc1" {
				t.Fatalf("expected youtube/acc1, got %+v", got[0])
			}
			if !tt.wantOptions {
				if got[0].PlatformSpecificData != nil {
					t.Fatalf("expected nil PlatformSpecificData, got %+v", got[0].PlatformSpecificData)
				}
				return
			}
			opts := got[0].PlatformSpecificData
			if opts == nil {
				t.Fatalf("expected PlatformSpecificData, got nil")
			}
			if opts.FirstComment != tt.firstComment {
				t.Fatalf("expected firstComment %q, got %q", tt.firstComment, opts.FirstComment)
			}
			if opts.Title != tt.wantTitle {
				t.Fatalf("expected title %q, got %q", tt.wantTitle, opts.Title)
			}
			if opts.Visibility != VisibilityPublic {
				t.Fatalf("expected visibility %q, got %q", VisibilityPublic, opts.Visibility)
			}
		})
	}
}
