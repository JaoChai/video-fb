package producer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	return f.assembleSingleWithScale(imagePath, audioPath, outputPath, 1920, 1080)
}

func (f *FFmpegAssembler) AssembleSingleImageVertical(imagePath, audioPath, outputPath string) error {
	return f.assembleSingleWithScale(imagePath, audioPath, outputPath, 1080, 1920)
}

func (f *FFmpegAssembler) assembleSingleWithScale(imagePath, audioPath, outputPath string, width, height int) error {
	dir := filepath.Dir(outputPath)
	os.MkdirAll(dir, 0755)

	vf := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1", width, height, width, height)

	args := []string{
		"-loop", "1", "-i", imagePath,
		"-i", audioPath,
		"-vf", vf,
		"-c:v", "libx264", "-preset", "medium", "-crf", "23",
		"-c:a", "aac", "-b:a", "128k",
		"-pix_fmt", "yuv420p",
		"-shortest",
		"-y", outputPath,
	}

	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	return nil
}

// ExtractThumbnail writes a single PNG thumbnail from videoPath at outPath.
// It seeks ~1.5s in (past the intro fade) so the thumbnail is a real content
// frame, not the near-black opening frame; -update 1 writes one image without
// ffmpeg's image-sequence-pattern warning.
func (f *FFmpegAssembler) ExtractThumbnail(videoPath, outPath string) error {
	os.MkdirAll(filepath.Dir(outPath), 0755)
	args := []string{"-ss", "1.5", "-i", videoPath, "-frames:v", "1", "-update", "1", "-y", outPath}
	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg thumbnail failed: %w", err)
	}
	return nil
}

// ExtractFrameAt writes a single PNG frame from videoPath at tsSeconds into the
// timeline. It is the generalized form of ExtractThumbnail (which is fixed at
// 1.5s). Used by Visual QA to grab one representative frame per scene. A
// negative tsSeconds is clamped to 0.
func (f *FFmpegAssembler) ExtractFrameAt(videoPath, outPath string, tsSeconds float64) error {
	if tsSeconds < 0 {
		tsSeconds = 0
	}
	os.MkdirAll(filepath.Dir(outPath), 0755)
	args := []string{"-ss", fmt.Sprintf("%.3f", tsSeconds), "-i", videoPath, "-frames:v", "1", "-update", "1", "-y", outPath}
	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extract frame at %.3fs failed: %w", tsSeconds, err)
	}
	return nil
}

// ProbeDurationSeconds returns the container duration of videoPath in seconds
// via ffprobe. QA frame sampling uses it to place frames on the real timeline
// instead of trusting per-scene duration estimates (which are often 0).
func (f *FFmpegAssembler) ProbeDurationSeconds(videoPath string) (float64, error) {
	args := []string{"-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", videoPath}
	out, err := exec.Command(ffprobePath(f.ffmpegPath), args...).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration failed: %w", err)
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration parse %q: %w", strings.TrimSpace(string(out)), err)
	}
	return d, nil
}

// ffprobePath derives the ffprobe binary path from the ffmpeg path (they ship
// together), falling back to "ffprobe" on PATH when ffmpegPath is bare or has
// no "ffmpeg" segment to rewrite.
func ffprobePath(ffmpegPath string) string {
	if ffmpegPath == "" || ffmpegPath == "ffmpeg" {
		return "ffprobe"
	}
	dir, base := filepath.Dir(ffmpegPath), filepath.Base(ffmpegPath)
	probeBase := strings.Replace(base, "ffmpeg", "ffprobe", 1)
	if probeBase == base {
		return "ffprobe"
	}
	return filepath.Join(dir, probeBase)
}

// BuildAmbientBed loops srcPath to at least durationSec, trims to exactly
// durationSec, and applies a 1.5s tail fade so the bed ends cleanly under the
// outro. Output is an mp3 at outPath. Used for the per-clip background ambient.
func (f *FFmpegAssembler) BuildAmbientBed(srcPath, outPath string, durationSec float64) error {
	if durationSec <= 0 {
		return fmt.Errorf("durationSec must be > 0, got %v", durationSec)
	}
	os.MkdirAll(filepath.Dir(outPath), 0755)
	fadeStart := durationSec - 1.5
	if fadeStart < 0 {
		fadeStart = 0
	}
	args := []string{
		"-stream_loop", "-1", "-i", srcPath,
		"-t", fmt.Sprintf("%.3f", durationSec),
		"-af", fmt.Sprintf("afade=t=out:st=%.3f:d=1.5", fadeStart),
		"-ar", "44100", "-b:a", "128k",
		"-y", outPath,
	}
	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg ambient bed failed: %w", err)
	}
	return nil
}
