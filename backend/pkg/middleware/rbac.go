package middleware

import (
	"net/http"
	"slices"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
)

func RequireRole(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleStr, ok := GetRoleFromContext(r.Context())
			if !ok {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			userRole := domain.Role(roleStr)
			if !slices.Contains(roles, userRole) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
