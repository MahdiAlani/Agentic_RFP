package project

import (
	"time"
	"github.com/google/uuid"
)

type project struct {
	ID           uuid.UUID `json:"id"`
	WorkspaceID  uuid.UUID `json:"workspace_id"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
}
