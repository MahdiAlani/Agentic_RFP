package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client *minio.Client
	bucket string
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

	useSSL := os.Getenv("MINIO_USE_SSL") == "1"

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	// Client connection failed
	if err != nil {
		return nil, err
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

	return &Storage{client: client, bucket: bucket}, nil
}

// PresignedPut returns a temporary URL the client can PUT a file to.
func (s *Storage) PresignedPut(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedPutObject(ctx, s.bucket, key, expiry)
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
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, reqParams)
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

func getenv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return v, nil
}
