package queue

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

// Constants for queue keys
const (
	StreamEmbed    = "embed"
	StreamGenerate = "generate"
)

type Queue struct {
	client *redis.Client
}

// New reads redis address from env, connects to redis.
func New(ctx context.Context) (*Queue, error) {

	addr, err := getenv("REDIS_ADDR")
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	q := Queue{
		client: client,
	}

	//Since NewClient does not return error, ping redis to confirm
	err = client.Ping(ctx).Err()

	if err != nil {
		return nil, err
	}

	return &q, nil
}

func getenv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return v, nil
}
