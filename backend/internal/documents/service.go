package document

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
	ErrNotFound     = errors.New("document not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrConflict     = errors.New("document already exists")
)

// Endpoints
type Service interface {
	Getdocument(ctx context.Context, id uuid.UUID) (*document, error)
	Listdocuments(ctx context.Context, workspaceID uuid.UUID) ([]*document, error)
	Createdocument(ctx context.Context, workspaceID uuid.UUID, projectID *uuid.UUID, fileName, fileKey, documentType string) (*document, error)
	Updatedocument(ctx context.Context, id uuid.UUID, projectID *uuid.UUID, fileName, documentType, status string) (*document, error)
	Deletedocument(ctx context.Context, id uuid.UUID) error
}

// Repo service
type service struct {
	repo Repository
}

// Initializes a new repo service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Getdocument(ctx context.Context, id uuid.UUID) (*document, error) {
	w, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapDBError(err)
	}
	return w, nil
}

func (s *service) Listdocuments(ctx context.Context, workspaceID uuid.UUID) ([]*document, error) {
	ws, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, mapDBError(err)
	}
	return ws, nil
}

func (s *service) Createdocument(ctx context.Context, workspaceID uuid.UUID, projectID *uuid.UUID, fileName, fileKey, documentType string) (*document, error) {
	fileName, fileKey, documentType, err := validate(workspaceID, fileName, fileKey, documentType)
	if err != nil {
		return nil, err
	}
	w, err := s.repo.Create(ctx, &document{
		WorkspaceID:  workspaceID,
		ProjectID:    projectID,
		FileName:     fileName,
		FileKey:      fileKey,
		DocumentType: documentType,
	})
	if err != nil {
		return nil, mapDBError(err)
	}
	return w, nil
}

func (s *service) Updatedocument(ctx context.Context, id uuid.UUID, projectID *uuid.UUID, fileName, documentType, status string) (*document, error) {
	fileName = strings.TrimSpace(fileName)
	documentType = strings.TrimSpace(documentType)
	status = strings.TrimSpace(status)
	if fileName == "" {
		return nil, fmt.Errorf("%w: file_name is required", ErrInvalidInput)
	}
	if documentType == "" {
		return nil, fmt.Errorf("%w: document_type is required", ErrInvalidInput)
	}
	if status == "" {
		return nil, fmt.Errorf("%w: status is required", ErrInvalidInput)
	}
	w, err := s.repo.Update(ctx, &document{
		ID:           id,
		ProjectID:    projectID,
		FileName:     fileName,
		DocumentType: documentType,
		Status:       status,
	})
	if err != nil {
		return nil, mapDBError(err)
	}
	return w, nil
}

func (s *service) Deletedocument(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return mapDBError(err)
	}
	return nil
}

// validate trims and checks the document-supplied fields, returning the cleaned
// name or an ErrInvalidInput-wrapped error describing what was wrong.
func validate(workspaceID uuid.UUID, fileName, fileKey, documentType string) (string, string, string, error) {
	fileName = strings.TrimSpace(fileName)
	fileKey = strings.TrimSpace(fileKey)
	documentType = strings.TrimSpace(documentType)
	if workspaceID == uuid.Nil {
		return "", "", "", fmt.Errorf("%w: workspace_id is required", ErrInvalidInput)
	}
	if fileName == "" {
		return "", "", "", fmt.Errorf("%w: file_name is required", ErrInvalidInput)
	}
	if fileKey == "" {
		return "", "", "", fmt.Errorf("%w: file_key is required", ErrInvalidInput)
	}
	if documentType == "" {
		return "", "", "", fmt.Errorf("%w: document_type is required", ErrInvalidInput)
	}
	return fileName, fileKey, documentType, nil
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
