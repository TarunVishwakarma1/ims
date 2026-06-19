package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey string

const (
	ContextKeyUserID contextKey = "userID"
	ContextKeyOrgID  contextKey = "orgID"
	ContextKeyRole   contextKey = "role"
)

type Claims struct {
	UserID string `json:"user_id"`
	OrgID  string `json:"org_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := ""
			viaHeader := false

			// Preferred: Authorization: Bearer <token>
			if h := r.Header.Get("Authorization"); h != "" {
				if len(h) < 7 || strings.ToLower(h[:7]) != "bearer " {
					http.Error(w, "Unauthorized: invalid authorization header format", http.StatusUnauthorized)
					return
				}
				tokenStr = h[7:]
				viaHeader = true
			}

			// Fallback: cookie (for SSE — EventSource can't set headers).
			// CSRF safety: cookies are ONLY accepted on GET/HEAD. State-
			// mutating verbs require the Authorization header, which the
			// browser will not attach automatically — so a malicious cross-
			// origin form/img submit can't forge a state change.
			if tokenStr == "" {
				if c, err := r.Cookie("ims_access_token"); err == nil && c.Value != "" {
					if !isSafeMethod(r.Method) {
						http.Error(w, "Unauthorized: cookie auth not allowed on state-changing methods (use Authorization header)", http.StatusUnauthorized)
						return
					}
					tokenStr = c.Value
				}
			}
			_ = viaHeader // reserved for future audit logging

			if tokenStr == "" {
				http.Error(w, "Unauthorized: missing authorization", http.StatusUnauthorized)
				return
			}
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "Unauthorized: invalid or expired token", http.StatusUnauthorized)
				return
			}

			if claims.RegisteredClaims.Subject != "access" {
				http.Error(w, "Unauthorized: invalid token type", http.StatusUnauthorized)
				return
			}

			// reject shop-audience tokens on B2B routes
			for _, a := range claims.RegisteredClaims.Audience {
				if a == "shop" {
					http.Error(w, "Unauthorized: wrong audience", http.StatusUnauthorized)
					return
				}
			}

			if claims.UserID == "" {
				http.Error(w, "Unauthorized: invalid token claims", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, ContextKeyOrgID, claims.OrgID)
			ctx = context.WithValue(ctx, ContextKeyRole, claims.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// isSafeMethod returns true for HTTP methods that don't change server
// state. Used by Auth to gate cookie-fallback behind safe verbs only.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(ContextKeyUserID).(string)
	return userID, ok
}

func GetOrgIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	orgIDStr, ok := ctx.Value(ContextKeyOrgID).(string)
	if !ok || orgIDStr == "" {
		return uuid.Nil, false
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return uuid.Nil, false
	}
	return orgID, true
}

func GetRoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(ContextKeyRole).(string)
	return role, ok
}

// Actor is the parsed identity for a single request. Populated once by
// Auth middleware (UUID parsing happens there), then handed to handlers
// via ActorFromRequest — no more per-handler uuid.Parse boilerplate.
type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   string
}

// ActorFromRequest extracts the resolved Actor from the request context.
// Returns ok=false when any required claim is missing or unparseable —
// handler should respond 401. After this, handlers never call
// GetOrgIDFromContext / GetUserIDFromContext directly.
func ActorFromRequest(r *http.Request) (Actor, bool) {
	ctx := r.Context()
	orgID, ok := GetOrgIDFromContext(ctx)
	if !ok {
		return Actor{}, false
	}
	userStr, _ := GetUserIDFromContext(ctx)
	userID, err := uuid.Parse(userStr)
	if err != nil {
		return Actor{}, false
	}
	role, _ := GetRoleFromContext(ctx)
	return Actor{UserID: userID, OrgID: orgID, Role: role}, true
}
