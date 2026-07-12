package answers

import (
	"time"

	"github.com/google/uuid"
)

type answer struct {
	ID          uuid.UUID  `json:"id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	ProjectID   *uuid.UUID `json:"project_id,omitempty"`
	DocumentID  *uuid.UUID `json:"document_id,omitempty"`
	Question    string     `json:"question"`
	AnswerText  *string    `json:"answer_text,omitempty"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
}
