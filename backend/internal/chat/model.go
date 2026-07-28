package chat

import (
	"time"

	"github.com/google/uuid"
)

type conversation struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type message struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Role           string    `json:"role"` // 'user' | 'assistant'
	Content        string    `json:"content"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}
