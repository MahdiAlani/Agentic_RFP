package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

// TopicDocumentUploaded is keyed by document ID, so every event for one
// document lands on the same partition and is read in order.
const TopicDocumentUploaded = "documents.uploaded"

type Publisher struct {
	client *kgo.Client
}

type DocumentUploaded struct {
	DocumentID  uuid.UUID `json:"document_id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
}

// New reads KAFKA_BROKERS (comma-separated host:port) and connects.
func New(ctx context.Context) (*Publisher, error) {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		return nil, fmt.Errorf("required environment variable KAFKA_BROKERS is not set")
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(strings.Split(brokers, ",")...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		// The workers create the topic with its partition count; this covers an
		// upload confirmed before any worker has started.
		kgo.AllowAutoTopicCreation(),
		// Fail the upload request instead of hanging when no broker answers.
		kgo.RecordDeliveryTimeout(5*time.Second),
	)
	if err != nil {
		return nil, err
	}

	p := &Publisher{client: client}
	if err := p.Ping(ctx); err != nil {
		client.Close()
		return nil, err
	}
	return p, nil
}

// DocumentUploaded waits until the broker has stored the event, so a nil error
// means the upload will be indexed.
func (p *Publisher) DocumentUploaded(ctx context.Context, docID, workspaceID uuid.UUID) error {
	value, err := json.Marshal(DocumentUploaded{DocumentID: docID, WorkspaceID: workspaceID})
	if err != nil {
		return err
	}
	return p.client.ProduceSync(ctx, &kgo.Record{
		Topic: TopicDocumentUploaded,
		Key:   []byte(docID.String()),
		Value: value,
	}).FirstErr()
}

// Ping checks a broker is reachable (used by the readiness probe).
func (p *Publisher) Ping(ctx context.Context) error {
	return p.client.Ping(ctx)
}

func (p *Publisher) Close() {
	p.client.Close()
}
