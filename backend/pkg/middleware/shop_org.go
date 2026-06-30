package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	shopsvc "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

// ShopResolver maps a shop slug to its owning org id.
type ShopResolver func(ctx context.Context, slug string) (uuid.UUID, error)

// ResolveShop reads the {shop} URL param, resolves it to an org, and stashes
// the org id in the request context (via shopsvc.WithShopOrg). A miss 404s.
// Shop services read it from context, falling back to their configured default
// org when absent (legacy single-shop routes).
func ResolveShop(resolve ShopResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := chi.URLParam(r, "shop")
			org, err := resolve(r.Context(), slug)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"shop_not_found"}`))
				return
			}
			next.ServeHTTP(w, r.WithContext(shopsvc.WithShopOrg(r.Context(), org)))
		})
	}
}
