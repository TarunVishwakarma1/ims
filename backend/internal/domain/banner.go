package domain

import (
	"time"

	"github.com/google/uuid"
)

type Banner struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	Title          string
	Subtitle       string
	ImageURL       string
	CTALabel       string
	CTALink        string
	EventKey       string
	StartsAt       time.Time
	EndsAt         time.Time
	Status         string
	SortOrder      int
	IsHero         bool
	AudienceFilter string
	CategorySlug   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
