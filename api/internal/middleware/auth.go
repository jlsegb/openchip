package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/openchip/openchip/api/internal/auth"
	"github.com/openchip/openchip/api/internal/httpx"
)

type contextKey string

const ClaimsKey contextKey = "claims"
const SessionCookieName = "openchip_session"

func RequireJWT(secret string, requireAdmin bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerTokenFromRequest(r)
			if token == "" {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing session token")
				return
			}

			claims, err := auth.Parse(secret, token)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid token")
				return
			}
			if requireAdmin && claims.Role != "admin" {
				httpx.WriteError(w, http.StatusForbidden, "forbidden", "admin role required")
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerTokenFromRequest(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}

	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func RequireAPIKey(keys map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			org, ok := keys[key]
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid api key")
				return
			}
			ctx := context.WithValue(r.Context(), contextKey("api_org"), org)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClaimsFromContext(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(ClaimsKey).(*auth.Claims)
	return claims
}

func APIOrgFromContext(ctx context.Context) string {
	org, _ := ctx.Value(contextKey("api_org")).(string)
	return org
}
