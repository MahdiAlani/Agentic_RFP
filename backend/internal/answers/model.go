package answers

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// citation points at the document chunk that grounded an answer. Position is
// the [n] marker the answer text uses for it; Content is the cited passage.
type citation struct {
	DocumentID uuid.UUID `json:"document_id"`
	FileName   string    `json:"file_name"`
	ChunkRef   string    `json:"chunk_ref"`
	Position   int       `json:"position"`
	Content    string    `json:"content"`
}

// answer is one question from an RFP plus its (AI-drafted, user-editable) reply.
// Status: 'new' -> 'pending' -> 'done' -> 'approved', or 'failed'.
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
	// Checks are the deterministic quality checks a worker last ran, as
	// [{name, passed, severity, detail}]. ChecksCurrent says they were run on
	// the current text; approval needs that.
	Checks        json.RawMessage `json:"checks,omitempty"`
	ChecksCurrent bool            `json:"checks_current"`
}

// check is one entry of answer.Checks.
type check struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Severity string `json:"severity"`
	Detail   string `json:"detail"`
}
