package document

import (
	"time"

	"github.com/google/uuid"
)

type document struct {
	ID           uuid.UUID  `json:"id"`
	WorkspaceID  uuid.UUID  `json:"workspace_id"`
	ProjectID    *uuid.UUID `json:"project_id,omitempty"`
	FileName     string     `json:"file_name"`
	FileKey      string     `json:"file_key"`
	DocumentType string     `json:"document_type"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
}
