package user

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Error helper
var (
	ErrNotFound     = errors.New("user not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrConflict     = errors.New("email already exists")
)

// Endpoints
type Service interface {
	GetUser(ctx context.Context, id uuid.UUID) (*User, error)
	CreateUser(ctx context.Context, email, name, password string) (*User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, email, name string) (*User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
}

// Repo service
type service struct {
	repo Repository
}

// Initializes a new repo service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, mapDBError(err)
	}
	return u, nil
}

func (s *service) CreateUser(ctx context.Context, email, name, password string) (*User, error) {
	email, name, err := validate(email, name)
	if err != nil {
		return nil, err
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	u, err := s.repo.Create(ctx, &User{Email: email, Name: name, PasswordHash: string(hash)})
	if err != nil {
		return nil, mapDBError(err)
	}
	return u, nil
}

func (s *service) UpdateUser(ctx context.Context, id uuid.UUID, email, name string) (*User, error) {
	email, name, err := validate(email, name)
	if err != nil {
		return nil, err
	}
	u, err := s.repo.Update(ctx, &User{ID: id, Email: email, Name: name})
	if err != nil {
		return nil, mapDBError(err)
	}
	return u, nil
}

func (s *service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return mapDBError(err)
	}
	return nil
}

// validate trims and checks the user-supplied fields, returning the cleaned
// values or an ErrInvalidInput-wrapped error describing what was wrong.
func validate(email, name string) (string, string, error) {
	email = strings.TrimSpace(email)
	name = strings.TrimSpace(name)
	if email == "" || name == "" {
		return "", "", fmt.Errorf("%w: email and name are required", ErrInvalidInput)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "", "", fmt.Errorf("%w: malformed email address", ErrInvalidInput)
	}
	return email, name, nil
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
