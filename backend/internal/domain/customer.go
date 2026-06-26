package domain

import (
	"time"

	"github.com/google/uuid"
)

type Customer struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Email        *string   `json:"email"` // nullable
	Phone        *string   `json:"phone"` // nullable
	PasswordHash string    `json:"-"`
	IsVerified   bool      `json:"is_verified"`
	IsGuest      bool      `json:"is_guest"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CustomerAddress struct {
	ID         uuid.UUID `json:"id"`
	CustomerID uuid.UUID `json:"customer_id"`
	Label      string    `json:"label"`
	Name       string    `json:"name"`
	Phone      string    `json:"phone"`
	Line1      string    `json:"line1"`
	Line2      string    `json:"line2"`
	City       string    `json:"city"`
	State      string    `json:"state"`
	Country    string    `json:"country"`
	PostalCode string    `json:"postal_code"`
	Lat        *float64  `json:"lat"`
	Lng        *float64  `json:"lng"`
	IsDefault  bool      `json:"is_default"`
	CreatedAt  time.Time `json:"created_at"`
}
