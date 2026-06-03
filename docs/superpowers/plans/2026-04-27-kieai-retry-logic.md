# Kie AI Retry Logic Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add automatic retry with exponential backoff to Kie AI API calls so temporary errors (HTTP 500, 429, network errors) don't crash the entire video production pipeline.

**Architecture:** Add a `retryableGenerate` helper in `kieai.go` that wraps the full create→poll→download cycle. Retryable errors (500, 429, timeout, network) trigger up to 3 retries with exponential backoff. Permanent errors (401, 402, 422) fail immediately. No new files — all changes in `kieai.go` only.

**Tech Stack:** Go 1.25, standard library only (no external retry libraries)

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/producer/kieai.go` | Modify | Add `isRetryable()`, `retryableGenerate()`, update `GenerateImage` + `GenerateVoice` + `UploadFile` |

No new files needed — retry is a cross-cutting concern within the Kie client only.

---

### Task 1: Add `isRetryable` error classifier

**Files:**
- Modify: `internal/producer/kieai.go` (add function after `extractFirstURL`, line ~215)

- [ ] **Step 1: Add the `isRetryable` function**

Add after `extractFirstURL` function (line 215):

```go
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	for _, pattern := range []string{
		"500", "Internal Error",
		"429", "rate limit",
		"timeout", "timed out",
		"connection refused", "connection reset",
		"EOF", "broken pipe",
	} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}
```

Note: `strings` is already imported in `producer.go` (same package), but NOT in `kieai.go`. Need to add `"strings"` to the import block in `kieai.go`.

- [ ] **Step 2: Add `"strings"` to imports in kieai.go**

Add `"strings"` to the import block in `kieai.go` (between `"path/filepath"` and `"time"`).

- [ ] **Step 3: Build check**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/producer/kieai.go
git commit -m "feat: add isRetryable error classifier for Kie AI API"
```

---

### Task 2: Add `retryableGenerate` wrapper

**Files:**
- Modify: `internal/producer/kieai.go` (add function after `isRetryable`)

- [ ] **Step 1: Add the `retryableGenerate` function**

Add after `isRetryable`:

```go
const maxRetries = 3

func (k *KieClient) retryableGenerate(ctx context.Context, operation string, generate func() error) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 10 * time.Second
			log.Printf("[retry] %s attempt %d/%d after %v (error: %v)", operation, attempt, maxRetries, backoff, lastErr)
			select {
			case <-ctx.Done():
				return fmt.Errorf("%s cancelled during retry: %w", operation, ctx.Err())
			case <-time.After(backoff):
			}
		}

		lastErr = generate()
		if lastErr == nil {
			if attempt > 0 {
				log.Printf("[retry] %s succeeded on attempt %d", operation, attempt)
			}
			return nil
		}

		if !isRetryable(lastErr) {
			return lastErr
		}
	}
	return fmt.Errorf("%s failed after %d retries: %w", operation, maxRetries, lastErr)
}
```

Design notes:
- `generate func() error` — caller passes the full create→poll→download cycle as a closure
- Backoff: attempt 1 = 10s, attempt 2 = 40s, attempt 3 = 90s
- Context-aware: stops retry if server is shutting down
- Logs every retry with attempt number and previous error
- Non-retryable errors (401, 402, 422) return immediately without retry

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/producer/kieai.go
git commit -m "feat: add retryableGenerate wrapper with exponential backoff"
```

---

### Task 3: Wrap `GenerateImage` with retry

**Files:**
- Modify: `internal/producer/kieai.go:69-90` (rewrite `GenerateImage`)

- [ ] **Step 1: Rewrite `GenerateImage` to use retry**

Replace the current `GenerateImage` function (lines 69-90) with:

```go
func (k *KieClient) GenerateImage(ctx context.Context, prompt, aspectRatio, outputPath string) error {
	return k.retryableGenerate(ctx, "generate-image", func() error {
		taskID, err := k.createTask(ctx, "gpt-image-2-text-to-image", map[string]any{
			"prompt":       prompt,
			"aspect_ratio": aspectRatio,
			"resolution":   "2K",
		})
		if err != nil {
			return fmt.Errorf("create image task: %w", err)
		}

		result, err := k.pollTask(ctx, taskID, 180*time.Second)
		if err != nil {
			return fmt.Errorf("poll image task: %w", err)
		}

		imageURL := extractFirstURL(result)
		if imageURL == "" {
			return fmt.Errorf("no image URL in result: %v", result)
		}

		return k.downloadFile(ctx, imageURL, outputPath)
	})
}
```

Key: the entire create→poll→download cycle is inside the closure, so a retry starts a fresh Kie AI task.

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/producer/kieai.go
git commit -m "feat: wrap GenerateImage with retry logic"
```

---

### Task 4: Wrap `GenerateVoice` with retry

**Files:**
- Modify: `internal/producer/kieai.go:92-113` (rewrite `GenerateVoice`)

- [ ] **Step 1: Rewrite `GenerateVoice` to use retry**

