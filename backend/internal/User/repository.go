package user

import (
	"context"

	"github.com/jackc/pgx/v5"

	"rfp-agent/internal/database"

	"github.com/google/uuid"
)

// User repo
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, u *User) (*User, error)
	Update(ctx context.Context, u *User) (*User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
// db
type postgresRepo struct {
	db *database.DB
}

// Repo constructor
func NewRepository(db *database.DB) Repository {
	return &postgresRepo{db: db}
}

// Gets a User using ID
func (r *postgresRepo) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	u := &User{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, email, name, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Gets a user using Email address
func (r *postgresRepo) GetByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, email, name, created_at FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Creates a new user
func (r *postgresRepo) Create(ctx context.Context, u *User) (*User, error) {
	created := &User{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id, email, name, created_at`,
		u.Email, u.Name, u.PasswordHash,
	).Scan(&created.ID, &created.Email, &created.Name, &created.CreatedAt)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Updates a user's data
func (r *postgresRepo) Update(ctx context.Context, u *User) (*User, error) {
	updated := &User{}
	err := r.db.Pool.QueryRow(ctx,
		`UPDATE users SET email = $1, name = $2 WHERE id = $3 RETURNING id, email, name, created_at`,
		u.Email, u.Name, u.ID,
	).Scan(&updated.ID, &updated.Email, &updated.Name, &updated.CreatedAt)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Deletes a user from the db
func (r *postgresRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
