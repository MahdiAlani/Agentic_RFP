package workspace

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
	ErrNotFound     = errors.New("workspace not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrConflict     = errors.New("workspace already exists")
)

// Endpoints
type Service interface {
	GetWorkspace(ctx context.Context, id uuid.UUID) (*workspace, error)
	ListWorkspaces(ctx context.Context, userID uuid.UUID) ([]*workspace, error)
	CreateWorkspace(ctx context.Context, userID uuid.UUID, name string) (*workspace, error)
	UpdateWorkspace(ctx context.Context, id uuid.UUID, name string) (*workspace, error)
	DeleteWorkspace(ctx context.Context, id uuid.UUID) error
}

// Repo service
type service struct {
	repo Repository
}

// Initializes a new repo service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetWorkspace(ctx context.Context, id uuid.UUID) (*workspace, error) {
	w, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapDBError(err)
	}
	return w, nil
}

func (s *service) ListWorkspaces(ctx context.Context, userID uuid.UUID) ([]*workspace, error) {
	ws, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, mapDBError(err)
	}
	return ws, nil
}

func (s *service) CreateWorkspace(ctx context.Context, userID uuid.UUID, name string) (*workspace, error) {
	name, err := validate(userID, name)
	if err != nil {
		return nil, err
	}
	w, err := s.repo.Create(ctx, &workspace{UserID: userID, Name: name})
	if err != nil {
		return nil, mapDBError(err)
	}
	return w, nil
}

func (s *service) UpdateWorkspace(ctx context.Context, id uuid.UUID, name string) (*workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	w, err := s.repo.Update(ctx, &workspace{ID: id, Name: name})
	if err != nil {
		return nil, mapDBError(err)
	}
	return w, nil
}

func (s *service) DeleteWorkspace(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return mapDBError(err)
	}
	return nil
}

// validate trims and checks the workspace-supplied fields, returning the cleaned
// name or an ErrInvalidInput-wrapped error describing what was wrong.
func validate(userID uuid.UUID, name string) (string, error) {
	name = strings.TrimSpace(name)
	if userID == uuid.Nil {
		return "", fmt.Errorf("%w: user_id is required", ErrInvalidInput)
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
