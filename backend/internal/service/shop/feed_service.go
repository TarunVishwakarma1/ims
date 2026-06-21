package shop

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
)

type FeedPageInfo struct {
	Tier string `json:"tier"`
	Page int    `json:"page"`
}

type FeedPage struct {
	Items      []ProductCard `json:"items"`
	NextCursor string        `json:"next_cursor,omitempty"`
	PageInfo   FeedPageInfo  `json:"page_info"`
}

type FeedService interface {
	Page(ctx context.Context, cursor, seedCategory string, limit int) (*FeedPage, error)
}

type feedCursor struct {
	Tier   string `json:"t"`
	Page   int    `json:"p"`
	Seed   string `json:"s"`
	Skip   int    `json:"k"` // rows already consumed from current tier
	Bucket string `json:"b"` // seedCategory carry-forward
}

func encodeFeedCursor(c feedCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeFeedCursor(s string) (feedCursor, error) {
	var c feedCursor
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	return c, nil
}

// EncodeFeedCursorForTest exists ONLY for tests in this package.
func EncodeFeedCursorForTest(tier string, page int, bucket, seed string) string {
	return encodeFeedCursor(feedCursor{Tier: tier, Page: page, Bucket: bucket, Seed: seed})
}

type feedService struct {
	pool  *pgxpool.Pool
	cache cache.Cache
	orgID uuid.UUID
}

func NewFeedService(pool *pgxpool.Pool, c cache.Cache, orgID uuid.UUID) FeedService {
	return &feedService{pool, c, orgID}
}

func (s *feedService) Page(ctx context.Context, cursor, seedCategory string, limit int) (*FeedPage, error) {
	if limit <= 0 {
		limit = 24
	}
	if limit > 100 {
		limit = 100
	}

	var cur feedCursor
	if cursor != "" {
		c, err := decodeFeedCursor(cursor)
		if err == nil {
			if len(c.Seed) > 64 || len(c.Bucket) > 200 || !validFeedTier(c.Tier) {
				zap.L().Debug("shop feed: invalid cursor payload, dropping", zap.String("tier", c.Tier))
			} else {
				cur = c
			}
		}
	}
	if cur.Tier == "" {
		cur = feedCursor{Tier: "category", Page: 1, Bucket: seedCategory, Seed: randomSeed()}
	}

	items := []ProductCard{}
	seen := make(map[uuid.UUID]struct{})
	// Report the tier that first contributed items (or the starting tier if none).
	reportTier := cur.Tier
	reportPage := cur.Page

	for len(items) < limit && cur.Tier != "" {
		need := limit - len(items)
		tierItems, exhausted, err := s.fetchTier(ctx, cur, need)
		if err != nil {
			return nil, err
		}
		// Dedup: skip items already delivered in this Page() call.
		added := 0
		for _, item := range tierItems {
			if _, dup := seen[item.ID]; !dup {
				seen[item.ID] = struct{}{}
				if len(items) == 0 && added == 0 {
					// First tier to contribute new items — record it.
					reportTier = cur.Tier
					reportPage = cur.Page
				}
				items = append(items, item)
				added++
			}
		}
		cur.Skip += len(tierItems)
		if exhausted {
			prevTier := cur.Tier
			cur = s.advance(cur)
			// Guard: random tier loops back on itself. If we just advanced random→random
			// AND produced no items this round, the catalog is empty/exhausted — break
			// to avoid infinite loop.
			if cur.Tier == prevTier && len(tierItems) == 0 {
				break
			}
		} else {
			break
		}
	}
	out := &FeedPage{PageInfo: FeedPageInfo{Tier: reportTier, Page: reportPage}}
	out.Items = items
	if cur.Tier != "" {
		out.NextCursor = encodeFeedCursor(cur)
	}
	return out, nil
}

// fetchTier returns up to `n` items from the current tier and whether the tier
// is now exhausted. Random tier is never exhausted when it returns items
// (it loops forever), so within a single Page() call we stop fill-through
// once random contributes something.
func (s *feedService) fetchTier(ctx context.Context, cur feedCursor, n int) ([]ProductCard, bool, error) {
	switch cur.Tier {
	case "category":
		return s.tierFromCategory(ctx, cur.Bucket, cur.Skip, n)
	case "related":
		return s.tierRelated(ctx, cur.Bucket, cur.Skip, n)
	case "popular":
		return s.tierPopular(ctx, cur.Skip, n)
	case "random":
		items, exhausted, err := s.tierRandom(ctx, cur.Seed, cur.Skip, n)
		if err != nil {
			return nil, false, err
		}
		if len(items) > 0 {
			// Random tier always has more (it cycles) — stop fill-through.
			return items, false, nil
		}
		return items, exhausted, nil
	}
	return nil, true, nil
}

func (s *feedService) advance(cur feedCursor) feedCursor {
	cur.Skip = 0
	cur.Page++
	switch cur.Tier {
	case "category":
		cur.Tier = "related"
	case "related":
		cur.Tier = "popular"
	case "popular":
		cur.Tier = "random"
	case "random":
		// stay; random loops forever via seed
		return cur
	}
	return cur
}

func (s *feedService) tierFromCategory(ctx context.Context, slug string, skip, n int) ([]ProductCard, bool, error) {
	if slug == "" {
		return nil, true, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, COALESCE(p.shop_slug,''), p.name,
		       COALESCE(p.shop_price_paise, p.price),
		       COALESCE(p.shop_image_urls[1], ''),
		       COALESCE(i.quantity, 0),
		       COALESCE(c.slug, '')
		  FROM products p
		  LEFT JOIN inventory i ON i.product_id = p.id
		  LEFT JOIN categories c ON c.id = p.category_id
		 WHERE p.org_id=$1 AND p.shop_visible=TRUE AND c.slug=$2
		 ORDER BY p.created_at DESC, p.id
		 LIMIT $3 OFFSET $4`, s.orgID, slug, n+1, skip)
	return readProductCards(rows, err, n)
}

func (s *feedService) tierRelated(ctx context.Context, slug string, skip, n int) ([]ProductCard, bool, error) {
	if slug == "" {
		return nil, true, nil
	}
	// "Related" V1 heuristic: pick any other shop-visible categories.
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, COALESCE(p.shop_slug,''), p.name,
		       COALESCE(p.shop_price_paise, p.price),
		       COALESCE(p.shop_image_urls[1], ''),
		       COALESCE(i.quantity, 0),
		       COALESCE(c.slug, '')
		  FROM products p
		  LEFT JOIN inventory i ON i.product_id = p.id
		  LEFT JOIN categories c ON c.id = p.category_id
		 WHERE p.org_id=$1 AND p.shop_visible=TRUE
		   AND c.shop_visible=TRUE AND c.slug <> $2
		 ORDER BY c.sort_order, p.created_at DESC, p.id
		 LIMIT $3 OFFSET $4`, s.orgID, slug, n+1, skip)
	return readProductCards(rows, err, n)
}

func (s *feedService) tierPopular(ctx context.Context, skip, n int) ([]ProductCard, bool, error) {
	var popMap map[uuid.UUID]int
	_ = s.cache.Get(ctx, "shop:popular:"+s.orgID.String(), &popMap)
	// Fallback: plain newest if no popularity yet.
	if len(popMap) == 0 {
		rows, err := s.pool.Query(ctx, `
			SELECT p.id, COALESCE(p.shop_slug,''), p.name,
			       COALESCE(p.shop_price_paise, p.price),
			       COALESCE(p.shop_image_urls[1], ''),
			       COALESCE(i.quantity, 0),
			       COALESCE(c.slug, '')
			  FROM products p
			  LEFT JOIN inventory i ON i.product_id = p.id
			  LEFT JOIN categories c ON c.id = p.category_id
			 WHERE p.org_id=$1 AND p.shop_visible=TRUE
			 ORDER BY p.created_at DESC, p.id
			 LIMIT $2 OFFSET $3`, s.orgID, n+1, skip)
		return readProductCards(rows, err, n)
	}
	// With popMap: VALUES join.
	args := []any{s.orgID}
	var b strings.Builder
	b.WriteString(`WITH pop(pid,score) AS (VALUES `)
	first := true
	for id, sc := range popMap {
		if !first {
			b.WriteString(",")
		}
		first = false
		args = append(args, id, sc)
		fmt.Fprintf(&b, "($%d::uuid,$%d::int)", len(args)-1, len(args))
	}
	b.WriteString(`)`)
	args = append(args, n+1, skip)
	query := b.String() + fmt.Sprintf(`
		SELECT p.id, COALESCE(p.shop_slug,''), p.name,
		       COALESCE(p.shop_price_paise, p.price),
		       COALESCE(p.shop_image_urls[1], ''),
		       COALESCE(i.quantity, 0),
		       COALESCE(c.slug, '')
		  FROM products p
		  LEFT JOIN inventory i ON i.product_id = p.id
		  LEFT JOIN categories c ON c.id = p.category_id
		  LEFT JOIN pop ON pop.pid = p.id
		 WHERE p.org_id=$1 AND p.shop_visible=TRUE
		 ORDER BY COALESCE(pop.score,0) DESC, p.created_at DESC, p.id
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	return readProductCards(rows, err, n)
}

func (s *feedService) tierRandom(ctx context.Context, seed string, skip, n int) ([]ProductCard, bool, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, COALESCE(p.shop_slug,''), p.name,
		       COALESCE(p.shop_price_paise, p.price),
		       COALESCE(p.shop_image_urls[1], ''),
		       COALESCE(i.quantity, 0),
		       COALESCE(c.slug, '')
		  FROM products p
		  LEFT JOIN inventory i ON i.product_id = p.id
		  LEFT JOIN categories c ON c.id = p.category_id
		 WHERE p.org_id=$1 AND p.shop_visible=TRUE
		 ORDER BY md5(p.id::text || $2)
		 LIMIT $3 OFFSET $4`, s.orgID, seed, n+1, skip)
	return readProductCards(rows, err, n)
}

func readProductCards(rows pgx.Rows, qerr error, want int) ([]ProductCard, bool, error) {
	if qerr != nil {
		return nil, false, qerr
	}
	defer rows.Close()
	got := []ProductCard{}
	for rows.Next() {
		var p ProductCard
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.PricePaise, &p.ImageURL, &p.AvailableQty, &p.CategorySlug); err != nil {
			return nil, false, err
		}
		got = append(got, p)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	// We asked for want+1. If we got that many, more remain.
	exhausted := len(got) <= want
	if len(got) > want {
		got = got[:want]
	}
	return got, exhausted, nil
}

func validFeedTier(t string) bool {
	switch t {
	case "", "category", "related", "popular", "random":
		return true
	}
	return false
}

func randomSeed() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
