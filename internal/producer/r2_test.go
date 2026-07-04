package producer

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestContentTypeFor(t *testing.T) {
	cases := map[string]string{
		"/x/output.mp4":    "video/mp4",
		"/x/thumbnail.png": "image/png",
		"/x/thumb.jpg":     "image/jpeg",
		"/x/thumb.jpeg":    "image/jpeg",
		"/x/unknown.bin":   "application/octet-stream",
	}
	for path, want := range cases {
		if got := contentTypeFor(path); got != want {
			t.Errorf("contentTypeFor(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestR2UploadIntegration uploads a tiny file and fetches it back via the public
// URL. Gated: only runs when R2_INTEGRATION_TEST=1 and DATABASE_URL is set (creds
// live in the settings table, and r2_storage_enabled must be 'true').
func TestR2UploadIntegration(t *testing.T) {
	if os.Getenv("R2_INTEGRATION_TEST") != "1" {
		t.Skip("set R2_INTEGRATION_TEST=1 to run")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	r2 := NewR2Client(pool)
	if !r2.Enabled(ctx) {
		t.Skip("r2 not enabled in settings")
	}

	tmp := filepath.Join(t.TempDir(), "hello.mp4")
	if err := os.WriteFile(tmp, []byte("smoke-test-body"), 0644); err != nil {
		t.Fatal(err)
	}
	key := "clips/_smoketest/hello.mp4"
	url, err := r2.Upload(ctx, tmp, key, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("uploaded to %s", url)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s = %d, want 200", url, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "video/mp4" {
		t.Errorf("Content-Type = %q, want video/mp4", ct)
	}
}
