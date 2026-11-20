// Package artifacts is the off-chain registry for ABIs and metadata: an
// S3-compatible store addressed as <bucket>/<prefix>/<name>. It follows the
// contract used by the codedeploy plugin (path-style endpoint, bucket without
// '/', prefix joined with '/'), so MinIO, the plugin's OSS/S3 and the demo
// server all behave the same.
package artifacts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Store wraps a bucket and a key prefix.
type Store struct {
	cli    *minio.Client
	Bucket string
	Prefix string
}

// Open connects, makes the bucket when missing, and returns the store.
func Open(endpoint, access, secret, bucket, prefix string, secure bool) (*Store, error) {
	if endpoint == "" || bucket == "" {
		return nil, fmt.Errorf("artifacts: endpoint and bucket are required")
	}
	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(access, secret, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("artifacts: %w", err)
	}
	ctx := context.Background()
	exists, err := cli.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("artifacts: bucket %s: %w", bucket, err)
	}
	if !exists {
		if err := cli.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("artifacts: create bucket %s: %w", bucket, err)
		}
	}
	return &Store{cli: cli, Bucket: bucket, Prefix: prefix}, nil
}

// Key joins the prefix and the name with exactly one slash.
func (s *Store) Key(name string) string {
	p := strings.Trim(s.Prefix, "/")
	n := strings.TrimLeft(name, "/")
	if p == "" {
		return n
	}
	return p + "/" + n
}

// Put stores body under the name and returns the full key.
func (s *Store) Put(ctx context.Context, name string, body []byte) (string, error) {
	key := s.Key(name)
	_, err := s.cli.PutObject(ctx, s.Bucket, key, bytes.NewReader(body), int64(len(body)),
		minio.PutObjectOptions{ContentType: contentType(name), DisableContentSha256: true})
	if err != nil {
		return "", fmt.Errorf("artifacts: put %s: %w", key, err)
	}
	return key, nil
}

// Get reads the object back.
func (s *Store) Get(ctx context.Context, name string) ([]byte, error) {
	key := s.Key(name)
	obj, err := s.cli.GetObject(ctx, s.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("artifacts: get %s: %w", key, err)
	}
	defer obj.Close()
	body, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("artifacts: read %s: %w", key, err)
	}
	return body, nil
}

func contentType(name string) string {
	if strings.HasSuffix(name, ".json") {
		return "application/json"
	}
	return "application/octet-stream"
}
