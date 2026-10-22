package chat

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"rfp-agent/internal/database"

	"github.com/google/uuid"
)

type Repository interface {
	CreateOrReuseEmptyConversation(ctx context.Context, c *conversation) (*conversation, bool, error)
	GetConversation(ctx context.Context, id uuid.UUID) (*conversation, error)
	ListConversations(ctx context.Context, projectID uuid.UUID) ([]*conversation, error)
	UpdateConversationTitle(ctx context.Context, id uuid.UUID, title string) (*conversation, error)
	DeleteConversation(ctx context.Context, id uuid.UUID) error
	CreateMessage(ctx context.Context, m *message) (*message, error)
	ListMessages(ctx context.Context, conversationID uuid.UUID) ([]*message, error)
}

type postgresRepo struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &postgresRepo{db: db}
}

// CreateOrReuseEmptyConversation returns the project's newest conversation with
// no messages, or inserts c if there is none. The bool reports whether c was inserted.
func (r *postgresRepo) CreateOrReuseEmptyConversation(ctx context.Context, c *conversation) (*conversation, bool, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	// Locking the project row makes concurrent creates for the same project wait
	// for each other, so only one of them can see "no empty chat" and insert.
	// If the project doesn't exist, this locks nothing and the insert below
	// reports the foreign key error.
	if _, err := tx.Exec(ctx, `SELECT 1 FROM projects WHERE id = $1 FOR UPDATE`, c.ProjectID); err != nil {
		return nil, false, err
	}

	found := &conversation{}
	err = tx.QueryRow(ctx,
		`SELECT id, project_id, title, created_at FROM conversations
		 WHERE project_id = $1
		   AND NOT EXISTS (SELECT 1 FROM messages WHERE messages.conversation_id = conversations.id)
		 ORDER BY created_at DESC
		 LIMIT 1`,
		c.ProjectID,
	).Scan(&found.ID, &found.ProjectID, &found.Title, &found.CreatedAt)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		return found, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}

	created := &conversation{}
	err = tx.QueryRow(ctx,
		`INSERT INTO conversations (project_id, title) VALUES ($1, $2)
		 RETURNING id, project_id, title, created_at`,
		c.ProjectID, c.Title,
	).Scan(&created.ID, &created.ProjectID, &created.Title, &created.CreatedAt)
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return created, true, nil
}

func (r *postgresRepo) GetConversation(ctx context.Context, id uuid.UUID) (*conversation, error) {
	c := &conversation{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, project_id, title, created_at FROM conversations WHERE id = $1`, id,
	).Scan(&c.ID, &c.ProjectID, &c.Title, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *postgresRepo) ListConversations(ctx context.Context, projectID uuid.UUID) ([]*conversation, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, project_id, title, created_at FROM conversations WHERE project_id = $1 ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []*conversation
	for rows.Next() {
		c := &conversation{}
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Title, &c.CreatedAt); err != nil {
			return nil, err
		}
		conversations = append(conversations, c)
	}
	return conversations, rows.Err()
}

func (r *postgresRepo) UpdateConversationTitle(ctx context.Context, id uuid.UUID, title string) (*conversation, error) {
	updated := &conversation{}
	err := r.db.Pool.QueryRow(ctx,
		`UPDATE conversations SET title = $1 WHERE id = $2
		 RETURNING id, project_id, title, created_at`,
		title, id,
	).Scan(&updated.ID, &updated.ProjectID, &updated.Title, &updated.CreatedAt)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Deletes a conversation; its messages go with it via ON DELETE CASCADE.
func (r *postgresRepo) DeleteConversation(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *postgresRepo) CreateMessage(ctx context.Context, m *message) (*message, error) {
	created := &message{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, role, content, status) VALUES ($1, $2, $3, $4)
		 RETURNING id, conversation_id, role, content, status, created_at`,
		m.ConversationID, m.Role, m.Content, m.Status,
	).Scan(&created.ID, &created.ConversationID, &created.Role, &created.Content, &created.Status, &created.CreatedAt)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *postgresRepo) ListMessages(ctx context.Context, conversationID uuid.UUID) ([]*message, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, conversation_id, role, content, status, created_at FROM messages
		 WHERE conversation_id = $1 ORDER BY created_at ASC`,
		conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*message
	for rows.Next() {
		m := &message{}
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Status, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
