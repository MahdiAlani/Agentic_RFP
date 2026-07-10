package document

import (
	"context"

	"github.com/jackc/pgx/v5"

	"rfp-agent/internal/database"

	"github.com/google/uuid"
)

// document repo
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*document, error)
	GetByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]*document, error)
	Create(ctx context.Context, u *document) (*document, error)
	Update(ctx context.Context, u *document) (*document, error)
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

// Gets a document using ID
func (r *postgresRepo) GetByID(ctx context.Context, id uuid.UUID) (*document, error) {
	p := &document{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, workspace_id, name, created_at FROM documents WHERE id = $1`, id,
	).Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Gets documents for a workspace
func (r *postgresRepo) GetByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]*document, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, workspace_id, name, created_at FROM documents WHERE workspace_id = $1`, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	// Close connection
	defer rows.Close()

	var documents []*document
	for rows.Next() {
		p := &document{}
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.CreatedAt); err != nil {
			return nil, err
		}
		documents = append(documents, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return documents, nil
}

// Creates a new document
func (r *postgresRepo) Create(ctx context.Context, u *document) (*document, error) {
	created := &document{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO documents (workspace_id, name) VALUES ($1, $2) RETURNING id, workspace_id, name, created_at`,
		u.WorkspaceID, u.Name,
	).Scan(&created.ID, &created.WorkspaceID, &created.Name, &created.CreatedAt)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Updates a document's data
func (r *postgresRepo) Update(ctx context.Context, u *document) (*document, error) {
	updated := &document{}
	err := r.db.Pool.QueryRow(ctx,
		`UPDATE documents SET name = $1 WHERE id = $2 RETURNING id, workspace_id, name, created_at`,
		u.Name, u.ID,
	).Scan(&updated.ID, &updated.WorkspaceID, &updated.Name, &updated.CreatedAt)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Deletes a document from the db
func (r *postgresRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM documents WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
