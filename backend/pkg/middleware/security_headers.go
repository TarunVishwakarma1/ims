package middleware

import "net/http"

// SecurityHeaders adds OWASP-recommended headers to every response.
// Defends against XSS, clickjacking, MIME sniffing, info leakage.
//
// `env` selects between dev and prod posture:
//   - prod: HSTS with two-year max-age + preload (only safe on HTTPS).
//   - dev:  HSTS skipped — accidental localhost HSTS pinning would
//           force the browser to https:// for every dev session.
func SecurityHeaders(env string) func(http.Handler) http.Handler {
	prod := env == "production"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()

			// Prevent MIME type sniffing
			h.Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking — no embedding in iframes
			h.Set("X-Frame-Options", "DENY")

			// Disable legacy XSS protection (modern browsers use CSP)
			h.Set("X-XSS-Protection", "0")

			// Force HTTPS — only in prod. Setting on localhost would lock
			// the dev browser into https:// even when the dev server is HTTP.
			if prod {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}

			// Limit referrer info leakage
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// API-only CSP: no scripts, no embedding, no base.
			// The Next.js web app at /web is a separate deployable and sets
			// its own CSP (with nonces) via next.config.js.
			h.Set("Content-Security-Policy",
				"default-src 'none'; frame-ancestors 'none'; base-uri 'none'")

			// Disable powerful browser features
			h.Set("Permissions-Policy",
				"geolocation=(), camera=(), microphone=(), payment=(), "+
					"usb=(), magnetometer=(), gyroscope=(), accelerometer=()")

			// Modern alternative to Feature-Policy
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-site")

			next.ServeHTTP(w, r)
		})
	}
}
