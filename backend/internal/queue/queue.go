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
	return q.enqueue(ctx, StreamEmbed, map[string]any{
		"kind":        "embed_document",
		"document_id": docID.String(),
	})
}

// EnqueueExtractQuestions asks a worker to read an uploaded RFP and create one
// answer row per question found. It rides the slow stream because reading a
// whole document must not delay chat replies.
func (q *Queue) EnqueueExtractQuestions(ctx context.Context, documentID, projectID, workspaceID uuid.UUID) error {
	return q.enqueue(ctx, StreamEmbed, map[string]any{
		"kind":         "extract_questions",
		"document_id":  documentID.String(),
		"project_id":   projectID.String(),
		"workspace_id": workspaceID.String(),
	})
}

func (q *Queue) EnqueueGenerate(ctx context.Context, answerID, workspaceID uuid.UUID, question string) error {
	return q.enqueue(ctx, StreamGenerate, map[string]any{
		"kind":      "answer",
		"answer_id": answerID.String(), "workspace_id": workspaceID.String(), "question": question,
	})
}

// EnqueueChatReply asks a worker to generate the assistant reply for a pending
// message. The worker loads the conversation history and fills message_id.
func (q *Queue) EnqueueChatReply(ctx context.Context, messageID, conversationID uuid.UUID) error {
	return q.enqueue(ctx, StreamGenerate, map[string]any{
		"kind":            "chat_reply",
		"message_id":      messageID.String(),
		"conversation_id": conversationID.String(),
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
