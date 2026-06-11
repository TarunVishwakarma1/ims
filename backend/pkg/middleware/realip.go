package middleware

import (
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/pkg/utils"
)

func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := utils.GetClientIP(r); ip != "" {
			r.RemoteAddr = ip
		}
		next.ServeHTTP(w, r)
	})
}
