package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// fakeRepo records calls; unimplemented methods panic via the nil interface.
type fakeRepo struct {
	Repository
	calls     []string
	deleteErr error
}

func (f *fakeRepo) Create(_ context.Context, w *workspace) (*workspace, error) {
	f.calls = append(f.calls, "create")
	c := *w
	c.ID = uuid.New()
	return &c, nil
}

func (f *fakeRepo) GetByID(context.Context, uuid.UUID) (*workspace, error) {
	return nil, pgx.ErrNoRows
}

func (f *fakeRepo) Delete(context.Context, uuid.UUID) error {
	f.calls = append(f.calls, "repo.delete")
	return f.deleteErr
}

type fakeCleaner struct {
	repo *fakeRepo
	err  error
}

func (c *fakeCleaner) RemoveWorkspaceObjects(context.Context, uuid.UUID) error {
	c.repo.calls = append(c.repo.calls, "remove objects")
	return c.err
}

func TestCreateWorkspaceValidation(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil)
	ctx := context.Background()

	if _, err := svc.CreateWorkspace(ctx, uuid.Nil, "Acme"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil user: err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.CreateWorkspace(ctx, uuid.New(), "   "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank name: err = %v, want ErrInvalidInput", err)
	}

	w, err := svc.CreateWorkspace(ctx, uuid.New(), "  Acme  ")
	if err != nil || w.Name != "Acme" {
		t.Fatalf("CreateWorkspace = %+v, %v; want trimmed name", w, err)
	}
}

func TestUpdateWorkspaceRequiresName(t *testing.T) {
	_, err := NewService(&fakeRepo{}, nil).UpdateWorkspace(context.Background(), uuid.New(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestGetWorkspaceNotFound(t *testing.T) {
	_, err := NewService(&fakeRepo{}, nil).GetWorkspace(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// Stored files must be removed before the row: once Postgres cascades the
// document rows away, their file keys can no longer be looked up.
func TestDeleteWorkspaceRemovesObjectsFirst(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, &fakeCleaner{repo: repo})
	if err := svc.DeleteWorkspace(context.Background(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(repo.calls) != 2 || repo.calls[0] != "remove objects" || repo.calls[1] != "repo.delete" {
		t.Fatalf("call order = %v", repo.calls)
	}
}

func TestDeleteWorkspaceKeepsRowWhenCleanupFails(t *testing.T) {
	repo := &fakeRepo{}
	boom := errors.New("storage down")
	err := NewService(repo, &fakeCleaner{repo: repo, err: boom}).DeleteWorkspace(context.Background(), uuid.New())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	for _, c := range repo.calls {
		if c == "repo.delete" {
			t.Fatal("row deleted even though object cleanup failed")
		}
	}
}
