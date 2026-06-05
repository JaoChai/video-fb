package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jaochai/video-fb/internal/producer"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: mascot-gen <reference.png|jpg>  (needs OPENAI_API_KEY env)")
		os.Exit(1)
	}
	ref := os.Args[1]
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		fmt.Println("OPENAI_API_KEY not set")
		os.Exit(1)
	}
	outDir := producer.MascotAssetDir
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Printf("mkdir: %v\n", err)
		os.Exit(1)
	}
	for _, pose := range producer.MascotPoseNames() {
		fmt.Printf("generating %s ...\n", pose)
		if err := genPose(key, ref, pose, filepath.Join(outDir, pose+".png")); err != nil {
			fmt.Printf("  FAILED %s: %v\n", pose, err)
			os.Exit(1)
		}
	}
	fmt.Println("done — review assets/mascot/*.png then commit")
}

func genPose(key, ref, pose, out string) error {
	refBytes, err := os.ReadFile(ref)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("model", "gpt-image-2")
	w.WriteField("prompt", producer.MascotEditPrompt(pose))
	w.WriteField("background", "transparent")
	w.WriteField("size", "1024x1024")
	w.WriteField("style_intensity", "high")
	fw, err := w.CreateFormFile("image", filepath.Base(ref))
	if err != nil {
		return err
	}
	if _, err := fw.Write(refBytes); err != nil {
		return err
	}
	w.Close()

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/images/edits", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("%d: %s", resp.StatusCode, string(body))
	}
	var r struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return err
	}
	if len(r.Data) == 0 {
		return fmt.Errorf("no image data")
	}
	dec, err := base64.StdEncoding.DecodeString(r.Data[0].B64JSON)
	if err != nil {
		return err
	}
	return os.WriteFile(out, dec, 0644)
}
