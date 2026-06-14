// Command visionsmoke is a THROWAWAY Phase 2 spike: it verifies whether the
// kie.ai /claude/v1/messages proxy forwards Anthropic image content blocks to
// the model. It generates a solid-red PNG, asks the model what color it is,
// and prints the HTTP status + response. If the model answers "red", vision
// works through the proxy and Phase 2 (Visual QA) can proceed. Delete after.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const kieClaudeAPI = "https://api.kie.ai/claude/v1/messages"

func main() {
	ctx := context.Background()

	apiKey := os.Getenv("KIE_API_KEY")
	if apiKey == "" {
		log.Fatal("KIE_API_KEY not set")
	}

	// Solid red 80x80 PNG.
	img := image.NewRGBA(image.Rect(0, 0, 80, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 80; x++ {
			img.Set(x, y, color.RGBA{R: 220, G: 20, B: 20, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatalf("encode png: %v", err)
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	body, _ := json.Marshal(map[string]any{
		"model":      "claude-sonnet-4-6",
		"max_tokens": 100,
		"stream":     false,
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "What single color fills this image? Answer with exactly one word."},
					map[string]any{"type": "image", "source": map[string]any{
						"type": "base64", "media_type": "image/png", "data": b64,
					}},
				},
			},
		},
	})

	req, _ := http.NewRequestWithContext(ctx, "POST", kieClaudeAPI, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	fmt.Printf("HTTP %d\n", resp.StatusCode)
	fmt.Printf("RESPONSE: %s\n", string(respBody))
	if resp.StatusCode == http.StatusOK {
		fmt.Println("\n>>> VISION SMOKE: proxy accepted the image block (check the answer says 'red')")
	} else {
		fmt.Println("\n>>> VISION SMOKE FAILED: proxy rejected the request — Phase 2 approach is BLOCKED")
	}
}
