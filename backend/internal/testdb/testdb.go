package testdb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MustOpen connects to the test database. Skips the test if DATABASE_URL_TEST
// (or DATABASE_URL as a fallback) is unset. Tests must clean up data they
// create (use random keys or t.Cleanup).
func MustOpen(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL_TEST")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("no DATABASE_URL_TEST or DATABASE_URL set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// SeedProduct creates a temporary product for the test, registers cleanup,
// and returns the product id. It auto-picks (or creates) an organization and
// category so tests can run on a fresh database with no seed data.
func SeedProduct(t *testing.T, pool *pgxpool.Pool, name string, pricePaise int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	// Pick any existing org, or create a throwaway test org if none exists.
	var orgID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM organizations LIMIT 1`).Scan(&orgID)
	if err != nil {
		slug := fmt.Sprintf("test-org-%d", time.Now().UnixNano())
		if err2 := pool.QueryRow(ctx, `
			INSERT INTO organizations (name, slug) VALUES ($1, $2) RETURNING id
		`, "Test Org", slug).Scan(&orgID); err2 != nil {
			t.Fatalf("seed: create org: %v", err2)
		}
		// Only register cleanup for orgs that SeedProduct created
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id=$1`, orgID)
		})
	}

	// Pick any category from that org, or create a throwaway one.
	var categoryID uuid.UUID
	err = pool.QueryRow(ctx, `SELECT id FROM categories WHERE org_id=$1 LIMIT 1`, orgID).Scan(&categoryID)
	if err != nil {
		if err2 := pool.QueryRow(ctx, `
			INSERT INTO categories (org_id, name) VALUES ($1, $2) RETURNING id
		`, orgID, "test_"+name).Scan(&categoryID); err2 != nil {
			t.Fatalf("seed: pick/create category: %v", err2)
		}
		// Only register cleanup for categories that SeedProduct created
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM categories WHERE id=$1`, categoryID)
		})
	}

	var prodID uuid.UUID
	sku := fmt.Sprintf("TEST-%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `
		INSERT INTO products (org_id, category_id, name, sku, price)
		VALUES ($1, $2, $3, $4, $5) RETURNING id
	`, orgID, categoryID, name, sku, pricePaise).Scan(&prodID); err != nil {
		t.Fatalf("seed product: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM products WHERE id=$1`, prodID)
	})
	return prodID
}
