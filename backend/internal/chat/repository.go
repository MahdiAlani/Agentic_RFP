package chat

import (
	"context"

	"github.com/jackc/pgx/v5"

	"rfp-agent/internal/database"

	"github.com/google/uuid"
)

type Repository interface {
	CreateConversation(ctx context.Context, c *conversation) (*conversation, error)
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

func (r *postgresRepo) CreateConversation(ctx context.Context, c *conversation) (*conversation, error) {
	created := &conversation{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO conversations (project_id, title) VALUES ($1, $2)
		 RETURNING id, project_id, title, created_at`,
		c.ProjectID, c.Title,
	).Scan(&created.ID, &created.ProjectID, &created.Title, &created.CreatedAt)
	if err != nil {
		return nil, err
	}
	return created, nil
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
