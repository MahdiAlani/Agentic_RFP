package project

import (
	"context"

	"github.com/jackc/pgx/v5"

	"rfp-agent/internal/database"

	"github.com/google/uuid"
)

// Project repo
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*project, error)
	GetByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]*project, error)
	Create(ctx context.Context, u *project) (*project, error)
	Update(ctx context.Context, u *project) (*project, error)
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

// Gets a project using ID
func (r *postgresRepo) GetByID(ctx context.Context, id uuid.UUID) (*project, error) {
	p := &project{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, workspace_id, name, created_at FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Gets projects for a workspace
func (r *postgresRepo) GetByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]*project, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, workspace_id, name, created_at FROM projects WHERE workspace_id = $1`, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	// Close connection
	defer rows.Close()

	var projects []*project
	for rows.Next() {
		p := &project{}
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return projects, nil
}

// Creates a new project
func (r *postgresRepo) Create(ctx context.Context, u *project) (*project, error) {
	created := &project{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name) VALUES ($1, $2) RETURNING id, workspace_id, name, created_at`,
		u.WorkspaceID, u.Name,
	).Scan(&created.ID, &created.WorkspaceID, &created.Name, &created.CreatedAt)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Updates a project's data
func (r *postgresRepo) Update(ctx context.Context, u *project) (*project, error) {
	updated := &project{}
	err := r.db.Pool.QueryRow(ctx,
		`UPDATE projects SET name = $1 WHERE id = $2 RETURNING id, workspace_id, name, created_at`,
		u.Name, u.ID,
	).Scan(&updated.ID, &updated.WorkspaceID, &updated.Name, &updated.CreatedAt)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Deletes a project from the db
func (r *postgresRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
