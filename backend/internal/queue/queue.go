package queue

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
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

// Enqueue a file to embed
func (q *Queue) EnqueueEmbed(ctx context.Context, docID uuid.UUID) error {
	return q.enqueue(ctx, StreamEmbed, map[string]any{"document_id": docID.String()})
}

func (q *Queue) EnqueueGenerate(ctx context.Context, answerID, workspaceID uuid.UUID, question string) error {
	return q.enqueue(ctx, StreamGenerate, map[string]any{
		"answer_id": answerID.String(), "workspace_id": workspaceID.String(), "question": question,
	})
}

func (q *Queue) enqueue(ctx context.Context, stream string, values map[string]any) error {
	_, err := q.client.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: values}).Result()
	return err
}

func getenv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return v, nil
}
