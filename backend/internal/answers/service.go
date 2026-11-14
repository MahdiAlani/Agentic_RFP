package answers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/google/uuid"
)

// Error helper
var (
	ErrNotFound     = errors.New("answer not found")
	ErrInvalidInput = errors.New("invalid input")
	// ErrChecksPending: the quality checks haven't run on the current text yet.
	ErrChecksPending = errors.New("quality checks are still running; try again in a moment")
	// ErrChecksFailed: a blocking quality check fails and nobody overrode it.
	ErrChecksFailed = errors.New("quality checks failed")
)

// Jobs is the work the service hands to the workers (*queue.Queue).
type Jobs interface {
	EnqueueGenerate(ctx context.Context, answerID, workspaceID uuid.UUID, question string) error
	EnqueueExtractQuestions(ctx context.Context, documentID, projectID, workspaceID uuid.UUID) error
	EnqueueCheck(ctx context.Context, answerID uuid.UUID) error
}

// Endpoints
type Service interface {
	CreateAnswer(ctx context.Context, workspaceID uuid.UUID, projectID, documentID *uuid.UUID, question string) (*answer, error)
	GetAnswer(ctx context.Context, id uuid.UUID) (*answer, error)
	ListAnswers(ctx context.Context, workspaceID uuid.UUID) ([]*answer, error)
	ListProjectAnswers(ctx context.Context, projectID uuid.UUID) ([]*answer, error)
	ExportProject(ctx context.Context, projectID uuid.UUID) (string, []*answer, error)
	UpdateAnswerText(ctx context.Context, id uuid.UUID, text string) (*answer, error)
	RegenerateAnswer(ctx context.Context, id uuid.UUID) (*answer, error)
	ApproveAnswer(ctx context.Context, id uuid.UUID, override bool) (*answer, error)
	FillProject(ctx context.Context, projectID uuid.UUID) (int, error)
	ExtractQuestions(ctx context.Context, workspaceID, projectID, documentID uuid.UUID) error
	DeleteAnswer(ctx context.Context, id uuid.UUID) error
}

// Repo service
type service struct {
	repo  Repository
	queue Jobs
}

// Initializes a new repo service
func NewService(repo Repository, q Jobs) Service {
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

// ExportProject returns the project's name and its answers (with citations),
// everything needed to lay out the response document.
func (s *service) ExportProject(ctx context.Context, projectID uuid.UUID) (string, []*answer, error) {
	name, err := s.repo.ProjectName(ctx, projectID)
	if err != nil {
		return "", nil, mapDBError(err)
	}
	as, err := s.ListProjectAnswers(ctx, projectID)
	if err != nil {
		return "", nil, err
	}
	return name, as, nil
}

// UpdateAnswerText saves a user's edit, which also marks the answer done, and
// has the quality checks re-run on the new text.
func (s *service) UpdateAnswerText(ctx context.Context, id uuid.UUID, text string) (*answer, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("%w: answer_text is required", ErrInvalidInput)
	}
	a, err := s.repo.UpdateText(ctx, id, text)
	if err != nil {
		return nil, mapDBError(err)
	}
	if err := s.queue.EnqueueCheck(ctx, id); err != nil {
		return nil, err
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

// ApproveAnswer signs off a drafted answer. Only a finished draft can be
// approved; editing the text afterwards returns it to 'done' for re-review.
//
// The quality checks must have run on the current text, and every blocking
// check must pass. A person may override failing blockers (an answer they
// wrote themselves has no sources, say); the MCP server never does.
func (s *service) ApproveAnswer(ctx context.Context, id uuid.UUID, override bool) (*answer, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapDBError(err)
	}
	if a.Status != "done" || a.AnswerText == nil || strings.TrimSpace(*a.AnswerText) == "" {
		return nil, fmt.Errorf("%w: only a drafted answer can be approved (status is %q)", ErrInvalidInput, a.Status)
	}

	if !a.ChecksCurrent {
		// Answers drafted before checks existed have none: queue them now.
		if err := s.queue.EnqueueCheck(ctx, id); err != nil {
			return nil, err
		}
		return nil, ErrChecksPending
	}
	if failed := failingBlockers(a.Checks); len(failed) > 0 && !override {
		return nil, fmt.Errorf("%w: %s", ErrChecksFailed, strings.Join(failed, "; "))
	}

	updated, err := s.repo.UpdateStatus(ctx, id, "approved")
	if err != nil {
		return nil, mapDBError(err)
	}
	return updated, nil
}

// failingBlockers describes each blocking check that failed, as "name: detail".
// Unreadable results count as a failure rather than a pass.
func failingBlockers(raw json.RawMessage) []string {
	var checks []check
	if err := json.Unmarshal(raw, &checks); err != nil {
		return []string{"checks: unreadable results"}
	}
	var failed []string
	for _, c := range checks {
		if c.Severity == "blocker" && !c.Passed {
			failed = append(failed, strings.TrimSuffix(c.Name+": "+c.Detail, ": "))
		}
	}
	return failed
}

// FillProject queues every unanswered question in a project and reports how
// many jobs it enqueued. Answers already done or approved are left alone.
func (s *service) FillProject(ctx context.Context, projectID uuid.UUID) (int, error) {
	as, err := s.repo.GetByProjectID(ctx, projectID)
	if err != nil {
		return 0, mapDBError(err)
	}

	queued := 0
	for _, a := range as {
		if a.Status == "done" || a.Status == "pending" || a.Status == "approved" {
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
