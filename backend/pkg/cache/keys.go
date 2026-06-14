package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Standard TTLs. Tune from one place.
const (
	TTLShort  = 30 * time.Second
	TTLMedium = 5 * time.Minute
	TTLLong   = 30 * time.Minute
)

// Key builders — keep them in one file so we never typo a key elsewhere.

func OrgPrefix(orgID uuid.UUID) string {
	return fmt.Sprintf("org:%s", orgID)
}

func ProductsListKey(orgID uuid.UUID) string {
	return OrgPrefix(orgID) + ":products:list"
}

func ProductsListPattern(orgID uuid.UUID) string {
	return OrgPrefix(orgID) + ":products:*"
}

func CategoriesListKey(orgID uuid.UUID) string {
	return OrgPrefix(orgID) + ":categories:list"
}

func CategoriesListPattern(orgID uuid.UUID) string {
	return OrgPrefix(orgID) + ":categories:*"
}

func LocationsListKey(orgID uuid.UUID) string {
	return OrgPrefix(orgID) + ":locations:list"
}

func LocationsListPattern(orgID uuid.UUID) string {
	return OrgPrefix(orgID) + ":locations:*"
}

func ListingsByOrgKey(orgID uuid.UUID) string {
	return OrgPrefix(orgID) + ":listings:list"
}

func ListingsByOrgPattern(orgID uuid.UUID) string {
	return OrgPrefix(orgID) + ":listings:*"
}

// Marketplace search is global. Hash the params into a single key.
func MarketplaceSearchKey(params string) string {
	h := sha1.Sum([]byte(params))
	return "marketplace:search:" + hex.EncodeToString(h[:8])
}

func MarketplaceSearchPattern() string {
	return "marketplace:search:*"
}
