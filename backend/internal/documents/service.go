package documents

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/google/uuid"

	"rfp-agent/internal/queue"
	"rfp-agent/internal/storage"
)

const presignExpiry = 15 * time.Minute

// Error helper
var (
	ErrNotFound     = errors.New("document not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrConflict     = errors.New("document already exists")
	ErrNotUploaded  = errors.New("object not uploaded")
)

// Endpoints
type Service interface {
	CreateDocument(ctx context.Context, workspaceID uuid.UUID, projectID *uuid.UUID, fileName, documentType string) (*document, string, error)
	ConfirmUpload(ctx context.Context, id uuid.UUID) (*document, error)
	GetDocument(ctx context.Context, id uuid.UUID) (*document, error)
	ListDocuments(ctx context.Context, workspaceID uuid.UUID) ([]*document, error)
	DownloadURL(ctx context.Context, id uuid.UUID) (string, error)
	DeleteDocument(ctx context.Context, id uuid.UUID) error
}

// Repo service
type service struct {
	repo    Repository
	storage *storage.Storage
	queue   *queue.Queue
}

// Initializes a new repo service
func NewService(repo Repository, st *storage.Storage, q *queue.Queue) Service {
	return &service{repo: repo, storage: st, queue: q}
}

// CreateDocument inserts a pending row and returns it plus a presigned upload URL.
func (s *service) CreateDocument(ctx context.Context, workspaceID uuid.UUID, projectID *uuid.UUID, fileName, documentType string) (*document, string, error) {
	fileName, documentType, err := validate(workspaceID, fileName, documentType)
	if err != nil {
		return nil, "", err
	}

	id := uuid.New()
	key := fmt.Sprintf("documents/%s/%s/%s", workspaceID, id, filepath.Base(fileName))

	doc, err := s.repo.Create(ctx, &document{
		ID:           id,
		WorkspaceID:  workspaceID,
		ProjectID:    projectID,
		FileName:     fileName,
		FileKey:      key,
		DocumentType: documentType,
		Status:       "pending",
	})
	if err != nil {
		return nil, "", mapDBError(err)
	}

	uploadURL, err := s.storage.PresignedPut(ctx, key, presignExpiry)
	if err != nil {
		_ = s.repo.Delete(ctx, id)
		return nil, "", err
	}

	return doc, uploadURL, nil
}

// ConfirmUpload verifies the object landed in storage, marks it uploaded, and queues embedding.
func (s *service) ConfirmUpload(ctx context.Context, id uuid.UUID) (*document, error) {
	doc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapDBError(err)
	}

	exists, err := s.storage.StatObject(ctx, doc.FileKey)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotUploaded
	}

	updated, err := s.repo.UpdateStatus(ctx, id, "uploaded")
	if err != nil {
		return nil, mapDBError(err)
	}

	if err := s.queue.EnqueueEmbed(ctx, id); err != nil {
		return nil, err
	}

	return updated, nil
}

func (s *service) GetDocument(ctx context.Context, id uuid.UUID) (*document, error) {
	doc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapDBError(err)
	}
	return doc, nil
}

func (s *service) ListDocuments(ctx context.Context, workspaceID uuid.UUID) ([]*document, error) {
	docs, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, mapDBError(err)
	}
	return docs, nil
}

// DownloadURL returns a presigned GET URL for the document's file.
func (s *service) DownloadURL(ctx context.Context, id uuid.UUID) (string, error) {
	doc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", mapDBError(err)
	}
	return s.storage.PresignedGet(ctx, doc.FileKey, doc.FileName, presignExpiry)
}

// DeleteDocument removes the object from storage (best-effort) and deletes the row.
func (s *service) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	doc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return mapDBError(err)
	}

	_ = s.storage.Remove(ctx, doc.FileKey)

	if err := s.repo.Delete(ctx, id); err != nil {
		return mapDBError(err)
	}
	return nil
}

// validate trims and checks the document-supplied fields, returning the cleaned
// values or an ErrInvalidInput-wrapped error describing what was wrong.
func validate(workspaceID uuid.UUID, fileName, documentType string) (string, string, error) {
	fileName = strings.TrimSpace(fileName)
	documentType = strings.TrimSpace(documentType)
	if workspaceID == uuid.Nil {
		return "", "", fmt.Errorf("%w: workspace_id is required", ErrInvalidInput)
	}
	if fileName == "" {
		return "", "", fmt.Errorf("%w: file_name is required", ErrInvalidInput)
	}
	if documentType == "" {
		return "", "", fmt.Errorf("%w: document_type is required", ErrInvalidInput)
	}
	return fileName, documentType, nil
}

// mapDBError translates storage-layer errors into the package's sentinel
// errors so callers (and the HTTP layer) stay decoupled from pgx.
func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return ErrConflict
		case "23503": // foreign_key_violation
			return fmt.Errorf("%w: referenced workspace or project does not exist", ErrInvalidInput)
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
