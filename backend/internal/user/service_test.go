package user

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// memRepo is an in-memory Repository keyed by email.
type memRepo struct {
	byEmail   map[string]*User
	createErr error
}

func newMemRepo() *memRepo { return &memRepo{byEmail: map[string]*User{}} }

func (m *memRepo) GetByID(_ context.Context, id uuid.UUID) (*User, error) {
	for _, u := range m.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (m *memRepo) GetByEmail(_ context.Context, email string) (*User, error) {
	if u, ok := m.byEmail[email]; ok {
		return u, nil
	}
	return nil, pgx.ErrNoRows
}

func (m *memRepo) Create(_ context.Context, u *User) (*User, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if _, ok := m.byEmail[u.Email]; ok {
		return nil, &pgconn.PgError{Code: "23505"}
	}
	c := *u
	c.ID = uuid.New()
	m.byEmail[c.Email] = &c
	return &c, nil
}

func (m *memRepo) Update(_ context.Context, u *User) (*User, error) { return u, nil }
func (m *memRepo) Delete(context.Context, uuid.UUID) error         { return nil }

func TestCreateUserValidation(t *testing.T) {
	cases := map[string]struct{ email, name, password string }{
		"empty email":     {"", "Ann", "password1"},
		"blank name":      {"ann@example.com", "   ", "password1"},
		"malformed email": {"not-an-email", "Ann", "password1"},
		"short password":  {"ann@example.com", "Ann", "short"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewService(newMemRepo()).CreateUser(context.Background(), tc.email, tc.name, tc.password)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestCreateUserHashesAndTrims(t *testing.T) {
	repo := newMemRepo()
	u, err := NewService(repo).CreateUser(context.Background(), "  ann@example.com ", " Ann ", "password1")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Email != "ann@example.com" || u.Name != "Ann" {
		t.Fatalf("fields not trimmed: %q %q", u.Email, u.Name)
	}
	if u.PasswordHash == "" || u.PasswordHash == "password1" {
		t.Fatalf("password stored unhashed: %q", u.PasswordHash)
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()
	if _, err := svc.CreateUser(ctx, "ann@example.com", "Ann", "password1"); err != nil {
		t.Fatal(err)
	}
	_, err := svc.CreateUser(ctx, "ann@example.com", "Ann 2", "password2")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestAuthenticate(t *testing.T) {
	svc := NewService(newMemRepo())
	ctx := context.Background()
	created, err := svc.CreateUser(ctx, "ann@example.com", "Ann", "password1")
	if err != nil {
		t.Fatal(err)
	}

	u, err := svc.Authenticate(ctx, " ann@example.com ", "password1")
	if err != nil || u.ID != created.ID {
		t.Fatalf("Authenticate(valid) = %v, %v", u, err)
	}

	// Wrong password and unknown email must be indistinguishable to the caller.
	for _, tc := range []struct{ email, pw string }{
		{"ann@example.com", "wrong-password"},
		{"nobody@example.com", "password1"},
	} {
		if _, err := svc.Authenticate(ctx, tc.email, tc.pw); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("Authenticate(%q) err = %v, want ErrInvalidCredentials", tc.email, err)
		}
	}
}

func TestGetUserNotFound(t *testing.T) {
	_, err := NewService(newMemRepo()).GetUser(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUnknownDBErrorPassesThrough(t *testing.T) {
	boom := errors.New("connection reset")
	repo := newMemRepo()
	repo.createErr = boom
	_, err := NewService(repo).CreateUser(context.Background(), "ann@example.com", "Ann", "password1")
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the original error", err)
	}
}
