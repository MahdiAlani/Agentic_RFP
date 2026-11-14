package answers

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
	answers   []*answer
	cites     map[uuid.UUID][]citation
	textSet   string
	byID      *answer
	statusSet string
}

func (f *fakeRepo) UpdateStatus(_ context.Context, id uuid.UUID, status string) (*answer, error) {
	f.statusSet = status
	return &answer{ID: id, Status: status}, nil
}

func (f *fakeRepo) GetByProjectID(context.Context, uuid.UUID) ([]*answer, error) {
	return f.answers, nil
}

func (f *fakeRepo) CitationsByProject(context.Context, uuid.UUID) (map[uuid.UUID][]citation, error) {
	return f.cites, nil
}

func (f *fakeRepo) GetByID(context.Context, uuid.UUID) (*answer, error) {
	if f.byID == nil {
		return nil, pgx.ErrNoRows
	}
	return f.byID, nil
}

func (f *fakeRepo) UpdateText(_ context.Context, id uuid.UUID, text string) (*answer, error) {
	f.textSet = text
	return &answer{ID: id, AnswerText: &text, Status: "done"}, nil
}

func TestCreateAnswerValidation(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil)
	ctx := context.Background()
	if _, err := svc.CreateAnswer(ctx, uuid.Nil, nil, nil, "Q?"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil workspace: err = %v", err)
	}
	if _, err := svc.CreateAnswer(ctx, uuid.New(), nil, nil, "  "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank question: err = %v", err)
	}
}

func TestExtractQuestionsRequiresAllIDs(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil)
	id := uuid.New()
	for _, ids := range [][3]uuid.UUID{
		{uuid.Nil, id, id},
		{id, uuid.Nil, id},
		{id, id, uuid.Nil},
	} {
		if err := svc.ExtractQuestions(context.Background(), ids[0], ids[1], ids[2]); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("ids %v: err = %v", ids, err)
		}
	}
}

// fakeJobs records the quality-check jobs a service queues.
type fakeJobs struct {
	Jobs
	checked []uuid.UUID
}

func (f *fakeJobs) EnqueueCheck(_ context.Context, id uuid.UUID) error {
	f.checked = append(f.checked, id)
	return nil
}

func TestUpdateAnswerText(t *testing.T) {
	repo, jobs := &fakeRepo{}, &fakeJobs{}
	svc := NewService(repo, jobs)
	if _, err := svc.UpdateAnswerText(context.Background(), uuid.New(), "\n "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank: err = %v", err)
	}
	id := uuid.New()
	if _, err := svc.UpdateAnswerText(context.Background(), id, " Edited. "); err != nil || repo.textSet != "Edited." {
		t.Fatalf("saved %q, err %v", repo.textSet, err)
	}
	if len(jobs.checked) != 1 || jobs.checked[0] != id {
		t.Fatalf("checks queued for %v, want the edited answer", jobs.checked)
	}
}

// FillProject must skip answers that are finished, approved or already queued,
// so clicking "fill" twice never double-bills the model. With nothing to queue
// it returns before touching Redis (the queue here is nil).
func TestFillProjectSkipsFinishedAndPending(t *testing.T) {
	repo := &fakeRepo{answers: []*answer{
		{ID: uuid.New(), Status: "done"},
		{ID: uuid.New(), Status: "pending"},
		{ID: uuid.New(), Status: "approved"},
	}}
	n, err := NewService(repo, nil).FillProject(context.Background(), uuid.New())
	if err != nil || n != 0 {
		t.Fatalf("FillProject = %d, %v; want 0 queued", n, err)
	}
}

func TestListProjectAnswersAttachesCitations(t *testing.T) {
	a1, a2 := uuid.New(), uuid.New()
	repo := &fakeRepo{
		answers: []*answer{{ID: a1}, {ID: a2}},
		cites:   map[uuid.UUID][]citation{a1: {{FileName: "security.pdf", ChunkRef: "3"}}},
	}
	got, err := NewService(repo, nil).ListProjectAnswers(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].Citations) != 1 || got[0].Citations[0].FileName != "security.pdf" {
		t.Fatalf("answer 1 citations = %+v", got[0].Citations)
	}
	if len(got[1].Citations) != 0 {
		t.Fatalf("answer 2 citations = %+v, want none", got[1].Citations)
	}
}

func TestApproveAnswer(t *testing.T) {
	text, blank := "Drafted.", "  "
	passing := []byte(`[{"name":"has_citations","passed":true,"severity":"blocker"},
		{"name":"length_reasonable","passed":false,"severity":"warning","detail":"3 words"}]`)
	failing := []byte(`[{"name":"has_citations","passed":false,"severity":"blocker","detail":"No sources."}]`)
	done := func(checks []byte, current bool) *answer {
		return &answer{Status: "done", AnswerText: &text, Checks: checks, ChecksCurrent: current}
	}
	cases := map[string]struct {
		existing *answer
		override bool
		wantErr  error
	}{
		"checks pass":              {done(passing, true), false, nil},
		"blocker fails":            {done(failing, true), false, ErrChecksFailed},
		"blocker overridden":       {done(failing, true), true, nil},
		"checks stale":             {done(passing, false), false, ErrChecksPending},
		"never checked":            {done(nil, false), true, ErrChecksPending},
		"missing":                  {nil, false, ErrNotFound},
		"still generating":         {&answer{Status: "pending", AnswerText: &text}, false, ErrInvalidInput},
		"never drafted":            {&answer{Status: "new"}, false, ErrInvalidInput},
		"blank text":               {&answer{Status: "done", AnswerText: &blank}, false, ErrInvalidInput},
		"already approved":         {&answer{Status: "approved", AnswerText: &text}, false, ErrInvalidInput},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo, jobs := &fakeRepo{byID: tc.existing}, &fakeJobs{}
			_, err := NewService(repo, jobs).ApproveAnswer(context.Background(), uuid.New(), tc.override)
			if errors.Is(tc.wantErr, ErrChecksPending) && len(jobs.checked) != 1 {
				t.Fatalf("stale checks were not re-queued")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil && repo.statusSet != "approved" {
				t.Fatalf("status set to %q, want approved", repo.statusSet)
			}
			if tc.wantErr != nil && repo.statusSet != "" {
				t.Fatalf("status changed to %q on a rejected approval", repo.statusSet)
			}
		})
	}
}

func TestRegenerateMissingAnswerIsNotFound(t *testing.T) {
	if _, err := NewService(&fakeRepo{}, nil).RegenerateAnswer(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMapDBErrorForeignKey(t *testing.T) {
	if err := mapDBError(&pgconn.PgError{Code: "23503"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}
