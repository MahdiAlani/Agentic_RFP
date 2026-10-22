package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/google/uuid"

	"rfp-agent/internal/queue"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
)

type Service interface {
	CreateConversation(ctx context.Context, projectID uuid.UUID, title string) (*conversation, bool, error)
	ListConversations(ctx context.Context, projectID uuid.UUID) ([]*conversation, error)
	GetConversation(ctx context.Context, id uuid.UUID) (*conversation, error)
	RenameConversation(ctx context.Context, id uuid.UUID, title string) (*conversation, error)
	DeleteConversation(ctx context.Context, id uuid.UUID) error
	ListMessages(ctx context.Context, conversationID uuid.UUID) ([]*message, error)
	SendMessage(ctx context.Context, conversationID uuid.UUID, content string) (*message, *message, error)
}

type service struct {
	repo  Repository
	queue *queue.Queue
}

func NewService(repo Repository, q *queue.Queue) Service {
	return &service{repo: repo, queue: q}
}

func (s *service) CreateConversation(ctx context.Context, projectID uuid.UUID, title string) (*conversation, bool, error) {
	if projectID == uuid.Nil {
		return nil, false, fmt.Errorf("%w: project_id is required", ErrInvalidInput)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New chat"
	}
	c, created, err := s.repo.CreateOrReuseEmptyConversation(ctx, &conversation{ProjectID: projectID, Title: title})
	if err != nil {
		return nil, false, mapDBError(err)
	}
	return c, created, nil
}

func (s *service) ListConversations(ctx context.Context, projectID uuid.UUID) ([]*conversation, error) {
	cs, err := s.repo.ListConversations(ctx, projectID)
	if err != nil {
		return nil, mapDBError(err)
	}
	return cs, nil
}

func (s *service) GetConversation(ctx context.Context, id uuid.UUID) (*conversation, error) {
	c, err := s.repo.GetConversation(ctx, id)
	if err != nil {
		return nil, mapDBError(err)
	}
	return c, nil
}

// RenameConversation changes a conversation's title.
func (s *service) RenameConversation(ctx context.Context, id uuid.UUID, title string) (*conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	c, err := s.repo.UpdateConversationTitle(ctx, id, title)
	if err != nil {
		return nil, mapDBError(err)
	}
	return c, nil
}

// DeleteConversation removes a conversation and all of its messages.
func (s *service) DeleteConversation(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteConversation(ctx, id); err != nil {
		return mapDBError(err)
	}
	return nil
}

func (s *service) ListMessages(ctx context.Context, conversationID uuid.UUID) ([]*message, error) {
	ms, err := s.repo.ListMessages(ctx, conversationID)
	if err != nil {
		return nil, mapDBError(err)
	}
	return ms, nil
}

// SendMessage stores the user's message plus a pending assistant message, then
// queues a worker to fill in the assistant reply. Returns both messages.
func (s *service) SendMessage(ctx context.Context, conversationID uuid.UUID, content string) (*message, *message, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil, fmt.Errorf("%w: message content is required", ErrInvalidInput)
	}

	// Ensure the conversation exists (also surfaces a clean 404).
	if _, err := s.repo.GetConversation(ctx, conversationID); err != nil {
		return nil, nil, mapDBError(err)
	}

	userMsg, err := s.repo.CreateMessage(ctx, &message{
		ConversationID: conversationID,
		Role:           "user",
		Content:        content,
		Status:         "complete",
	})
	if err != nil {
		return nil, nil, mapDBError(err)
	}

	assistantMsg, err := s.repo.CreateMessage(ctx, &message{
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        "",
		Status:         "pending",
	})
	if err != nil {
		return nil, nil, mapDBError(err)
	}

	if err := s.queue.EnqueueChatReply(ctx, assistantMsg.ID, conversationID); err != nil {
		return nil, nil, err
	}

	return userMsg, assistantMsg, nil
}

func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
		return fmt.Errorf("%w: referenced project or conversation does not exist", ErrInvalidInput)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
