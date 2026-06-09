package routes

import (
	"auth/internal/controllers"
	"auth/internal/services"
	"database/sql"

	"net/http"

	"github.com/redis/go-redis/v9"
)

// RegisterResetPasswordRoutes sets up the reset password endpoints
func RegisterResetPasswordRoutes(mux *http.ServeMux, db *sql.DB, tokenService *services.TokenService, redisClient *redis.Client) {
	resetController := &controllers.ResetPasswordController{
		DB:           db,
		TokenService: tokenService,
		RedisClient:  redisClient,
	}

	// Step 1: Request reset link
	mux.HandleFunc("/request-reset", resetController.RequestResetHandler)

	// Step 2: Confirm reset with new password
	mux.HandleFunc("/confirm-reset", resetController.ConfirmResetHandler)
}
