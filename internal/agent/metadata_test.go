package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// Locks the prompt↔struct contract for the seeded `metadata` prompt (migration
// 030): it must emit youtube_title / youtube_description / youtube_tags, which
// must unmarshal into GeneratedMetadata.
func TestMetadataOutputParsesSeededSchema(t *testing.T) {
	raw := `{
	  "youtube_title": "บัญชีโฆษณาโดนแบน แก้ยังไง",
	  "youtube_description": "วิธีกันบัญชีโฆษณาโดนแบน และสิ่งที่ต้องทำก่อนสาย",
	  "youtube_tags": ["บัญชีโฆษณา", "โดนแบน", "facebook ads", "ยิงแอด"]
	}`

	var md GeneratedMetadata
	if err := json.Unmarshal([]byte(raw), &md); err != nil {
		t.Fatalf("seeded metadata JSON did not unmarshal into GeneratedMetadata: %v", err)
	}
	if md.YoutubeTitle != "บัญชีโฆษณาโดนแบน แก้ยังไง" {
		t.Errorf("YoutubeTitle = %q", md.YoutubeTitle)
	}
	if md.YoutubeDescription == "" {
		t.Errorf("YoutubeDescription is empty")
	}
	if len(md.YoutubeTags) != 4 || md.YoutubeTags[0] != "บัญชีโฆษณา" {
		t.Errorf("YoutubeTags = %v", md.YoutubeTags)
	}
}

// Guards MetadataTemplateData field names against the §4.6 registry
// (Topic, Script, Category, AudiencePersona).
func TestMetadataTemplateRendersRegistryVars(t *testing.T) {
	tmpl := "topic={{.Topic}} cat={{.Category}} script={{.Script}} persona={{.AudiencePersona}}"
	out, err := renderTemplate(tmpl, MetadataTemplateData{
		Topic:           "TOPIC",
		Script:          "SCRIPT",
		Category:        "CAT",
		AudiencePersona: "PERSONA",
	})
	if err != nil {
		t.Fatalf("renderTemplate err: %v", err)
	}
	for _, want := range []string{"topic=TOPIC", "cat=CAT", "script=SCRIPT", "persona=PERSONA"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q: %s", want, out)
		}
	}
}
