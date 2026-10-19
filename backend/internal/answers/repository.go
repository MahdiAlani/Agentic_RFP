package answers

import (
	"context"

	"github.com/jackc/pgx/v5"

	"rfp-agent/internal/database"

	"github.com/google/uuid"
)

// Shared column list so every query returns the same shape.
const answerCols = `id, workspace_id, project_id, document_id, question, answer_text, status, position, created_at`

// answer repo
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*answer, error)
	GetByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]*answer, error)
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*answer, error)
	Create(ctx context.Context, a *answer) (*answer, error)
	UpdateText(ctx context.Context, id uuid.UUID, text string) (*answer, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*answer, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CitationsByAnswer(ctx context.Context, answerID uuid.UUID) ([]citation, error)
	CitationsByProject(ctx context.Context, projectID uuid.UUID) (map[uuid.UUID][]citation, error)
	ProjectName(ctx context.Context, projectID uuid.UUID) (string, error)
}

// db
type postgresRepo struct {
	db *database.DB
}

// Repo constructor
func NewRepository(db *database.DB) Repository {
	return &postgresRepo{db: db}
}

// scanner is satisfied by both pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanAnswer(s scanner) (*answer, error) {
	a := &answer{}
	err := s.Scan(&a.ID, &a.WorkspaceID, &a.ProjectID, &a.DocumentID, &a.Question,
		&a.AnswerText, &a.Status, &a.Position, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Gets an answer using ID
func (r *postgresRepo) GetByID(ctx context.Context, id uuid.UUID) (*answer, error) {
	return scanAnswer(r.db.Pool.QueryRow(ctx,
		`SELECT `+answerCols+` FROM answers WHERE id = $1`, id,
	))
}

// Gets answers for a workspace
func (r *postgresRepo) GetByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]*answer, error) {
	return r.query(ctx,
		`SELECT `+answerCols+` FROM answers WHERE workspace_id = $1 ORDER BY position, created_at`,
		workspaceID)
}

// Gets answers for a project, in RFP order
func (r *postgresRepo) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*answer, error) {
	return r.query(ctx,
		`SELECT `+answerCols+` FROM answers WHERE project_id = $1 ORDER BY position, created_at`,
		projectID)
}

func (r *postgresRepo) query(ctx context.Context, sql string, args ...any) ([]*answer, error) {
	rows, err := r.db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []*answer
	for rows.Next() {
		a, err := scanAnswer(rows)
		if err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return answers, nil
}

// Creates a new answer (answer_text left null for the worker to fill)
func (r *postgresRepo) Create(ctx context.Context, a *answer) (*answer, error) {
	return scanAnswer(r.db.Pool.QueryRow(ctx,
		`INSERT INTO answers (workspace_id, project_id, document_id, question, status, position)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+answerCols,
		a.WorkspaceID, a.ProjectID, a.DocumentID, a.Question, a.Status, a.Position,
	))
}

// UpdateText stores a user-edited answer and marks it done.
func (r *postgresRepo) UpdateText(ctx context.Context, id uuid.UUID, text string) (*answer, error) {
	return scanAnswer(r.db.Pool.QueryRow(ctx,
		`UPDATE answers SET answer_text = $1, status = 'done' WHERE id = $2 RETURNING `+answerCols,
		text, id,
	))
}

func (r *postgresRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*answer, error) {
	return scanAnswer(r.db.Pool.QueryRow(ctx,
		`UPDATE answers SET status = $1 WHERE id = $2 RETURNING `+answerCols,
		status, id,
	))
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

// ProjectName is used to title an exported response.
func (r *postgresRepo) ProjectName(ctx context.Context, projectID uuid.UUID) (string, error) {
	var name string
	err := r.db.Pool.QueryRow(ctx, `SELECT name FROM projects WHERE id = $1`, projectID).Scan(&name)
	return name, err
}

// citationSelect returns each citation with its document name and the cited
// passage, ordered to match the answer's [n] markers. The chunk join is a LEFT
// JOIN: a citation outlives its passage text if the document is re-embedded.
const citationSelect = `
	SELECT c.answer_id, c.document_id, d.file_name, c.chunk_ref, c.position, COALESCE(ch.content, '')
	FROM answer_citations c
	JOIN answers a ON a.id = c.answer_id
	JOIN documents d ON d.id = c.document_id
	LEFT JOIN document_chunks ch
		ON ch.document_id = c.document_id AND ch.chunk_index::text = c.chunk_ref`

func (r *postgresRepo) CitationsByAnswer(ctx context.Context, answerID uuid.UUID) ([]citation, error) {
	byAnswer, err := r.citations(ctx, citationSelect+` WHERE c.answer_id = $1 ORDER BY c.position`, answerID)
	if err != nil {
		return nil, err
	}
	return byAnswer[answerID], nil
}

// CitationsByProject fetches every citation for a project's answers in one
// query, keyed by answer id, so listing doesn't need a query per row.
func (r *postgresRepo) CitationsByProject(ctx context.Context, projectID uuid.UUID) (map[uuid.UUID][]citation, error) {
	return r.citations(ctx, citationSelect+` WHERE a.project_id = $1 ORDER BY c.answer_id, c.position`, projectID)
}

func (r *postgresRepo) citations(ctx context.Context, sql string, args ...any) (map[uuid.UUID][]citation, error) {
	rows, err := r.db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byAnswer := make(map[uuid.UUID][]citation)
	for rows.Next() {
		var answerID uuid.UUID
		var c citation
		if err := rows.Scan(&answerID, &c.DocumentID, &c.FileName, &c.ChunkRef, &c.Position, &c.Content); err != nil {
			return nil, err
		}
		byAnswer[answerID] = append(byAnswer[answerID], c)
	}
	return byAnswer, rows.Err()
}
