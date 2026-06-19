package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

const ContextKeyCustomerID contextKey = "customerID"

func RequireCustomer(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if len(h) < 8 || strings.ToLower(h[:7]) != "bearer " {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			cid, err := shop.ParseShopJWT(secret, h[7:])
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ContextKeyCustomerID, cid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetCustomerIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ContextKeyCustomerID).(uuid.UUID)
	return v, ok
}
