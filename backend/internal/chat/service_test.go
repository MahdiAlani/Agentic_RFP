package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeRepo struct {
	Repository
	created  *conversation
	messages int
}

func (f *fakeRepo) CreateConversation(_ context.Context, c *conversation) (*conversation, error) {
	f.created = c
	return c, nil
}

func (f *fakeRepo) GetConversation(context.Context, uuid.UUID) (*conversation, error) {
	return nil, pgx.ErrNoRows
}

func (f *fakeRepo) CreateMessage(_ context.Context, m *message) (*message, error) {
	f.messages++
	return m, nil
}

func TestCreateConversation(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, nil)
	ctx := context.Background()

	if _, err := svc.CreateConversation(ctx, uuid.Nil, "x"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil project: err = %v", err)
	}
	if _, err := svc.CreateConversation(ctx, uuid.New(), "   "); err != nil || repo.created.Title != "New chat" {
		t.Fatalf("blank title -> %q, %v; want default title", repo.created.Title, err)
	}
}

func TestRenameConversationRequiresTitle(t *testing.T) {
	if _, err := NewService(&fakeRepo{}, nil).RenameConversation(context.Background(), uuid.New(), ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestSendMessageRejectsBeforeWriting(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, nil)

	if _, _, err := svc.SendMessage(context.Background(), uuid.New(), "  "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank: err = %v", err)
	}
	// A message to a conversation that doesn't exist is a 404, and nothing is
	// stored or queued for it.
	if _, _, err := svc.SendMessage(context.Background(), uuid.New(), "hello"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing conversation: err = %v", err)
	}
	if repo.messages != 0 {
		t.Fatalf("%d messages written for rejected sends", repo.messages)
	}
}
