package project

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/google/uuid"
)

// Error helper
var (
	ErrNotFound     = errors.New("project not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrConflict     = errors.New("project already exists")
)

// Endpoints
type Service interface {
	GetProject(ctx context.Context, id uuid.UUID) (*project, error)
	ListProjects(ctx context.Context, workspaceID uuid.UUID) ([]*project, error)
	CreateProject(ctx context.Context, workspaceID uuid.UUID, name string) (*project, error)
	UpdateProject(ctx context.Context, id uuid.UUID, name string) (*project, error)
	DeleteProject(ctx context.Context, id uuid.UUID) error
}

// Repo service
type service struct {
	repo Repository
}

// Initializes a new repo service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetProject(ctx context.Context, id uuid.UUID) (*project, error) {
	w, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapDBError(err)
	}
	return w, nil
}

func (s *service) ListProjects(ctx context.Context, workspaceID uuid.UUID) ([]*project, error) {
	ws, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, mapDBError(err)
	}
	return ws, nil
}

func (s *service) CreateProject(ctx context.Context, workspaceID uuid.UUID, name string) (*project, error) {
	name, err := validate(workspaceID, name)
	if err != nil {
		return nil, err
	}
	w, err := s.repo.Create(ctx, &project{WorkspaceID: workspaceID, Name: name})
	if err != nil {
		return nil, mapDBError(err)
	}
	return w, nil
}

func (s *service) UpdateProject(ctx context.Context, id uuid.UUID, name string) (*project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	w, err := s.repo.Update(ctx, &project{ID: id, Name: name})
	if err != nil {
		return nil, mapDBError(err)
	}
	return w, nil
}

func (s *service) DeleteProject(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return mapDBError(err)
	}
	return nil
}

// validate trims and checks the project-supplied fields, returning the cleaned
// name or an ErrInvalidInput-wrapped error describing what was wrong.
func validate(workspaceID uuid.UUID, name string) (string, error) {
	name = strings.TrimSpace(name)
	if workspaceID == uuid.Nil {
		return "", fmt.Errorf("%w: workspace_id is required", ErrInvalidInput)
	}
	if name == "" {
		return "", fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	return name, nil
}

// mapDBError translates storage-layer errors into the package's sentinel
// errors so callers (and the HTTP layer) stay decoupled from pgx.
func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return ErrConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
