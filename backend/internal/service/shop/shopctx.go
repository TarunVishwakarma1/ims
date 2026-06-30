package shop

import (
	"context"

	"github.com/google/uuid"
)

// shopOrgCtxKey carries the per-request shop org resolved from a {shop} slug.
// Lives in this package (not pkg/middleware) because middleware already imports
// this package — the reverse import would be a cycle.
type shopOrgCtxKey struct{}

// WithShopOrg returns a context carrying the selected shop's org id. Set by the
// ResolveShop middleware on per-shop routes.
func WithShopOrg(ctx context.Context, orgID uuid.UUID) context.Context {
	return context.WithValue(ctx, shopOrgCtxKey{}, orgID)
}

// shopOrgFromContext returns the resolved shop org, if a per-shop route set it.
func shopOrgFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(shopOrgCtxKey{}).(uuid.UUID)
	return v, ok
}
