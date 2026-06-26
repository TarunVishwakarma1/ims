// Package devseed inserts demo categories, products, inventory, and banners
// into the shop org. Idempotent (ON CONFLICT DO NOTHING on stable UUIDs).
// Intended for development only; gated by config.SeedDevData.
package devseed

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type catSeed struct {
	id        uuid.UUID
	slug      string
	name      string
	iconURL   string
	sortOrder int
}

type prodSeed struct {
	id          uuid.UUID
	categoryID  uuid.UUID
	sku         string
	shopSlug    string
	name        string
	description string
	pricePaise  int64
	gstRate     int
	imageURL    string
	stockQty    int64
}

func mustUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}

// img returns a deterministic placeholder image URL based on slug.
func img(slug string) string {
	return fmt.Sprintf("https://picsum.photos/seed/kirana-%s/600/600", slug)
}

func categories() []catSeed {
	return []catSeed{
		{mustUUID("c0000001-0000-4000-8000-000000000001"), "snacks", "Snacks & Namkeen", img("snacks-cat"), 10},
		{mustUUID("c0000001-0000-4000-8000-000000000002"), "beverages", "Beverages", img("beverages-cat"), 20},
		{mustUUID("c0000001-0000-4000-8000-000000000003"), "dairy", "Dairy & Eggs", img("dairy-cat"), 30},
		{mustUUID("c0000001-0000-4000-8000-000000000004"), "staples", "Staples & Grains", img("staples-cat"), 40},
		{mustUUID("c0000001-0000-4000-8000-000000000005"), "personal-care", "Personal Care", img("personal-cat"), 50},
	}
}

func products() []prodSeed {
	snacks := mustUUID("c0000001-0000-4000-8000-000000000001")
	bev := mustUUID("c0000001-0000-4000-8000-000000000002")
	dairy := mustUUID("c0000001-0000-4000-8000-000000000003")
	staples := mustUUID("c0000001-0000-4000-8000-000000000004")
	personal := mustUUID("c0000001-0000-4000-8000-000000000005")

	return []prodSeed{
		// Snacks
		{mustUUID("d0000001-0000-4000-8000-000000000001"), snacks, "KIRANA-SNK-LAYS-MM-52G", "lays-magic-masala-52g", "Lay's Magic Masala 52g", "Crunchy potato chips with magic masala flavour.", 2000, 12, img("lays-mm"), 120},
		{mustUUID("d0000001-0000-4000-8000-000000000002"), snacks, "KIRANA-SNK-KURKURE-MM-90G", "kurkure-masala-munch-90g", "Kurkure Masala Munch 90g", "Crispy corn puffs with masala munch flavour.", 2000, 12, img("kurkure-mm"), 95},
		{mustUUID("d0000001-0000-4000-8000-000000000003"), snacks, "KIRANA-SNK-MARIE-GOLD-250G", "britannia-marie-gold-250g", "Britannia Marie Gold 250g", "Light tea-time biscuit, perfect with chai.", 4500, 18, img("marie-gold"), 150},
		{mustUUID("d0000001-0000-4000-8000-000000000004"), snacks, "KIRANA-SNK-PARLE-G-200G", "parle-g-200g", "Parle-G Glucose Biscuits 200g", "The original glucose biscuit since 1939.", 3000, 18, img("parle-g"), 200},

		// Beverages
		{mustUUID("d0000001-0000-4000-8000-000000000005"), bev, "KIRANA-BEV-COKE-750ML", "coca-cola-750ml", "Coca-Cola 750ml", "Refreshing cola in a sharing-size bottle.", 4000, 28, img("coke"), 80},
		{mustUUID("d0000001-0000-4000-8000-000000000006"), bev, "KIRANA-BEV-BISLERI-1L", "bisleri-water-1l", "Bisleri Mineral Water 1L", "Purified mineral water, single bottle.", 2000, 0, img("bisleri"), 300},
		{mustUUID("d0000001-0000-4000-8000-000000000007"), bev, "KIRANA-BEV-TROPICANA-OJ-1L", "tropicana-orange-1l", "Tropicana Orange Juice 1L", "100% orange juice, no added sugar.", 11000, 12, img("tropicana"), 50},
		{mustUUID("d0000001-0000-4000-8000-000000000008"), bev, "KIRANA-BEV-REAL-MIXED-1L", "real-mixed-fruit-1l", "Real Mixed Fruit Juice 1L", "Goodness of seven fruits in one pack.", 12500, 12, img("real-mf"), 45},

		// Dairy
		{mustUUID("d0000001-0000-4000-8000-000000000009"), dairy, "KIRANA-DAI-AMUL-GOLD-1L", "amul-gold-milk-1l", "Amul Gold Full Cream Milk 1L", "Tetra-pack full-cream milk, 6% fat.", 7500, 5, img("amul-gold"), 100},
		{mustUUID("d0000001-0000-4000-8000-00000000000a"), dairy, "KIRANA-DAI-MD-CURD-400G", "mother-dairy-curd-400g", "Mother Dairy Fresh Curd 400g", "Fresh thick set curd, classic taste.", 5500, 5, img("md-curd"), 70},
		{mustUUID("d0000001-0000-4000-8000-00000000000b"), dairy, "KIRANA-DAI-AMUL-BUTTER-100G", "amul-butter-100g", "Amul Butter 100g", "Utterly butterly delicious salted butter.", 5800, 12, img("amul-butter"), 90},
		{mustUUID("d0000001-0000-4000-8000-00000000000c"), dairy, "KIRANA-DAI-EGGS-12PC", "farm-eggs-12pc", "Farm Fresh Eggs (12 pcs)", "Locally sourced grade-A chicken eggs.", 9000, 0, img("eggs"), 60},

		// Staples
		{mustUUID("d0000001-0000-4000-8000-00000000000d"), staples, "KIRANA-STA-INDIAGATE-1KG", "india-gate-basmati-1kg", "India Gate Classic Basmati Rice 1kg", "Long-grain aged basmati rice.", 22000, 5, img("indiagate"), 80},
		{mustUUID("d0000001-0000-4000-8000-00000000000e"), staples, "KIRANA-STA-AASHIRVAAD-5KG", "aashirvaad-atta-5kg", "Aashirvaad Whole Wheat Atta 5kg", "100% MP wheat, sharbati blend.", 27500, 5, img("aashirvaad"), 50},
		{mustUUID("d0000001-0000-4000-8000-00000000000f"), staples, "KIRANA-STA-TATA-SALT-1KG", "tata-salt-1kg", "Tata Salt 1kg", "Iodised vacuum-evaporated salt.", 2800, 5, img("tata-salt"), 200},
		{mustUUID("d0000001-0000-4000-8000-000000000010"), staples, "KIRANA-STA-FORTUNE-SFL-1L", "fortune-sunflower-1l", "Fortune Sunlite Sunflower Oil 1L", "Refined sunflower cooking oil.", 16500, 5, img("fortune-sfl"), 65},

		// Personal Care
		{mustUUID("d0000001-0000-4000-8000-000000000011"), personal, "KIRANA-PC-DOVE-100G", "dove-soap-100g", "Dove Cream Beauty Bathing Bar 100g", "1/4 moisturising cream soap bar.", 6500, 18, img("dove"), 100},
		{mustUUID("d0000001-0000-4000-8000-000000000012"), personal, "KIRANA-PC-COLGATE-ST-200G", "colgate-strong-200g", "Colgate Strong Teeth Toothpaste 200g", "Calcium-enriched cavity protection.", 11000, 18, img("colgate"), 80},
		{mustUUID("d0000001-0000-4000-8000-000000000013"), personal, "KIRANA-PC-HEADSHOULDERS-180ML", "head-shoulders-180ml", "Head & Shoulders Anti-Dandruff Shampoo 180ml", "Cool menthol anti-dandruff shampoo.", 18500, 18, img("h-and-s"), 55},
		{mustUUID("d0000001-0000-4000-8000-000000000014"), personal, "KIRANA-PC-VIM-LIQ-500ML", "vim-dishwash-500ml", "Vim Dishwash Liquid 500ml", "Strong grease-cutting lemon liquid.", 14500, 18, img("vim"), 75},
	}
}

