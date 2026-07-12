package documents

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
	Create(ctx context.Context, d *document) (*document, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*document, error)
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
	d := &document{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, workspace_id, project_id, file_name, file_key, document_type, status, created_at FROM documents WHERE id = $1`, id,
	).Scan(&d.ID, &d.WorkspaceID, &d.ProjectID, &d.FileName, &d.FileKey, &d.DocumentType, &d.Status, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// Gets documents for a workspace
func (r *postgresRepo) GetByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]*document, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, workspace_id, project_id, file_name, file_key, document_type, status, created_at FROM documents WHERE workspace_id = $1`, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var documents []*document
	for rows.Next() {
		d := &document{}
		if err := rows.Scan(&d.ID, &d.WorkspaceID, &d.ProjectID, &d.FileName, &d.FileKey, &d.DocumentType, &d.Status, &d.CreatedAt); err != nil {
			return nil, err
		}
		documents = append(documents, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return documents, nil
}

// Creates a new document with a server-generated id (needed up front to build the file_key)
func (r *postgresRepo) Create(ctx context.Context, d *document) (*document, error) {
	created := &document{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO documents (id, workspace_id, project_id, file_name, file_key, document_type, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, workspace_id, project_id, file_name, file_key, document_type, status, created_at`,
		d.ID, d.WorkspaceID, d.ProjectID, d.FileName, d.FileKey, d.DocumentType, d.Status,
	).Scan(&created.ID, &created.WorkspaceID, &created.ProjectID, &created.FileName, &created.FileKey, &created.DocumentType, &created.Status, &created.CreatedAt)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Updates a document's status (pending -> uploaded, etc.)
func (r *postgresRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*document, error) {
	updated := &document{}
	err := r.db.Pool.QueryRow(ctx,
		`UPDATE documents SET status = $1 WHERE id = $2
		 RETURNING id, workspace_id, project_id, file_name, file_key, document_type, status, created_at`,
		status, id,
	).Scan(&updated.ID, &updated.WorkspaceID, &updated.ProjectID, &updated.FileName, &updated.FileKey, &updated.DocumentType, &updated.Status, &updated.CreatedAt)
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
