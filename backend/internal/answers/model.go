package answers

import (
	"time"

	"github.com/google/uuid"
)

// citation points at the document chunk that grounded an answer.
type citation struct {
	DocumentID uuid.UUID `json:"document_id"`
	FileName   string    `json:"file_name"`
	ChunkRef   string    `json:"chunk_ref"`
}

// answer is one question from an RFP plus its (AI-drafted, user-editable) reply.
// Status: 'new' -> 'pending' -> 'done', or 'failed'.
type answer struct {
	ID          uuid.UUID  `json:"id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	ProjectID   *uuid.UUID `json:"project_id,omitempty"`
	DocumentID  *uuid.UUID `json:"document_id,omitempty"`
	Question    string     `json:"question"`
	AnswerText  *string    `json:"answer_text,omitempty"`
	Status      string     `json:"status"`
	Position    int        `json:"position"`
	CreatedAt   time.Time  `json:"created_at"`
	Citations   []citation `json:"citations,omitempty"`
}