// Run inserts the demo catalog into the shop org. Safe to call repeatedly.
func Run(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now()

	for _, c := range categories() {
		_, err := tx.Exec(ctx, `
			INSERT INTO categories
			  (id, org_id, name, description, created_at, updated_at,
			   slug, icon_url, shop_visible, sort_order)
			VALUES ($1, $2, $3, '', $4, $4, $5, $6, TRUE, $7)
			ON CONFLICT (id) DO NOTHING
		`, c.id, orgID, c.name, now, c.slug, c.iconURL, c.sortOrder)
		if err != nil {
			return fmt.Errorf("seed category %s: %w", c.slug, err)
		}
	}

	for _, p := range products() {
		_, err := tx.Exec(ctx, `
			INSERT INTO products
			  (id, category_id, name, description, sku, price, created_at, updated_at,
			   org_id, gst_rate, shop_visible, shop_slug, shop_description,
			   shop_image_urls, shop_price_paise)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $7,
			        $8, $9, TRUE, $10, $4,
			        ARRAY[$11]::TEXT[], $6)
			ON CONFLICT (id) DO NOTHING
		`, p.id, p.categoryID, p.name, p.description, p.sku, p.pricePaise, now,
			orgID, p.gstRate, p.shopSlug, p.imageURL)
		if err != nil {
			return fmt.Errorf("seed product %s: %w", p.shopSlug, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO inventory
			  (product_id, quantity, low_stock_threshold, org_id, updated_at)
			VALUES ($1, $2, 10, $3, $4)
			ON CONFLICT (product_id) DO NOTHING
		`, p.id, p.stockQty, orgID, now)
		if err != nil {
			return fmt.Errorf("seed inventory %s: %w", p.shopSlug, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
