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
	existing *conversation // empty chat the fake reports as already present
	messages int
}

func (f *fakeRepo) CreateOrReuseEmptyConversation(_ context.Context, c *conversation) (*conversation, bool, error) {
	if f.existing != nil {
		return f.existing, false, nil
	}
	f.created = c
	return c, true, nil
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

	if _, _, err := svc.CreateConversation(ctx, uuid.Nil, "x"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil project: err = %v", err)
	}
	if _, created, err := svc.CreateConversation(ctx, uuid.New(), "   "); err != nil || !created || repo.created.Title != "New chat" {
		t.Fatalf("blank title -> created=%v, %v; want new chat with default title", created, err)
	}
}

func TestCreateConversationReusesEmptyChat(t *testing.T) {
	existing := &conversation{ID: uuid.New(), Title: "New chat"}
	repo := &fakeRepo{existing: existing}
	svc := NewService(repo, nil)

	c, created, err := svc.CreateConversation(context.Background(), uuid.New(), "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if created {
		t.Fatal("created = true, want false when an empty chat is reused")
	}
	if c.ID != existing.ID {
		t.Fatalf("got conversation %v, want existing %v", c.ID, existing.ID)
	}
	if repo.created != nil {
		t.Fatal("a new conversation was inserted despite an empty one existing")
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
