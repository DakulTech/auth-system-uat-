package middleware

import (
	"net/http"
	"strings"

	"auth/internal/services"
)

type AuthMiddleware struct {
	TokenService *services.TokenService
}

func NewAuthMiddleware(ts *services.TokenService) *AuthMiddleware {
	return &AuthMiddleware{TokenService: ts}
}

func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		// Expect "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}
		accessToken := parts[1]

		// For now, just check token exists in DB or is syntactically valid
		// (stub: you can extend to JWT verification or DB lookup)
		if accessToken == "" {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}
