package storage

import (
	"context"
	"fmt"
	"os"

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

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, err
	}

	// Create the bucket
	if !exists {
		client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	}

	return &Storage{client: client, bucket: bucket}, nil
}

func getenv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return v, nil
}
