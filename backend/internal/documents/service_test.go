package documents

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRepo struct {
	Repository
	renamed string
}

func (f *fakeRepo) UpdateName(_ context.Context, id uuid.UUID, name string) (*document, error) {
	f.renamed = name
	return &document{ID: id, FileName: name}, nil
}

func (f *fakeRepo) GetByID(context.Context, uuid.UUID) (*document, error) {
	return nil, pgx.ErrNoRows
}

func TestCreateDocumentValidation(t *testing.T) {
	// Storage and queue are nil: validation must reject bad input before
	// touching either.
	svc := NewService(&fakeRepo{}, nil, nil)
	ws := uuid.New()
	cases := map[string]struct {
		workspace     uuid.UUID
		name, docType string
	}{
		"nil workspace": {uuid.Nil, "a.pdf", "rfp"},
		"blank name":    {ws, "  ", "rfp"},
		"blank type":    {ws, "a.pdf", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := svc.CreateDocument(context.Background(), tc.workspace, nil, tc.name, tc.docType)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestRenameDocument(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, nil, nil)

	if _, err := svc.RenameDocument(context.Background(), uuid.New(), "   "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank: err = %v", err)
	}
	if _, err := svc.RenameDocument(context.Background(), uuid.New(), " spec.pdf "); err != nil || repo.renamed != "spec.pdf" {
		t.Fatalf("renamed to %q, err %v; want trimmed name", repo.renamed, err)
	}
}

func TestMissingDocumentIsNotFound(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil, nil)
	ctx := context.Background()
	if _, err := svc.GetDocument(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetDocument err = %v", err)
	}
	// ConfirmUpload looks the row up before calling storage, so a stale ID is a
	// clean 404 rather than a storage error.
	if _, err := svc.ConfirmUpload(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ConfirmUpload err = %v", err)
	}
	if err := svc.DeleteDocument(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteDocument err = %v", err)
	}
}

func TestMapDBError(t *testing.T) {
	if err := mapDBError(&pgconn.PgError{Code: "23503"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("FK violation -> %v, want ErrInvalidInput", err)
	}
	if err := mapDBError(&pgconn.PgError{Code: "23505"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("unique violation -> %v, want ErrConflict", err)
	}
	boom := errors.New("boom")
	if err := mapDBError(boom); err != boom {
		t.Fatalf("unknown error -> %v, want passthrough", err)
	}
}
