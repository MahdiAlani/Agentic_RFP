package answers

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

// Error helper
var (
	ErrNotFound     = errors.New("answer not found")
	ErrInvalidInput = errors.New("invalid input")
)

// Endpoints
type Service interface {
	CreateAnswer(ctx context.Context, workspaceID uuid.UUID, projectID, documentID *uuid.UUID, question string) (*answer, error)
	GetAnswer(ctx context.Context, id uuid.UUID) (*answer, error)
	ListAnswers(ctx context.Context, workspaceID uuid.UUID) ([]*answer, error)
	DeleteAnswer(ctx context.Context, id uuid.UUID) error
}

// Repo service
type service struct {
	repo  Repository
	queue *queue.Queue
}

// Initializes a new repo service
func NewService(repo Repository, q *queue.Queue) Service {
	return &service{repo: repo, queue: q}
}

// CreateAnswer inserts a pending row and queues the generation job.
func (s *service) CreateAnswer(ctx context.Context, workspaceID uuid.UUID, projectID, documentID *uuid.UUID, question string) (*answer, error) {
	question = strings.TrimSpace(question)
	if workspaceID == uuid.Nil {
		return nil, fmt.Errorf("%w: workspace_id is required", ErrInvalidInput)
	}
	if question == "" {
		return nil, fmt.Errorf("%w: question is required", ErrInvalidInput)
	}

	a, err := s.repo.Create(ctx, &answer{
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
		DocumentID:  documentID,
		Question:    question,
		Status:      "pending",
	})
	if err != nil {
		return nil, mapDBError(err)
	}

	if err := s.queue.EnqueueGenerate(ctx, a.ID, workspaceID, question); err != nil {
		return nil, err
	}

	return a, nil
}

func (s *service) GetAnswer(ctx context.Context, id uuid.UUID) (*answer, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapDBError(err)
	}
	return a, nil
}

func (s *service) ListAnswers(ctx context.Context, workspaceID uuid.UUID) ([]*answer, error) {
	as, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, mapDBError(err)
	}
	return as, nil
}

func (s *service) DeleteAnswer(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return mapDBError(err)
	}
	return nil
}

// mapDBError translates storage-layer errors into the package's sentinel
// errors so callers (and the HTTP layer) stay decoupled from pgx.
func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
		return fmt.Errorf("%w: referenced workspace, project, or document does not exist", ErrInvalidInput)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
