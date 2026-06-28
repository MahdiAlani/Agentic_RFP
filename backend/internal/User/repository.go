package user

import (
	"context"

	"rfp-agent/internal/database"
)

type Repository interface {
	GetByID(ctx context.Context, id int) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, u *User) (*User, error)
	Update(ctx context.Context, u *User) (*User, error)
	Delete(ctx context.Context, id int) error
}

type postgresRepo struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &postgresRepo{db: db}
}

func (r *postgresRepo) GetByID(ctx context.Context, id int) (*User, error) {
	u := &User{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, email, name, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

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

func (r *postgresRepo) Create(ctx context.Context, u *User) (*User, error) {
	created := &User{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO users (email, name) VALUES ($1, $2) RETURNING id, email, name, created_at`,
		u.Email, u.Name,
	).Scan(&created.ID, &created.Email, &created.Name, &created.CreatedAt)
	if err != nil {
		return nil, err
	}
	return created, nil
}

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

func (r *postgresRepo) Delete(ctx context.Context, id int) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}
