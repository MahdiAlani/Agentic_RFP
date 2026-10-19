package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client *minio.Client
	// presigner signs URLs handed to browsers. It differs from client when the
	// API reaches storage on an internal hostname (e.g. minio:9000 inside Docker)
	// that browsers can't resolve; a presigned URL is only valid for the host it
	// was signed for.
	presigner *minio.Client
	bucket    string
}

// New reads MinIO config from env, connects, and ensures the bucket exists.
func New(ctx context.Context) (*Storage, error) {

	endpoint, err := getenv("MINIO_ENDPOINT")
	if err != nil {
		return nil, err
	}

	accessKey, err := getenv("MINIO_ACCESS_KEY")
	if err != nil {
		return nil, err
	}

	secretKey, err := getenv("MINIO_SECRET_KEY")
	if err != nil {
		return nil, err
	}

	bucket, err := getenv("MINIO_BUCKET")
	if err != nil {
		return nil, err
	}

	// Accepts true/false or 1/0, matching how the Python workers read it.
	useSSL, _ := strconv.ParseBool(os.Getenv("MINIO_USE_SSL"))
	creds := credentials.NewStaticV4(accessKey, secretKey, "")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  creds,
		Secure: useSSL,
	})
	// Client connection failed
	if err != nil {
		return nil, err
	}

	presigner := client
	if public := os.Getenv("MINIO_PUBLIC_ENDPOINT"); public != "" {
		// Setting the region up front means signing never makes a network call
		// to the public host, which the API itself may not be able to reach.
		presigner, err = minio.New(public, &minio.Options{
			Creds:  creds,
			Secure: useSSL,
			Region: envOr("MINIO_REGION", "us-east-1"),
		})
		if err != nil {
			return nil, err
		}
	}

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, err
	}

	// Create the bucket
	if !exists {
		err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, err
		}
	}

	return &Storage{client: client, presigner: presigner, bucket: bucket}, nil
}

// PresignedPut returns a temporary URL the client can PUT a file to.
func (s *Storage) PresignedPut(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.presigner.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return "", err
	}

	return u.String(), nil
}

// PresignedGet returns a temporary URL to download a file (with download filename).
func (s *Storage) PresignedGet(ctx context.Context, key, downloadFilename string, expiry time.Duration) (string, error) {
	reqParams := url.Values{}

	if downloadFilename != "" {
		reqParams.Set("response-content-disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadFilename))
	}
	u, err := s.presigner.PresignedGetObject(ctx, s.bucket, key, expiry, reqParams)
	if err != nil {
		return "", err
	}

	return u.String(), nil
}

// StatObject reports whether the object exists in the bucket.
func (s *Storage) StatObject(ctx context.Context, key string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)

		// Object not found but no error in status fetch
		if errResp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil

}

// Remove deletes the object from the bucket.
func (s *Storage) Remove(ctx context.Context, key string) error {

	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// Ping checks object storage is reachable and the bucket still exists (used by
// the readiness probe).
func (s *Storage) Ping(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("bucket %q does not exist", s.bucket)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return v, nil
}
