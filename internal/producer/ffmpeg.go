package producer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FFmpegAssembler struct {
	ffmpegPath string
	fontPath   string
}

func NewFFmpegAssembler(ffmpegPath, fontPath string) *FFmpegAssembler {
	return &FFmpegAssembler{ffmpegPath: ffmpegPath, fontPath: fontPath}
}

type AssemblyScene struct {
	ImagePath       string
	DurationSeconds float64
	TextOverlay     string
}

func (f *FFmpegAssembler) Assemble(scenes []AssemblyScene, audioPath, outputPath string) error {
	dir := filepath.Dir(outputPath)
	os.MkdirAll(dir, 0755)

	var inputs []string
	var filterParts []string
	var concat strings.Builder

	for i, scene := range scenes {
		inputs = append(inputs, "-loop", "1", "-t", fmt.Sprintf("%.1f", scene.DurationSeconds), "-i", scene.ImagePath)

		scale := fmt.Sprintf("[%d:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1[v%d]", i, i)
		filterParts = append(filterParts, scale)
		concat.WriteString(fmt.Sprintf("[v%d]", i))
	}

	audioIdx := len(scenes)
	inputs = append(inputs, "-i", audioPath)

	filter := strings.Join(filterParts, ";") + ";" +
		concat.String() + fmt.Sprintf("concat=n=%d:v=1:a=0[vout]", len(scenes))

	args := append(inputs,
		"-filter_complex", filter,
		"-map", "[vout]",
		"-map", fmt.Sprintf("%d:a", audioIdx),
		"-c:v", "libx264", "-preset", "medium", "-crf", "23",
		"-c:a", "aac", "-b:a", "128k",
		"-pix_fmt", "yuv420p",
		"-shortest",
		"-y", outputPath,
	)

	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	return nil
}

func (f *FFmpegAssembler) AssembleSingleImage(imagePath, audioPath, outputPath string) error {
	dir := filepath.Dir(outputPath)
	os.MkdirAll(dir, 0755)

	args := []string{
		"-loop", "1", "-i", imagePath,
		"-i", audioPath,
		"-vf", "scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1",
		"-c:v", "libx264", "-preset", "medium", "-crf", "23",
		"-c:a", "aac", "-b:a", "128k",
		"-pix_fmt", "yuv420p",
		"-shortest",
		"-y", outputPath,
	}

	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg single image failed: %w", err)
	}
	return nil
}
