package domain

import (
	"time"

	"github.com/google/uuid"
)

type OrgLocation struct {
	ID         uuid.UUID `json:"id"`
	OrgID      uuid.UUID `json:"org_id"`
	Name       string    `json:"name"`
	Address    string    `json:"address"`
	City       string    `json:"city"`
	State      string    `json:"state"`
	Country    string    `json:"country"`
	PostalCode string    `json:"postal_code"`
	Lat        *float64  `json:"lat"`
	Lng        *float64  `json:"lng"`
	IsPrimary  bool      `json:"is_primary"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
