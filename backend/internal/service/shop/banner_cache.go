package shop

import (
	"time"

	"github.com/google/uuid"
)

const (
	bannerCacheKeyPrefix = "shop:banners:active:"
	bannerCacheTTL       = 5 * time.Minute
)

func bannerActiveKey(orgID uuid.UUID, categorySlug string) string {
	if categorySlug == "" {
		return bannerCacheKeyPrefix + orgID.String()
	}
	return bannerCacheKeyPrefix + orgID.String() + ":" + categorySlug
}
