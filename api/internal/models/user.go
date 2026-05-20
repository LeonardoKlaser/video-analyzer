package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID              uuid.UUID        `json:"id"`
	Email           string           `json:"email"`
	BusinessContext *BusinessContext `json:"business_context,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}