Replace the current `GenerateVoice` function with:

```go
func (k *KieClient) GenerateVoice(ctx context.Context, text, voice, outputPath string) error {
	return k.retryableGenerate(ctx, "generate-voice", func() error {
		taskID, err := k.createTask(ctx, "elevenlabs/text-to-dialogue-v3", map[string]any{
			"dialogue":      []map[string]string{{"text": text, "voice": voice}},
			"language_code": "th",
			"stability":     0.5,
		})
		if err != nil {
			return fmt.Errorf("create voice task: %w", err)
		}

		result, err := k.pollTask(ctx, taskID, 300*time.Second)
		if err != nil {
			return fmt.Errorf("poll voice task: %w", err)
		}

		audioURL := extractFirstURL(result)
		if audioURL == "" {
			return fmt.Errorf("no audio URL in result: %v", result)
		}

		return k.downloadFile(ctx, audioURL, outputPath)
	})
}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/producer/kieai.go
git commit -m "feat: wrap GenerateVoice with retry logic"
```

---

### Task 5: Wrap `UploadFile` with retry

**Files:**
- Modify: `internal/producer/kieai.go:256-306` (rewrite `UploadFile`)

- [ ] **Step 1: Extract upload logic into retryable closure**

Replace the current `UploadFile` function with:

```go
func (k *KieClient) UploadFile(ctx context.Context, localPath, uploadPath string) (string, error) {
	var fileURL string
	err := k.retryableGenerate(ctx, "upload-file", func() error {
		apiKey, err := k.getAPIKey(ctx)
		if err != nil {
			return err
		}

		f, err := os.Open(localPath)
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		defer f.Close()

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)

		part, err := writer.CreateFormFile("file", filepath.Base(localPath))
		if err != nil {
			return fmt.Errorf("create form file: %w", err)
		}
		if _, err := io.Copy(part, f); err != nil {
			return fmt.Errorf("copy file data: %w", err)
		}

		if uploadPath != "" {
			writer.WriteField("uploadPath", uploadPath)
		}
		writer.Close()

		req, err := http.NewRequestWithContext(ctx, "POST", kieFileUploadAPI+"/file-stream-upload", &body)
		if err != nil {
			return fmt.Errorf("create upload request: %w", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := k.uploadClient.Do(req)
		if err != nil {
			return fmt.Errorf("upload file: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var result kieUploadResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return fmt.Errorf("parse upload response: %w (body: %s)", err, string(respBody[:min(len(respBody), 200)]))
		}
		if !result.Success {
			return fmt.Errorf("upload failed: %s (code: %d)", result.Msg, result.Code)
		}
		fileURL = result.Data.FileURL
		return nil
	})
	return fileURL, err
}
```

Note: `UploadFile` returns `(string, error)` not just `error`, so we use a captured `fileURL` variable.

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/producer/kieai.go
git commit -m "feat: wrap UploadFile with retry logic"
```

---

### Task 6: Final verification — deploy and test

- [ ] **Step 1: Run full build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 2: Push and deploy**

```bash
git push origin master
```

Wait for GitHub Actions to complete both `deploy-backend` and `deploy-frontend`.

- [ ] **Step 3: Verify health**

```bash
curl -s https://adsvance-v2-production.up.railway.app/health
```

Expected: `{"status":"ok"}`

- [ ] **Step 4: Trigger test production**

```bash
curl -s -X POST "https://adsvance-v2-production.up.railway.app/api/v1/orchestrator/produce" \
  -H "Authorization: adsvance-v2-api-key-2026" \
  -H "Content-Type: application/json" \
  -d '{"count": 1}'
```

- [ ] **Step 5: Monitor for retry logs**

Check Railway logs for `[retry]` entries — if any appear, retry logic is working. If production completes without retries, that's also fine (means no errors occurred).

---

## Summary of changes

| What | Before | After |
|------|--------|-------|
| Kie AI 500 error | Clip fails immediately | Retry up to 3× with backoff (10s, 40s, 90s) |
| Rate limit (429) | Clip fails immediately | Retry up to 3× with backoff |
| Network timeout | Clip fails immediately | Retry up to 3× with backoff |
| Auth error (401) | Clip fails immediately | Fail immediately (no retry — permanent) |
| Credits (402) | Clip fails immediately | Fail immediately (no retry — permanent) |
| Validation (422) | Clip fails immediately | Fail immediately (no retry — permanent) |
| Upload failure | Clip fails immediately | Retry up to 3× with backoff |
| Human intervention | Required on temp errors | **Not needed** — fully automatic |

## Risk assessment

- **Low risk:** Changes are isolated to `kieai.go` only — no changes to orchestrator, producer logic, or database
- **Backoff prevents abuse:** Exponential backoff (10s→40s→90s) won't overwhelm Kie AI API
- **Context cancellation:** Retry stops immediately if server shuts down
- **Cost:** Max 4× API calls per generation in worst case — acceptable since failures already cost credits
