package producer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// R2Client uploads media to a Cloudflare R2 bucket and returns permanent public
// URLs (served via the bucket's custom domain). It mirrors KieClient: config is
// read lazily from the settings table so credentials can rotate — and the whole
// feature can be toggled — without a redeploy.
type R2Client struct {
	pool *pgxpool.Pool
}

func NewR2Client(pool *pgxpool.Pool) *R2Client {
	return &R2Client{pool: pool}
}

type r2Config struct {
	accountID, accessKey, secretKey, bucket, publicBaseURL string
}

// loadConfig reads all R2 settings in one query. Missing/empty rows yield zero
// values, which Enabled/Upload treat as "not configured".
func (r *R2Client) loadConfig(ctx context.Context) (r2Config, bool, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT key, value FROM settings WHERE key IN
		 ('r2_account_id','r2_access_key_id','r2_secret_access_key','r2_bucket','r2_public_base_url','r2_storage_enabled')`)
	if err != nil {
		return r2Config{}, false, fmt.Errorf("load r2 settings: %w", err)
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return r2Config{}, false, err
		}
		m[k] = v
	}
	if err := rows.Err(); err != nil {
		return r2Config{}, false, err
	}
	cfg := r2Config{
		accountID:     m["r2_account_id"],
		accessKey:     m["r2_access_key_id"],
		secretKey:     m["r2_secret_access_key"],
		bucket:        m["r2_bucket"],
		publicBaseURL: strings.TrimRight(m["r2_public_base_url"], "/"),
	}
	enabled := m["r2_storage_enabled"] == "true" &&
		cfg.accountID != "" && cfg.accessKey != "" && cfg.secretKey != "" &&
		cfg.bucket != "" && cfg.publicBaseURL != ""
	return cfg, enabled, nil
}

func (r *R2Client) Enabled(ctx context.Context) bool {
	_, enabled, err := r.loadConfig(ctx)
	return err == nil && enabled
}

func (r *R2Client) s3Client(ctx context.Context, cfg r2Config) (*s3.Client, error) {
	awsCfg, err := awscfg.LoadDefaultConfig(ctx,
		awscfg.WithRegion("auto"),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.accessKey, cfg.secretKey, "")),
		// R2 does not support the SDK's default (CRC32) request/response checksums;
		// only calculate when an operation strictly requires it.
		awscfg.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		awscfg.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.accountID)
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	}), nil
}

// Upload puts localPath at key in the bucket and returns its permanent public URL.
// contentType may be empty to infer from the file extension.
func (r *R2Client) Upload(ctx context.Context, localPath, key, contentType string) (string, error) {
	cfg, enabled, err := r.loadConfig(ctx)
	if err != nil {
		return "", err
	}
	if !enabled {
		return "", fmt.Errorf("r2 not enabled/configured")
	}
	f, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", localPath, err)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", localPath, err)
	}

	client, err := r.s3Client(ctx, cfg)
	if err != nil {
		return "", err
	}
	if contentType == "" {
		contentType = contentTypeFor(localPath)
	}
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(cfg.bucket),
		Key:           aws.String(key),
		Body:          f,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(stat.Size()),
	})
	if err != nil {
		return "", fmt.Errorf("r2 put %s: %w", key, err)
	}
	return cfg.publicBaseURL + "/" + key, nil
}

func contentTypeFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4":
		return "video/mp4"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}
