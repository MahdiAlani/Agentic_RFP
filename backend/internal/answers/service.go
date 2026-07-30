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
	ListProjectAnswers(ctx context.Context, projectID uuid.UUID) ([]*answer, error)
	UpdateAnswerText(ctx context.Context, id uuid.UUID, text string) (*answer, error)
	RegenerateAnswer(ctx context.Context, id uuid.UUID) (*answer, error)
	FillProject(ctx context.Context, projectID uuid.UUID) (int, error)
	ExtractQuestions(ctx context.Context, workspaceID, projectID, documentID uuid.UUID) error
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
	cites, err := s.repo.CitationsByAnswer(ctx, id)
	if err != nil {
		return nil, mapDBError(err)
	}
	a.Citations = cites
	return a, nil
}

func (s *service) ListAnswers(ctx context.Context, workspaceID uuid.UUID) ([]*answer, error) {
	as, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, mapDBError(err)
	}
	return as, nil
}

// ListProjectAnswers returns an RFP's questions in order, each with its citations.
func (s *service) ListProjectAnswers(ctx context.Context, projectID uuid.UUID) ([]*answer, error) {
	as, err := s.repo.GetByProjectID(ctx, projectID)
	if err != nil {
		return nil, mapDBError(err)
	}
	cites, err := s.repo.CitationsByProject(ctx, projectID)
	if err != nil {
		return nil, mapDBError(err)
	}
	for _, a := range as {
		a.Citations = cites[a.ID]
	}
	return as, nil
}

// UpdateAnswerText saves a user's edit, which also marks the answer done.
func (s *service) UpdateAnswerText(ctx context.Context, id uuid.UUID, text string) (*answer, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("%w: answer_text is required", ErrInvalidInput)
	}
	a, err := s.repo.UpdateText(ctx, id, text)
	if err != nil {
		return nil, mapDBError(err)
	}
	return a, nil
}

// RegenerateAnswer re-queues a single question for the model.
func (s *service) RegenerateAnswer(ctx context.Context, id uuid.UUID) (*answer, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapDBError(err)
	}

	updated, err := s.repo.UpdateStatus(ctx, id, "pending")
	if err != nil {
		return nil, mapDBError(err)
	}

	if err := s.queue.EnqueueGenerate(ctx, a.ID, a.WorkspaceID, a.Question); err != nil {
		return nil, err
	}
	return updated, nil
}

// FillProject queues every unanswered question in a project and reports how
// many jobs it enqueued. Answers already done are left alone.
func (s *service) FillProject(ctx context.Context, projectID uuid.UUID) (int, error) {
	as, err := s.repo.GetByProjectID(ctx, projectID)
	if err != nil {
		return 0, mapDBError(err)
	}

	queued := 0
	for _, a := range as {
		if a.Status == "done" || a.Status == "pending" {
			continue
		}
		if _, err := s.repo.UpdateStatus(ctx, a.ID, "pending"); err != nil {
			return queued, mapDBError(err)
		}
		if err := s.queue.EnqueueGenerate(ctx, a.ID, a.WorkspaceID, a.Question); err != nil {
			return queued, err
		}
		queued++
	}
	return queued, nil
}

// ExtractQuestions asks a worker to read an RFP document and create one answer
// row per question it finds.
func (s *service) ExtractQuestions(ctx context.Context, workspaceID, projectID, documentID uuid.UUID) error {
	if workspaceID == uuid.Nil || projectID == uuid.Nil || documentID == uuid.Nil {
		return fmt.Errorf("%w: workspace_id, project_id and document_id are required", ErrInvalidInput)
	}
	return s.queue.EnqueueExtractQuestions(ctx, documentID, projectID, workspaceID)
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
