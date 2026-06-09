package routes

import (
	"auth/internal/controllers"
	"auth/internal/middleware"
	"auth/internal/services"
	"database/sql"
	"net/http"

	"github.com/redis/go-redis/v9"
)

func RegisterLoginRoutes(mux *http.ServeMux, db *sql.DB, tokenService *services.TokenService, redisClient *redis.Client) {
	loginController := &controllers.LoginController{
		DB:           db,
		TokenService: tokenService,
		RedisClient:  redisClient,
	}

	// Middlewares
	idem := middleware.NewIdempotencyMiddleware(redisClient, tokenService.TokenExpiry)
	auth := middleware.NewAuthMiddleware(tokenService)

	// STEP 1: POST /login → authenticate credentials, send OTP
	mux.Handle("/login", idem.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		loginController.LoginHandler(w, r)
	})))

	// STEP 2: POST /login/otp → validate OTP, issue tokens
	mux.Handle("/login/otp", idem.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		loginController.OTPHandler(w, r)
	})))

	// POST /refresh → renew access token
	mux.Handle("/refresh", idem.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		loginController.RefreshHandler(w, r)
	})))

	// Example protected route: GET /profile
	mux.Handle("/profile", auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("This is a protected profile endpoint"))
	})))
}
