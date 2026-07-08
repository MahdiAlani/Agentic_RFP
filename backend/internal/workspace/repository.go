package workspace

import (
	"context"

	"github.com/jackc/pgx/v5"

	"rfp-agent/internal/database"

	"github.com/google/uuid"
)

// workspace repo
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*workspace, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*workspace, error)
	Create(ctx context.Context, u *workspace) (*workspace, error)
	Update(ctx context.Context, u *workspace) (*workspace, error)
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

// Gets a workspace using ID
func (r *postgresRepo) GetByID(ctx context.Context, id uuid.UUID) (*workspace, error) {
	w := &workspace{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, created_at FROM workspaces WHERE id = $1`, id,
	).Scan(&w.ID, &w.UserID, &w.Name, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// Gets workspaces for a user
func (r *postgresRepo) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*workspace, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, name, created_at FROM workspaces WHERE user_id = $1`, userID,
	)
	if err != nil {
		return nil, err
	}
	// Close connection
	defer rows.Close()

	var workspaces []*workspace
	for rows.Next() {
		w := &workspace{}
		if err := rows.Scan(&w.ID, &w.UserID, &w.Name, &w.CreatedAt); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return workspaces, nil
}

// Creates a new workspace
func (r *postgresRepo) Create(ctx context.Context, u *workspace) (*workspace, error) {
	created := &workspace{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO workspaces (user_id, name) VALUES ($1, $2) RETURNING id, user_id, name, created_at`,
		u.UserID, u.Name,
	).Scan(&created.ID, &created.UserID, &created.Name, &created.CreatedAt)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Updates a workspace's data
func (r *postgresRepo) Update(ctx context.Context, u *workspace) (*workspace, error) {
	updated := &workspace{}
	err := r.db.Pool.QueryRow(ctx,
		`UPDATE workspaces SET name = $1 WHERE id = $2 RETURNING id, user_id, name, created_at`,
		u.Name, u.ID,
	).Scan(&updated.ID, &updated.UserID, &updated.Name, &updated.CreatedAt)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Deletes a workspace from the db
func (r *postgresRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
