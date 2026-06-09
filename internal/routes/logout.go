package routes

import (
	"auth/internal/controllers"
	"auth/internal/middleware"
	"auth/internal/services"
	"database/sql"
	"net/http"

	"github.com/redis/go-redis/v9"
)

func RegisterLogoutRoutes(mux *http.ServeMux, db *sql.DB, tokenService *services.TokenService, redisClient *redis.Client) {
	logoutController := &controllers.LogoutController{
		DB:           db,
		TokenService: tokenService,
		RedisClient:  redisClient,
	}

	// Middleware: require authentication
	auth := middleware.NewAuthMiddleware(tokenService)

	// POST /logout → invalidate sessions
	mux.Handle("/logout", auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		logoutController.LogoutHandler(w, r)
	})))
}
