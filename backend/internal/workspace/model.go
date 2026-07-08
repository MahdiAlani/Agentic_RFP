package workspace

import (
	"time"
	"github.com/google/uuid"
)

type workspace struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
}
