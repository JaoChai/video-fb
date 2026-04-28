# Granular Production Resume Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Skip already-completed production steps (voice, images, assembly) on retry by checking if output files already exist locally.

**Architecture:** Before each step in `producer.Produce()`, check if the output file exists. If it does, log that we're skipping and move to the next step. Upload step always runs (cheap, and URLs need to be captured). This works within the same Railway container session — if the server redeploys, files are lost and steps re-run (acceptable).

**Tech Stack:** Go, os.Stat for file existence checks

---

### Task 1: Add file-exists helper

**Files:**
- Modify: `internal/producer/producer.go`

- [ ] **Step 1: Add `fileExists` helper function**

Add at the bottom of `producer.go`:

```go
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: success (no output)

---

### Task 2: Skip voice generation if voice.mp3 exists

**Files:**
- Modify: `internal/producer/producer.go` (inside `Produce` method, lines 66-74)

- [ ] **Step 1: Wrap voice generation with file check**

Replace the voice generation block (lines 66-74):

```go
	p.tracker.StartStep("voice")
	voice := p.getVoice(ctx)
	log.Printf("Generating voice for %s (voice: %s)", clipID, voice)
	voicePath := filepath.Join(clipDir, "voice.mp3")
	if err := p.kie.GenerateVoice(ctx, voiceScript, voice, voicePath); err != nil {
		p.tracker.FailStep("voice", err)
		return nil, fmt.Errorf("generate voice: %w", err)
	}
	p.tracker.CompleteStep("voice")
```

With:

```go
	voicePath := filepath.Join(clipDir, "voice.mp3")
	if fileExists(voicePath) {
		log.Printf("Skipping voice for %s (file exists)", clipID)
		p.tracker.StartStep("voice")
		p.tracker.CompleteStep("voice")
	} else {
		p.tracker.StartStep("voice")
		voice := p.getVoice(ctx)
		log.Printf("Generating voice for %s (voice: %s)", clipID, voice)
		if err := p.kie.GenerateVoice(ctx, voiceScript, voice, voicePath); err != nil {
			p.tracker.FailStep("voice", err)
			return nil, fmt.Errorf("generate voice: %w", err)
		}
		p.tracker.CompleteStep("voice")
	}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: success

---

### Task 3: Skip image generation if image files exist

**Files:**
- Modify: `internal/producer/producer.go` (inside `Produce` method, lines 76-90)

- [ ] **Step 1: Wrap 16:9 image generation with file check**

Replace the 16:9 image block:

```go
	img169 := filepath.Join(clipDir, "question-16x9.png")
	if err := p.kie.GenerateImage(ctx, prompt.ImagePrompt169, "16:9", img169); err != nil {
		p.tracker.FailStep("images", err)
		return nil, fmt.Errorf("generate 16:9 image: %w", err)
	}
```

With:

```go
	img169 := filepath.Join(clipDir, "question-16x9.png")
	if fileExists(img169) {
		log.Printf("Skipping 16:9 image for %s (file exists)", clipID)
	} else {
		if err := p.kie.GenerateImage(ctx, prompt.ImagePrompt169, "16:9", img169); err != nil {
			p.tracker.FailStep("images", err)
			return nil, fmt.Errorf("generate 16:9 image: %w", err)
		}
	}
```

- [ ] **Step 2: Wrap 9:16 image generation with file check**

Same pattern for the 9:16 image block:

```go
	img916 := filepath.Join(clipDir, "question-9x16.png")
	if fileExists(img916) {
		log.Printf("Skipping 9:16 image for %s (file exists)", clipID)
	} else {
		if err := p.kie.GenerateImage(ctx, prompt.ImagePrompt916, "9:16", img916); err != nil {
			p.tracker.FailStep("images", err)
			return nil, fmt.Errorf("generate 9:16 image: %w", err)
		}
	}
```

- [ ] **Step 3: Move tracker calls around combined image block**

The `p.tracker.StartStep("images")` and `p.tracker.CompleteStep("images")` should wrap both image checks. The current code has StartStep before the first image and CompleteStep after the second. Keep this structure but ensure tracker is called even if both images are skipped.

- [ ] **Step 4: Build check**

Run: `go build ./...`
Expected: success

---

### Task 4: Skip assembly if video files exist

**Files:**
- Modify: `internal/producer/producer.go` (inside `Produce` method, lines 93-107)

- [ ] **Step 1: Wrap assembly with file checks**

Replace the assembly block:

```go
	p.tracker.StartStep("assembly")
	video169 := filepath.Join(clipDir, "video-16x9.mp4")
	log.Printf("Assembling 16:9 video for %s", clipID)
	if err := p.ffmpeg.AssembleSingleImage(img169, voicePath, video169); err != nil {
		return nil, fmt.Errorf("assemble 16:9: %w", err)
	}

	video916 := filepath.Join(clipDir, "video-9x16.mp4")
	log.Printf("Assembling 9:16 video for %s", clipID)
	if err := p.ffmpeg.AssembleSingleImageVertical(img916, voicePath, video916); err != nil {
		return nil, fmt.Errorf("assemble 9:16: %w", err)
	}

	thumbPath := img169
	p.tracker.CompleteStep("assembly")
```

With:

```go
	p.tracker.StartStep("assembly")
	video169 := filepath.Join(clipDir, "video-16x9.mp4")
	if fileExists(video169) {
		log.Printf("Skipping 16:9 assembly for %s (file exists)", clipID)
	} else {
		log.Printf("Assembling 16:9 video for %s", clipID)
		if err := p.ffmpeg.AssembleSingleImage(img169, voicePath, video169); err != nil {
			return nil, fmt.Errorf("assemble 16:9: %w", err)
		}
	}

	video916 := filepath.Join(clipDir, "video-9x16.mp4")
	if fileExists(video916) {
		log.Printf("Skipping 9:16 assembly for %s (file exists)", clipID)
	} else {
		log.Printf("Assembling 9:16 video for %s", clipID)
		if err := p.ffmpeg.AssembleSingleImageVertical(img916, voicePath, video916); err != nil {
			return nil, fmt.Errorf("assemble 9:16: %w", err)
		}
	}

	thumbPath := img169
	p.tracker.CompleteStep("assembly")
```

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: success

---

### Task 5: Verify and commit

**Files:**
- All changes in `internal/producer/producer.go`

- [ ] **Step 1: Full build check**

Run: `go build ./...`
Expected: success

- [ ] **Step 2: Review the final diff**

Run: `git diff internal/producer/producer.go`
Verify: each production step (voice, image 16:9, image 9:16, assembly 16:9, assembly 9:16) has a `fileExists` check that skips generation if the file is present.

- [ ] **Step 3: Commit and push**

```bash
git add internal/producer/producer.go
git commit -m "feat: skip completed production steps on retry by checking local files

Voice, images, and assembly steps now check if their output file
already exists before regenerating. Saves Kie AI credits and time
when retrying a clip that partially completed production.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
git push origin master
```
