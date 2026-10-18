package project

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
	calls     []string
	createErr error
}

func (f *fakeRepo) Create(_ context.Context, p *project) (*project, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	c := *p
	c.ID = uuid.New()
	return &c, nil
}

func (f *fakeRepo) GetByID(context.Context, uuid.UUID) (*project, error) {
	return nil, pgx.ErrNoRows
}

func (f *fakeRepo) Delete(context.Context, uuid.UUID) error {
	f.calls = append(f.calls, "repo.delete")
	return nil
}

type fakeCleaner struct {
	repo *fakeRepo
	err  error
}

func (c *fakeCleaner) RemoveProjectObjects(context.Context, uuid.UUID) error {
	c.repo.calls = append(c.repo.calls, "remove objects")
	return c.err
}

func TestCreateProject(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil)
	ctx := context.Background()

	if _, err := svc.CreateProject(ctx, uuid.Nil, "RFP"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil workspace: err = %v", err)
	}
	if _, err := svc.CreateProject(ctx, uuid.New(), " "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank name: err = %v", err)
	}
	p, err := svc.CreateProject(ctx, uuid.New(), " Q3 RFP ")
	if err != nil || p.Name != "Q3 RFP" {
		t.Fatalf("CreateProject = %+v, %v", p, err)
	}
}

func TestCreateProjectDuplicateIsConflict(t *testing.T) {
	svc := NewService(&fakeRepo{createErr: &pgconn.PgError{Code: "23505"}}, nil)
	if _, err := svc.CreateProject(context.Background(), uuid.New(), "RFP"); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestCreateProjectInMissingWorkspaceIsInvalidInput(t *testing.T) {
	svc := NewService(&fakeRepo{createErr: &pgconn.PgError{Code: "23503"}}, nil)
	if _, err := svc.CreateProject(context.Background(), uuid.New(), "RFP"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput (400, not 500)", err)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	if _, err := NewService(&fakeRepo{}, nil).GetProject(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteProjectOrder(t *testing.T) {
	repo := &fakeRepo{}
	if err := NewService(repo, &fakeCleaner{repo: repo}).DeleteProject(context.Background(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(repo.calls) != 2 || repo.calls[0] != "remove objects" {
		t.Fatalf("call order = %v, want objects removed before the row", repo.calls)
	}

	repo = &fakeRepo{}
	err := NewService(repo, &fakeCleaner{repo: repo, err: errors.New("down")}).DeleteProject(context.Background(), uuid.New())
	if err == nil || len(repo.calls) != 1 {
		t.Fatalf("cleanup failure: err = %v, calls = %v; want row kept", err, repo.calls)
	}
}
