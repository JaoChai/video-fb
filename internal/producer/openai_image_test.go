package producer

import "testing"

func TestGptImageSize(t *testing.T) {
	cases := map[string]string{
		"9:16": "864x1536",
		"16:9": "1536x864",
		"1:1":  "1024x1024",
		"":     "1024x1024",
	}
	for aspect, want := range cases {
		if got := gptImageSize(aspect); got != want {
			t.Errorf("gptImageSize(%q) = %q, want %q", aspect, got, want)
		}
	}
}
