package domain

import (
	"time"

	"github.com/google/uuid"
)

type Banner struct {
	ID             uuid.UUID `json:"id"`
	OrgID          uuid.UUID `json:"org_id"`
	Title          string    `json:"title"`
	Subtitle       string    `json:"subtitle,omitempty"`
	ImageURL       string    `json:"image_url,omitempty"`
	CTALabel       string    `json:"cta_label,omitempty"`
	CTALink        string    `json:"cta_link,omitempty"`
	EventKey       string    `json:"event_key,omitempty"`
	StartsAt       time.Time `json:"starts_at"`
	EndsAt         time.Time `json:"ends_at"`
	Status         string    `json:"status"`
	SortOrder      int       `json:"sort_order"`
	IsHero         bool      `json:"is_hero"`
	AudienceFilter string    `json:"audience_filter,omitempty"`
	CategorySlug   string    `json:"category_slug,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
