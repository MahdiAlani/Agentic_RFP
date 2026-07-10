package document

import (
	"github.com/google/uuid"
	"time"
)

type document struct {
	ID           uuid.UUID `json:"id"`
	WorkspaceID  uuid.UUID `json:"workspace_id"`
	FileName     string    `json:"file_name"`
	FileKey      string    `json:"file_key"`
	DocumentType string    `json:"document_type"`
	CreatedAt    time.Time `json:"created_at"`
}
