// Package auth — HTTP middleware for JWT authentication and RBAC.
package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// ── Context keys ─────────────────────────────────────────────────────────────

type ctxKey string

const (
	// CTXClaims holds the parsed JWT claims in the request context.
	CTXClaims ctxKey = "auth:claims"
	// CTXToken holds the raw JWT token string in the request context.
	CTXToken ctxKey = "auth:token"
)

// ── Middleware ───────────────────────────────────────────────────────────────

// AuthRequired is middleware that validates the Bearer token and injects Claims
// into the request context. Unauthenticated requests get a 401 JSON response.
func AuthRequired(manager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip CORS preflight
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			token := extractBearer(r)
			if token == "" {
				writeAuthError(w, http.StatusUnauthorized, "missing Authorization header")
				return
			}

			claims, err := manager.ValidateToken(token)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "invalid or expired token: "+err.Error())
				return
			}

			if claims.TokenType != AccessToken {
				writeAuthError(w, http.StatusUnauthorized, "token is not an access token")
				return
			}

			ctx := context.WithValue(r.Context(), CTXClaims, claims)
			ctx = context.WithValue(ctx, CTXToken, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns middleware that enforces a minimum role for the endpoint.
// Must be used after AuthRequired.
func RequireRole(role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaims(r.Context())
			if !ok {
				writeAuthError(w, http.StatusForbidden, "no claims in context")
				return
			}

			if !hasAtLeastRole(claims.Role, role) {
				writeAuthError(w, http.StatusForbidden,
					"insufficient permissions: role "+string(claims.Role)+" requires at least "+string(role))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission returns middleware that checks a specific resource+action pair.
// Must be used after AuthRequired.
func RequirePermission(resource Resource, action Action) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaims(r.Context())
			if !ok {
				writeAuthError(w, http.StatusForbidden, "no claims in context")
				return
			}

			if !HasPermission(claims.Role, resource, action) {
				writeAuthError(w, http.StatusForbidden,
					"permission denied: cannot "+string(action)+" "+string(resource))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORS adds CORS headers for SSE (Server-Sent Events) support and browser-based auth.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowOrigin := "*"
			if allowedOrigins[0] != "*" {
				allowOrigin = ""
				for _, o := range allowedOrigins {
					if o == origin {
						allowOrigin = o
						break
					}
				}
			}

			if allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Expose-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Logging is optional middleware that logs auth attempts.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearer(r)
			masked := "none"
			if token != "" {
				if len(token) > 8 {
					masked = token[:4] + "..." + token[len(token)-4:]
				} else {
					masked = "***"
				}
			}
			logger.Debug("auth attempt",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("token", masked),
			)
			next.ServeHTTP(w, r)
		})
	}
}

// ── Context helpers ──────────────────────────────────────────────────────────

// GetClaims extracts JWT claims from the request context.
func GetClaims(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(CTXClaims).(*Claims)
	return claims, ok
}

// GetToken extracts the raw JWT from the request context.
func GetToken(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(CTXToken).(string)
	return token, ok
}

// ── HTTP helpers ─────────────────────────────────────────────────────────────

// extractBearer pulls the token from the Authorization header.
func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		// Also check query param for SSE connections
		if t := r.URL.Query().Get("token"); t != "" {
			return t
		}
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}

// writeAuthError writes a JSON error response for auth failures.
func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}

// ── Role comparison ──────────────────────────────────────────────────────────

var roleHierarchy = map[Role]int{
	RoleAdmin:    3,
	RoleOperator: 2,
	RoleViewer:   1,
}

// hasAtLeastRole checks if the given role is at least the required role.
func hasAtLeastRole(actual, required Role) bool {
	return roleHierarchy[actual] >= roleHierarchy[required]
}

// ── Compile-time interface checks ────────────────────────────────────────────

// AuthMiddleware is the interface satisfied by AuthRequired's result.
type AuthMiddleware func(http.Handler) http.Handler

var _ AuthMiddleware = AuthRequired(nil)