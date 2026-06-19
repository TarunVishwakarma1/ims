package middleware

import (
	"net/http"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// RequestLogger is a chi-compatible middleware that emits one structured
// log line per HTTP request. Replaces chi's stdlib-text Logger so file
// logs stay JSON and `jq` / log aggregators can parse them.
//
// Each line carries:
//
//	request_id  — set by chiMiddleware.RequestID
//	method      — HTTP verb
//	path        — request URI (with query string)
//	status      — response status code
//	bytes       — response body size
//	duration_ms — wall time in milliseconds
//	remote      — client IP (set by RealIP middleware if mounted before this)
//	user_agent  — User-Agent header
//
// Level is chosen from status: 5xx → error, 4xx → warn, else info. So a
// single "level: error" filter in the log viewer surfaces real failures
// without dragging in every 404 / 401.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chiMiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			dur := time.Since(start)
			fields := []zap.Field{
				zap.String("request_id", chiMiddleware.GetReqID(r.Context())),
				zap.String("method", r.Method),
				zap.String("path", r.RequestURI),
				zap.Int("status", ww.Status()),
				zap.Int("bytes", ww.BytesWritten()),
				zap.Float64("duration_ms", float64(dur.Microseconds())/1000.0),
				zap.String("remote", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
			}
			log := zap.L()
			switch {
			case ww.Status() >= 500:
				log.Error("http", fields...)
			case ww.Status() >= 400:
				log.Warn("http", fields...)
			default:
				log.Info("http", fields...)
			}
		}()
		next.ServeHTTP(ww, r)
	})
}
