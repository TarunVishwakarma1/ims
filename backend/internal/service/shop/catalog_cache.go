package shop

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"time"
)

const (
	keyCategories    = "shop:categories:%s"
	keyProductList   = "shop:plist:%s:%s"
	keyProductDetail = "shop:pdet:%s:%s"
	keyFeedTier      = "shop:feed:%s:%s:%s:%d"
	keyPopular       = "shop:popular:%s"

	ttlShort  = 30 * time.Second
	ttlMedium = 5 * time.Minute
	ttlLong   = 30 * time.Minute
)

func plistHash(q ProductListQuery) string {
	b, _ := json.Marshal(q)
	h := sha1.Sum(b)
	return hex.EncodeToString(h[:])
}
