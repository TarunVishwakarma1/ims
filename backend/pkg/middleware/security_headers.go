package middleware

import "net/http"

// SecurityHeaders adds OWASP-recommended headers to every response.
// Defends against XSS, clickjacking, MIME sniffing, info leakage.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()

			// Prevent MIME type sniffing
			h.Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking — no embedding in iframes
			h.Set("X-Frame-Options", "DENY")

			// Disable legacy XSS protection (modern browsers use CSP)
			h.Set("X-XSS-Protection", "0")

			// Force HTTPS (effective once deployed behind SSL)
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")

			// Limit referrer info leakage
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Restrict resource origins. API-only — no inline JS, no eval.
			h.Set("Content-Security-Policy",
				"default-src 'none'; frame-ancestors 'none'; base-uri 'none'")

			// Disable powerful browser features
			h.Set("Permissions-Policy",
				"geolocation=(), camera=(), microphone=(), payment=()")

			// Modern alternative to Feature-Policy
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-site")

			next.ServeHTTP(w, r)
		})
	}
}
