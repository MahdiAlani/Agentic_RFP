package answers

import (
	"context"

	"github.com/jackc/pgx/v5"

	"rfp-agent/internal/database"

	"github.com/google/uuid"
)

// answer repo
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*answer, error)
	GetByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]*answer, error)
	Create(ctx context.Context, a *answer) (*answer, error)
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

// Gets an answer using ID
func (r *postgresRepo) GetByID(ctx context.Context, id uuid.UUID) (*answer, error) {
	a := &answer{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, workspace_id, project_id, document_id, question, answer_text, status, created_at FROM answers WHERE id = $1`, id,
	).Scan(&a.ID, &a.WorkspaceID, &a.ProjectID, &a.DocumentID, &a.Question, &a.AnswerText, &a.Status, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Gets answers for a workspace
func (r *postgresRepo) GetByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]*answer, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, workspace_id, project_id, document_id, question, answer_text, status, created_at FROM answers WHERE workspace_id = $1`, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []*answer
	for rows.Next() {
		a := &answer{}
		if err := rows.Scan(&a.ID, &a.WorkspaceID, &a.ProjectID, &a.DocumentID, &a.Question, &a.AnswerText, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return answers, nil
}

// Creates a new answer (pending, answer_text left null for the worker to fill)
func (r *postgresRepo) Create(ctx context.Context, a *answer) (*answer, error) {
	created := &answer{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO answers (workspace_id, project_id, document_id, question, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, workspace_id, project_id, document_id, question, answer_text, status, created_at`,
		a.WorkspaceID, a.ProjectID, a.DocumentID, a.Question, a.Status,
	).Scan(&created.ID, &created.WorkspaceID, &created.ProjectID, &created.DocumentID, &created.Question, &created.AnswerText, &created.Status, &created.CreatedAt)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Deletes an answer from the db
func (r *postgresRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM answers WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
