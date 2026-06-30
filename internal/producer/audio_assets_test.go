package producer

import (
	"io/fs"
	"strings"
	"testing"
)

func TestAudioAssetsEmbedded(t *testing.T) {
	var ambient, sfx int
	err := fs.WalkDir(audioAssetsFS, "assets/audio", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".mp3") {
			return err
		}
		switch {
		case strings.Contains(p, "/ambient/"):
			ambient++
		case strings.Contains(p, "/sfx/transition/"):
			sfx++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embed: %v", err)
	}
	if ambient < 1 {
		t.Errorf("want >=1 ambient bed, got %d", ambient)
	}
	if sfx < 1 {
		t.Errorf("want >=1 transition sfx, got %d", sfx)
	}
}
